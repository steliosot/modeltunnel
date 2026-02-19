#!/bin/bash
#
# Modeltunnel Update Script
#
# Usage: ./update.sh
#
# Preserves: ~/.config/modeltunnel/ (config.yaml, keys.db, tunnel.url)
#

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

CONFIG_DIR="$HOME/.config/modeltunnel"
BACKUP_DIR="/tmp/modeltunnel-backup-$(date +%Y%m%d-%H%M%S)"
SUDO=""
[ "$EUID" -ne 0 ] && SUDO="sudo"

print_info() { echo -e "${CYAN}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

create_systemd_services() {
    # Only for Linux
    if [ "$(uname -s)" != "Linux" ]; then
        return
    fi

    # Skip if services already exist
    if [ -f "/etc/systemd/system/modeltunnel.service" ]; then
        return
    fi

    print_info "Setting up systemd services..."

    # Create Ollama service
    sudo tee /etc/systemd/system/ollama.service > /dev/null <<EOF
[Unit]
Description=Ollama Local LLM Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ollama serve
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    # Create Modeltunnel service
    sudo tee /etc/systemd/system/modeltunnel.service > /dev/null <<EOF
[Unit]
Description=Modeltunnel API Server
After=network-online.target ollama.service
Wants=ollama.service

[Service]
Type=simple
ExecStart=/usr/local/bin/modeltunnel up --ollama --tunnel
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    # Reload systemd and enable services
    sudo systemctl daemon-reload
    sudo systemctl enable ollama modeltunnel > /dev/null 2>&1 || true
    sudo systemctl restart ollama
    sudo systemctl start modeltunnel

    print_success "Systemd services created and started"
    print_info "Services: sudo systemctl status ollama modeltunnel"
}

# Backup existing data
print_info "Backing up existing installation..."
mkdir -p "$BACKUP_DIR"
[ -d "$CONFIG_DIR" ] && cp -r "$CONFIG_DIR" "$BACKUP_DIR/" 2>/dev/null || true
print_success "Backup at: $BACKUP_DIR"

# Check if running, stop it
print_info "Stopping modeltunnel..."
pgrep -x modeltunnel > /dev/null 2>&1 && $SUDO pkill modeltunnel 2>/dev/null || true
sleep 2

# Download and install using the installer
print_info "Updating modeltunnel (preserving data)..."
TEMP_SCRIPT=$(mktemp)
curl -fsSL https://raw.githubusercontent.com/steliosot/modeltunnel/main/install.sh -o "$TEMP_SCRIPT"

# Run installer in silent mode - it will detect existing config and preserve it
PRINT_MODE="info" bash "$TEMP_SCRIPT" --silent || print_error "Update failed"

rm -f "$TEMP_SCRIPT"

# Create systemd services if they don't exist (for persistence)
create_systemd_services

# Verify and restart
if [ -f "/usr/local/bin/modeltunnel" ]; then
    print_success "New binary installed"

    # Setup systemd services if not present
    create_systemd_services

    # Restart with existing config
    print_info "Restarting..."
    cd /tmp
    sudo nohup /usr/local/bin/modeltunnel up --ollama --tunnel > /dev/null 2>&1 &

    sleep 3
    if pgrep modeltunnel > /dev/null; then
        print_success "Modeltunnel running"
        sudo cat ~/.config/modeltunnel/tunnel.url 2>/dev/null | head -1 || echo "No tunnel URL yet"
    else
        print_error "Modeltunnel failed to start"
    fi
else
    print_error "Binary not installed"
fi

echo ""
echo "Update complete! Data preserved in:"
echo "  - $CONFIG_DIR/config.yaml"
echo "  - $CONFIG_DIR/keys.db"
echo "  - $CONFIG_DIR/tunnel.url"
echo ""
echo "Services running? Use: sudo systemctl status ollama modeltunnel"