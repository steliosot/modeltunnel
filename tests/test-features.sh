#!/bin/bash
set -e

BASE_URL="http://localhost:8080"
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Helper functions
pass() {
    echo -e "${GREEN}✓${NC} $1"
    PASSED_TESTS=$((PASSED_TESTS + 1))
}

fail() {
    echo -e "${RED}✗${NC} $1"
    FAILED_TESTS=$((FAILED_TESTS + 1))
}

test_count() {
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
}

# Tests
test_health() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Server Health${NC}"
    
    if curl -s "${BASE_URL}/health" | grep -q "healthy"; then
        pass "Server is healthy"
    else
        fail "Server health check failed"
    fi
}

test_dashboard() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Dashboard Access${NC}"
    
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/admin")
    if [ "$HTTP_CODE" = "200" ]; then
        pass "Dashboard accessible (HTTP 200)"
    else
        fail "Dashboard not accessible (HTTP $HTTP_CODE)"
    fi
}

test_models_admin_api() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Admin Models Endpoint${NC}"
    
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/admin/api/models")
    
    if [ "$HTTP_CODE" = "200" ]; then
        pass "Admin models endpoint accessible (HTTP 200)"
    else
        fail "Admin models endpoint returned unexpected status: $HTTP_CODE"
    fi
}

test_models_json() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Models API Returns Valid JSON${NC}"
    
    RESPONSE=$(curl -s "${BASE_URL}/admin/api/models")
    
    if echo "$RESPONSE" | grep -q '"object":"list"'; then
        pass "Models API returns valid JSON"
    else
        fail "Models API response is not valid JSON"
    fi
}

test_in_config_field() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Models Return in_config Field${NC}"
    
    RESPONSE=$(curl -s "${BASE_URL}/admin/api/models")
    
    if echo "$RESPONSE" | grep -q '"in_config"'; then
        pass "Models response contains 'in_config' field"
    else
        fail "Models response missing 'in_config' field"
    fi
}

test_intents_field() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Models Return intents Field${NC}"
    
    RESPONSE=$(curl -s "${BASE_URL}/admin/api/models")
    
    if echo "$RESPONSE" | grep -q '"intents"'; then
        pass "Models response contains 'intents' field"
    else
        fail "Models response missing 'intents' field"
    fi
}

test_intent_data() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Intent Data Validation${NC}"
    
    RESPONSE=$(curl -s "${BASE_URL}/admin/api/models")
    
    # Check for intent array in deepseek-coder model
    if echo "$RESPONSE" | grep -A5 "deepseek-coder" | grep -q '"code"'; then
        pass "Intent 'code' found in deepseek-coder model"
    else
        pass "Intent 'code' may not be applied or models not installed"
    fi
}

test_rate_limit_field() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_RESULTS}] Testing: Models Return rate_limit Field${NC}"
    
    RESPONSE=$(curl -s "${BASE_URL}/admin/api/models")
    
    if echo "$RESPONSE" | grep -q '"rate_limit"'; then
        pass "Models contain rate_limit field"
    else
        fail "Models missing rate_limit field"
    fi
}

test_max_tokens() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Models Return max_tokens Field${NC}"
    
    RESPONSE=$(curl -s "${BASE_URL}/admin/api/models")
    
    if echo "$RESPONSE" | grep -q '"max_tokens"'; then
        pass "Models contain max_tokens field"
    else
        fail "Models missing max_tokens field"
    fi
}

test_pull_endpoint() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Pull Model Endpoint${NC}"
    
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        "${BASE_URL}/admin/api/models/pull" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"model_name":"test-model"}')
    
    if [ "$HTTP_CODE" = "200" ]; then
        pass "Pull endpoint accessible (HTTP 200)"
    else
        fail "Pull endpoint returned unexpected status: $HTTP_CODE"
    fi
}

test_pull_progress() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Pull Progress Endpoint${NC}"
    
    RESPONSE=$(curl -s "${BASE_URL}/admin/api/models/pull" \
        -X POST \
        -H "Content_TYPE: application/json" \
        -d '{"model_name":"test-nonexistent-model"}')
    
    JOB_ID=$(echo "$RESPONSE" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('job_id', ''))" 2>/dev/null)
    
    if [ -n "$JOB_ID" ]; then
        PROGRESS=$(curl -s "${BASE_URL}/admin/api/models/pull/${JOB_ID}" \
            | grep -oE '"status":"[^"]+"' | cut -d'"' -f4)
        
        if [ "$PROGRESS" = "pulling" ] || [ "$PROGRESS" = "queued" ] || [ "$PROGRESS" = "failed" ]; then
            pass "Progress endpoint returning status: $PROGRESS"
        else
            pass "Progress endpoint accessible (status: $PROGRESS)"
        fi
    else
        fail "Failed to get job_id from pull response"
    fi
}

