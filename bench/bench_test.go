package bench

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"

	algofft "github.com/MeKo-Christian/algo-fft"
	"github.com/meko-christian/go-fftw/fftw"
	"github.com/mjibson/go-dsp/fft"
	takatohfft "github.com/takatoh/fft"
	"gonum.org/v1/gonum/dsp/fourier"
)

func BenchmarkFFT(b *testing.B) {
	sizes := benchSizes()
	b.ReportAllocs()

	for _, n := range sizes {
		n := n
		b.Run(fmt.Sprintf("algo-fft/%d", n), func(b *testing.B) {
			benchAlgoFFT(b, n)
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
		b.Run(fmt.Sprintf("takatoh/%d", n), func(b *testing.B) {
			benchTakatoh(b, n)
		})
	}
}

func BenchmarkIFFT(b *testing.B) {
	sizes := benchSizes()
	b.ReportAllocs()

	for _, n := range sizes {
		n := n
		b.Run(fmt.Sprintf("algo-fft/%d", n), func(b *testing.B) {
			benchAlgoIFFT(b, n)
		})
		b.Run(fmt.Sprintf("go-fftw/%d", n), func(b *testing.B) {
			benchGoFFTW_IFFT(b, n)
		})
		b.Run(fmt.Sprintf("gonum/%d", n), func(b *testing.B) {
			benchGonum_IFFT(b, n)
		})
		b.Run(fmt.Sprintf("go-dsp-fft/%d", n), func(b *testing.B) {
			benchGoDSP_IFFT(b, n)
		})
		b.Run(fmt.Sprintf("takatoh/%d", n), func(b *testing.B) {
			benchTakatoh_IFFT(b, n)
		})
	}
}

func benchAlgoFFT(b *testing.B, n int) {
	plan, err := algofft.NewPlan64(n)
	if err != nil {
		b.Fatalf("algo-fft plan: %v", err)
	}

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

func benchGoFFTW(b *testing.B, n int) {
	src := fftw.NewArray(n)
	dst := fftw.NewArray(n)
	fillComplex128(src.Elems)

	plan := fftw.NewPlan(src, dst, fftw.Forward, fftw.Estimate)
	defer plan.Destroy()

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plan.Execute()
	}
}

func benchGonum(b *testing.B, n int) {
	plan := fourier.NewCmplxFFT(n)
	src := make([]complex128, n)
	dst := make([]complex128, n)
	fillComplex128(src)

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = plan.Coefficients(dst, src)
	}
}

func benchGoDSP(b *testing.B, n int) {
	src := make([]complex128, n)
	fillComplex128(src)

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fft.FFT(src)
	}
}

func benchTakatoh(b *testing.B, n int) {
	src := make([]complex128, n)
	fillComplex128(src)

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = takatohfft.FFT(src, n)
	}
}

func benchAlgoIFFT(b *testing.B, n int) {
	plan, err := algofft.NewPlan64(n)
	if err != nil {
		b.Fatalf("algo-fft plan: %v", err)
	}

	src := make([]complex128, n)
	dst := make([]complex128, n)
	fillComplex128(src)

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := plan.Inverse(dst, src); err != nil {
			b.Fatalf("algo-fft inverse: %v", err)
		}
	}
}

func benchGoFFTW_IFFT(b *testing.B, n int) {
	src := fftw.NewArray(n)
	dst := fftw.NewArray(n)
	fillComplex128(src.Elems)

	plan := fftw.NewPlan(src, dst, fftw.Backward, fftw.Estimate)
	defer plan.Destroy()

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plan.Execute()
	}
}

func benchGonum_IFFT(b *testing.B, n int) {
	plan := fourier.NewCmplxFFT(n)
	src := make([]complex128, n)
	dst := make([]complex128, n)
	fillComplex128(src)

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = plan.Sequence(dst, src)
	}
}

func benchGoDSP_IFFT(b *testing.B, n int) {
	src := make([]complex128, n)
	fillComplex128(src)

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fft.IFFT(src)
	}
}

func benchTakatoh_IFFT(b *testing.B, n int) {
	src := make([]complex128, n)
	fillComplex128(src)

	b.SetBytes(int64(n) * 16)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = takatohfft.IFFT(src, n)
	}
}

func TestFFTRoundTrip(t *testing.T) {
	sizes := []int{8, 16, 32, 64, 128, 256}

	for _, n := range sizes {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			original := make([]complex128, n)
			fillComplex128(original)

			// Test algo-fft
			plan, err := algofft.NewPlan64(n)
			if err == nil {
				fftResult := make([]complex128, n)
				ifftResult := make([]complex128, n)
				if err := plan.Forward(fftResult, original); err == nil {
					if err := plan.Inverse(ifftResult, fftResult); err == nil {
						checkAccuracy(t, "algo-fft", original, ifftResult, n)
					}
				}
			}

			// Test gonum
			fftPlan := fourier.NewCmplxFFT(n)
			fftResult := make([]complex128, n)
			ifftResult := make([]complex128, n)
			fftPlan.Coefficients(fftResult, original)
			fftPlan.Sequence(ifftResult, fftResult)
			// Gonum doesn't normalize, so divide by n
			for i := range ifftResult {
				ifftResult[i] /= complex(float64(n), 0)
			}
			checkAccuracy(t, "gonum", original, ifftResult, n)

			// Test go-dsp
			fftResult = fft.FFT(original)
			ifftResult = fft.IFFT(fftResult)
			checkAccuracy(t, "go-dsp", original, ifftResult, n)

			// Test takatoh
			fftResult = takatohfft.FFT(original, n)
			ifftResult = takatohfft.IFFT(fftResult, n)
			checkAccuracy(t, "takatoh", original, ifftResult, n)
		})
	}
}

func checkAccuracy(t *testing.T, name string, original, result []complex128, n int) {
	const tolerance = 1e-10
	maxError := 0.0
	for i := range original {
		realDiff := math.Abs(real(result[i]) - real(original[i]))
		imagDiff := math.Abs(imag(result[i]) - imag(original[i]))
		error := math.Max(realDiff, imagDiff)
		if error > maxError {
			maxError = error
		}
	}
	if maxError > tolerance {
		t.Errorf("%s: max error %e exceeds tolerance %e", name, maxError, tolerance)
	} else {
		t.Logf("%s: max error %e", name, maxError)
	}
}

func benchSizes() []int {
	maxSize := 8192
	if value := os.Getenv("FFT_BENCH_MAX"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			maxSize = parsed
		}
	}

	sizes := make([]int, 0, 14)
	for n := 8; n <= maxSize; n *= 2 {
		sizes = append(sizes, n)
	}
	return sizes
}

func fillComplex128(dst []complex128) {
	for i := range dst {
		dst[i] = complex(float64(i+1), float64(-i))
	}
}
