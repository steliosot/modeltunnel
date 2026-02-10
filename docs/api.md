# API Reference

Modeltunnel provides an OpenAI-compatible API with additional features for local LLM management.

**Base URL:** `http://localhost:8080/v1`

**Authentication:** All API endpoints (except health) require an API key in the `Authorization` header:
```
Authorization: Bearer YOUR_API_KEY
```

---

## Table of Contents

- [Chat Completions](#chat-completions)
- [Models](#models)
- [Async Jobs](#async-jobs)
- [Health Check](#health-check)
- [Headers](#headers)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)

---

## Chat Completions

Create a chat completion request. Compatible with OpenAI's API.

```http
POST /v1/chat/completions
```

### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `model` | string | Yes | Model name (e.g., `ollama/mistral:latest` or `auto`) |
| `messages` | array | Yes | Array of message objects |
| `stream` | boolean | No | Enable streaming (default: false) |
| `max_tokens` | integer | No | Maximum tokens to generate |
| `temperature` | float | No | Sampling temperature (0-2) |
| `top_p` | float | No | Nucleus sampling (0-1) |

### Request Example

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/mistral:latest",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello!"}
    ],
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

### Response

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1704067200,
  "model": "ollama/mistral:latest",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 10,
    "total_tokens": 35
  }
}
```

### Streaming Response

When `stream: true`, responses are sent as Server-Sent Events (SSE):

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/mistral:latest",
    "messages": [{"role": "user", "content": "Hi"}],
    "stream": true
  }'
```

Response:
```
data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1704067200,"model":"ollama/mistral:latest","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1704067200,"model":"ollama/mistral:latest","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1704067200,"model":"ollama/mistral:latest","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: {"id":"chatcmpl-1","object":"chat.completion.chunk","created":1704067200,"model":"ollama/mistral:latest","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

---

## Models

### List Models

```http
GET /v1/models
```

Returns a list of available models from configured upstreams.

```bash
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer mt_sk_alice_abc123"
```

Response:
```json
{
  "object": "list",
  "data": [
    {
      "id": "ollama/mistral:latest",
      "object": "model",
      "created": 1704067200,
      "owned_by": "ollama",
      "size": 4110913818,
      "modified_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": "ollama/phi:latest",
      "object": "model",
      "created": 1704067200,
      "owned_by": "ollama",
      "size": 1594386432,
      "modified_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

---

## Async Jobs

Submit long-running requests and poll for results.

### Submit Async Job

```http
POST /v1/async
```

Accepts the same request body as `/v1/chat/completions`.

```bash
curl -X POST http://localhost:8080/v1/async \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/phi:latest",
    "messages": [{"role": "user", "content": "Write a story"}],
    "max_tokens": 500
  }'
```

Response:
```json
{
  "job_id": "job_1770736516577382000",
  "status": "queued"
}
```

### Get Job Status

```http
GET /v1/jobs/{job_id}
```

```bash
curl http://localhost:8080/v1/jobs/job_1770736516577382000 \
  -H "Authorization: Bearer mt_sk_alice_abc123"
```

Response (queued):
```json
{
  "job_id": "job_1770736516577382000",
  "status": "queued",
  "created_at": "2026-02-10T15:15:16.577383Z"
}
```

Response (running):
```json
{
  "job_id": "job_1770736516577382000",
  "status": "running",
  "created_at": "2026-02-10T15:15:16.577383Z",
  "started_at": "2026-02-10T15:15:17.123456Z"
}
```

Response (completed):
```json
{
  "job_id": "job_1770736516577382000",
  "status": "completed",
  "result": {
    "id": "chatcmpl-abc123",
    "object": "chat.completion",
    "created": 1704067200,
    "model": "ollama/phi:latest",
    "choices": [
      {
        "index": 0,
        "message": {
          "role": "assistant",
          "content": "Once upon a time..."
        },
        "finish_reason": "stop"
      }
    ],
    "usage": {
      "prompt_tokens": 10,
      "completion_tokens": 100,
      "total_tokens": 110
    }
  },
  "created_at": "2026-02-10T15:15:16.577383Z",
  "started_at": "2026-02-10T15:15:17.123456Z",
  "completed_at": "2026-02-10T15:15:18.789012Z"
}
```

Response (failed):
```json
{
  "job_id": "job_1770736516577382000",
  "status": "failed",
  "error": "upstream timeout",
  "created_at": "2026-02-10T15:15:16.577383Z",
  "started_at": "2026-02-10T15:15:17.123456Z",
  "completed_at": "2026-02-10T15:17:17.456789Z"
}
```

### Job Status Values

| Status | Description |
|--------|-------------|
| `queued` | Job is waiting in queue |
| `running` | Job is being processed |
| `completed` | Job finished successfully |
| `failed` | Job failed with error |

**Implementation Details:**
- In-memory queue (1000 jobs max)
- 3 concurrent workers
- 120 second timeout per job
- Jobs lost on server restart

See [ASYNC_JOBS.md](ASYNC_JOBS.md) for Python examples and polling patterns.

---

## Health Check

```http
GET /health
```

No authentication required.

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2026-02-10T15:00:00Z"
}
```

---

## Headers

### Request Headers

| Header | Description | Example |
|--------|-------------|---------|
| `Authorization` | API key authentication | `Bearer mt_sk_alice_abc123` |
| `Content-Type` | Request body format | `application/json` |
| `X-Model-Intent` | Intent-based routing (optional) | `plan`, `code`, `chat` |

### Response Headers

| Header | Description |
|--------|-------------|
| `X-Request-ID` | Unique request identifier |
| `X-RateLimit-Limit` | Rate limit for the window |
| `X-RateLimit-Remaining` | Remaining requests in window |
| `X-RateLimit-Reset` | Unix timestamp when limit resets |
| `X-Routed-Model` | Actual model used (intent routing) |
| `X-Model-Intent` | Intent that was applied |

---

## Intent-Based Routing

Use the `X-Model-Intent` header to let Modeltunnel select the best model:

```bash
# Planning/Reasoning → deepseek-r1 → qwen2.5 → mistral
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "X-Model-Intent: plan" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Create a project roadmap"}]
  }'

# Coding → qwen2.5 → mistral → phi
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "X-Model-Intent: code" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Debug this function"}]
  }'

# Chat → phi → tinyllama → mistral
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "X-Model-Intent: chat" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Response Headers:**
```
X-Routed-Model: deepseek-r1:latest
X-Model-Intent: plan
```

See [INTENT_ROUTING.md](INTENT_ROUTING.md) for details.

---

## Error Handling

### Error Response Format

```json
{
  "error": {
    "message": "Invalid API key",
    "type": "authentication_error",
    "code": 401
  }
}
```

### HTTP Status Codes

| Code | Meaning | Common Causes |
|------|---------|---------------|
| 200 | OK | Request successful |
| 202 | Accepted | Async job queued |
| 400 | Bad Request | Invalid JSON, missing fields |
| 401 | Unauthorized | Invalid or missing API key |
| 404 | Not Found | Job ID not found |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Server error |
| 502 | Bad Gateway | Upstream unavailable |

### Common Errors

**401 - Invalid API Key:**
```json
{
  "error": {
    "message": "invalid API key",
    "type": "authentication_error",
    "code": 401
  }
}
```

**429 - Rate Limited:**
```json
{
  "error": {
    "message": "rate limit exceeded",
    "type": "rate_limit_error",
    "code": 429
  }
}
```

---

## Rate Limiting

Modeltunnel implements token bucket rate limiting per API key.

### Rate Limit Headers

Every response includes rate limit information:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 59
X-RateLimit-Reset: 1704067260
```

### Per-Model Limits

Different models can have different rate limits:

```yaml
# config.yaml
policies:
  student:
    rate_limit: 60/min
    models:
      mistral:
        rate_limit: 5/min
      phi:
        rate_limit: 100/min
```

### Rate Limit Response

When exceeded:
```json
{
  "error": {
    "message": "rate limit exceeded",
    "type": "rate_limit_error",
    "code": 429
  }
}
```

---

## Code Examples

### Python

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="mt_sk_alice_abc123"
)

# Sync request
response = client.chat.completions.create(
    model="ollama/mistral:latest",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)

# With intent routing
response = client.chat.completions.create(
    model="auto",
    messages=[{"role": "user", "content": "Create a roadmap"}],
    extra_headers={"X-Model-Intent": "plan"}
)

# Streaming
for chunk in client.chat.completions.create(
    model="ollama/mistral:latest",
    messages=[{"role": "user", "content": "Hello"}],
    stream=True
):
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### JavaScript

```javascript
const response = await fetch('http://localhost:8080/v1/chat/completions', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer mt_sk_alice_abc123',
    'Content-Type': 'application/json',
    'X-Model-Intent': 'code'
  },
  body: JSON.stringify({
    model: 'auto',
    messages: [{ role: 'user', content: 'Write a function' }]
  })
});

