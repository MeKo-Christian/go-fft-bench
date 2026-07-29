package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cwbudde/go-fft-bench/bench"
)

const version = "1.1.0"

// algoFFTLibrary is the library under test: the one whose complex64 series
// needs a baseline the reference libraries cannot provide.
const algoFFTLibrary = "algo-fft"

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	BenchType string  `json:"benchType"`
	Library   string  `json:"library"`
	Size      int     `json:"size"`
	Class     string  `json:"class"`
	NsOp      float64 `json:"nsOp"`
	MBs       float64 `json:"mbPerSec"`
	BOp       int     `json:"bytesPerOp"`
	AllocsOp  int     `json:"allocsPerOp"`
}

// RunMeta describes the machine and build a result set came from. Benchmark
// numbers are only comparable within one meta block.
type RunMeta struct {
	Label     string `json:"label"`
	Tags      string `json:"tags"`
	GOAMD64   string `json:"goamd64"`
	GoVersion string `json:"goVersion"`
	CPU       string `json:"cpu"`
	MaxSize   int    `json:"maxSize"`
	AlgoFFT   string `json:"algoFFTVersion"`
}

// RunFile is the on-disk shape of a benchmark run: what was measured, on
// what, plus the algo-fft plan routes that explain the numbers.
type RunFile struct {
	Meta      RunMeta           `json:"meta"`
	Results   []BenchmarkResult `json:"results"`
	PlanInfos []bench.PlanInfo  `json:"planInfos,omitempty"`
}

// BenchmarkRunner manages benchmark execution and result formatting
type BenchmarkRunner struct {
	MaxSize     int
	Baseline    string
	GOAMD64     string
	Tags        string
	Output      string
	JSON        string
	Label       string
	Show        bool
	MaxLoad     float64
	WaitForIdle time.Duration
	Results     map[string]map[string]map[int]*BenchmarkResult
	PlanInfos   []bench.PlanInfo
}

