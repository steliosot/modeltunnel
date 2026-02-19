#!/bin/bash
#
# Modeltunnel & Ollama Interactive Installation Wizard
# 
# One-line install:
#   curl -fsSL https://raw.githubusercontent.com/steliosot/modeltunnel/main/install.sh | bash
#
# With options:
#   curl -fsSL ... | bash -s -- --silent --with-ollama --models=llama3.2,phi

set -euo pipefail

# Version
VERSION="1.0.0"

# Configuration
OLLAMA_REPO="ollama/ollama"
MODELTUNNEL_REPO="steliosot/modeltunnel"
DEFAULT_INSTALL_DIR="/usr/local/bin"
DEFAULT_OLLAMA_PORT="11434"
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
INSTALL_OLLAMA=false
INSTALL_MODELTUNNEL=true
OLLAMA_VERSION=""
MODELTUNNEL_VERSION=""
OS=""
ARCH=""
INSTALL_DIR="$DEFAULT_INSTALL_DIR"
OLLAMA_PORT="$DEFAULT_OLLAMA_PORT"
MODELTUNNEL_PORT="$DEFAULT_MODELTUNNEL_PORT"
SELECTED_MODELS=()
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

spinner() {
    local pid=$1
    local delay=0.1
    local spinstr='|/-\'
    while [ -d /proc/$pid ]; do
        local temp=${spinstr#?}
        printf " [%c]  " "$spinstr"
        local spinstr=$temp${spinstr%"$temp"}
        sleep $delay
        printf "\b\b\b\b\b\b"
    done
    printf "    \b\b\b\b"
}

ask_yes_no() {
    local prompt="$1"
    local default="${2:-Y}"
    local response
    
    if [ "$default" = "Y" ]; then
        read -p "$prompt [Y/n]: " response
    else
        read -p "$prompt [y/N]: " response
    fi
    
    response=${response:-$default}
    
    case "$response" in
        [Yy]* ) return 0 ;;
        [Nn]* ) return 1 ;;
        * ) 
            if [ "$default" = "Y" ]; then
                return 0
            else
                return 1
            fi
            ;;
    esac
}

ask_input() {
    local prompt="$1"
    local default="$2"
    local response
    
    if [ -n "$default" ]; then
        read -p "$prompt [$default]: " response
        echo "${response:-$default}"
    else
        read -p "$prompt: " response
        echo "$response"
    fi
}

check_sudo() {
    if [ "$EUID" -eq 0 ]; then
        REQUIRES_SUDO=false
        return 0
    fi
    
    if sudo -n true 2>/dev/null; then
        REQUIRES_SUDO=true
        return 0
    fi
    
    echo ""
    print_warning "This installer requires sudo privileges for some operations."
    
    if sudo -v; then
        REQUIRES_SUDO=true
        return 0
    else
        print_error "Sudo access required but not granted."
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

    if [ "$OS" = "linux" ]; then
        if ! command -v zstd &> /dev/null; then
            missing_deps+=("zstd")
        fi
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "Missing required dependencies: ${missing_deps[*]}"
        print_info "Please install them and run this script again."

        if [ "$OS" = "linux" ]; then
            print_info "On Ubuntu/Debian: sudo apt-get install -y ${missing_deps[*]}"
            print_info "On CentOS/RHEL: sudo dnf install -y ${missing_deps[*]}"
        elif [ "$OS" = "darwin" ]; then
            print_info "On macOS: brew install ${missing_deps[*]}"
        fi

        exit 1
    fi

    print_success "All dependencies satisfied"
}

# ============================================
# OLLAMA DETECTION & VERSION
# ============================================

check_ollama_installed() {
    if command -v ollama &> /dev/null; then
        OLLAMA_VERSION=$(ollama --version 2>&1 | head -1 | grep -oP '\d+\.\d+\.\d+' || echo "unknown")
        return 0
    else
        return 1
    fi
}

get_latest_ollama_version() {
    local latest=$(curl -s "https://api.github.com/repos/$OLLAMA_REPO/releases/latest" | grep '"tag_name"' | sed 's/.*: "\([^"]*\)".*/\1/' || echo "")
    if [ -z "$latest" ]; then
        print_warning "Could not fetch latest Ollama version, using default"
        echo "v0.16.2"
    else
        echo "$latest"
    fi
}

# ============================================
# MAIN INSTALLATION MENU
# ============================================

