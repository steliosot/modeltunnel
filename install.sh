#!/bin/bash
#
# Modeltunnel Installation Wizard
#
# Prerequisites: Ollama must be installed first
#   curl -fsSL https://ollama.com/install.sh | bash
#
# One-line install:
#   curl -fsSL https://raw.githubusercontent.com/steliosot/modeltunnel/main/install.sh | bash
#
# With options:
#   curl -fsSL ... | bash -s -- --silent

set -euo pipefail

# Version
VERSION="1.0.0"

# Configuration
MODELTUNNEL_REPO="steliosot/modeltunnel"
DEFAULT_INSTALL_DIR="/usr/local/bin"
DEFAULT_MODELTUNNEL_PORT="8080"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Global variables
INSTALL_MODELTUNNEL=true
MODELTUNNEL_VERSION=""
OS=""
ARCH=""
INSTALL_DIR="$DEFAULT_INSTALL_DIR"
MODELTUNNEL_PORT="$DEFAULT_MODELTUNNEL_PORT"
RUN_AS_SERVICE=true
REQUIRES_SUDO=false
SILENT_MODE=false

# ============================================
# UTILITY FUNCTIONS
# ============================================

print_header() {
    clear
    echo -e "${CYAN}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║                                                              ║"
    echo "║           🚀 Modeltunnel Installation Wizard                 ║"
    echo "║                                                              ║"
    echo "║              v${VERSION} - Interactive Setup                 ║"
    echo "║                                                              ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    echo ""
}

print_step() {
    echo -e "${BLUE}➜${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC}  $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${CYAN}ℹ${NC}  $1"
}

ask_yes_no() {
    local prompt="$1"
    local default="$2"

    if [ "$SILENT_MODE" = true ]; then
        # In silent mode, return the default
        [ "$default" = "Y" ]
        return
    fi

    while true; do
        echo -e "${BLUE}❓${NC} $prompt" >&2
        echo -n "   (${default}) " >&2
        read -r response

        if [ -z "$response" ]; then
            response="$default"
        fi

        case $response in
            [Yy]*) return 0 ;;
            [Nn]*) return 1 ;;
            *) echo -e "${RED}Invalid input. Please enter Y or N.${NC}" >&2 ;;
        esac
    done
}

ask_input() {
    local prompt="$1"
    local default="$2"

    if [ "$SILENT_MODE" = true ]; then
        echo "$default"
        return
    fi

    while true; do
        echo -e "${BLUE}❓${NC} $prompt" >&2
        echo -e "   ${CYAN}Enter a value or press Enter to use ${default}.${NC}" >&2
        echo -n "   > " >&2
        read -r response

        if [ -z "$response" ]; then
            echo "$default"
            return
        else
            echo "$response"
            return
        fi
    done
}

check_sudo() {
    if [ "$EUID" -eq 0 ]; then
        REQUIRES_SUDO=false
    elif sudo -n true 2>/dev/null; then
        REQUIRES_SUDO=true
    else
        print_error "This installer requires sudo privileges"
        print_info "Please run with sudo or use a sudo-enabled user"
        exit 1
    fi
}

run_with_sudo() {
    if [ "$REQUIRES_SUDO" = true ]; then
        sudo "$@"
    else
        "$@"
    fi
}

# ============================================
# SYSTEM DETECTION
# ============================================

detect_os() {
    print_step "Detecting operating system..."

    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        OS="linux"
        if [ -f /etc/os-release ]; then
            . /etc/os-release
            OS_NAME="$NAME"
            OS_VERSION="$VERSION_ID"
        fi
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="darwin"
        OS_NAME="macOS"
        OS_VERSION=$(sw_vers -productVersion)
    else
        print_error "Unsupported operating system: $OSTYPE"
        exit 1
    fi

    print_success "Detected: $OS_NAME ${OS_VERSION:-}"
}

detect_arch() {
    print_step "Detecting architecture..."

    local machine=$(uname -m)
    case "$machine" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $machine"
            exit 1
            ;;
    esac

    print_success "Architecture: $ARCH"
}

check_dependencies() {
    print_step "Checking dependencies..."

    local missing_deps=()

    if ! command -v curl &> /dev/null && ! command -v wget &> /dev/null; then
        missing_deps+=("curl or wget")
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "Missing required dependencies: ${missing_deps[*]}"
        print_info "Please install them and run this script again."

        if [ "$OS" = "linux" ]; then
            print_info "On Ubuntu/Debian: sudo apt-get install -y ${missing_deps[*]}"
            print_info "On CentOS/RHEL: sudo yum install -y ${missing_deps[*]}"
        elif [ "$OS" = "darwin" ]; then
            print_info "On macOS: brew install ${missing_deps[*]}"
        fi

        exit 1
    fi

    print_success "All dependencies satisfied"
}

check_prerequisites() {
    print_step "Checking local backends (optional)..."
    if command -v ollama &> /dev/null; then
        print_success "Ollama detected (optional)"
    else
        print_info "Ollama not detected (optional). You can run it later."
    fi
}

