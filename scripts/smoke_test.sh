#!/usr/bin/env bash
set -euo pipefail

PORT="${PORT:-8082}"
BIN="${BIN:-./build/modeltunnel}"

LOG="/tmp/modeltunnel_smoke_${PORT}.log"
PIDFILE="/tmp/modeltunnel_smoke_${PORT}.pid"
KEYFILE="/tmp/modeltunnel_smoke_key.txt"
KEYNAMEFILE="/tmp/modeltunnel_smoke_key_name.txt"

export PORT KEYFILE KEYNAMEFILE

pass() { printf "PASS: %s\n" "$1"; }
fail() { printf "FAIL: %s\n" "$1"; exit 1; }

"$BIN" up --port "$PORT" >"$LOG" 2>&1 &
echo $! >"$PIDFILE"

cleanup() {
  if [ -f "$PIDFILE" ]; then
    kill "$(cat "$PIDFILE")" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# Wait for server
for _ in $(seq 1 60); do
  code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/health" || true)
  if [ "$code" = "200" ]; then
    break
  fi
  sleep 0.25
done

# Pick a usable API key (prefer 'ratetest', else first key). Never print the key.
python3 - <<'PY'
import json
import os
import urllib.request

port = int(os.environ.get("PORT", "8082"))
keyfile = os.environ["KEYFILE"]
keynamefile = os.environ["KEYNAMEFILE"]

url = f"http://127.0.0.1:{port}/admin/api/keys"
with urllib.request.urlopen(url) as resp:
    data = json.loads(resp.read().decode("utf-8"))

keys = data.get("keys") or []
chosen = None
for k in keys:
    if k.get("name") == "ratetest":
        chosen = k
        break
if chosen is None and keys:
    chosen = keys[0]
if chosen is None:
    raise SystemExit("No keys available to run smoke tests")

open(keyfile, "w").write(chosen["key"])
open(keynamefile, "w").write(chosen.get("name", ""))
PY

KEY=$(python3 -c 'from pathlib import Path; print(Path("/tmp/modeltunnel_smoke_key.txt").read_text().strip())')
KEYNAME=$(python3 -c 'from pathlib import Path; print(Path("/tmp/modeltunnel_smoke_key_name.txt").read_text().strip())')

export KEY

# 1) Health
code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/health")
[ "$code" = "200" ] && pass "/health" || fail "/health expected 200 got $code"

# 2) Models requires auth
code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/v1/models")
[ "$code" = "401" ] && pass "/v1/models requires auth" || fail "/v1/models expected 401 got $code"

# 3) Models with auth
code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/v1/models" -H "Authorization: Bearer $KEY")
[ "$code" = "200" ] && pass "/v1/models with auth" || fail "/v1/models expected 200 got $code"

# 4) Chat completion
code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
  -d '{"model":"default/tinyllama:latest","messages":[{"role":"user","content":"Hi"}],"max_tokens":8}')
[ "$code" = "200" ] && pass "/v1/chat/completions" || fail "/v1/chat/completions expected 200 got $code"

# 5) Intent routing (auto)
HDRS=$(mktemp)
code=$(curl -s -D "$HDRS" -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/v1/chat/completions" \
  -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" -H "X-Model-Intent: code" \
  -d '{"model":"auto","messages":[{"role":"user","content":"Write a one-line bash command to print hello."}],"max_tokens":32}')
if [ "$code" != "200" ]; then
  rm -f "$HDRS"
  fail "intent routing expected 200 got $code"
fi
export HDRS
if python3 - <<'PY'
import os
import re
from pathlib import Path

h = Path(os.environ["HDRS"]).read_text()
if not re.search(r"^X-Routed-Model:", h, flags=re.M):
    raise SystemExit(1)
PY
then
  pass "intent routing sets X-Routed-Model"
else
  fail "missing X-Routed-Model header"
fi
rm -f "$HDRS"

# 6) Streaming returns SSE 'data:' prefix
python3 - <<'PY'
import json
import os
import urllib.request

port = int(os.environ.get("PORT", "8082"))
key = os.environ["KEY"]

req = urllib.request.Request(
    f"http://127.0.0.1:{port}/v1/chat/completions",
    method="POST",
    headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
    data=json.dumps({
        "model": "default/tinyllama:latest",
        "messages": [{"role": "user", "content": "Say hello in 2 words."}],
        "stream": True,
        "max_tokens": 8,
    }).encode("utf-8"),
)
with urllib.request.urlopen(req, timeout=30) as resp:
    chunk = resp.read(200)
text = chunk.decode("utf-8", errors="replace")
if "data:" not in text:
    raise SystemExit("missing SSE data: prefix")
PY
pass "streaming SSE"

# 7) Async job submit + status
python3 - <<'PY'
import json
import os
import time
import urllib.request

port = int(os.environ.get("PORT", "8082"))
key = os.environ["KEY"]

