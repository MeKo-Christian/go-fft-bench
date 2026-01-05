# Run Go benchmarks with allocation stats.
test:
	GOAMD64=v3 go test -tags=asm -bench . -benchmem ./bench

# Run inverse FFT benchmarks.
test-ifft:
	GOAMD64=v3 go test -tags=asm -bench=BenchmarkIFFT -benchmem ./bench

# Run accuracy tests.
test-accuracy:
	go test -run=TestFFTRoundTrip ./bench