show_main_menu() {
    echo ""
    echo -e "${BOLD}What would you like to install?${NC}"
    echo ""
    echo "  1) Install both Ollama + Modeltunnel (recommended)"
    echo "  2) Install Modeltunnel only (I already have Ollama)"
    echo "  3) Install Ollama only"
    echo "  4) Exit"
    echo ""
    
    local choice=$(ask_input "Enter your choice" "1")
    
    case "$choice" in
        1)
            INSTALL_OLLAMA=true
            INSTALL_MODELTUNNEL=true
            ;;
        2)
            INSTALL_OLLAMA=false
            INSTALL_MODELTUNNEL=true
            ;;
        3)
            INSTALL_OLLAMA=true
            INSTALL_MODELTUNNEL=false
            ;;
        4)
            echo ""
            print_info "Installation cancelled by user."
            exit 0
            ;;
        *)
            print_error "Invalid choice. Please try again."
            show_main_menu
            ;;
    esac
}

# ============================================
# OLLAMA INSTALLATION
# ============================================

handle_ollama_installation() {
    echo ""
    echo -e "${BOLD}🦙 Ollama Installation${NC}"
    echo "═══════════════════════════════════════════════════════════"
    
    if check_ollama_installed; then
        print_info "Ollama is already installed (version: $OLLAMA_VERSION)"
        echo ""
        echo "What would you like to do?"
        echo "  1) Keep current version"
        echo "  2) Upgrade to latest version"
        echo "  3) Reinstall (fresh install)"
        echo "  4) Skip Ollama installation"
        echo ""
        
        local choice=$(ask_input "Enter your choice" "1")
        
        case "$choice" in
            1)
                print_info "Keeping Ollama $OLLAMA_VERSION"
                INSTALL_OLLAMA=false
                return
                ;;
            2)
                print_info "Will upgrade Ollama"
                ;;
            3)
                print_info "Will reinstall Ollama"
                ;;
            4)
                INSTALL_OLLAMA=false
                return
                ;;
            *)
                print_error "Invalid choice"
                handle_ollama_installation
                return
                ;;
        esac
    else
        echo ""
        print_info "Ollama is not installed on your system."
        
        if ! ask_yes_no "Would you like to install Ollama?"; then
            print_warning "Skipping Ollama installation"
            INSTALL_OLLAMA=false
            return
        fi
    fi
    
    # Get latest version
    local latest_version=$(get_latest_ollama_version)
    print_info "Latest Ollama version: $latest_version"
    
    # Configuration
    configure_ollama
    
    # Install
    install_ollama
}

configure_ollama() {
    echo ""
    echo -e "${BOLD}Ollama Configuration${NC}"
    echo "───────────────────────────────────────────────────────────"
    
    # Installation directory
    INSTALL_DIR=$(ask_input "Installation directory" "$DEFAULT_INSTALL_DIR")
    
    # Port
    OLLAMA_PORT=$(ask_input "Ollama port" "$DEFAULT_OLLAMA_PORT")
    
    # Service
    if [ "$OS" = "linux" ]; then
        if ask_yes_no "Run Ollama as a background service?" "Y"; then
            RUN_AS_SERVICE=true
        else
            RUN_AS_SERVICE=false
        fi
    fi
    
    echo ""
    print_info "Configuration summary:"
    echo "  Installation directory: $INSTALL_DIR"
    echo "  Port: $OLLAMA_PORT"
    if [ "$OS" = "linux" ]; then
        echo "  Background service: $RUN_AS_SERVICE"
    fi
    
    if ! ask_yes_no "Is this correct?"; then
        configure_ollama
    fi
}

install_ollama() {
    local temp_dir="/tmp/ollama-install-$$"

    echo ""
    print_step "Installing Ollama..."
    print_info "This may take a few minutes depending on your connection."
    print_info "Download size: ~450MB"

    mkdir -p "$temp_dir"

    # Download Ollama install script which handles all formats
    OLLAMA_INSTALL_URL="https://ollama.com/install.sh"
    if ! curl -fsSL "$OLLAMA_INSTALL_URL" -o "$temp_dir/ollama-install.sh"; then
        print_error "Failed to download Ollama install script"
        rm -rf "$temp_dir"
        return 1
    fi

    chmod +x "$temp_dir/ollama-install.sh"

    # Run Ollama install
    print_info "Running Ollama installation script..."
    if bash "$temp_dir/ollama-install.sh"; then
        print_success "Ollama installed successfully"
    else
        print_error "Ollama installation failed"
        rm -rf "$temp_dir"
        return 1
    fi

    rm -rf "$temp_dir"

    # Setup directories
    print_step "Setting up directories..."
    mkdir -p "$HOME/.ollama"

    # Setup systemd service (Linux only)
    if [ "$OS" = "linux" ] && [ "$RUN_AS_SERVICE" = true ]; then
        setup_ollama_service
    fi
    
    print_success "Ollama installed successfully!"
}

