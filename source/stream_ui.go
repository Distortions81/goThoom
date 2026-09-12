package main

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"time"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	text "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	clipboard "golang.design/x/clipboard"
)

var (
	streamWin         *eui.WindowData
	streamSource      *eui.ItemData
	streamResolution  *eui.ItemData
	streamFrameRate   *eui.ItemData
	streamHint        *eui.ItemData
	streamIdle        *eui.ItemData
	streamCursor      *eui.ItemData
	streamStatus      *eui.ItemData
	streamLight       *eui.ItemData
	streamGameBadge   *eui.ItemData
	streamStartButton *eui.ItemData
	streamStopButton  *eui.ItemData
)

func showStreamWindow() {
	if isWASM {
		consoleMessage("Local streaming is available in the desktop client.")
		return
	}
	if streamWin != nil && streamWin.Open {
		streamWin.BringForward()
		return
	}
	if streamWin == nil {
		makeStreamWindow()
	}
	refreshStreamWindow()
	streamWin.MarkOpen()
	streamWin.Refresh()
}

func refreshStreamWindow() {
	if streamWin == nil {
		return
	}
	running := streamOutputRunning()
	if streamSource != nil {
		streamSource.Selected = gs.StreamSource
		streamSource.Disabled = false
	}
	if streamResolution != nil {
		streamResolution.Selected = 0
		for index, resolution := range streamResolutionValues {
			if gs.StreamResolution == resolution {
				streamResolution.Selected = index
				break
			}
		}
		streamResolution.Disabled = running
	}
	if streamFrameRate != nil {
		streamFrameRate.Selected = 1
		if gs.StreamFPS == 15 {
			streamFrameRate.Selected = 0
		} else if gs.StreamFPS == 30 {
			streamFrameRate.Selected = 1
		} else if gs.StreamFPS == 60 {
			streamFrameRate.Selected = 2
		}
		streamFrameRate.Disabled = running
	}
	if streamHint != nil {
		streamHint.Text = streamBrowserSourceHint(gs.StreamResolution)
	}
	if streamIdle != nil {
		streamIdle.Selected = 0
		if gs.StreamIdleBlack {
			streamIdle.Selected = 1
		}
		streamIdle.Disabled = running
	}
	if streamCursor != nil {
		streamCursor.Checked = gs.StreamShowCursor
	}
	if streamStatus != nil {
		if running {
			streamStatus.Text = "Streaming"
		} else {
			streamStatus.Text = ""
		}
	}
	updateStreamIndicators(time.Now())
	if streamStartButton != nil {
		streamStartButton.Disabled = running
	}
	if streamStopButton != nil {
		streamStopButton.Disabled = !running
	}
}

