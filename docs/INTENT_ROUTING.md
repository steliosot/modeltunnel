# Intent-Based Model Routing

Modeltunnel's intent-based routing automatically selects the best model for your task. Instead of manually choosing models, send an `X-Model-Intent` header and let the system optimize for your use case.

## Table of Contents

- [Quick Start](#quick-start)
- [Understanding Intents](#understanding-intents)
- [Detailed Use Cases](#detailed-use-cases)
- [Complete Examples](#complete-examples)
- [Configuration](#configuration)
- [How It Works](#how-it-works)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

Add the `X-Model-Intent` header to your request:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-Model-Intent: plan" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Create a project roadmap"}]
  }'
```

**Response Headers:**
```
X-Routed-Model: deepseek-r1:latest
X-Model-Intent: plan
```

---

## Understanding Intents

### Intent Comparison

| Intent | Best For | Routes To | Temperature | Max Tokens | Speed |
|--------|----------|-----------|-------------|------------|-------|
| **plan** | Strategy, architecture, reasoning | deepseek-r1 → qwen2.5 → mistral | 0.3 (focused) | 4000 | Slow |
| **code** | Programming, debugging, technical | qwen2.5 → mistral → phi | 0.2 (precise) | 2000 | Medium |
| **chat** | Conversation, Q&A, support | phi → tinyllama → mistral | 0.7 (creative) | 1000 | Fast |

### Model Priority Lists

**Plan Intent Priority:**
1. `deepseek-r1:latest` - Best reasoning, slowest
2. `qwen2.5:latest` - Good reasoning, medium speed
3. `mistral:latest` - Fallback, fast

**Code Intent Priority:**
1. `qwen2.5:latest` - Excellent for code
2. `mistral:latest` - Good code performance
3. `phi:latest` - Fast, decent code

**Chat Intent Priority:**
1. `phi:latest` - Fast, conversational
2. `tinyllama:latest` - Very fast, lightweight
3. `mistral:latest` - Fallback

---

## Detailed Use Cases

### Plan Intent

**Best for:** Strategic planning, architecture, roadmapping, reasoning, analysis

**Use Cases:**
- 🏗️ System architecture design
- 📋 Project planning and roadmaps
- 🎯 Strategy development
- 🧮 Complex problem solving
- 📊 Data analysis planning
- 🔍 Research methodology

**Examples:**

```bash
# Architecture planning
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: plan" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Design a microservices architecture for an e-commerce platform"}]
  }'

# Project roadmap
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: plan" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Create a 6-month roadmap for launching a mobile app"}]
  }'

# Strategic analysis
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: plan" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Analyze the competitive landscape for AI writing tools"}]
  }'
```

### Code Intent

**Best for:** Programming, debugging, technical questions, code review

**Use Cases:**
- 💻 Writing functions and classes
- 🐛 Debugging errors
- 📖 Code review and optimization
- 🔧 Technical troubleshooting
- 📚 API documentation
- 🧪 Test generation

**Examples:**

```bash
# Write a function
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: code" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Write a Python function to validate email addresses using regex"}]
  }'

# Debug code
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: code" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Debug this: def foo(x): return x + y"}]
  }'

# Code review
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: code" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "Review this code for security issues: [code]"}]
  }'
```

### Chat Intent

**Best for:** General conversation, Q&A, customer support, quick responses

**Use Cases:**
- 💬 General questions
- 🎧 Customer support
- ⚡ Quick information lookup
- 🎯 Casual conversation
- 📱 Chatbot responses
- ❓ FAQ handling

**Examples:**

```bash
# General question
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: chat" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "What's the weather like?"}]
  }'

# Customer support
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: chat" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "How do I reset my password?"}]
  }'

# Quick Q&A
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: chat" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "What is the capital of France?"}]
  }'
```

---

## Complete Examples

### Example 1: Smart Application Router

```python
from openai import OpenAI
import time

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="mt_sk_user_abc123"
)

