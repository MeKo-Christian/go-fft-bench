package main

import (
	"fmt"
	"io"
	"sort"
)

// benchTypeOrder is the narrative order for a comparison: complex128 forward
// first, then its inverse and the complex64 pair, then the arbitrary lengths.
var benchTypeOrder = []string{"FFT", "IFFT", "FFT32", "IFFT32", "FFTAny", "FFTAny32"}

// writeComparison reports what changed between two sweeps of the same
// benchmark suite — typically two algo-fft versions measured on one machine.
//
// Only algo-fft is reported as a result: it is the library whose code changed.
// The competitors are unchanged between the runs, so their deltas measure the
// machine rather than the library, and that is exactly what makes them useful
// as a drift check (see writeDriftCheck).
func writeComparison(w io.Writer, old, new *Run) {
	fmt.Fprintf(w, "# Benchmark comparison\n\n")
	writeRunHeader(w, old, new)

	for _, typ := range benchTypeOrder {
		if len(new.Sizes(typ)) == 0 {
			continue
		}
		compareBenchType(w, old, new, typ)
	}

	writeDriftCheck(w, old, new)
	writePlanDiff(w, old, new)
}

// writeRunHeader echoes both runs' provenance. A comparison across machines,
// Go versions or GOAMD64 levels measures the difference between those as much
// as the library's, so a mismatch is called out rather than left to be noticed
// in the numbers.
func writeRunHeader(w io.Writer, old, new *Run) {
	fmt.Fprintf(w, "%-10s %-12s %-38s %-12s %s\n", "run", "algo-fft", "cpu", "go", "GOAMD64")
	fmt.Fprintf(w, "%s\n", "--------------------------------------------------------------------------------")
	for _, r := range []struct {
		label string
		run   *Run
	}{{"baseline", old}, {"current", new}} {
		m := r.run.Meta
		fmt.Fprintf(w, "%-10s %-12s %-38s %-12s %s\n", r.label, m.AlgoFFT, m.CPU, m.GoVersion, m.GOAMD64)
	}
	fmt.Fprintf(w, "\n")

	var warnings []string
	if old.Meta.CPU != new.Meta.CPU {
		warnings = append(warnings, "different CPUs")
	}
	if old.Meta.GoVersion != new.Meta.GoVersion {
		warnings = append(warnings, "different Go versions")
	}
	if old.Meta.GOAMD64 != new.Meta.GOAMD64 {
		warnings = append(warnings, "different GOAMD64 levels")
	}
	if old.Meta.Tags != new.Meta.Tags {
		warnings = append(warnings, "different build tags")
	}
	if len(warnings) > 0 {
		fmt.Fprintf(w, "WARNING: the runs are not directly comparable — %s.\n",
			joinWords(warnings))
		fmt.Fprintf(w, "         Treat every delta below as suspect.\n\n")
	}
}

// delta is one size's before/after pair.
type delta struct {
	n        int
	class    string
	old, new float64
}

// ratio is new/old: below 1 means the current run is faster.
func (d delta) ratio() float64 { return d.new / d.old }

// pctChange is the delta as a signed percentage, negative meaning faster.
func (d delta) pctChange() float64 { return (d.new/d.old - 1) * 100 }

// compareBenchType prints one benchmark type's per-size table plus the
// aggregate, grouped by size class so that the classes a release targets can
// be read off directly.
func compareBenchType(w io.Writer, old, new *Run, typ string) {
	deltas, onlyNew, onlyOld := collectDeltas(old, new, typ, "algo-fft")
	if len(deltas) == 0 && len(onlyNew) == 0 && len(onlyOld) == 0 {
		return
	}

	fmt.Fprintf(w, "## %s (algo-fft)\n\n", typ)

	if len(deltas) > 0 {
		fmt.Fprintf(w, "%9s %-20s %12s %12s %9s\n", "n", "class", "old ns/op", "new ns/op", "change")
		fmt.Fprintf(w, "%s\n", "--------------------------------------------------------------------------------")
		for _, d := range deltas {
			fmt.Fprintf(w, "%9d %-20s %12.1f %12.1f %8.1f%%\n",
				d.n, d.class, d.old, d.new, d.pctChange())
		}
		fmt.Fprintf(w, "\n")

		writeAggregate(w, "overall", deltas)

		// Per-class only where there is more than one class to separate;
		// the power-of-two benchmarks are a single class by construction.
		byClass := groupByClass(deltas)
		if len(byClass) > 1 {
			fmt.Fprintf(w, "\nby class:\n\n")
			for _, class := range new.Classes(typ) {
				if group, ok := byClass[class]; ok {
					writeAggregate(w, class, group)
				}
			}
		}
		fmt.Fprintf(w, "\n")
	}

	// Sizes that exist in one run only are reported rather than dropped: a
	// changed size table is a real difference between the runs, and silently
	// intersecting them would hide it.
	if len(onlyNew) > 0 {
		fmt.Fprintf(w, "only in the current run: %v\n", onlyNew)
	}
	if len(onlyOld) > 0 {
		fmt.Fprintf(w, "only in the baseline run: %v\n", onlyOld)
	}
	if len(onlyNew) > 0 || len(onlyOld) > 0 {
		fmt.Fprintf(w, "\n")
	}
}