setup_ollama_service() {
    print_step "Creating systemd service..."
    
    local service_file="/etc/systemd/system/ollama.service"
    
    run_with_sudo tee "$service_file" > /dev/null <<EOF
[Unit]
Description=Ollama Service
After=network-online.target

[Service]
ExecStart=$INSTALL_DIR/ollama serve
User=$USER
Group=$USER
Restart=always
RestartSec=3
Environment="OLLAMA_HOST=0.0.0.0:$OLLAMA_PORT"
Environment="HOME=$HOME"

[Install]
WantedBy=default.target
EOF
    
    run_with_sudo systemctl daemon-reload
    run_with_sudo systemctl enable ollama
    run_with_sudo systemctl start ollama
    
    print_success "Systemd service created and started"
}

setup_modeltunnel_service() {
    print_step "Creating systemd services..."
    
    # Install Ollama service
    setup_ollama_service
    
    # Install Modeltunnel service
    local modeltunnel_service_file="/etc/systemd/system/modeltunnel.service"
    
    run_with_sudo tee "$modeltunnel_service_file" > /dev/null <<EOF
[Unit]
Description=Modeltunnel API Server
After=network-online.target ollama.service
Wants=ollama.service

[Service]
Type=simple
ExecStart=$INSTALL_DIR/modeltunnel up --ollama --tunnel
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
    
    run_with_sudo systemctl daemon-reload
    
    # Enable both services (start on boot)
    run_with_sudo systemctl enable modeltunnel
    run_with_sudo systemctl enable ollama
    
    # Order: start ollama first, then modeltunnel
    print_info "Starting services..."
    run_with_sudo systemctl restart ollama
    sleep 3
    run_with_sudo systemctl start modeltunnel
    
    print_success "Systemd services created and started"
    print_info "Services: sudo systemctl status ollama modeltunnel"
    
    # Save tunnel URL from modeltunnel
    print_info "Waiting for tunnel connection..."
    sleep 5
    
    if [ -f "$HOME/.config/modeltunnel/tunnel.url" ]; then
        print_info "Public URL: $(cat $HOME/.config/modeltunnel/tunnel.url)"
    fi
}

# ============================================
# MODELTUNNEL INSTALLATION
# ============================================

handle_modeltunnel_installation() {
    echo ""
    echo -e "${BOLD}🚀 Modeltunnel Installation${NC}"
    echo "═══════════════════════════════════════════════════════════"
    
    if ! check_ollama_installed && [ "$INSTALL_OLLAMA" = false ]; then
        print_warning "Ollama is not installed. Modeltunnel requires Ollama to function."
        if ask_yes_no "Would you like to install Ollama first?"; then
            INSTALL_OLLAMA=true
            handle_ollama_installation
        else
            print_error "Cannot install Modeltunnel without Ollama"
            exit 1
        fi
    fi
    
    configure_modeltunnel
    install_modeltunnel
    
    # Setup systemd service for Modeltunnel and Ollama (Linux only)
    if [ "$OS" = "linux" ] && [ "$RUN_AS_SERVICE" = true ]; then
        setup_modeltunnel_service
    fi
}

configure_modeltunnel() {
    echo ""
    echo -e "${BOLD}Modeltunnel Configuration${NC}"
    echo "───────────────────────────────────────────────────────────"
    
    MODELTUNNEL_PORT=$(ask_input "Modeltunnel port" "$DEFAULT_MODELTUNNEL_PORT")
    
    echo ""
    print_info "Configuration summary:"
    echo "  Port: $MODELTUNNEL_PORT"
    
    if ! ask_yes_no "Is this correct?"; then
        configure_modeltunnel
    fi
}

