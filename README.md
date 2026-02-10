# Modeltunnel

**ngrok for LLMs** - Expose your local Ollama models with an OpenAI-compatible API and key-based authentication.

[![Release](https://img.shields.io/github/v/release/steliosot/modeltunnel?style=flat-square)](https://github.com/steliosot/modeltunnel/releases)
[![License](https://img.shields.io/github/license/steliosot/modeltunnel?style=flat-square)](LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/steliosot/modeltunnel?style=flat-square)](https://goreportcard.com/report/github.com/steliosot/modeltunnel)

[Features](#features) • [Quick Start](#quick-start) • [Installation](#installation) • [Documentation](#documentation) • [Contributing](#contributing)

---

<p align="center">
  <img src="docs/images/dashboard-overview.png" alt="Modeltunnel Dashboard" width="800">
</p>

## Features

- **Zero Configuration** - Works out of the box with Ollama
- **Key-Based Authentication** - Secure API access with granular permissions
- **OpenAI-Compatible API** - Drop-in replacement for OpenAI SDK
- **Async Job API** - Submit long-running jobs and poll for results
- **Intent-Based Routing** - Smart model selection via `X-Model-Intent` header
- **Built-in Dashboard** - Web UI for managing keys and monitoring usage
- **Public Tunnels** - Built-in support for LocalTunnel (zero setup)
- **Per-Model Rate Limiting** - Different limits for different models
- **Persistent Keys** - SQLite database for key storage (CLI keys available immediately)
- **Hot Reload** - Configuration changes apply without restart
- **Request Logging** - Real-time monitoring via WebSocket

## Installation

### Using Homebrew (macOS/Linux)

```bash
brew tap steliosot/modeltunnel
brew install modeltunnel
```

### Using Go

```bash
go install github.com/steliosot/modeltunnel/cmd/modeltunnel@latest
```

### From Source

```bash
git clone https://github.com/steliosot/modeltunnel.git
cd modeltunnel
make build
sudo make install
```

### Pre-built Binaries

Download from [Releases](https://github.com/steliosot/modeltunnel/releases)

### Docker

```bash
# Build the image
docker build -t modeltunnel:latest .

# Run with Ollama (local only)
docker run -d -p 8080:8080 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  modeltunnel:latest up --ollama --model mistral

# Run with public tunnel
docker run -d -p 8080:8080 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  modeltunnel:latest up --ollama --model mistral --tunnel

# Custom config (mount your config.yaml)
docker run -d -p 8080:8080 \
  -v /path/to/config.yaml:/home/appuser/.config/modeltunnel/config.yaml \
  modeltunnel:latest up
```

Docker image size: ~45MB
```

## Quick Start

### 1. Start the Server

```bash
# With Ollama (local only)
modeltunnel up --ollama --model mistral

# With public tunnel (accessible from anywhere)
modeltunnel up --ollama --model mistral --tunnel
```

### 2. Access the Dashboard

Open http://localhost:8080/admin in your browser

### 3. Create an API Key

```bash
modeltunnel key create alice --models mistral,phi
```

### 4. Use with Any OpenAI Client

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="mt_sk_alice_..."
)

response = client.chat.completions.create(
    model="ollama/mistral:latest",
    messages=[{"role": "user", "content": "Hello!"}]
)

print(response.choices[0].message.content)
```

## Advanced Features

### Async Jobs

Submit long-running requests and poll for results:

```bash
# Submit async job
curl -X POST http://localhost:8080/v1/async \
  -H "Authorization: Bearer YOUR_KEY" \
  -d '{"model": "ollama/phi:latest", "messages": [{"role": "user", "content": "Hello"}]}'

# Response: {"job_id": "job_123...", "status": "queued"}

# Check status
curl http://localhost:8080/v1/jobs/job_123... \
  -H "Authorization: Bearer YOUR_KEY"
```

### Intent-Based Routing

Let Modeltunnel choose the best model for your task:

```bash
# Planning/Reasoning → routes to deepseek-r1
curl http://localhost:8080/v1/chat/completions \
  -H "X-Model-Intent: plan" \
  -d '{"model": "auto", "messages": [...]}'

# Coding → routes to qwen2.5
curl http://localhost:8080/v1/chat/completions \
  -H "X-Model-Intent: code" \
  -d '{"model": "auto", "messages": [...]}'

# Chat → routes to phi (fast)
curl http://localhost:8080/v1/chat/completions \
  -H "X-Model-Intent: chat" \
  -d '{"model": "auto", "messages": [...]}'
```

See [docs/ASYNC_JOBS.md](docs/ASYNC_JOBS.md) and [docs/INTENT_ROUTING.md](docs/INTENT_ROUTING.md) for details.

---

### Connect to External Apps

Use your local Ollama models from OpenCode, Cursor, or any OpenAI-compatible app:

```bash
# Create public tunnel
./build/modeltunnel up --ollama --model mistral --tunnel

# Create key for OpenCode
./build/modeltunnel key create opencode --models mistral --rate 100/min

# Fill in these settings in OpenCode:
# - Base URL: https://your-url.loca.lt/v1
# - API key: mt_sk_opencode_xxxxx...
# - Model: ollama/mistral:latest
```

See [docs/CONNECTING_APPS.md](docs/CONNECTING_APPS.md) for complete step-by-step guide.

---

## Documentation

- **[Connecting Apps](docs/CONNECTING_APPS.md)** - Connect OpenCode, Cursor, and other apps ✨
- **[Installation Guide](docs/installation.md)** - Detailed installation instructions
- **[Configuration](docs/configuration.md)** - YAML configuration reference
- **[API Reference](docs/api.md)** - Complete API documentation with examples
- **[Async Jobs](docs/ASYNC_JOBS.md)** - Asynchronous request processing with 5 complete examples
- **[Intent Routing](docs/INTENT_ROUTING.md)** - Smart model selection with 5 complete examples
- **[CLI Reference](docs/cli.md)** - Command-line interface documentation
- **[Examples Index](docs/EXAMPLES.md)** - Quick reference for all code examples
- **[Security](SECURITY.md)** - Security best practices
- **[Contributing](CONTRIBUTING.md)** - How to contribute

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   Dashboard     │────▶│   Modeltunnel    │────▶│     Ollama      │
│  (Web UI)       │     │   Server         │     │   (Local LLM)   │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │   SQLite DB      │
                       │   (API Keys)     │
                       └──────────────────┘
```

## Configuration

Configuration file: `~/.config/modeltunnel/config.yaml`

```yaml
server:
  host: 127.0.0.1
  port: 8080

upstreams:
  default:
    type: ollama
    base_url: http://127.0.0.1:11434
    model: mistral

policies:
  default:
    rate_limit: 60/min
    max_tokens: 4096
  student:
    rate_limit: 60/min
    max_tokens: 4096
    models:
      mistral:
        rate_limit: 5/min
      phi:
        rate_limit: 100/min

# Intent Routing Configuration (Optional)
intents:
  plan:
    priority:
      - deepseek-r1:latest
      - qwen2.5:latest
      - mistral:latest
    temperature: 0.3
    max_tokens: 4000
    description: "Planning, strategy, reasoning"
  
  code:
    priority:
      - qwen2.5:latest
      - mistral:latest
      - phi:latest
    temperature: 0.2
    max_tokens: 2000
    description: "Programming, debugging, technical"
  
  chat:
    priority:
      - phi:latest
      - tinyllama:latest
      - mistral:latest
    temperature: 0.7
    max_tokens: 1000
    description: "General chat, Q&A, support"

async:
  enabled: true
  workers: 3
  queue_size: 1000
  timeout: 120s
```

**Key Features:**
- 🔧 Hot-reload enabled - changes apply without restart
- 📊 Per-model rate limiting
- 🎯 Intent-based routing via YAML
- ⚡ Async job system
- 💾 SQLite persistence for API keys

## Testing

```bash
# Run all tests
make test-all

# Run unit tests only
make test

# Run integration tests
make test-integration
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

Quick start for contributors:

```bash
# Fork and clone
git clone https://github.com/YOUR_USERNAME/modeltunnel.git
cd modeltunnel

# Install dependencies
make deps

# Make changes
# ...

# Test
make test

# Submit PR
```

## Comparison

| Feature | Modeltunnel | ngrok | Cloudflare Tunnel |
|---------|-------------|-------|-------------------|
| Local LLM Support | Yes | No | No |
| Built-in Auth | Yes | No | No |
| Dashboard | Yes | No | No |
| Async Jobs API | Yes | No | No |
| Intent-Based Routing | Yes | No | No |
| Per-Model Rate Limits | Yes | No | No |
| OpenAI API Compatible | Yes | No | No |
| Free Public URLs | Yes | Limited | Yes |

## Security

- **Authentication Required** - All API endpoints require valid keys
- **Rate Limiting** - Configurable limits per key and per model
- **Local by Default** - Binds to localhost only (127.0.0.1)
- **HTTPS Tunnels** - Public tunnels use HTTPS automatically
- **No Key Storage in Logs** - API keys are never logged

See [SECURITY.md](SECURITY.md) for details.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history.

## License

MIT License - see [LICENSE](LICENSE) file

## Acknowledgments

- [Ollama](https://ollama.ai/) - For making local LLMs accessible
- [LocalTunnel](https://localtunnel.me/) - For free public tunnels
- [Gorilla WebSocket](https://github.com/gorilla/websocket) - For real-time dashboard

## Community

- [GitHub Discussions](https://github.com/steliosot/modeltunnel/discussions) - Q&A and ideas
- [Issues](https://github.com/steliosot/modeltunnel/issues) - Bug reports and features

---

<p align="center">
  Made by <a href="https://github.com/steliosot">steliosot</a>
</p>