const data = await response.json();
console.log(data.choices[0].message.content);
```

### cURL

```bash
# Basic request
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/mistral:latest",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# With intent routing
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "X-Model-Intent: plan" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Create a plan"}]
  }'

# Streaming
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/phi:latest",
    "messages": [{"role": "user", "content": "Hi"}],
    "stream": true
  }'
```

---

## Dashboard

Access the web dashboard at:

```
http://localhost:8080/admin
```

**Features:**
- View API usage statistics
- Create and revoke API keys
- Configure rate limits
- Monitor request logs in real-time
- Copy code samples

---

## WebSocket Logs

Connect to WebSocket for real-time request logs:

```javascript
const ws = new WebSocket('ws://localhost:8080/ws/logs');

ws.onmessage = (event) => {
  const log = JSON.parse(event.data);
  console.log(log.timestamp, log.method, log.path, log.status);
};
```

**Log Format:**
```json
{
  "timestamp": "2026-02-10T15:00:00Z",
  "method": "POST",
  "path": "/v1/chat/completions",
  "status": 200,
  "duration_ms": 1200,
  "client_ip": "127.0.0.1"
}
```

---

## Combining Features

### Async + Intent Routing

Use async jobs with intent routing for long-running optimized tasks:

```bash
# Submit async job with intent routing
curl -X POST http://localhost:8080/v1/async \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "Content-Type: application/json" \
  -H "X-Model-Intent: plan" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Design a microservices architecture"}],
    "max_tokens": 4000
  }'

