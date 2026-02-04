#!/bin/bash

# SSH Tunnel Manager - Installation Script

set -e

REPO="gouh/ssh-tunnel-manager"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="ssh-tunnel-manager"

# Detect OS and Architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
    linux)
        OS_NAME="linux"
        ;;
    darwin)
        OS_NAME="darwin"
        ;;
    *)
        echo "Unsupported operating system: $OS"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        ARCH_NAME="amd64"
        ;;
    arm64|aarch64)
        ARCH_NAME="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

DOWNLOAD_FILE="ssh-tunnel-manager-${OS_NAME}-${ARCH_NAME}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${DOWNLOAD_FILE}"

echo "🚀 Installing SSH Tunnel Manager (latest)"
echo "   OS: ${OS_NAME}"
echo "   Architecture: ${ARCH_NAME}"
echo ""

# Download binary
echo "📥 Downloading from GitHub..."
if command -v curl &> /dev/null; then
    curl -L -f -o "/tmp/${BINARY_NAME}" "${DOWNLOAD_URL}" || {
        echo "❌ Error: Failed to download binary"
        echo "URL: ${DOWNLOAD_URL}"
        exit 1
    }
elif command -v wget &> /dev/null; then
    wget -O "/tmp/${BINARY_NAME}" "${DOWNLOAD_URL}" || {
        echo "❌ Error: Failed to download binary"
        echo "URL: ${DOWNLOAD_URL}"
        exit 1
    }
else
    echo "❌ Error: curl or wget is required"
    exit 1
fi

# Verify it's a binary file
if file "/tmp/${BINARY_NAME}" | grep -q "text"; then
    echo "❌ Error: Downloaded file is not a binary"
    echo "This usually means the release doesn't exist yet."
    echo "Please create the release at: https://github.com/${REPO}/releases/new"
    rm "/tmp/${BINARY_NAME}"
    exit 1
fi

# Make executable
chmod +x "/tmp/${BINARY_NAME}"

# Install
echo "📦 Installing to ${INSTALL_DIR}..."
if [ -w "$INSTALL_DIR" ]; then
    mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
else
    sudo mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
fi

echo ""
echo "✅ Installation complete!"
echo ""
echo "Run 'ssh-tunnel-manager' to start the application"
