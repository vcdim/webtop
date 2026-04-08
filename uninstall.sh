#!/bin/bash
set -e

if [ "$(id -u)" -ne 0 ]; then
  exec sudo bash "$0" "$@"
fi

echo "Uninstalling portview..."

# Stop and disable service if running
systemctl stop portview 2>/dev/null || true
systemctl disable portview 2>/dev/null || true

# Remove package
apt-get remove -y portview -qq

# Remove APT repo source
rm -f /etc/apt/sources.list.d/portview.list

echo ""
echo "portview uninstalled."
