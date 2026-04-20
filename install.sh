#!/bin/bash
set -e

# Detect OS and architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

case $OS in
  Linux)
    OS="linux"
    ;;
  Darwin)
    OS="darwin"
    ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

case $ARCH in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Get latest version
LATEST_VERSION=$(curl -s https://api.github.com/repos/NotHarshhaa/pod-why-dead/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
  echo "Failed to fetch latest version"
  exit 1
fi

echo "Installing pod-why-dead $LATEST_VERSION for $OS-$ARCH..."

# Download and extract
DOWNLOAD_URL="https://github.com/NotHarshhaa/pod-why-dead/releases/download/${LATEST_VERSION}/pod-why-dead_${LATEST_VERSION}_${OS}_${ARCH}.tar.gz"
TMP_DIR=$(mktemp -d)
cd $TMP_DIR

curl -sSL $DOWNLOAD_URL -o pod-why-dead.tar.gz
tar xzf pod-why-dead.tar.gz

# Install
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

mv pod-why-dead "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/pod-why-dead"

# Cleanup
cd -
rm -rf $TMP_DIR

echo "Successfully installed pod-why-dead to $INSTALL_DIR/pod-why-dead"
echo "Make sure $INSTALL_DIR is in your PATH"
