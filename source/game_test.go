package main

import (
	"image/color"
	"math"
	"testing"
	"time"

	"gothoom/climg"
	"gothoom/eui"
)

func TestGameWindowTitle(t *testing.T) {
	originalPlayerName := playerName
	t.Cleanup(func() { playerName = originalPlayerName })

	playerName = ""
	if got, want := gameWindowTitle(), "goThoom"; got != want {
		t.Fatalf("gameWindowTitle() = %q, want %q", got, want)
	}
	playerName = "Aldaron"
	if got, want := gameWindowTitle(), "goThoom -- Aldaron"; got != want {
		t.Fatalf("gameWindowTitle() = %q, want %q", got, want)
	}
}

func TestPlayfieldBackgroundColorUsesGammaCorrection(t *testing.T) {
	oldImages := clImages
	images := &climg.CLImages{}
	clImages = images
	t.Cleanup(func() { clImages = oldImages })

	if got, want := playfieldBackgroundColor(), (color.RGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff}); got != want {
		t.Fatalf("uncorrected background got %v, want %v", got, want)
	}

	images.SetGammaCorrection(true, 1.8, 2.2)
	if got, want := playfieldBackgroundColor(), (color.RGBA{R: 0x98, G: 0x98, B: 0x98, A: 0xff}); got != want {
		t.Fatalf("corrected background got %v, want %v", got, want)
	}
}

func TestStopWalkIfOutside(t *testing.T) {
	old := gs.ClickToToggle
	gs.ClickToToggle = true
	walkToggled = true
	stopWalkIfOutside(true, false)
	if walkToggled {
		t.Fatalf("walkToggled should be false after outside click")
	}

	walkToggled = true
	stopWalkIfOutside(true, true)
	if !walkToggled {
		t.Fatalf("walkToggled should remain true when clicking inside game")
	}

	walkToggled = true
	stopWalkIfOutside(false, false)
	if !walkToggled {
		t.Fatalf("walkToggled should remain true when not clicking")
	}

	gs.ClickToToggle = old
}

func TestContinueHeldWalk(t *testing.T) {
	prev := inputState{mouseDown: true}
	if !continueHeldWalk(prev, false, true, 0, false) {
		t.Fatalf("walk should continue when mouse is held outside")
	}
	if continueHeldWalk(prev, false, false, 0, false) {
		t.Fatalf("walk should stop when mouse button is released")
	}
	if !continueHeldWalk(inputState{}, true, true, 2, false) {
		t.Fatalf("walk should start when mouse is held inside game")
	}
}

func TestWorldDrawInfoSubtractsTitleHeight(t *testing.T) {
	oldGameWin := gameWin
	oldScale := gs.GameScale
	oldTiledWindows := gs.TiledWindows
	defer func() {
		gameWin = oldGameWin
		gs.GameScale = oldScale
		gs.TiledWindows = oldTiledWindows
	}()

	gs.GameScale = 1
	gs.TiledWindows = false
	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.Position = eui.Point{}
	gameWin.Size = eui.Point{X: gameAreaSizeX + 4, Y: gameAreaSizeY + 4 + 20}
	gameWin.Margin = 0
	gameWin.Border = 0
	gameWin.BorderPad = 0
	gameWin.Padding = 0
	gameWin.TitleHeight = 20

	x, y, scale := worldDrawInfo()
	if x != 2 || y != 22 {
		t.Fatalf("world origin got (%d,%d), want (2,22)", x, y)
	}
	wantScale := float64(gameAreaSizeX-1) / float64(gameAreaSizeX)
	if math.Abs(scale-wantScale) > 0.000001 {
		t.Fatalf("world scale got %v, want %v", scale, wantScale)
	}
}

func TestSpeechBubbleScaleDoesNotFollowSupersamplingDefault(t *testing.T) {
	if got := speechBubbleWindowScale(3); got != 1 {
		t.Fatalf("3x physical scale got bubble scale %v, want 1", got)
	}
	if got := speechBubbleWindowScale(4); math.Abs(got-4.0/3.0) > 1e-9 {
		t.Fatalf("4x physical scale got bubble scale %v, want %v", got, 4.0/3.0)
	}
	if got := speechBubbleWindowScale(0); got != 1 {
		t.Fatalf("invalid physical scale got bubble scale %v, want 1", got)
	}
}

