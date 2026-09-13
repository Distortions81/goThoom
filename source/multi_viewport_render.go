package main

import (
	"image"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const viewportRenderAtlasGap = 2

type viewportRenderRequest struct {
	target   *ebiten.Image
	session  *Session
	state    *viewportRenderState
	selected bool
}

type viewportRenderFrame struct {
	request viewportRenderRequest
	result  viewportRenderResult

	worldKey     worldRenderKey
	renderScale  float64
	snap         drawSnapshot
	alpha        float64
	mobileFade   float32
	pictFade     float32
	useLighting  bool
	useComposite bool
	worldStarted time.Time

	scene       *ebiten.Image
	lit         *ebiten.Image
	output      *ebiten.Image
	directWorld *ebiten.Image
	atlasBacked bool
	haveSnap    bool
	renderScene bool
	splash      bool
	finalize    bool
}

// viewportRenderAtlas lets Ebitengine retain one destination while it renders
// different session regions. Scene work is completed for every changed view
// before lighting switches to the second shared target.
type viewportRenderAtlas struct {
	scene *ebiten.Image
	lit   *ebiten.Image
	size  image.Point
}

func (a *viewportRenderAtlas) ensure(size image.Point) bool {
	if a == nil || size.X < 1 || size.Y < 1 {
		return false
	}
	limit := ebiten.MaxImageSize()
	if limit > 0 && (size.X > limit || size.Y > limit) {
		return false
	}
	if a.scene != nil && a.lit != nil && a.size.X >= size.X && a.size.Y >= size.Y {
		return true
	}
	width, height := size.X, size.Y
	if a.size.X > width {
		width = a.size.X
	}
	if a.size.Y > height {
		height = a.size.Y
	}
	if limit > 0 && (width > limit || height > limit) {
		return false
	}
	if a.scene != nil {
		a.scene.Deallocate()
	}
	if a.lit != nil {
		a.lit.Deallocate()
	}
	a.scene = ebiten.NewImageWithOptions(image.Rect(0, 0, width, height), &ebiten.NewImageOptions{Unmanaged: true})
	a.lit = ebiten.NewImageWithOptions(image.Rect(0, 0, width, height), &ebiten.NewImageOptions{Unmanaged: true})
	a.size = image.Pt(width, height)
	return true
}

// packViewportRenderRects picks the most balanced valid row-major grid, using
// area to break ties. Four equal tiled viewports naturally become a 2x2 atlas,
// while uneven freeform windows can use a different column count.
func packViewportRenderRects(sizes []image.Point, limit int) ([]image.Rectangle, image.Point, bool) {
	if len(sizes) == 0 {
		return nil, image.Point{}, false
	}
	bestArea := int64(^uint64(0) >> 1)
	bestLongest := int(^uint(0) >> 1)
	var best []image.Rectangle
	var bestSize image.Point
	for columns := 1; columns <= len(sizes); columns++ {
		rows := (len(sizes) + columns - 1) / columns
		columnWidths := make([]int, columns)
		rowHeights := make([]int, rows)
		valid := true
		for index, size := range sizes {
			if size.X < 1 || size.Y < 1 || limit > 0 && (size.X > limit || size.Y > limit) {
				valid = false
				break
			}
			column := index % columns
			row := index / columns
			columnWidths[column] = max(columnWidths[column], size.X)
			rowHeights[row] = max(rowHeights[row], size.Y)
		}
		if !valid {
			continue
		}
		width := viewportRenderAtlasGap * (columns - 1)
		for _, columnWidth := range columnWidths {
			width += columnWidth
		}
		height := viewportRenderAtlasGap * (rows - 1)
		for _, rowHeight := range rowHeights {
			height += rowHeight
		}
		if limit > 0 && (width > limit || height > limit) {
			continue
		}
		area := int64(width) * int64(height)
		longest := max(width, height)
		if longest > bestLongest || longest == bestLongest && area >= bestArea {
			continue
		}
		xOffsets := make([]int, columns)
		for column := 1; column < columns; column++ {
			xOffsets[column] = xOffsets[column-1] + columnWidths[column-1] + viewportRenderAtlasGap
		}
		yOffsets := make([]int, rows)
		for row := 1; row < rows; row++ {
			yOffsets[row] = yOffsets[row-1] + rowHeights[row-1] + viewportRenderAtlasGap
		}
		rects := make([]image.Rectangle, len(sizes))
		for index, size := range sizes {
			column := index % columns
			row := index / columns
			rects[index] = image.Rectangle{Min: image.Pt(xOffsets[column], yOffsets[row]), Max: image.Pt(xOffsets[column]+size.X, yOffsets[row]+size.Y)}
		}
		bestArea = area
		bestLongest = longest
		best = rects
		bestSize = image.Pt(width, height)
	}
	return best, bestSize, best != nil
}

func prepareViewportRenderFrame(request viewportRenderRequest, now time.Time, assetTrace *assetLoadFrameTrace) viewportRenderFrame {
	frame := viewportRenderFrame{request: request}
	if request.target == nil || request.session == nil || request.state == nil {
		return frame
	}
	bufW := request.target.Bounds().Dx()
	bufH := request.target.Bounds().Dy()
	frame.result.viewRect, frame.renderScale = fittedWorldView(bufW, bufH)
	if request.selected && assetTrace != nil {
		assetTrace.setWorldContext(bufW, bufH, frame.renderScale)
	}
	frame.worldKey = currentSessionWorldRenderKey(request.session, bufW, bufH)
	if viewportWorldRenderCanBeReused(request.state, frame.worldKey) {
		if request.selected {
			layoutActiveGameOverlays(frame.result.viewRect, clientActivityNone, request.session)
		}
		frame.result.ready = true
		return frame
	}
	frame.finalize = true
	if request.selected && assetTrace != nil {
		frame.worldStarted = time.Now()
	}
	if showingSessionGameSplash(request.session) {
		request.state.nightTransition = nightTransitionState{}
		frame.splash = true
		frame.renderScene = true
		return frame
	}

	captureSessionDrawSnapshotIfChanged(request.session, &request.state.drawSnapshot)
	if request.selected && bubbleTorture {
		prepareBubbleTortureSnapshot(&request.state.drawSnapshot, now)
	} else if request.selected && setupWizardPreviewActive {
		prepareSetupWizardSceneSnapshot(&request.state.drawSnapshot, now)
	}
	frame.snap = request.state.drawSnapshot
	frame.alpha, frame.mobileFade, frame.pictFade = computeInterpolation(now, frame.snap.prevTime, frame.snap.curTime, gs.MobileBlendAmount, gs.BlendAmount)
	previousScale := gs.GameScale
	gs.GameScale = frame.renderScale
	pinSceneSpriteSlots(frame.snap)
	deferred := prepareSceneArtworkFrame(request.session, frame.snap)
	gs.GameScale = previousScale
	if deferred {
		noteClientActivity(clientActivityGPU)
		return frame
	}
	frame.useLighting = shaderLightingEnabled() && lightingShader != nil
	frame.useComposite = frame.useLighting && sceneMayNeedLighting(frame.snap)
	frame.renderScene = true
	return frame
}

func prepareDirectViewportTargets(frame *viewportRenderFrame) {
	target := frame.request.target
	target.Fill(playfieldBackgroundColor())
	worldView := target.RecyclableSubImage(frame.result.viewRect)
	frame.directWorld = worldView
	frame.output = worldView
	if frame.useComposite {
		frame.scene = ensureViewportLightingTmp(frame.request.state, worldView.Bounds())
		frame.scene.Fill(playfieldBackgroundColor())
	} else {
		frame.scene = worldView
	}
}

func renderViewportSceneStage(frame *viewportRenderFrame) {
	if !frame.renderScene {
		return
	}
	previousScale := gs.GameScale
	gs.GameScale = frame.renderScale
	defer func() { gs.GameScale = previousScale }()
	if frame.splash {
		prepareDirectViewportTargets(frame)
		drawSplash(frame.output, 0, 0)
		frame.result.rendered = true
		frame.result.ready = true
		return
	}
	if frame.scene == nil {
		prepareDirectViewportTargets(frame)
	}
	frame.scene.Fill(playfieldBackgroundColor())
	drawScene(frame.scene, 0, 0, frame.snap, frame.alpha, frame.mobileFade, frame.pictFade, frame.request.state)
	drawReplacementEffects(frame.scene, frame.scene.Bounds().Min.X, frame.scene.Bounds().Min.Y, frame.snap.mobiles, frame.snap.prevMobiles, frame.snap.picShiftX, frame.snap.picShiftY, frame.alpha)
	if frame.useLighting {
		addNightDarkSourcesForViewport(frame.request.state, frame.scene.Bounds(), float32(frame.alpha), frame.snap.night)
	} else {
		drawNightOverlayForNight(frame.scene, 0, 0, frame.snap.night)
	}
	frame.haveSnap = true
}

func renderViewportLightingStage(frame *viewportRenderFrame) {
	if !frame.renderScene || frame.splash {
		return
	}
	state := frame.request.state
	hasLighting := len(state.lighting.lights) != 0 || len(state.lighting.darks) != 0
	if frame.atlasBacked && frame.useLighting && (frame.useComposite || hasLighting) {
		applyWorldCompositeForViewport(state, frame.snap.night, frame.lit, frame.scene, state.lighting.lights, state.lighting.darks, float32(frame.alpha), true)
		frame.output = frame.lit
	} else if !frame.atlasBacked && frame.useComposite {
		applyWorldCompositeForViewport(state, frame.snap.night, frame.output, frame.scene, state.lighting.lights, state.lighting.darks, float32(frame.alpha), true)
	} else if !frame.atlasBacked && frame.useLighting && hasLighting {
		applyLightingShaderForViewportNight(state, frame.snap.night, frame.output, state.lighting.lights, state.lighting.darks, float32(frame.alpha))
	} else {
		applyDetailedCharacterShadowForViewport(state, frame.scene)
		frame.output = frame.scene
	}
	frame.result.rendered = true
	frame.result.ready = true
}

func publishViewportRenderFrame(frame *viewportRenderFrame, assetTrace *assetLoadFrameTrace) {
	request := frame.request
	if frame.atlasBacked && frame.result.rendered {
		request.target.Fill(playfieldBackgroundColor())
		worldView := request.target.RecyclableSubImage(frame.result.viewRect)
		op := &ebiten.DrawImageOptions{Blend: ebiten.BlendCopy}
		op.GeoM.Translate(
			float64(worldView.Bounds().Min.X),
			float64(worldView.Bounds().Min.Y),
		)
		worldView.DrawImage(frame.output, op)
		worldView.Recycle()
	}
	if frame.atlasBacked && frame.scene != nil {
		frame.scene.Recycle()
		frame.scene = nil
	}
	if frame.atlasBacked && frame.lit != nil {
		frame.lit.Recycle()
		frame.lit = nil
	}
	if !frame.finalize {
		return
	}

	worldView := frame.directWorld
	if worldView == nil {
		worldView = request.target.RecyclableSubImage(frame.result.viewRect)
	}
	previousScale := gs.GameScale
	gs.GameScale = frame.renderScale
	if frame.haveSnap && (request.selected || !gs.ToolbarStatusBars) {
		drawStatusBars(worldView, 0, 0, frame.snap, frame.alpha)
	}
	if request.selected && replacementEffectsPreview {
		drawReplacementEffectsPreview(worldView)
	}
	if frame.haveSnap && !frame.result.viewRect.Empty() {
		finalScale := frame.renderScale
		if finalScale <= 0 {
			finalScale = 1
		}
		drawSpeechBubblesForViewport(worldView, frame.snap, frame.alpha, speechBubbleWindowScale(finalScale), request.state)
		drawScriptOverlaysForSession(request.session, worldView, finalScale)
	}
	gs.GameScale = previousScale
	worldView.Recycle()
	frame.directWorld = nil

	if request.selected {
		activity := takeClientActivity()
		overlays := layoutActiveGameOverlays(frame.result.viewRect, activity, request.session)
		if frame.haveSnap {
			drawRecPlayBadge(request.target, overlays.recPlay, overlays.recPlayLabel)
		}
		drawFPSOverlay(request.target, overlays.fps, overlays.fpsLabel)
		drawClientActivityIndicators(request.target, activity, overlays.activity)
		drawGameMessageOverlays(request.target)
		if assetTrace != nil {
			assetTrace.addWorldDuration(time.Since(frame.worldStarted))
		}
	}
	if frame.result.rendered {
		request.state.lastWorldRenderKey = frame.worldKey
		request.state.worldRenderValid = true
	}
}

func renderSessionViewports(atlas *viewportRenderAtlas, requests []viewportRenderRequest, now time.Time, assetTrace *assetLoadFrameTrace) []viewportRenderResult {
	frames := make([]viewportRenderFrame, len(requests))
	batchIndexes := make([]int, 0, len(requests))
	sizes := make([]image.Point, 0, len(requests))
	snapshots := make([]drawSnapshot, 0, len(requests))
	for index, request := range requests {
		frames[index] = prepareViewportRenderFrame(request, now, assetTrace)
		if frames[index].renderScene && !frames[index].splash {
			batchIndexes = append(batchIndexes, index)
			sizes = append(sizes, frames[index].result.viewRect.Size())
			snapshots = append(snapshots, frames[index].snap)
		}
	}
	if len(snapshots) != 0 {
		pinSceneSpriteSlotsForSnapshots(snapshots)
	}

	if len(batchIndexes) > 1 && atlas != nil {
		if rects, size, ok := packViewportRenderRects(sizes, ebiten.MaxImageSize()); ok && atlas.ensure(size) {
			for packedIndex, frameIndex := range batchIndexes {
				frame := &frames[frameIndex]
				frame.scene = atlas.scene.RecyclableSubImage(rects[packedIndex])
				frame.lit = atlas.lit.RecyclableSubImage(rects[packedIndex])
				frame.atlasBacked = true
			}
		}
	}

	// Keep painter order within a viewport, but finish the expensive scene pass
	// for every session before switching the shared destination to lighting.
	for _, atlasBacked := range []bool{true, false} {
		for index := range frames {
			if frames[index].atlasBacked == atlasBacked {
				renderViewportSceneStage(&frames[index])
			}
		}
	}
	for _, atlasBacked := range []bool{true, false} {
		for index := range frames {
			if frames[index].atlasBacked == atlasBacked {
				renderViewportLightingStage(&frames[index])
			}
		}
	}
	results := make([]viewportRenderResult, len(frames))
	for index := range frames {
		publishViewportRenderFrame(&frames[index], assetTrace)
		results[index] = frames[index].result
	}
	return results
}
