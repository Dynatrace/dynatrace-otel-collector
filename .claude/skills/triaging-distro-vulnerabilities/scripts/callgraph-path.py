#!/usr/bin/env python3
# Shortest call path from main to each target function in a saved callgraph file.
# Edge format per line: "caller --> callee". Root is the distro main function.
#
# usage: python3 callgraph-path.py <callgraph-file> <target-function>...
import sys, collections
g = collections.defaultdict(list)
for line in open(sys.argv[1]):
    a, sep, b = line.rstrip("\n").partition(" --> ")
    if sep: g[a].append(b)
root = "github.com/Dynatrace/dynatrace-otel-collector.main"
for target in sys.argv[2:]:
    prev, q, hit = {root: None}, collections.deque([root]), False
    while q:
        n = q.popleft()
        if n == target: hit = True; break
        for m in g.get(n, ()):
            if m not in prev: prev[m] = n; q.append(m)
    print("\n==", target)
    if not hit: print("  unreachable from main"); continue
    path, n = [], target
    while n: path.append(n); n = prev[n]
    for i, n in enumerate(reversed(path)): print(f"  {i:2d} {n}")
