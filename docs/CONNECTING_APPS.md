# Connecting External Apps to Modeltunnel

Complete guide to connect AI apps like **OpenCode**, **Cursor**, **Continue.dev**, and any OpenAI-compatible client to your local Ollama models via Modeltunnel.

---

## Quick Setup (5 Minutes)

### Step 1: Start Modeltunnel with Public Tunnel

```bash
# Start server with public URL (accessible from anywhere)
./build/modeltunnel up --ollama --model mistral --tunnel
```

**Output:**
```
🌐 Public URL: https://your-random-name.loca.lt/v1
```

**Save this URL** - you'll need it for OpenCode.

---

### Step 2: Create API Key for OpenCode

```bash
# Create dedicated key for OpenCode with rate limit
./build/modeltunnel key create opencode --models mistral --rate 100/min
```

**Output:**
```
✅ API Key created successfully

Name: opencode
Key: mt_sk_opencode_xxxxxxxxxxxxxxxxxxxxxxx  <-- COPY THIS KEY
Rate Limit: 100/min
Allowed Models: mistral
Status: active

⚠️  Save this key - it won't be shown again!
```

---

### Step 3: Configure OpenCode

#### Finding OpenCode Settings:

1. **Open OpenCode Settings/Preferences**
   - Look for: **Providers** or **Model Settings**
   - Find: **Custom Providers** or **Add Custom Provider**
   - Click: **+** or **Add Provider**

#### Fill In the Provider Settings:

| Field | What Enter |
|-------|-------------------|
| **Provider ID** | `modeltunnel` |
| **Display name** | `Local Mistral` |
| **Base URL** | `https://your-random-name.loca.lt/v1` (from Step 1) |
| **API key** | `mt_sk_opencode_xxxxxxxxxxxxxxxxxxxxxxx` (from Step 2) |
| **Models → ID** | `ollama/mistral:latest` |
| **Models → Name** | `Mistral (Local)` |

#### Optional: Add Intent Header (for smart routing):

| Header | Value |
|--------|-------|
| **X-Model-Intent** | `plan` (or `code` or `chat`) |

---

### Step 4: Test the Connection

In OpenCode, type a message like *"Hi from OpenCode!"* or *"Are you working?"* 

You should get a response from your local Mistral model!

---

## Alternative: Local Only (Same Machine)

If OpenCode is on the same machine, you don't need the tunnel:

| Field | Value |
|-------|-------|
| **Base URL** | `http://localhost:8080/v1` |
| **Other fields** | Same as above |

---

## Complete Example

```bash
# 1. Start tunnel
./build/modeltunnel up --ollama --model mistral --tunnel

# Output: 🌐 Public URL: https://slick-trains-type.loca.lt/v1

# 2. Create key
./build/modeltunnel key create opencode --models mistral --rate 100/min

# Output: Key: mt_sk_opencode_4a2cc6d7abb3939e25d51697669e254631457985823872a4e796784c1da039e8

# 3. Configure OpenCode with:
# - Base URL: https://slick-trains-type.loca.lt/v1
# - API key: mt_sk_opencode_4a2cc6d7abb3939e25d51697669e254631457985823872a4e796784c1da039e8
# - Model: ollama/mistral:latest
```

---

## Troubleshooting

### Error: "Invalid request body"

**Fix**: Ensure your message format is correct. Try a simple message first:
```
{"role": "user", "content": "Hello"}
```

### Error: "Unauthorized"

**Fix**: Verify the API key is correct. Check if any spaces or extra chars.

### Error: "Rate limit exceeded"

**Fix**: Wait a few seconds, or use `--rate 500/min` when creating the key.

### Error: Tunnel not accessible

**Fix**: 
- Check if tunnel is still running
- Restart: `pkill -f modeltunnel` then start again
- Tunnel URL changes on each restart

---

## Other Apps

The same setup works for any OpenAI-compatible app:

**Cursor:**
- Settings → AI → Add Custom Provider

**Continue.dev:**
- Settings → Models → Add Custom

**Claude Desktop:**
- Settings → API Keys → Custom Provider

**Aider:**
- `.env` file: `OPENAI_API_BASE_URL=https://your-url.loca.lt/v1`

---

## Security Best Practices

1. **Create separate keys** for each app
2. **Use rate limits** to protect your machine
3. **Revoke unused keys**: `./build/modeltunnel key revoke opencode`
4. **Monitor usage**: Check `/admin/dashboard` dashboard
5. **Use tunnel only when needed**: Local is more secure

---

## Advanced: Custom Intents

In OpenCode settings, add the `X-Model-Intent` header to use intent routing:

| Setting | Value |
|---------|-------|
| **Header** | `X-Model-Intent` |
| **Value** | `plan` (for reasoning) or `code` (for programming) or `chat` (for chat) |

This tells Modeltunnel to auto-select the best model for your task!

---

## Quick Reference

### Commands to Run:

```bash
# Start with public tunnel
./build/modeltunnel up --ollama --model mistral --tunnel

# Create key for OpenCode
./build/modeltunnel key create opencode --models mistral --rate 100/min

# List all keys
./build/modeltunnel key list

# Revoke a key
./build/modeltunnel key revoke opencode
```

### What You'll See:

```
Tunnel URL: https://random-name.loca.lt/v1
API Key: mt_sk_opencode_xxxxxxxxxxxxxx
Model: olama/mistral:latest
```

---

That's it! Your local Mistral model is now accessible from anywhere ✅