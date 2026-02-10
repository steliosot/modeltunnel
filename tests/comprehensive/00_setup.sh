#!/bin/bash
# Comprehensive Test Suite Setup
set -e

echo "=== Modeltunnel Comprehensive Test Suite ===" 
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' No Color

# Test server details
SERVER_URL="http://localhost:8080"
PUBLIC_URL=$(cat ~/.config/modeltunnel/tunnel.url 2>/dev/null || echo "")
API_KEY="mt_sk_test_$(head -c 16 /dev/urandom | xxd -p)"
TEST_REPORT="tests/comprehensive/REPORT.md"

echo -e "${GREEN}✓ Setup complete${NC}"
echo "Server URL: $SERVER_URL"
echo "Public URL: $PUBLIC_URL"
echo "API Key: $API_KEY (for testing)"
echo "Report: $TEST_REPORT"

# Save test configuration
cat > tests/comprehensive/test_config.sh << EOFCFG
SERVER_URL="$SERVER_URL"
PUBLIC_URL="$PUBLIC_URL"
API_KEY="$API_KEY"
EOFCFG

chmod +x tests/comprehensive/test_config.sh
