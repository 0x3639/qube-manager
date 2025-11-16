#!/bin/sh
set -e

# Qube Manager Installation Script
# Usage: curl -fsSL https://raw.githubusercontent.com/hypercore-one/qube-manager/master/scripts/install.sh | sh
# Or: wget -qO- https://raw.githubusercontent.com/hypercore-one/qube-manager/master/scripts/install.sh | sh

REPO="hypercore-one/qube-manager"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/.qube-manager"
SERVICE_USER="${USER}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
info() {
    printf "${GREEN}[INFO]${NC} %s\n" "$1"
}

warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$1"
}

error() {
    printf "${RED}[ERROR]${NC} %s\n" "$1"
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    BINARY_NAME="qube-manager-${OS}-${ARCH}"
    info "Detected platform: ${OS}/${ARCH}"
}

# Check if running as root (needed for systemd service)
check_root() {
    if [ "$(id -u)" -eq 0 ]; then
        SUDO=""
        IS_ROOT=1
    else
        SUDO="sudo"
        IS_ROOT=0
        info "Running as non-root user. Will use sudo for system operations."
    fi
}

# Get latest release version
get_latest_release() {
    info "Fetching latest release information..."

    # Try using GitHub API
    if command -v curl >/dev/null 2>&1; then
        LATEST_RELEASE=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        LATEST_RELEASE=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        error "Neither curl nor wget found. Please install one of them."
    fi

    if [ -z "$LATEST_RELEASE" ]; then
        error "Failed to fetch latest release information"
    fi

    info "Latest release: ${LATEST_RELEASE}"
}

# Download binary
download_binary() {
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_RELEASE}/${BINARY_NAME}"
    TEMP_FILE="/tmp/qube-manager-$$"

    info "Downloading qube-manager from ${DOWNLOAD_URL}..."

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "${DOWNLOAD_URL}" -o "${TEMP_FILE}" || error "Download failed"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "${TEMP_FILE}" "${DOWNLOAD_URL}" || error "Download failed"
    fi

    # Download checksum
    CHECKSUM_URL="${DOWNLOAD_URL}.sha256"
    CHECKSUM_FILE="${TEMP_FILE}.sha256"

    info "Downloading checksum..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "${CHECKSUM_URL}" -o "${CHECKSUM_FILE}" 2>/dev/null || warn "Checksum download failed, skipping verification"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "${CHECKSUM_FILE}" "${CHECKSUM_URL}" 2>/dev/null || warn "Checksum download failed, skipping verification"
    fi

    # Verify checksum if available
    if [ -f "${CHECKSUM_FILE}" ]; then
        info "Verifying checksum..."
        if command -v sha256sum >/dev/null 2>&1; then
            (cd /tmp && sha256sum -c "${CHECKSUM_FILE##*/}" 2>/dev/null) || error "Checksum verification failed"
        elif command -v shasum >/dev/null 2>&1; then
            (cd /tmp && shasum -a 256 -c "${CHECKSUM_FILE##*/}" 2>/dev/null) || error "Checksum verification failed"
        else
            warn "No SHA256 tool found, skipping verification"
        fi
    fi

    info "Download successful"
}

# Install binary
install_binary() {
    info "Installing qube-manager to ${INSTALL_DIR}..."

    $SUDO mkdir -p "${INSTALL_DIR}"
    $SUDO mv "${TEMP_FILE}" "${INSTALL_DIR}/qube-manager"
    $SUDO chmod +x "${INSTALL_DIR}/qube-manager"

    # Verify installation
    if "${INSTALL_DIR}/qube-manager" --version >/dev/null 2>&1; then
        info "Binary installed successfully"
        "${INSTALL_DIR}/qube-manager" --version
    else
        error "Binary installation verification failed"
    fi

    # Cleanup
    rm -f "${CHECKSUM_FILE}"
}

