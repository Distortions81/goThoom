package main

import (
	"fmt"
	"image/color"
	"math"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"gothoom/eui"
)

const (
	statsSampleInterval  = 500 * time.Millisecond
	statsHistoryDuration = 5 * time.Minute
	statsHistorySize     = int(statsHistoryDuration / statsSampleInterval)
	statsGraphHeight     = 62
	statsScaleWidth      = 110
)

type statsGraphScale struct {
	minimumMaximum float64
	unit           string
}

type liveStatsSample struct {
	fps, updateRate float64
	reply, jitter   time.Duration
	cacheMemory     float64
	gpuMemory       float64
}

type statsMetric struct {
	value *eui.ItemData
}

type statsLegendEntry struct {
	label string
	color color.Color
}

var (
	statsReplyColor       = color.RGBA{R: 74, G: 211, B: 235, A: 255}
	statsJitterColor      = color.RGBA{R: 244, G: 196, B: 74, A: 255}
	statsFPSColor         = color.RGBA{R: 73, G: 210, B: 111, A: 255}
	statsUpdateColor      = color.RGBA{R: 185, G: 118, B: 235, A: 255}
	statsCacheMemoryColor = color.RGBA{R: 62, G: 201, B: 181, A: 255}
	statsGPUMemoryColor   = color.RGBA{R: 85, G: 151, B: 235, A: 255}
)

var (
	statsWin               *eui.WindowData
	statsReplyMetric       statsMetric
	statsJitterMetric      statsMetric
	statsRecentLoss        statsMetric
	statsSessionLoss       statsMetric
	statsPNACheckbox       *eui.ItemData
	advancedPNACheckbox    *eui.ItemData
	statsNetworkText       *eui.ItemData
	statsPNAAlert          *eui.ItemData
	statsFPSMetric         statsMetric
	statsUpdateMetric      statsMetric
	statsCPUMetric         statsMetric
	statsRateText          *eui.ItemData
	statsArtwork           statsMetric
	statsSounds            statsMetric
	statsCacheTotal        statsMetric
	statsGPUMemory         statsMetric
	statsMemoryText        *eui.ItemData
	statsNetworkGraph      *eui.ItemData
	statsRateGraph         *eui.ItemData
	statsCacheGraph        *eui.ItemData
	statsNetworkUpperScale *eui.ItemData
	statsNetworkLowerScale *eui.ItemData
	statsRateUpperScale    *eui.ItemData
	statsRateLowerScale    *eui.ItemData
	statsCacheUpperScale   *eui.ItemData
	statsCacheLowerScale   *eui.ItemData
	statsNetworkImage      *ebiten.Image
	statsRateImage         *ebiten.Image
	statsCacheImage        *ebiten.Image
	statsHistory           [statsHistorySize]liveStatsSample
	statsHistoryCount      int
	statsHistoryNext       int
	lastStatsSample        time.Time
	lastStatsRender        time.Time
	gameWorkMu             sync.Mutex
	gameWorkBuckets        [5]time.Duration
	gameWorkTimes          [5]int64
)

// recordGameLoopWork tracks time spent in this client's Update and Draw
// methods. It is a portable approximation of client CPU use; it deliberately
// excludes work done by unrelated processes and background worker goroutines.
func recordGameLoopWork(duration time.Duration) {
	if duration <= 0 {
		return
	}
	gameWorkMu.Lock()
	defer gameWorkMu.Unlock()
	now := time.Now().Unix()
	idx := int(now % int64(len(gameWorkBuckets)))
	if gameWorkTimes[idx] != now {
		gameWorkTimes[idx] = now
		gameWorkBuckets[idx] = 0
	}
	gameWorkBuckets[idx] += duration
}

func gameLoopCPULoad() float64 {
	gameWorkMu.Lock()
	defer gameWorkMu.Unlock()
	now := time.Now().Unix()
	var work time.Duration
	for i := range gameWorkBuckets {
		if now-gameWorkTimes[i] < 5 {
			work += gameWorkBuckets[i]
		}
	}
	return float64(work) * 100 / float64(5*time.Second)
}

