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

## Next Steps

- [CLI Reference](cli.md) - Command documentation
- [Configuration](configuration.md) - Server configuration
- [API Reference](api.md) - HTTP API documentation
