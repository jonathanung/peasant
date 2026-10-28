#!/bin/bash
# PR #716 before/after demo — recorded with asciinema.
export PATH=$HOME/workspace/tools/go/bin:$PATH
export GOPATH=$HOME/workspace/tools/gopath
export GOCACHE=$HOME/workspace/tools/gocache
export TMPDIR=$HOME/workspace/tools/tmp
export GOTMPDIR=$HOME/workspace/tools/tmp
BEFORE=$HOME/workspace/pr-evidence/pr-716-before
AFTER=$HOME/workspace/pr-evidence/pr-716

echo "PR #716 — perf(indexformat): build the main-entry set once when validating title refs"
echo "BEFORE = develop be2bc57f (per-title-ref entryRefs() rebuild) + PR scaling test file"
echo "AFTER  = PR head f4daefbb (rebased onto be2bc57f, entry set built once)"
echo
echo "### 1) Scaling test BEFORE (256 vs 1024 entries: quadratic allocates ~16x)"
(cd "$BEFORE" && go test ./internal/indexformat/ -run 'TestGenerationValidateScalesLinearlyWithTitleRefs' -count=1 -v 2>&1) | grep -E 'allocated|FAIL|PASS|ok ' | head -6
echo
echo "### 2) Scaling test AFTER"
(cd "$AFTER" && go test ./internal/indexformat/ -run 'TestGenerationValidateScalesLinearlyWithTitleRefs' -count=1 -v 2>&1) | grep -E 'allocated|FAIL|PASS|ok ' | head -4
echo
echo "### 3) Benchmark BEFORE — BenchmarkGenerationValidateLongSession (1000 / 4000 entries)"
(cd "$BEFORE" && go test ./internal/indexformat/ -run '^$' -bench 'BenchmarkGenerationValidateLongSession' -benchmem -count=1 2>&1) | grep -E 'Benchmark|ok '
echo
echo "### 4) Benchmark AFTER — same benchmark, same machine"
(cd "$AFTER" && go test ./internal/indexformat/ -run '^$' -bench 'BenchmarkGenerationValidateLongSession' -benchmem -count=1 2>&1) | grep -E 'Benchmark|ok '
echo
echo "### 5) Full package tests AFTER (semantics unchanged)"
(cd "$AFTER" && go test ./internal/indexformat/ -count=1 2>&1) | tail -2
echo "DONE"