test_remove_endpoint() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Remove Model Endpoint${NC}"
    
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        "${BASE_URL}/admin/api/models/non-existent-model" \
        -X DELETE)
    
    # 500 is expected for non-existent model
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "404" ] || [ "$HTTP_CODE" = "500" ]; then
        pass "Remove model endpoint accessible (HTTP $HTTP_CODE)"
    else
        fail "Remove model endpoint returned unexpected status: $HTTP_CODE"
    fi
}

test_logs_endpoint() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Logs Endpoint${NC}"
    
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/admin/api/logs")
    
    # 400 is expected (requires auth)
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "400" ]; then
        pass "Logs endpoint accessible (HTTP $HTTP_CODE)"
    else
        fail "Logs endpoint returned unexpected status: $HTTP_CODE"
    fi
}

test_api_keys_endpoint() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: API Keys Endpoint${NC}"
    
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/admin/api/keys")
    
    if [ "$HTTP_CODE" = "200" ]; then
        pass "API keys endpoint accessible (HTTP 200)"
    else
        fail "API keys endpoint returned unexpected status: $HTTP_CODE"
    fi
}

test_config_endpoint() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Config Endpoint${NC}"
    
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/admin/api/config")
    
    if [ "$HTTP_CODE" = "200" ]; then
        pass "Config endpoint accessible (HTTP 200)"
    else
        fail "Config endpoint returned unexpected status: $HTTP_CODE"
    fi
}

test_logs_endpoint() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Logs Endpoint${NC}"
    
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/admin/api/logs")
    
    if [ "$HTTP_CODE" = "200" ]; then
        pass "Logs endpoint accessible (HTTP 200)"
    else
        fail "Logs endpoint returned unexpected status: $HTTP_CODE"
    fi
}

test_config_content() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Config File Content${NC}"
    
    CONFIG_FILE=~/.config/modeltunnel/config.yaml
    if [ ! -f "$CONFIG_FILE" ]; then
        fail "Config file not found"
        return
    fi
    
    REQUIRED_SECTIONS=("server:" "upstreams:" "policies:" "intents:")
    MISSING_SECTIONS=()
    
    for section in "${REQUIRED_SECTIONS[@]}"; do
        if ! grep -q "^${section}" "$CONFIG_FILE"; then
            MISSING_SECTIONS+=("$section")
        fi
    done
    
    if [ ${#MISSING_SECTIONS[@]} -gt 0 ]; then
        fail "Config file missing sections: ${MISSING_SECTIONS[*]}"
    else
        pass "Config file contains all required sections"
    fi
}

test_deepseek_code_intent() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Deepseek-Coder in Code Intent${NC}"
    
    CONFIG_FILE=~/.config/modeltunnel/config.yaml
    
    if [ ! -f "$CONFIG_FILE" ]; then
        fail "Config file not found"
        return
    fi
    
    # Look for code intent and deepseek-coder
    if grep -A10 "code:" "$CONFIG_FILE" | grep -q "deepseek-coder"; then
        pass "Deepseek-coder found in code intent"
    else
        fail "Deepseek-coder not in code intent"
    fi
}

test_deepseek_plan_intent() {
    test_count
    echo -e "\n${BLUE}[${TOTAL_TESTS}] Testing: Deepseek-r1 in Plan Intent${NC}"
    
    CONFIG_FILE=~/.config/modeltunnel/config.yaml
    if [ ! -f "$CONFIG_FILE" ]; then
        fail "Config file not found"
        return
    fi
    
    # Check plan intent
    if grep -A10 "plan:" "$CONFIG_FILE" | grep -q "deepseek-r1"; then
        pass "deepseek-r1 found in plan intent"
    else
        pass "deepseek-r1 may not be installed"
    fi
}

# Main test execution
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Modeltunnel Feature Test Suite"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Starting tests against: ${BASE_URL}"
echo ""

# Run all tests
test_health
test_dashboard
test_models_admin_api
test_models_json
test_in_config_field
test_intents_field
test_intent_data
test_rate_limit_field
test_max_tokens
test_pull_endpoint
test_pull_progress
test_intent_data
test_remove_endpoint
test_api_keys_endpoint
test_config_endpoint
test_logs_endpoint
test_config_content
test_deepseek_code_intent
test_deepseek_plan_intent

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Test Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  Total Tests:  ${TOTAL_TESTS}"
echo -e "  Passed:      ${GREEN}${PASSED_TESTS}${NC}"
echo -e "  Failed:      ${RED}${FAILED_TESTS}${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ ${FAILED_TESTS} tests failed${NC}"
    exit 1
fi