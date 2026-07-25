package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	algofft "github.com/cwbudde/algo-fft"
)

// PlanInfo records which route algo-fft picked for one length. The strategy
// is the coarse kernel family; the algorithm is the concrete kernel or
// codelet bound to the plan (e.g. "rader", "bluestein", "dit1024_radix4_avx2"),
// which is what distinguishes Rader from Bluestein inside the same family.
type PlanInfo struct {
	N           int    `json:"n"`
	Class       string `json:"class"`
	Strategy    string `json:"strategy"`
	Algorithm   string `json:"algorithm"`
	Strategy32  string `json:"strategy32"`
	Algorithm32 string `json:"algorithm32"`
}

// CollectPlanInfo builds a plan for every size in the benchmark tables and
// records the route algo-fft resolved for it. Because plan resolution depends
// on the build (SIMD codelets vs the purego fallback) and on the host CPU,
// this must run in the same binary as the benchmarks it annotates.
func CollectPlanInfo(maxPow2 int) ([]PlanInfo, error) {
	sizes := make(map[int]SizeClass, 32)
	for _, n := range Pow2Sizes(maxPow2) {
		sizes[n] = ClassPow2
	}
	for _, spec := range AnySizes() {
		sizes[spec.N] = spec.Class
	}

	keys := make([]int, 0, len(sizes))
	for n := range sizes {
		keys = append(keys, n)
	}
	sort.Ints(keys)

	infos := make([]PlanInfo, 0, len(keys))
	for _, n := range keys {
		info := PlanInfo{N: n, Class: string(sizes[n])}

		plan64, err := algofft.NewPlan64(n)
		if err != nil {
			return nil, fmt.Errorf("plan complex128 n=%d: %w", n, err)
		}
		info.Strategy = plan64.KernelStrategy().String()
		info.Algorithm = plan64.Algorithm()
		plan64.Close()

		plan32, err := algofft.NewPlan32(n)
		if err != nil {
			return nil, fmt.Errorf("plan complex64 n=%d: %w", n, err)
		}
		info.Strategy32 = plan32.KernelStrategy().String()
		info.Algorithm32 = plan32.Algorithm()
		plan32.Close()

		infos = append(infos, info)
	}

	return infos, nil
}

// WritePlanInfo writes the collected plan routes to path as JSON.
func WritePlanInfo(path string, infos []PlanInfo) error {
	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
