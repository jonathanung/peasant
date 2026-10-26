#!/usr/bin/env python3
"""Resolve CHANGELOG.md conflicts inside [Unreleased]: keep both sides (ours first, i.e. the
earlier-merged PR), then regroup the Unreleased entries under one heading per Keep a Changelog
type, in canonical order, keeping each entry's original order. Usage: changelog_union.py CHANGELOG.md"""
import re, sys
p = sys.argv[1]; s = open(p).read()
s = re.sub(r'<<<<<<< [^\n]*\n(.*?)=======\n(.*?)>>>>>>> [^\n]*\n', lambda m: m.group(1) + m.group(2), s, flags=re.S)
assert '<<<<<<<' not in s and '>>>>>>>' not in s
start = s.index('## [Unreleased]\n') + len('## [Unreleased]\n')
end = s.index('\n## [', start)
body = s[start:end]
order = ['Added', 'Changed', 'Deprecated', 'Removed', 'Fixed', 'Security']
groups, cur = {}, None
for line in body.split('\n'):
    m = re.match(r'### (\w+)\s*$', line)
    if m:
        cur = m.group(1); groups.setdefault(cur, []); continue
    if cur is None:
        assert line.strip() == '', 'text before first heading: %r' % line
        continue
    groups[cur].append(line)
out = []
for k in order + sorted(set(groups) - set(order)):
    if k in groups:
        lines = [l for l in groups[k]]
        while lines and not lines[-1].strip(): lines.pop()
        while lines and not lines[0].strip(): lines.pop(0)
        lines = [l for i, l in enumerate(lines) if l.strip() or (i + 1 < len(lines) and not lines[i+1].startswith('  ') and lines[i+1].strip())]
        out.append('\n### %s\n' % k + '\n'.join(lines) + '\n')
s = s[:start] + ''.join(out) + s[end:]
open(p, 'w').write(s)