func TestUpdateGameImageSizeKeepsVisibleImageExactWhenShrinking(t *testing.T) {
	oldGameWin := gameWin
	oldGameImageItem := gameImageItem
	oldGameImage := gameImage
	oldGameImageBacking := gameImageBacking
	oldScale := eui.UIScale()
	oldTiledWindows := gs.TiledWindows
	defer func() {
		gameWin = oldGameWin
		gameImageItem = oldGameImageItem
		gameImage = oldGameImage
		gameImageBacking = oldGameImageBacking
		eui.SetUIScale(oldScale)
		gs.TiledWindows = oldTiledWindows
	}()

	eui.SetUIScale(1)
	gs.TiledWindows = false
	gameImageItem = nil
	gameImage = nil
	gameImageBacking = nil
	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.Padding = 0
	gameWin.Border = 0
	gameWin.BorderPad = 0
	gameWin.Margin = 0
	gameWin.TitleHeight = 0

	gameWin.Size = eui.Point{X: 104, Y: 84}
	updateGameImageSize()
	if gameImage == nil {
		t.Fatalf("initial visible image is nil")
	}
	if gameImage.Bounds().Dx() != 100 || gameImage.Bounds().Dy() != 80 {
		t.Fatalf("initial visible image got %v, want 100x80", gameImage.Bounds())
	}
	backing := gameImageBacking

	gameWin.Size = eui.Point{X: 84, Y: 64}
	updateGameImageSize()
	if gameImageBacking != backing {
		t.Fatalf("backing image was reallocated while shrinking")
	}
	if gameImage == nil {
		t.Fatalf("shrunk visible image is nil")
	}
	if gameImage.Bounds().Dx() != 80 || gameImage.Bounds().Dy() != 60 {
		t.Fatalf("shrunk visible image got %v, want 80x60", gameImage.Bounds())
	}
	if gameImageItem.Image != gameImage {
		t.Fatalf("game image item does not use current visible subimage")
	}
}

func TestTiledGameResizeKeepsManagedWindowSize(t *testing.T) {
	oldGameWin := gameWin
	oldGameImageItem := gameImageItem
	oldGameImage := gameImage
	oldGameImageBacking := gameImageBacking
	oldTiledWindows := gs.TiledWindows
	oldInAspectResize := inAspectResize
	defer func() {
		gameWin = oldGameWin
		gameImageItem = oldGameImageItem
		gameImage = oldGameImage
		gameImageBacking = oldGameImageBacking
		gs.TiledWindows = oldTiledWindows
		inAspectResize = oldInAspectResize
	}()

	gameImageItem = nil
	gameImage = nil
	gameImageBacking = nil
	inAspectResize = false
	gs.TiledWindows = true
	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.Padding = 0
	gameWin.Border = 0
	gameWin.BorderPad = 0
	gameWin.Margin = 0
	gameWin.TitleHeight = 0
	gameWin.Size = eui.Point{X: 600, Y: 800}

	onGameWindowResize()

	if got, want := gameWin.GetSize(), (eui.Point{X: 600, Y: 800}); got != want {
		t.Fatalf("tiled game window size got %v, want %v", got, want)
	}
}

func TestTiledGameImageFillsEveryManagedPixel(t *testing.T) {
	oldGameWin := gameWin
	oldGameImageItem := gameImageItem
	oldGameImage := gameImage
	oldGameImageBacking := gameImageBacking
	oldTiledWindows := gs.TiledWindows
	defer func() {
		gameWin = oldGameWin
		gameImageItem = oldGameImageItem
		gameImage = oldGameImage
		gameImageBacking = oldGameImageBacking
		gs.TiledWindows = oldTiledWindows
	}()

	gameImageItem = nil
	gameImage = nil
	gameImageBacking = nil
	gs.TiledWindows = true
	gameWin = eui.NewWindow()
	gameWin.NoScale = true
	gameWin.Padding = 0
	gameWin.TitleHeight = 0
	gameWin.Size = eui.Point{X: 601, Y: 799}

	updateGameImageSize()

	if got := gameImage.Bounds().Size(); got.X != 601 || got.Y != 799 {
		t.Fatalf("tiled game image size = %v, want 601x799", got)
	}
	if got := gameImageItem.Position; got != (eui.Point{}) {
		t.Fatalf("tiled game image position = %v, want zero", got)
	}
}

