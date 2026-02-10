#!/bin/bash
#
# Master Test Runner Script
# Runs all test modules and generates a comprehensive report
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_DIR="/Users/stelios/Desktop/share-model"
TEST_DIR="$PROJECT_DIR/tests"
REPORT_FILE="$TEST_DIR/test_report.txt"

echo -e "${BLUE}===============================================${NC}"
echo -e "${BLUE}  MODELTUNNEL COMPREHENSIVE TEST SUITE${NC}"
echo -e "${BLUE}===============================================${NC}"
echo ""

# Change to project directory
cd "$PROJECT_DIR"

# Check if server is running
echo -e "${YELLOW}Checking if server is running...${NC}"
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${YELLOW}Server not running. Starting server...${NC}"
    ./modeltunnel up --ollama --model mistral > /tmp/modeltunnel_test.log 2>&1 &
    SERVER_PID=$!
    
    # Wait for server to start
    echo -e "${YELLOW}Waiting for server to start...${NC}"
    for i in {1..30}; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            echo -e "${GREEN}Server is ready!${NC}"
            break
        fi
        sleep 1
    done
    
    if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${RED}Failed to start server!${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}Server is already running${NC}"
    SERVER_PID=""
fi

# Initialize report
echo "Modeltunnel Test Report" > "$REPORT_FILE"
echo "Generated: $(date)" >> "$REPORT_FILE"
echo "===============================================" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

TOTAL_PASSED=0
TOTAL_FAILED=0

# Function to run a test module
run_test() {
    local test_name=$1
    local test_file=$2
    
    echo -e "${BLUE}\n-----------------------------------------------${NC}"
    echo -e "${BLUE}Running: $test_name${NC}"
    echo -e "${BLUE}-----------------------------------------------${NC}"
    
    echo "" >> "$REPORT_FILE"
    echo "=== $test_name ===" >> "$REPORT_FILE"
    
    if python3 "$test_file" >> "$REPORT_FILE" 2>&1; then
        echo -e "${GREEN}✅ $test_name PASSED${NC}"
        echo "Status: PASSED" >> "$REPORT_FILE"
        ((TOTAL_PASSED++))
        return 0
    else
        echo -e "${RED}❌ $test_name FAILED${NC}"
        echo "Status: FAILED" >> "$REPORT_FILE"
        ((TOTAL_FAILED++))
        return 1
    fi
}

# Run main integration tests
echo -e "${YELLOW}\nStarting tests...${NC}"

run_test "Main Integration Tests" "$TEST_DIR/run_all_tests.py"
run_test "Rate Limiting Tests" "$TEST_DIR/integration/test_rate_limiting.py"
run_test "Persistence Tests" "$TEST_DIR/integration/test_persistence.py"
run_test "Model Features Tests" "$TEST_DIR/integration/test_models.py"

# Additional CLI tests
echo -e "${BLUE}\n-----------------------------------------------${NC}"
echo -e "${BLUE}Running: CLI Tests${NC}"
echo -e "${BLUE}-----------------------------------------------${NC}"

echo "" >> "$REPORT_FILE"
echo "=== CLI Tests ===" >> "$REPORT_FILE"

# Test key creation
./modeltunnel key create clitest --models mistral > /tmp/clitest.log 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ CLI Key Creation${NC}"
    echo "Key Creation: PASSED" >> "$REPORT_FILE"
    ((TOTAL_PASSED++))
else
    echo -e "${RED}❌ CLI Key Creation${NC}"
    echo "Key Creation: FAILED" >> "$REPORT_FILE"
    ((TOTAL_FAILED++))
fi

# Test key list
if ./modeltunnel key list | grep -q "clitest"; then
    echo -e "${GREEN}✅ CLI Key List${NC}"
    echo "Key List: PASSED" >> "$REPORT_FILE"
    ((TOTAL_PASSED++))
else
    echo -e "${RED}❌ CLI Key List${NC}"
    echo "Key List: FAILED" >> "$REPORT_FILE"
    ((TOTAL_FAILED++))
fi

# Test key revoke
./modeltunnel key revoke clitest > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ CLI Key Revoke${NC}"
    echo "Key Revoke: PASSED" >> "$REPORT_FILE"
    ((TOTAL_PASSED++))
else
    echo -e "${RED}❌ CLI Key Revoke${NC}"
    echo "Key Revoke: FAILED" >> "$REPORT_FILE"
    ((TOTAL_FAILED++))
fi

# Summary
echo -e "${BLUE}\n===============================================${NC}"
echo -e "${BLUE}  TEST SUMMARY${NC}"
echo -e "${BLUE}===============================================${NC}"
echo ""
echo -e "${GREEN}✅ Passed: $TOTAL_PASSED${NC}"
echo -e "${RED}❌ Failed: $TOTAL_FAILED${NC}"
echo ""

# Add summary to report
echo "" >> "$REPORT_FILE"
echo "===============================================" >> "$REPORT_FILE"
echo "SUMMARY" >> "$REPORT_FILE"
echo "===============================================" >> "$REPORT_FILE"
echo "Total Passed: $TOTAL_PASSED" >> "$REPORT_FILE"
echo "Total Failed: $TOTAL_FAILED" >> "$REPORT_FILE"
echo "Report saved to: $REPORT_FILE" >> "$REPORT_FILE"

if [ $TOTAL_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed!${NC}"
    echo ""
    echo -e "${BLUE}Full report saved to: $REPORT_FILE${NC}"
    exit 0
else
    echo -e "${RED}⚠️  Some tests failed. Check the report for details.${NC}"
    echo ""
    echo -e "${BLUE}Full report saved to: $REPORT_FILE${NC}"
    exit 1
fi