func makeStreamWindow() {
	win := eui.NewWindow()
	streamWin = win
	win.Title = "Local Stream"
	win.Closable, win.Movable, win.AutoSize = true, true, true
	win.Resizable = false
	win.Padding = 12
	root := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Size: eui.Point{X: 400, Y: 1}}

	source, sourceEvents := eui.NewDropdown()
	source.Label = "Stream"
	source.Options = []string{"Game render", "Whole game window and UI"}
	source.Selected = gs.StreamSource
	source.Size = eui.Point{X: 400, Y: 28}
	source.SetTooltip("Game render shares only the playfield. The other option includes all visible client windows and UI. You can switch while streaming; output dimensions stay fixed.")
	streamSource = source
	root.AddItem(source)

	row := eui.NewRow()
	resolution, resolutionEvents := eui.NewDropdown()
	resolution.Label = "Resolution"
	resolution.Options = []string{"1280 × 720 (720p)", "1920 × 1080 (1080p)", "2560 × 1440 (1440p)", "3840 × 2160 (4K)"}
	resolution.SetTooltip("Output dimensions stay fixed until you restart the stream.")
	for index, streamResolution := range streamResolutionValues {
		if gs.StreamResolution == streamResolution {
			resolution.Selected = index
			break
		}
	}
	resolution.Size = eui.Point{X: 244, Y: 28}
	streamResolution = resolution
	frameRate, frameRateEvents := eui.NewDropdown()
	frameRate.Label = "Frame rate"
	frameRate.Options = []string{"15 FPS", "30 FPS", "60 FPS"}
	switch gs.StreamFPS {
	case 15:
		frameRate.Selected = 0
	case 60:
		frameRate.Selected = 2
	default:
		frameRate.Selected = 1
	}
	frameRate.Size = eui.Point{X: 148, Y: 28}
	frameRate.Position.X = 8
	streamFrameRate = frameRate
	row.AddItem(resolution)
	row.AddItem(frameRate)
	root.AddItem(row)

	idle, idleEvents := eui.NewDropdown()
	idle.Label = "Before login"
	idle.Options = []string{"Show splash screen", "Black"}
	if gs.StreamIdleBlack {
		idle.Selected = 1
	}
	idle.Size = eui.Point{X: 400, Y: 28}
	idle.SetTooltip("Choose what the stream shows before the game render is available.")
	streamIdle = idle
	root.AddItem(idle)

	cursor, cursorEvents := eui.NewCheckbox()
	cursor.Text = "Show cursor in stream"
	cursor.Size = eui.Point{X: 400, Y: 24}
	cursor.Checked = gs.StreamShowCursor
	cursor.SetTooltip("Draws a small cursor indicator into the stream before it is encoded. The desktop cursor is not captured automatically.")
	streamCursor = cursor
	root.AddItem(cursor)

	statusRow, light, status := newStreamIndicator()
	statusRow.Size.X = 400
	status.Size.X = 380
	streamStatus, streamLight = status, light
	hint, _ := eui.NewText()
	hint.Text = streamBrowserSourceHint(gs.StreamResolution)
	streamHint = hint
	hint.FontSize = 11
	hint.Size = eui.Point{X: 400, Y: 48}
	hint.SelectableText = true
	hint.SetTooltip("Click the URL to copy it.")
	hint.OnURLClick = func(url string) {
		if _, err := clipboard.Write(context.Background(), clipboard.FmtText, []byte(url)); err != nil {
			hint.SetTooltip("Could not copy the stream URL.")
		} else {
			hint.SetTooltip("COPIED!")
		}
		win.Refresh()
	}
	root.AddItem(hint)
	root.AddItem(statusRow)

	updateSettings := func() streamConfig {
		gs.StreamSource = source.Selected
		gs.StreamResolution = streamResolutionValues[resolution.Selected]
		gs.StreamFPS = []int{15, 30, 60}[frameRate.Selected]
		gs.StreamIdleBlack = idle.Selected == 1
		gs.StreamShowCursor = cursor.Checked
		settingsDirty = true
		return streamConfig{wholeClient: gs.StreamSource == 1, resolution: gs.StreamResolution, fps: gs.StreamFPS, idleBlack: gs.StreamIdleBlack, showCursor: gs.StreamShowCursor}
	}
	cursorEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventCheckboxChanged {
			updateSettings()
			setStreamOutputShowCursor(gs.StreamShowCursor)
			win.Refresh()
		}
	}
	for _, handler := range []*eui.EventHandler{sourceEvents, resolutionEvents, frameRateEvents, idleEvents} {
		handler.Handle = func(ev eui.UIEvent) {
			if ev.Type == eui.EventDropdownSelected {
				updateSettings()
				if ev.Item == source {
					setStreamOutputSource(gs.StreamSource == 1)
				}
				refreshStreamWindow()
				win.Refresh()
			}
		}
	}

	footer := eui.NewRow()
	stop, stopEvents := eui.NewButton()
	stop.Text = "End Stream"
	stop.Size = eui.Point{X: 120, Y: 32}
	streamStopButton = stop
	stopEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		stopStreamOutput()
		consoleMessage("stream output stopped")
		refreshStreamWindow()
		win.Refresh()
	}
	closeButton, closeEvents := eui.NewButton()
	closeButton.Text = "Close"
	closeButton.Size = eui.Point{X: 120, Y: 32}
	closeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			win.Close()
		}
	}
	start, startEvents := eui.NewButton()
	start.Text = "Start"
	start.Size = eui.Point{X: 152, Y: 32}
	streamStartButton = start
	startEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventClick {
			return
		}
		if err := startStreamOutput(updateSettings()); err != nil {
			status.Text = fmt.Sprintf("Could not start: %v", err)
			win.Refresh()
			return
		}
		refreshStreamWindow()
		win.Refresh()
	}
	footer.AddItem(stop)
	footer.AddItem(closeButton)
	footer.AddItem(start)
	root.AddItem(footer)
	win.AddItem(root)
	win.DefaultButton = start
	win.AddWindow(false)
	_ = win.SetPos(eui.Point{X: 80, Y: 80})
}

