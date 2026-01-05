#!/usr/bin/env python3
import csv
import re
import sys

BENCH_RE = re.compile(
    r"^(Benchmark\S+)\s+"  # name
    r"(\d+)\s+"           # iterations
    r"([0-9.]+)\s+ns/op\s+"  # ns/op
    r"([0-9.]+)\s+MB/s\s+"    # MB/s
    r"(\d+)\s+B/op\s+"        # B/op
    r"(\d+)\s+allocs/op"       # allocs/op
)

NAME_RE = re.compile(r"^BenchmarkFFT/([^/]+)/([0-9]+)-\d+$")

writer = csv.writer(sys.stdout)
writer.writerow([
    "benchmark",
    "library",
    "size",
    "iterations",
    "ns_per_op",
    "mb_per_s",
    "bytes_per_op",
    "allocs_per_op",
])

for line in sys.stdin:
    line = line.strip()
    if not line.startswith("Benchmark"):
        continue
    match = BENCH_RE.match(line)
    if not match:
        continue

    name, iterations, ns_per_op, mb_per_s, bytes_per_op, allocs_per_op = match.groups()
    library = ""
    size = ""
    name_match = NAME_RE.match(name)
    if name_match:
        library, size = name_match.groups()

    writer.writerow([
        name,
        library,
        size,
        iterations,
        ns_per_op,
        mb_per_s,
        bytes_per_op,
        allocs_per_op,
    ])
