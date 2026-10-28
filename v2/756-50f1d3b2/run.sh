#!/bin/bash
# Before/after evidence for peasant PR #756 (Strike lifecycle event kinds, #449)
# Usage: run.sh <label> <binary>
set -u
LABEL="$1"; BIN="$2"
E="$HOME/workspace/pr-evidence/pr-756-evidence"
D="$E/run-$LABEL"
rm -rf "$D"; mkdir -p "$D/home" "$D/source" "$D/export"
cp "$HOME/workspace/pr-evidence/pr-756/cmd/peasant/testdata/strike_event_kinds/"* "$D/source/"
export HOME="$D/home"
LEAD=CCCCCCCCCCCCCCCCCCCCCCCCCC; CHILD=DDDDDDDDDDDDDDDDDDDDDDDDDD; FORK=EEEEEEEEEEEEEEEEEEEEEEEEEE
echo "=== $LABEL: harvest Strike session set (lead contains all 49 lifecycle kinds) ==="
"$BIN" --data-dir "$D/data" --config-dir "$D/data" harvest \
  --source-harness strike --source-path "$D/source" \
  --output "$D/sync" --include-active --json 2>&1 | tee "$D/harvest.json" | head -c 1200; echo
echo "=== $LABEL: export each session ==="
for id in $LEAD $CHILD $FORK; do
  "$BIN" --data-dir "$D/data" --config-dir "$D/data" export sessions \
    --session "$id" --output-dir "$D/export" 2>&1 | tee "$D/export-$id.log" | head -5
done
ls -la "$D/export"
echo "=== $LABEL: summary ==="
python3 "$E/summarize.py" "$D/export" "$LABEL"
echo "=== $LABEL done ==="
