# Installation Guide

This guide covers all installation methods for Modeltunnel.

## Requirements

- **Go 1.21+** (for building from source)
- **Ollama** (for local LLM support)
- **macOS**, **Linux**, or **Windows** (WSL recommended)

---

## Quick Install

### macOS/Linux (Homebrew)

```bash
# Add tap
brew tap steliosot/modeltunnel

# Install
brew install modeltunnel

# Verify
modeltunnel version
```

### Using Go

```bash
go install github.com/steliosot/modeltunnel/cmd/modeltunnel@latest

# Ensure $GOPATH/bin is in your PATH
modeltunnel version
```

---

## From Source

### Prerequisites

Install Go 1.21 or later:

**macOS:**
```bash
brew install go
```

**Ubuntu/Debian:**
```bash
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

**Verify:**
```bash
go version
```

### Build

```bash
# Clone repository
git clone https://github.com/steliosot/modeltunnel.git
cd modeltunnel

# Build
make build

# Install to /usr/local/bin (optional)
sudo make install

# Verify
modeltunnel version
```

---

## Pre-built Binaries

Download from [GitHub Releases](https://github.com/steliosot/modeltunnel/releases):

```bash
# macOS (Intel)
curl -L -o modeltunnel.tar.gz https://github.com/steliosot/modeltunnel/releases/download/v1.0.0/modeltunnel-darwin-amd64.tar.gz

# macOS (Apple Silicon)
curl -L -o modeltunnel.tar.gz https://github.com/steliosot/modeltunnel/releases/download/v1.0.0/modeltunnel-darwin-arm64.tar.gz

# Linux
curl -L -o modeltunnel.tar.gz https://github.com/steliosot/modeltunnel/releases/download/v1.0.0/modeltunnel-linux-amd64.tar.gz

# Extract
tar -xzf modeltunnel.tar.gz
sudo mv modeltunnel /usr/local/bin/

# Verify
modeltunnel version
```

---

## Systemd Services (Persistent Installation)

### Problem: Closing VM Window Kills the Process

Running `modeltunnel up --ollama --tunnel &` and closing the VM window kills the process because the terminal sends a SIGHUP signal.

### Solution: Systemd Services

Services survive closing windows, session termination, and restart on failure:

```bash
# Install with systemd service enabled
curl -fsSL https://raw.githubusercontent.com/steliosot/modeltunnel/main/install.sh | bash -

# Services created automatically
sudo systemctl status ollama      # Check Ollama service
sudo systemctl status modeltunnel  # Check Modeltunnel service
```

### Features

- ✓ Auto-restart on failure (3-10 seconds)
- ✓ Survives closing VM window
- ✓ Auto-start on system boot
- ✓ Logs available: `journalctl -u modeltunnel -f`

### Managing Services

```bash
# Check status
sudo systemctl status ollama modeltunnel

# Restart services
sudo systemctl restart modeltunnel

# View logs
sudo journalctl -u modeltunnel -f

# Stop/Start
sudo systemctl stop modeltunnel
sudo systemctl start modeltunnel

# Enable/disable auto-start
sudo systemctl enable modeltunnel  # Enable
sudo systemctl disable modeltunnel # Disable
```

### Service Files

Both services are created by the installer at:
- `/etc/systemd/system/ollama.service` - Ollama LLM server
- `/etc/systemd/system/modeltunnel.service` - Modeltunnel API + tunnel

Modeltunnel service depends on Ollama and both auto-restart on failure.

---

## Docker

### Using Pre-Built Image

```bash
# Pull latest image from GitHub Container Registry
docker pull ghcr.io/steliosot/modeltunnel:latest

# Run with Ollama (local only)
docker run -d -p 8080:8080 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  ghcr.io/steliosot/modeltunnel:latest up --ollama --model mistral

# Run with public tunnel
docker run -d -p 8080:8080 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  ghcr.io/steliosot/modeltunnel:latest up --ollama --model mistral --tunnel

# Custom config (mount your config.yaml)
docker run -d -p 8080:8080 \
  -v /path/to/config.yaml:/home/appuser/.config/modeltunnel/config.yaml \
  ghcr.io/steliosot/modeltunnel:latest up