func newStatsMetric(label string, width float32) (*eui.ItemData, statsMetric) {
	metric := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	metric.Size = eui.Point{X: width, Y: 42}

	labelText, _ := eui.NewText()
	labelText.Text = label
	labelText.FontSize = 9
	labelText.Size = eui.Point{X: width, Y: 17}
	labelText.TextColor = eui.AccentColor()
	labelText.ForceTextColor = true
	metric.AddItem(labelText)

	valueText, _ := eui.NewText()
	valueText.Text = "--"
	valueText.FontSize = 13
	valueText.Size = eui.Point{X: width, Y: 25}
	applyBoldFace(valueText)
	metric.AddItem(valueText)

	return metric, statsMetric{value: valueText}
}

func newStatsMetricRow(width float32, labels ...string) (*eui.ItemData, []statsMetric) {
	row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	row.Size = eui.Point{X: width, Y: 42}
	metrics := make([]statsMetric, 0, len(labels))
	metricWidth := width / float32(len(labels))
	for _, label := range labels {
		item, metric := newStatsMetric(label, metricWidth)
		row.AddItem(item)
		metrics = append(metrics, metric)
	}
	return row, metrics
}

func newStatsDetail(width, height, fontSize float32) *eui.ItemData {
	detail, _ := eui.NewText()
	detail.FontSize = fontSize
	detail.Size = eui.Point{X: width, Y: height}
	return detail
}

func newStatsLegend(width float32, entries ...statsLegendEntry) *eui.ItemData {
	const (
		height      float32 = 18
		swatchWidth         = 18
	)
	legend := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	legend.Size = eui.Point{X: width, Y: height}
	entryWidth := width / float32(len(entries))
	for _, spec := range entries {
		entry := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		entry.Size = eui.Point{X: entryWidth, Y: height}
		swatch, swatchImage := eui.NewImageItem(int(swatchWidth), int(height))
		vector.FillRect(swatchImage, 2, 4, 10, 10, spec.color, true)
		entry.AddItem(swatch)
		label := newStatsDetail(entryWidth-swatchWidth, height, 9)
		label.Text = spec.label
		entry.AddItem(label)
		legend.AddItem(entry)
	}
	return legend
}

func newStatsGraphHeader(width float32, entries ...statsLegendEntry) (*eui.ItemData, *eui.ItemData) {
	header := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	header.Size = eui.Point{X: width, Y: 18}
	header.AddItem(newStatsLegend(width-statsScaleWidth, entries...))
	scaleSlot, scale := newStatsScaleSlot(entries[0].color)
	header.AddItem(scaleSlot)
	return header, scale
}

func newStatsGraphFooter(width float32, tint color.Color) (*eui.ItemData, *eui.ItemData) {
	footer := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	footer.Size = eui.Point{X: width, Y: 18}
	history := newStatsDetail(width-statsScaleWidth, 18, 8)
	history.Text = fmt.Sprintf("%.0fm ago  →  now", statsHistoryDuration.Minutes())
	footer.AddItem(history)
	scaleSlot, scale := newStatsScaleSlot(tint)
	footer.AddItem(scaleSlot)
	return footer, scale
}