func TestPNAInitialLeadUsesFrameRateJitterAndSafety(t *testing.T) {
	originalSafety := networkAdjustmentSafetyPercent.Load()
	networkAdjustmentSafetyPercent.Store(10)
	resetPNAController()
	t.Cleanup(func() {
		networkAdjustmentSafetyPercent.Store(originalSafety)
		resetPNAController()
	})

	const interval = 200 * time.Millisecond
	const jitter = 12 * time.Millisecond
	if got := networkAdjustmentSafetyMargin(interval); got != 20*time.Millisecond {
		t.Fatalf("safety margin = %v, want 20ms", got)
	}
	if got := pnaBaseLead(interval, jitter); got != 32*time.Millisecond {
		t.Fatalf("base lead = %v, want 32ms", got)
	}
	// The controller begins conservatively at one quarter of a frame, then
	// command acknowledgements can probe later in bounded steps.
	if got := pnaLeadSnapshot(interval, jitter); got != 50*time.Millisecond {
		t.Fatalf("initial lead = %v, want 50ms", got)
	}
}

func TestRecordServerFrameTimingTracksCadenceAndIgnoresDuplicates(t *testing.T) {
	primarySession.timing.cadenceMu.Lock()
	oldLastFrameTime, oldFrameInterval := primarySession.timing.lastFrameAt, primarySession.timing.interval
	oldLastTimingFrame := primarySession.timing.lastFrame
	oldTimingSamples := append([]timedDurationSample(nil), primarySession.timing.samples...)
	oldJitter, oldRate := primarySession.timing.jitter, primarySession.timing.updatesPerSecond
	primarySession.timing.lastFrameAt = time.Time{}
	primarySession.timing.interval = framems * time.Millisecond
	primarySession.timing.lastFrame = 0
	primarySession.timing.samples = nil
	primarySession.timing.jitter = 0
	primarySession.timing.updatesPerSecond = 0
	primarySession.timing.cadenceMu.Unlock()
	t.Cleanup(func() {
		primarySession.timing.cadenceMu.Lock()
		primarySession.timing.lastFrameAt, primarySession.timing.interval = oldLastFrameTime, oldFrameInterval
		primarySession.timing.lastFrame = oldLastTimingFrame
		primarySession.timing.samples = oldTimingSamples
		primarySession.timing.jitter, primarySession.timing.updatesPerSecond = oldJitter, oldRate
		primarySession.timing.cadenceMu.Unlock()
	})

	start := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	if !recordServerFrameTiming(100, start) {
		t.Fatal("first frame was not accepted as the phase origin")
	}
	for i := 1; i <= pnaTimingWarmupSamples; i++ {
		if !recordServerFrameTiming(int32(100+i), start.Add(time.Duration(i)*200*time.Millisecond)) {
			t.Fatalf("frame %d was not accepted", 100+i)
		}
	}
	// A three-frame gap still represents 200ms server updates, not one 600ms
	// update. Packet-loss fallback handles the missing datagrams separately.
	if !recordServerFrameTiming(108, start.Add(1600*time.Millisecond)) {
		t.Fatal("frame after a sequence gap was not accepted")
	}
	if recordServerFrameTiming(108, start.Add(1700*time.Millisecond)) {
		t.Fatal("duplicate frame changed the timing origin")
	}
	if !recordServerFrameTiming(109, start.Add(1500*time.Millisecond)) {
		t.Fatal("newer cross-channel frame was not accepted")
	}

	primarySession.timing.cadenceMu.Lock()
	gotInterval, gotJitter, gotRate := primarySession.timing.interval, primarySession.timing.jitter, primarySession.timing.updatesPerSecond
	gotLastTime, gotLastFrame := primarySession.timing.lastFrameAt, primarySession.timing.lastFrame
	gotSamples := len(primarySession.timing.samples)
	primarySession.timing.cadenceMu.Unlock()
	if gotInterval != 200*time.Millisecond || gotJitter != 0 || gotRate != 5 {
		t.Fatalf("timing estimate = interval %v jitter %v rate %v, want 200ms/0/5", gotInterval, gotJitter, gotRate)
	}
	if gotLastTime != start.Add(1600*time.Millisecond) || gotLastFrame != 109 || gotSamples != pnaTimingWarmupSamples+1 {
		t.Fatalf("phase state = time %v frame %d samples %d", gotLastTime, gotLastFrame, gotSamples)
	}
}

