package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

// The types below mirror the JSON that `benchrunner -json` writes. They are
// duplicated rather than imported from ./bench on purpose: the bench package
// links cgo FFTW, and a plotting tool has no business pulling that in.

// Result is one measured (benchmark, library, size) triple.
type Result struct {
	BenchType string  `json:"benchType"`
	Library   string  `json:"library"`
	Size      int     `json:"size"`
	Class     string  `json:"class"`
	NsOp      float64 `json:"nsOp"`
	MBs       float64 `json:"mbPerSec"`
	BOp       int     `json:"bytesPerOp"`
	AllocsOp  int     `json:"allocsPerOp"`
}

// PlanInfo records which algorithm algo-fft resolved for one length.
type PlanInfo struct {
	N           int    `json:"n"`
	Class       string `json:"class"`
	Strategy    string `json:"strategy"`
	Algorithm   string `json:"algorithm"`
	Strategy32  string `json:"strategy32"`
	Algorithm32 string `json:"algorithm32"`
}

// Meta describes the machine and build one result set came from.
type Meta struct {
	Label     string `json:"label"`
	Tags      string `json:"tags"`
	GOAMD64   string `json:"goamd64"`
	GoVersion string `json:"goVersion"`
	CPU       string `json:"cpu"`
	MaxSize   int    `json:"maxSize"`
	AlgoFFT   string `json:"algoFFTVersion"`
}

// Run is one benchmark sweep: everything in it was measured by the same
// binary on the same machine, so its numbers are comparable with each other
// and only cautiously with another Run's.
type Run struct {
	Meta      Meta       `json:"meta"`
	Results   []Result   `json:"results"`
	PlanInfos []PlanInfo `json:"planInfos"`

	index map[key]Result
	plans map[int]PlanInfo
}

type key struct {
	benchType string
	library   string
	size      int
}

// LoadRun reads and indexes a benchrunner JSON file.
func LoadRun(path string) (*Run, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var run Run
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(run.Results) == 0 {
		return nil, fmt.Errorf("%s: no results", path)
	}

	run.index = make(map[key]Result, len(run.Results))
	for _, r := range run.Results {
		run.index[key{r.BenchType, r.Library, r.Size}] = r
	}
	run.plans = make(map[int]PlanInfo, len(run.PlanInfos))
	for _, p := range run.PlanInfos {
		run.plans[p.N] = p
	}

	return &run, nil
}

// Lookup returns one measurement, reporting whether it was present: a
// library that cannot handle a size simply has no row.
func (r *Run) Lookup(benchType, library string, size int) (Result, bool) {
	res, ok := r.index[key{benchType, library, size}]
	return res, ok
}

// Plan returns the resolved algo-fft route for a size.
func (r *Run) Plan(size int) (PlanInfo, bool) {
	p, ok := r.plans[size]
	return p, ok
}

// Libraries returns the libraries measured for a benchmark type, in the
// preferred display order with any unknown library appended alphabetically.
func (r *Run) Libraries(benchType string) []string {
	seen := map[string]bool{}
	for _, res := range r.Results {
		if res.BenchType == benchType {
			seen[res.Library] = true
		}
	}

	preferred := []string{"algo-fft", "go-fftw", "gonum", "go-dsp-fft", "takatoh"}
	out := make([]string, 0, len(seen))
	for _, lib := range preferred {
		if seen[lib] {
			out = append(out, lib)
			delete(seen, lib)
		}
	}

	rest := make([]string, 0, len(seen))
	for lib := range seen {
		rest = append(rest, lib)
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// Sizes returns the sorted sizes measured for a benchmark type.
func (r *Run) Sizes(benchType string) []int {
	seen := map[int]bool{}
	for _, res := range r.Results {
		if res.BenchType == benchType {
			seen[res.Size] = true
		}
	}
	out := make([]int, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

// SizesOfClass returns the sizes of one class, sorted.
func (r *Run) SizesOfClass(benchType, class string) []int {
	var out []int
	for _, n := range r.Sizes(benchType) {
		if res, ok := r.Lookup(benchType, "algo-fft", n); ok && res.Class == class {
			out = append(out, n)
		}
	}
	return out
}

// Classes returns the size classes present for a benchmark type, in a fixed
// narrative order: smooth lengths first, then the hard ones.
func (r *Run) Classes(benchType string) []string {
	seen := map[string]bool{}
	for _, res := range r.Results {
		if res.BenchType == benchType && res.Class != "" {
			seen[res.Class] = true
		}
	}

	order := []string{"pow2", "5-smooth", "7/11-smooth", "prime (smooth p-1)", "prime (rough p-1)", "practical"}
	out := make([]string, 0, len(seen))
	for _, c := range order {
		if seen[c] {
			out = append(out, c)
		}
	}
	return out
}

// MFlops converts a transform time into the pseudo-flop rate the FFT
// literature uses for comparing sizes: 5*n*log2(n) is the conventional
// operation count of a radix-2 complex FFT. It is not an instruction count —
// its only job is to divide out the O(n log n) growth so that curves for
// different sizes are readable on one axis.
func MFlops(n int, nsOp float64) float64 {
	if nsOp <= 0 || n < 2 {
		return 0
	}
	ops := 5.0 * float64(n) * math.Log2(float64(n))
	return ops / nsOp * 1e3 // ops/ns -> Mops/s
}

// GeoMean returns the geometric mean, the right average for ratios.
func GeoMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	count := 0
	for _, v := range values {
		if v <= 0 {
			continue
		}
		sum += math.Log(v)
		count++
	}
	if count == 0 {
		return 0
	}
	return math.Exp(sum / float64(count))
}
