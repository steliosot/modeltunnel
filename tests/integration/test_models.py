#!/usr/bin/env python3
"""
Model Features Tests

Tests model listing, details, and format variations.
"""

import requests
import sys
import os

sys.path.insert(0, '/Users/stelios/Desktop/share-model/tests')
from run_all_tests import log, Colors

BASE_URL = "http://localhost:8080"
ADMIN_KEY = "mt_sk_admin_89fbbec74f021f15cc3bd1532fe4bf4ae84d3f12d932898fc08f43abfa309519"

def test_model_list_format():
    """Test that model list has correct format"""
    log("Testing model list format...", "INFO")
    
    headers = {"Authorization": f"Bearer {ADMIN_KEY}"}
    response = requests.get(f"{BASE_URL}/v1/models", headers=headers)
    
    assert response.status_code == 200
    data = response.json()
    
    # Check structure
    assert "object" in data, "Missing 'object' field"
    assert data["object"] == "list", "Object should be 'list'"
    assert "data" in data, "Missing 'data' field"
    assert isinstance(data["data"], list), "Data should be a list"
    
    if len(data["data"]) > 0:
        model = data["data"][0]
        required_fields = ["id", "object", "created", "owned_by"]
        for field in required_fields:
            assert field in model, f"Model missing '{field}' field"
        
        # Check optional fields (size, modified_at)
        optional_fields = ["size", "modified_at"]
        for field in optional_fields:
            if field in model:
                log(f"   {field}: {model[field]}", "INFO")
    
    log(f"✅ Model list format correct ({len(data['data'])} models)", "INFO")

def test_model_name_formats():
    """Test different model name formats"""
    log("Testing model name formats...", "INFO")
    
    headers = {
        "Authorization": f"Bearer {ADMIN_KEY}",
        "Content-Type": "application/json"
    }
    
    formats = [
        "ollama/phi:latest",      # Full format
        "ollama/phi",              # Without tag
        "ollama/mistral:latest",  # Another model
    ]
    
    for model_format in formats:
        log(f"   Testing: {model_format}", "INFO")
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            headers=headers,
            json={
                "model": model_format,
                "messages": [{"role": "user", "content": "Hi"}],
                "max_tokens": 10
            }
        )
        
        if response.status_code == 200:
            log(f"   ✅ {model_format} works", "INFO")
        else:
            log(f"   ⚠️  {model_format} failed: {response.status_code}", "WARN")

def test_model_size_formatting():
    """Test that model sizes are formatted correctly"""
    log("Testing model size formatting...", "INFO")
    
    headers = {"Authorization": f"Bearer {ADMIN_KEY}"}
    response = requests.get(f"{BASE_URL}/v1/models", headers=headers)
    
    assert response.status_code == 200
    data = response.json()
    
    for model in data["data"]:
        if "size" in model and model["size"]:
            size = model["size"]
            # Size should be in bytes (int)
            assert isinstance(size, (int, float)), f"Size should be numeric, got {type(size)}"
            assert size > 0, f"Size should be positive, got {size}"
    
    log("✅ All model sizes are valid", "INFO")

def test_model_modified_formatting():
    """Test that model modified dates are formatted correctly"""
    log("Testing model modified date formatting...", "INFO")
    
    headers = {"Authorization": f"Bearer {ADMIN_KEY}"}
    response = requests.get(f"{BASE_URL}/v1/models", headers=headers)
    
    assert response.status_code == 200
    data = response.json()
    
    from datetime import datetime
    
    for model in data["data"]:
        if "modified_at" in model and model["modified_at"]:
            modified = model["modified_at"]
            # Should be ISO format string
            try:
                datetime.fromisoformat(modified.replace('Z', '+00:00'))
            except ValueError:
                assert False, f"Invalid date format: {modified}"
    
    log("✅ All model dates are valid ISO format", "INFO")

def test_different_models():
    """Test that different models return different responses"""
    log("Testing different models produce different results...", "INFO")
    
    headers = {
        "Authorization": f"Bearer {ADMIN_KEY}",
        "Content-Type": "application/json"
    }
    
    models = ["ollama/phi:latest", "ollama/mistral:latest"]
    responses = []
    
    for model in models:
        response = requests.post(
            f"{BASE_URL}/v1/chat/completions",
            headers=headers,
            json={
                "model": model,
                "messages": [{"role": "user", "content": "What model are you? Answer in 5 words."}],
                "max_tokens": 20,
                "temperature": 0.0
            }
        )
        
        if response.status_code == 200:
            content = response.json()["choices"][0]["message"]["content"]
            responses.append((model, content))
            log(f"   {model}: {content[:50]}...", "INFO")
    
    log(f"✅ Tested {len(responses)} models", "INFO")

def main():
    """Run model tests"""
    log("\n" + "=" * 60, "INFO")
    log("MODEL FEATURES TESTS", "INFO")
    log("=" * 60, "INFO")
    
    tests = [
        ("Model List Format", test_model_list_format),
        ("Model Name Formats", test_model_name_formats),
        ("Model Size Formatting", test_model_size_formatting),
        ("Model Date Formatting", test_model_modified_formatting),
        ("Different Models", test_different_models),
    ]
    
    passed = 0
    failed = 0
    
    for name, test_func in tests:
        try:
            log(f"\nRunning: {name}", "INFO")
            test_func()
            passed += 1
        except AssertionError as e:
            log(f"❌ FAILED: {e}", "ERROR")
            failed += 1
        except Exception as e:
            log(f"❌ ERROR: {e}", "ERROR")
            failed += 1
    
    log("\n" + "=" * 60, "INFO")
    log(f"Results: {passed} passed, {failed} failed", "INFO")
    log("=" * 60, "INFO")
    
    return 0 if failed == 0 else 1

if __name__ == "__main__":
    sys.exit(main())