func TestPNAFeedbackMovesLaterSlowlyAndEarlierAfterMiss(t *testing.T) {
	originalEnabled := gs.AltNetMode
	originalSafety := networkAdjustmentSafetyPercent.Load()
	primarySession.timing.cadenceMu.Lock()
	originalJitter := primarySession.timing.jitter
	primarySession.timing.jitter = 0
	primarySession.timing.cadenceMu.Unlock()
	primarySession.frames.statsMu.Lock()
	originalFrameBuckets, originalLostBuckets, originalBucketTimes := primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes
	primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes = [5]int{}, [5]int{}, [5]int64{}
	primarySession.frames.statsMu.Unlock()
	primarySession.timing.controllerMu.Lock()
	originalController := primarySession.timing.controller
	primarySession.timing.controller = pnaControllerState{initialized: true, lead: 50 * time.Millisecond}
	primarySession.timing.controllerMu.Unlock()
	primarySession.timing.fallbackMu.Lock()
	originalFallback := primarySession.timing.fallback
	primarySession.timing.fallback = pnaFallbackState{}
	primarySession.timing.fallbackMu.Unlock()
	gs.AltNetMode = true
	networkAdjustmentSafetyPercent.Store(10)
	t.Cleanup(func() {
		gs.AltNetMode = originalEnabled
		networkAdjustmentSafetyPercent.Store(originalSafety)
		primarySession.timing.cadenceMu.Lock()
		primarySession.timing.jitter = originalJitter
		primarySession.timing.cadenceMu.Unlock()
		primarySession.frames.statsMu.Lock()
		primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes = originalFrameBuckets, originalLostBuckets, originalBucketTimes
		primarySession.frames.statsMu.Unlock()
		primarySession.timing.controllerMu.Lock()
		primarySession.timing.controller = originalController
		primarySession.timing.controllerMu.Unlock()
		primarySession.timing.fallbackMu.Lock()
		primarySession.timing.fallback = originalFallback
		primarySession.timing.fallbackMu.Unlock()
	})

	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	for i := 0; i < pnaSuccessesBeforeLater; i++ {
		recordPNACommandFeedback(50*time.Millisecond, 150*time.Millisecond, 200*time.Millisecond, 10, 11, now.Add(time.Duration(i)*time.Second))
	}
	primarySession.timing.controllerMu.Lock()
	laterLead := primarySession.timing.controller.lead
	primarySession.timing.controllerMu.Unlock()
	if laterLead != 42500*time.Microsecond {
		t.Fatalf("lead after stable next-frame replies = %v, want 42.5ms", laterLead)
	}

	missTime := now.Add(5 * time.Second)
	recordPNACommandFeedback(242500*time.Microsecond, 157500*time.Microsecond, 200*time.Millisecond, 11, 13, missTime)
	primarySession.timing.controllerMu.Lock()
	missLead, learnedFloor, holdUntil := primarySession.timing.controller.lead, primarySession.timing.controller.learnedLeadFloor, primarySession.timing.controller.holdUntil
	primarySession.timing.controllerMu.Unlock()
	if missLead != 67500*time.Microsecond || learnedFloor != missLead || holdUntil != missTime.Add(pnaFeedbackHold) {
		t.Fatalf("miss recovery = lead %v floor %v hold %v, want 67.5ms floor and %v", missLead, learnedFloor, holdUntil, missTime.Add(pnaFeedbackHold))
	}

	// Successful replies during the hold cannot immediately undo the safer
	// correction, which prevents the controller from amplifying its own probe.
	recordPNACommandFeedback(67500*time.Microsecond, 132500*time.Microsecond, 200*time.Millisecond, 13, 14, missTime.Add(time.Second))
	primarySession.timing.controllerMu.Lock()
	heldLead, heldHits := primarySession.timing.controller.lead, primarySession.timing.controller.consecutiveHits
	primarySession.timing.controllerMu.Unlock()
	if heldLead != missLead || heldHits != 0 {
		t.Fatalf("feedback hold changed controller = lead %v hits %d", heldLead, heldHits)
	}

	// Once the hold expires, successful replies must not probe later across
	// the boundary that already caused a full-frame miss.
	for i := 0; i < pnaSuccessesBeforeLater*3; i++ {
		recordPNACommandFeedback(100*time.Millisecond, 132500*time.Microsecond, 200*time.Millisecond, 13, 14,
			holdUntil.Add(time.Duration(i+1)*time.Second))
	}
	primarySession.timing.controllerMu.Lock()
	afterHoldLead, afterHoldFloor := primarySession.timing.controller.lead, primarySession.timing.controller.learnedLeadFloor
	primarySession.timing.controllerMu.Unlock()
	if afterHoldLead != missLead || afterHoldFloor != missLead {
		t.Fatalf("post-hold feedback re-crossed learned boundary: lead %v floor %v, want %v", afterHoldLead, afterHoldFloor, missLead)
	}

	// Once per long cooldown, make one much smaller later probe so a changed
	// server boundary can eventually be discovered without a short limit cycle.
	probeTime := missTime.Add(pnaBoundaryProbeInterval)
	recordPNACommandFeedback(100*time.Millisecond, 132500*time.Microsecond, 200*time.Millisecond, 14, 15, probeTime)
	primarySession.timing.controllerMu.Lock()
	probeLead, probeFloor, nextProbe := primarySession.timing.controller.lead, primarySession.timing.controller.learnedLeadFloor, primarySession.timing.controller.nextBoundaryProbe
	primarySession.timing.controllerMu.Unlock()
	if probeLead != 62500*time.Microsecond || probeFloor != probeLead || nextProbe != probeTime.Add(pnaBoundaryProbeInterval) {
		t.Fatalf("rare boundary probe = lead %v floor %v next %v", probeLead, probeFloor, nextProbe)
	}
	recordPNACommandFeedback(100*time.Millisecond, 137500*time.Microsecond, 200*time.Millisecond, 15, 16, probeTime.Add(time.Second))
	primarySession.timing.controllerMu.Lock()
	heldProbeLead := primarySession.timing.controller.lead
	primarySession.timing.controllerMu.Unlock()
	if heldProbeLead != probeLead {
		t.Fatalf("boundary probed again before cooldown: lead %v, want %v", heldProbeLead, probeLead)
	}

	// A miss following a probe moves the floor earlier and restarts cooldown.
	secondMiss := probeTime.Add(2 * time.Second)
	recordPNACommandFeedback(250*time.Millisecond, 132500*time.Microsecond, 200*time.Millisecond, 14, 16, secondMiss)
	primarySession.timing.controllerMu.Lock()
	secondLead, secondFloor, secondProbe := primarySession.timing.controller.lead, primarySession.timing.controller.learnedLeadFloor, primarySession.timing.controller.nextBoundaryProbe
	primarySession.timing.controllerMu.Unlock()
	if secondLead != 87500*time.Microsecond || secondFloor != secondLead || secondProbe != secondMiss.Add(pnaBoundaryProbeInterval) {
		t.Fatalf("second miss did not move learned boundary earlier: lead %v floor %v next %v", secondLead, secondFloor, secondProbe)
	}
}

