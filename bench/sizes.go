// Package bench holds the FFT benchmark suite and the size tables it runs
// over. The non-test files export the size tables and plan-introspection
// helpers so the benchrunner and plotting tools can share them.
package bench

import (
	"os"
	"strconv"
)

// DefaultMaxPow2 is the largest power-of-two size benchmarked unless
// FFT_BENCH_MAX overrides it.
const DefaultMaxPow2 = 8192

// SizeClass groups a non-power-of-two length by the property that decides
// which algorithm an FFT library can use for it.
type SizeClass string

const (
	// ClassPow2 is a power of two: every library's fast path.
	ClassPow2 SizeClass = "pow2"
	// ClassSmooth5 is 5-smooth (2^a*3^b*5^c) and non-power-of-two: exactly
	// executable by a mixed-radix engine.
	ClassSmooth5 SizeClass = "5-smooth"
	// ClassSmooth711 has factors 7 and/or 11 on top of a 5-smooth part:
	// exact only for engines carrying radix-7/11 butterflies.
	ClassSmooth711 SizeClass = "7/11-smooth"
	// ClassPrimeSmooth is a prime p whose p-1 is 5-smooth, which makes the
	// length-(p-1) cyclic convolution of Rader's algorithm cheap.
	ClassPrimeSmooth SizeClass = "prime (smooth p-1)"
	// ClassPrimeHard is a prime whose p-1 is not smooth: Bluestein territory.
	ClassPrimeHard SizeClass = "prime (rough p-1)"
	// ClassPractical are lengths that show up in real signal processing
	// rather than in an algorithm taxonomy.
	ClassPractical SizeClass = "practical"
)

// SizeSpec is one benchmarked length together with the class it illustrates
// and a short note used as a chart annotation.
type SizeSpec struct {
	N     int
	Class SizeClass
	Note  string
}

// Pow2Sizes returns the power-of-two sizes from 8 up to max (inclusive).
func Pow2Sizes(max int) []int {
	if max < 8 {
		max = 8
	}

	sizes := make([]int, 0, 16)
	for n := 8; n <= max; n *= 2 {
		sizes = append(sizes, n)
	}
	return sizes
}

// MaxPow2 reports the largest power-of-two size to benchmark, honouring the
// FFT_BENCH_MAX environment variable.
func MaxPow2() int {
	if value := os.Getenv("FFT_BENCH_MAX"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return DefaultMaxPow2
}

// anySizes is the non-power-of-two size table. It is deliberately small and
// hand-picked: every entry stands for one routing decision a planner has to
// make, so the results stay readable in a chart.
var anySizes = []SizeSpec{
	// 5-smooth: the mixed-radix engine can factor these exactly.
	{N: 96, Class: ClassSmooth5, Note: "2^5*3"},
	{N: 480, Class: ClassSmooth5, Note: "2^5*3*5"},
	{N: 768, Class: ClassSmooth5, Note: "2^8*3"},
	{N: 1152, Class: ClassSmooth5, Note: "2^7*3^2"},
	{N: 1920, Class: ClassSmooth5, Note: "2^7*3*5"},
	{N: 12000, Class: ClassSmooth5, Note: "2^5*3*5^3"},

	// 7/11-smooth: exact only with radix-7/11 butterflies.
	{N: 448, Class: ClassSmooth711, Note: "2^6*7"},
	{N: 704, Class: ClassSmooth711, Note: "2^6*11"},
	{N: 1344, Class: ClassSmooth711, Note: "2^6*3*7"},
	{N: 2016, Class: ClassSmooth711, Note: "2^5*3^2*7"},

	// Primes with smooth p-1: Rader's algorithm is cheap here.
	{N: 97, Class: ClassPrimeSmooth, Note: "p-1 = 2^5*3"},
	{N: 257, Class: ClassPrimeSmooth, Note: "p-1 = 2^8"},
	{N: 641, Class: ClassPrimeSmooth, Note: "p-1 = 2^7*5"},
	{N: 1153, Class: ClassPrimeSmooth, Note: "p-1 = 2^7*3^2"},
	{N: 4001, Class: ClassPrimeSmooth, Note: "p-1 = 2^5*5^3"},
	{N: 12289, Class: ClassPrimeSmooth, Note: "p-1 = 2^12*3"},

	// Primes with rough p-1: no shortcut, Bluestein pads to a power of two.
	{N: 1009, Class: ClassPrimeHard, Note: "p-1 = 2^4*3^2*7"},
	{N: 2003, Class: ClassPrimeHard, Note: "p-1 = 2*7*11*13"},
	{N: 9973, Class: ClassPrimeHard, Note: "p-1 = 2^2*3^2*277"},

	// Lengths that occur in practice.
	{N: 1000, Class: ClassPractical, Note: "1 ms at 1 MHz"},
	{N: 2205, Class: ClassPractical, Note: "50 ms at 44.1 kHz"},
	{N: 3600, Class: ClassPractical, Note: "1 h at 1 Hz"},
	{N: 44100, Class: ClassPractical, Note: "1 s at 44.1 kHz"},
}

// AnySizes returns the non-power-of-two size table.
func AnySizes() []SizeSpec {
	out := make([]SizeSpec, len(anySizes))
	copy(out, anySizes)
	return out
}

// ClassOf reports the class recorded for n, or ClassPow2 if n is a power of
// two, or an empty class if n is not in the table.
func ClassOf(n int) SizeClass {
	if n > 0 && n&(n-1) == 0 {
		return ClassPow2
	}
	for _, spec := range anySizes {
		if spec.N == n {
			return spec.Class
		}
	}
	return ""
}