setup_modeltunnel_config() {
    # Detect environment for binding configuration
    local env_type="local"
    local bind_host="127.0.0.1"
    
    # Check for Docker
    if [[ -f /.dockerenv ]]; then
        env_type="docker"
        bind_host="0.0.0.0"
    # Check for WSL2
    elif [[ -f /proc/version ]] && grep -q Microsoft /proc/version; then
        env_type="wsl2"
        bind_host="0.0.0.0"
    # Check for Cloud VM
    elif [[ -f /sys/hypervisor/uuid ]] || [[ -f /etc/os-release ]] && (grep -q "ID=google\|ID=ubuntu-cloud" /etc/os-release 2>/dev/null); then
        env_type="cloud"
        bind_host="0.0.0.0"
    # Check for VirtualBox
    elif [[ -f /sys/class/dmi/id/product_name ]] && grep -qi "virtualbox" /sys/class/dmi/id/product_name 2>/dev/null; then
        env_type="virtualbox"
        bind_host="0.0.0.0"
    # Check for VMware
    elif grep -qi "vmware" /proc/cpuinfo 2>/dev/null; then
        env_type="vmware"
        bind_host="0.0.0.0"
    fi
    
    local config_dir="$HOME/.config/modeltunnel"
    local config_file="$config_dir/config.yaml"
    
    mkdir -p "$config_dir"
    
    cat > "$config_file" << CONFIG_EOF
server:
  host: ${bind_host}
  port: ${MODELTUNNEL_PORT}

policies:
  default:
    rate_limit: 60/min
    max_tokens: 4096

intents:
  chat:
    priority:
      - phi
      - mistral:latest
    description: General chat, Q&A, support
    temperature: 0.7
    max_tokens: 1000
  code:
    priority:
      - deepseek-coder:6.7b
      - mistral:latest
    description: Programming, debugging, technical
    temperature: 0.2
    max_tokens: 2000
  plan:
    priority:
      - deepseek-r1:latest
      - qemu:latest
    description: Planning, strategy, reasoning
    temperature: 0.3
    max_tokens: 4000
CONFIG_EOF
    
    print_info "Configuration created at: $config_file"
    if [[ "$bind_host" == "0.0.0.0" ]]; then
        echo ""
        print_info "Running in ${env_type} environment: server will be accessible from external machines"
        echo ""
        echo "  After starting, access via:"
        echo "  • Dashboard: http://YOUR_VM_IP:${MODELTUNNEL_PORT}/admin"
        echo "  • API:      http://YOUR_VM_IP:${MODELTUNNEL_PORT}/v1"
    fi
}

install_modeltunnel() {
    print_step "Installing Modeltunnel..."
    
    # Check if we should build from source or download binary
    local use_source=false
    
    if command -v go &> /dev/null; then
            if ask_yes_no "Go is installed. Build from source instead of downloading binary?" "N"; then
            use_source=true
        fi
    fi
    
    if [ "$use_source" = true ]; then
            install_modeltunnel_from_source
    else
            install_modeltunnel_binary
    fi
    
    # Setup config file after installation
    setup_modeltunnel_config
}

install_modeltunnel_from_source() {
    print_step "Building Modeltunnel from source..."

    if ! command -v git &> /dev/null || ! command -v go &> /dev/null; then
        print_info "Git and/or Go not found, installing..."
        if [ "$OS" = "linux" ]; then
            if command -v apt-get &> /dev/null; then
                run_with_sudo apt-get update -qq
                run_with_sudo apt-get install -y git golang
            elif command -v yum &> /dev/null; then
                run_with_sudo yum install -y git golang
            else
                print_error "Please install Git and Go first"
                exit 1
            fi
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
    go build -o modeltunnel ./cmd/modeltunnel/main.go || {
        print_error "Build failed"
        ls -la
        pwd
        exit 1
    }

    run_with_sudo mv modeltunnel "$INSTALL_DIR/"

    cd - > /dev/null
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
    
    run_with_sudo chmod +x "$temp_file"
    run_with_sudo mv "$temp_file" "$INSTALL_DIR/modeltunnel"
    
    print_success "Modeltunnel installed!"
}

# ============================================
# MODEL INSTALLATION
# ============================================