```

### Building from Source

```bash
# Clone and build
git clone https://github.com/steliosot/modeltunnel.git
cd modeltunnel
docker build -t modeltunnel:latest .

# Run
docker run -d -p 8080:8080 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  modeltunnel:latest up --ollama --model mistral
```

Docker image size: ~45MB

**Environment Variables:**
- `OLLAMA_BASE_URL` - Ollama server URL (default: `http://127.0.0.1:11434`)

**Volumes:**
- `/home/appuser/.config/modeltunnel/config.yaml` - Custom configuration
- `/home/appuser/.config/modeltunnel/keys.db` - SQLite database for keys

### Using Docker Compose (Recommended)

Docker Compose provides Modeltunnel with an integrated Ollama backend and persistent model storage.

**Quick Start:**

```bash
# Clone repository
git clone https://github.com/steliosot/modeltunnel.git
cd modeltunnel

# Start both services
docker-compose up -d
```

This starts:
- **Ollama** on port 11434 (for model storage and execution)
- **Modeltunnel** on port 8080 (API and dashboard)

**Access Dashboard and Pull Models:**

```bash
# Open dashboard
open http://localhost:8080/admin

# Pull models via web UI or API
# Dashboard shows:
# - Pull input field for custom models
# - Recommended models by category
# - Real-time pull progress
# - Model badges (intents, rate limits, tokens)

# Example: Pull deepseek-r1 from dashboard
# Then test API:
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $(docker exec modeltunnel-app ./modeltunnel key list --format json | jq -r '.[0].key')" \
  -H "Content-Type: application/json" \
  -d '{"model": "ollama/deepseek-r1:latest", "messages": [{"role": "user", "content": "Hello!"}]}'
```

**Pull Models from Command Line:**

```bash
# Pull models via Ollama container
docker exec -it modeltunnel-ollama ollama pull deepseek-r1
docker exec -it modeltunnel-ollama ollama pull deepseek-coder:6.7b
docker exec -it modeltunnel-ollama ollama pull phi
```

**Volumes and Persistence:**

```yaml
# docker-compose.yml includes:
volumes:
  ollama_data:    # Model storage (persists models)
  config_data:    # Config and keys database
```

Models persist across container restarts - no re-download needed.

**Stop and Clean Up:**

```bash
# Stop containers
docker-compose down

# Stop and remove volumes (deletes models)
docker-compose down -v

# View logs
docker-compose logs -f modeltunnel
docker-compose logs -f ollama
```

**Custom Configuration:**

```bash
# Mount custom config
docker-compose -f docker-compose.yml \
  -f custom-compose.override.yml up -d

# Or edit docker-compose.yml directly
```

---

## Verify Installation

```bash
# Check version
modeltunnel version

# Initialize config
modeltunnel init

# Test server (requires Ollama running)
modeltunnel up --ollama --model mistral
```

---

## Ollama Setup

Modeltunnel requires Ollama to be running locally.

### Install Ollama

**macOS:**
```bash
brew install ollama
```

**Linux:**
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

