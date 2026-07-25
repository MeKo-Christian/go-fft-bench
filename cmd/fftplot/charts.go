package main

import (
	"fmt"
	"math"
	"sort"

	_ "github.com/cwbudde/matplotlib-go/backends/all" // registers the .png writer
	"github.com/cwbudde/matplotlib-go/color"
	"github.com/cwbudde/matplotlib-go/core"
	"github.com/cwbudde/matplotlib-go/geom"
	"github.com/cwbudde/matplotlib-go/render"
	"github.com/cwbudde/matplotlib-go/style"
	"github.com/cwbudde/matplotlib-go/ticker"
	"github.com/cwbudde/matplotlib-go/transform"
)

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

func f64(v float64) *float64 { return &v }

func marker(m core.MarkerType) *core.MarkerType { return &m }

// palette gives each library a stable colour across every chart, so a reader
// who learns "orange is FFTW" on the first chart keeps that on the last.
var palette = map[string]render.Color{
	"algo-fft":   color.Tab10[0],
	"go-fftw":    color.Tab10[1],
	"gonum":      color.Tab10[2],
	"go-dsp-fft": color.Tab10[3],
	"takatoh":    color.Tab10[4],
}

// markers likewise, so the charts survive being printed in greyscale.
var markers = map[string]core.MarkerType{
	"algo-fft":   core.MarkerCircle,
	"go-fftw":    core.MarkerSquare,
	"gonum":      core.MarkerTriangleUp,
	"go-dsp-fft": core.MarkerDiamond,
	"takatoh":    core.MarkerCross,
}

func libColor(lib string) render.Color {
	if c, ok := palette[lib]; ok {
		return c
	}
	return color.Tab10[7]
}

func libMarker(lib string) core.MarkerType {
	if m, ok := markers[lib]; ok {
		return m
	}
	return core.MarkerPlus
}

// classColors keep the size classes distinguishable in the bar charts.
var classColors = map[string]render.Color{
	"pow2":               color.Tab10[7],
	"5-smooth":           color.Tab10[0],
	"7/11-smooth":        color.Tab10[2],
	"prime (smooth p-1)": color.Tab10[4],
	"prime (rough p-1)":  color.Tab10[3],
	"practical":          color.Tab10[5],
}

func classColor(class string) render.Color {
	if c, ok := classColors[class]; ok {
		return c
	}
	return color.Tab10[7]
}

// newFigure builds a figure with a single axes and the publication theme.
func newFigure(w, h int, dpi float64) (*core.Figure, *core.Axes) {
	fig := core.NewFigure(w, h,
		style.WithTheme(style.MustTheme("publication")),
		style.WithDPI(dpi),
	)
	ax := fig.AddAxes(geom.Rect{
		Min: geom.Pt{X: 0.115, Y: 0.145},
		Max: geom.Pt{X: 0.975, Y: 0.905},
	})
	return fig, ax
}

// logLimits sets a logarithmic y range that contains every value with a
// little margin, so nothing is silently autoscaled off the chart.
func logLimits(ax *core.Axes, values []float64) {
	min, max := 0.0, 0.0
	for _, v := range values {
		if v <= 0 {
			continue
		}
		if min == 0 || v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	if min == 0 || max == 0 {
		return
	}
	ax.SetYLim(min/1.8, max*1.8)
}

// headroom sets a linear y range from zero with space above the tallest bar
// for the legend. Bar charts must not use a log axis: a bar runs from the
// baseline, and log(0) is not a coordinate — the renderer fills the whole
// column when asked.
func headroom(ax *core.Axes, values []float64, factor float64) {
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		return
	}
	ax.SetYLim(0, max*factor)
}

// useLog2X turns the x axis into a power-of-two axis with ticks at every
// power of two, which is the only readable choice for FFT sizes.
func useLog2X(ax *core.Axes, lo, hi float64) error {
	if err := ax.SetXScale("log", transform.WithScaleBase(2)); err != nil {
		return err
	}
	ax.SetXLim(lo, hi)
	ax.XAxis.Locator = ticker.LogLocator{Base: 2}
	ax.XAxis.Formatter = ticker.LogFormatter{Base: 2}
	return nil
}

// styleGrid applies the same restrained grid to every chart.
func styleGrid(ax *core.Axes) {
	g := ax.AddYGrid()
	g.Dashes = []float64{2, 3}
	g.LineWidth = 0.6
	ax.SetAxisBelow(true)
}

