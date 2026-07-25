#!/bin/sh
# Run the full benchmark sweep twice — once for the default SIMD build and
# once for algo-fft's pure-Go fallback — and write both the markdown tables
# and the JSON the plotting tool consumes.
#
# Both runs refuse to start while the machine is busy (see benchrunner's
# -max-load): a compile storm in another window skews every number, and
# nothing downstream can detect that afterwards. Override the threshold with
# MAX_LOAD, and the patience with WAIT.
#
#   sh scripts/sweep.sh [max-size]
#   MAX_LOAD=4 WAIT=30m sh scripts/sweep.sh 8192
set -eu

MAX_SIZE="${1:-32768}"
WAIT="${WAIT:-3h}"

# Empty MAX_LOAD means "use benchrunner's core-count-derived default".
if [ -n "${MAX_LOAD:-}" ]; then
	LOAD_FLAG="-max-load $MAX_LOAD"
else
	LOAD_FLAG=""
fi

cd "$(dirname "$0")/.."
mkdir -p results

go build -o bin/benchrunner ./cmd/benchrunner

echo "=== SIMD build (default) ==="
# shellcheck disable=SC2086 # LOAD_FLAG is intentionally word-split or empty
./bin/benchrunner \
	-max-size "$MAX_SIZE" \
	-label simd \
	-json results/simd.json \
	-output BENCHMARKS.md \
	-wait-for-idle "$WAIT" \
	$LOAD_FLAG

echo
echo "=== pure-Go build (-tags purego) ==="
# shellcheck disable=SC2086
./bin/benchrunner \
	-max-size "$MAX_SIZE" \
	-tags purego \
	-label purego \
	-json results/purego.json \
	-output BENCHMARKS-purego.md \
	-wait-for-idle "$WAIT" \
	$LOAD_FLAG

echo
echo "Done. Render the charts with:"
echo "  just plot"
