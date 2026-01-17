package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const version = "1.0.0"

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	BenchType string
	Library   string
	Size      int
	NsOp      float64
	MBs       float64
	BOp       int
	AllocsOp  int
}

// BenchmarkRunner manages benchmark execution and result formatting
type BenchmarkRunner struct {
	MaxSize  int
	Baseline string
	GOAMD64  string
	Tags     string
	Output   string
	Show     bool
	Results  map[string]map[string]map[int]*BenchmarkResult
}

var benchRE = regexp.MustCompile(
	`^Benchmark(FFT|IFFT|FFT32|IFFT32)/([^/]+)/(\d+)-\d+\s+` +
		`\d+\s+` + // iterations
		`([0-9.]+)\s+ns/op\s+` +
		`([0-9.]+)\s+MB/s\s+` +
		`(\d+)\s+B/op\s+` +
		`(\d+)\s+allocs/op`,
)

func main() {
	runner := &BenchmarkRunner{
		Results: make(map[string]map[string]map[int]*BenchmarkResult),
	}

	flag.IntVar(&runner.MaxSize, "max-size", 32768, "Maximum FFT size to benchmark")
	flag.StringVar(&runner.Baseline, "baseline", "go-fftw", "Baseline library for comparison")
	flag.StringVar(&runner.GOAMD64, "goamd64", "v3", "GOAMD64 version")
	flag.StringVar(&runner.Tags, "tags", "asm", "Go build tags")
	flag.StringVar(&runner.Output, "output", "BENCHMARKS.md", "Output file")
	flag.BoolVar(&runner.Show, "show", false, "Print to stdout instead of writing to file")

	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("benchrunner version %s\n", version)
		os.Exit(0)
	}

	if err := runner.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func (r *BenchmarkRunner) Run() error {
	fmt.Fprintf(os.Stderr, "Running benchmarks (max size: %d)...\n", r.MaxSize)
	fmt.Fprintf(os.Stderr, "Command: go test -bench . -benchmem -run ^$ -tags=%s ./bench\n", r.Tags)
	fmt.Fprintf(os.Stderr, "Environment: FFT_BENCH_MAX=%d GOAMD64=%s\n\n", r.MaxSize, r.GOAMD64)

	if err := r.runBenchmarks(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\nBenchmarks completed successfully!\n")
	fmt.Fprintf(os.Stderr, "Parsed %d results\n", r.countResults())

	markdown := r.generateMarkdown()

	if r.Show {
		fmt.Println(markdown)
	} else {
		if err := os.WriteFile(r.Output, []byte(markdown), 0644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "\nResults written to %s\n", r.Output)
	}

	return nil
}

func (r *BenchmarkRunner) runBenchmarks() error {
	// Find the bench directory
	benchDir := "./bench"
	if _, err := os.Stat(benchDir); os.IsNotExist(err) {
		// Try from project root
		if wd, err := os.Getwd(); err == nil {
			if strings.HasSuffix(wd, "/cmd/benchrunner") {
				benchDir = "../../bench"
			}
		}
	}

	cmd := exec.Command("go", "test",
		"-bench", ".",
		"-benchmem",
		"-run", "^$",
		"-tags="+r.Tags,
		benchDir,
	)

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("FFT_BENCH_MAX=%d", r.MaxSize),
		fmt.Sprintf("GOAMD64=%s", r.GOAMD64),
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting command: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "Benchmark") {
			fmt.Fprintf(os.Stderr, "  %s\n", line)
		}

		if err := r.parseBenchmarkLine(line); err != nil {
			// Ignore parse errors, not all lines are benchmark results
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading output: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("benchmark command failed: %w", err)
	}

	return nil
}

func (r *BenchmarkRunner) parseBenchmarkLine(line string) error {
	matches := benchRE.FindStringSubmatch(line)
	if matches == nil {
		return fmt.Errorf("no match")
	}

	benchType := matches[1]
	library := matches[2]
	size, _ := strconv.Atoi(matches[3])
	nsOp, _ := strconv.ParseFloat(matches[4], 64)
	mbs, _ := strconv.ParseFloat(matches[5], 64)
	bOp, _ := strconv.Atoi(matches[6])
	allocsOp, _ := strconv.Atoi(matches[7])

	result := &BenchmarkResult{
		BenchType: benchType,
		Library:   library,
		Size:      size,
		NsOp:      nsOp,
		MBs:       mbs,
		BOp:       bOp,
		AllocsOp:  allocsOp,
	}

	if r.Results[benchType] == nil {
		r.Results[benchType] = make(map[string]map[int]*BenchmarkResult)
	}
	if r.Results[benchType][library] == nil {
		r.Results[benchType][library] = make(map[int]*BenchmarkResult)
	}
	r.Results[benchType][library][size] = result

	return nil
}

func (r *BenchmarkRunner) countResults() int {
	count := 0
	for _, benchType := range r.Results {
		for _, library := range benchType {
			count += len(library)
		}
	}
	return count
}

