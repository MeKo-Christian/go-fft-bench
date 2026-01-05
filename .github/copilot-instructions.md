# Go FFT Bench - AI Coding Instructions

This repository contains benchmarks for various Go FFT libraries, focusing on 1D complex forward FFTs in double precision (`complex128`).

## Project Architecture & Context
- **Benchmark Suite**: All benchmarks are located in [bench/bench_test.go](bench/bench_test.go).
- **Local Dependencies**: The project uses `replace` directives in [go.mod](go.mod) for `algo-fft` and `go-fftw`. These are expected to be checked out in sibling directories (`../algo-fft` and `../go-fftw`).
- **FFTW Requirement**: `go-fftw` requires FFTW shared libraries installed on the system.

## Critical Workflows
- **Run Benchmarks**:
  ```bash
  go test -bench . -benchmem ./bench
  ```
- **Extend Benchmark Range**: Use the `FFT_BENCH_MAX` environment variable (default is 8192).
  ```bash
  FFT_BENCH_MAX=32768 go test -bench . -benchmem ./bench
  ```

## Coding Patterns & Conventions
- **Benchmark Structure**: Use `b.Run` to categorize by library and size:
  ```go
  b.Run(fmt.Sprintf("library-name/%d", n), func(b *testing.B) {
      // benchmark implementation
  })
  ```
- **Data Initialization**: Use the `fillComplex128` helper in [bench/bench_test.go](bench/bench_test.go#L110) to populate source buffers with consistent test data.
- **Memory Management**:
    - `algo-fft` and `gonum` use reusable plans.
    - `go-fftw` uses `fftw.NewArray` and requires `plan.Destroy()`.
    - `go-dsp/fft` allocates on every call; benchmarks should reflect this.
- **Performance Metrics**: Always use `b.ReportAllocs()` and `b.SetBytes(int64(n) * 16)` (for `complex128`) to provide meaningful throughput and allocation data.

## Integration Points
- **Adding a Library**:
    1. Add the dependency to [go.mod](go.mod).
    2. Create a `benchLibraryName(b *testing.B, n int)` function in [bench/bench_test.go](bench/bench_test.go).
    3. Register the new benchmark in the `BenchmarkFFT` loop.
