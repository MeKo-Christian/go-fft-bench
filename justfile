# Show available recipes
help:
	@just --list

# Build the benchrunner tool
build:
	@echo "Building benchrunner..."
	@go build -o bin/benchrunner ./cmd/benchrunner
	@echo "Built: bin/benchrunner"

# Install benchrunner to GOPATH/bin
install:
	@echo "Installing benchrunner..."
	@go install ./cmd/benchrunner
	@echo "Installed to $(go env GOPATH)/bin/benchrunner"

# Run all benchmarks and update BENCHMARKS.md
bench: build
	@echo "Running benchmarks..."
	@./bin/benchrunner

# Run benchmarks with custom max size
bench-size SIZE: build
	@./bin/benchrunner -max-size {{SIZE}}

# Full sweep: the default SIMD build and the pure-Go build, writing both
# markdown tables and the JSON the plotting tool reads. Waits for the machine
# to go idle first — benchmarking under load measures the contention.
sweep SIZE="32768":
	@sh scripts/sweep.sh {{SIZE}}

# Render the charts from the sweep results into plots/.
# -tags purego avoids matplotlib-go's cgo FreeType backend, whose vendored
# prefix is not distributed with the module.
plot:
	@go run -tags purego ./cmd/fftplot \
		-simd results/simd.json -purego results/purego.json -out plots

# Sweep, then plot.
report SIZE="32768": (sweep SIZE)
	@just plot

# Run Go tests
test:
	@echo "Running tests..."
	@go test ./...

# Run manual benchmarks with allocation stats
test-bench:
	@GOAMD64=v3 go test -bench . -benchmem ./bench

# Run inverse FFT benchmarks manually
test-ifft:
	@GOAMD64=v3 go test -bench=BenchmarkIFFT -benchmem ./bench

# Run the pure-Go benchmarks (no SIMD codelets)
test-purego:
	@GOAMD64=v3 go test -tags purego -bench . -benchmem ./bench

# Run accuracy tests (round-trip plus the arbitrary-length cross-check)
test-accuracy:
	@go test -run='TestFFTRoundTrip|TestAnySizesAccuracy' ./bench

# Clean built binaries
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@echo "Done"
