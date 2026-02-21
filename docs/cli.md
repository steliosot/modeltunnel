# CLI Reference

Modeltunnel provides a command-line interface for server management and API key administration.

## Installation

```bash
# Homebrew (macOS/Linux)
brew tap steliosot/modeltunnel
brew install modeltunnel

# Go
go install github.com/steliosot/modeltunnel/cmd/modeltunnel@latest

# From source
git clone https://github.com/steliosot/modeltunnel.git
cd modeltunnel
make build
sudo make install
```

## Updating Modeltunnel

Safely update to the latest version while preserving your configuration and API keys:

```bash
# One-command update (preserves all data)
curl -fsSL https://raw.githubusercontent.com/steliosot/modeltunnel/main/update.sh | sudo bash
```

**What Gets Preserved:**
- ✓ `~/.config/modeltunnel/config.yaml` - Your server configuration
- ✓ `~/.config/modeltunnel/keys.db` - All your API keys
- ✓ `~/.config/modeltunnel/tunnel.url` - Your tunnel settings

**What Gets Updated:**
- `/usr/local/bin/modeltunnel` - The binary only

For full details, see [update.sh](../update.sh) source code.

---

## Global Options

```bash
modeltunnel [global-options] <command> [command-options]
```

| Option | Description |
|--------|-------------|
| `-h, --help` | Show help message |
| `-v, --version` | Show version information |

---

## Commands

### `modeltunnel up`

Start the Modeltunnel server.

```bash
modeltunnel up [options]
```

#### Options

| Option | Default | Description |
|--------|---------|-------------|
| `--ollama` | false | (Optional) Use Ollama upstream |
| `--model` | mistral | Default model to use |
| `--host` | 127.0.0.1 | Server host address |
| `--port` | 8080 | Server port |
| `--tunnel` | false | Enable public tunnel via LocalTunnel |
| `--config` | ~/.config/modeltunnel/config.yaml | Config file path |

#### Examples

```bash
# Start with Ollama (local only)
modeltunnel up

# Start with public tunnel
modeltunnel up --tunnel

# Custom port
modeltunnel up --port 3000

# Production mode (all interfaces)
modeltunnel up --host 0.0.0.0 --port 8080
```

#### Output

```
🚀 Starting Modeltunnel...

Server Configuration:
  Host: 127.0.0.1
  Port: 8080
  Admin Key: mt_sk_admin_xxxxxxxx (save this!)
  Upstream: ollama (http://127.0.0.1:11434)

📊 Dashboard: http://127.0.0.1:8080/admin
🔑 API Base URL: http://127.0.0.1:8080/v1

✅ Server ready
```

---

### `modeltunnel init`

Initialize configuration and create necessary directories.

```bash
modeltunnel init
```

Creates:
- `~/.config/modeltunnel/config.yaml` - Configuration file
- `~/.config/modeltunnel/keys.db` - SQLite database

#### Example

```bash
modeltunnel init
```

Output:
```
✅ Configuration initialized at ~/.config/modeltunnel/config.yaml
```

---

### `modeltunnel key create`

Create a new API key with specified permissions.

```bash
modeltunnel key create <name> [options]
```

#### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Unique key identifier (e.g., `alice`, `prod-api`) |

#### Options

| Option | Description |
|--------|-------------|
| `--models` | Comma-separated list of allowed models (e.g., `mistral,phi`) |
| `--rate` | Rate limit (e.g., `60/min`, `100/hour`, `1000/day`) |
| `--policy` | Policy name from config (e.g., `student`, `default`) |

#### Examples

```bash
# Create basic key
modeltunnel key create alice

# Create key with model restrictions
modeltunnel key create bob --models mistral,phi

# Create key with custom rate limit
modeltunnel key create api-user --rate 100/min

# Create key with policy
modeltunnel key create student1 --policy student

# Combined options
modeltunnel key create prod-api --models mistral --rate 1000/hour
```

#### Output

```
✅ API Key created successfully

Name: alice
Key: mt_sk_alice_a1b2c3d4e5f6
Rate Limit: 60/min
Allowed Models: all
Status: active

⚠️  Save this key - it won't be shown again!
```

#### Key Format

API keys follow the format: `mt_sk_{name}_{random}`

Examples:
- `mt_sk_alice_a1b2c3d4e5f6`
- `mt_sk_prod-api_7g8h9i0j1k2l`

