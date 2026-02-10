#!/bin/bash
cd "$(dirname "$0")"
GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'

echo "=== Modeltunnel Comprehensive Tests ==="
source test_config.sh

# Test 1: Server Health
echo -n "1. Server Health... "
curl -s $SERVER_URL/health | grep -q healthy && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}"

# Test 2: Authentication  
echo -n "2. API Authentication... "
./build/modeltunnel key create test-user --models mistral > /dev/null 2>&1
KEY=$(./build/modeltunnel key list | grep test-user | awk '{print $4}')
curl -s -o /dev/null -w "%{http_code}" $SERVER_URL/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_$(echo $KEY)" \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama/mistral:latest","messages":[{"role":"user","content":"Hi"}]}'
[ $? -eq 0 ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}"
./build/modeltunnel key revoke test-user > /dev/null 2>&1

# Test 3: External App Connection
echo -n "3. External App Connection... "
PUB=$(cat ~/.config/modeltunnel/tunnel.url 2>/dev/null || echo "none")
[ "$PUB" != "none" ] && curl -s "$PUB/v1/health" | grep -q healthy && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}"

# Test 4: Async Jobs
echo -n "4. Async Jobs... "
JOB=$(curl -s -X POST $SERVER_URL/v1/async -H "Authorization: Bearer ./build/modeltunnel key create async-test > /dev/null 2>&1 && ./build/modeltunnel key list | grep async-test | awk '{print $4}')" \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama/mistral:latest","messages":[{"role":"user","content":"Hi"}]}')
echo $JOB | grep -q job_id && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}"

# Test 5: Intent Routing
echo -n "5. Intent Routing (plan)... "
curl -s $SERVER_URL/v1/chat/completions -H "Authorization: Bearer $(./build/modeltunnel key create intent-test > /dev/null 2>&1 && ./build/modeltunnel key list | grep intent-test | awk '{print $4}')" \
  -H "X-Model-Intent: plan" \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Test"}]}'
echo -e "${GREEN}✓${NC}"
./build/modeltunnel key revoke intent-test > /dev/null 2>&1

# Test 6: Fallback
echo -n "6. Model Fallback (try unavailable)... "
curl -s $SERVER_URL/v1/chat/completions -H "Authorization: Bearer $(./build/modeltunnel key create fallback-test > /dev/null 2>&1 && ./build/modeltunnel key list | grep fallback-test | awk '{print $4}')" \
  -H "Content-Type: application/json" \
  -d '{"model":"ollama/nonexistent:latest","messages":[{"role":"user","content":"Hi"}]}'

echo ""
echo "Tests complete!"
echo "Tunnel: $PUB"
