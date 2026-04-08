#!/bin/bash
set -e

OS="$(uname -s)"

# Check root on Linux
if [ "$OS" = "Linux" ] && [ "$(id -u)" -ne 0 ]; then
  echo "Error: root required. Run: curl -fsSL https://vcdim.github.io/portview/install.sh | sudo bash"
  exit 1
fi

echo "Installing portview..."

if [ "$OS" = "Darwin" ]; then
  # macOS — use Homebrew
  if ! command -v brew &>/dev/null; then
    echo "Error: Homebrew is required. Install it from https://brew.sh"
    exit 1
  fi
  brew install vcdim/portview/portview

  echo ""
  echo "portview installed!"
  echo ""
  echo "To start: portview"
  echo "To uninstall: brew uninstall portview"

elif [ "$OS" = "Linux" ]; then
  # Linux — use APT
  echo "deb [trusted=yes] https://vcdim.github.io/portview/ /" > /etc/apt/sources.list.d/portview.list
  apt-get update -o Dir::Etc::sourcelist="sources.list.d/portview.list" -o Dir::Etc::sourceparts="-" -o APT::Get::List-Cleanup="0" -qq
  apt-get install -y portview -qq

  echo ""
  echo "portview installed!"
  echo ""
  echo "To start now:        sudo portview"
  echo "To run as a service: sudo systemctl enable --now portview"
  echo "To stop the service: sudo systemctl stop portview"
  echo "Then open:           http://localhost:9999"
  echo ""
  echo "To uninstall:        curl -fsSL https://vcdim.github.io/portview/uninstall.sh | sudo bash"

else
  echo "Error: Unsupported OS: $OS"
  exit 1
fi