show_disk_space_warning() {
    echo ""
    echo -e "${YELLOW}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${YELLOW}║  💾 IMPORTANT: Disk Space Information                       ║${NC}"
    echo -e "${YELLOW}╠══════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${YELLOW}║                                                              ║${NC}"
    echo -e "${YELLOW}║  Models require significant disk space:                     ║${NC}"
    echo -e "${YELLOW}║                                                              ║${NC}"
    echo -e "${YELLOW}║  • Small models (3B-7B):  ~2-4 GB each                      ║${NC}"
    echo -e "${YELLOW}║  • Medium models (13B):   ~8-10 GB each                     ║${NC}"
    echo -e "${YELLOW}║  • Large models (70B+):   ~40+ GB each                      ║${NC}"
    echo -e "${YELLOW}║                                                              ║${NC}"
    echo -e "${YELLOW}║  💡 Tip: Start with smaller models for testing              ║${NC}"
    echo -e "${YELLOW}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    # Show available disk space
    local available=$(df -h "$HOME" | awk 'NR==2 {print $4}')
    print_info "Available disk space: $available"
}

install_models_menu() {
    echo ""
    echo -e "${BOLD}📦 Model Installation${NC}"
    echo "═══════════════════════════════════════════════════════════"
    
    show_disk_space_warning
    
    if ! ask_yes_no "Would you like to install AI models now?"; then
        print_info "Skipping model installation"
        return
    fi
    
    echo ""
    echo -e "${BOLD}Select models to install:${NC}"
    echo ""
    echo "  [✓] = Recommended (pre-selected)"
    echo ""
    
    declare -A models
    models["llama3.2:3b"]="General purpose, fast, 3B parameters (~2GB)"
    models["phi:latest"]="Small & efficient, 2.7B parameters (~1.6GB) ✓"
    models["mistral:7b"]="Balanced performance, 7B parameters (~4GB)"
    models["codellama:7b"]="Code generation, 7B parameters (~4GB)"
    models["deepseek-coder:6.7b"]="Advanced coding, 6.7B parameters (~4GB)"
    models["llava:7b"]="Vision capabilities, 7B parameters (~4GB)"
    models["deepseek-r1:8b"]="Reasoning & planning, 8B parameters (~5GB) ✓"
    models["tinyllama:latest"]="Very small, 1.1B parameters (~600MB)"
    
    local selected=()
    
    # Pre-select recommended models
    selected+=("phi:latest")
    selected+=("deepseek-r1:8b")
    
    for model in "${!models[@]}"; do
        local desc="${models[$model]}"
        local marker="[ ]"
        
        if [[ " ${selected[*]} " =~ " ${model} " ]]; then
            marker="[✓]"
        fi
        
        printf "  %s %-30s %s\n" "$marker" "$model" "$desc"
    done
    
    echo ""
    echo "Enter model names to install (comma-separated, or 'all' for all models)"
    echo "Pre-selected: phi:latest, deepseek-r1:8b"
    echo ""
    
    local input=$(ask_input "Models to install" "phi:latest,deepseek-r1:8b")
    
    if [ "$input" = "all" ]; then
        SELECTED_MODELS=("${!models[@]}")
    else
        IFS=',' read -ra SELECTED_MODELS <<< "$input"
    fi
    
    # Calculate total size
    local total_size=0
    for model in "${SELECTED_MODELS[@]}"; do
        case "$model" in
            "llama3.2:3b") total_size=$((total_size + 2)) ;;
            "phi:latest") total_size=$((total_size + 2)) ;;
            "mistral:7b") total_size=$((total_size + 4)) ;;
            "codellama:7b") total_size=$((total_size + 4)) ;;
            "deepseek-coder:6.7b") total_size=$((total_size + 4)) ;;
            "llava:7b") total_size=$((total_size + 4)) ;;
            "deepseek-r1:8b") total_size=$((total_size + 5)) ;;
            "tinyllama:latest") total_size=$((total_size + 1)) ;;
        esac
    done
    
    echo ""
    print_info "You selected ${#SELECTED_MODELS[@]} model(s)"
    print_info "Estimated download size: ~${total_size}GB"
    
    if ask_yes_no "Proceed with installation?"; then
        install_selected_models
    else
        print_info "Skipping model installation"
    fi
}

install_selected_models() {
    echo ""
    print_step "Installing selected models..."
    echo ""
    
    for model in "${SELECTED_MODELS[@]}"; do
        model=$(echo "$model" | xargs) # trim whitespace
        print_info "Pulling $model..."
        
        if ollama pull "$model" 2>&1; then
            print_success "✓ $model installed"
        else
            print_error "✗ Failed to install $model"
        fi
        echo ""
    done
    
    print_success "Model installation complete!"
}

# ============================================
# SUMMARY & COMPLETION
# ============================================

