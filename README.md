# go-fft-bench

Benchmarks comparing multiple Go FFT libraries across power-of-two sizes.

## Included libraries

- `algo-fft` (`github.com/MeKo-Christian/algo-fft`)
- `go-fftw` (`github.com/meko-christian/go-fftw`, requires FFTW shared libs)
- `gonum` (`gonum.org/v1/gonum/dsp/fourier`)
- `go-dsp/fft` (`github.com/mjibson/go-dsp/fft`)
- `takatoh/fft` (`github.com/takatoh/fft`)

## Sizes

By default, benchmarks run sizes 8..8192 (powers of two). You can extend the range:

```bash
FFT_BENCH_MAX=32768 go test -bench . -benchmem ./bench
```

## Run

### Quick Start - Automated Benchmarking

Build and run the benchrunner tool to generate formatted results in one command:

```bash
# Build and run benchmarks in one step
just bench

# Or build separately
just build
./bin/benchrunner

# See all available recipes
just help
```

This will:
- Run all benchmark types (FFT, IFFT, FFT32, IFFT32)
- Test all sizes from 8 to 32768
- Generate formatted markdown with baseline comparisons
- Update `BENCHMARKS.md` automatically

**Options:**

```bash
# Run with custom max size
just bench-size 16384

# Or use the binary directly
./bin/benchrunner -max-size 16384

# Print to stdout instead of updating file
./bin/benchrunner -show

# Use different baseline library
./bin/benchrunner -baseline algo-fft

# See all options
./bin/benchrunner -help
```

**Install globally (optional):**

```bash
just install
# Then use anywhere:
benchrunner
```

### Manual Benchmarking

Run specific benchmark types manually:

```bash
# Run all benchmarks
go test -bench . -benchmem ./bench

# Run inverse FFT benchmarks
go test -bench=BenchmarkIFFT -benchmem ./bench

# Run single-precision (complex64) FFT benchmarks
go test -bench=BenchmarkFFT32 -benchmem ./bench

# Run single-precision (complex64) inverse FFT benchmarks
go test -bench=BenchmarkIFFT32 -benchmem ./bench

# Run with extended size range
FFT_BENCH_MAX=32768 go test -bench . -benchmem ./bench

# Run accuracy tests
go test -run=TestFFTRoundTrip ./bench
```

**Note:** If FFTW is installed in a non-standard path, update `LD_LIBRARY_PATH` or install FFTW to a default search path (e.g. `/usr/local/lib`).

## Benchmark Results

See [BENCHMARKS.md](BENCHMARKS.md) for detailed benchmark results.

## Notes

- These benchmarks focus on 1D complex forward and inverse FFTs in double precision (`complex128`) and single precision (`complex64`).
- Only `algo-fft` currently supports single-precision benchmarking.
- `go-dsp/fft` allocates on every call (no reusable plan), so its results will include allocation overhead.
- Accuracy tests verify that FFT → IFFT round-trip preserves the original data within numerical tolerance.
