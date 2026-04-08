#!/bin/bash
set -e

OS="$(uname -s)"

if [ "$OS" = "Darwin" ]; then
  echo "Uninstalling portview..."
  brew uninstall portview 2>/dev/null || echo "portview not found in Homebrew"
  echo "portview uninstalled."

elif [ "$OS" = "Linux" ]; then
  if [ "$(id -u)" -ne 0 ]; then
    echo "Error: root required. Run: curl -fsSL https://vcdim.github.io/portview/uninstall.sh | sudo bash"
    exit 1
  fi
  echo "Uninstalling portview..."
  systemctl stop portview 2>/dev/null || true
  systemctl disable portview 2>/dev/null || true
  apt-get remove -y portview -qq
  rm -f /etc/apt/sources.list.d/portview.list
  echo "portview uninstalled."

else
  echo "Error: Unsupported OS: $OS"
  exit 1
fi