func newStatsScaleSlot(tint color.Color) (*eui.ItemData, *eui.ItemData) {
	slot := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	slot.Size = eui.Point{X: statsScaleWidth, Y: 18}
	scale := newStatsDetail(statsScaleWidth, 18, 8)
	scale.Position = eui.Point{}
	scale.Text = "Scale --"
	r, g, b, a := tint.RGBA()
	scale.TextColor = eui.NewColor(uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
	scale.ForceTextColor = true
	slot.AddItem(scale)
	return slot, scale
}

func setStatsMetric(metric statsMetric, value string) {
	if metric.value == nil {
		return
	}
	metric.value.Text = value
	metric.value.Dirty = true
}

func setPNAEnabled(enabled bool) {
	changed := gs.AltNetMode != enabled
	gs.AltNetMode = enabled
	if changed {
		resetPNAController()
		resetPNAFallback()
	}
	for _, checkbox := range []*eui.ItemData{statsPNACheckbox, advancedPNACheckbox} {
		if checkbox != nil && checkbox.Checked != enabled {
			checkbox.Checked = enabled
			checkbox.Dirty = true
		}
	}
	settingsDirty = true
}

func clearStatsHistory() {
	statsHistory = [statsHistorySize]liveStatsSample{}
	statsHistoryCount = 0
	statsHistoryNext = 0
	lastStatsSample = time.Time{}
	lastStatsRender = time.Time{}
}

func makeStatsWindow() {
	if statsWin != nil {
		return
	}
	const width float32 = 460
	statsWin = eui.NewWindow()
	statsWin.Title = "Live Stats"
	statsWin.Closable = true
	statsWin.Resizable = false
	statsWin.AutoSize = true
	statsWin.Movable = true

	flow := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL}

	networkSection := newConfigurationSection("Network", width)
	networkControls := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	networkControls.Size = eui.Point{X: width, Y: 24}
	statsPNACheckbox, pnaEvents := eui.NewCheckbox()
	statsPNACheckbox.Text = "Enable NLSPT"
	statsPNACheckbox.Size = eui.Point{X: width - 92, Y: 24}
	statsPNACheckbox.Checked = gs.AltNetMode
	statsPNACheckbox.SetTooltip("Network Latency & Server Phase Timing learns the server frame phase and sends fresh input shortly before its next processing window. Command replies tune the lead; packet loss pauses NLSPT.")
	pnaEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			setPNAEnabled(ev.Checked)
		}
	}
	networkControls.AddItem(statsPNACheckbox)
	clearButton, clearEvents := eui.NewButton()
	clearButton.Text = "Clear Stats"
	clearButton.Size = eui.Point{X: 92, Y: 24}
	clearButton.FontSize = 10
	clearButton.SetTooltip("Clear the five-minute graph history. Session packet totals and NLSPT measurements are unchanged.")
	clearEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			clearStatsHistory()
			updateStatsWindow(time.Now())
		}
	}
	networkControls.AddItem(clearButton)
	networkSection.AddItem(networkControls)
	networkMetrics, metrics := newStatsMetricRow(width, "CMD REPLY", "FRAME JITTER P95", "RECENT LOSS", "SESSION LOSS")
	statsReplyMetric, statsJitterMetric, statsRecentLoss, statsSessionLoss = metrics[0], metrics[1], metrics[2], metrics[3]
	networkSection.AddItem(networkMetrics)
	statsNetworkText = newStatsDetail(width, 54, 10)
	statsNetworkText.FontSize = 10
	networkSection.AddItem(statsNetworkText)
	statsPNAAlert = newStatsDetail(width, 22, 10)
	statsPNAAlert.Padding = 4
	statsPNAAlert.ForceTextColor = true
	networkSection.AddItem(statsPNAAlert)
	networkGraphHeader, networkScale := newStatsGraphHeader(width,
		statsLegendEntry{label: "Reply", color: statsReplyColor},
		statsLegendEntry{label: "Jitter p95", color: statsJitterColor},
	)
	statsNetworkUpperScale = networkScale
	networkSection.AddItem(networkGraphHeader)
	statsNetworkGraph, statsNetworkImage = eui.NewImageItem(int(width), statsGraphHeight)
	networkSection.AddItem(statsNetworkGraph)
	networkGraphFooter, networkLowerScale := newStatsGraphFooter(width, statsJitterColor)
	statsNetworkLowerScale = networkLowerScale
	networkSection.AddItem(networkGraphFooter)
	flow.AddItem(networkSection)

	rateSection := newConfigurationSection("Frame timing", width)
	rateMetrics, metrics := newStatsMetricRow(width, "CLIENT FPS", "SERVER RATE", "GAME-LOOP CPU")
	statsFPSMetric, statsUpdateMetric, statsCPUMetric = metrics[0], metrics[1], metrics[2]
	rateSection.AddItem(rateMetrics)
	statsRateText = newStatsDetail(width, 20, 10)
	rateSection.AddItem(statsRateText)
	rateGraphHeader, rateScale := newStatsGraphHeader(width,
		statsLegendEntry{label: "FPS", color: statsFPSColor},
		statsLegendEntry{label: "Server rate", color: statsUpdateColor},
	)
	statsRateUpperScale = rateScale
	rateSection.AddItem(rateGraphHeader)
	statsRateGraph, statsRateImage = eui.NewImageItem(int(width), statsGraphHeight)
	rateSection.AddItem(statsRateGraph)
	rateGraphFooter, rateLowerScale := newStatsGraphFooter(width, statsUpdateColor)
	statsRateLowerScale = rateLowerScale
	rateSection.AddItem(rateGraphFooter)
	flow.AddItem(rateSection)

	cacheSection := newConfigurationSection("Memory Use", width)
	cacheMetrics, metrics := newStatsMetricRow(width, "ARTWORK CACHE", "SOUND CACHE", "TOTAL CACHE", "GPU TEXTURES")
	statsArtwork, statsSounds, statsCacheTotal, statsGPUMemory = metrics[0], metrics[1], metrics[2], metrics[3]
	cacheSection.AddItem(cacheMetrics)
	memoryControls := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
	memoryControls.Size = eui.Point{X: width, Y: 24}
	statsMemoryText = newStatsDetail(width-104, 24, 10)
	memoryControls.AddItem(statsMemoryText)
	clearCachesButton, clearCachesEvents := eui.NewButton()
	clearCachesButton.Text = "Clear Caches"
	clearCachesButton.Size = eui.Point{X: 104, Y: 24}
	clearCachesButton.FontSize = 10
	clearCachesButton.SetTooltip("Clear decoded artwork and sound caches. Enabled precaching may begin filling them again immediately.")
	clearCachesEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			clearCaches()
			lastStatsSample = time.Time{}
			lastStatsRender = time.Time{}
			updateStatsWindow(time.Now())
		}
	}
	memoryControls.AddItem(clearCachesButton)
	cacheSection.AddItem(memoryControls)
	cacheGraphHeader, cacheScale := newStatsGraphHeader(width,
		statsLegendEntry{label: "Cache", color: statsCacheMemoryColor},
		statsLegendEntry{label: "GPU", color: statsGPUMemoryColor},
	)
	statsCacheUpperScale = cacheScale
	cacheSection.AddItem(cacheGraphHeader)
	statsCacheGraph, statsCacheImage = eui.NewImageItem(int(width), statsGraphHeight)
	cacheSection.AddItem(statsCacheGraph)
	cacheGraphFooter, cacheLowerScale := newStatsGraphFooter(width, statsGPUMemoryColor)
	statsCacheLowerScale = cacheLowerScale
	cacheSection.AddItem(cacheGraphFooter)
	flow.AddItem(cacheSection)

	statsWin.AddItem(flow)
	statsWin.AddWindow(false)
}