---

### `modeltunnel key list`

List all API keys and their status.

```bash
modeltunnel key list [options]
```

#### Options

| Option | Description |
|--------|-------------|
| `--format` | Output format: `table` (default), `json` |

#### Examples

```bash
# List all keys
modeltunnel key list

# JSON output
modeltunnel key list --format json
```

#### Output (table)

```
NAME        RATE LIMIT    MODELS        STATUS    CREATED
admin       10000/min     all           active    2026-02-10 10:00:00
alice       60/min        mistral,phi   active    2026-02-10 10:30:00
bob         5/min         all           revoked   2026-02-10 09:00:00
```

#### Output (json)

```json
[
  {
    "name": "admin",
    "key_preview": "mt_sk_admin_xxxx...",
    "rate_limit": "10000/min",
    "models": ["all"],
    "status": "active",
    "created_at": "2026-02-10T10:00:00Z"
  }
]
```

---

### `modeltunnel key revoke`

Revoke an API key (immediately disables access).

```bash
modeltunnel key revoke <name>
```

#### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Key name to revoke |

#### Examples

```bash
# Revoke a key
modeltunnel key revoke bob

# Confirm revocation
Are you sure you want to revoke key 'bob'? [y/N]: y
✅ Key 'bob' revoked successfully
```

#### Important Notes

- Revoked keys immediately lose access (within 5 seconds)
- Revocation cannot be undone
- Revoked keys remain in list with `revoked` status
- Database cleanup removes revoked keys periodically

---

### `modeltunnel version`

Show version information.

```bash
modeltunnel version
```

#### Output

```
Modeltunnel v1.0.0
Commit: abc1234
Built: 2026-02-10
Go Version: go1.21
```

---

## Configuration File

Default location: `~/.config/modeltunnel/config.yaml`

### Example Configuration

```yaml
server:
  host: 127.0.0.1
  port: 8080
  dashboard_enabled: true

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

tunnel:
  enabled: false
  subdomain: my-modeltunnel
```

### Configuration Hot-Reload

Configuration changes are automatically applied without restart:

1. Edit `~/.config/modeltunnel/config.yaml`
2. Save file
3. Changes apply within 5 seconds

---

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `MODELTUNNEL_CONFIG` | Config file path | `/etc/modeltunnel/config.yaml` |
| `MODELTUNNEL_LOG_LEVEL` | Log level | `debug`, `info`, `warn`, `error` |

---

## Common Workflows

### 1. Development Setup

```bash
# Initialize
modeltunnel init

# Start server
modeltunnel up

# Create dev key
modeltunnel key create dev --rate 1000/min
```

### 2. Production Deployment

```bash
# Create restricted key
modeltunnel key create prod-api --models mistral --rate 10000/hour

# Start with public tunnel
modeltunnel up --tunnel

# Save the public URL from output
```

### 3. Student/Team Access

```bash
# Create policy in config.yaml
# Then create keys

modeltunnel key create student1 --policy student
modeltunnel key create student2 --policy student
modeltunnel key create student3 --policy student

# List all student keys
modeltunnel key list | grep student
```

### 4. Key Rotation

```bash
# Create new key
modeltunnel key create prod-api-v2 --models mistral --rate 10000/hour

# Update clients to use new key

# Revoke old key
modeltunnel key revoke prod-api
```

---

## Troubleshooting

### Server won't start

```bash
# Check if port is in use
lsof -i :8080

# Use different port
modeltunnel up --port 3000
```

### Key not recognized

Keys created via CLI are available within 5 seconds. If not:

```bash
# Verify key exists
modeltunnel key list

# Check server is running
curl http://localhost:8080/health

# Verify API call
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer YOUR_KEY"
```

### Permission denied

```bash
# Check config directory permissions
ls -la ~/.config/modeltunnel/

# Fix permissions
chmod 755 ~/.config/modeltunnel
chmod 644 ~/.config/modeltunnel/config.yaml
```

---

## See Also

- [API Reference](api.md) - HTTP API documentation
- [Configuration](configuration.md) - Detailed configuration options
- [ASYNC_JOBS.md](ASYNC_JOBS.md) - Async job system
- [INTENT_ROUTING.md](INTENT_ROUTING.md) - Intent-based routing