// caption writes the provenance line every chart carries, so a chart lifted
// out of the article still says what machine and version produced it.
func caption(fig *core.Figure, meta Meta) {
	text := fmt.Sprintf("algo-fft %s · %s · %s · GOAMD64=%s",
		meta.AlgoFFT, meta.GoVersion, meta.CPU, meta.GOAMD64)
	fig.Text(0.5, 0.02, text, core.TextOptions{
		HAlign:   core.TextAlignCenter,
		FontSize: 7.5,
		Color:    render.Color{R: 0.45, G: 0.45, B: 0.45, A: 1},
	})
}

// plotLine draws one library's series with its stable colour and marker.
func plotLine(ax *core.Axes, lib string, xs, ys []float64, label string) error {
	c := libColor(lib)
	m := libMarker(lib)
	_, err := ax.Plot(xs, ys, core.PlotOptions{
		Label:           label,
		Color:           &c,
		LineWidth:       f64(1.6),
		LineStyle:       core.LineStyleSolid,
		Marker:          marker(m),
		MarkerSize:      f64(4.5),
		MarkerFaceColor: &c,
	})
	return err
}

// hline draws a horizontal reference line at y.
func hline(ax *core.Axes, xs []float64, y float64, label string) error {
	line := make([]float64, len(xs))
	for i := range line {
		line[i] = y
	}
	grey := render.Color{R: 0.35, G: 0.35, B: 0.35, A: 1}
	_, err := ax.Plot(xs, line, core.PlotOptions{
		Label:     label,
		Color:     &grey,
		LineWidth: f64(1.0),
		LineStyle: core.LineStyleDashed,
	})
	return err
}

// ---------------------------------------------------------------------------
// 01 — throughput at power-of-two sizes
// ---------------------------------------------------------------------------

func plotThroughputPow2(run *Run, path string, dpi float64) error {
	fig, ax := newFigure(1100, 660, dpi)

	sizes := run.Sizes("FFT")
	if len(sizes) == 0 {
		return fmt.Errorf("no FFT results")
	}

	for _, lib := range run.Libraries("FFT") {
		var xs, ys []float64
		for _, n := range sizes {
			res, ok := run.Lookup("FFT", lib, n)
			if !ok {
				continue
			}
			xs = append(xs, float64(n))
			ys = append(ys, MFlops(n, res.NsOp))
		}
		if len(xs) == 0 {
			continue
		}
		if err := plotLine(ax, lib, xs, ys, lib); err != nil {
			return err
		}
	}

	if err := useLog2X(ax, float64(sizes[0])/1.4, float64(sizes[len(sizes)-1])*1.4); err != nil {
		return err
	}
	ax.SetTitle("Complex128 forward FFT throughput, power-of-two lengths")
	ax.SetXLabel("transform length n")
	ax.SetYLabel("pseudo-MFLOPS  (5 n log₂ n / t)")
	styleGrid(ax)
	ax.AddLegend().Location = core.LegendLowerLeft
	caption(fig, run.Meta)

	return fig.Save(path)
}

// ---------------------------------------------------------------------------
// 02 — speedup relative to FFTW
// ---------------------------------------------------------------------------

func plotSpeedupVsFFTW(run *Run, path string, dpi float64) error {
	fig, ax := newFigure(1100, 660, dpi)

	sizes := run.Sizes("FFT")
	if len(sizes) == 0 {
		return fmt.Errorf("no FFT results")
	}

	var axisXs []float64
	for _, n := range sizes {
		axisXs = append(axisXs, float64(n))
	}

	var allRatios []float64
	for _, lib := range run.Libraries("FFT") {
		if lib == "go-fftw" {
			continue
		}
		var xs, ys []float64
		for _, n := range sizes {
			base, okBase := run.Lookup("FFT", "go-fftw", n)
			res, ok := run.Lookup("FFT", lib, n)
			if !ok || !okBase || res.NsOp <= 0 {
				continue
			}
			xs = append(xs, float64(n))
			ys = append(ys, base.NsOp/res.NsOp)
		}
		if len(xs) == 0 {
			continue
		}
		allRatios = append(allRatios, ys...)
		if err := plotLine(ax, lib, xs, ys, lib); err != nil {
			return err
		}
	}

	if err := hline(ax, axisXs, 1.0, "FFTW3 (baseline)"); err != nil {
		return err
	}

	if err := useLog2X(ax, float64(sizes[0])/1.4, float64(sizes[len(sizes)-1])*1.4); err != nil {
		return err
	}
	if err := ax.SetYScale("log", transform.WithScaleBase(10)); err != nil {
		return err
	}
	// Autoscaling clips the slowest libraries off the bottom of a log axis,
	// which would quietly hide exactly the comparison the chart is for.
	logLimits(ax, append(allRatios, 1.0))
	ax.SetTitle("Speed relative to FFTW3, complex128 forward FFT")
	ax.SetXLabel("transform length n")
	ax.SetYLabel("speedup vs FFTW3  (>1 is faster)")
	styleGrid(ax)
	ax.AddLegend().Location = core.LegendLowerLeft
	caption(fig, run.Meta)

	return fig.Save(path)
}

