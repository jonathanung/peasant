#!/bin/bash
export HOME_DEMO_ROOT=/tmp/pr705-demo
E=$HOME/workspace/pr-evidence/pr-705-evidence
echo "PR #705 / issue #676 -- --data-dir must keep peasant-sync under <data-dir>/peasant"
echo "One synthetic Claude session in a throwaway HOME; harvest with --data-dir \$DD."
echo
sleep 1
bash "$E/run-demo.sh" BEFORE-develop "$E/bin/peasant-before"
sleep 2
bash "$E/run-demo.sh" AFTER-pr705 "$E/bin/peasant-after"
sleep 1
echo
echo "RESULT: BEFORE (develop be2bc57f): Output + copies go to default XDG_DATA_HOME, 0 under --data-dir."
echo "        AFTER  (PR #705 cee1b4e7): Output + copies under <data-dir>/peasant/peasant-sync, 0 under XDG."
sleep 2
