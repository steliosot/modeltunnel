#!/bin/bash
GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'
REPORT="tests/report.md"
SERVER="http://localhost:8080"
PUBLIC=$(cat ~/.config/modeltunnel/tunnel.url 2>/dev/null || echo "")

echo "# Test Report $(date)" > $REPORT
echo "" >> $REPORT
echo "## Test Results" >> $REPORT

test_it() {
    local name="$1"
    local cmd="$2" 
    printf "%-30s " "$name"
    if eval "$cmd" > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}" >> $REPORT
        return 0
    else
        echo -e " ${RED}✗${NC}" >> $REPORT
        return 1
    fi
}

test_it "Server Health" "curl -s http://localhost:8080/health | grep -q healthy"
test_it "Key Creation" "./build/modeltunnel key create test1 --models mistral > /dev/null"
TESTKEY=$(./build/modeltunnel key list | grep test1 | awk '{print $4}')
test_it "Key Usage" "curl -s http://localhost:8080/v1/models -H \"Authorization: Bearer $TESTKEY\" | grep -q mistral"
test_it "Async Jobs" "curl -s http://localhost:8080/v1/async -H \"Authorization: Bearer $TESTKEY\" -H \"Content-Type: application/json\" -d '{\"model\":\"ollama/mistral\",\"messages\":[{\"role\":\"user\",\"content\":\"Hi\"}]}'"
test_it "Intent Routing" "curl -s http://localhost:8080/v1/chat/competitions -H \"Authorization: Bearer $TESTKEY\" -H \"X-Model-Intent: plan\" -H \"Content-Type: application/json\" -d '{\"model\":\"auto\",\"messages\":[{\"role\":\"user\",\"content\":\"Hi\"}]}'"
test_it "Tunnel Access" "curl -s $PUBLIC/v1/health | grep -q healthy"

./build/modeltunnel key revoke test1 > /dev/null 2>&1

echo "" >> $REPORT.md
echo "**Tunnel:** $PUBLIC" >> $REPORT.md

echo "Tests complete! See $REPORT"