// ---------------------------------------------------------------------------
// 03 — arbitrary lengths, summarised by size class
// ---------------------------------------------------------------------------

func plotNonPow2ByClass(run *Run, path string, dpi float64) error {
	fig, ax := newFigure(1100, 660, dpi)

	classes := run.Classes("FFTAny")
	if len(classes) == 0 {
		return fmt.Errorf("no FFTAny results")
	}

	libs := []string{"algo-fft", "gonum", "go-dsp-fft"}
	width := 0.8 / float64(len(libs))

	centers := make([]float64, len(classes))
	for i := range classes {
		centers[i] = float64(i)
	}

	var allValues []float64
	for li, lib := range libs {
		offset := (float64(li)-float64(len(libs)-1)/2)*width + 0.0
		var pos, vals []float64

		for ci, class := range classes {
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
				continue
			}
			pos = append(pos, centers[ci]+offset)
			vals = append(vals, GeoMean(ratios))
		}
		if len(pos) == 0 {
			continue
		}
		allValues = append(allValues, vals...)

		c := libColor(lib)
		edge := render.Color{R: 0.15, G: 0.15, B: 0.15, A: 1}
		if _, err := ax.Bar(pos, vals, core.BarOptions{
			Label:     lib,
			Color:     &c,
			Width:     f64(width * 0.92),
			EdgeColor: &edge,
			EdgeWidth: f64(0.6),
		}); err != nil {
			return err
		}
	}

	if err := hline(ax, []float64{-0.6, float64(len(classes)) - 0.4}, 1.0, "FFTW3 (baseline)"); err != nil {
		return err
	}

	ax.SetXLim(-0.6, float64(len(classes))-0.4)
	ax.XAxis.Locator = ticker.FixedLocator{TicksList: centers}
	ax.XAxis.Formatter = ticker.FixedFormatter{Labels: classes}
	headroom(ax, append(allValues, 1.0), 1.28)
	_ = ax.TickParams(core.TickParams{Which: "major", LabelSize: f64(8.5)})
	ax.SetTitle("Non-power-of-two lengths: geometric mean speed vs FFTW3")
	ax.SetXLabel("length class")
	ax.SetYLabel("speedup vs FFTW3  (>1 is faster)")
	styleGrid(ax)
	leg := ax.AddLegend()
	leg.Location = core.LegendUpperCenter
	leg.NumColumns = 4
	leg.FontSize = 8.5
	caption(fig, run.Meta)

	return fig.Save(path)
}

// ---------------------------------------------------------------------------
// 04 — arbitrary lengths, one bar per size, coloured by class
// ---------------------------------------------------------------------------

