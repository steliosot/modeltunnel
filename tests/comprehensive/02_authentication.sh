#!/bin/bash
source tests/comprehensive/test_config.sh

echo -e "\n## Authentication\n" >> tests/comprehensive/REPORT.md

echo "Testing API key authentication..." >> tests/comprehensive/REPORT.md

KEY_RESPONSE=$(./build/modeltunnel key create test --models mistral --rate 100/min 2>&1)
echo $KEY_RESPONSE | grep -q "Key created successfully"
echo "Test key created: $?"

echo -e "\nTest unauthorized request rejection..." >> tests/comprehensive/REPORT.md
STATUS_CODE=$(curl -s -o /dev/null -w "%{http_code}" tests/comprehensive/SERVER_URL/v1/chat/completions -H "Content-Type: application/json" -d '{"model": "ollama/mistral:latest", "messages": [{"role": "user", "content": "Hi"}]}')
[ "$STATUS_CODE" = "401" ]

echo "Authentication tests complete." >> tests/comprehensive/REPORT.md
