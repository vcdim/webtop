#!/bin/bash
set -e

echo "Installing portview..."

# Add APT repository
echo "deb [trusted=yes] https://vcdim.github.io/portview/ /" > /etc/apt/sources.list.d/portview.list

# Install
apt-get update -o Dir::Etc::sourcelist="sources.list.d/portview.list" -o Dir::Etc::sourceparts="-" -o APT::Get::List-Cleanup="0" -qq
apt-get install -y portview -qq

echo "portview installed! Run: sudo portview -p 8080"