submit = urllib.request.Request(
    f"http://127.0.0.1:{port}/v1/async",
    method="POST",
    headers={"Authorization": f"Bearer {key}", "Content-Type": "application/json"},
    data=json.dumps({
        "model": "default/tinyllama:latest",
        "messages": [{"role": "user", "content": "Return the word OK."}],
        "max_tokens": 5,
    }).encode("utf-8"),
)
with urllib.request.urlopen(submit, timeout=15) as resp:
    if resp.status != 202:
        raise SystemExit(f"expected 202 got {resp.status}")
    out = json.loads(resp.read().decode("utf-8"))
job_id = out.get("job_id")
if not job_id:
    raise SystemExit("missing job_id")

status_url = f"http://127.0.0.1:{port}/v1/jobs/{job_id}"
for _ in range(60):
    r = urllib.request.Request(status_url, headers={"Authorization": f"Bearer {key}"})
    with urllib.request.urlopen(r, timeout=10) as resp:
        job = json.loads(resp.read().decode("utf-8"))
    st = job.get("status")
    if st in ("completed", "failed"):
        break
    time.sleep(0.25)
else:
    raise SystemExit("job did not complete in time")
if st != "completed":
    raise SystemExit(f"job status {st}")
PY
pass "async jobs"

# 8) Admin endpoints basic sanity
code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/admin")
[ "$code" = "200" ] && pass "/admin" || fail "/admin expected 200 got $code"

code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/admin/api/config")
[ "$code" = "200" ] && pass "/admin/api/config GET" || fail "/admin/api/config GET expected 200 got $code"

# Validate+save config by POSTing the current config back
python3 - <<'PY'
import json
import os
import urllib.request

port = int(os.environ.get("PORT", "8082"))
cfg = json.loads(urllib.request.urlopen(f"http://127.0.0.1:{port}/admin/api/config").read().decode("utf-8"))["config"]
req = urllib.request.Request(
    f"http://127.0.0.1:{port}/admin/api/config",
    method="POST",
    headers={"Content-Type": "application/json"},
    data=json.dumps({"config": cfg}).encode("utf-8"),
)
with urllib.request.urlopen(req) as resp:
    if resp.status != 200:
        raise SystemExit(f"expected 200 got {resp.status}")
PY
pass "config validate+save"

code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/admin/api/models")
[ "$code" = "200" ] && pass "/admin/api/models" || fail "/admin/api/models expected 200 got $code"

code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/admin/api/tunnel")
[ "$code" = "200" ] && pass "/admin/api/tunnel" || fail "/admin/api/tunnel expected 200 got $code"

code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/admin/api/network")
[ "$code" = "200" ] && pass "/admin/api/network" || fail "/admin/api/network expected 200 got $code"

code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/admin/api/providers/types")
[ "$code" = "200" ] && pass "/admin/api/providers/types" || fail "/admin/api/providers/types expected 200 got $code"

code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/admin/api/providers")
if [ "$code" = "200" ] || [ "$code" = "503" ]; then
  pass "/admin/api/providers (200 or 503)"
else
  fail "/admin/api/providers expected 200 or 503 got $code"
fi

# 9) Rate limit retry-after sanity (only if we have a 2/min ratetest key)
if [ "$KEYNAME" = "ratetest" ]; then
  HDRS=$(mktemp)
  curl -s -D "$HDRS" -o /dev/null "http://127.0.0.1:${PORT}/v1/chat/completions" \
    -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
    -d '{"model":"default/tinyllama:latest","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}' >/dev/null
  curl -s -D "$HDRS" -o /dev/null "http://127.0.0.1:${PORT}/v1/chat/completions" \
    -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
    -d '{"model":"default/tinyllama:latest","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}' >/dev/null
  code=$(curl -s -D "$HDRS" -o /dev/null -w "%{http_code}" "http://127.0.0.1:${PORT}/v1/chat/completions" \
    -H "Authorization: Bearer $KEY" -H "Content-Type: application/json" \
    -d '{"model":"default/tinyllama:latest","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}')
  if [ "$code" != "429" ]; then
    rm -f "$HDRS"
    fail "rate limit expected 429 got $code"
  fi
  export HDRS
  if python3 - <<'PY'
import os
import re
from pathlib import Path

h = Path(os.environ["HDRS"]).read_text()
m = re.search(r"^Retry-After: (\d+)$", h, flags=re.M)
if not m:
    raise SystemExit("missing Retry-After")
val = int(m.group(1))
if not (0 < val <= 60):
    raise SystemExit(f"bad Retry-After: {val}")
PY
  then
    pass "rate limit Retry-After sane"
  else
    fail "rate limit Retry-After invalid"
  fi
  rm -f "$HDRS"
else
  pass "rate limit test skipped (no ratetest key)"
fi

printf "\nAll smoke checks passed on port %s using key '%s' (value not printed).\n" "$PORT" "$KEYNAME"
