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

```bash
go test -bench . -benchmem ./bench
```

To run inverse FFT benchmarks:
```bash
go test -bench=BenchmarkIFFT -benchmem ./bench
```

To run single-precision (complex64) FFT benchmarks:
```bash
go test -bench=BenchmarkFFT32 -benchmem ./bench
```

To run single-precision (complex64) inverse FFT benchmarks:
```bash
go test -bench=BenchmarkIFFT32 -benchmem ./bench
```

To run accuracy tests:
```bash
go test -run=TestFFTRoundTrip ./bench
```

If FFTW is installed in a non-standard path, update `LD_LIBRARY_PATH` or install FFTW to a default search path (e.g. `/usr/local/lib`).

## Benchmark Results

See [BENCHMARKS.md](BENCHMARKS.md) for detailed benchmark results.

## Notes

- These benchmarks focus on 1D complex forward and inverse FFTs in double precision (`complex128`) and single precision (`complex64`).
- Only `algo-fft` currently supports single-precision benchmarking.
- `go-dsp/fft` allocates on every call (no reusable plan), so its results will include allocation overhead.
- Accuracy tests verify that FFT → IFFT round-trip preserves the original data within numerical tolerance.
