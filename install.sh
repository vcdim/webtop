#!/bin/bash
set -e

OS="$(uname -s)"

# Check root on Linux
if [ "$OS" = "Linux" ] && [ "$(id -u)" -ne 0 ]; then
  echo "Error: root required. Run: curl -fsSL https://vcdim.github.io/webtop/install.sh | sudo bash"
  exit 1
fi

echo "Installing webtop..."

if [ "$OS" = "Darwin" ]; then
  # macOS — use Homebrew
  if ! command -v brew &>/dev/null; then
    echo "Error: Homebrew is required. Install it from https://brew.sh"
    exit 1
  fi
  brew install vcdim/webtop/webtop

  echo ""
  echo "webtop installed!"
  echo ""
  echo "To start: webtop"
  echo "To uninstall: brew uninstall webtop"

elif [ "$OS" = "Linux" ]; then
  # Linux — use APT
  echo "deb [trusted=yes] https://vcdim.github.io/webtop/ /" > /etc/apt/sources.list.d/webtop.list
  apt-get update -o Dir::Etc::sourcelist="sources.list.d/webtop.list" -o Dir::Etc::sourceparts="-" -o APT::Get::List-Cleanup="0" -qq
  apt-get install -y webtop -qq

  echo ""
  echo "webtop installed!"
  echo ""
  echo "To start now:        sudo webtop"
  echo "To run as a service: sudo systemctl enable --now webtop"
  echo "To stop the service: sudo systemctl stop webtop"
  echo "Then open:           http://localhost:9999"
  echo ""
  echo "To uninstall:        curl -fsSL https://vcdim.github.io/webtop/uninstall.sh | sudo bash"

else
  echo "Error: Unsupported OS: $OS"
  exit 1
fi
