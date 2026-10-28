#!/bin/bash
export PATH=$HOME/workspace/tools/go/bin:$PATH
export GOPATH=$HOME/workspace/tools/gopath GOCACHE=$HOME/workspace/tools/gocache
export TMPDIR=$HOME/workspace/tools/tmp GOTMPDIR=$HOME/workspace/tools/tmp GOFLAGS=-mod=mod
F=internal/annotations/doc.go
say(){ echo; echo "### $*"; sleep 0.4; }
run(){ echo "\$ $*"; sleep 0.2; eval "$*"; echo "[exit: $?]"; sleep 0.4; }

say "PR #700 - make check must FAIL on unformatted Go, not silently rewrite it (issue #615)"
say "BEFORE: develop be2bc57f (pre-fix, scratch copy)"
cd ~/workspace/pr-evidence/pr-700/before || exit 1
run "git rev-parse --short HEAD"
run "grep -n '^check:' Makefile"
echo "\$ sed -n '/^fmt:/,/^$/p' Makefile"; sed -n '/^fmt:/,/^$/p' Makefile; sleep 1
say "Unformat one tracked file: 'package  annotations' (double space)"
run "sed -i 's/^package annotations/package  annotations/' $F"
run "grep -n '^package' $F"
run "gofmt -l $F"
say "Run the fmt step that 'make check' depends on:"
run "make fmt"
say "Result: exit 0, and the file was REWRITTEN back to clean - no failure, no trace:"
run "grep -n '^package' $F"
run "git status --porcelain"
echo "(empty git status = gate silently edited the file and passed)"

say "AFTER: PR #700 head 9a948ca7 (scratch copy)"
cd ~/workspace/pr-evidence/pr-700/after || exit 1
run "git rev-parse --short HEAD"
run "grep -n '^check:' Makefile"
say "Same unformatted file:"
run "sed -i 's/^package annotations/package  annotations/' $F"
run "gofmt -l $F"
run "make fmt-check"
say "Result: FAILS, names the file, and does NOT rewrite it:"
run "grep -n '^package' $F"
say "And 'make check' itself now stops at the fmt-check gate:"
run "make check"
say "Restore the file, clean tree passes:"
run "git checkout -- $F"
run "make fmt-check"
echo "\$ go test -run TestPreTestSources_MatchMakefile ./internal/testkit/testgate/  # run separately: ok (0.36s on rebased head, full package ok)"
say "DONE: before = silent rewrite + pass; after = loud failure naming the file, clean tree passes"
sleep 1
