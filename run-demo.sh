#!/bin/bash
# Before/after demo for PR #705 / issue #676: --data-dir must own peasant-sync.
# Usage: run-demo.sh <label> <peasant-binary>
set -u
LABEL="$1"
BIN="$2"
ROOT="$HOME_DEMO_ROOT"
H="$ROOT/home-$LABEL"
rm -rf "$H"
mkdir -p "$H/.local/share" "$H/.config" "$H/.local/state" "$H/src/projects/-tmp-qa-proj"
SID="aaaaaaaa-0000-4000-8000-000000000001"
cat > "$H/src/projects/-tmp-qa-proj/$SID.jsonl" <<'JSONL'
{"type":"user","sessionId":"aaaaaaaa-0000-4000-8000-000000000001","uuid":"u1","cwd":"/tmp/qa-proj","timestamp":"2026-10-01T10:00:00Z","message":{"role":"user","content":"hello"}}
{"type":"assistant","sessionId":"aaaaaaaa-0000-4000-8000-000000000001","uuid":"a1","parentUuid":"u1","cwd":"/tmp/qa-proj","timestamp":"2026-10-01T10:00:05Z","message":{"role":"assistant","model":"claude-sonnet-4-5","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":3,"output_tokens":2}}}
JSONL
printf 'version: 1\nsources:\n  claude-code:\n    enabled: false\n  opencode:\n    enabled: false\n  cursor:\n    enabled: false\n  pi:\n    enabled: false\n' > "$H/config.yaml"
export HOME="$H"
export XDG_DATA_HOME="$H/.local/share"
export XDG_CONFIG_HOME="$H/.config"
export XDG_STATE_HOME="$H/.local/state"
DD="$H/dd"
echo "=================================================================="
echo "== $LABEL: $($BIN --version 2>/dev/null | head -1)"
echo "== HOME=$H"
echo "== XDG_DATA_HOME (default) = $XDG_DATA_HOME"
echo "== --data-dir              = $DD"
echo "------------------------------------------------------------------"
echo "\$ peasant --data-dir \$DD harvest --source-harness claude-code --source-path \$HOME/src/projects"
"$BIN" --data-dir "$DD" --config-dir "$H/cfg" --config "$H/config.yaml" harvest --source-harness claude-code --source-path "$H/src/projects" 2>&1 | grep -E "Output:|ingest|session|error" | head -8
echo "------------------------------------------------------------------"
echo "-- files under --data-dir (\$DD):"
find "$DD" | sed "s|$H|~|" | sort
echo "-- peasant-sync copies under --data-dir:"
find "$DD" -path '*peasant-sync*' -type f | sed "s|$H|~|" | sort
echo "   count: $(find "$DD" -path '*peasant-sync*' -type f 2>/dev/null | wc -l)"
echo "-- peasant-sync copies under default XDG_DATA_HOME:"
find "$XDG_DATA_HOME" -path '*peasant-sync*' -type f 2>/dev/null | sed "s|$H|~|" | sort
echo "   count: $(find "$XDG_DATA_HOME" -path '*peasant-sync*' -type f 2>/dev/null | wc -l)"
echo "-- database:"
find "$DD" "$XDG_DATA_HOME" -name '*.db' 2>/dev/null | sed "s|$H|~|" | sort
echo "=================================================================="
