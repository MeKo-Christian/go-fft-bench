# Repository Guidelines

## Project Structure & Module Organization
- `bench/` holds the benchmark suite (`bench_test.go`). All benchmark logic lives here.
- `cmd/benchrunner/` contains the automated benchmark runner tool (Go CLI)
- `bin/` contains built binaries (created by `just build`)
- `go.mod` defines the module and benchmark dependencies.
- `justfile` provides build recipes (`build`, `install`, `bench`, `test`, `clean`)
- `README.md` documents benchmark scope and usage.
- `BENCHMARKS.md` contains the latest benchmark results.

## Build, Test, and Development Commands

### Automated Benchmarking (Recommended)
- `just build` builds the benchrunner tool to `bin/benchrunner`
- `just bench` builds and runs benchmarks, updating `BENCHMARKS.md` automatically
- `just bench-size N` runs benchmarks with custom max size
- `just install` installs benchrunner to `GOPATH/bin` for global use
- `just help` shows all available recipes
- `./bin/benchrunner` runs all benchmarks and updates `BENCHMARKS.md` automatically
  - Runs FFT, IFFT, FFT32, IFFT32 benchmarks with sizes 8..32768 (configurable)
  - Parses results in real-time and generates formatted markdown
  - Calculates speedup vs baseline (default: go-fftw)
  - Use `-max-size N` to customize max FFT size
  - Use `-show` to print results to stdout instead of writing to file
  - Use `-baseline LIB` to use a different baseline library
  - Use `-help` to see all options

### Manual Benchmarking
- `go test -bench . -benchmem ./bench` runs the FFT benchmarks with allocation stats.
- `FFT_BENCH_MAX=32768 go test -bench . -benchmem ./bench` extends the benchmark size range beyond the default 8..8192.
- `go test ./...` is a quick sanity check; this repo primarily contains benchmarks, not unit tests.

## Coding Style & Naming Conventions
- Follow standard Go style: tabs for indentation, `gofmt` formatting, and idiomatic naming.
- Benchmark functions follow Go’s `BenchmarkXxx` pattern; sub-benchmarks use `b.Run` with `lib/size` names (e.g., `gonum/1024`).
- Keep helper names concise and descriptive (`benchGonum`, `fillComplex128`).

## Testing Guidelines
- Benchmarks are in `_test.go` files and run via `go test -bench`.
- There is no coverage target or unit-test framework configured beyond the Go toolchain.
- If you add tests, keep them near the benchmark file and name them `TestXxx`/`BenchmarkXxx`.

## Commit & Pull Request Guidelines
- Git history is not available in this workspace, so commit conventions are unknown.
- Use clear, imperative commit messages (e.g., "Add FFT size range flag") and include benchmark output changes in PR descriptions when relevant.
- PRs should explain which libraries or sizes changed and how to reproduce results.

## Configuration Notes
- `go-fftw` expects FFTW shared libraries. If they are not on a standard search path, set `LD_LIBRARY_PATH` (e.g., `/usr/local/lib`).
- `algo-fft` and `go-fftw` are pinned in `go.mod`; update versions there when needed.
