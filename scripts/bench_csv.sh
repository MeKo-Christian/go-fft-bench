#!/usr/bin/env bash
set -euo pipefail

GOAMD64=v3 go test -tags=asm -bench . -benchmem ./bench | "$(dirname "$0")/bench_to_csv.py"
