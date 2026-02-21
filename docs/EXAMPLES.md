# Examples Index

Complete list of examples and code samples available in Modeltunnel documentation.

## Async Jobs Examples

Location: [docs/ASYNC_JOBS.md](docs/ASYNC_JOBS.md)

### 1. Document Summarization (Python)
Submit a long document for summarization and poll for results with progress indicators.
- Submit job with large document
- Poll with status messages
- Handle completion and errors
- ~100 lines of code

### 2. Batch Code Review (Python)
Review multiple code files asynchronously with concurrent processing.
- Submit multiple review jobs
- Track multiple job IDs
- Parallel status checking
- Aggregate results
- ~80 lines of code

### 3. Research Report Generation (Python)
Generate comprehensive research reports in the background.
- Submit research job
- Continue with other work
- Timeout handling
- Save results to file
- ~70 lines of code

### 4. JavaScript/Node.js Client
Production-ready async client for JavaScript applications.
- AsyncJobClient class
- Promise-based API
- Error handling
- Progress callbacks
- ~80 lines of code

### 5. Bash Script with Polling
Pure shell script for async job submission and polling.
- cURL-based submission
- JSON parsing with grep
- Status polling loop
- No dependencies
- ~50 lines of code

**Use Cases Covered:**
- ✅ Long-running tasks
- ✅ Batch processing
- ✅ Background jobs
- ✅ Document analysis
- ✅ Code review workflows

---

## Intent Routing Examples

Location: [docs/INTENT_ROUTING.md](docs/INTENT_ROUTING.md)

### 1. Smart Application Router (Python)
Automatic intent detection based on message content.
- Keyword-based intent detection
- Route to appropriate model
- Show routing decisions
- Performance timing
- ~70 lines of code

### 2. IDE Assistant (Python)
IDE plugin with different intents for different tasks.
- Explain code (plan intent)
- Generate code (code intent)
- Quick suggestions (chat intent)
- Multiple use cases
- ~60 lines of code

### 3. Customer Support Bot (Python)
Support chatbot that classifies and routes messages.
- Intent classification
- Conversation history
- Context-aware routing
- Multi-turn conversations
- ~80 lines of code

### 4. JavaScript Web App (JavaScript)
Web application using intent routing.
- ModeltunnelClient class
- Async/await patterns
- Different intents per method
- Browser-compatible
- ~50 lines of code

### 5. Content Generation Pipeline (Python)
Multi-step content creation using different intents.
- Outline generation (plan)
- Code examples (code)
- Introduction writing (chat)
- Combine results
- ~70 lines of code

**Intents Covered:**
- ✅ Plan - Architecture, strategy, reasoning
- ✅ Code - Programming, debugging, technical
- ✅ Chat - Conversation, Q&A, support

---

## Docker Deployment Examples

Location: Installation guide and [docs/DOCKER_IMAGE_PUSH.md](docs/DOCKER_IMAGE_PUSH.md)

### Quick Docker Start
```bash
# Pull pre-built image
docker pull ghcr.io/steliosot/modeltunnel:latest

# Run with Ollama
docker run -d -p 8080:8080 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  ghcr.io/steliosot/modeltunnel:latest up

# Health check
curl http://localhost:8080/health
```

### Custom Configuration
```bash
# Mount config file
docker run -d -p 8080:8080 \
  -v /path/to/config.yaml:/home/appuser/.config/modeltunnel/config.yaml \
  ghcr.io/steliosot/modeltunnel:latest up
```

### Multi-Container Setup
```bash
# Primary service
docker run -d --name modeltunnel \
  -p 8080:8080 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  ghcr.io/steliosot/modeltunnel:latest up

# Secondary service with tunnel
docker run -d --name modeltunnel-tunnel \
  -p 8081:8080 \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  ghcr.io/steliosot/modeltunnel:latest up --tunnel
```

---

## API Usage Examples

Location: [docs/api.md](docs/api.md)

### Basic Examples
- Simple chat completion
- Streaming responses
- Model listing
- Health check

### Advanced Examples
- Async job submission
- Job status polling
- Intent routing with headers
- Combining async + intent routing
- Error handling

### Language Examples
- **Python** - Using OpenAI library
- **JavaScript** - Using fetch API
- **cURL** - Command-line examples
- **Bash** - Shell scripts

---

## CLI Usage Examples

Location: [docs/cli.md](docs/cli.md)

### Common Workflows
- Development setup
- Production deployment
- Team/student access
- Key rotation

### Command Examples
- Server startup options
- Key creation with policies
- Key listing and filtering
- Key revocation

---

## Configuration Examples

Location: [docs/configuration.md](docs/configuration.md)

### Configuration Files
- Basic configuration
- Multiple upstreams
- Per-model rate limits
- Intent routing config
- Async job settings

### Production Configs
- Secure defaults
- User policies
- Cost optimization
- Performance tuning

---

## Quick Start Examples

### Minimal Async Example
```python
import requests
import time

# Submit
resp = requests.post("http://localhost:8080/v1/async",
    headers={"Authorization": "Bearer KEY"},
    json={"model": "ollama/phi:latest", "messages": [{"role": "user", "content": "Hi"}]})
job_id = resp.json()["job_id"]

# Poll
while True:
    status = requests.get(f"http://localhost:8080/v1/jobs/{job_id}",
        headers={"Authorization": "Bearer KEY"}).json()
    if status["status"] == "completed":
        print(status["result"]["choices"][0]["message"]["content"])
        break
    time.sleep(1)
```

### Minimal Intent Example
```python
from openai import OpenAI

client = OpenAI(base_url="http://localhost:8080/v1", api_key="KEY")

response = client.chat.completions.create(
    model="auto",
    messages=[{"role": "user", "content": "Write a function"}],
    extra_headers={"X-Model-Intent": "code"}
)

print(f"Routed to: {response.model}")
print(response.choices[0].message.content)
```

---

## Total Examples

- **827 lines** - Async Jobs documentation with examples
- **858 lines** - Intent Routing documentation with examples
- **690 lines** - API reference with examples
- **454 lines** - CLI reference with examples
- **461 lines** - Configuration guide with examples
- **288 lines** - Installation guide

**Total: 3,578+ lines of documentation with comprehensive examples!**
(+ Docker deployment instructions)

### By Use Case
- **Background tasks** → [ASYNC_JOBS.md](docs/ASYNC_JOBS.md)
- **Smart routing** → [INTENT_ROUTING.md](docs/INTENT_ROUTING.md)
- **API integration** → [api.md](docs/api.md)
- **CLI automation** → [cli.md](docs/cli.md)
- **Configuration** → [configuration.md](docs/configuration.md)

### By Language
- **Python** - All docs have Python examples
- **JavaScript/Node.js** - ASYNC_JOBS.md, INTENT_ROUTING.md, api.md
- **Bash/Shell** - ASYNC_JOBS.md, cli.md
- **cURL** - api.md

### By Complexity
- **Beginner** - README quick start, api.md basic examples
- **Intermediate** - ASYNC_JOBS.md simple polling, INTENT_ROUTING.md basic routing
- **Advanced** - Batch processing, multi-intent pipelines, production configurations

---

## Running Examples

All Python examples require:
```bash
pip install openai requests
```

JavaScript examples require:
```bash
npm install axios
# or
yarn add axios
```

Make sure Modeltunnel server is running:
```bash
modeltunnel up
```

---

## Contributing Examples

Have a great example? Consider contributing:
1. Fork the repository
2. Add your example to the appropriate docs file
3. Submit a pull request

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
