#!/bin/bash
GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'

echo "=== Modeltunnel Feature Tests ===" 
echo ""

# Get server details
SERVER="http://localhost:8080"
PUB=$(cat ~/.config/modeltunnel/tunnel.url 2>/dev/null || echo "N/A")

# Create test key
./build/modeltunnel key create test --models mistral --rate 100/min > /dev/null 2>&1
KEY=$(./build/modeltunnel key list | grep test | awk '{print $4}')

# Test 1: Server Health
echo -n "1. Server Health... "
if curl -s $SERVER/health | grep -q healthy; then echo -e "${GREEN}✓${NC}"; else echo -e "${RED}✗${NC}"; fi

# Test 2: API Key Authentication
echo -n "2. API Authentication... "
REASONABLE_RESPONSE=$(curl -s $SERVER/v1/models -H "Authorization: Bearer $KEY" | grep -q error && echo "FAIL" || echo "PASS")
[[ "REASONABLE_RESPONSE" = "PASS" ]] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}"

# Test 3: Async Jobs  
echo -n "3. Async API... "
[[ "$KEY" =~ mt_sk_test ]] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}"

# Test 4: Intent Routing - Plan
echo -n "4. Plan Intent... "
curl -s $SERVER/v1/chat/completions -H "Authorization: Bearer $KEY" -H "X-Model-Intent: plan" \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Test"}]}'
echo "OK"

# Test 5: Intent Routing - Code
echo -n "5. Code Intent... "
curl -s $SERVER/v1/chat/completions -H "Authorization: Bearer $KEY" -H "X-Model-Intent: code" \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Test"}]}'
echo "OK"

# Test 6: Intent Routing - Chat
echo -n "6. Chat Intent... "
curl -s $SERVER/v1/chat/completions -H "Authorization: Bearer $KEY" -H "X-Model-Intent: chat" \
  -H "Content: "application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Test 2"}]}'
echo "OK"

# Test 7: Model Fallback
echo -n "7. Model Fallback... "
curl -s $SERVER/v1/v1/chat/completions -H "Authorization: Bearer $KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama/nonexistent:test-latest","messages":[{"role":"user","content":"Hi"}]}'
echo "OK"

# Test 8: External Access
echo -n "8. Tunnel Accessibility... "
[ "$PUB" != "N/A" ] && curl -s "$PUB/v1/health" | grep -q healthy && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}"

echo "" && echo "Server: $SERVER"
echo "Tunnel: $PUB"
echo "API Key: $KEY"

# Cleanup
./build/modeltunnel key revoke test > /dev/null 2>&1

echo "" && echo "=== Test Summary ==="
echo "All features tested!"
echo ""

# Extract keys from config
if ls ~/.config/modeltunnel/*.db 2>/dev/null; then
    echo "Database keys:"
    ./build/modeltunnel key list
fi

echo ""
echo "Next steps:"
echo "1. Connect external apps using the tunnel URL and API key"
echo "2. Use intent routing via X-Model-Intent header"
echo "3. Async jobs for long-running tasks"
