package main

import (
	"fmt"
	"io"
	"sort"
)

// writeSummary prints the aggregate numbers a write-up needs, so that prose
// claims come from the same data as the charts rather than from squinting at
// them.
func writeSummary(w io.Writer, simd *Run, pure *Run) {
	fmt.Fprintf(w, "# Benchmark summary\n\n")
	fmt.Fprintf(w, "machine:  %s\n", simd.Meta.CPU)
	fmt.Fprintf(w, "go:       %s (GOAMD64=%s)\n", simd.Meta.GoVersion, simd.Meta.GOAMD64)
	fmt.Fprintf(w, "algo-fft: %s\n\n", simd.Meta.AlgoFFT)

	summarizePow2(w, simd)
	summarizeNonPow2(w, simd)
	if pure != nil {
		summarizeBuilds(w, simd, pure)
	}
}

// summarizePow2 reports each library's standing against FFTW at powers of two.
func summarizePow2(w io.Writer, run *Run) {
	fmt.Fprintf(w, "## Powers of two (complex128 forward), vs FFTW3\n\n")
	fmt.Fprintf(w, "%-12s %8s %8s %8s   %s\n", "library", "geomean", "best", "worst", "range")
	fmt.Fprintf(w, "%s\n", "------------------------------------------------------------")

	for _, lib := range run.Libraries("FFT") {
		if lib == "go-fftw" {
			continue
		}
		var ratios []float64
		best, worst := 0.0, 0.0
		bestN, worstN := 0, 0
		for _, n := range run.Sizes("FFT") {
			base, okBase := run.Lookup("FFT", "go-fftw", n)
			res, ok := run.Lookup("FFT", lib, n)
			if !ok || !okBase || res.NsOp <= 0 {
				continue
			}
			r := base.NsOp / res.NsOp
			ratios = append(ratios, r)
			if best == 0 || r > best {
				best, bestN = r, n
			}
			if worst == 0 || r < worst {
				worst, worstN = r, n
			}
		}
		if len(ratios) == 0 {
			continue
		}
		fmt.Fprintf(w, "%-12s %8.2f %8.2f %8.2f   best at n=%d, worst at n=%d\n",
			lib, GeoMean(ratios), best, worst, bestN, worstN)
	}

	fmt.Fprintf(w, "\nabsolute ns/op, complex128 forward:\n\n")
	fmt.Fprintf(w, "%8s", "n")
	libs := run.Libraries("FFT")
	for _, lib := range libs {
		fmt.Fprintf(w, " %12s", lib)
	}
	fmt.Fprintf(w, "\n")
	for _, n := range run.Sizes("FFT") {
		fmt.Fprintf(w, "%8d", n)
		for _, lib := range libs {
			if res, ok := run.Lookup("FFT", lib, n); ok {
				fmt.Fprintf(w, " %12.0f", res.NsOp)
			} else {
				fmt.Fprintf(w, " %12s", "-")
			}
		}
		fmt.Fprintf(w, "\n")
	}
	fmt.Fprintf(w, "\n")
}

