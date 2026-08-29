package main

import (
	"fmt"
	"image/color"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"gothoom/eui"
)

const (
	statsSampleInterval  = 500 * time.Millisecond
	statsHistoryDuration = 5 * time.Minute
	statsHistorySize     = int(statsHistoryDuration / statsSampleInterval)
)

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
	statsWin          *eui.WindowData
	statsReplyMetric  statsMetric
	statsJitterMetric statsMetric
	statsRecentLoss   statsMetric
	statsSessionLoss  statsMetric
	statsNetworkText  *eui.ItemData
	statsFPSMetric    statsMetric
	statsUpdateMetric statsMetric
	statsCPUMetric    statsMetric
	statsRateText     *eui.ItemData
	statsArtwork      statsMetric
	statsSounds       statsMetric
	statsCacheTotal   statsMetric
	statsGPUMemory    statsMetric
	statsMemoryText   *eui.ItemData
	statsNetworkGraph *eui.ItemData
	statsRateGraph    *eui.ItemData
	statsCacheGraph   *eui.ItemData
	statsNetworkImage *ebiten.Image
	statsRateImage    *ebiten.Image
	statsCacheImage   *ebiten.Image
	statsHistory      [statsHistorySize]liveStatsSample
	statsHistoryCount int
	statsHistoryNext  int
	lastStatsSample   time.Time
	lastStatsRender   time.Time
	gameWorkMu        sync.Mutex
	gameWorkBuckets   [5]time.Duration
	gameWorkTimes     [5]int64
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