# Create systemd service (Linux only)
create_systemd_service() {
    if [ "$OS" != "linux" ]; then
        warn "Systemd service creation is only supported on Linux"
        return
    fi

    info "Creating systemd service..."

    SERVICE_FILE="/etc/systemd/system/qube-manager.service"

    $SUDO tee "$SERVICE_FILE" > /dev/null <<EOF
[Unit]
Description=Qube Manager - Decentralized Quorum-based Manager
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
ExecStart=${INSTALL_DIR}/qube-manager
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=qube-manager

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=${CONFIG_DIR}

[Install]
WantedBy=multi-user.target
EOF

    info "Systemd service created at ${SERVICE_FILE}"
    info "Reloading systemd daemon..."
    $SUDO systemctl daemon-reload
}

# Create launchd service (macOS only)
create_launchd_service() {
    if [ "$OS" != "darwin" ]; then
        return
    fi

    info "Creating launchd service..."

    PLIST_FILE="$HOME/Library/LaunchAgents/com.hypercore.qube-manager.plist"
    mkdir -p "$HOME/Library/LaunchAgents"

    cat > "$PLIST_FILE" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.hypercore.qube-manager</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/qube-manager</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${CONFIG_DIR}/qube-manager.log</string>
    <key>StandardErrorPath</key>
    <string>${CONFIG_DIR}/qube-manager.error.log</string>
</dict>
</plist>
EOF

    info "Launchd service created at ${PLIST_FILE}"
}

# Setup configuration
setup_config() {
    info "Setting up configuration directory at ${CONFIG_DIR}..."

    mkdir -p "${CONFIG_DIR}"
    chmod 700 "${CONFIG_DIR}"

    # Run qube-manager once to generate default config
    if [ ! -f "${CONFIG_DIR}/config.yaml" ]; then
        info "Generating default configuration..."
        timeout 2 "${INSTALL_DIR}/qube-manager" 2>/dev/null || true
    fi

    info "Configuration directory ready at ${CONFIG_DIR}"
}

# Main installation flow
main() {
    echo ""
    info "==================================================="
    info "  Qube Manager Installation Script"
    info "==================================================="
    echo ""

    check_root
    detect_platform
    get_latest_release
    download_binary
    install_binary
    setup_config

    # Create service based on OS
    if [ "$OS" = "linux" ]; then
        create_systemd_service
    elif [ "$OS" = "darwin" ]; then
        create_launchd_service
    fi

    echo ""
    info "==================================================="
    info "  Installation Complete!"
    info "==================================================="
    echo ""
    info "Qube Manager has been installed to: ${INSTALL_DIR}/qube-manager"
    info "Configuration directory: ${CONFIG_DIR}"
    echo ""
    info "Next steps:"
    echo ""

    if [ "$OS" = "linux" ]; then
        info "1. Review and edit configuration:"
        echo "   vi ${CONFIG_DIR}/config.yaml"
        echo ""
        info "2. Start the service:"
        echo "   ${SUDO} systemctl start qube-manager"
        echo ""
        info "3. Enable service to start on boot:"
        echo "   ${SUDO} systemctl enable qube-manager"
        echo ""
        info "4. Check service status:"
        echo "   ${SUDO} systemctl status qube-manager"
        echo ""
        info "5. View logs:"
        echo "   ${SUDO} journalctl -u qube-manager -f"
    elif [ "$OS" = "darwin" ]; then
        info "1. Review and edit configuration:"
        echo "   vi ${CONFIG_DIR}/config.yaml"
        echo ""
        info "2. Load the service:"
        echo "   launchctl load ~/Library/LaunchAgents/com.hypercore.qube-manager.plist"
        echo ""
        info "3. Check if service is running:"
        echo "   launchctl list | grep qube-manager"
        echo ""
        info "4. View logs:"
        echo "   tail -f ${CONFIG_DIR}/qube-manager.log"
    else
        info "1. Review and edit configuration:"
        echo "   vi ${CONFIG_DIR}/config.yaml"
        echo ""
        info "2. Run qube-manager:"
        echo "   qube-manager"
    fi

    echo ""
    info "Documentation: https://github.com/${REPO}"
    echo ""
}

main
