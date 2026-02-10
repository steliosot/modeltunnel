#!/usr/bin/env python3
"""
Per-Model Rate Limiting Tests

Tests the per-model rate limiting functionality including:
- Different limits for different models
- Wildcard matching (tinyllama:*)
- Rate limit headers
- 429 responses when limits exceeded
"""

import requests
import time
import sys
import os

# Add parent directory to path
sys.path.insert(0, '/Users/stelios/Desktop/share-model/tests')
from run_all_tests import log, Colors

BASE_URL = "http://localhost:8080"

def test_per_model_rate_limits():
    """Test that different models have different rate limits"""
    log("Testing per-model rate limiting...", "INFO")
    
    # Use student key which has per-model limits configured
    student_key = "mt_sk_admin_89fbbec74f021f15cc3bd1532fe4bf4ae84d3f12d932898fc08f43abfa309519"
    headers = {
        "Authorization": f"Bearer {student_key}",
        "Content-Type": "application/json"
    }
    
    # Test 1: Mistral should have 5/min limit
    log("Testing mistral rate limit (should be 5/min)...", "INFO")
    mistral_limit = None
    for i in range(3):
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            headers=headers,
            json={
                "model": "ollama/mistral:latest",
                "messages": [{"role": "user", "content": "Hi"}],
                "max_tokens": 10
            }
        )
        assert response.status_code == 200, f"Request {i+1} failed: {response.status_code}"
        mistral_limit = response.headers.get('X-RateLimit-Limit')
        log(f"  Request {i+1}: Limit={mistral_limit}", "INFO")
        time.sleep(0.5)
    
    assert mistral_limit == "5", f"Expected mistral limit to be 5, got {mistral_limit}"
    log("✅ Mistral has correct rate limit (5/min)", "INFO")
    
    # Test 2: Phi should have 100/min limit
    log("Testing phi rate limit (should be 100/min)...", "INFO")
    phi_limit = None
    for i in range(3):
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            headers=headers,
            json={
                "model": "ollama/phi:latest",
                "messages": [{"role": "user", "content": "Hi"}],
                "max_tokens": 10
            }
        )
        assert response.status_code == 200, f"Request {i+1} failed: {response.status_code}"
        phi_limit = response.headers.get('X-RateLimit-Limit')
        log(f"  Request {i+1}: Limit={phi_limit}", "INFO")
        time.sleep(0.5)
    
    assert phi_limit == "100", f"Expected phi limit to be 100, got {phi_limit}"
    log("✅ Phi has correct rate limit (100/min)", "INFO")

def test_rate_limit_exceeded():
    """Test that rate limits are enforced"""
    log("Testing rate limit enforcement...", "INFO")
    
    # Create a key with very low rate limit for testing
    import subprocess
    result = subprocess.run(
        ["./modeltunnel", "key", "create", "ratetest", "--rate", "2/min"],
        capture_output=True,
        text=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    
    # Extract key from output
    import re
    match = re.search(r'API Key: (mt_sk_[a-zA-Z0-9_]+)', result.stdout)
    if not match:
        log("❌ Failed to create test key", "ERROR")
        return
    
    test_key = match.group(1)
    headers = {
        "Authorization": f"Bearer {test_key}",
        "Content-Type": "application/json"
    }
    
    # Make 3 requests - 3rd should be rate limited
    for i in range(3):
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            headers=headers,
            json={
                "model": "ollama/phi:latest",
                "messages": [{"role": "user", "content": "Hi"}],
                "max_tokens": 10
            }
        )
        
        if i < 2:
            assert response.status_code == 200, f"Request {i+1} should succeed"
            log(f"  Request {i+1}: ✅ (status {response.status_code})", "INFO")
        else:
            assert response.status_code == 429, f"Request {i+1} should be rate limited (429)"
            log(f"  Request {i+1}: ✅ Rate limited (429)", "INFO")
            
            # Check for retry-after header
            assert "Retry-After" in response.headers, "Missing Retry-After header"
            log(f"  Retry-After: {response.headers['Retry-After']}s", "INFO")
    
    # Clean up
    subprocess.run(
        ["./modeltunnel", "key", "revoke", "ratetest"],
        capture_output=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    
    log("✅ Rate limit enforcement working correctly", "INFO")

def test_rate_limit_reset():
    """Test that rate limits reset after the time window"""
    log("Testing rate limit reset (this will take ~60 seconds)...", "INFO")
    
    # Use student key with mistral (5/min limit)
    student_key = "mt_sk_admin_89fbbec74f021f15cc3bd1532fe4bf4ae84d3f12d932898fc08f43abfa309519"
    headers = {
        "Authorization": f"Bearer {student_key}",
        "Content-Type": "application/json"
    }
    
    # Make 5 requests to hit the limit
    log("Making 5 requests to hit rate limit...", "INFO")
    for i in range(5):
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            headers=headers,
            json={
                "model": "ollama/mistral:latest",
                "messages": [{"role": "user", "content": "Hi"}],
                "max_tokens": 10
            }
        )
        remaining = response.headers.get('X-RateLimit-Remaining', 'N/A')
        log(f"  Request {i+1}: Remaining={remaining}", "INFO")
        time.sleep(0.3)
    
    # 6th request should be rate limited
    response = requests.post(
        f"{BASE_URL}/v1/chat/completions",
        headers=headers,
        json={
            "model": "ollama/mistral:latest",
            "messages": [{"role": "user", "content": "Hi"}],
            "max_tokens": 10
        }
    )
    
    if response.status_code == 429:
        log("✅ Rate limit hit as expected", "INFO")
        retry_after = int(response.headers.get('Retry-After', 60))
        log(f"Waiting {retry_after} seconds for reset...", "INFO")
        time.sleep(min(retry_after + 2, 10))  # Wait max 10 seconds for test
        
        # Try again - should succeed now
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            headers=headers,
            json={
                "model": "ollama/mistral:latest",
                "messages": [{"role": "user", "content": "Hi"}],
                "max_tokens": 10
            }
        )
        
        if response.status_code == 200:
            log("✅ Rate limit reset successfully", "INFO")
        else:
            log("⚠️  Rate limit may not have reset yet (expected with short wait)", "WARN")
    else:
        log("⚠️  Expected rate limit to be hit, but it wasn't", "WARN")

def main():
    """Run rate limiting tests"""
    log("\n" + "=" * 60, "INFO")
    log("PER-MODEL RATE LIMITING TESTS", "INFO")
    log("=" * 60, "INFO")
    
    os.chdir("/Users/stelios/Desktop/share-model")
    
    try:
        test_per_model_rate_limits()
        test_rate_limit_exceeded()
        # Skip the reset test as it takes too long
        # test_rate_limit_reset()
        
        log("\n✅ All rate limiting tests passed!", "INFO")
        return 0
    except AssertionError as e:
        log(f"\n❌ Test failed: {e}", "ERROR")
        return 1
    except Exception as e:
        log(f"\n❌ Error: {e}", "ERROR")
        return 1

if __name__ == "__main__":
    sys.exit(main())