func setStatsMetric(metric statsMetric, value string) {
	if metric.value == nil {
		return
	}
	metric.value.Text = value
	metric.value.Dirty = true
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
	networkMetrics, metrics := newStatsMetricRow(width, "REPLY TIME", "P99 JITTER", "RECENT LOSS", "SESSION LOSS")
	statsReplyMetric, statsJitterMetric, statsRecentLoss, statsSessionLoss = metrics[0], metrics[1], metrics[2], metrics[3]
	networkSection.AddItem(networkMetrics)
	statsNetworkText = newStatsDetail(width, 38, 10)
	statsNetworkText.FontSize = 10
	networkSection.AddItem(statsNetworkText)
	networkSection.AddItem(newStatsLegend(width,
		statsLegendEntry{label: "Reply time", color: statsReplyColor},
		statsLegendEntry{label: "P99 jitter", color: statsJitterColor},
	))
	statsNetworkGraph, statsNetworkImage = eui.NewImageItem(int(width), 76)
	networkSection.AddItem(statsNetworkGraph)
	flow.AddItem(networkSection)

	rateSection := newConfigurationSection("Frame timing", width)
	rateMetrics, metrics := newStatsMetricRow(width, "CLIENT FPS", "SERVER RATE", "GAME-LOOP CPU")
	statsFPSMetric, statsUpdateMetric, statsCPUMetric = metrics[0], metrics[1], metrics[2]
	rateSection.AddItem(rateMetrics)
	statsRateText = newStatsDetail(width, 20, 10)
	rateSection.AddItem(statsRateText)
	rateSection.AddItem(newStatsLegend(width,
		statsLegendEntry{label: "Client FPS", color: statsFPSColor},
		statsLegendEntry{label: "Server updates", color: statsUpdateColor},
	))
	statsRateGraph, statsRateImage = eui.NewImageItem(int(width), 76)
	rateSection.AddItem(statsRateGraph)
	flow.AddItem(rateSection)

	cacheSection := newConfigurationSection("Memory Use", width)
	cacheMetrics, metrics := newStatsMetricRow(width, "ARTWORK CACHE", "SOUND CACHE", "TOTAL CACHE", "GPU TEXTURES")
	statsArtwork, statsSounds, statsCacheTotal, statsGPUMemory = metrics[0], metrics[1], metrics[2], metrics[3]
	cacheSection.AddItem(cacheMetrics)
	statsMemoryText = newStatsDetail(width, 20, 10)
	cacheSection.AddItem(statsMemoryText)
	cacheSection.AddItem(newStatsLegend(width,
		statsLegendEntry{label: "Total cache memory", color: statsCacheMemoryColor},
		statsLegendEntry{label: "GPU textures", color: statsGPUMemoryColor},
	))
	statsCacheGraph, statsCacheImage = eui.NewImageItem(int(width), 76)
	cacheSection.AddItem(statsCacheGraph)
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

func drawStatsGraph(dst *ebiten.Image, upper, lower []float64, upperColor, lowerColor color.Color) {
	if dst == nil {
		return
	}
	dst.Fill(color.RGBA{R: 19, G: 23, B: 26, A: 255})
	bounds := dst.Bounds()
	w, h := float32(bounds.Dx()), float32(bounds.Dy())
	grid := color.RGBA{R: 74, G: 82, B: 88, A: 150}
	vector.StrokeLine(dst, 4, h/2, w-4, h/2, 1, grid, false)
	vector.StrokeLine(dst, 4, h-3, w-4, h-3, 1, grid, false)
	drawStatsSeries(dst, upper, 4, 4, w-4, h/2-4, upperColor)
	drawStatsSeries(dst, lower, 4, h/2+4, w-4, h-4, lowerColor)
}

func drawStatsSeries(dst *ebiten.Image, values []float64, left, top, right, bottom float32, lineColor color.Color) {
	if dst == nil || len(values) == 0 || right <= left || bottom <= top {
		return
	}
	minValue, maxValue := values[0], values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	valueY := func(value float64) float32 {
		if maxValue == minValue {
			return (top + bottom) / 2
		}
		return bottom - float32((value-minValue)/(maxValue-minValue))*(bottom-top)
	}
	if len(values) == 1 {
		vector.FillCircle(dst, right, valueY(values[0]), 1.5, lineColor, true)
		return
	}
	step := (right - left) / float32(len(values)-1)
	previousX, previousY := left, valueY(values[0])
	for i, value := range values[1:] {
		x := left + float32(i+1)*step
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
	frameMu.Unlock()
	if interval <= 0 {
		interval = framems * time.Millisecond
	}
	reply, jitter := networkLatencySnapshot()
	images := imageCacheStats()
	sounds, soundBytes := soundCacheStats()
	imageCount := images.sheetCount + images.frameCount + images.scaledFrameCount + images.mobileCount + images.scaledMobileCount
	imageBytes := images.sheetBytes + images.frameBytes + images.scaledFrameBytes + images.mobileBytes + images.scaledMobileBytes
	gpuTextures := imageBytes + ebitenImageBytes(gameImageBacking) + ebitenImageBytes(backgroundImg) + ebitenImageBytes(splashImg) + ebitenImageBytes(toolbarHandsImage) + ebitenImageBytes(toolbarLeftComposite) + ebitenImageBytes(toolbarRightComposite)
	if sampleDue {
		appendStatsSample(liveStatsSample{
			fps:         ebiten.ActualFPS(),
			updateRate:  float64(time.Second) / float64(interval),
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

	mode := "OFF"
	if gs.AltNetMode {
		mode = "ON"
	}
	adjustment := networkAdjustmentDelay(interval, reply, jitter, gs.AltNetDelay)
	configuredOffset := time.Duration(gs.AltNetDelay) * time.Millisecond
	advance := configuredOffset - adjustment
	if advance < 0 {
		advance = 0
	}
	recentLoss, sessionLoss, received, lost := packetLossSnapshot()
	setStatsMetric(statsReplyMetric, formatToolbarLatency(reply))
	setStatsMetric(statsJitterMetric, formatToolbarLatency(jitter))
	setStatsMetric(statsRecentLoss, fmt.Sprintf("%.1f%%", recentLoss))
	setStatsMetric(statsSessionLoss, fmt.Sprintf("%.2f%%", sessionLoss))
	if statsNetworkText != nil {
		timing := "original network timing"
		if gs.AltNetMode {
			timing = fmt.Sprintf("send in %s, %s early", formatToolbarLatency(adjustment), formatToolbarLatency(advance))
		}
		statsNetworkText.Text = fmt.Sprintf("PNA %s   |   Offset %s   |   %s\nPackets: %d received, %d lost",
			mode, formatToolbarLatency(configuredOffset), timing, received, lost)
		statsNetworkText.Dirty = true
	}
	drawStatsGraph(statsNetworkImage, replies, jitters, statsReplyColor, statsJitterColor)
	if statsNetworkGraph != nil {
		statsNetworkGraph.Dirty = true
	}
	setStatsMetric(statsFPSMetric, fmt.Sprintf("%.1f", ebiten.ActualFPS()))
	setStatsMetric(statsUpdateMetric, fmt.Sprintf("%.2f / sec", float64(time.Second)/float64(interval)))
	setStatsMetric(statsCPUMetric, fmt.Sprintf("~%.1f%%", gameLoopCPULoad()))
	if statsRateText != nil {
		statsRateText.Text = fmt.Sprintf("Server frame interval: %s", interval.Round(time.Millisecond))
		statsRateText.Dirty = true
	}
	drawStatsGraph(statsRateImage, fps, updates, statsFPSColor, statsUpdateColor)
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
	drawStatsGraph(statsCacheImage, cacheMemory, gpuMemory, statsCacheMemoryColor, statsGPUMemoryColor)
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
