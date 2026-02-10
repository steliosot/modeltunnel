# PITCH

## Problem

Running local LLMs (Ollama) creates siloed environments with no unified API layer, authentication, or rate limiting, forcing developers to rebuild the same infrastructure for each project or expose models insecurely over the internet.

## Solution

Modeltunnel provides an OpenAI-compatible API gateway that transforms local Ollama instances into production-ready LLM services with enterprise-grade security, intent-based routing, and async job processing—all within 45MB and zero external dependencies.

## What It Does

Modeltunnel exposes local Ollama models through a standardized HTTP API that matches the OpenAI specification, enabling drop-in compatibility with existing LLM client libraries while adding authentication, rate limiting, and intelligent routing.

## Features

### Authentication & Authorization
- **API Key Management**: SQLite-backed key store with CLI/Server sync, supporting key creation, listing, and revocation
- **Per-Key Policies**: Configure rate limits, allowed models, and access rules per client
- **Hot-Reload Persistence**: Keys sync immediately between CLI and running server without restart

### API Compatibility
- **OpenAI API Spec**: `/v1/models`, `/v1/chat/completions` with streaming support
- **Content Array Parsing**: Handles both simple text and OpenAI content arrays (text/image)
- **Client Library Compatibility**: Works with OpenAI SDKs (Python, JavaScript), cURL, and any OpenAI-compatible client

### Intent-Based Routing
- **Smart Model Selection**: Configurable intent definitions (plan, code, chat) with YAML priority lists
- **Header-Based Routing**: `X-Model-Intent` header triggers automatic model selection based on task type
- **Hot-Reloading**: Intent config changes apply in ~5 seconds without restart

### Async Job Processing
- **Async Job API**: `/v1/async` endpoint for long-running requests, `/v1/jobs/{id}` for status checking
- **Worker Pool**: 3 concurrent workers with 1000-job max queue, 120s timeout per job
- **Job Lifecycle**: queued → processing → completed/failed states with result streaming

### Rate Limiting
- **Multi-Level Limits**: Per-key and per-model rate limiting (e.g., mistral=5/min, phi=100/min)
- **Configurable Policies**: YAML-based policy definitions with model-specific overrides

### Public Tunnel Support
- **LocalTunnel Integration**: Free public URLs via LocalTunnel (https://*.loca.lt)
- **Tunnel Health Monitoring**: Real-time tunnel status in dashboard
- **Subdomain Assignment**: Custom subdomains for persistent URLs

### Web Dashboard
- **Admin Interface**: `/admin` endpoint with key management, request logs, and configuration
- **Code Samples**: Python, JavaScript, and cURL examples for quick integration
- **Real-time Monitoring**: WebSocket-based request logging and tunnel status

### Configuration
- **YAML-Based**: `~/.config/modeltunnel/config.yaml` for all settings
- **Hot-Reload**: Configuration changes apply in ~5 seconds via file system watcher
- **Default Configs**: Sensible defaults with override support

### Security
- **Localhost Binding**: Default 127.0.0.1 binding, public access only via tunnel
- **HTTPS Tunnels**: Public URLs automatically use HTTPS
- **No Secrets in Logs**: API keys and tokens are never logged
- **Non-Root Container**: Docker image runs as UID 1000 (appuser) with minimal Alpine base

### Deployment
- **Binary Distribution**: Single static binary for Linux, macOS, Windows (~15MB)
- **Docker Support**: Multi-stage Dockerfile producing 45MB image with Alpine runtime
- **CLI Tool**: `init`, `up`, `key create/list/revoke` commands
- **Zero Dependencies**: No external services required beyond Ollama

### Developer Experience
- **CLI Reference**: Complete command documentation with examples
- **API Documentation**: Full OpenAI-compatible endpoint reference
- **Quick Start**: 3-step setup (init, up, key create)
- **Error Handling**: Clear error messages with action items

### Observability
- **Health Endpoint**: `/health` with server status and timestamp
- **Request Logging**: Real-time WebSocket-based request monitoring
- **Tunnel Status**: Dashboard displays tunnel connection state and URL

### External App Integration
- **OpenCode/Cursor Support**: Step-by-step connection guide
- **Multi-Platform**: macOS, Linux, Windows with native client apps

### Storage
- **SQLite Database**: Persistent key storage at `~/.config/modeltunnel/keys.db`
- **Config Backup**: Keys preserved in YAML as backup
- **Database Migration**: Automatic SQL migrations on startup

---

## Tech Stack

- **Language**: Go 1.20+ (static binaries, no runtime deps)
- **Database**: SQLite (file-based, embedded)
- **Tunneling**: LocalTunnel service
- **Web UI**: Static HTML/CSS/JS with WebSocket
- **Config**: YAML with fsnotify hot-reload

## Image Details

- **Base**: Alpine 3.19
- **Size**: ~45MB (multi-stage build)
- **User**: Non-root (appuser:group, UID 1000)
- **Entry**: `/app/modeltunnel up --ollama --model mistral`

## URLs

- **GitHub**: https://github.com/steliosot/modeltunnel
- **Releases**: https://github.com/steliosot/modeltunnel/releases
- **Documentation**: https://github.com/steliosot/modeltunnel/tree/main/docs
- **Docker Image**: `ghcr.io/steliosot/modeltunnel`

## Version

v1.0.0 (Initial Release)

## License

MIT