package main

import (
	"strings"
	"testing"
)

// newTestRunner builds a runner holding one complex128 section with a real
// baseline library and one complex64 section with none — the shape that used
// to render as "Error: Baseline library 'go-fftw' not found".
func newTestRunner() *BenchmarkRunner {
	result := func(benchType, library string, size int, nsOp float64) *BenchmarkResult {
		return &BenchmarkResult{
			BenchType: benchType, Library: library, Size: size,
			Class: "pow2", NsOp: nsOp, MBs: 1000,
		}
	}

	return &BenchmarkRunner{
		MaxSize:  1024,
		Baseline: "go-fftw",
		Results: map[string]map[string]map[int]*BenchmarkResult{
			"FFT": {
				"go-fftw":  {1024: result("FFT", "go-fftw", 1024, 1200)},
				"algo-fft": {1024: result("FFT", "algo-fft", 1024, 1000)},
			},
			"FFT32": {
				"algo-fft": {1024: result("FFT32", "algo-fft", 1024, 500)},
			},
		},
	}
}

func TestBaselineForBorrowsComplex128Series(t *testing.T) {
	runner := newTestRunner()

	base, ok := runner.baselineFor("FFT")
	if !ok || !base.own || base.label != "go-fftw (FFTW3)" {
		t.Fatalf("FFT baseline = %+v, ok=%v; want the configured library", base, ok)
	}

	base, ok = runner.baselineFor("FFT32")
	if !ok {
		t.Fatal("FFT32 has no baseline; the complex64 section would be dropped")
	}
	if base.own {
		t.Error("FFT32 baseline reported as its own section's; it is borrowed from FFT")
	}
	if got := base.data[1024].NsOp; got != 1000 {
		t.Errorf("FFT32 baseline ns/op = %v, want the complex128 series' 1000", got)
	}
}

func TestBaselineForReportsMissingBaseline(t *testing.T) {
	runner := newTestRunner()
	runner.Results["FFT32"] = map[string]map[int]*BenchmarkResult{
		"algo-fft": {1024: {BenchType: "FFT32", Library: "algo-fft", Size: 1024, NsOp: 500}},
	}
	delete(runner.Results, "FFT")

	if _, ok := runner.baselineFor("FFT32"); ok {
		t.Error("baselineFor succeeded without a complex128 twin to borrow from")
	}
}

func TestMarkdownRendersComplex64Section(t *testing.T) {
	md := newTestRunner().generateMarkdown()

	if strings.Contains(md, "Baseline library") {
		t.Error("markdown still carries the missing-baseline error")
	}

	section := md[strings.Index(md, "## FFT32 Benchmarks"):]
	if !strings.Contains(section, "algo-fft FFT (complex128)") {
		t.Errorf("FFT32 section does not name its borrowed baseline:\n%s", section)
	}
	// 1000 ns complex128 against 500 ns complex64.
	if !strings.Contains(section, "2.00x") {
		t.Errorf("FFT32 section lacks the complex128/complex64 ratio:\n%s", section)
	}
	if strings.Contains(section, "### Baseline:") {
		t.Errorf("FFT32 section re-tabulates the borrowed baseline:\n%s", section)
	}
}