// writeAggregate prints the geomean and the extremes for one set of deltas.
// The geomean is the right average here because these are ratios.
func writeAggregate(w io.Writer, label string, deltas []delta) {
	ratios := make([]float64, 0, len(deltas))
	best, worst := deltas[0], deltas[0]
	for _, d := range deltas {
		if d.old <= 0 || d.new <= 0 {
			continue
		}
		ratios = append(ratios, d.ratio())
		if d.ratio() < best.ratio() {
			best = d
		}
		if d.ratio() > worst.ratio() {
			worst = d
		}
	}
	if len(ratios) == 0 {
		return
	}
	fmt.Fprintf(w, "  %-20s geomean %+6.1f%%   best %+6.1f%% (n=%d)   worst %+6.1f%% (n=%d)\n",
		label, (GeoMean(ratios)-1)*100,
		best.pctChange(), best.n, worst.pctChange(), worst.n)
}

// collectDeltas pairs one library's measurements across the two runs, and
// reports the sizes that only one run has.
func collectDeltas(old, new *Run, typ, lib string) (paired []delta, onlyNew, onlyOld []int) {
	for _, n := range new.Sizes(typ) {
		cur, okCur := new.Lookup(typ, lib, n)
		prev, okPrev := old.Lookup(typ, lib, n)
		switch {
		case !okCur:
			continue
		case !okPrev:
			onlyNew = append(onlyNew, n)
		case cur.NsOp <= 0 || prev.NsOp <= 0:
			continue
		default:
			paired = append(paired, delta{n: n, class: cur.Class, old: prev.NsOp, new: cur.NsOp})
		}
	}
	for _, n := range old.Sizes(typ) {
		if _, ok := old.Lookup(typ, lib, n); !ok {
			continue
		}
		if _, ok := new.Lookup(typ, lib, n); !ok {
			onlyOld = append(onlyOld, n)
		}
	}
	sort.Ints(onlyNew)
	sort.Ints(onlyOld)
	return paired, onlyNew, onlyOld
}

func groupByClass(deltas []delta) map[string][]delta {
	out := map[string][]delta{}
	for _, d := range deltas {
		out[d.class] = append(out[d.class], d)
	}
	return out
}

// writeDriftCheck reports the same aggregate for libraries whose code did not
// change between the runs. Their numbers should reproduce; a geomean far from
// zero means the machine was in a different state (thermal, load, frequency)
// and the algo-fft deltas carry that same error.
func writeDriftCheck(w io.Writer, old, new *Run) {
	fmt.Fprintf(w, "## Drift check (unchanged libraries — expect ~0%%)\n\n")
	for _, lib := range []string{"go-fftw", "gonum", "go-dsp-fft", "takatoh"} {
		var all []delta
		for _, typ := range benchTypeOrder {
			paired, _, _ := collectDeltas(old, new, typ, lib)
			all = append(all, paired...)
		}
		if len(all) == 0 {
			continue
		}
		writeAggregate(w, lib, all)
	}
	fmt.Fprintf(w, "\n")
}

// writePlanDiff lists the lengths where algo-fft resolved a different route.
// A large delta at a size whose route changed is explained; one at a size
// whose route did not is a kernel-level change (or noise).
func writePlanDiff(w io.Writer, old, new *Run) {
	// The two precisions resolve independently, so a length is reported once
	// per precision that changed. Reporting only complex128 would print rows
	// whose two route columns are identical, which is what a complex64-only
	// change looks like from there.
	type change struct {
		n         int
		class     string
		precision string
		old, new  string
	}
	var changes []change

	sizes := make([]int, 0, len(new.PlanInfos))
	for _, p := range new.PlanInfos {
		sizes = append(sizes, p.N)
	}
	sort.Ints(sizes)

	for _, n := range sizes {
		cur, okCur := new.Plan(n)
		prev, okPrev := old.Plan(n)
		if !okCur || !okPrev {
			continue
		}
		for _, p := range []struct {
			label            string
			oldAlgo, newAlgo string
			oldStr, newStr   string
		}{
			{"complex128", prev.Algorithm, cur.Algorithm, prev.Strategy, cur.Strategy},
			{"complex64", prev.Algorithm32, cur.Algorithm32, prev.Strategy32, cur.Strategy32},
		} {
			if p.oldAlgo == p.newAlgo && p.oldStr == p.newStr {
				continue
			}
			changes = append(changes, change{
				n: n, class: cur.Class, precision: p.label,
				old: p.oldAlgo + "/" + p.oldStr,
				new: p.newAlgo + "/" + p.newStr,
			})
		}
	}

	fmt.Fprintf(w, "## algo-fft plan-route changes\n\n")
	if len(changes) == 0 {
		fmt.Fprintf(w, "none — every length resolved to the same route in both runs.\n\n")
		return
	}
	fmt.Fprintf(w, "%7s %-18s %-11s %-30s %-30s\n", "n", "class", "precision", "old (algorithm/strategy)", "new (algorithm/strategy)")
	fmt.Fprintf(w, "%s\n", "----------------------------------------------------------------------------------------------------")
	for _, c := range changes {
		fmt.Fprintf(w, "%7d %-18s %-11s %-30s %-30s\n", c.n, c.class, c.precision, c.old, c.new)
	}
	fmt.Fprintf(w, "\n")
}

func joinWords(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return fmt.Sprintf("%s and %s", joinList(words[:len(words)-1]), words[len(words)-1])
}

func joinList(words []string) string {
	out := words[0]
	for _, wd := range words[1:] {
		out += ", " + wd
	}
	return out
}
