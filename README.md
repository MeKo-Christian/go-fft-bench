# go-fft-bench

Benchmarks comparing multiple Go FFT libraries, at power-of-two **and**
non-power-of-two lengths, with charts.

## Included libraries

- `algo-fft` (`github.com/cwbudde/algo-fft`) — both precisions
- `go-fftw` (`github.com/cwbudde/go-fftw`, requires FFTW shared libs) — the baseline
- `gonum` (`gonum.org/v1/gonum/dsp/fourier`)
- `go-dsp/fft` (`github.com/madelynnblue/go-dsp/fft`)
- `takatoh/fft` (`github.com/takatoh/fft`) — power-of-two only

## Sizes

Two size tables, both defined in [`bench/sizes.go`](bench/sizes.go):

- **Powers of two**, 8 up to `FFT_BENCH_MAX` (8192 by default, 32768 via the
  runner).
- **Non-power-of-two lengths**, hand-picked so that each one stands for a
  routing decision a planner has to make: 5-smooth, 7/11-smooth, primes with
  smooth `p-1` (where Rader's algorithm is cheap), primes with rough `p-1`
  (Bluestein territory), and lengths that turn up in practice such as 44100.

`takatoh` is excluded from the non-power-of-two benchmarks: it is radix-2 only
and returns a wrong answer rather than an error for other lengths.

## Run

### The full sweep

```bash
just sweep          # both builds, markdown + JSON
just plot           # render charts into plots/
just report         # sweep, then plot
```

`just sweep` runs the whole matrix twice — once for algo-fft's default SIMD
build and once for `-tags purego` — writing `BENCHMARKS.md`,
`BENCHMARKS-purego.md`, and machine-readable JSON under `results/`.

**It refuses to start on a busy machine.** The runner reads the 1-minute load
average and declines above a threshold derived from the core count
(`NumCPU/4`, minimum 1.5). Benchmarking against a compile storm measures the
contention, and nothing downstream can detect that from the results. Override
with `-max-load`, or pass `-wait-for-idle` to wait instead of failing:

```bash
MAX_LOAD=4 WAIT=30m just sweep
./bin/benchrunner -max-load 0            # disable the check entirely
```

### Single runs

```bash
just build
./bin/benchrunner -max-size 16384        # custom max size
./bin/benchrunner -show                  # print instead of writing the file
./bin/benchrunner -baseline algo-fft     # compare against something else
./bin/benchrunner -tags purego -label purego -json results/purego.json
./bin/benchrunner -help
```

Each run also records which algorithm algo-fft resolved for every benchmarked
length — `rader`, `bluestein`, a specific SIMD codelet — collected by a test
binary built with the same tags and running on the same CPU as the benchmarks.
Without it the non-power-of-two numbers are unreadable, because the same
library is running a different algorithm at each size.

### Charts

```bash
just plot
```

`cmd/fftplot` reads the JSON and renders seven charts with
[matplotlib-go](https://github.com/CWBudde/matplotlib-go):

| Chart | Shows |
| ----- | ----- |
| `01-throughput-pow2` | all five libraries, complex128, powers of two |
| `02-speedup-vs-fftw` | the same, as a ratio against FFTW3 |
| `03-nonpow2-by-class` | geometric mean speed vs FFTW3 per length class |
| `04-nonpow2-detail` | every non-power-of-two length, coloured by class |
| `05-precision` | complex64 vs complex128 |
| `06-simd-vs-purego` | what the hand-written codelets buy |
| `07-purego-vs-competitors` | the pure-Go build against the other Go libraries |

**Build the plotting tool with `-tags purego`** (the `just plot` recipe does).
matplotlib-go's default AGG backend compiles a cgo FreeType binding against a
vendored prefix that is not shipped in the module; the pure-Go text path
renders the same charts with no C dependency.

### Manual benchmarking

```bash
go test -bench . -benchmem ./bench                    # everything
go test -bench=BenchmarkFFTAny -benchmem ./bench      # arbitrary lengths only
go test -bench=BenchmarkFFT32 -benchmem ./bench       # single precision
FFT_BENCH_MAX=32768 go test -bench . -benchmem ./bench
just test-accuracy                                    # correctness gates
```

**Note:** If FFTW is installed in a non-standard path, update
`LD_LIBRARY_PATH` or install FFTW to a default search path (e.g.
`/usr/local/lib`).

## Benchmark Results

See [BENCHMARKS.md](BENCHMARKS.md) (default build) and
[BENCHMARKS-purego.md](BENCHMARKS-purego.md).

## Notes

- 1D complex forward and inverse transforms, `complex128` and `complex64`.
  Only `algo-fft` offers a single-precision API.
- Every library with a plan object gets one, built outside the timing loop.
  `go-dsp` and `takatoh` have no such API and allocate on every call.
- Libraries are interleaved per size rather than run in per-library blocks, so
  thermal drift over a long sweep lands on all of them equally and the ratios
  stay meaningful.
- `TestAnySizesAccuracy` cross-checks every non-power-of-two length against
  gonum before it is benchmarked. A fast wrong answer is not a result.
- `matplotlib-go` is wired in through a local `replace` directive: the API used
  here (plot methods returning `error`) is newer than its latest tag. Point it
  at a released version once one exists.