// summarizeNonPow2 reports the arbitrary-length results by class and by size,
// annotated with the algorithm algo-fft actually resolved.
func summarizeNonPow2(w io.Writer, run *Run) {
	fmt.Fprintf(w, "## Non-power-of-two lengths, vs FFTW3\n\n")

	libs := []string{"algo-fft", "gonum", "go-dsp-fft"}
	fmt.Fprintf(w, "%-20s", "class")
	for _, lib := range libs {
		fmt.Fprintf(w, " %11s", lib)
	}
	fmt.Fprintf(w, "\n%s\n", "------------------------------------------------------------")

	for _, class := range run.Classes("FFTAny") {
		fmt.Fprintf(w, "%-20s", class)
		for _, lib := range libs {
			var ratios []float64
			for _, n := range run.SizesOfClass("FFTAny", class) {
				base, okBase := run.Lookup("FFTAny", "go-fftw", n)
				res, ok := run.Lookup("FFTAny", lib, n)
				if !ok || !okBase || res.NsOp <= 0 {
					continue
				}
				ratios = append(ratios, base.NsOp/res.NsOp)
			}
			if len(ratios) == 0 {
				fmt.Fprintf(w, " %11s", "-")
				continue
			}
			fmt.Fprintf(w, " %11.2f", GeoMean(ratios))
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "\nper size (ns/op, and algo-fft's speedup vs FFTW3):\n\n")
	fmt.Fprintf(w, "%7s %-19s %-12s %10s %10s %10s %8s\n",
		"n", "class", "algorithm", "algo-fft", "go-fftw", "go-dsp", "vs FFTW")
	fmt.Fprintf(w, "%s\n", "--------------------------------------------------------------------------------")

	for _, class := range run.Classes("FFTAny") {
		for _, n := range run.SizesOfClass("FFTAny", class) {
			res, ok := run.Lookup("FFTAny", "algo-fft", n)
			if !ok {
				continue
			}
			algo := ""
			if info, ok := run.Plan(n); ok {
				algo = info.Algorithm
			}
			base, _ := run.Lookup("FFTAny", "go-fftw", n)
			dsp, _ := run.Lookup("FFTAny", "go-dsp-fft", n)

			ratio := 0.0
			if base.NsOp > 0 && res.NsOp > 0 {
				ratio = base.NsOp / res.NsOp
			}
			fmt.Fprintf(w, "%7d %-19s %-12s %10.0f %10.0f %10.0f %8.2f\n",
				n, class, algo, res.NsOp, base.NsOp, dsp.NsOp, ratio)
		}
	}
	fmt.Fprintf(w, "\n")
}

// summarizeBuilds reports what the SIMD codelets buy over the pure-Go build.
func summarizeBuilds(w io.Writer, simd, pure *Run) {
	fmt.Fprintf(w, "## Default build vs -tags purego\n\n")

	for _, bench := range []struct{ typ, label string }{
		{"FFT", "complex128, powers of two"},
		{"FFT32", "complex64, powers of two"},
		{"FFTAny", "complex128, arbitrary lengths"},
	} {
		var ratios []float64
		best, bestN := 0.0, 0
		worst, worstN := 0.0, 0
		sizes := simd.Sizes(bench.typ)
		sort.Ints(sizes)
		for _, n := range sizes {
			fast, okFast := simd.Lookup(bench.typ, "algo-fft", n)
			slow, okSlow := pure.Lookup(bench.typ, "algo-fft", n)
			if !okFast || !okSlow || fast.NsOp <= 0 {
				continue
			}
			r := slow.NsOp / fast.NsOp
			ratios = append(ratios, r)
			if best == 0 || r > best {
				best, bestN = r, n
			}
			if worst == 0 || r < worst {
				worst, worstN = r, n
			}
		}
		if len(ratios) == 0 {
			continue
		}
		fmt.Fprintf(w, "%-32s geomean %.2fx, best %.2fx (n=%d), worst %.2fx (n=%d)\n",
			bench.label, GeoMean(ratios), best, bestN, worst, worstN)
	}

	fmt.Fprintf(w, "\npure-Go build vs the other Go libraries (complex128, powers of two, geomean):\n\n")
	for _, lib := range []string{"gonum", "go-dsp-fft", "takatoh"} {
		var ratios []float64
		for _, n := range simd.Sizes("FFT") {
			other, okOther := simd.Lookup("FFT", lib, n)
			ours, okOurs := pure.Lookup("FFT", "algo-fft", n)
			if !okOther || !okOurs || ours.NsOp <= 0 {
				continue
			}
			ratios = append(ratios, other.NsOp/ours.NsOp)
		}
		if len(ratios) == 0 {
			continue
		}
		fmt.Fprintf(w, "  vs %-12s %.2fx\n", lib, GeoMean(ratios))
	}
	fmt.Fprintf(w, "\n")
}