def smart_chat(user_message: str, context: str = None) -> str:
    """
    Automatically route to the best model based on message content.
    Uses intent detection keywords.
    """
    
    # Detect intent from message content
    message_lower = user_message.lower()
    
    # Plan keywords
    if any(kw in message_lower for kw in ['plan', 'roadmap', 'architecture', 'strategy', 'design', 'analyze']):
        intent = "plan"
        print("🧠 Routing to planning model (deepseek-r1)...")
    
    # Code keywords  
    elif any(kw in message_lower for kw in ['code', 'function', 'bug', 'error', 'debug', 'python', 'javascript']):
        intent = "code"
        print("💻 Routing to coding model (qwen2.5)...")
    
    # Default to chat
    else:
        intent = "chat"
        print("💬 Routing to chat model (phi)...")
    
    # Build messages
    messages = []
    if context:
        messages.append({"role": "system", "content": context})
    messages.append({"role": "user", "content": user_message})
    
    # Make request with intent
    start_time = time.time()
    response = client.chat.completions.create(
        model="auto",
        messages=messages,
        extra_headers={"X-Model-Intent": intent}
    )
    elapsed = time.time() - start_time
    
    # Show routing info
    routed_model = response.model
    print(f"✅ Used: {routed_model} ({elapsed:.2f}s)")
    
    return response.choices[0].message.content

# Test different types of requests
print("="*60)
print("Test 1: Planning")
result = smart_chat("Create a project plan for building a SaaS application")
print(f"Response: {result[:100]}...\n")

print("="*60)
print("Test 2: Coding")
result = smart_chat("Write a function to calculate fibonacci numbers")
print(f"Response: {result[:100]}...\n")

print("="*60)
print("Test 3: Chat")
result = smart_chat("What's your favorite color?")
print(f"Response: {result[:100]}...\n")
```

### Example 2: IDE Assistant with Intent

```python
from openai import OpenAI

class IDEAssistant:
    def __init__(self, api_key: str, base_url: str = "http://localhost:8080/v1"):
        self.client = OpenAI(base_url=base_url, api_key=api_key)
    
    def explain_code(self, code: str, language: str = "python") -> str:
        """Explain what code does - uses plan intent for thoroughness"""
        response = self.client.chat.completions.create(
            model="auto",
            messages=[{
                "role": "user",
                "content": f"Explain this {language} code in detail:\n```\n{code}\n```"
            }],
            extra_headers={"X-Model-Intent": "plan"}
        )
        return response.choices[0].message.content
    
    def generate_code(self, description: str, language: str = "python") -> str:
        """Generate code - uses code intent for precision"""
        response = self.client.chat.completions.create(
            model="auto",
            messages=[{
                "role": "user",
                "content": f"Write {language} code for: {description}"
            }],
            extra_headers={"X-Model-Intent": "code"}
        )
        return response.choices[0].message.content
    
    def quick_suggestion(self, prompt: str) -> str:
        """Quick suggestion - uses chat intent for speed"""
        response = self.client.chat.completions.create(
            model="auto",
            messages=[{"role": "user", "content": prompt}],
            extra_headers={"X-Model-Intent": "chat"}
        )
        return response.choices[0].message.content

# Usage
assistant = IDEAssistant("mt_sk_user_abc123")

# Different intents for different tasks
code = """
def process_data(data):
    results = []
    for item in data:
        if item.value > 10:
            results.append(item.transform())
    return results
"""

print("Explaining code (plan intent)...")
explanation = assistant.explain_code(code)
print(explanation[:200] + "...\n")

print("Generating code (code intent)...")
generated = assistant.generate_code("a function to download files from URLs")
print(generated[:200] + "...\n")

print("Quick suggestion (chat intent)...")
suggestion = assistant.quick_suggestion("Name for a data processing function")
print(suggestion)
```

### Example 3: Customer Support Bot

```python
from openai import OpenAI
from typing import List, Dict

class SupportBot:
    def __init__(self, api_key: str):
        self.client = OpenAI(
            base_url="http://localhost:8080/v1",
            api_key=api_key
        )
        self.conversation_history: List[Dict] = []
    
    def handle_message(self, user_message: str) -> str:
        """Handle support message with appropriate intent"""
        
        # Classify intent based on message
        intent = self._classify_intent(user_message)
        
        # Add to history
        self.conversation_history.append({
            "role": "user",
            "content": user_message
        })
        
        # Generate response
        response = self.client.chat.completions.create(
            model="auto",
            messages=self.conversation_history,
            extra_headers={"X-Model-Intent": intent},
            max_tokens=500
        )
        
        reply = response.choices[0].message.content
        
        # Add to history
        self.conversation_history.append({
            "role": "assistant",
            "content": reply
        })
        
        return reply
    
    def _classify_intent(self, message: str) -> str:
        """Classify message intent"""
        msg_lower = message.lower()
        
        # Technical troubleshooting
        if any(kw in msg_lower for kw in ['error', 'bug', 'not working', 'failed', 'crash']):
            return "code"
        
        # Complex questions
        elif any(kw in msg_lower for kw in ['how do i', 'guide', 'tutorial', 'setup']):
            return "plan"
        
        # Simple questions
        else:
            return "chat"