func appendStatsSample(sample liveStatsSample) {
	statsHistory[statsHistoryNext] = sample
	statsHistoryNext = (statsHistoryNext + 1) % len(statsHistory)
	if statsHistoryCount < len(statsHistory) {
		statsHistoryCount++
	}
}

func statsSamples() []liveStatsSample {
	samples := make([]liveStatsSample, statsHistoryCount)
	start := (statsHistoryNext - statsHistoryCount + len(statsHistory)) % len(statsHistory)
	for i := range samples {
		samples[i] = statsHistory[(start+i)%len(statsHistory)]
	}
	return samples
}

func drawStatsGraph(dst *ebiten.Image, upper, lower []float64, upperColor, lowerColor color.Color, upperScale, lowerScale statsGraphScale) (float64, float64) {
	upperMaximum := statsGraphMaximum(upper, upperScale.minimumMaximum)
	lowerMaximum := statsGraphMaximum(lower, lowerScale.minimumMaximum)
	if dst == nil {
		return upperMaximum, lowerMaximum
	}
	dst.Fill(color.RGBA{R: 19, G: 23, B: 26, A: 255})
	bounds := dst.Bounds()
	w, h := float32(bounds.Dx()), float32(bounds.Dy())
	const (
		outerPadding float32 = 4
		panelGap     float32 = 6
	)
	left, right := outerPadding, w-outerPadding
	plotTop, plotBottom := outerPadding, h-outerPadding
	center := (plotTop + plotBottom) / 2
	upperTop, upperBottom := plotTop, center-panelGap/2
	lowerTop, lowerBottom := center+panelGap/2, plotBottom

	drawStatsSeries(dst, upper, left, upperTop, right, upperBottom, upperMaximum, statsHistorySize, upperColor)
	drawStatsSeries(dst, lower, left, lowerTop, right, lowerBottom, lowerMaximum, statsHistorySize, lowerColor)
	return upperMaximum, lowerMaximum
}