// benchTypes are matched longest-first so that FFTAny32 does not lose to
// FFTAny and FFT32 does not lose to FFT.
var benchRE = regexp.MustCompile(
	`^Benchmark(FFTAny32|FFTAny|IFFT32|IFFT|FFT32|FFT)/([^/]+)/(\d+)-\d+\s+` +
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
	flag.StringVar(&runner.Tags, "tags", "", "Go build tags (e.g. purego for the pure-Go algo-fft build)")
	flag.StringVar(&runner.Output, "output", "BENCHMARKS.md", "Output file")
	flag.StringVar(&runner.JSON, "json", "", "Also write machine-readable results to this JSON file (for cmd/fftplot)")
	flag.StringVar(&runner.Label, "label", "", "Name for this run in the JSON output (defaults to the build tags, or \"simd\")")
	flag.BoolVar(&runner.Show, "show", false, "Print to stdout instead of writing to file")
	flag.Float64Var(&runner.MaxLoad, "max-load", defaultMaxLoad(),
		"Refuse to benchmark above this 1-minute load average (0 disables the check)")
	flag.DurationVar(&runner.WaitForIdle, "wait-for-idle", 0, "Wait up to this long for the load average to drop instead of failing")

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
	if err := r.awaitIdle(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Running benchmarks (max size: %d)...\n", r.MaxSize)
	fmt.Fprintf(os.Stderr, "Command: go test -bench . -benchmem -run ^$ -tags=%s ./bench\n", r.Tags)
	fmt.Fprintf(os.Stderr, "Environment: FFT_BENCH_MAX=%d GOAMD64=%s\n\n", r.MaxSize, r.GOAMD64)

	// Collect the plan routes first: it is cheap, and a failure here means
	// the build is broken, which is better to learn before a long sweep.
	if err := r.collectPlanInfo(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not collect algo-fft plan routes: %v\n", err)
	}

	if err := r.runBenchmarks(); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\nBenchmarks completed successfully!\n")
	fmt.Fprintf(os.Stderr, "Parsed %d results\n", r.countResults())

	// Persist the measurements before formatting anything. A sweep costs
	// tens of minutes on an otherwise idle machine; a bug in a table
	// renderer must not be able to throw that away.
	if r.JSON != "" {
		if err := r.writeJSON(); err != nil {
			return fmt.Errorf("writing JSON output: %w", err)
		}
		fmt.Fprintf(os.Stderr, "JSON results written to %s\n", r.JSON)
	}

	markdown, err := r.renderMarkdown()
	if err != nil {
		return fmt.Errorf("rendering markdown (results are safe in %s): %w", r.JSON, err)
	}

	if r.Show {
		fmt.Println(markdown)
	} else {
		if err := os.WriteFile(r.Output, []byte(markdown), 0644); err != nil {
			return fmt.Errorf("writing output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Results written to %s\n", r.Output)
	}

	return nil
}

// renderMarkdown turns a panic in the table formatting into an error, so a
// completed measurement is reported rather than lost behind a stack trace.
func (r *BenchmarkRunner) renderMarkdown() (out string, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return r.generateMarkdown(), nil
}

// defaultMaxLoad scales the idle threshold with the core count. The
// benchmarks are single-threaded, so what matters is not an idle machine but
// an uncontended core: a quarter of the cores busy still leaves plenty of
// headroom, while the same absolute load on a dual-core box would not.
func defaultMaxLoad() float64 {
	quarter := float64(runtime.NumCPU()) / 4
	if quarter < 1.5 {
		return 1.5
	}
	return quarter
}

// awaitIdle refuses to measure while the machine is busy. A compile storm in
// another window skews every number in the sweep, and nothing downstream can
// detect that from the results — so the check belongs here, before the run,
// not in a caveat afterwards.
func (r *BenchmarkRunner) awaitIdle() error {
	if r.MaxLoad <= 0 {
		return nil
	}

	deadline := time.Now().Add(r.WaitForIdle)
	for {
		load, err := loadAverage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot read load average (%v); skipping the idle check\n", err)
			return nil
		}
		if load <= r.MaxLoad {
			fmt.Fprintf(os.Stderr, "Load average %.2f is below the %.2f threshold; starting.\n", load, r.MaxLoad)
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("load average is %.2f, above the -max-load threshold of %.2f: "+
				"benchmarking now would measure the contention, not the code "+
				"(raise -max-load, pass -wait-for-idle, or wait for the machine to settle)",
				load, r.MaxLoad)
		}
		fmt.Fprintf(os.Stderr, "Load average %.2f > %.2f; waiting (%s left)...\n",
			load, r.MaxLoad, time.Until(deadline).Round(time.Second))
		time.Sleep(30 * time.Second)
	}
}

// loadAverage returns the 1-minute load average.
func loadAverage() (float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("unexpected /proc/loadavg format")
	}
	return strconv.ParseFloat(fields[0], 64)
}

// label returns the run label, defaulting to the build tags so that a
// `-tags purego` run is not silently indistinguishable from a SIMD run.
func (r *BenchmarkRunner) label() string {
	if r.Label != "" {
		return r.Label
	}
	if r.Tags != "" {
		return r.Tags
	}
	return "simd"
}

// collectPlanInfo runs the bench package's plan-route dump inside a test
// binary built with the same tags as the benchmarks, so the recorded routes
// match the code that was measured.
func (r *BenchmarkRunner) collectPlanInfo() error {
	tmp, err := os.CreateTemp("", "planinfo-*.json")
	if err != nil {
		return err
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	cmd := exec.Command("go", "test",
		"-run", "TestWritePlanInfo",
		"-count", "1",
		"-tags="+r.Tags,
		r.benchDir(),
	)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("FFT_BENCH_MAX=%d", r.MaxSize),
		fmt.Sprintf("GOAMD64=%s", r.GOAMD64),
		"FFT_BENCH_PLANINFO="+path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &r.PlanInfos); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Collected %d algo-fft plan routes\n", len(r.PlanInfos))
	return nil
}

// writeJSON flattens the nested result map into a stable, sorted list.
func (r *BenchmarkRunner) writeJSON() error {
	file := RunFile{
		Meta: RunMeta{
			Label:     r.label(),
			Tags:      r.Tags,
			GOAMD64:   r.GOAMD64,
			GoVersion: runtime.Version(),
			CPU:       cpuModel(),
			MaxSize:   r.MaxSize,
			AlgoFFT:   algoFFTVersion(),
		},
		PlanInfos: r.PlanInfos,
	}

	for _, byLib := range r.Results {
		for _, bySize := range byLib {
			for _, result := range bySize {
				result.Class = string(bench.ClassOf(result.Size))
				file.Results = append(file.Results, *result)
			}
		}
	}

	sort.Slice(file.Results, func(i, j int) bool {
		a, b := file.Results[i], file.Results[j]
		if a.BenchType != b.BenchType {
			return a.BenchType < b.BenchType
		}
		if a.Size != b.Size {
			return a.Size < b.Size
		}
		return a.Library < b.Library
	})

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(r.JSON); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(r.JSON, append(data, '\n'), 0o644)
}

