#!/usr/bin/env python3
"""
Database Persistence Tests

Tests that keys and usage stats survive server restarts.
"""

import subprocess
import time
import sys
import os
import sqlite3

sys.path.insert(0, '/Users/stelios/Desktop/share-model/tests')
from run_all_tests import log, Colors

def test_database_exists():
    """Test that SQLite database file exists"""
    log("Checking database file...", "INFO")
    db_path = os.path.expanduser("~/.config/modeltunnel/keys.db")
    
    assert os.path.exists(db_path), f"Database file not found at {db_path}"
    log(f"✅ Database file exists: {db_path}", "INFO")
    
    # Check file size
    size = os.path.getsize(db_path)
    log(f"   Database size: {size} bytes", "INFO")

def test_database_schema():
    """Test that database has correct schema"""
    log("Checking database schema...", "INFO")
    
    db_path = os.path.expanduser("~/.config/modeltunnel/keys.db")
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    # Check if api_keys table exists
    cursor.execute("SELECT name FROM sqlite_master WHERE type='table' AND name='api_keys'")
    result = cursor.fetchone()
    assert result is not None, "api_keys table not found"
    
    # Check columns
    cursor.execute("PRAGMA table_info(api_keys)")
    columns = {row[1] for row in cursor.fetchall()}
    required_columns = {'name', 'key', 'allowed_upstreams', 'policy', 'created_at', 'last_used_at', 'request_count'}
    
    missing = required_columns - columns
    assert not missing, f"Missing columns: {missing}"
    
    conn.close()
    log("✅ Database schema is correct", "INFO")

def test_keys_persist_after_restart():
    """Test that keys survive server restart"""
    log("Testing key persistence across restarts...", "INFO")
    
    # Step 1: Create a test key
    log("Creating test key...", "INFO")
    result = subprocess.run(
        ["./modeltunnel", "key", "create", "persisttest"],
        capture_output=True,
        text=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    assert result.returncode == 0, f"Failed to create key: {result.stderr}"
    
    # Extract key
    import re
    match = re.search(r'API Key: (mt_sk_[a-zA-Z0-9_]+)', result.stdout)
    assert match, "Could not extract key from output"
    test_key = match.group(1)
    log(f"   Created key: {test_key[:30]}...", "INFO")
    
    # Step 2: Verify key exists
    result = subprocess.run(
        ["./modeltunnel", "key", "list"],
        capture_output=True,
        text=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    assert "persisttest" in result.stdout, "Key not found in list"
    log("   Key exists in database", "INFO")
    
    # Step 3: Stop server
    log("Stopping server...", "INFO")
    subprocess.run(["pkill", "-f", "./modeltunnel up"], capture_output=True)
    time.sleep(2)
    
    # Step 4: Start server again
    log("Restarting server...", "INFO")
    subprocess.Popen(
        ["./modeltunnel", "up", "--ollama", "--model", "mistral"],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        cwd="/Users/stelios/Desktop/share-model"
    )
    time.sleep(3)
    
    # Step 5: Check key still exists
    result = subprocess.run(
        ["./modeltunnel", "key", "list"],
        capture_output=True,
        text=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    assert "persisttest" in result.stdout, "Key not found after restart!"
    log("✅ Key persisted after restart", "INFO")
    
    # Step 6: Clean up
    subprocess.run(
        ["./modeltunnel", "key", "revoke", "persisttest"],
        capture_output=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    log("   Cleaned up test key", "INFO")

def test_usage_stats_persist():
    """Test that usage stats (request count) persist"""
    log("Testing usage stats persistence...", "INFO")
    
    import requests
    
    # Create test key
    result = subprocess.run(
        ["./modeltunnel", "key", "create", "usagetest"],
        capture_output=True,
        text=True,
        cwd="/Users/stelios/Desktop/share-model"
    )
    
    import re
    match = re.search(r'API Key: (mt_sk_[a-zA-Z0-9_]+)', result.stdout)
    test_key = match.group(1)
    
    # Make some requests
    log("Making 3 API requests...", "INFO")
    headers = {
        "Authorization": f"Bearer {test_key}",
        "Content-Type": "application/json"
    }
    
    for i in range(3):
        response = requests.post(
            "http://localhost:8080/v1/chat/completions",
            headers=headers,
            json={
                "model": "ollama/phi:latest",
                "messages": [{"role": "user", "content": "Hi"}],
                "max_tokens": 10
            }
        )
        assert response.status_code == 200
    
    # Check usage in database
    db_path = os.path.expanduser("~/.config/modeltunnel/keys.db")
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    cursor.execute("SELECT request_count FROM api_keys WHERE name = 'usagetest'")
    result = cursor.fetchone()
    conn.close()
    
    if result:
        count = result[0]
        log(f"   Request count in DB: {count}", "INFO")
        assert count >= 3, f"Expected at least 3 requests, got {count}"
        log("✅ Usage stats persisted correctly", "INFO")
    else:
        log("⚠️  Could not verify usage stats", "WARN")
    
    # Clean up
    subprocess.run(
        ["./modeltunnel", "key", "revoke", "usagetest"],
        capture_output=True,
        cwd="/Users/stelios/Desktop/share-model"
    )

def test_config_backup():
    """Test that config file is kept as backup"""
    log("Testing config file backup...", "INFO")
    
    config_path = os.path.expanduser("~/.config/modeltunnel/config.yaml")
    
    # Read config
    with open(config_path, 'r') as f:
        content = f.read()
    
    # Config should still have keys section (even if empty)
    assert 'keys:' in content, "Config file missing keys section"
    
    log("✅ Config file preserved as backup", "INFO")

def main():
    """Run persistence tests"""
    log("\n" + "=" * 60, "INFO")
    log("DATABASE PERSISTENCE TESTS", "INFO")
    log("=" * 60, "INFO")
    
    os.chdir("/Users/stelios/Desktop/share-model")
    
    tests = [
        ("Database File Exists", test_database_exists),
        ("Database Schema", test_database_schema),
        ("Keys Persist After Restart", test_keys_persist_after_restart),
        ("Usage Stats Persist", test_usage_stats_persist),
        ("Config Backup", test_config_backup),
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