func statsGraphMaximum(values []float64, minimumMaximum float64) float64 {
	peak := 0.0
	for _, value := range values {
		if !math.IsNaN(value) && !math.IsInf(value, 0) && value > peak {
			peak = value
		}
	}
	if minimumMaximum > 0 && peak <= minimumMaximum {
		return minimumMaximum
	}
	if peak <= 0 {
		return 1
	}

	// Aim for three readable intervals and select a nearby 1/2/2.5/5/10 step.
	rawStep := peak / 3
	magnitude := math.Pow(10, math.Floor(math.Log10(rawStep)))
	fraction := rawStep / magnitude
	niceFraction := 1.0
	bestDistance := math.Inf(1)
	for _, candidate := range []float64{1, 2, 2.5, 5, 10} {
		if distance := math.Abs(candidate - fraction); distance < bestDistance {
			niceFraction = candidate
			bestDistance = distance
		}
	}
	step := niceFraction * magnitude
	return math.Ceil(peak/step) * step
}

func formatStatsScaleValue(value float64, unit string) string {
	switch unit {
	case "ms":
		return fmt.Sprintf("%.0fms", value)
	case "fps":
		return fmt.Sprintf("%.0ffps", value)
	case "/s":
		if value < 10 && value != math.Trunc(value) {
			return fmt.Sprintf("%.1f/s", value)
		}
		return fmt.Sprintf("%.0f/s", value)
	case "MiB":
		if value >= 1024 {
			return fmt.Sprintf("%.1fGiB", value/1024)
		}
		if value < 10 && value != math.Trunc(value) {
			return fmt.Sprintf("%.1fMiB", value)
		}
		return fmt.Sprintf("%.0fMiB", value)
	default:
		return fmt.Sprintf("%.1f", value)
	}
}

func statsGraphScaleText(maximum float64, unit string) string {
	return "Scale " + formatStatsScaleValue(maximum, unit)
}

func setStatsGraphScale(item *eui.ItemData, maximum float64, unit string) {
	if item == nil {
		return
	}
	item.Text = statsGraphScaleText(maximum, unit)
	item.Position.X = 0
	item.Size.X = statsScaleWidth
	if source := eui.FontSource(); source != nil && item.Parent != nil {
		uiScale := eui.UIScale()
		if uiScale <= 0 {
			uiScale = 1
		}
		face := &text.GoTextFace{Source: source, Size: float64(item.FontSize*uiScale + 2)}
		width, _ := text.Measure(item.Text, face, 0)
		logicalWidth := float32(math.Ceil(width)) / uiScale
		if logicalWidth < item.Parent.Size.X {
			item.Size.X = logicalWidth
			item.Position.X = item.Parent.Size.X - logicalWidth
		}
	}
	item.Dirty = true
}

func drawStatsSeries(dst *ebiten.Image, values []float64, left, top, right, bottom float32, maximum float64, capacity int, lineColor color.Color) {
	if dst == nil || len(values) == 0 || right <= left || bottom <= top || maximum <= 0 || capacity < 2 {
		return
	}
	if len(values) > capacity {
		values = values[len(values)-capacity:]
	}
	valueY := func(value float64) float32 {
		if value < 0 {
			value = 0
		} else if value > maximum {
			value = maximum
		}
		return bottom - float32(value/maximum)*(bottom-top)
	}
	step := (right - left) / float32(capacity-1)
	startX := right - float32(len(values)-1)*step
	if len(values) == 1 {
		vector.FillCircle(dst, startX, valueY(values[0]), 1.5, lineColor, true)
		return
	}
	previousX, previousY := startX, valueY(values[0])
	for i, value := range values[1:] {
		x := startX + float32(i+1)*step
		y := valueY(value)
		vector.StrokeLine(dst, previousX, previousY, x, y, 1.5, lineColor, true)
		previousX, previousY = x, y
	}
}

