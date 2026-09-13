package main

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

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
	output      *ebiten.Image
	directWorld *ebiten.Image
	haveSnap    bool
	renderScene bool
	splash      bool
	finalize    bool
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
	if frame.useComposite {
		applyWorldCompositeForViewport(state, frame.snap.night, frame.output, frame.scene, state.lighting.lights, state.lighting.darks, float32(frame.alpha), true)
	} else if frame.useLighting && hasLighting {
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