// cpuModel returns the host CPU model name, or the empty string where
// /proc/cpuinfo is unavailable.
func cpuModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if name, ok := strings.CutPrefix(line, "model name"); ok {
			if _, value, found := strings.Cut(name, ":"); found {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

// algoFFTVersion reports the algo-fft version the benchmarks were built
// against, so a result file can be traced back to a release.
func algoFFTVersion() string {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Version}}", "github.com/cwbudde/algo-fft").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// benchDir locates the benchmark package relative to the working directory.
func (r *BenchmarkRunner) benchDir() string {
	benchDir := "./bench"
	if _, err := os.Stat(benchDir); os.IsNotExist(err) {
		// Try from project root
		if wd, err := os.Getwd(); err == nil {
			if strings.HasSuffix(wd, "/cmd/benchrunner") {
				benchDir = "../../bench"
			}
		}
	}
	return benchDir
}

func (r *BenchmarkRunner) runBenchmarks() error {
	benchDir := r.benchDir()

	// -timeout 0 disables go test's 10 minute default, which a full sweep
	// (six benchmark types over ~40 sizes) exceeds on a laptop.
	cmd := exec.Command("go", "test",
		"-bench", ".",
		"-benchmem",
		"-run", "^$",
		"-timeout", "0",
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
	if cpu := cpuModel(); cpu != "" {
		fmt.Fprintf(&b, "Machine: %s, %s, algo-fft %s\n\n", cpu, runtime.Version(), algoFFTVersion())
	}
	fmt.Fprintf(&b, "Notes:\n\n")
	fmt.Fprintf(&b, "- Results are from the latest local run.\n")
	fmt.Fprintf(&b, "- `algo-fft` benchmarks include both complex128 and complex64.\n")
	fmt.Fprintf(&b, "- `go-fftw` (FFTW3) is used as the **baseline** for comparison.\n")
	fmt.Fprintf(&b, "- `go-fftw` requires FFTW shared libraries.\n")
	fmt.Fprintf(&b, "- `go-dsp-fft` allocates on every call (no reusable plan).\n")
	fmt.Fprintf(&b, "- `FFTAny` covers non-power-of-two lengths; `takatoh` is excluded there because it is radix-2 only.\n")
	fmt.Fprintf(&b, "- **Speedup** shows performance relative to go-fftw baseline (higher is better).\n")
	fmt.Fprintf(&b, "- The `*32` sections are complex64. FFTW has no complex64 arm, so they are\n")
	fmt.Fprintf(&b, "  compared against `algo-fft`'s own complex128 series instead: their speedup\n")
	fmt.Fprintf(&b, "  column is the complex128/complex64 ratio, i.e. what the extra precision costs.\n\n")

	if len(r.PlanInfos) > 0 {
		r.writePlanRouteTable(&b)
	}

	// Sort benchmark types
	typeOrder := map[string]int{"FFT": 0, "IFFT": 1, "FFT32": 2, "IFFT32": 3, "FFTAny": 4, "FFTAny32": 5}
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
		base, hasBaseline := r.baselineFor(benchType)

		if !hasBaseline {
			fmt.Fprintf(&b, "### Error: Baseline library '%s' not found\n\n", r.Baseline)
			continue
		}
		baselineData := base.data

		// Baseline table. Skipped when the baseline was borrowed from another
		// section — its own numbers are tabulated there.
		if base.own {
			r.writeBaselineTable(&b, base.label, baselineData)
		} else {
			fmt.Fprintf(&b, "Compared against **%s**, i.e. the speedup column is the\n"+
				"complex128/complex64 ratio at that size.\n\n", base.label)
		}

		// Comparison tables for other libraries: the known ones in a fixed
		// display order, then anything unrecognised sorted alphabetically.
		// The split point is how many known libraries were actually present,
		// not len(libOrder) — a benchmark type that only some libraries
		// implement (FFT32, FFTAny) has fewer.
		libOrder := []string{algoFFTLibrary, "go-dsp-fft", "gonum", "takatoh"}
		var otherLibs []string
		for _, lib := range libOrder {
			if _, ok := libraries[lib]; ok && lib != r.Baseline {
				otherLibs = append(otherLibs, lib)
			}
		}
		known := len(otherLibs)
		for lib := range libraries {
			if !contains(libOrder, lib) && lib != r.Baseline {
				otherLibs = append(otherLibs, lib)
			}
		}
		sort.Strings(otherLibs[known:])

		for _, library := range otherLibs {
			r.writeComparisonTable(&b, benchType, library, base.column, libraries[library], baselineData)
		}
	}

	return b.String()
}

// writePlanRouteTable documents which algorithm algo-fft resolved for each
// non-power-of-two length. Without it the FFTAny numbers are unreadable: the
// same library is running Rader at one size and Bluestein at the next.
func (r *BenchmarkRunner) writePlanRouteTable(w io.Writer) {
	fmt.Fprintf(w, "## algo-fft plan routes\n\n")
	fmt.Fprintf(w, "Resolved by this build on this CPU. `rader`/`bluestein` are both\n")
	fmt.Fprintf(w, "reported under the Bluestein strategy; the algorithm column is what\n")
	fmt.Fprintf(w, "distinguishes them.\n\n")
	fmt.Fprintf(w, "| Size  | Class              | Strategy   | Algorithm (c128)          | Algorithm (c64)           |\n")
	fmt.Fprintf(w, "| ----- | ------------------ | ---------- | ------------------------- | ------------------------- |\n")

	for _, info := range r.PlanInfos {
		if info.Class == string(bench.ClassPow2) {
			continue
		}
		fmt.Fprintf(w, "| %-5d | %-18s | %-10s | %-25s | %-25s |\n",
			info.N, info.Class, info.Strategy, info.Algorithm, info.Algorithm32)
	}
	fmt.Fprintf(w, "\n")
}

// baselineRef is the series a section's speedup column is computed against.
// own is false when it was borrowed from another section, which is how the
// complex64 benchmarks get a baseline at all.
type baselineRef struct {
	label  string // full name, for the section heading
	column string // short name, for the speedup column header
	data   map[int]*BenchmarkResult
	own    bool
}

// complex128Twin maps a complex64 benchmark type to the complex128 one that
// runs the same sizes.
var complex128Twin = map[string]string{
	"FFT32":    "FFT",
	"IFFT32":   "IFFT",
	"FFTAny32": "FFTAny",
}

// baselineFor resolves the baseline for one benchmark type. The configured
// library wins where it ran. FFTW has no complex64 arm, so the *32 sections
// would otherwise be dropped entirely despite being measured; they fall back
// to algo-fft's own complex128 series, which turns their speedup column into
// the complex128/complex64 ratio — the number that says what the extra
// precision costs.
func (r *BenchmarkRunner) baselineFor(benchType string) (baselineRef, bool) {
	libraries := r.Results[benchType]
	if data, ok := libraries[r.Baseline]; ok {
		label := r.Baseline
		if r.Baseline == "go-fftw" {
			label = "go-fftw (FFTW3)"
		}
		return baselineRef{label: label, column: "baseline", data: data, own: true}, true
	}

	if twin, ok := complex128Twin[benchType]; ok {
		if data, ok := r.Results[twin][algoFFTLibrary]; ok {
			return baselineRef{
				label:  algoFFTLibrary + " " + twin + " (complex128)",
				column: "complex128",
				data:   data,
			}, true
		}
	}

	return baselineRef{}, false
}

func (r *BenchmarkRunner) writeBaselineTable(w io.Writer, baselineName string, data map[int]*BenchmarkResult) {
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

func (r *BenchmarkRunner) writeComparisonTable(w io.Writer, benchType, library, baselineColumn string, data, baselineData map[int]*BenchmarkResult) {
	sectionSuffix := ""
	if strings.HasPrefix(benchType, "IFFT") && library != algoFFTLibrary {
		sectionSuffix = fmt.Sprintf(" (%s)", benchType)
	}

	fmt.Fprintf(w, "### %s%s\n\n", library, sectionSuffix)
	fmt.Fprintf(w, "| Size  | ns/op  | Speedup vs %-8s | MB/s     | B/op   | allocs/op |\n", baselineColumn)
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
