// Command fftplot renders the charts for a benchmark sweep produced by
// `benchrunner -json`.
//
// It is deliberately a separate binary from benchrunner: plotting must never
// run while a benchmark is being measured, and this tool does not link cgo
// FFTW.
//
// Build it with `-tags purego`; matplotlib-go's default AGG backend compiles
// a cgo FreeType binding against a vendored prefix that is not distributed.
//
//	go run -tags purego ./cmd/fftplot -simd results/simd.json \
//	    -purego results/purego.json -out plots
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	var (
		simdPath   = flag.String("simd", "results/simd.json", "benchrunner JSON for the default (SIMD) build")
		puregoPath = flag.String("purego", "results/purego.json", "benchrunner JSON for the -tags purego build (optional)")
		outDir     = flag.String("out", "plots", "directory to write PNG charts into")
		dpi        = flag.Float64("dpi", 144, "output resolution")
		summary    = flag.Bool("summary", false, "print the aggregate numbers to stdout instead of rendering charts")
	)
	flag.Parse()

	if *summary {
		if err := printSummary(*simdPath, *puregoPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := run(*simdPath, *puregoPath, *outDir, *dpi); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// printSummary loads the same inputs as the charts and writes the aggregate
// numbers as text.
func printSummary(simdPath, puregoPath string) error {
	simd, err := LoadRun(simdPath)
	if err != nil {
		return err
	}
	var pure *Run
	if puregoPath != "" {
		if pure, err = LoadRun(puregoPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: no purego results (%v)\n", err)
			pure = nil
		}
	}
	writeSummary(os.Stdout, simd, pure)
	return nil
}

func run(simdPath, puregoPath, outDir string, dpi float64) error {
	simd, err := LoadRun(simdPath)
	if err != nil {
		return fmt.Errorf("loading SIMD results: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d results from %s (%s)\n", len(simd.Results), simdPath, simd.Meta.Label)

	// The purego run is optional: the competitor charts do not need it.
	var pure *Run
	if puregoPath != "" {
		pure, err = LoadRun(puregoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: no purego results (%v); skipping the build-comparison chart\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Loaded %d results from %s (%s)\n", len(pure.Results), puregoPath, pure.Meta.Label)
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	charts := []struct {
		name string
		fn   func(string) error
	}{
		{"01-throughput-pow2.png", func(p string) error { return plotThroughputPow2(simd, p, dpi) }},
		{"02-speedup-vs-fftw.png", func(p string) error { return plotSpeedupVsFFTW(simd, p, dpi) }},
		{"03-nonpow2-by-class.png", func(p string) error { return plotNonPow2ByClass(simd, p, dpi) }},
		{"04-nonpow2-detail.png", func(p string) error { return plotNonPow2Detail(simd, p, dpi) }},
		{"05-precision.png", func(p string) error { return plotPrecision(simd, p, dpi) }},
	}
	if pure != nil {
		charts = append(charts,
			struct {
				name string
				fn   func(string) error
			}{"06-simd-vs-purego.png", func(p string) error { return plotSIMDvsPurego(simd, pure, p, dpi) }},
			struct {
				name string
				fn   func(string) error
			}{"07-purego-vs-competitors.png", func(p string) error { return plotPuregoVsCompetitors(simd, pure, p, dpi) }},
		)
	}

	for _, chart := range charts {
		path := filepath.Join(outDir, chart.name)
		if err := chart.fn(path); err != nil {
			return fmt.Errorf("%s: %w", chart.name, err)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", path)
	}

	return nil
}