# ============================================
# MODELTUNNEL INSTALLATION
# ============================================

handle_modeltunnel_installation() {
    echo ""
    echo -e "${BOLD}📦 Modeltunnel Installation${NC}"
    echo "═══════════════════════════════════════════════════════════"

    # Check if we should build from source or download binary
    local use_source=false

    if command -v go &> /dev/null; then
            if ask_yes_no "Go is installed. Build from source instead of downloading binary?" "N"; then
                use_source=true
            fi
    else
            print_warning "Go not found, will build from source"
            use_source=true
    fi

    if [ "$use_source" = true ]; then
        install_modeltunnel_from_source
    else
        install_modeltunnel_binary
    fi

    # Setup config file after installation
    setup_modeltunnel_config
}

configure_modeltunnel() {
    echo ""
    echo -e "${BOLD}Modeltunnel Configuration${NC}"
    echo "───────────────────────────────────────────────────────────"
    echo ""

    MODELTUNNEL_PORT=$(ask_input "Modeltunnel port" "$DEFAULT_MODELTUNNEL_PORT")

    # Service
    if [ "$OS" = "linux" ]; then
        if ask_yes_no "Run as a background service?" "Y"; then
            RUN_AS_SERVICE=true
        fi
    fi

    echo ""
    echo "Configuration:"
    echo "  Port: $MODELTUNNEL_PORT"
    echo "  Service: $([[ "$RUN_AS_SERVICE" = true ]] && echo "Enabled as service" || echo "Disabled")"

    echo ""
    if ask_yes_no "Is this correct?" "Y"; then
        return
    else
        configure_modeltunnel
    fi
}

setup_modeltunnel_config() {
    print_step "Setting up configuration..."
    local config_dir="$HOME/.config/modeltunnel"
    mkdir -p "$config_dir"

    if [ ! -f "$config_dir/config.yaml" ]; then
        print_info "Creating default configuration..."
        cat > "$config_dir/config.yaml" <<EOF
server:
  host: "127.0.0.1"
  port: 8080

upstreams:
  default:
    type: ollama
    base_url: "http://127.0.0.1:11434"
  vllm:
    type: vllm
    base_url: "http://127.0.0.1:8000"

policies:
  default:
    rate_limit: "60/min"
    max_tokens: 4096

intents:
  chat:
    priority:
      - phi
      - tinyllama:latest
      - mistral:latest
    description: General conversation
    temperature: 0.7
    max_tokens: 1000
  code:
    priority:
      - deepseek-coder:6.7b
      - mistral:latest
      - phi3:latest
    description: Programming
    temperature: 0.2
    max_tokens: 4096

keys: []
providers: []
EOF
    else
        print_info "Configuration file already exists"
    fi
}

install_modeltunnel_from_source() {
    print_step "Building Modeltunnel from source..."

    if ! command -v git &> /dev/null || ! command -v go &> /dev/null; then
        print_info "Git and/or Go not found, installing..."
        if [ "$OS" = "linux" ]; then
            run_with_sudo apt-get update -qq
            run_with_sudo apt-get install -y git golang
        elif [ "$OS" = "darwin" ]; then
            print_info "Please install Git and Go: brew install git go"
            exit 1
        fi
    fi

    local temp_dir="/tmp/modeltunnel-build"
    rm -rf "$temp_dir"
    mkdir -p "$temp_dir"

    print_info "Cloning repository..."
    git clone --depth 1 "https://github.com/$MODELTUNNEL_REPO.git" "$temp_dir" || {
        print_error "Failed to clone repository"
        exit 1
    }

    cd "$temp_dir"
    print_info "Building..."

    CGO_ENABLED=1 GOOS="$OS" GOARCH="$ARCH" \
        go build -ldflags="-s -w" -o modeltunnel ./cmd/modeltunnel/main.go || {
        print_error "Build failed"
        exit 1
    }

    run_with_sudo mv modeltunnel "$INSTALL_DIR/"
    rm -rf "$temp_dir"

    print_success "Modeltunnel built and installed!"
}

install_modeltunnel_binary() {
    local download_url="https://github.com/$MODELTUNNEL_REPO/releases/latest/download/modeltunnel-${OS}-${ARCH}"
    local temp_file="/tmp/modeltunnel-${OS}-${ARCH}"

    print_step "Downloading Modeltunnel..."
    print_info "Download size: ~15MB"

    if command -v curl &> /dev/null; then
        curl -fsSL --progress-bar "$download_url" -o "$temp_file" || {
            print_error "Failed to download Modeltunnel"
            print_info "Falling back to building from source..."
            install_modeltunnel_from_source
            return
        }
    else
        wget -q --show-progress "$download_url" -O "$temp_file" || {
            print_error "Failed to download Modeltunnel"
            print_info "Falling back to building from source..."
            install_modeltunnel_from_source
            return
        }
    fi

    print_success "Download complete"

    print_step "Installing Modeltunnel..."
    run_with_sudo chmod +x "$temp_file"
    run_with_sudo mv "$temp_file" "$INSTALL_DIR/modeltunnel"
    rm -f "$temp_file"
}

