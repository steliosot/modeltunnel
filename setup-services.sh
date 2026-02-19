#!/bin/bash
#
# Setup systemd services for Ollama and Modeltunnel
# Creates persistent services that survive closing VM window
#

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

print_info() { echo -e "${CYAN}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if running on Linux
if [ "$(uname -s)" != "Linux" ]; then
    print_error "This script only works on Linux"
    exit 1
fi

# Check if systemd is available
if ! command -v systemctl &> /dev/null; then
    print_error "systemd not found. This script requires systemd."
    exit 1
fi

print_info "Setting up systemd services..."

# Create Ollama service
sudo tee /etc/systemd/system/ollama.service > /dev/null <<'EOF'
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

print_success "Created ollama.service"

# Create Modeltunnel service
sudo tee /etc/systemd/system/modeltunnel.service > /dev/null <<'EOF'
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

print_success "Created modeltunnel.service"

# Reload systemd daemon
print_info "Reloading systemd..."
sudo systemctl daemon-reload

# Enable services (start on boot)
print_info "Enabling services..."
sudo systemctl enable ollama > /dev/null 2>&1 || print_error "Failed to enable ollama"
sudo systemctl enable modeltunnel > /dev/null 2>&1 || print_error "Failed to enable modeltunnel"

# Start the services in order
print_info "Starting services..."
sudo systemctl restart ollama
sleep 3
sudo systemctl start modeltunnel

# Check status
if systemctl is-active ollama > /dev/null && systemctl is-active modeltunnel > /dev/null; then
    print_success "All services running!"
else
    print_error "Services failed to start. Check logs:"
    print_info "  sudo journalctl -u ollama -n 20"
    print_info "  sudo journalctl -u modeltunnel -n 20"
    exit 1
fi

# Show tunnel URL if available
sleep 5
if [ -f "$HOME/.config/modeltunnel/tunnel.url" ]; then
    print_success "Public tunnel URL: $(cat $HOME/.config/modeltunnel/tunnel.url)"
fi

print_info "Management commands:"
print_info "  sudo systemctl status ollama modeltunnel   # Check status"
print_info "  sudo systemctl restart modeltunnel            # Restart Modeltunnel"
print_info "  sudo journalctl -u modeltunnel -f              # View logs"
print_info "  sudo systemctl stop modeltunnel               # Stop service"
print_info "  sudo systemctl start modeltunnel              # Start service"