# Usage
bot = SupportBot("mt_sk_user_abc123")

# Test different types of support requests
messages = [
    "Hi there!",  # chat
    "I'm getting an error when I try to login",  # code
    "How do I set up two-factor authentication?",  # plan
    "Thanks for your help!"  # chat
]

for msg in messages:
    print(f"User: {msg}")
    response = bot.handle_message(msg)
    print(f"Bot: {response}\n")
```

### Example 4: JavaScript Web App

```javascript
class ModeltunnelClient {
  constructor(apiKey, baseUrl = 'http://localhost:8080/v1') {
    this.apiKey = apiKey;
    this.baseUrl = baseUrl;
  }

  async chat(message, intent = 'chat') {
    const response = await fetch(`${this.baseUrl}/chat/completions`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.apiKey}`,
        'Content-Type': 'application/json',
        'X-Model-Intent': intent
      },
      body: JSON.stringify({
        model: 'auto',
        messages: [{ role: 'user', content: message }],
        max_tokens: 1000
      })
    });

    const data = await response.json();
    
    // Log routing info
    console.log(`Routed to: ${data.model}`);
    console.log(`Intent: ${intent}`);
    
    return data.choices[0].message.content;
  }

  async plan(message) {
    return this.chat(message, 'plan');
  }

  async code(message) {
    return this.chat(message, 'code');
  }
}

// Usage
const client = new ModeltunnelClient('mt_sk_user_abc123');

// Planning
const architecture = await client.plan(
  'Design a scalable notification system'
);

// Coding
const code = await client.code(
  'Write a React component for a todo list'
);

// Chat
const response = await client.chat(
  'What are your capabilities?'
);
```

### Example 5: Content Generation Pipeline

```python
from openai import OpenAI
import json

class ContentPipeline:
    def __init__(self, api_key: str):
        self.client = OpenAI(
            base_url="http://localhost:8080/v1",
            api_key=api_key
        )
    
    def generate_blog_post(self, topic: str) -> dict:
        """Generate a complete blog post with multiple intents"""
        
        # Step 1: Plan the structure (plan intent)
        print("📋 Planning blog structure...")
        plan_response = self.client.chat.completions.create(
            model="auto",
            messages=[{
                "role": "user",
                "content": f"Create a detailed outline for a blog post about: {topic}"
            }],
            extra_headers={"X-Model-Intent": "plan"}
        )
        outline = plan_response.choices[0].message.content
        
        # Step 2: Write the code examples (code intent)
        print("💻 Writing code examples...")
        code_response = self.client.chat.completions.create(
            model="auto",
            messages=[{
                "role": "user",
                "content": f"Write code examples for: {topic}\nBased on this outline:\n{outline}"
            }],
            extra_headers={"X-Model-Intent": "code"}
        )
        code_examples = code_response.choices[0].message.content
        
        # Step 3: Write conversational intro/outro (chat intent)
        print("💬 Writing introduction...")
        intro_response = self.client.chat.completions.create(
            model="auto",
            messages=[{
                "role": "user",
                "content": f"Write an engaging introduction for a blog about: {topic}"
            }],
            extra_headers={"X-Model-Intent": "chat"}
        )
        introduction = intro_response.choices[0].message.content
        
        # Combine everything
        blog_post = f"""{introduction}

## Outline
{outline}

## Code Examples
{code_examples}

---
*Generated using Modeltunnel with intelligent intent routing*
"""
        
        return {
            "topic": topic,
            "content": blog_post,
            "outline": outline,
            "code_examples": code_examples,
            "introduction": introduction
        }

# Usage
pipeline = ContentPipeline("mt_sk_user_abc123")
post = pipeline.generate_blog_post("Introduction to Machine Learning")

print("\n" + "="*60)
print(f"Blog Post: {post['topic']}")
print("="*60)
print(post['content'][:1000] + "...")
```

---

## Configuration

### Default Configuration

```yaml
# config.yaml
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
```

### Custom Intents

You can add custom intents in your configuration:

```yaml
intents:
  creative:
    priority:
      - mistral:latest
      - qwen2.5:latest
    temperature: 0.9
    max_tokens: 2000
  
  summarize:
    priority:
      - phi:latest
      - mistral:latest
    temperature: 0.1
    max_tokens: 500
```

Usage:
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "X-Model-Intent: creative" \
  -d '{"model": "auto", "messages": [...]}'
```

---

## How It Works

### 1. Intent Detection

```
Request arrives with X-Model-Intent: plan
```

### 2. Model Selection

```
Check available models:
  ✓ deepseek-r1:latest available? → YES → Use it
  ✗ qwen2.5:latest available? → NO
  ✗ mistral:latest available? → (not checked, already found)
```

### 3. Parameter Application

```
Apply intent-specific parameters:
  Temperature: 0.3 (from plan config)
  Max tokens: 4000 (from plan config)
```

### 4. Request Processing

```
Forward to selected model with applied parameters
```

### 5. Response Headers

```
X-Routed-Model: deepseek-r1:latest
X-Model-Intent: plan
```

### Fallback Behavior

If the preferred model is unavailable:

1. Try next model in priority list
2. Continue until available model found
3. If none available, return error

Example:
```
Intent: plan
Priority: [deepseek-r1, qwen2.5, mistral]

Available models: [mistral, phi]
Selected: mistral (first available in priority list)
```

---

## Best Practices

### 1. Choose the Right Intent

**Use `plan` when:**
- You need thorough analysis
- Quality is more important than speed
- Complex reasoning required
- One-time important task

**Use `code` when:**
- Writing or debugging code
- Technical accuracy matters
- Syntax needs to be correct
- Programming-related tasks

**Use `chat` when:**
- Speed matters most
- Simple Q&A
- Conversational interface
- High volume of requests

### 2. Override When Needed

```python
# Default to auto-routing
response = client.chat.completions.create(
    model="auto",
    messages=[...]
)

# Override for specific use case
response = client.chat.completions.create(
    model="ollama/mistral:latest",  # Force specific model
    messages=[...]
)
```

### 3. Monitor Routing

```python
response = client.chat.completions.create(
    model="auto",
    messages=[...],
    extra_headers={"X-Model-Intent": "plan"}
)

print(f"Routed to: {response.model}")
print(f"Tokens used: {response.usage.total_tokens}")
```

### 4. Combine with Rate Limiting

Different intents may have different rate limits:

```yaml
policies:
  default:
    rate_limit: 60/min
    models:
      deepseek-r1:latest:
        rate_limit: 5/min   # Expensive model, limit it
      phi:latest:
        rate_limit: 100/min  # Cheap model, allow more
```

### 5. Cost Optimization

```python
# For cost-sensitive applications
def cost_optimized_chat(message: str) -> str:
    # Try cheap model first
    response = client.chat.completions.create(
        model="ollama/phi:latest",
        messages=[{"role": "user", "content": message}],
        max_tokens=500
    )
    
    # If response seems low quality, retry with better model
    content = response.choices[0].message.content
    if is_low_quality(content):
        response = client.chat.completions.create(
            model="auto",
            messages=[{"role": "user", "content": message}],
            extra_headers={"X-Model-Intent": "plan"}
        )
        content = response.choices[0].message.content
    
    return content
```

---

## Troubleshooting

### Intent Not Working

**Problem:** Model not routing as expected

**Check:**
```bash
# Verify model availability
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer KEY"

# Check response headers
curl -v http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer KEY" \
  -H "X-Model-Intent: plan" \
  -d '{"model": "auto", "messages": [...]}' 2>&1 | grep "X-"
```

### Wrong Model Selected

**Problem:** Expected deepseek-r1 but got mistral

**Cause:** deepseek-r1 not available

**Solution:** Pull the model:
```bash
ollama pull deepseek-r1
```

### Performance Issues

**Problem:** Plan intent too slow

**Options:**
1. Use `code` or `chat` intent instead
2. Force faster model: `"model": "ollama/phi:latest"`
3. Reduce `max_tokens`

### Debugging Routing

```python
import logging

# Enable debug logging
logging.basicConfig(level=logging.DEBUG)

# Make request
response = client.chat.completions.create(
    model="auto",
    messages=[...],
    extra_headers={"X-Model-Intent": "plan"}
)

# Check what was routed
print(f"Requested intent: plan")
print(f"Routed model: {response.model}")
```

---

## Future Enhancements

Coming soon:
- [ ] **Automatic Intent Detection** - Detect intent from prompt content
- [ ] **Custom Intent Configuration** - Define your own intents
- [ ] **Intent-Based Rate Limiting** - Different limits per intent
- [ ] **Intent Analytics** - Track which intents are used most
- [ ] **Multi-Intent Requests** - Combine multiple intents

---

## See Also

- [API Reference](api.md) - Complete API documentation
- [Async Jobs](ASYNC_JOBS.md) - Asynchronous processing
- [CLI Reference](cli.md) - Command-line tools
- [Configuration](configuration.md) - Server configuration