# ============================================
# SYSTEMD SERVICES (Linux)
# ============================================

setup_modeltunnel_service() {
    # Only for Linux
    if [ "$OS" != "linux" ]; then
        return
    fi

    if [ "$RUN_AS_SERVICE" != true ]; then
        return
    fi

    print_step "Setting up systemd service for Modeltunnel..."

    run_with_sudo tee /etc/systemd/system/modeltunnel.service > /dev/null <<EOF
[Unit]
Description=Modeltunnel API Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/modeltunnel up --host 0.0.0.0
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    run_with_sudo systemctl daemon-reload
    run_with_sudo systemctl enable modeltunnel > /dev/null 2>&1 || true

    print_success "Modeltunnel service created"
    print_info "Start with: sudo systemctl start modeltunnel"
}

# ============================================
# SUMMARY & COMPLETION
# ============================================

show_summary() {
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}                 ${BOLD}Installation Complete${NC}                   ${CYAN}║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo ""
    echo -e "${BOLD}📖 Quick Start${NC}"
    echo "─────────────────"
    echo "  1) Start server:"
    echo "     $INSTALL_DIR/modeltunnel up"
    echo ""
    echo "  2) Open dashboard:"
    echo "     http://127.0.0.1:$MODELTUNNEL_PORT/admin"
    echo ""
    echo "  3) Create API keys in the dashboard"
    echo ""
    echo "  4) Configure backend URL:"
    echo "     ~/.config/modeltunnel/config.yaml"
    echo ""
    echo "  Optional tunnel:"
    echo "     $INSTALL_DIR/modeltunnel up --tunnel"
    echo ""
    echo -e "${BOLD}📚 Documentation${NC}"
    echo "────────────────"
    echo "  API Reference: https://github.com/$MODELTUNNEL_REPO/tree/main/docs/api.md"
    echo "  Examples: https://github.com/$MODELTUNNEL_REPO/tree/main/docs/EXAMPLES.md"
    echo ""
}

# ============================================
# MAIN
# ============================================

main() {
    # Parse args first (allow --help to work without terminal)
    parse_args "$@"

    # Check if running in non-interactive mode (unless --silent is set)
    if [ "$SILENT_MODE" = false ] && [ ! -t 0 ]; then
        if [ -r /dev/tty ]; then
            # Reattach stdin to the terminal for interactive prompts
            exec </dev/tty
        else
            print_error "This installer requires an interactive terminal"
            print_info "Please run: curl -fsSL ... | bash"
            print_info "Or for non-interactive: curl -fsSL ... | bash -s -- --silent"
            exit 1
        fi
    fi

    if [ "$SILENT_MODE" = false ]; then
        print_header
    fi

    # System checks
    detect_os
    detect_arch
    check_dependencies
    check_sudo
    check_prerequisites

    if [ "$SILENT_MODE" = true ]; then
        # Silent mode - just install modeltunnel binary
        print_info "Installing Modeltunnel in silent mode..."
        print_info "Detected: $OS_NAME ${OS_VERSION:-} ($ARCH)"

        # Enable systemd services for persistent installation
        if [ "$OS" = "linux" ]; then
            RUN_AS_SERVICE=true
        fi

        # Build from source (no pre-built binaries available)
        install_modeltunnel_from_source

        # Setup systemd services for persistent installation
        if [ "$OS" = "linux" ] && [ "$RUN_AS_SERVICE" = true ]; then
            setup_modeltunnel_service
        fi

        show_summary
    else
        # Interactive mode
        configure_modeltunnel

        # Install Modeltunnel
        if [ "$INSTALL_MODELTUNNEL" = true ]; then
            handle_modeltunnel_installation
        fi

        # Show final summary
        show_summary
    fi
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --silent)
                SILENT_MODE=true
                shift
                ;;
            --help)
                echo "Modeltunnel Installer"
                echo ""
                echo "Usage:"
                echo "  curl -fsSL ... | bash"
                echo "  curl -fsSL ... | bash -s -- [options]"
                echo ""
                echo "Prerequisites:"
                echo "  Ollama must be installed first"
                echo ""
                echo "Options:"
                echo "  --silent         Non-interactive mode (installs Modeltunnel only)"
                echo "  --help           Show this help message"
                exit 0
                ;;
            *)
                shift
                ;;
        esac
    done
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --help)
                echo "Modeltunnel Installer"
                echo ""
                echo "Usage:"
                echo "  curl -fsSL ... | bash"
                echo "  curl -fsSL ... | bash -s -- [options]"
                echo ""
                echo "Prerequisites:"
                echo "  Ollama must be installed first: curl -fsSL https://ollama.com/install.sh | bash"
                echo ""
                echo "Options:"
                echo "  --silent         Non-interactive mode (installs Modeltunnel only)"
                echo "  --help           Show this help message"
                exit 0
                ;;
            --silent)
                SILENT_MODE=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done
}

# Run main function
main "$@"