# Poll for result
curl http://localhost:8080/v1/jobs/job_123 \
  -H "Authorization: Bearer mt_sk_alice_abc123"
```

**Python Example:**
```python
import requests
import time

# Submit research job with planning intent
response = requests.post(
    "http://localhost:8080/v1/async",
    headers={
        "Authorization": "Bearer mt_sk_alice_abc123",
        "X-Model-Intent": "plan"  # Routes to deepseek-r1
    },
    json={
        "model": "auto",
        "messages": [{"role": "user", "content": "Analyze market trends"}],
        "max_tokens": 4000
    }
)

job_id = response.json()["job_id"]

# Poll for completion
while True:
    status = requests.get(
        f"http://localhost:8080/v1/jobs/{job_id}",
        headers={"Authorization": "Bearer mt_sk_alice_abc123"}
    ).json()
    
    if status["status"] == "completed":
        print(f"Routed model: {status['result']['model']}")
        print(f"Result: {status['result']['choices'][0]['message']['content']}")
        break
    
    time.sleep(2)
```

### Streaming + Intent Routing

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_alice_abc123" \
  -H "Content-Type: application/json" \
  -H "X-Model-Intent: code" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Write a function"}],
    "stream": true,
    "max_tokens": 2000
  }'
```

---

## See Also

- [ASYNC_JOBS.md](ASYNC_JOBS.md) - Complete async job documentation with examples
- [INTENT_ROUTING.md](INTENT_ROUTING.md) - Detailed intent routing guide
- [Configuration](configuration.md) - Server configuration
- [CLI Reference](cli.md) - Command-line tools
