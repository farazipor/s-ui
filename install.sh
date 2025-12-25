#!/usr/bin/env bash
set -euo pipefail

REPO="farazipor/s-ui"

echo "The OS release is: $(. /etc/os-release && echo "$ID")"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ASSET="s-ui-linux-amd64.tar.gz" ;;
  aarch64|arm64) ASSET="s-ui-linux-arm64.tar.gz" ;;
  *) echo "Unsupported arch: $ARCH" ; exit 1 ;;
esac

apt-get update -y
apt-get install -y curl tar tzdata wget ca-certificates

# گرفتن آخرین ورژن از GitHub Releases
TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -m1 '"tag_name":' | cut -d '"' -f4)"
if [[ -z "${TAG}" ]]; then
  echo "Could not detect latest release tag."
  exit 1
fi

echo "Got s-ui latest version: ${TAG}, beginning the installation..."

URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
TMP="/tmp/${ASSET}"

wget -O "${TMP}" "${URL}"
rm -rf /tmp/s-ui
mkdir -p /tmp/s-ui
tar -xzf "${TMP}" -C /tmp/s-ui

# نصب به /usr/local
rm -rf /usr/local/s-ui
mkdir -p /usr/local/s-ui
cp -r /tmp/s-ui/s-ui/* /usr/local/s-ui/
chmod +x /usr/local/s-ui/sui /usr/local/s-ui/s-ui.sh || true

# سرویس systemd
cp /usr/local/s-ui/s-ui.service /etc/systemd/system/s-ui.service
systemctl daemon-reload
systemctl enable --now s-ui

echo "Install/update finished! For security it's recommended to modify panel settings"

if [[ -x /usr/local/s-ui/s-ui.sh ]]; then
  echo -n "Do you want to continue with the modification [y/n]?: "
  read -r yn
  if [[ "${yn}" =~ ^[Yy]$ ]]; then
    /usr/local/s-ui/s-ui.sh
  fi
else
  echo "NOTE: /usr/local/s-ui/s-ui.sh not found/executable. Skipping interactive config."
fi