func TestPNASimulationAcrossCadencesAndLatency(t *testing.T) {
	originalEnabled := gs.AltNetMode
	primarySession.timing.cadenceMu.Lock()
	originalJitter := primarySession.timing.jitter
	primarySession.timing.cadenceMu.Unlock()
	primarySession.frames.statsMu.Lock()
	originalFrameBuckets, originalLostBuckets, originalBucketTimes := primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes
	primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes = [5]int{}, [5]int{}, [5]int64{}
	primarySession.frames.statsMu.Unlock()
	primarySession.timing.controllerMu.Lock()
	originalController := primarySession.timing.controller
	primarySession.timing.controllerMu.Unlock()
	primarySession.timing.fallbackMu.Lock()
	originalFallback := primarySession.timing.fallback
	primarySession.timing.fallbackMu.Unlock()
	t.Cleanup(func() {
		gs.AltNetMode = originalEnabled
		primarySession.timing.cadenceMu.Lock()
		primarySession.timing.jitter = originalJitter
		primarySession.timing.cadenceMu.Unlock()
		primarySession.frames.statsMu.Lock()
		primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes = originalFrameBuckets, originalLostBuckets, originalBucketTimes
		primarySession.frames.statsMu.Unlock()
		primarySession.timing.controllerMu.Lock()
		primarySession.timing.controller = originalController
		primarySession.timing.controllerMu.Unlock()
		primarySession.timing.fallbackMu.Lock()
		primarySession.timing.fallback = originalFallback
		primarySession.timing.fallbackMu.Unlock()
	})

	// Each scenario supplies fixed socket-arrival times. The simulation has no
	// timer, goroutine, or network dependency: it exercises the same phase,
	// cadence, and acknowledgement controller decisions as the live path.
	scenarios := []struct {
		name    string
		rate    int
		jitter  time.Duration
		replies []time.Duration
	}{
		{name: "two_hz_stable", rate: 2, replies: []time.Duration{120 * time.Millisecond, 120 * time.Millisecond, 120 * time.Millisecond}},
		{name: "five_hz_variation", rate: 5, jitter: 8 * time.Millisecond, replies: []time.Duration{120 * time.Millisecond, 132 * time.Millisecond, 113 * time.Millisecond}},
		{name: "ten_hz_variation", rate: 10, jitter: 5 * time.Millisecond, replies: []time.Duration{105 * time.Millisecond, 118 * time.Millisecond, 96 * time.Millisecond}},
		{name: "twenty_hz_stable_latency_spans_frames", rate: 20, jitter: 3 * time.Millisecond, replies: []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond}},
		{name: "sixty_hz_variation_spans_frames", rate: 60, jitter: 2 * time.Millisecond, replies: []time.Duration{82 * time.Millisecond, 95 * time.Millisecond, 88 * time.Millisecond}},
	}

	start := time.Date(2026, time.September, 10, 12, 0, 0, 0, time.UTC)
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			interval := time.Second / time.Duration(scenario.rate)
			gs.AltNetMode = true
			primarySession.timing.cadenceMu.Lock()
			primarySession.timing.jitter = scenario.jitter
			primarySession.timing.cadenceMu.Unlock()
			primarySession.timing.controllerMu.Lock()
			primarySession.timing.controller = pnaControllerState{}
			primarySession.timing.controllerMu.Unlock()
			primarySession.timing.fallbackMu.Lock()
			primarySession.timing.fallback = pnaFallbackState{}
			primarySession.timing.fallbackMu.Unlock()

			lead := pnaLeadSnapshot(interval, scenario.jitter)
			if lead <= 0 || lead >= interval {
				t.Fatalf("initial lead %s is outside one %s frame", lead, interval)
			}

			// Simulate one second of unchanged predictive input. High-rate
			// servers must not turn this into a proportional packet flood.
			var lastSend time.Time
			sends := 0
			for at := start; at.Before(start.Add(time.Second)); at = at.Add(interval) {
				if pnaInputSendAllowed(lastSend, at, false, false) {
					lastSend = at
					sends++
				}
			}
			wantMax := scenario.rate
			if wantMax > 5 {
				wantMax = 5
			}
			if sends > wantMax {
				t.Fatalf("unchanged predictive sends = %d at %d Hz, want at most %d", sends, scenario.rate, wantMax)
			}
			if !pnaInputSendAllowed(lastSend, start.Add(time.Millisecond), true, false) ||
				!pnaInputSendAllowed(lastSend, start.Add(time.Millisecond), false, true) {
				t.Fatal("changed input or a queued command was delayed by the predictive rate limit")
			}

			// At high server rates these ordinary round trips cover several frame
			// IDs. They are not evidence that a send missed an observable phase
			// boundary, so they must not create a learned early floor.
			for i, reply := range scenario.replies {
				ackFrames := int32((reply + interval - 1) / interval)
				if ackFrames < 1 {
					ackFrames = 1
				}
				recordPNACommandFeedback(reply, interval-lead, interval, 100, 100+ackFrames, start.Add(time.Duration(i)*time.Second))
			}
			primarySession.timing.controllerMu.Lock()
			controller := primarySession.timing.controller
			primarySession.timing.controllerMu.Unlock()
			if controller.lead <= 0 || controller.lead >= interval {
				t.Fatalf("controller lead %s ran outside %s frame", controller.lead, interval)
			}
			if controller.learnedLeadFloor != 0 {
				t.Fatalf("ordinary replies learned an early boundary: floor=%s rate=%dHz", controller.learnedLeadFloor, scenario.rate)
			}
		})
	}
}