func plotNonPow2Detail(run *Run, path string, dpi float64) error {
	fig, ax := newFigure(1400, 700, dpi)

	type entry struct {
		n       int
		class   string
		speedup float64
		algo    string
	}

	var entries []entry
	for _, class := range run.Classes("FFTAny") {
		for _, n := range run.SizesOfClass("FFTAny", class) {
			base, okBase := run.Lookup("FFTAny", "go-fftw", n)
			res, ok := run.Lookup("FFTAny", "algo-fft", n)
			if !ok || !okBase || res.NsOp <= 0 {
				continue
			}
			algo := ""
			if info, ok := run.Plan(n); ok {
				algo = info.Algorithm
			}
			entries = append(entries, entry{n, class, base.NsOp / res.NsOp, algo})
		}
	}
	if len(entries) == 0 {
		return fmt.Errorf("no FFTAny results")
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].class != entries[j].class {
			return classRank(entries[i].class) < classRank(entries[j].class)
		}
		return entries[i].n < entries[j].n
	})

	// One Bar call per class so the legend carries the class names.
	byClass := map[string][]int{}
	for i, e := range entries {
		byClass[e.class] = append(byClass[e.class], i)
	}

	labels := make([]string, len(entries))
	centers := make([]float64, len(entries))
	for i, e := range entries {
		labels[i] = fmt.Sprintf("%d", e.n)
		centers[i] = float64(i)
	}

	for _, class := range run.Classes("FFTAny") {
		idx := byClass[class]
		if len(idx) == 0 {
			continue
		}
		pos := make([]float64, len(idx))
		vals := make([]float64, len(idx))
		for k, i := range idx {
			pos[k] = centers[i]
			vals[k] = entries[i].speedup
		}
		c := classColor(class)
		edge := render.Color{R: 0.15, G: 0.15, B: 0.15, A: 1}
		if _, err := ax.Bar(pos, vals, core.BarOptions{
			Label:     class,
			Color:     &c,
			Width:     f64(0.72),
			EdgeColor: &edge,
			EdgeWidth: f64(0.5),
		}); err != nil {
			return err
		}
	}

	if err := hline(ax, []float64{-0.8, float64(len(entries)) - 0.2}, 1.0, "FFTW3 (baseline)"); err != nil {
		return err
	}

	ax.SetXLim(-0.8, float64(len(entries))-0.2)
	ax.XAxis.Locator = ticker.FixedLocator{TicksList: centers}
	ax.XAxis.Formatter = ticker.FixedFormatter{Labels: labels}

	speedups := make([]float64, 0, len(entries)+1)
	for _, e := range entries {
		speedups = append(speedups, e.speedup)
	}
	headroom(ax, append(speedups, 1.0), 1.42)
	_ = ax.TickParams(core.TickParams{Which: "major", LabelSize: f64(7.5)})
	ax.SetTitle("Every non-power-of-two length measured, algo-fft vs FFTW3")
	ax.SetXLabel("transform length n, grouped by class")
	ax.SetYLabel("speedup vs FFTW3  (>1 is faster)")
	styleGrid(ax)
	leg := ax.AddLegend()
	leg.Location = core.LegendUpperCenter
	leg.NumColumns = 6
	leg.FontSize = 8
	caption(fig, run.Meta)

	return fig.Save(path)
}

func classRank(class string) int {
	order := []string{"pow2", "5-smooth", "7/11-smooth", "prime (smooth p-1)", "prime (rough p-1)", "practical"}
	for i, c := range order {
		if c == class {
			return i
		}
	}
	return len(order)
}

// ---------------------------------------------------------------------------
// 05 — single vs double precision
// ---------------------------------------------------------------------------

func plotPrecision(run *Run, path string, dpi float64) error {
	fig, ax := newFigure(1100, 660, dpi)

	sizes := run.Sizes("FFT")
	if len(sizes) == 0 {
		return fmt.Errorf("no FFT results")
	}

	series := []struct {
		benchType string
		label     string
		color     render.Color
		marker    core.MarkerType
	}{
		{"FFT", "algo-fft, complex128", color.Tab10[0], core.MarkerCircle},
		{"FFT32", "algo-fft, complex64", color.Tab10[2], core.MarkerSquare},
	}

	for _, s := range series {
		var xs, ys []float64
		for _, n := range sizes {
			res, ok := run.Lookup(s.benchType, "algo-fft", n)
			if !ok {
				continue
			}
			xs = append(xs, float64(n))
			ys = append(ys, MFlops(n, res.NsOp))
		}
		if len(xs) == 0 {
			continue
		}
		c := s.color
		if _, err := ax.Plot(xs, ys, core.PlotOptions{
			Label:           s.label,
			Color:           &c,
			LineWidth:       f64(1.6),
			Marker:          marker(s.marker),
			MarkerSize:      f64(4.5),
			MarkerFaceColor: &c,
		}); err != nil {
			return err
		}
	}

	// FFTW's double-precision curve as the reference the reader already knows.
	var fx, fy []float64
	for _, n := range sizes {
		if res, ok := run.Lookup("FFT", "go-fftw", n); ok {
			fx = append(fx, float64(n))
			fy = append(fy, MFlops(n, res.NsOp))
		}
	}
	if len(fx) > 0 {
		c := libColor("go-fftw")
		if _, err := ax.Plot(fx, fy, core.PlotOptions{
			Label:     "FFTW3, complex128",
			Color:     &c,
			LineWidth: f64(1.2),
			LineStyle: core.LineStyleDashed,
		}); err != nil {
			return err
		}
	}

	if err := useLog2X(ax, float64(sizes[0])/1.4, float64(sizes[len(sizes)-1])*1.4); err != nil {
		return err
	}
	ax.SetTitle("Single vs double precision, forward FFT")
	ax.SetXLabel("transform length n")
	ax.SetYLabel("pseudo-MFLOPS  (5 n log₂ n / t)")
	styleGrid(ax)
	ax.AddLegend().Location = core.LegendLowerLeft
	caption(fig, run.Meta)

	return fig.Save(path)
}

// ---------------------------------------------------------------------------
// 06 — what the hand-written codelets actually buy
// ---------------------------------------------------------------------------

