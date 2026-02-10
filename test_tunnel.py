from openai import OpenAI
import sys

client = OpenAI(
    base_url="https://introduced-mpg-evaluations-soldier.trycloudflare.com/v1",
    api_key="mt_sk_admin_0043aea2fd65c19a7b31a132104584df47df6cc5f0f632d57b822766c3fd57a1"
)

def test_chat():
    print("Testing chat with ollama/mistral:latest...")
    try:
        response = client.chat.completions.create(
            model="ollama/mistral:latest",  # or try "default/mistral:latest"
            messages=[{"role": "user", "content": "Hello! What model are you?"}]
        )
        print(f"✅ Success: {response.choices[0].message.content}")
    except Exception as e:
        print(f"❌ Error: {e}")

def test_streaming():
    print("\nTesting streaming...")
    try:
        for chunk in client.chat.completions.create(
            model="ollama/mistral:latest",
            messages=[{"role": "user", "content": "Count to 5"}],
            stream=True
        ):
            if chunk.choices[0].delta.content:
                print(chunk.choices[0].delta.content, end="", flush=True)
        print("\n✅ Streaming complete")
    except Exception as e:
        print(f"\n❌ Error: {e}")

if __name__ == "__main__":
    test_chat()
    test_streaming()

