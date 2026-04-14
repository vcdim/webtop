#!/bin/bash
set -e

OS="$(uname -s)"

if [ "$OS" = "Darwin" ]; then
  echo "Uninstalling webtop..."
  brew uninstall webtop 2>/dev/null || echo "webtop not found in Homebrew"
  echo "webtop uninstalled."

elif [ "$OS" = "Linux" ]; then
  if [ "$(id -u)" -ne 0 ]; then
    echo "Error: root required. Run: curl -fsSL https://vcdim.github.io/webtop/uninstall.sh | sudo bash"
    exit 1
  fi
  echo "Uninstalling webtop..."
  systemctl stop webtop 2>/dev/null || true
  systemctl disable webtop 2>/dev/null || true
  apt-get remove -y webtop -qq
  rm -f /etc/apt/sources.list.d/webtop.list
  echo "webtop uninstalled."

else
  echo "Error: Unsupported OS: $OS"
  exit 1
fi
