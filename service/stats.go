package service

import (
	"sync"
	"time"

	"github.com/alireza0/s-ui/database"
	"github.com/alireza0/s-ui/database/model"

	"gorm.io/gorm"
)

type onlines struct {
	Inbound  []string `json:"inbound,omitempty"`
	User     []string `json:"user,omitempty"`
	Outbound []string `json:"outbound,omitempty"`
}

var (
	onlineResources   = &onlines{}
	onlineUserCount   = map[string]int{}
	onlineResourcesMu sync.RWMutex
)

type StatsService struct {
}

func (s *StatsService) SaveStats(enableTraffic bool) error {
	if !corePtr.IsRunning() {
		return nil
	}
	stats := corePtr.GetInstance().StatsTracker().GetStats()

	// Build next snapshots (avoid mutating shared globals while iterating)
	nextOnlineResources := onlines{}
	nextOnlineUserCount := map[string]int{}

	if len(*stats) == 0 {
		onlineResourcesMu.Lock()
		onlineResources = &onlines{}
		onlineUserCount = map[string]int{}
		onlineResourcesMu.Unlock()
		return nil
	}

	var err error
	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
		} else {
			tx.Rollback()
		}
	}()

	for _, stat := range *stats {
		if stat.Resource == "user" {
			if stat.Direction {
				err = tx.Model(model.Client{}).Where("name = ?", stat.Tag).
					UpdateColumn("up", gorm.Expr("up + ?", stat.Traffic)).Error
			} else {
				err = tx.Model(model.Client{}).Where("name = ?", stat.Tag).
					UpdateColumn("down", gorm.Expr("down + ?", stat.Traffic)).Error
			}
			if err != nil {
				return err
			}
		}

		// online snapshot and per-user count (upload direction only)
		if stat.Direction {
			switch stat.Resource {
			case "inbound":
				nextOnlineResources.Inbound = append(nextOnlineResources.Inbound, stat.Tag)
			case "outbound":
				nextOnlineResources.Outbound = append(nextOnlineResources.Outbound, stat.Tag)
			case "user":
				nextOnlineResources.User = append(nextOnlineResources.User, stat.Tag)
				nextOnlineUserCount[stat.Tag]++
			}
		}
	}

	// Swap snapshots atomically
	onlineResourcesMu.Lock()
	onlineResources.Inbound = nextOnlineResources.Inbound
	onlineResources.Outbound = nextOnlineResources.Outbound
	onlineResources.User = nextOnlineResources.User
	onlineUserCount = nextOnlineUserCount
	onlineResourcesMu.Unlock()

	if !enableTraffic {
		return nil
	}
	return tx.Create(&stats).Error
}

func (s *StatsService) GetStats(resource string, tag string, limit int) ([]model.Stats, error) {
	var err error
	var result []model.Stats

	currentTime := time.Now().Unix()
	timeDiff := currentTime - (int64(limit) * 3600)

	db := database.GetDB()
	resources := []string{resource}
	if resource == "endpoint" {
		resources = []string{"inbound", "outbound"}
	}
	err = db.Model(model.Stats{}).Where("resource in ? AND tag = ? AND date_time > ?", resources, tag, timeDiff).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *StatsService) GetOnlines() (onlines, error) {
	onlineResourcesMu.RLock()
	defer onlineResourcesMu.RUnlock()
	return *onlineResources, nil
}

func (s *StatsService) GetOnlineUserCounts() map[string]int {
	onlineResourcesMu.RLock()
	defer onlineResourcesMu.RUnlock()

	result := make(map[string]int, len(onlineUserCount))
	for user, count := range onlineUserCount {
		result[user] = count
	}
	return result
}

func (s *StatsService) DelOldStats(days int) error {
	oldTime := time.Now().AddDate(0, 0, -(days)).Unix()
	db := database.GetDB()
	return db.Where("date_time < ?", oldTime).Delete(model.Stats{}).Error
}
