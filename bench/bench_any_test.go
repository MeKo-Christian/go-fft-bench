package bench

import (
	"fmt"
	"math/cmplx"
	"os"
	"testing"

	algofft "github.com/cwbudde/algo-fft"
	"github.com/cwbudde/go-fftw/fftw"
	"github.com/madelynnblue/go-dsp/fft"
	"gonum.org/v1/gonum/dsp/fourier"
)

// BenchmarkFFTAny covers the non-power-of-two lengths. takatoh/fft is absent
// on purpose: it is radix-2 only and silently returns garbage for other
// lengths, so there is nothing to compare.
func BenchmarkFFTAny(b *testing.B) {
	b.ReportAllocs()

	for _, spec := range AnySizes() {
		n := spec.N
		b.Run(fmt.Sprintf("algo-fft/%d", n), func(b *testing.B) {
			benchAlgoAny(b, n)
		})
		b.Run(fmt.Sprintf("go-fftw/%d", n), func(b *testing.B) {
			benchGoFFTW(b, n)
		})
		b.Run(fmt.Sprintf("gonum/%d", n), func(b *testing.B) {
			benchGonum(b, n)
		})
		b.Run(fmt.Sprintf("go-dsp-fft/%d", n), func(b *testing.B) {
			benchGoDSP(b, n)
		})
	}
}

// BenchmarkFFTAny32 runs the same lengths in single precision. Only algo-fft
// is measured: none of the other libraries exposes a complex64 transform.
func BenchmarkFFTAny32(b *testing.B) {
	b.ReportAllocs()

	for _, spec := range AnySizes() {
		n := spec.N
		b.Run(fmt.Sprintf("algo-fft/%d", n), func(b *testing.B) {
			benchAlgoAny32(b, n)
		})
	}
}

func benchAlgoAny(b *testing.B, n int) {
	plan, err := algofft.NewPlan64(n)
	if err != nil {
		b.Fatalf("algo-fft plan: %v", err)
	}
	defer plan.Close()

	src := make([]complex128, n)
	dst := make([]complex128, n)
	fillComplex128(src)

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := plan.Forward(dst, src); err != nil {
			b.Fatalf("algo-fft forward: %v", err)
		}
	}
}

func benchAlgoAny32(b *testing.B, n int) {
	plan, err := algofft.NewPlan32(n)
	if err != nil {
		b.Fatalf("algo-fft plan: %v", err)
	}
	defer plan.Close()

	src := make([]complex64, n)
	dst := make([]complex64, n)
	fillComplex64(src)

	b.SetBytes(int64(n) * 8)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := plan.Forward(dst, src); err != nil {
			b.Fatalf("algo-fft forward: %v", err)
		}
	}
}

// TestAnySizesAccuracy guards the arbitrary-length benchmarks: a fast wrong
// answer is not a benchmark result. Every library measured in
// BenchmarkFFTAny is checked against gonum's transform of the same input.
func TestAnySizesAccuracy(t *testing.T) {
	for _, spec := range AnySizes() {
		n := spec.N
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			src := make([]complex128, n)
			fillComplex128(src)

			want := make([]complex128, n)
			fourier.NewCmplxFFT(n).Coefficients(want, src)

			plan, err := algofft.NewPlan64(n)
			if err != nil {
				t.Fatalf("algo-fft plan: %v", err)
			}
			defer plan.Close()

			got := make([]complex128, n)
			if err := plan.Forward(got, src); err != nil {
				t.Fatalf("algo-fft forward: %v", err)
			}
			compareSpectra(t, "algo-fft", want, got)

			compareSpectra(t, "go-dsp-fft", want, fft.FFT(src))

			fftwSrc := fftw.NewArray(n)
			fftwDst := fftw.NewArray(n)
			copy(fftwSrc.Elems, src)
			fftwPlan := fftw.NewPlan(fftwSrc, fftwDst, fftw.Forward, fftw.Estimate)
			fftwPlan.Execute()
			fftwPlan.Destroy()
			compareSpectra(t, "go-fftw", want, fftwDst.Elems)
		})
	}
}

// compareSpectra compares two spectra with a tolerance scaled by the largest
// coefficient magnitude: absolute error grows with the input energy, which
// for these ramp inputs grows with n^2.
func compareSpectra(t *testing.T, name string, want, got []complex128) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s: got %d coefficients, want %d", name, len(got), len(want))
	}

	scale := 0.0
	for _, w := range want {
		if m := cabs(w); m > scale {
			scale = m
		}
	}
	tolerance := 1e-9 * scale

	maxErr := 0.0
	for i := range want {
		if e := cabs(want[i] - got[i]); e > maxErr {
			maxErr = e
		}
	}
	if maxErr > tolerance {
		t.Errorf("%s: max error %e exceeds tolerance %e", name, maxErr, tolerance)
	}
}

func cabs(c complex128) float64 {
	return cmplx.Abs(c)
}

// TestWritePlanInfo dumps the resolved algo-fft plan routes for every
// benchmarked size to the file named by FFT_BENCH_PLANINFO. It is a test
// rather than a separate tool so that the routes are collected by the same
// binary, build tags and CPU as the benchmarks they annotate.
func TestWritePlanInfo(t *testing.T) {
	path := os.Getenv("FFT_BENCH_PLANINFO")
	if path == "" {
		t.Skip("FFT_BENCH_PLANINFO not set")
	}

	infos, err := CollectPlanInfo(MaxPow2())
	if err != nil {
		t.Fatalf("collect plan info: %v", err)
	}
	if err := WritePlanInfo(path, infos); err != nil {
		t.Fatalf("write plan info: %v", err)
	}
	t.Logf("wrote %d plan routes to %s", len(infos), path)
}