**Windows:**
Download from [ollama.com](https://ollama.com)

### Start Ollama

```bash
# Start service
ollama serve

# Pull a model
ollama pull mistral
ollama pull phi
ollama pull deepseek-r1

# Test
ollama run mistral
```

### Verify Ollama

```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags
```

---

## Post-Installation

### 1. Initialize Configuration

```bash
modeltunnel init
```

Creates:
- `~/.config/modeltunnel/config.yaml`
- `~/.config/modeltunnel/keys.db`

### 2. Start Server

```bash
# Basic
modeltunnel up --ollama --model mistral

# With public tunnel
modeltunnel up --ollama --model mistral --tunnel
```

### 3. Create API Key

```bash
modeltunnel key create mykey
```

### 4. Test API

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer YOUR_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/mistral:latest",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

---

## Web Dashboard Model Management

After installation, Modeltunnel provides a web dashboard for pulling models directly from Ollama without using the command line.

### Pulling Models from Dashboard

1. **Access Dashboard**
   ```
   http://localhost:8080/admin
   ```

2. **Navigate to Models Section**
   Click on the "Models" tab in the navigation menu

3. **Pull New Models**
   
   **Option 1: Pull from Input Field**
   - Fill in the pull input field with the model name (e.g., `llama3.2:3b`)
   - Click the "Pull" button
   - Watch the progress indicator
   - Model appears in "Installed Models" list when complete

   **Option 2: Pull from Recommended Models**
   - Browse the "Recommended Models" section with categories:
     - **General Purpose** - `llama3.2:3b`, `mistral:7b`, `phi4`
     - **Coding** - `codellama:7b`, `deepseek-coder:6.7b`
     - **Vision** - `llava:7b`, `bakllava:7b`
     - **Reasoning** - `deepseek-r1:8b`
   - Click "Pull" to download

4. **View Model Badges**
   After installation, each model displays configuration badges:
   - **Intents** - Yellow badges showing which intents use the model (e.g., "plan", "code", "chat")
   - **Rate Limits** - Blue badges with custom limits (e.g., "5/min", "100/min")
   - **Max Tokens** - Token limits (e.g., "2048 tokens", "4096 tokens")

### Remove Models from Dashboard

In the "Installed Models" list:
- Click the "Remove" button next to any model
- Confirm the deletion when prompted
- Model is permanently removed from your Ollama instance

### Pull Progress Feedback

While pulling models:
- Button shows real-time progress: `45%`, `85%`, etc.
- Visual spinner animation during download
- Checkmark ✓ appears when complete
- Error messages display if pull fails

Note: Pull progress tracks download of model files; larger models take longer to download.

---

## Troubleshooting

### Command not found

```bash
# Check if in PATH
which modeltunnel

# If not found, add to PATH
export PATH=$PATH:/usr/local/bin

# Or use full path
./modeltunnel version
```

### Permission denied

```bash
# Fix permissions
chmod +x /usr/local/bin/modeltunnel

# Or use sudo for install
sudo make install
```

### Port already in use

```bash
# Check what's using port 8080
lsof -i :8080

# Use different port
modeltunnel up --ollama --port 3000
```

### Build errors

```bash
# Clean build
make clean
make build

# Update dependencies
make deps
```

---

## Uninstall

```bash
# If installed via Homebrew
brew uninstall modeltunnel
brew untap steliosot/modeltunnel

# If installed manually
sudo rm /usr/local/bin/modeltunnel
rm -rf ~/.config/modeltunnel

# Remove database (optional)
rm ~/.config/modeltunnel/keys.db
```

---

## Updating Modeltunnel

Safely update Modeltunnel without losing your configuration, API keys, or database:

### One-Command Update

```bash
curl -fsSL https://raw.githubusercontent.com/steliosot/modeltunnel/main/update.sh | sudo bash
```

### What Gets Preserved

The update script automatically backs up and preserves:

- ✓ **Configuration**: `~/.config/modeltunnel/config.yaml`
- ✓ **API Keys**: `~/.config/modeltunnel/keys.db` (SQLite database)
- ✓ **Tunnel Settings**: `~/.config/modeltunnel/tunnel.url`
- **Default location**: `/usr/local/bin/modeltunnel` (binary only)

### What Gets Updated

Just the binary - all your data remains intact.

### Update Script Workflow

1. **Backup**: Creates backup of `~/.config/modeltunnel/` to `/tmp/modeltunnel-backup-<timestamp>/`
2. **Stop Running Service**: Gracefully stops modeltunnel if running
3. **Install New Binary**: Downloads and installs latest modeltunnel
4. **Restore Service**: Restarts with your existing configuration

### Running in Docker

For Docker installations, simply restart the container:

```bash
# Stop and remove container
docker-compose down

# Pull latest image
docker pull ghcr.io/steliosot/modeltunnel:latest

# Restart with your previous volumes
docker-compose up -d

# Data persists in named volumes (ollama_data)
```

---

## Next Steps

- [CLI Reference](cli.md) - Command documentation
- [Configuration](configuration.md) - Server configuration
- [API Reference](api.md) - HTTP API documentation
