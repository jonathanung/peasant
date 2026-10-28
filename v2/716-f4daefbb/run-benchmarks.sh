#!/bin/bash
# Before/after benchmark for PR #716 — same machine, interleaved rounds.
export PATH=$HOME/workspace/tools/go/bin:$PATH
export GOPATH=$HOME/workspace/tools/gopath
export GOCACHE=$HOME/workspace/tools/gocache
export TMPDIR=$HOME/workspace/tools/tmp
export GOTMPDIR=$HOME/workspace/tools/tmp
mkdir -p "$TMPDIR"
BEFORE=$HOME/workspace/pr-evidence/pr-716-before
AFTER=$HOME/workspace/pr-evidence/pr-716
OUT=$HOME/workspace/pr-evidence/pr-716/evidence/bench-raw.txt
: > "$OUT"

echo "=== scaling test: BEFORE (develop be2bc57f + PR test file only) ===" | tee -a "$OUT"
(cd "$BEFORE" && go test ./internal/indexformat/ -run 'TestGenerationValidateScalesLinearlyWithTitleRefs' -count=1 -v 2>&1) | tail -8 | tee -a "$OUT"
echo "=== scaling test: AFTER (PR head, rebased) ===" | tee -a "$OUT"
(cd "$AFTER" && go test ./internal/indexformat/ -run 'TestGenerationValidateScalesLinearlyWithTitleRefs' -count=1 -v 2>&1) | tail -8 | tee -a "$OUT"

for round in 1 2 3; do
  echo "=== benchmark round $round: BEFORE ===" | tee -a "$OUT"
  (cd "$BEFORE" && go test ./internal/indexformat/ -run '^$' -bench 'BenchmarkGenerationValidateLongSession' -benchmem -count=1 2>&1) | grep -E 'Benchmark|ok ' | tee -a "$OUT"
  echo "=== benchmark round $round: AFTER ===" | tee -a "$OUT"
  (cd "$AFTER" && go test ./internal/indexformat/ -run '^$' -bench 'BenchmarkGenerationValidateLongSession' -benchmem -count=1 2>&1) | grep -E 'Benchmark|ok ' | tee -a "$OUT"
done
echo "ALL DONE" | tee -a "$OUT"
