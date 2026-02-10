# Configuration Guide

Modeltunnel uses YAML for configuration with hot-reload support.

## Configuration File

**Default location:** `~/.config/modeltunnel/config.yaml`

**Environment variable:** `MODELTUNNEL_CONFIG`

```bash
# Use custom config
export MODELTUNNEL_CONFIG=/etc/modeltunnel/config.yaml
modeltunnel up --ollama
```

---

## Configuration Sections

### Server Configuration

```yaml
server:
  host: 127.0.0.1           # Bind address (use 0.0.0.0 for public)
  port: 8080                # Server port
  dashboard_enabled: true   # Enable web dashboard
  read_timeout: 30s         # HTTP read timeout
  write_timeout: 30s        # HTTP write timeout
```

| Option | Default | Description |
|--------|---------|-------------|
| `host` | 127.0.0.1 | Server bind address |
| `port` | 8080 | Server port |
| `dashboard_enabled` | true | Enable web dashboard at /admin |
| `read_timeout` | 30s | Maximum request read time |
| `write_timeout` | 30s | Maximum response write time |

---

### Upstreams

Configure upstream LLM providers.

```yaml
upstreams:
  default:
    type: ollama
    base_url: http://127.0.0.1:11434
    model: mistral
    timeout: 30s
  
  production:
    type: ollama
    base_url: http://10.0.0.5:11434
    model: mixtral
    timeout: 60s
  
  remote:
    type: ollama
    base_url: https://ollama.example.com
    model: llama2
    timeout: 120s
```

| Option | Default | Description |
|--------|---------|-------------|
| `type` | ollama | Upstream type (only ollama supported) |
| `base_url` | required | Ollama API URL |
| `model` | mistral | Default model |
| `timeout` | 30s | Request timeout |

---

### Rate Limiting Policies

Define rate limiting policies for different user types.

```yaml
policies:
  default:
    rate_limit: 60/min
    max_tokens: 4096
    models:
      mistral:
        rate_limit: 60/min
      phi:
        rate_limit: 100/min
  
  student:
    rate_limit: 30/min
    max_tokens: 2048
    models:
      mistral:
        rate_limit: 5/min
      phi:
        rate_limit: 50/min
      tinyllama:
        rate_limit: 100/min
  
  premium:
    rate_limit: 1000/min
    max_tokens: 8192
    models:
      mistral:
        rate_limit: 500/min
      deepseek-r1:
        rate_limit: 100/min
```

#### Rate Limit Formats

```yaml
rate_limit: 60/min      # 60 requests per minute
rate_limit: 100/hour    # 100 requests per hour
rate_limit: 1000/day    # 1000 requests per day
rate_limit: 10/second   # 10 requests per second
```

#### Per-Model Limits

```yaml
policies:
  mypolicy:
    rate_limit: 100/min        # Default for all models
    models:
      mistral:
        rate_limit: 5/min      # Specific limit for mistral
      phi:
        rate_limit: 100/min    # Specific limit for phi
```

---

### Tunnel Configuration

```yaml
tunnel:
  enabled: false
  subdomain: my-modeltunnel    # Request specific subdomain (optional)
```

| Option | Default | Description |
|--------|---------|-------------|
| `enabled` | false | Enable public tunnel |
| `subdomain` | random | Request specific subdomain |

**Note:** Subdomain requests are not guaranteed. If unavailable, a random subdomain is assigned.

---

### Async Jobs Configuration

```yaml
async:
  enabled: true
  workers: 3              # Number of concurrent workers
  queue_size: 1000        # Maximum pending jobs
  timeout: 120s           # Job timeout
```

| Option | Default | Description |
|--------|---------|-------------|
| `enabled` | true | Enable async jobs API |
| `workers` | 3 | Concurrent job workers |
| `queue_size` | 1000 | Maximum queue size |
| `timeout` | 120s | Per-job timeout |

---

### Intent Routing

Configure model priorities for intent-based routing in YAML.

**✅ Now Fully Configurable!** Intent routing can be customized via `~/.config/modeltunnel/config.yaml` with hot-reload support.

```yaml
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
  
  # Custom intents - add your own!
  creative:
    priority:
      - mistral:latest
      - qwen2.5:latest
    temperature: 0.9
    max_tokens: 3000
    description: "Creative writing, storytelling"
```

**Key Points:**
- If no `intents` section defined, built-in defaults are used
- Changes apply via hot-reload (no restart needed)
- Custom intents can be added beyond plan/code/chat
- Priority list defines fallback order
- Temperature and max_tokens applied automatically per intent

---

### Logging

