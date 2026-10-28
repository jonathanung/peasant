#!/usr/bin/env python3
"""Summarize one evidence run dir (harvest + exports) for PR #756."""
import json, glob, sys, os, collections

d = sys.argv[1]
label = sys.argv[2] if len(sys.argv) > 2 else os.path.basename(d)
print(f"--- {label} ---")
# harvest log: warnings + retained kinds summary if present
harv = os.path.join(os.path.dirname(d.rstrip('/')), f"run-{label}", "harvest.json")
for p in sorted(glob.glob(os.path.join(d, "*.json"))):
    doc = json.load(open(p))
    ru = doc.get("retainedUnknown") or []
    kinds = collections.Counter(r.get("kind") for r in ru)
    partial = (doc.get("diagnostics") or {}).get("partial")
    turns = doc.get("turns") or []
    print(f"{doc.get('id','?')[:8]}… export: retainedUnknown={len(ru)} partial={partial} turns={len(turns)}")
    if kinds:
        print(f"   retained kinds ({len(kinds)} distinct): {', '.join(sorted(kinds))}")
