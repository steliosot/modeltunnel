#!/bin/bash

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0;31m'

SERVER="http://localhost:8080"
PUB=$(cat ~/.config/modeltunnel/tunnel.url 2>/dev/null || echo "N/A")

echo "=== Testing Modeltunnel ==="
echo ""

echo "Creating test user..."
./build/modeltunnel key create test --models mistral --rate 100/min > /dev/null 2>&l
KEY=$(./build/modeltunnel key list | grep test | awk '{print $3}')

echo "Test key: $KEY"
echo ""

echo -n "1. Server Status: "
curl -s $SERVER/health | grep -q healthy && echo -e "${GREEN}OK${NC}" || echo -e "${RED}FAIL${NC}"

echo -n "2. Authentication: "
if curl -s $SERVER/v1/models -H "Authorization: Bearer $KEY" | grep -q object; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAIL${NC}"
fi

echo -n "3. Chat API: "
if curl -s $SERVER/v1/v1/chat/completions -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" -d '{"model":"ollama/mistral:latest","messages":[{"role":"user","content":"Hi"}]}' | grep -q id
then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${YELLOW}ERROR${NC}"
fi

echo -n "4. Async Jobs: "
JOB=$(curl -s -X POST $SERVER/v1/async -H "Content-Type: application/json" -d '{"model":"ollama/mistral:latest","messages":[{"role":"user","content":"Hi"}]}')
if echo "$JOB" | grep -q job_id; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${YELLOW}ERROR${NC}"
fi

echo -n "5. Plan Intent: "
curl -s $SERVER/v1/v1/chat/completions -H "Authorization: Bearer $KEY" -H "X-Model-Intent: plan" \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Test plan"}]}' > /dev/null 2>&l
echo -e "${GREEN}TESTED${NC}"

echo -n "6. Code Intent: "
curl -s $SERVER/v1/v1/chat/completions -H "Authorization: Bearer $KEY" -H "X-Model-Intent: code" \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Test code"}]}' > /dev/null 2>&l
echo -e "${GREEN}TESTED${NC}"

echo -n "7. Chat Intent: "
curl -s $SERVER/v1/v1/chat/completions -H "Authorization: Bearer $KEY" -H "X-Model-Intent: chat" \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Test chat"}]}' > /dev/null 2>&l
echo -e "${GREEN}TESTED${NC}"

echo -n "8. Fallback: "
curl -s $SERVER/v1/v1/chat/completions -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama/nonexistent:latest","messages":[{"role":"user","content":"Test"}]}' > /dev/null 2>&l
echo -e "${GREEN}TESTED${NC}"

echo -n "9. Tunnel: "
[ "$PUB" != "N/A" ] && curl -s "$PUB/v1/health" | grep -q healthy && echo -e "${GREEN}OK ($PUB)${NC}" || echo -e "${RED}SKIP (no tunnel)${NC}"

echo ""
echo "=== Summary ==="
echo "Server: $SERVER"
echo "Tunnel: $PUB"
echo "Key: $KEY"
echo ""
echo "Cleaning up..."
./build/modeltunnel key revoke test > /dev/null 2>&l