func (r *BenchmarkRunner) generateMarkdown() string {
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "# Benchmarks\n\n")
	fmt.Fprintf(&b, "Command used: `FFT_BENCH_MAX=%d GOAMD64=%s go test -tags=%s -bench . -benchmem ./bench`\n\n",
		r.MaxSize, r.GOAMD64, r.Tags)
	fmt.Fprintf(&b, "Notes:\n\n")
	fmt.Fprintf(&b, "- Results are from the latest local run.\n")
	fmt.Fprintf(&b, "- `algo-fft` benchmarks include both complex128 and complex64.\n")
	fmt.Fprintf(&b, "- `go-fftw` (FFTW3) is used as the **baseline** for comparison.\n")
	fmt.Fprintf(&b, "- `go-fftw` requires FFTW shared libraries.\n")
	fmt.Fprintf(&b, "- `go-dsp-fft` allocates on every call (no reusable plan).\n")
	fmt.Fprintf(&b, "- **Speedup** shows performance relative to go-fftw baseline (higher is better).\n\n")

	// Sort benchmark types
	typeOrder := map[string]int{"FFT": 0, "IFFT": 1, "FFT32": 2, "IFFT32": 3}
	var types []string
	for t := range r.Results {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		return typeOrder[types[i]] < typeOrder[types[j]]
	})

	for _, benchType := range types {
		fmt.Fprintf(&b, "## %s Benchmarks\n\n", benchType)

		libraries := r.Results[benchType]
		baselineData, hasBaseline := libraries[r.Baseline]

		if !hasBaseline {
			fmt.Fprintf(&b, "### Error: Baseline library '%s' not found\n\n", r.Baseline)
			continue
		}

		// Baseline table
		r.writeBaselineTable(&b, benchType, baselineData)

		// Comparison tables for other libraries
		libOrder := []string{"algo-fft", "go-dsp-fft", "gonum", "takatoh"}
		var otherLibs []string
		for _, lib := range libOrder {
			if _, ok := libraries[lib]; ok && lib != r.Baseline {
				otherLibs = append(otherLibs, lib)
			}
		}
		for lib := range libraries {
			if !contains(libOrder, lib) && lib != r.Baseline {
				otherLibs = append(otherLibs, lib)
			}
		}
		sort.Strings(otherLibs[len(libOrder):])

		for _, library := range otherLibs {
			r.writeComparisonTable(&b, benchType, library, libraries[library], baselineData)
		}
	}

	return b.String()
}

func (r *BenchmarkRunner) writeBaselineTable(w io.Writer, benchType string, data map[int]*BenchmarkResult) {
	baselineName := r.Baseline
	if r.Baseline == "go-fftw" {
		baselineName = "go-fftw (FFTW3)"
	}
	fmt.Fprintf(w, "### Baseline: %s\n\n", baselineName)
	fmt.Fprintf(w, "| Size  | ns/op  | MB/s     | B/op | allocs/op |\n")
	fmt.Fprintf(w, "| ----- | ------ | -------- | ---- | --------- |\n")

	var sizes []int
	for size := range data {
		sizes = append(sizes, size)
	}
	sort.Ints(sizes)

	for _, size := range sizes {
		result := data[size]
		fmt.Fprintf(w, "| %-5d | %-6s | %-8s | %-4d | %-9d |\n",
			size,
			formatNumber(result.NsOp, -1),
			formatNumber(result.MBs, -1),
			result.BOp,
			result.AllocsOp,
		)
	}
	fmt.Fprintf(w, "\n")
}

func (r *BenchmarkRunner) writeComparisonTable(w io.Writer, benchType, library string, data, baselineData map[int]*BenchmarkResult) {
	sectionSuffix := ""
	if strings.HasPrefix(benchType, "IFFT") && library != "algo-fft" {
		sectionSuffix = fmt.Sprintf(" (%s)", benchType)
	}

	fmt.Fprintf(w, "### %s%s\n\n", library, sectionSuffix)
	fmt.Fprintf(w, "| Size  | ns/op  | Speedup vs baseline | MB/s     | B/op   | allocs/op |\n")
	fmt.Fprintf(w, "| ----- | ------ | ------------------- | -------- | ------ | --------- |\n")

	var sizes []int
	for size := range data {
		sizes = append(sizes, size)
	}
	sort.Ints(sizes)

	for _, size := range sizes {
		result := data[size]
		baselineResult := baselineData[size]

		speedupStr := "N/A"
		if baselineResult != nil && result.NsOp > 0 {
			speedup := baselineResult.NsOp / result.NsOp
			speedupStr = fmt.Sprintf("%.2fx", speedup)
			if speedup >= 1.0 {
				speedupStr = fmt.Sprintf("**%s**", speedupStr)
			}
		}

		fmt.Fprintf(w, "| %-5d | %-6s | %-19s | %-8s | %-6d | %-9d |\n",
			size,
			formatNumber(result.NsOp, -1),
			speedupStr,
			formatNumber(result.MBs, -1),
			result.BOp,
			result.AllocsOp,
		)
	}
	fmt.Fprintf(w, "\n")
}

func formatNumber(value float64, precision int) string {
	if precision >= 0 {
		return fmt.Sprintf("%.*f", precision, value)
	}

	// Adaptive precision based on magnitude
	if value >= 1000000 {
		return fmt.Sprintf("%d", int(value))
	} else if value >= 1000 {
		return fmt.Sprintf("%d", int(value))
	} else if value >= 100 {
		if value == float64(int(value)) {
			return fmt.Sprintf("%d", int(value))
		}
		return fmt.Sprintf("%.1f", value)
	} else if value >= 10 {
		if value == float64(int(value)) {
			return fmt.Sprintf("%d", int(value))
		}
		return fmt.Sprintf("%.2f", value)
	} else if value >= 1 {
		return fmt.Sprintf("%.2f", value)
	}
	s := fmt.Sprintf("%.4f", value)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
