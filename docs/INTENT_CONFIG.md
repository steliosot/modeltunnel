# Intent Routing Configuration

Intent routing in Modeltunnel can be configured via YAML or used with defaults.

## Quick Answer

**Yes, intent routing can be configured in YAML!**

Edit `~/.config/modeltunnel/config.yaml`:

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
```

## Configuration Fields

### Intent Configuration

Each intent has these fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `priority` | list | Yes | Ordered list of preferred models (tried in order) |
| `temperature` | float | No | Default temperature for this intent (0.0-2.0) |
| `max_tokens` | integer | No | Default max tokens for this intent |
| `description` | string | No | Human-readable description |

### Priority List

The `priority` list defines which models to try, in order:

```yaml
priority:
  - deepseek-r1:latest    # Try this first (best)
  - qwen2.5:latest      # If unavailable, try this
  - mistral:latest         # Fallback option
```

**Behavior:**
1. Try first model in list
2. If not available, try second
3. Continue until available model found
4. If none available, return error

## Default Configuration

If no `intents` section exists in config, these defaults are used:

```yaml
# Auto-generated defaults (no config needed)
plan:
  priority: [deepseek-r1, qwen2.5, mistral]
  temperature: 0.3
  max_tokens: 4000

code:
  priority: [qwen2.5, mistral, phi]
  temperature: 0.2
  max_tokens: 2000

chat:
  priority: [phi, tinyllama, mistral]
  temperature: 0.7
  max_tokens: 1000
```

## Complete Example Config

```yaml
# ~/.config/modeltunnel/config.yaml

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

# INTENT ROUTING CONFIGURATION
intents:
  # Planning/Reasoning - uses best reasoning models
  plan:
    priority:
      - deepseek-r1:latest    # Best reasoning
      - qwen2.5:latest      # Good reasoning
      - mistral:latest         # Fallback
    temperature: 0.3    # Lower temp for accuracy
    max_tokens: 4000     # Longer responses
    description: "Planning, strategy, reasoning"
  
  # Coding - uses best coding models
  code:
    priority:
      - qwen2.5:latest      # Best for code
      - mistral:latest         # Good for code
      - phi:latest            # Fallback, fast
    temperature: 0.2    # Very low for precision
    max_tokens: 2000     # Medium length
    description: "Programming, debugging, technical"
  
  # Chat - uses fastest models
  chat:
    priority:
      - phi:latest            # Fastest
      - tinyllama:latest     # Very fast
      - mistral:latest         # Fallback
    temperature: 0.7    # Higher for creativity
    max_tokens: 1000     # Short, quick
    description: "General chat, Q&A, support"
  
  # Custom intent for creative tasks
  creative:
    priority:
      - mistral:latest
      - qwen2.5:latest
    temperature: 0.9    # Very high temp
    max_tokens: 3000
    description: "Creative writing, storytelling"

async:
  enabled: true
  workers: 3
  queue_size: 1000
  timeout: 120s

tunnel:
  enabled: false
```

## Hot Reload

Intent configuration is **hot-reloadable**:

1. Edit `~/.config/modeltunnel/config.yaml`
2. Save file
3. Changes apply within 5 seconds

No restart required!

## Usage Examples

### With Default Intents

Just use the header:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "X-Model-Intent: plan" \
  -d '{"model": "auto", "messages": [...]}'
```

### With Custom Intents

After adding custom intent to config:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "X-Model-Intent: creative" \
  -d '{"model": "auto", "messages": [...]}'
```

### Python

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="your-key"
)

# Uses plan intent with your custom priority
response = client.chat.completions.create(
    model="auto",
    messages=[{"role": "user", "content": "Design a system"}],
    extra_headers={"X-Model-Intent": "plan"}
)
```

## Model Matching

The priority list supports flexible model matching:

| Format | Example | Matches |
|--------|---------|----------|
| Full with tag | `deepseek-r1:latest` | Exact match |
| Base name | `mistral` | `mistral`, `mistral:latest`, `mistral:7b` |
| Partial | `qwen2.5` | `qwen2.5:latest`, `qwen2.5:14b` |

## Troubleshooting

### Intent Not Working

**Check config:**
```bash
cat ~/.config/modeltunnel/config.yaml
```

**Verify YAML syntax:**
```bash
python3 -c "import yaml; yaml.safe_load(open('config.yaml'))"
```

**Check logs:**
```bash
# Server logs show loaded intents
tail -f /tmp/modeltunnel.log
```

### Wrong Model Selected

1. Check if model is available:
   ```bash
   curl http://localhost:8080/v1/models
   ```

2. Verify priority list order in config

3. Check model name format (include tag if needed)

## Best Practices

1. **Order by capability** - Put best model first
2. **Include fallbacks** - Always have 2-3 options
3. **Match intent to use case** - Plan→reasoning, Code→coding
4. **Set appropriate temps** - Lower for accuracy, higher for creativity
5. **Test priority list** - Verify models are actually available

## Advanced: Per-Intent Rate Limits

Coming soon! Future versions will support per-intent rate limiting:

```yaml
intents:
  plan:
    priority: [...]
    rate_limit: 5/min  # Per-intent limits
    max_tokens: 4000
```

---

## Summary

- ✅ Intents **can be configured** in YAML
- ✅ Config location: `~/.config/modeltunnel/config.yaml`
- ✅ **Hot-reloadable** - changes apply without restart
- ✅ Defaults available if no config provided
- ✅ Flexible model matching (full name, base, partial)
- ✅ Custom intents supported

Configure your intents and restart the server to apply!
