# Async Job System

Modeltunnel supports asynchronous model requests. Submit a job and get a `job_id` immediately, then poll for results later. Perfect for long-running requests, batch processing, or when you don't want to block the client.

## Table of Contents

- [Quick Start](#quick-start)
- [When to Use Async](#when-to-use-async)
- [Complete Examples](#complete-examples)
- [API Reference](#api-reference)
- [Implementation Details](#implementation-details)
- [Best Practices](#best-practices)
- [Error Handling](#error-handling)
- [Advanced Usage](#advanced-usage)

---

## Quick Start

### 1. Submit Async Request

```bash
curl -X POST http://localhost:8080/v1/async \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/phi:latest",
    "messages": [{"role": "user", "content": "Say hello"}],
    "max_tokens": 50
  }'
```

**Response:**
```json
{
  "job_id": "job_1770736516577382000",
  "status": "queued"
}
```

### 2. Check Job Status

```bash
curl http://localhost:8080/v1/jobs/job_1770736516577382000 \
  -H "Authorization: Bearer YOUR_API_KEY"
```

**Response (queued):**
```json
{
  "job_id": "job_1770736516577382000",
  "status": "queued",
  "created_at": "2026-02-10T15:15:16.577383Z"
}
```

**Response (completed):**
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
          "content": "Hello! How can I help you today?"
        },
        "finish_reason": "stop"
      }
    ],
    "usage": {
      "prompt_tokens": 10,
      "completion_tokens": 10,
      "total_tokens": 20
    }
  },
  "created_at": "2026-02-10T15:15:16.577383Z",
  "started_at": "2026-02-10T15:15:17.123456Z",
  "completed_at": "2026-02-10T15:15:18.789012Z"
}
```

---

## When to Use Async

### Use Async Jobs When:

1. **Long-Running Requests** - Tasks that take >10 seconds
2. **Batch Processing** - Process multiple items without blocking
3. **Unreliable Networks** - Client may disconnect, job continues
4. **Background Tasks** - Generate reports, summaries, analysis
5. **High Latency Tolerance** - User doesn't need immediate response
6. **Resource Intensive** - Large token counts, complex reasoning

### Sync vs Async Comparison

| Scenario | Sync | Async |
|----------|------|-------|
| Simple Q&A | ✅ | ❌ |
| Chat interface | ✅ | ❌ |
| Code generation (small) | ✅ | ❌ |
| Long-form content | ❌ | ✅ |
| Document analysis | ❌ | ✅ |
| Report generation | ❌ | ✅ |
| Batch processing | ❌ | ✅ |
| Background tasks | ❌ | ✅ |

---

## Complete Examples

### Example 1: Document Summarization

Summarize a long document without blocking your application.

```python
import requests
import time

API_KEY = "mt_sk_user_abc123"
BASE_URL = "http://localhost:8080"

# Long document to summarize
document = """
Artificial Intelligence (AI) has transformed numerous industries...
[5000 words of content]
"""

# Submit async job
response = requests.post(
    f"{BASE_URL}/v1/async",
    headers={"Authorization": f"Bearer {API_KEY}"},
    json={
        "model": "ollama/mistral:latest",
        "messages": [
            {"role": "system", "content": "You are a document summarizer. Create a concise summary."},
            {"role": "user", "content": f"Summarize this document in 3 paragraphs:\n\n{document}"}
        ],
        "max_tokens": 1000,
        "temperature": 0.3
    }
)

job_id = response.json()["job_id"]
print(f"Job submitted: {job_id}")

# Poll for completion with progress indicator
poll_count = 0
while True:
    status_response = requests.get(
        f"{BASE_URL}/v1/jobs/{job_id}",
        headers={"Authorization": f"Bearer {API_KEY}"}
    ).json()
    
    status = status_response["status"]
    poll_count += 1
    
    if status == "queued":
        print(f"⏳ Queued... (poll #{poll_count})")
    elif status == "running":
        print(f"🔄 Running... (poll #{poll_count})")
    elif status == "completed":
        summary = status_response["result"]["choices"][0]["message"]["content"]
        print(f"✅ Complete! (polls: {poll_count})")
        print("\nSummary:")
        print(summary)
        break
    elif status == "failed":
        error = status_response.get("error", "Unknown error")
        print(f"❌ Failed: {error}")
        break
    
    time.sleep(2)  # Poll every 2 seconds

# Output:
# Job submitted: job_1770736516577382000
# ⏳ Queued... (poll #1)
# 🔄 Running... (poll #2)
# ✅ Complete! (polls: 5)
# 
# Summary:
# This document explores the transformative impact of AI across industries...
```

### Example 2: Batch Code Review

Review multiple code files asynchronously.

```python
import requests
import json
from concurrent.futures import ThreadPoolExecutor

API_KEY = "mt_sk_user_abc123"
BASE_URL = "http://localhost:8080"

# Files to review
code_files = {
    "auth.py": "def authenticate(user, password):...",
    "database.py": "class Database:...",
    "api.py": "@app.route('/api/users'):..."
}

def submit_review(filename, code):
    """Submit a code review job"""
    response = requests.post(
        f"{BASE_URL}/v1/async",
        headers={"Authorization": f"Bearer {API_KEY}"},
        json={
            "model": "ollama/mistral:latest",
            "messages": [
                {"role": "system", "content": "Review this code for bugs, security issues, and improvements. Be concise."},
                {"role": "user", "content": f"Review {filename}:\n```python\n{code}\n```"}
            ],
            "max_tokens": 500
        }
    )
    return filename, response.json()["job_id"]

def check_review_status(job_id):
    """Check if review is complete"""
    response = requests.get(
        f"{BASE_URL}/v1/jobs/{job_id}",
        headers={"Authorization": f"Bearer {API_KEY}"}
    )
    return response.json()

# Submit all reviews
print("Submitting code reviews...")
jobs = {}
for filename, code in code_files.items():
    fname, job_id = submit_review(filename, code)
    jobs[job_id] = fname
    print(f"  {fname}: {job_id}")

# Poll for all completions
print("\nWaiting for reviews...")
results = {}
while jobs:
    for job_id, filename in list(jobs.items()):
        status = check_review_status(job_id)
        
        if status["status"] == "completed":
            review = status["result"]["choices"][0]["message"]["content"]
            results[filename] = review
            print(f"  ✅ {filename} complete")
            del jobs[job_id]
        elif status["status"] == "failed":
            print(f"  ❌ {filename} failed: {status.get('error')}")
            del jobs[job_id]
    
    if jobs:
        time.sleep(1)

# Display results
print("\n" + "="*50)
print("CODE REVIEW RESULTS")
print("="*50)
for filename, review in results.items():
    print(f"\n{filename}:")
    print("-" * 40)
    print(review)
```

### Example 3: Research Report Generation

Generate a comprehensive research report in the background.

```python
import requests
import time
from datetime import datetime

API_KEY = "mt_sk_user_abc123"
BASE_URL = "http://localhost:8080"

def generate_research_report(topic):
    """Generate a research report asynchronously"""
    
    # Submit job
    response = requests.post(
        f"{BASE_URL}/v1/async",
        headers={"Authorization": f"Bearer {API_KEY}"},
        json={
            "model": "ollama/deepseek-r1:latest",
            "messages": [
                {"role": "system", "content": "You are a research assistant. Generate a comprehensive report with introduction, key findings, and conclusion."},
                {"role": "user", "content": f"Generate a research report on: {topic}"}
            ],
            "max_tokens": 4000,
            "temperature": 0.4
        }
    )
    
    job_id = response.json()["job_id"]
    print(f"📄 Report generation started: {job_id}")
    print(f"   Topic: {topic}")
    print(f"   Started: {datetime.now().strftime('%H:%M:%S')}")
    
    return job_id

def wait_for_report(job_id, timeout=300):
    """Wait for report with timeout"""
    start_time = time.time()
    
    while time.time() - start_time < timeout:
        status = requests.get(
            f"{BASE_URL}/v1/jobs/{job_id}",
            headers={"Authorization": f"Bearer {API_KEY}"}
        ).json()
        
        if status["status"] == "completed":
            elapsed = time.time() - start_time
            print(f"✅ Report complete in {elapsed:.1f}s")
            return status["result"]["choices"][0]["message"]["content"]
        
        elif status["status"] == "failed":
            raise Exception(f"Report generation failed: {status.get('error')}")
        
        elif status["status"] == "running":
            elapsed = time.time() - start_time
            print(f"🔄 Still generating... ({elapsed:.0f}s elapsed)", end="\r")
        
        time.sleep(3)
    
    raise TimeoutError(f"Report generation timed out after {timeout}s")

# Generate report
topic = "The impact of quantum computing on cryptography"
job_id = generate_research_report(topic)

# Do other work while report generates...
print("\n💡 You can do other work here while the report generates!")
print("   (Checking email, browsing web, etc.)\n")

# Wait for completion
report = wait_for_report(job_id)

# Save report
filename = f"research_report_{datetime.now().strftime('%Y%m%d_%H%M%S')}.md"
with open(filename, 'w') as f:
    f.write(f"# Research Report: {topic}\n\n")
    f.write(f"Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n\n")
    f.write(report)

print(f"\n💾 Report saved to: {filename}")
```

### Example 4: JavaScript/Node.js Client

```javascript
const axios = require('axios');

const API_KEY = 'mt_sk_user_abc123';
const BASE_URL = 'http://localhost:8080';

class AsyncJobClient {
  constructor(apiKey, baseUrl) {
    this.apiKey = apiKey;
    this.baseUrl = baseUrl;
  }

  async submitJob(model, messages, options = {}) {
    const response = await axios.post(
      `${this.baseUrl}/v1/async`,
      {
        model,
        messages,
        ...options
      },
      {
        headers: { 'Authorization': `Bearer ${this.apiKey}` }
      }
    );
    return response.data.job_id;
  }

  async getJobStatus(jobId) {
    const response = await axios.get(
      `${this.baseUrl}/v1/jobs/${jobId}`,
      {
        headers: { 'Authorization': `Bearer ${this.apiKey}` }
      }
    );
    return response.data;
  }

  async waitForCompletion(jobId, pollInterval = 1000, onProgress) {
    while (true) {
      const status = await this.getJobStatus(jobId);
      
      if (onProgress) {
        onProgress(status);
      }

      if (status.status === 'completed') {
        return status.result;
      }

      if (status.status === 'failed') {
        throw new Error(`Job failed: ${status.error}`);
      }

      await new Promise(resolve => setTimeout(resolve, pollInterval));
    }
  }
}

// Usage
async function main() {
  const client = new AsyncJobClient(API_KEY, BASE_URL);

  console.log('Submitting async job...');
  const jobId = await client.submitJob(
    'ollama/mistral:latest',
    [{ role: 'user', content: 'Write a poem about AI' }],
    { max_tokens: 500 }
  );

  console.log(`Job ID: ${jobId}`);
  console.log('Waiting for completion...\n');

  const result = await client.waitForCompletion(
    jobId,
    1000, // Poll every second
    (status) => {
      console.log(`Status: ${status.status}`);
    }
  );

  console.log('\n✅ Result:');
  console.log(result.choices[0].message.content);
}

main().catch(console.error);
```

### Example 5: cURL with Polling Script

```bash
#!/bin/bash

API_KEY="mt_sk_user_abc123"
BASE_URL="http://localhost:8080"

# Submit async job
echo "Submitting async job..."
RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/async" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "ollama/phi:latest",
    "messages": [{"role": "user", "content": "Explain quantum computing"}],
    "max_tokens": 1000
  }')

JOB_ID=$(echo $RESPONSE | grep -o '"job_id":"[^"]*"' | cut -d'"' -f4)
echo "Job ID: $JOB_ID"
echo ""

# Poll for completion
echo "Polling for results..."
while true; do
  STATUS=$(curl -s "${BASE_URL}/v1/jobs/${JOB_ID}" \
    -H "Authorization: Bearer ${API_KEY}")
  
  JOB_STATUS=$(echo $STATUS | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
  
  if [ "$JOB_STATUS" = "completed" ]; then
    echo ""
    echo "✅ Job completed!"
    echo $STATUS | python3 -m json.tool 2>/dev/null || echo $STATUS
    break
  elif [ "$JOB_STATUS" = "failed" ]; then
    echo ""
    echo "❌ Job failed!"
    echo $STATUS | python3 -m json.tool 2>/dev/null || echo $STATUS
    break
  else
    echo -ne "\r⏳ Status: $JOB_STATUS (polling...)"
  fi
  
  sleep 2
done
```

---

## API Reference

### Submit Async Job

```http
POST /v1/async
```

**Request Body:** Same as `/v1/chat/completions`

```json
{
  "model": "ollama/mistral:latest",
  "messages": [{"role": "user", "content": "Hello"}],
  "max_tokens": 100,
  "temperature": 0.7,
  "stream": false
}
```

**Response:**
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

**Response (queued):**
```json
{
  "job_id": "job_1770736516577382000",
  "status": "queued",
  "created_at": "2026-02-10T15:15:16.577383Z"
}
```

**Response (running):**
```json
{
  "job_id": "job_1770736516577382000",
  "status": "running",
  "created_at": "2026-02-10T15:15:16.577383Z",
  "started_at": "2026-02-10T15:15:17.123456Z"
}
```

**Response (completed):**
```json
{
  "job_id": "job_1770736516577382000",
  "status": "completed",
  "result": {
    "id": "chatcmpl-abc123",
    "object": "chat.completion",
    "created": 1704067200,
    "model": "ollama/mistral:latest",
    "choices": [...],
    "usage": {...}
  },
  "created_at": "2026-02-10T15:15:16.577383Z",
  "started_at": "2026-02-10T15:15:17.123456Z",
  "completed_at": "2026-02-10T15:15:18.789012Z"
}
```

**Response (failed):**
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

---

## Implementation Details

### Job Lifecycle

```
queued → running → completed
                ↘ failed
```

1. **queued**: Job submitted, waiting in queue
2. **running**: Worker picked up job, processing
3. **completed**: Job finished successfully
4. **failed**: Job failed (timeout, error, etc.)

### System Limits

| Parameter | Value | Description |
|-----------|-------|-------------|
| Max queue size | 1000 jobs | Queue full = new jobs wait |
| Workers | 3 | Concurrent job processors |
| Timeout | 120 seconds | Per-job timeout |
| Persistence | None | Jobs lost on restart |

### Performance Characteristics

- **Submission**: <10ms (returns immediately)
- **Queue wait**: Depends on load (typically <5s)
- **Processing time**: Model-dependent
- **Polling overhead**: Minimal

---

## Best Practices

### 1. Polling Strategies

**Conservative (Minimal API calls):**
```python
# Poll every 5 seconds for long jobs
time.sleep(5)
```

**Responsive (Quick feedback):**
```python
# Poll every 1 second for UX
time.sleep(1)
```

**Exponential Backoff:**
```python
delay = 1  # Start with 1 second
while True:
    status = check_status()
    if status["status"] in ["completed", "failed"]:
        break
    time.sleep(delay)
    delay = min(delay * 2, 30)  # Max 30 seconds
```

### 2. Error Handling

```python
try:
    result = wait_for_job(job_id)
except TimeoutError:
    # Job taking too long, notify user
    pass
except Exception as e:
    # Job failed, log error
    logger.error(f"Job failed: {e}")
```

### 3. Resource Management

- Don't poll too aggressively (respect server resources)
- Implement client-side timeouts
- Handle job failures gracefully
- Cache results when possible

### 4. Production Considerations

```python
# Production-ready async client
import requests
import time
import logging
from typing import Optional

logger = logging.getLogger(__name__)

class ProductionAsyncClient:
    def __init__(self, api_key: str, base_url: str):
        self.api_key = api_key
        self.base_url = base_url
        self.session = requests.Session()
        self.session.headers.update({
            "Authorization": f"Bearer {api_key}"
        })
    
    def submit(self, model: str, messages: list, **kwargs) -> str:
        """Submit job with error handling"""
        try:
            resp = self.session.post(
                f"{self.base_url}/v1/async",
                json={"model": model, "messages": messages, **kwargs}
            )
            resp.raise_for_status()
            return resp.json()["job_id"]
        except requests.exceptions.RequestException as e:
            logger.error(f"Failed to submit job: {e}")
            raise
    
    def get_result(self, job_id: str, timeout: int = 300) -> dict:
        """Get result with timeout and retry logic"""
        start = time.time()
        delay = 1
        
        while time.time() - start < timeout:
            try:
                resp = self.session.get(f"{self.base_url}/v1/jobs/{job_id}")
                resp.raise_for_status()
                status = resp.json()
                
                if status["status"] == "completed":
                    return status["result"]
                elif status["status"] == "failed":
                    raise Exception(f"Job failed: {status.get('error')}")
                
                time.sleep(delay)
                delay = min(delay * 1.5, 10)  # Exponential backoff
                
            except requests.exceptions.RequestException as e:
                logger.warning(f"Polling error (retrying): {e}")
                time.sleep(2)
        
        raise TimeoutError(f"Job {job_id} timeout after {timeout}s")
```

---

## Error Handling

### Common Errors

**Job Not Found (404):**
```json
{
  "error": {
    "message": "job not found",
    "type": "not_found",
    "code": 404
  }
}
```

**Rate Limited (429):**
```json
{
  "error": {
    "message": "rate limit exceeded",
    "type": "rate_limit_error",
    "code": 429
  }
}
```

**Authentication Error (401):**
```json
{
  "error": {
    "message": "invalid API key",
    "type": "authentication_error",
    "code": 401
  }
}
```

**Upstream Timeout:**
```json
{
  "job_id": "job_123",
  "status": "failed",
  "error": "upstream timeout after 120s"
}
```

---

## Advanced Usage

### Combining with Intent Routing

```python
# Use intent routing with async jobs
response = requests.post(
    f"{BASE_URL}/v1/async",
    headers={
        "Authorization": f"Bearer {API_KEY}",
        "X-Model-Intent": "plan"  # Routes to deepseek-r1
    },
    json={
        "model": "auto",
        "messages": [{"role": "user", "content": "Create a project plan"}],
        "max_tokens": 4000
    }
)
```

### WebSocket Notifications (Future)

```python
# Coming soon: WebSocket notifications instead of polling
import websocket

def on_message(ws, message):
    data = json.loads(message)
    if data["job_id"] == my_job_id and data["status"] == "completed":
        print("Job complete!")
        ws.close()

ws = websocket.WebSocketApp(
    "ws://localhost:8080/ws/jobs",
    on_message=on_message
)
ws.run_forever()
```

---

## Future Enhancements

- [ ] **Webhook callbacks** - POST to your URL on completion
- [ ] **Job cancellation** - DELETE /v1/jobs/{id}
- [ ] **Persistent storage** - Survive restarts
- [ ] **Retry logic** - Auto-retry failed jobs
- [ ] **Job priority** - High/medium/low priority queues
- [ ] **Batch submission** - Submit multiple jobs at once
- [ ] **WebSocket notifications** - Real-time status updates

---

## See Also

- [API Reference](api.md) - Complete API documentation
- [Intent Routing](INTENT_ROUTING.md) - Smart model selection
- [CLI Reference](cli.md) - Command-line tools