func streamBrowserSourceHint(resolution int) string {
	size := "matching the source size"
	if resolution != 0 {
		size = fmt.Sprintf("with size %d × %d", resolution*16/9, resolution)
	}
	return "In OBS, add a Browser Source " + size + ".\n" + streamOutputURL
}

var streamResolutionValues = []int{720, 1080, 1440, 2160}

func newStreamIndicator() (row, light, label *eui.ItemData) {
	row = eui.NewRow()
	row.Size = eui.Point{X: 112, Y: 22}
	light, _ = eui.NewImageFastItem(18, 22)
	label, _ = eui.NewText()
	label.FontSize = 11
	label.Size = eui.Point{X: 94, Y: 22}
	row.AddItem(light)
	row.AddItem(label)
	return
}

// Animate UI items without invalidating the cached world render or adding
// display-rate readbacks to the playfield stream.
func updateStreamIndicators(now time.Time) {
	running := streamOutputRunning()
	lightColor := eui.NewColor(85, 18, 18, 255)
	if running && now.UnixMilli()%1000 < 500 {
		lightColor = eui.NewColor(235, 45, 45, 255)
	}
	updateLight := func(light *eui.ItemData) bool {
		if light == nil || (light.Checked == running && light.TextColor == lightColor) {
			return false
		}
		light.Checked, light.TextColor = running, lightColor
		light.Image.Clear()
		if running {
			vector.FillCircle(light.Image, 9, 11, 5, color.RGBA(lightColor), true)
		}
		return true
	}
	if streamWin != nil && updateLight(streamLight) {
		streamWin.Refresh()
	}
	if gameWin == nil || gameImageItem == nil {
		return
	}
	created := false
	if running && (streamGameBadge == nil || streamGameBadge.ParentWindow != gameWin) {
		width, height := text.Measure("Streaming", mainFontBold, 0)
		streamGameBadge, _ = eui.NewImageFastItem(int(math.Ceil(width))+32, int(math.Ceil(height))+12)
		gameWin.AddItem(streamGameBadge)
		created = true
	}
	if streamGameBadge == nil {
		return
	}
	changed := created || streamGameBadge.TextColor != lightColor
	if changed {
		streamGameBadge.TextColor = lightColor
		drawStreamBadge(streamGameBadge.Image, color.RGBA(lightColor))
	}
	if streamGameBadge.Invisible != !running {
		streamGameBadge.Invisible = !running
		changed = true
	}
	if changed {
		gameWin.Refresh()
	}
}

// Match the recording/playback overlays: a vector lamp and bold caption on
// transparent pixels. A small text shadow keeps it readable over the world.
func drawStreamBadge(dst *ebiten.Image, lamp color.RGBA) {
	dst.Clear()
	vector.FillCircle(dst, 12, 12, 6, lamp, true)
	op := acquireTextDrawOpts()
	op.GeoM.Translate(25, 5)
	op.ColorScale.Scale(0, 0, 0, 0.8)
	text.Draw(dst, "Streaming", mainFontBold, op)
	op.GeoM.Translate(-1, -1)
	op.ColorScale.Reset()
	text.Draw(dst, "Streaming", mainFontBold, op)
	releaseTextDrawOpts(op)
}
