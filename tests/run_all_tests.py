#!/usr/bin/env python3
"""
Master Test Runner for Modeltunnel

This script runs all integration tests and reports results.
Usage: python3 run_all_tests.py
"""

import subprocess
import sys
import os
import time
import json
from datetime import datetime
import requests

# Configuration
BASE_URL = "http://localhost:8080"
ADMIN_KEY = "mt_sk_admin_89fbbec74f021f15cc3bd1532fe4bf4ae84d3f12d932898fc08f43abfa309519"
TEST_RESULTS = {
    "passed": 0,
    "failed": 0,
    "tests": []
}

class Colors:
    GREEN = '\033[92m'
    RED = '\033[91m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    END = '\033[0m'

def log(message, level="INFO"):
    """Log a message with timestamp"""
    timestamp = datetime.now().strftime("%H:%M:%S")
    color = Colors.BLUE if level == "INFO" else Colors.YELLOW if level == "WARN" else Colors.RED
    print(f"{color}[{timestamp}] [{level}] {message}{Colors.END}")

def test(name, func):
    """Run a single test and record result"""
    log(f"Running: {name}", "INFO")
    try:
        func()
        log(f"✅ PASSED: {name}", "INFO")
        TEST_RESULTS["passed"] += 1
        TEST_RESULTS["tests"].append({"name": name, "status": "PASSED"})
        return True
    except Exception as e:
        log(f"❌ FAILED: {name} - {str(e)}", "ERROR")
        TEST_RESULTS["failed"] += 1
        TEST_RESULTS["tests"].append({"name": name, "status": "FAILED", "error": str(e)})
        return False

def check_server_running():
    """Check if modeltunnel server is running"""
    try:
        response = requests.get(f"{BASE_URL}/health", timeout=5)
        return response.status_code == 200
    except:
        return False

def wait_for_server(timeout=30):
    """Wait for server to be ready"""
    log("Waiting for server to be ready...", "INFO")
    start_time = time.time()
    while time.time() - start_time < timeout:
        if check_server_running():
            log("Server is ready!", "INFO")
            return True
        time.sleep(1)
    raise Exception("Server did not start within timeout")

# ==================== TEST FUNCTIONS ====================

def test_health_endpoint():
    """Test health check endpoint"""
    response = requests.get(f"{BASE_URL}/health")
    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    data = response.json()
    assert "status" in data, "Response missing 'status' field"

def test_list_models():
    """Test listing models endpoint"""
    global ADMIN_KEY
    headers = {"Authorization": f"Bearer {ADMIN_KEY}"}
    response = requests.get(f"{BASE_URL}/v1/models", headers=headers)
    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    data = response.json()
    assert "data" in data, "Response missing 'data' field"
    assert len(data["data"]) > 0, "No models found"
    log(f"Found {len(data['data'])} models", "INFO")

def test_chat_completion():
    """Test chat completion endpoint"""
    global ADMIN_KEY
    headers = {
        "Authorization": f"Bearer {ADMIN_KEY}",
        "Content-Type": "application/json"
    }
    payload = {
        "model": "ollama/phi:latest",
        "messages": [{"role": "user", "content": "Say hello"}],
        "max_tokens": 50
    }
    response = requests.post(f"{BASE_URL}/v1/chat/completions", headers=headers, json=payload)
    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    data = response.json()
    assert "choices" in data, "Response missing 'choices' field"
    assert len(data["choices"]) > 0, "No choices in response"

def test_streaming_chat():
    """Test streaming chat completion"""
    global ADMIN_KEY
    headers = {
        "Authorization": f"Bearer {ADMIN_KEY}",
        "Content-Type": "application/json"
    }
    payload = {
        "model": "ollama/phi:latest",
        "messages": [{"role": "user", "content": "Count to 3"}],
        "stream": True,
        "max_tokens": 50
    }
    response = requests.post(f"{BASE_URL}/v1/chat/completions", headers=headers, json=payload, stream=True)
    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    
    # Read at least one chunk
    chunks = 0
    for line in response.iter_lines():
        if line:
            chunks += 1
            if chunks >= 3:  # Read at least 3 chunks
                break
    
    assert chunks > 0, "No chunks received in streaming response"

def test_rate_limit_headers():
    """Test that rate limit headers are present"""
    global ADMIN_KEY
    headers = {
        "Authorization": f"Bearer {ADMIN_KEY}",
        "Content-Type": "application/json"
    }
    payload = {
        "model": "ollama/phi:latest",
        "messages": [{"role": "user", "content": "Hi"}]
    }
    response = requests.post(f"{BASE_URL}/v1/chat/completions", headers=headers, json=payload)
    assert response.status_code == 200, f"Expected 200, got {response.status_code}"
    
    # Check rate limit headers
    assert "X-RateLimit-Limit" in response.headers, "Missing X-RateLimit-Limit header"
    assert "X-RateLimit-Remaining" in response.headers, "Missing X-RateLimit-Remaining header"
    assert "X-RateLimit-Reset" in response.headers, "Missing X-RateLimit-Reset header"
    
    log(f"Rate Limit: {response.headers['X-RateLimit-Limit']}", "INFO")
    log(f"Remaining: {response.headers['X-RateLimit-Remaining']}", "INFO")

def test_model_details():
    """Test that model details (size, modified_at) are present"""
    global ADMIN_KEY
    headers = {"Authorization": f"Bearer {ADMIN_KEY}"}
    response = requests.get(f"{BASE_URL}/v1/models", headers=headers)
    assert response.status_code == 200
    data = response.json()
    
    if len(data["data"]) > 0:
        model = data["data"][0]
        assert "size" in model, "Model missing 'size' field"
        assert "modified_at" in model, "Model missing 'modified_at' field"
        log(f"Model {model['id']}: size={model.get('size', 'N/A')} bytes", "INFO")

def test_authentication_required():
    """Test that authentication is required"""
    response = requests.get(f"{BASE_URL}/v1/models")
    assert response.status_code == 401, f"Expected 401, got {response.status_code}"
    
    response = requests.post(f"{BASE_URL}/v1/chat/completions", json={"model": "test"})
    assert response.status_code == 401, f"Expected 401, got {response.status_code}"

def test_invalid_api_key():
    """Test with invalid API key"""
    headers = {"Authorization": "Bearer invalid_key_12345"}
    response = requests.get(f"{BASE_URL}/v1/models", headers=headers)
    assert response.status_code == 401, f"Expected 401, got {response.status_code}"

# ==================== CLI TESTS ====================

def test_cli_key_create():
    """Test CLI key creation"""
    result = subprocess.run(
        ["./modeltunnel", "key", "create", "testuser", "--models", "mistral,phi"],
        capture_output=True,
        text=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    assert result.returncode == 0, f"Key creation failed: {result.stderr}"
    assert "Created key" in result.stdout, "Key creation success message not found"
    log("Key created successfully via CLI", "INFO")

def test_cli_key_list():
    """Test CLI key listing"""
    result = subprocess.run(
        ["./modeltunnel", "key", "list"],
        capture_output=True,
        text=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    assert result.returncode == 0, f"Key list failed: {result.stderr}"
    assert "API Keys" in result.stdout, "Key list header not found"
    assert "testuser" in result.stdout, "Created key not in list"
    log("Key list working correctly", "INFO")

def test_cli_key_revoke():
    """Test CLI key revocation"""
    result = subprocess.run(
        ["./modeltunnel", "key", "revoke", "testuser"],
        capture_output=True,
        text=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    assert result.returncode == 0, f"Key revoke failed: {result.stderr}"
    assert "Revoked" in result.stdout, "Key revoke success message not found"
    log("Key revoked successfully via CLI", "INFO")

# ==================== MAIN ====================

def main():
    """Main test runner"""
    log("=" * 60, "INFO")
    log("MODELTUNNEL COMPREHENSIVE TEST SUITE", "INFO")
    log("=" * 60, "INFO")
    
    # Change to project directory
    os.chdir("/Users/stelios/Desktop/share-model")
    
    # Check if server is running
    if not check_server_running():
        log("Server not running. Starting server...", "WARN")
        # Start server in background
        subprocess.Popen(
            ["./modeltunnel", "up", "--ollama", "--model", "mistral"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL
        )
        time.sleep(3)
        wait_for_server(timeout=30)
    
    # Get admin key from server output or use a known key
    global ADMIN_KEY
    ADMIN_KEY = "mt_sk_admin_89fbbec74f021f15cc3bd1532fe4bf4ae84d3f12d932898fc08f43abfa309519"
    
    # Run all tests
    log("\n" + "=" * 60, "INFO")
    log("RUNNING API TESTS", "INFO")
    log("=" * 60, "INFO")
    
    test("Health Endpoint", test_health_endpoint)
    test("List Models", test_list_models)
    test("Model Details (size, modified_at)", test_model_details)
    test("Authentication Required", test_authentication_required)
    test("Invalid API Key", test_invalid_api_key)
    test("Chat Completion", test_chat_completion)
    test("Streaming Chat", test_streaming_chat)
    test("Rate Limit Headers", test_rate_limit_headers)
    
    log("\n" + "=" * 60, "INFO")
    log("RUNNING CLI TESTS", "INFO")
    log("=" * 60, "INFO")
    
    test("CLI Key Create", test_cli_key_create)
    test("CLI Key List", test_cli_key_list)
    test("CLI Key Revoke", test_cli_key_revoke)
    
    # Summary
    log("\n" + "=" * 60, "INFO")
    log("TEST SUMMARY", "INFO")
    log("=" * 60, "INFO")
    
    total = TEST_RESULTS["passed"] + TEST_RESULTS["failed"]
    log(f"Total Tests: {total}", "INFO")
    log(f"✅ Passed: {TEST_RESULTS['passed']}", "INFO")
    log(f"❌ Failed: {TEST_RESULTS['failed']}", "ERROR" if TEST_RESULTS["failed"] > 0 else "INFO")
    
    if TEST_RESULTS["failed"] > 0:
        log("\nFailed Tests:", "ERROR")
        for t in TEST_RESULTS["tests"]:
            if t["status"] == "FAILED":
                log(f"  - {t['name']}: {t.get('error', 'Unknown error')}", "ERROR")
    
    # Save results to file
    with open("/Users/stelios/Desktop/share-model/tests/test_results.json", "w") as f:
        json.dump(TEST_RESULTS, f, indent=2)
    
    log(f"\nResults saved to: tests/test_results.json", "INFO")
    
    return 0 if TEST_RESULTS["failed"] == 0 else 1

if __name__ == "__main__":
    sys.exit(main())
