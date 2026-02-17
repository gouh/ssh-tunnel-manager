#!/bin/bash

# SSH Tunnel Manager - Installation Script

set -e

REPO="gouh/ssh-tunnel-manager"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="ssh-tunnel-manager"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

detect_os_arch() {
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
            echo -e "${RED}✗ Unsupported operating system: $OS${NC}"
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
            echo -e "${RED}✗ Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac

    DOWNLOAD_FILE="ssh-tunnel-manager-${OS_NAME}-${ARCH_NAME}"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${DOWNLOAD_FILE}"
}

print_banner() {
    clear
    echo -e "${MAGENTA}"
    echo '  ███████╗██╗  ██╗ █████╗ ██████╗  ██████╗  █████╗ ██████╗  ██████╗ ███████╗██████╗ '
    echo '  ██╔════╝██║  ██║██╔══██╗██╔══██╗██╔═══██╗██╔══██╗██╔══██╗██╔═══██╗██╔════╝██╔══██╗'
    echo '  ███████╗███████║███████║██║  ██║██║   ██║███████║██████╔╝██║   ██║█████╗  ██████╔╝'
    echo '  ╚════██║██╔══██║██╔══██║██║  ██║██║   ██║██╔══██║██╔══██╗██║   ██║██╔══╝  ██╔══██╗'
    echo '  ███████║██║  ██║██║  ██║██████╔╝╚██████╔╝██║  ██║██║  ██║╚██████╔╝██║     ██║  ██║'
    echo '  ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═════╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝     ╚═╝  ╚═╝'
    echo -e "${NC}"
    echo -e "${CYAN}                         T U N N E L   M A N A G E R${NC}"
    echo ""
}

print_status() {
    echo -e "${YELLOW}┌─────────────────────────────────────────────┐${NC}"
    echo -e "${YELLOW}│${NC}  ${GREEN}►${NC} $1"
    echo -e "${YELLOW}└─────────────────────────────────────────────┘${NC}"
}

spinner() {
    local pid=$1
    local delay=0.1
    local spinstr='|/-\'
    echo -ne "${CYAN}"
    while ps -p $pid > /dev/null 2>&1; do
        local temp=${spinstr#?}
        printf " [%c] " "$spinstr"
        local spinstr=$temp${spinstr%"$temp"}
        sleep $delay
        printf "\b\b\b\b\b"
    done
    printf "\b\b\b\b\b"
    echo -ne "${NC}"
}

print_banner

echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
echo -e "  ${YELLOW}⚡${NC}  Installing SSH Tunnel Manager (latest)"
echo -e "  ${YELLOW}⚡${NC}  Detecting system..."
echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
echo ""

detect_os_arch

echo -e "  ${CYAN}▸${NC} OS:           ${GREEN}$OS_NAME${NC}"
echo -e "  ${CYAN}▸${NC} Architecture: ${GREEN}$ARCH_NAME${NC}"
echo -e "  ${CYAN}▸${NC} Binary:       ${GREEN}$DOWNLOAD_FILE${NC}"
echo ""

echo -ne "  ${YELLOW}▸${NC} Downloading...  "
if command -v curl &> /dev/null; then
    curl -L -f -o "/tmp/${BINARY_NAME}" "${DOWNLOAD_URL}" 2>/dev/null &
    CURL_PID=$!
    spinner $CURL_PID
    wait $CURL_PID
    CURL_STATUS=$?
elif command -v wget &> /dev/null; then
    wget -q -O "/tmp/${BINARY_NAME}" "${DOWNLOAD_URL}" 2>/dev/null &
    WGET_PID=$!
    spinner $WGET_PID
    wait $WGET_PID
    WGET_STATUS=$?
else
    echo -e "${RED}✗${NC}"
    echo -e "  ${RED}Error: curl or wget is required${NC}"
    exit 1
fi

if [ $? -ne 0 ]; then
    echo -e "${RED}✗${NC}"
    echo ""
    echo -e "${RED}═══════════════════════════════════════════════════${NC}"
    echo -e "  ${RED}✗ ERROR: Failed to download binary${NC}"
    echo -e "  ${RED}✗ URL: ${DOWNLOAD_URL}${NC}"
    echo -e "${RED}═══════════════════════════════════════════════════${NC}"
    exit 1
fi
echo -e "${GREEN}✓${NC}"

if file "/tmp/${BINARY_NAME}" | grep -q "text"; then
    echo -e "${RED}✗ Error: Downloaded file is not a binary${NC}"
    echo "This usually means the release doesn't exist yet."
    echo "Please create the release at: https://github.com/${REPO}/releases/new"
    rm "/tmp/${BINARY_NAME}" 2>/dev/null || true
    exit 1
fi

chmod +x "/tmp/${BINARY_NAME}"

echo -ne "  ${YELLOW}▸${NC} Installing...   "
if [ -w "$INSTALL_DIR" ]; then
    mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
else
    sudo mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
fi
echo -e "${GREEN}✓${NC}"

echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo -e "  ${GREEN}✓ Installation complete!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════${NC}"
echo ""
echo -e "  ${CYAN}Run:${NC} ${YELLOW}ssh-tunnel-manager${NC} to start"
echo ""