func TestPNAHighRateDelayedAcknowledgementDoesNotStick(t *testing.T) {
	originalEnabled := gs.AltNetMode
	primarySession.timing.cadenceMu.Lock()
	originalJitter := primarySession.timing.jitter
	primarySession.timing.cadenceMu.Unlock()
	primarySession.frames.statsMu.Lock()
	originalFrameBuckets, originalLostBuckets, originalBucketTimes := primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes
	primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes = [5]int{}, [5]int{}, [5]int64{}
	primarySession.frames.statsMu.Unlock()
	primarySession.timing.controllerMu.Lock()
	originalController := primarySession.timing.controller
	primarySession.timing.controllerMu.Unlock()
	primarySession.timing.fallbackMu.Lock()
	originalFallback := primarySession.timing.fallback
	primarySession.timing.fallbackMu.Unlock()
	t.Cleanup(func() {
		gs.AltNetMode = originalEnabled
		primarySession.timing.cadenceMu.Lock()
		primarySession.timing.jitter = originalJitter
		primarySession.timing.cadenceMu.Unlock()
		primarySession.frames.statsMu.Lock()
		primarySession.frames.frames, primarySession.frames.lostFrames, primarySession.frames.bucketTimes = originalFrameBuckets, originalLostBuckets, originalBucketTimes
		primarySession.frames.statsMu.Unlock()
		primarySession.timing.controllerMu.Lock()
		primarySession.timing.controller = originalController
		primarySession.timing.controllerMu.Unlock()
		primarySession.timing.fallbackMu.Lock()
		primarySession.timing.fallback = originalFallback
		primarySession.timing.fallbackMu.Unlock()
	})

	const interval = time.Second / 60
	now := time.Date(2026, time.September, 10, 12, 0, 0, 0, time.UTC)
	gs.AltNetMode = true
	primarySession.timing.cadenceMu.Lock()
	primarySession.timing.jitter = 0
	primarySession.timing.cadenceMu.Unlock()
	primarySession.timing.controllerMu.Lock()
	primarySession.timing.controller = pnaControllerState{}
	primarySession.timing.controllerMu.Unlock()
	primarySession.timing.fallbackMu.Lock()
	primarySession.timing.fallback = pnaFallbackState{}
	primarySession.timing.fallbackMu.Unlock()

	lead := pnaLeadSnapshot(interval, 0)
	// A 20 ms acknowledgement spans two 60 Hz frame IDs. At this cadence the
	// delta cannot distinguish a delayed path from a missed phase, so it must
	// not create a persistent learned floor.
	recordPNACommandFeedback(20*time.Millisecond, interval-lead, interval, 10, 12, now)
	primarySession.timing.controllerMu.Lock()
	learned := primarySession.timing.controller.learnedLeadFloor
	primarySession.timing.controllerMu.Unlock()
	if learned != 0 {
		t.Fatalf("high-rate acknowledgement created a stuck learned floor: %s", learned)
	}
}