show_summary() {
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║           ✅ Installation Complete!                          ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    if [ "$INSTALL_OLLAMA" = true ] || check_ollama_installed; then
        echo -e "${BOLD}🦙 Ollama${NC}"
        echo "  Version: $(ollama --version 2>&1 | head -1)"
        echo "  Port: $OLLAMA_PORT"
        if [ "$OS" = "linux" ] && [ "$RUN_AS_SERVICE" = true ]; then
            echo "  Service: sudo systemctl status ollama"
        fi
        echo ""
        
        echo -e "${BOLD}Quick Commands:${NC}"
        echo "  ollama list              # List installed models"
        echo "  ollama run llama3.2:3b   # Run a model"
        echo "  ollama serve             # Start Ollama server"
        echo ""
    fi
    
    if [ "$INSTALL_MODELTUNNEL" = true ]; then
        echo -e "${BOLD}🚀 Modeltunnel${NC}"
        echo "  Port: $MODELTUNNEL_PORT"
        echo "  Dashboard: http://localhost:$MODELTUNNEL_PORT/admin"
        echo ""
        
        echo -e "${BOLD}Quick Commands:${NC}"
        echo "  modeltunnel up --ollama              # Start with Ollama"
        echo "  modeltunnel key create mykey         # Create API key"
        echo "  modeltunnel --help                   # Show all commands"
        echo ""
        
        echo -e "${BOLD}Next Steps:${NC}"
        echo "  1. Start Modeltunnel: modeltunnel up --ollama"
        echo "  2. Open dashboard: http://localhost:$MODELTUNNEL_PORT/admin"
        echo "  3. Create an API key and start using your models!"
    fi
    
    echo ""
    echo -e "${CYAN}Happy prompting! 🎉${NC}"
    echo ""
}

# ============================================
# MAIN
# ============================================

main() {
    # Check if running in non-interactive mode (unless --silent is set)
    if [ "$SILENT_MODE" = false ] && [ ! -t 0 ]; then
        print_error "This installer requires an interactive terminal"
        print_info "Please run: curl -fsSL ... | bash"
        print_info "Or for non-interactive: curl -fsSL ... | bash -s -- --silent"
        exit 1
    fi
    
    if [ "$SILENT_MODE" = false ]; then
        print_header
    fi
    
    # System checks
    detect_os
    detect_arch
    check_dependencies
    check_sudo
    
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

        # Install Ollama (required for model support)
        if ! command -v ollama &> /dev/null; then
            print_info "Installing Ollama for local model support..."
            if install_ollama ""; then
                print_success "Ollama installed!"
            else
                print_warning "Ollama installation skipped or failed"
                print_info "You can install Ollama manually later with:"
                print_info "  curl -fsSL https://ollama.com/install.sh | bash"
                return 0
            fi
        fi

        # Setup systemd services for persistent installation
        if [ "$OS" = "linux" ] && [ "$RUN_AS_SERVICE" = true ]; then
            setup_modeltunnel_service
        fi

        print_success "Installation complete!"
        print_info "Run 'modeltunnel --help' to get started"
        print_info "Dashboard: http://localhost:8080/admin"
        print_info "Dashboard: http://localhost:8080/admin"
    else
        # Interactive mode
        # Show main menu
        show_main_menu
        
        # Install Ollama if requested
        if [ "$INSTALL_OLLAMA" = true ]; then
            handle_ollama_installation
        fi
        
        # Install Modeltunnel if requested
        if [ "$INSTALL_MODELTUNNEL" = true ]; then
            handle_modeltunnel_installation
        fi
        
        # Install models
        if [ "$INSTALL_OLLAMA" = true ] || check_ollama_installed; then
            install_models_menu
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
            --with-ollama)
                INSTALL_OLLAMA=true
                shift
                ;;
            --install-dir)
                INSTALL_DIR="$2"
                shift 2
                ;;
            --help)
                echo "Modeltunnel Installer"
                echo ""
                echo "Usage:"
                echo "  curl -fsSL ... | bash"
                echo "  curl -fsSL ... | bash -s -- [options]"
                echo ""
                echo "Options:"
                echo "  --silent         Non-interactive mode (installs modeltunnel only)"
                echo "  --with-ollama    Also install Ollama"
                echo "  --install-dir    Installation directory (default: /usr/local/bin)"
                echo "  --help           Show this help message"
                exit 0
                ;;
            *)
                shift
                ;;
        esac
    done
}

# Run main function
parse_args "$@"
main "$@"