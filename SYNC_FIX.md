# Modeltunnel CLI/Server Synchronization Fix

## Problem
Keys created via CLI (`modeltunnel key create`) were not recognized by the running server until restart.

## Solution Implemented
Added automatic key refresh from database every 5 seconds.

### Changes Made

**File: `internal/keys/store.go`**

1. Added `startKeyRefresher()` goroutine that runs every 5 seconds
2. Added `refreshFromDB()` method that:
   - Reloads keys from SQLite database
   - Adds new keys without losing usage stats
   - Updates existing keys' mutable fields
   - Removes revoked keys
   - Preserves in-memory usage statistics

### How It Works

```
Server starts
    ↓
Load keys from DB (initial load)
    ↓
Start background refresher (every 5 seconds)
    ↓
CLI: Create key → Saved to DB
    ↓
Server: Next refresh cycle → New key loaded
    ↓
Key is immediately usable (within 5 seconds max)
```

### Performance Impact
- Minimal: Only reads from DB every 5 seconds
- No writes during refresh
- Preserves all in-memory stats
- Thread-safe with proper locking

## Testing Results

✅ **Before Fix:**
- CLI: Create key → Saved to DB ✓
- CLI: List keys → Shows new key ✓  
- Server: API call with new key → 401 Unauthorized ✗
- Required server restart to use key

✅ **After Fix:**
- CLI: Create key → Saved to DB ✓
- CLI: List keys → Shows new key ✓
- Server: API call with new key → Works within 2-5 seconds ✓
- No restart required!

## Usage Example

```bash
# Terminal 1: Start server
./modeltunnel up --ollama --model mistral

# Terminal 2: Create key
./modeltunnel key create newuser --models mistral,phi
# Output: API Key: mt_sk_newuser_abc123...

# Immediately use the key (within 5 seconds):
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mt_sk_newuser_abc123..." \
  -H "Content-Type: application/json" \
  -d '{"model": "ollama/phi:latest", "messages": [{"role": "user", "content": "Hi"}]}'
# ✅ Works immediately!
```

## Technical Details

- **Refresh Interval:** 5 seconds (configurable if needed)
- **Thread Safety:** Uses existing mutex locks
- **Usage Stats:** Preserved across refreshes
- **Revoked Keys:** Automatically removed from memory
- **Database Errors:** Logged but don't crash server

## Backward Compatibility

✅ Fully backward compatible:
- Works with existing databases
- No changes to CLI commands
- No changes to API behavior
- No configuration changes needed
