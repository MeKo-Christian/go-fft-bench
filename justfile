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

# Run Go tests
test:
	@echo "Running tests..."
	@go test ./...

# Run manual benchmarks with allocation stats
test-bench:
	@GOAMD64=v3 go test -tags=asm -bench . -benchmem ./bench

# Run inverse FFT benchmarks manually
test-ifft:
	@GOAMD64=v3 go test -tags=asm -bench=BenchmarkIFFT -benchmem ./bench

# Run accuracy tests
test-accuracy:
	@go test -run=TestFFTRoundTrip ./bench

# Clean built binaries
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@echo "Done"
