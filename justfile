# Run Go benchmarks with allocation stats.
test:
	GOAMD64=v3 go test -tags=asm -bench . -benchmem ./bench

# Run inverse FFT benchmarks.
test-ifft:
	GOAMD64=v3 go test -tags=asm -bench=BenchmarkIFFT -benchmem ./bench

# Run accuracy tests.
test-accuracy:
	go test -run=TestFFTRoundTrip ./bench

# Run all benchmarks and update BENCHMARKS.md
bench:
	FFT_BENCH_MAX=32768 GOAMD64=v3 go test -tags=asm -bench . -benchmem ./bench > benchmark_results.txt
	cat benchmark_results.txt | ./scripts/bench_to_csv.py > bench.csv
	./scripts/csv_to_md.py