```yaml
logging:
  level: info              # debug, info, warn, error
  format: text             # text, json
  file: ""                 # Log file path (empty = stdout)
```

---

## Complete Example

```yaml
# Modeltunnel Configuration
# Location: ~/.config/modeltunnel/config.yaml

server:
  host: 127.0.0.1
  port: 8080
  dashboard_enabled: true
  read_timeout: 30s
  write_timeout: 30s

upstreams:
  default:
    type: ollama
    base_url: http://127.0.0.1:11434
    model: mistral
    timeout: 30s

policies:
  default:
    rate_limit: 60/min
    max_tokens: 4096
    models:
      mistral:
        rate_limit: 60/min
      phi:
        rate_limit: 100/min
  
  student:
    rate_limit: 30/min
    max_tokens: 2048
    models:
      mistral:
        rate_limit: 5/min
      phi:
        rate_limit: 50/min
      tinyllama:
        rate_limit: 100/min
  
  premium:
    rate_limit: 1000/min
    max_tokens: 8192

async:
  enabled: true
  workers: 3
  queue_size: 1000
  timeout: 120s

intents:
  plan:
    priority:
      - deepseek-r1:latest
      - qwen2.5:latest
      - mistral:latest
    temperature: 0.3
    max_tokens: 4000
  
  code:
    priority:
      - qwen2.5:latest
      - mistral:latest
      - phi:latest
    temperature: 0.2
    max_tokens: 2000
  
  chat:
    priority:
      - phi:latest
      - tinyllama:latest
      - mistral:latest
    temperature: 0.7
    max_tokens: 1000

tunnel:
  enabled: false
  subdomain: ""

logging:
  level: info
  format: text
  file: ""
```

---

## Hot Reload

Configuration changes are automatically applied without restart.

### How It Works

1. Edit `~/.config/modeltunnel/config.yaml`
2. Save the file
3. Changes apply within 5 seconds

### What Can Be Reloaded

✅ **Hot-reloadable:**
- Rate limits
- Policy settings
- Upstream configuration
- Logging levels
- Intent routing

❌ **Requires restart:**
- Server host/port
- Database settings
- Async worker count

### Verification

```bash
# Check logs for reload
modeltunnel up --ollama

# Edit config and save
# Watch for log message:
# "🔄 Configuration reloaded"
```

---

## Environment Variables

Override configuration with environment variables:

```bash
# Server settings
export MODELTUNNEL_SERVER_HOST=0.0.0.0
export MODELTUNNEL_SERVER_PORT=8080

# Upstream
export MODELTUNNEL_UPSTREAM_DEFAULT_BASE_URL=http://localhost:11434
export MODELTUNNEL_UPSTREAM_DEFAULT_MODEL=mistral

# Logging
export MODELTUNNEL_LOG_LEVEL=debug
```

---

## Best Practices

### 1. Use Per-Model Limits

```yaml
policies:
  default:
    rate_limit: 60/min
    models:
      expensive-model:
        rate_limit: 5/min
      cheap-model:
        rate_limit: 100/min
```

### 2. Separate Policies for Different Users

```yaml
policies:
  free:
    rate_limit: 10/min
    max_tokens: 1024
  
  pro:
    rate_limit: 100/min
    max_tokens: 4096
  
  enterprise:
    rate_limit: 1000/min
    max_tokens: 8192
```

### 3. Secure Default Upstream

```yaml
upstreams:
  default:
    type: ollama
    base_url: http://127.0.0.1:11434  # Localhost only
    model: mistral
```

### 4. Intent Routing for UX

```yaml
intents:
  plan:
    priority:
      - deepseek-r1:latest
    temperature: 0.3  # Lower temp for reasoning
  
  chat:
    priority:
      - phi:latest
    temperature: 0.7  # Higher temp for creativity
```

---

## Troubleshooting

### Config not loading

```bash
# Check config location
echo $MODELTUNNEL_CONFIG
cat ~/.config/modeltunnel/config.yaml

# Validate YAML
modeltunnel up --ollama 2>&1 | head -20
```

### Hot reload not working

```bash
# Check file permissions
ls -la ~/.config/modeltunnel/

# Verify YAML syntax
python3 -c "import yaml; yaml.safe_load(open('config.yaml'))"

# Check logs for errors
modeltunnel up --ollama --log-level debug
```

### Rate limits not applying

```bash
# Verify policy assigned to key
modeltunnel key list

# Check config syntax
# Policies must be under 'policies:' section
```

---

## See Also

- [API Reference](api.md) - HTTP API documentation
- [CLI Reference](cli.md) - Command-line tools
- [Installation](installation.md) - Installation guide
