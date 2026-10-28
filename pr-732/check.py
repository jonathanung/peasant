#!/usr/bin/env python3
"""Check README harvest/verify flag tables against real `peasant harvest --help`.

Usage: check.py <README.md> <peasant-binary>

Checks, per command:
  * every flag in the README table exists in --help (no UNKNOWN)
  * every command-specific flag in --help is in the README table (no UNDOCUMENTED)
  * global flags (--config/--data-dir/...) need be documented once, in the
    harvest table; the verify table is checked for covering them via harvest.
Also compares the --source-harness value lists. --help/--version excluded.
"""
import re, subprocess, sys

readme_path, binary = sys.argv[1], sys.argv[2]
lines = open(readme_path).read().splitlines()

def section(kind):
    out, capture = [], False
    for ln in lines:
        if ln.startswith("### "):
            is_verify = "verify" in ln and "flags" in ln
            is_main = ("harvest" in ln or "ingest" in ln) and "flags" in ln and "verify" not in ln
            capture = is_verify if kind == "verify" else is_main
            continue
        if capture and ln.startswith("## "):
            break
        if capture:
            out.append(ln)
    return out

def readme_flags(kind):
    flags = []
    for ln in section(kind):
        if ln.startswith("|") and "`--" in ln:
            flags += re.findall(r"--[a-z][a-z-]*", ln.split("|")[1])
    return sorted(set(flags))

def help_flags(args):
    out = subprocess.run([binary] + args + ["--help"], capture_output=True, text=True).stdout
    local, glob, mode = [], [], None
    for ln in out.splitlines():
        if ln.startswith("Flags:"):
            mode = "local"; continue
        if ln.startswith("Global Flags:"):
            mode = "global"; continue
        if ln.startswith("Use "):
            mode = None
        if mode:
            (local if mode == "local" else glob).extend(re.findall(r"--[a-z][a-z-]*", ln))
    drop = ("--help", "--version")
    return (sorted(set(f for f in local if f not in drop)),
            sorted(set(f for f in glob if f not in drop)))

def harness_values(text_lines, is_help):
    for ln in text_lines:
        if "--source-harness" in ln:
            if is_help:
                m = re.search(r"\(([^)]*)\)\s*$", ln.strip())
                return sorted(m.group(1).split(", ")) if m else []
            vals = re.findall(r"`([a-z][a-z-]*)`", ln.split("|")[2] if "|" in ln else ln)
            return sorted(v for v in vals if v != "--source-harness")
    return []

fail = 0
readme_harvest = readme_flags("harvest")
for kind, args in (("harvest", ["harvest"]), ("verify", ["harvest", "verify"])):
    r = readme_flags(kind)
    local, glob = help_flags(args)
    unknown = sorted(set(r) - set(local) - set(glob))
    undoc_local = sorted(set(local) - set(r))
    undoc_glob = sorted(set(glob) - set(r) - set(readme_harvest))
    print(f"== {kind}: README flags ({len(r)}): {' '.join(r)}")
    print(f"== {kind}: --help command flags ({len(local)}): {' '.join(local)}")
    print(f"== {kind}: --help global flags ({len(glob)}): {' '.join(glob)}")
    print(f"UNKNOWN (in README, not in --help): {' '.join(unknown) or 'none'}")
    print(f"UNDOCUMENTED command flags: {' '.join(undoc_local) or 'none'}")
    print(f"UNDOCUMENTED global flags (not in this table nor the harvest table): {' '.join(undoc_glob) or 'none'}")
    ok = not unknown and not undoc_local and not undoc_glob
    print(f"{kind} diff: {'IDENTICAL (command flags; globals documented)' if ok else 'MISMATCH'}")
    if not ok:
        fail = 1

help_out = subprocess.run([binary, "harvest", "--help"], capture_output=True, text=True).stdout.splitlines()
rh, hh = harness_values(section("harvest"), False), harness_values(help_out, True)
print(f"== source-harness values README ({len(rh)}): {' '.join(rh)}")
print(f"== source-harness values --help ({len(hh)}): {' '.join(hh)}")
print(f"harness diff: {'IDENTICAL' if rh == hh else 'MISMATCH'}")
if rh != hh:
    fail = 1
print("RESULT:", "PASS - README tables match --help" if fail == 0 else "FAIL - mismatches above")
sys.exit(fail)
