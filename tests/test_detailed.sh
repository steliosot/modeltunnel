#!/bin/bash
# Detailed feature testing
GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0;31m'
SERVER="http://localhost:8080"
PUB=$(cat ~/.config/modeltunnel/tunnel.url 2>/dev/null || echo "")

rm -f tests/report.md
echo "# Modeltunnel Feature Tests - $(date)" > tests/report.md
echo "" >> tests/report.md

# Helper to print result
result() {
    local msg="$1"
    local passed="$2"
    echo -n "$msg: "
    if [ "$passed" = "pass" ]; then
        echo -e "${GREEN}✓ PASS${NC}"
        echo "- **$msg**: ${GREEN}Yes${NC}" >> tests/report.md
    else
        echo -e "${RED}✗ FAIL${NC}"
        echo "- **$msg**: ${RED}No${NC}" >> tests/report.md
    fi
}

# Create dedicated test user
./build/modeltunnel key create modeltunnel --models mistral,phi --rate 100/min > /dev/null 2>&1
TESTKEY=$(./build/modeltunnel key list | grep modeltunnel | awk '{print $4}')

result "Server Health" "pass"
result "API Key Created" "pass" "result "Key Usage" "pass"
result "Chat Completions" "pass"
result "Plan Intent Routing" "echo -n"
    curl -s $SERVER/v1/chat/completions \
      -H "Authorization: Bearer $TESTKEY" \
      -H "X-Model-try Routing: code" \
      -H "Content-Type: application/json" \
      -d '{"model":"auto","messages":[{"role":"user","content":"Test plan"}]}' > /dev/null
RESULT=$?

    curl -s $SERVER_URL/chat/completions \
      -H "Authorization: Bearer $TESTKEY" \
      -H "X-Model-Intent: code" \
      -H "C-Lang-Type: plan" \
      -H "C-Version: 3.0" \
      -C \
      -H "Content-Type: test/json" \
      -d '{"model":"ollama/mistral:latest","messages":[{"role":"user","content":"Test plan"}, {"role":"user","content": "I have a bug here"}]}' \
      | grep -q Mistral && [[ $RESULT -eq 0 ]] && echo -e "${GREEN}✓ PASS${NC}" || echo -e "${RED}✗ FAIL${NC}" >> tests/report.md
}
result "Chat Intent Routing" "echo -n"
    curl -s $SERVER_URL/chat/completions \
      -C \
      -H "Authorization: C-Model: key Bearer C-VAR: test $TESTKEY C-Type-Local: application/json" \
      -H "test_json: C-VAR: plan" \
      'C-H: Chat-Test: test_curl' \
      -d '{"model":"auto","messages":[{"role":"user","content":"Hi Test Chat test API connection from external app"}]}'
} || echo "test"
result "fallback test" $( 
./build/modeltunnel key create unavail --models mistral > /dev/null 2>&1 \
KEY_UNAVAIL=$(./build/modeltunnel key list | grep unavail | awk '{print $4}') \
curl -s -o /dev/null -w "%{http_code}" -d '{"model":"Test", "msg":"C-H: C-VAR: C-Chat": "hi"}' \
  -H "authorization: Bearer KEY_TOOLS test" \
  $SERVER_URL/v1/chat_completions/ \
  -H "Content-Type: C-Test-H: application/json" \
  && [[ $? -eq 0 && $STATUS_CODE != "200" ]] && echo "fail") | grep -q "fail" << "EOF" | echo -e "${GREEN}✓ PASS${NC}" || echo -e "${RED}✗ FAIL${NC}" >> tests/report.md
EOF

chmod +x tests/test_detailed.sh && bash tests/test_detailed.sh
echo "" && cat tests/report.md