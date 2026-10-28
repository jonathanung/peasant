#!/bin/bash
# PR #756 before/after proof — Strike lifecycle event kinds (#449)
export TERM=xterm-256color GIT_PAGER=cat PAGER=cat LESS=-FX
E="$HOME/workspace/pr-evidence/pr-756-evidence"
pause(){ sleep "${1:-2}"; }
clear
echo "================================================================"
echo " PR #756 — feat(ingest): classify Strike lifecycle event kinds"
echo " Same synthetic Strike sessions (PR fixtures, cmd/peasant/"
echo " testdata/strike_event_kinds): lead holds ALL 49 lifecycle kinds,"
echo " plus a child (sidecar has leadSessionId only) and a fork."
echo "================================================================"
pause 3
echo
echo "########## BEFORE — develop be2bc57f ##########"
bash "$E/run.sh" before "$E/bin/peasant-before" 2>&1 | grep -v "^notice:"
echo
echo ">> BEFORE: lead export carries 49 retained-unknown records,"
echo ">> partial=true; agent.message / turn.steered text is NOT a turn (2 turns)."
pause 4
echo
echo "########## AFTER — PR head 50f1d3b2 (rebased on develop) ##########"
bash "$E/run.sh" after "$E/bin/peasant-after" 2>&1 | grep -v "^notice:"
echo
echo ">> AFTER: 0 retained-unknown, not partial, lead has 4 turns"
echo ">> (agent.message + turn.steered kept as user turns)."
echo ">> Child link / root / fork context_from / cache tokens are"
echo ">> asserted by TestStrikeLifecycleSessionExportsAndPublishes (cmd/peasant)."
pause 4