func TestPNAFallbackReason(t *testing.T) {
	tests := []struct {
		name string
		loss float64
		want string
	}{
		{name: "healthy"},
		{name: "packet loss", loss: 1, want: "recent packet loss"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pnaFallbackReason(tt.loss); got != tt.want {
				t.Fatalf("pnaFallbackReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPNAReplyTimeAndJitterNeverPauseTiming(t *testing.T) {
	resetPNAFallback()
	t.Cleanup(resetPNAFallback)
	if usePNA, reason := pnaTimingStatus(0, time.Now()); !usePNA || reason != "" {
		t.Fatalf("PNA paused without packet loss: use=%v reason=%q", usePNA, reason)
	}
}

func TestPNATimingStatusCooldownAndHysteresis(t *testing.T) {
	resetPNAFallback()
	t.Cleanup(resetPNAFallback)
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	if usePNA, reason := pnaTimingStatus(1, now); usePNA || reason != "recent packet loss" {
		t.Fatalf("loss status = use:%v reason:%q, want immediate fallback", usePNA, reason)
	}
	if usePNA, reason := pnaTimingStatus(0, now.Add(4*time.Second)); usePNA || reason != "cooldown after recent packet loss" {
		t.Fatalf("cooldown status = use:%v reason:%q, want fallback", usePNA, reason)
	}
	if usePNA, reason := pnaTimingStatus(0.2, now.Add(6*time.Second)); usePNA || reason != "waiting for packet loss to clear" {
		t.Fatalf("hysteresis status = use:%v reason:%q, want fallback", usePNA, reason)
	}
	if usePNA, reason := pnaTimingStatus(0, now.Add(6*time.Second)); !usePNA || reason != "" {
		t.Fatalf("recovered status = use:%v reason:%q, want PNA", usePNA, reason)
	}
}

func TestNetworkAdjustmentSafetyMargin(t *testing.T) {
	original := networkAdjustmentSafetyPercent.Load()
	networkAdjustmentSafetyPercent.Store(10)
	t.Cleanup(func() { networkAdjustmentSafetyPercent.Store(original) })
	if got := networkAdjustmentSafetyMargin(200 * time.Millisecond); got != 20*time.Millisecond {
		t.Fatalf("networkAdjustmentSafetyMargin() = %v, want 20ms", got)
	}
}

func TestP95Duration(t *testing.T) {
	if got := p95Duration(nil); got != 0 {
		t.Fatalf("p95Duration(nil) = %v, want 0", got)
	}
	if got := p95Duration([]time.Duration{2 * time.Millisecond, 7 * time.Millisecond, time.Millisecond}); got != 7*time.Millisecond {
		t.Fatalf("p95Duration() = %v, want 7ms while the sample window is warming up", got)
	}
	samples := make([]time.Duration, 100)
	for i := range samples {
		samples[i] = time.Duration(i+1) * time.Millisecond
	}
	if got := p95Duration(samples); got != 95*time.Millisecond {
		t.Fatalf("p95Duration() = %v, want 95ms", got)
	}
}

func TestRetainRecentTimingSamples(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	samples := []timedDurationSample{
		{at: now.Add(-pnaTimingWindow - time.Nanosecond), value: 50 * time.Millisecond},
		{at: now.Add(-pnaTimingWindow), value: 10 * time.Millisecond},
		{at: now.Add(-time.Second), value: 5 * time.Millisecond},
	}
	got := retainRecentTimingSamples(samples, now)
	if len(got) != 2 || got[0].value != 10*time.Millisecond || got[1].value != 5*time.Millisecond {
		t.Fatalf("retainRecentTimingSamples() = %#v, want the two samples from the last minute", got)
	}
}
