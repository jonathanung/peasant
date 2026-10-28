#!/bin/bash
# PR #715 evidence: --port validation for web start / web stop / open (issue #681)
BIN_BEFORE=/home/hatch/workspace/pr-evidence/pr-715-evidence/bin/peasant-before
BIN_AFTER=/home/hatch/workspace/pr-evidence/pr-715-evidence/bin/peasant-after

setup_home() {
  export HOME="$(mktemp -d)"
  case "$HOME" in /tmp/*) ;; *) echo "refusing: HOME is not a scratch dir" >&2; exit 1;; esac
  export XDG_CONFIG_HOME="$HOME/.config" XDG_DATA_HOME="$HOME/.local/share" XDG_STATE_HOME="$HOME/.local/state"
}
cleanup_home() {
  pkill -x peasant-before 2>/dev/null; pkill -x peasant-after 2>/dev/null; sleep 1
  rm -rf "$HOME"
}

echo '############################################################'
echo '# BEFORE: develop (be2bc57f) - peasant web start --port ...'
echo '############################################################'
setup_home
for p in 0 -1 70000; do
  echo "\$ peasant web start --no-browser --port $p"
  timeout 30 "$BIN_BEFORE" web start --no-browser --port "$p"
  echo "exit=$?"
done
echo '$ ls $XDG_STATE_HOME/peasant/'
ls "$XDG_STATE_HOME/peasant/"
cleanup_home

echo
echo '############################################################'
echo '# AFTER: PR #715 (qa/pr-port-range) - same commands'
echo '############################################################'
setup_home
for p in 0 -1 70000; do
  echo "\$ peasant web start --no-browser --port $p"
  timeout 30 "$BIN_AFTER" web start --no-browser --port "$p"
  echo "exit=$?"
done
echo '$ peasant web stop --port 70000'
timeout 30 "$BIN_AFTER" web stop --port 70000
echo "exit=$?"
echo '$ peasant open --port 70000'
timeout 30 "$BIN_AFTER" open --port 70000
echo "exit=$?"
echo '$ ls $XDG_STATE_HOME/peasant/   (expect: no PID files)'
ls "$XDG_STATE_HOME/peasant/" 2>&1

echo
echo '############################################################'
echo '# AFTER: a valid port still starts and stops normally'
echo '############################################################'
echo '$ peasant web start --no-browser --port 18715'
timeout 30 "$BIN_AFTER" web start --no-browser --port 18715
echo "exit=$?"
echo '$ ls $XDG_STATE_HOME/peasant/'
ls "$XDG_STATE_HOME/peasant/"
echo '$ peasant web stop --port 18715'
timeout 30 "$BIN_AFTER" web stop --port 18715
echo "exit=$?"
echo '$ ls $XDG_STATE_HOME/peasant/   (expect: PID file removed)'
ls "$XDG_STATE_HOME/peasant/" 2>&1
cleanup_home
echo 'DONE'