func plotSIMDvsPurego(simd, pure *Run, path string, dpi float64) error {
	fig, ax := newFigure(1100, 660, dpi)

	sizes := simd.Sizes("FFT")
	if len(sizes) == 0 {
		return fmt.Errorf("no FFT results")
	}

	series := []struct {
		benchType string
		label     string
		color     render.Color
		marker    core.MarkerType
	}{
		{"FFT", "complex128", color.Tab10[0], core.MarkerCircle},
		{"FFT32", "complex64", color.Tab10[2], core.MarkerSquare},
	}

	var axisXs []float64
	for _, n := range sizes {
		axisXs = append(axisXs, float64(n))
	}

	for _, s := range series {
		var xs, ys []float64
		for _, n := range sizes {
			fast, okFast := simd.Lookup(s.benchType, "algo-fft", n)
			slow, okSlow := pure.Lookup(s.benchType, "algo-fft", n)
			if !okFast || !okSlow || fast.NsOp <= 0 {
				continue
			}
			xs = append(xs, float64(n))
			ys = append(ys, slow.NsOp/fast.NsOp)
		}
		if len(xs) == 0 {
			continue
		}
		c := s.color
		if _, err := ax.Plot(xs, ys, core.PlotOptions{
			Label:           s.label,
			Color:           &c,
			LineWidth:       f64(1.6),
			Marker:          marker(s.marker),
			MarkerSize:      f64(4.5),
			MarkerFaceColor: &c,
		}); err != nil {
			return err
		}
	}

	if err := hline(ax, axisXs, 1.0, "pure Go (baseline)"); err != nil {
		return err
	}

	if err := useLog2X(ax, float64(sizes[0])/1.4, float64(sizes[len(sizes)-1])*1.4); err != nil {
		return err
	}
	ax.SetTitle("What the hand-written SIMD codelets buy over pure Go")
	ax.SetXLabel("transform length n")
	ax.SetYLabel("speedup of the default build over -tags purego")
	styleGrid(ax)
	ax.AddLegend().Location = core.LegendUpperLeft
	caption(fig, simd.Meta)

	return fig.Save(path)
}

// ---------------------------------------------------------------------------
// 07 — the pure-Go build against the other Go libraries
// ---------------------------------------------------------------------------

func plotPuregoVsCompetitors(simd, pure *Run, path string, dpi float64) error {
	fig, ax := newFigure(1100, 660, dpi)

	sizes := simd.Sizes("FFT")
	if len(sizes) == 0 {
		return fmt.Errorf("no FFT results")
	}

	type series struct {
		run   *Run
		lib   string
		label string
		c     render.Color
		m     core.MarkerType
		dash  bool
	}

	all := []series{
		{simd, "algo-fft", "algo-fft (SIMD)", color.Tab10[0], core.MarkerCircle, false},
		{pure, "algo-fft", "algo-fft (purego)", color.Tab10[0], core.MarkerCircle, true},
		{simd, "gonum", "gonum", color.Tab10[2], core.MarkerTriangleUp, false},
		{simd, "go-dsp-fft", "go-dsp-fft", color.Tab10[3], core.MarkerDiamond, false},
		{simd, "takatoh", "takatoh", color.Tab10[4], core.MarkerCross, false},
	}

	for _, s := range all {
		var xs, ys []float64
		for _, n := range sizes {
			res, ok := s.run.Lookup("FFT", s.lib, n)
			if !ok {
				continue
			}
			xs = append(xs, float64(n))
			ys = append(ys, MFlops(n, res.NsOp))
		}
		if len(xs) == 0 {
			continue
		}
		c := s.c
		opts := core.PlotOptions{
			Label:           s.label,
			Color:           &c,
			LineWidth:       f64(1.6),
			Marker:          marker(s.m),
			MarkerSize:      f64(4.5),
			MarkerFaceColor: &c,
		}
		if s.dash {
			opts.LineStyle = core.LineStyleDashed
			opts.MarkerFaceColor = nil
		}
		if _, err := ax.Plot(xs, ys, opts); err != nil {
			return err
		}
	}

	if err := useLog2X(ax, float64(sizes[0])/1.4, float64(sizes[len(sizes)-1])*1.4); err != nil {
		return err
	}
	ax.SetTitle("Pure Go against the other Go FFT libraries (and against itself with SIMD)")
	ax.SetXLabel("transform length n")
	ax.SetYLabel("pseudo-MFLOPS  (5 n log₂ n / t)")
	styleGrid(ax)
	ax.AddLegend().Location = core.LegendLowerLeft
	caption(fig, simd.Meta)

	return fig.Save(path)
}

var _ = math.Log2
