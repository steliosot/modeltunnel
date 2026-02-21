import os
from openai import OpenAI

client = OpenAI(
    base_url=os.environ.get("MODELTUNNEL_URL", "http://localhost:8080/v1"),
    api_key=os.environ.get("MODELTUNNEL_API_KEY", "your-api-key-here")
)

# List available models
print("Available models:")
models = client.models.list()
for model in models.data:
    print(f"  - {model.id}")
print()

# Chat completion
print("Testing chat completion...")
response = client.chat.completions.create(
    model="ollama/mistral:latest",
    messages=[{"role": "user", "content": "Hello!"}]
)

print(f"Response: {response.choices[0].message.content}")
print()

# Streaming response
print("Testing streaming...")
for chunk in client.chat.completions.create(
    model="ollama/mistral:latest",
    messages=[{"role": "user", "content": "Tell me a story"}],
    stream=True
):
    if chunk.choices and chunk.choices[0].delta and chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
print()
