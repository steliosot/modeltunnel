#!/bin/bash
# Test 1: Server Health Check
source test_config.sh

echo "=== Test 1: Server Health ===" >> REPORT.md

echo date "+%Y-%m-%d %H:%M:%S" >> REPORT.md

# Test health endpoint
echo "## Server Health" >> REPORT.md
echo "" >> REPORT.md

echo -n "Testing health endpoint... "
curl -s "$SERVER_URL/health" | grep -q "healthy" && echo -e "${GREEN}✓ PASS${NC}" || echo -e "${RED}✗ FAIL${NC}" >> REPORT.md

# Check dashboard
echo -n "Checking dashboard accessibility... "
curl -s "$SERVER_URL/admin" | grep -q "Dashboard" && echo -e "${GREEN}✓ PASS${NC}" || echo -e "${RED}✗ FAIL${NC}" >> REPORT.md

echo "" >> REPORT.md