func updateStatsWindow(now time.Time) {
	sampleDue := lastStatsSample.IsZero() || now.Sub(lastStatsSample) >= statsSampleInterval
	windowOpen := statsWin != nil && statsWin.IsOpen()
	renderDue := windowOpen && (lastStatsRender.IsZero() || now.Sub(lastStatsRender) >= statsSampleInterval)
	if !sampleDue && !renderDue {
		return
	}
	frameMu.Lock()
	interval := frameInterval
	updatesPerSecond := serverUpdatesPerSecond
	frameMu.Unlock()
	if interval <= 0 {
		interval = framems * time.Millisecond
	}
	reply, jitter := networkTimingSnapshot()
	images := imageCacheStats()
	sounds, soundBytes := soundCacheStats()
	imageCount := images.sheetCount + images.frameCount + images.scaledFrameCount + images.mobileCount + images.scaledMobileCount
	imageBytes := images.sheetBytes + images.frameBytes + images.scaledFrameBytes + images.mobileBytes + images.scaledMobileBytes
	gpuTextures := imageBytes + ebitenImageBytes(gameImageBacking) + ebitenImageBytes(backgroundImg) + ebitenImageBytes(splashImg) + ebitenImageBytes(toolbarHandsImage) + ebitenImageBytes(toolbarLeftComposite) + ebitenImageBytes(toolbarRightComposite)
	if sampleDue {
		appendStatsSample(liveStatsSample{
			fps:         ebiten.ActualFPS(),
			updateRate:  updatesPerSecond,
			reply:       reply,
			jitter:      jitter,
			cacheMemory: float64(imageBytes+soundBytes) / (1024 * 1024),
			gpuMemory:   float64(gpuTextures) / (1024 * 1024),
		})
		lastStatsSample = now
	}
	if !renderDue {
		return
	}

	samples := statsSamples()
	fps := make([]float64, len(samples))
	updates := make([]float64, len(samples))
	replies := make([]float64, len(samples))
	jitters := make([]float64, len(samples))
	cacheMemory := make([]float64, len(samples))
	gpuMemory := make([]float64, len(samples))
	for i, sample := range samples {
		fps[i] = sample.fps
		updates[i] = sample.updateRate
		replies[i] = float64(sample.reply) / float64(time.Millisecond)
		jitters[i] = float64(sample.jitter) / float64(time.Millisecond)
		cacheMemory[i] = sample.cacheMemory
		gpuMemory[i] = sample.gpuMemory
	}

	recentLoss, sessionLoss, received, lost := packetLossSnapshot()
	usePNA := true
	fallbackReason := ""
	if gs.AltNetMode {
		usePNA, fallbackReason = pnaTimingStatus(recentLoss, now)
	}
	var phase, lead time.Duration
	var timingReady bool
	if gs.AltNetMode && usePNA {
		_, _, _, phase, lead, timingReady = pnaScheduleSnapshot()
	}
	safety := networkAdjustmentSafetyMargin(interval)
	if statsPNACheckbox != nil && statsPNACheckbox.Checked != gs.AltNetMode {
		statsPNACheckbox.Checked = gs.AltNetMode
		statsPNACheckbox.Dirty = true
	}
	setStatsMetric(statsReplyMetric, formatToolbarLatency(reply))
	setStatsMetric(statsJitterMetric, formatToolbarLatency(jitter))
	setStatsMetric(statsRecentLoss, fmt.Sprintf("%.1f%%", recentLoss))
	setStatsMetric(statsSessionLoss, fmt.Sprintf("%.2f%%", sessionLoss))
	if statsNetworkText != nil {
		mode := "OFF"
		timing := "Original timing"
		switch {
		case gs.AltNetMode && !usePNA:
			mode = "PAUSED"
		case gs.AltNetMode && !timingReady:
			mode = "LEARNING"
			timing = "Original timing while learning"
		case gs.AltNetMode:
			mode = "ON"
			timing = fmt.Sprintf("Send phase %s   |   Lead %s", formatToolbarLatency(phase), formatToolbarLatency(lead))
		}
		serverTiming := "learning"
		if updatesPerSecond > 0 {
			serverTiming = fmt.Sprintf("%.2f/sec (%s)", updatesPerSecond, formatToolbarLatency(interval))
		}
		statsNetworkText.Text = fmt.Sprintf("NLSPT %s   |   Server %s\n%s   |   Safety %d%% (%s)\nPackets: %d received, %d lost",
			mode, serverTiming, timing, networkAdjustmentSafetyPercent.Load(), formatToolbarLatency(safety), received, lost)
		statsNetworkText.Dirty = true
	}
	if statsPNAAlert != nil {
		statsPNAAlert.Filled = gs.AltNetMode && !usePNA
		if statsPNAAlert.Filled {
			statsPNAAlert.Color = eui.NewColor(154, 36, 36, 255)
			statsPNAAlert.TextColor = eui.NewColor(255, 255, 255, 255)
			statsPNAAlert.Text = "NLSPT PAUSED — " + pnaFallbackExplanation(fallbackReason, recentLoss)
		} else {
			statsPNAAlert.Text = ""
		}
		statsPNAAlert.Dirty = true
	}
	networkUpperScale := statsGraphScale{minimumMaximum: 250, unit: "ms"}
	networkLowerScale := statsGraphScale{minimumMaximum: 32, unit: "ms"}
	networkUpperMaximum, networkLowerMaximum := drawStatsGraph(statsNetworkImage, replies, jitters, statsReplyColor, statsJitterColor,
		networkUpperScale, networkLowerScale)
	setStatsGraphScale(statsNetworkUpperScale, networkUpperMaximum, networkUpperScale.unit)
	setStatsGraphScale(statsNetworkLowerScale, networkLowerMaximum, networkLowerScale.unit)
	if statsNetworkGraph != nil {
		statsNetworkGraph.Dirty = true
	}
	setStatsMetric(statsFPSMetric, fmt.Sprintf("%.1f", ebiten.ActualFPS()))
	setStatsMetric(statsUpdateMetric, fmt.Sprintf("%.2f / sec", updatesPerSecond))
	setStatsMetric(statsCPUMetric, fmt.Sprintf("~%.1f%%", gameLoopCPULoad()))
	if statsRateText != nil {
		statsRateText.Text = fmt.Sprintf("Server frame interval: %s", interval.Round(time.Millisecond))
		statsRateText.Dirty = true
	}
	rateUpperScale := statsGraphScale{minimumMaximum: 65, unit: "fps"}
	rateLowerScale := statsGraphScale{minimumMaximum: 10, unit: "/s"}
	rateUpperMaximum, rateLowerMaximum := drawStatsGraph(statsRateImage, fps, updates, statsFPSColor, statsUpdateColor,
		rateUpperScale, rateLowerScale)
	setStatsGraphScale(statsRateUpperScale, rateUpperMaximum, rateUpperScale.unit)
	setStatsGraphScale(statsRateLowerScale, rateLowerMaximum, rateLowerScale.unit)
	if statsRateGraph != nil {
		statsRateGraph.Dirty = true
	}
	setStatsMetric(statsArtwork, humanize.Bytes(uint64(imageBytes)))
	setStatsMetric(statsSounds, humanize.Bytes(uint64(soundBytes)))
	setStatsMetric(statsCacheTotal, humanize.Bytes(uint64(imageBytes+soundBytes)))
	setStatsMetric(statsGPUMemory, "~"+humanize.Bytes(uint64(gpuTextures)))
	if statsMemoryText != nil {
		statsMemoryText.Text = fmt.Sprintf("Cache entries: %d artwork, %d sounds", imageCount, sounds)
		statsMemoryText.Dirty = true
	}
	cacheUpperScale := statsGraphScale{minimumMaximum: 2048, unit: "MiB"}
	cacheLowerScale := statsGraphScale{minimumMaximum: 2048, unit: "MiB"}
	cacheUpperMaximum, cacheLowerMaximum := drawStatsGraph(statsCacheImage, cacheMemory, gpuMemory, statsCacheMemoryColor, statsGPUMemoryColor,
		cacheUpperScale, cacheLowerScale)
	setStatsGraphScale(statsCacheUpperScale, cacheUpperMaximum, cacheUpperScale.unit)
	setStatsGraphScale(statsCacheLowerScale, cacheLowerMaximum, cacheLowerScale.unit)
	if statsCacheGraph != nil {
		statsCacheGraph.Dirty = true
	}
	statsWin.Refresh()
	lastStatsRender = now
}

func ebitenImageBytes(img *ebiten.Image) int {
	if img == nil {
		return 0
	}
	bounds := img.Bounds()
	return bounds.Dx() * bounds.Dy() * 4
}
