#!/bin/bash
# Master test runner
cd "$(dirname "$0")"

echo "=== Modeltunnel Comprehensive Test Suite ===" 
echo ""
echo "Starting at: $(date)"
echo ""

# Initialize
./00_setup.sh

# Run all tests
./01_server_health.sh
./02_authentication.sh

# ./03_external_apps.sh
# ./04_async_jobs.sh
# ./05_intent_routing.sh
# ./06_fallback.sh

echo ""
echo "=== Complete. View REPORT.md for results ==="
