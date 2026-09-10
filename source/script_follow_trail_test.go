package main

import (
	"testing"
	"time"

	scriptapi "gt2"
)

func TestFollowBreadcrumbTransitions(t *testing.T) {
	initFont()
	for _, scenario := range []string{"turn-and-edge", "camera-only", "old-direction", "join-recent-track", "scenery-door", "location-door", "temporary-no-self", "no-wandering", "loss-during-wiggle", "trail-blocked", "wait", "manual", "fallen", "self-fallen", "frame-reset"} {
		t.Run(scenario, func(t *testing.T) {
			isolateScriptWorld(t)
			const owner = "follow_transition"
			sim := activateBundledProofScript(t, owner, "follow_player.go")
			sim.login(t, "Hero")
			scriptLocationMu.Lock()
			oldLocation := scriptLocation
			scriptLocation = "Inside"
			scriptLocationMu.Unlock()
			t.Cleanup(func() {
				scriptLocationMu.Lock()
				scriptLocation = oldLocation
				scriptLocationMu.Unlock()
			})
			frame := 0
			var blockers []frameMobile
			base := time.Now()
			pictures := []framePicture{{PictID: 100, H: -200, V: -200, Background: true}, {PictID: 101, H: -100, V: -100}}
			update := func(self, leader *frameMobile) inputState {
				frame++
				stateMu.Lock()
				state.descriptors = map[uint8]frameDescriptor{1: {Index: 1, Name: "Hero", Type: kDescPlayer}, 2: {Index: 2, Name: "Leader", Type: kDescPlayer}, 3: {Index: 3, Name: "Rat", Type: kDescMonster}, 7: {Index: 7, Name: "Le ader", Type: kDescPlayer}}
				state.liveMobs = nil
				if self != nil {
					state.liveMobs = append(state.liveMobs, *self)
				}
				if leader != nil {
					state.liveMobs = append(state.liveMobs, *leader)
				}
				state.liveMobs = append(state.liveMobs, blockers...)
				state.pictures = append([]framePicture(nil), pictures...)
				state.logicalFrame, state.receivedAt = frame, base.Add(time.Duration(frame)*200*time.Millisecond)
				// Deliberately leave CameraShift at zero, as with smoothing off.
				markWorldStateChanged()
				stateMu.Unlock()
				dispatchScriptChange(ChangeEvent{Type: ChangeWorld})
				sim.barrier(t)
				return applyScriptMovement(inputState{}, time.Now())
			}
			self := &frameMobile{Index: 1}
			update(self, &frameMobile{Index: 2, H: 80})
			sim.command(t, "follow", "Leader")
			if scenario == "camera-only" {
				for i := range pictures {
					pictures[i].H -= 20
					pictures[i].V -= 20
				}
				update(self, &frameMobile{Index: 2, H: 60, V: -20})
				self.H, self.V = 60, -20
				if update(self, nil).mouseDown {
					t.Fatal("camera motion invented an exit direction for a stationary player")
				}
				return
			}
			if scenario == "join-recent-track" {
				self.H = 60
			}
			update(self, &frameMobile{Index: 2, H: 80, V: 20})
			if scenario == "join-recent-track" {
				self.H, self.V = 110, 20
				update(self, &frameMobile{Index: 2, H: 140, V: 60})
				got := update(self, nil)
				if !got.mouseDown || got.mouseY <= self.V {
					t.Fatalf("returned to old corner instead of rejoining recent track: %+v", got)
				}
				return
			}
			if scenario == "old-direction" {
				for i := 0; i < 10; i++ {
					update(self, &frameMobile{Index: 2, H: 80, V: 20})
				}
				self.H, self.V = 80, 20
				if update(self, nil).mouseDown {
					t.Fatal("continued along an old heading after target stopped")
				}
				return
			}
			if scenario == "loss-during-wiggle" {
				for i := 0; i < 5; i++ {
					update(self, &frameMobile{Index: 2, H: 80, V: 20})
				}
			}
			// Retained edge mobiles must count as missing, not fresh observations.
			got := update(self, &frameMobile{Index: 2, H: 80, V: 20, Persist: true})
			if scenario != "loss-during-wiggle" && (!got.mouseDown || got.mouseX != 80 || got.mouseY != 0) {
				t.Fatalf("did not follow earlier breadcrumb before the turn: %+v", got)
			}
			switch scenario {
			case "turn-and-edge":
				// Shift scenery left by 60 as we approach the first breadcrumb.
				for i := range pictures {
					pictures[i].H -= 60
				}
				self.H = 20
				got = update(self, nil)
				if !got.mouseDown || got.mouseX != 20 || got.mouseY <= 0 {
					t.Fatalf("camera-adjusted trail did not turn toward doorway: %+v", got)
				}
				self.V = 20
				got = update(self, nil)
				if !got.mouseDown || got.mouseX != 20 || got.mouseY != 116 {
					t.Fatalf("did not continue straight beyond last sighting toward boundary: %+v", got)
				}
				self.V = 44 // The doorway crosses only beyond the last sighting.
				pictures[0].PictID, pictures[1].PictID = 200, 201
				if update(self, nil).mouseDown {
					t.Fatal("continued old exit coordinates after crossing boundary")
				}
				got = update(self, &frameMobile{Index: 7, H: -100, V: 44})
				if !got.mouseDown || got.mouseX >= self.H {
					t.Fatalf("did not reacquire by name with a new descriptor: %+v", got)
				}
			case "scenery-door", "location-door", "temporary-no-self":
				if scenario == "scenery-door" {
					pictures[0].PictID, pictures[1].PictID = 200, 201
				} else if scenario == "location-door" {
					scriptLocationMu.Lock()
					scriptLocation = "Outside"
					scriptLocationMu.Unlock()
				} else {
					if got = update(nil, nil); got.mouseDown {
						t.Fatal("steered without self during transition")
					}
				}
				if got = update(self, nil); got.mouseDown {
					t.Fatal("carried old coordinates into new scene")
				}
				got = update(self, &frameMobile{Index: 7, H: -100})
				if !got.mouseDown || got.mouseX >= 0 {
					t.Fatalf("did not resume following in new scene: %+v", got)
				}
			case "no-wandering":
				self.H = 40
				got = update(self, nil)
				if !got.mouseDown || got.mouseX != 80 || got.mouseY != 0 {
					t.Fatalf("left unobstructed trail while making progress: %+v", got)
				}
			case "loss-during-wiggle":
				if !got.mouseDown {
					t.Fatal("target loss discarded pending trail")
				}
				if !update(self, &frameMobile{Index: 2, H: 100}).mouseDown {
					t.Fatal("did not resume after target loss during recovery")
				}
			case "trail-blocked":
				blockers = []frameMobile{{Index: 3, H: 40}}
				got = update(self, nil)
				if !got.mouseDown || got.mouseY == 0 {
					t.Fatalf("did not route around a mobile on the trail: %+v", got)
				}
				blockers = nil
				self.H = 20
				got = update(self, nil)
				if !got.mouseDown || got.mouseX != 80 || got.mouseY != 0 {
					t.Fatalf("did not resume recorded trail after blocker left: %+v", got)
				}
			case "wait":
				self.H, self.V = 80, 20
				got = update(self, nil)
				if !got.mouseDown || got.mouseX != 80 || got.mouseY != 116 {
					t.Fatalf("exit continuation wandered or stopped short: %+v", got)
				}
				self.V = 116
				for i := 0; i < 100; i++ {
					if update(self, nil).mouseDown {
						t.Fatal("wandered after reaching the end of the trail")
					}
				}
				if !update(self, &frameMobile{Index: 7, H: -100, V: 20}).mouseDown {
					t.Fatal("forgot target while waiting at end of trail")
				}
			case "manual":
				interruptScriptMovement(time.Now())
				sim.timers(t)
				scriptMovement.Lock()
				scriptMovement.manualUntil = time.Time{}
				scriptMovement.Unlock()
				if update(self, &frameMobile{Index: 2, H: 100}).mouseDown {
					t.Fatal("manual cancellation resumed pursuit")
				}
			case "fallen":
				if update(self, &frameMobile{Index: 2, H: 80, State: poseDead}).mouseDown {
					t.Fatal("pursued fallen leader")
				}
				if !update(self, &frameMobile{Index: 2, H: 100}).mouseDown {
					t.Fatal("did not resume after target stood up")
				}
			case "self-fallen":
				self.State = poseDead
				if update(self, nil).mouseDown {
					t.Fatal("moved while fallen")
				}
				self.State = 0
				if !update(self, &frameMobile{Index: 2, H: 100}).mouseDown {
					t.Fatal("did not resume after self stood up")
				}
			case "frame-reset":
				frame = 0
				if update(self, nil).mouseDown {
					t.Fatal("pursued across frame reset")
				}
				if !update(self, &frameMobile{Index: 2, H: 100}).mouseDown {
					t.Fatal("did not resume after frame reset")
				}
			}
		})
	}
}

func TestFollowTrailSceneryRegistration(t *testing.T) {
	src, err := scriptScripts.ReadFile("script_library/follow_player.go")
	if err != nil {
		t.Fatal(err)
	}
	const owner = "follow_registration"
	grantScriptPermissionsForTest(t, owner)
	prepared, err := compileScriptSource(owner, src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	defer disposePreparedScript(prepared)
	value, err := prepared.interpreter.Eval("trailShift")
	if err != nil {
		t.Fatal(err)
	}
	shift := value.Interface().(func(scriptapi.World, scriptapi.World) (float64, float64, bool))
	before := scriptapi.World{Frame: 10, Pictures: []scriptapi.Picture{
		{PictID: 1, Background: true}, {PictID: 2, H: 100},
		{PictID: 3, Moving: true}, {PictID: 4, Shadow: true},
		{PictID: 5, H: 40}, {PictID: 5, H: 80}, // Repeated tiles cannot anchor.
	}}
	after := before
	after.Frame = 13 // A skipped callback requires cumulative scenery registration.
	after.Pictures = append([]scriptapi.Picture(nil), before.Pictures...)
	after.Pictures[0].H, after.Pictures[1].H = -30, 70
	after.CameraShiftX = -10 // Only one frame's shift; must not use this.
	if x, y, ok := shift(before, after); !ok || x != -30 || y != 0 {
		t.Fatalf("cumulative scenery shift = (%v, %v, %v)", x, y, ok)
	}
	after.Pictures = []scriptapi.Picture{{PictID: 9}, {PictID: 10}}
	if _, _, ok := shift(before, after); ok {
		t.Fatal("unrelated scenery retained old track")
	}
	before.Pictures = []scriptapi.Picture{{PictID: 1, Background: true}}
	after.Pictures = []scriptapi.Picture{{PictID: 2, Background: true}}
	after.Frame = 11
	if _, _, ok := shift(before, after); ok {
		t.Fatal("replacement background retained old track")
	}
	before.Pictures, after.Pictures = nil, nil
	after.Frame = 13
	if _, _, ok := shift(before, after); ok {
		t.Fatal("applied single-frame camera estimate across skipped frames")
	}
}

func TestFollowBreadcrumbOverlayPreference(t *testing.T) {
	initFont()
	isolateScriptWorld(t)
	const owner = "follow_breadcrumb_overlay"
	sim := activateBundledProofScript(t, owner, "follow_player.go")
	sim.login(t, "Hero")
	base := time.Now()
	pictures := []framePicture{{PictID: 100, H: -200, Background: true}, {PictID: 101, H: -100}}
	update := func(frame int, leaderH int16) {
		stateMu.Lock()
		state.descriptors = map[uint8]frameDescriptor{
			1: {Index: 1, Name: "Hero", Type: kDescPlayer},
			2: {Index: 2, Name: "Leader", Type: kDescPlayer},
		}
		state.liveMobs = []frameMobile{{Index: 1}, {Index: 2, H: leaderH}}
		state.pictures = append([]framePicture(nil), pictures...)
		state.logicalFrame, state.receivedAt = frame, base.Add(time.Duration(frame)*200*time.Millisecond)
		markWorldStateChanged()
		stateMu.Unlock()
		dispatchScriptChange(ChangeEvent{Type: ChangeWorld})
		sim.barrier(t)
	}
	overlay := func() []overlayOp {
		overlayMu.RLock()
		defer overlayMu.RUnlock()
		return append([]overlayOp(nil), scriptOverlayOps[owner]...)
	}

	update(1, 80)
	sim.command(t, "follow", "Leader")
	if ops := overlay(); len(ops) != 0 {
		t.Fatalf("breadcrumbs drawn by default: %+v", ops)
	}
	if !scriptSetConfigValue(owner, "draw-breadcrumbs", true) {
		t.Fatal("draw-breadcrumbs preference was not registered")
	}
	sim.barrier(t)
	if ops := overlay(); len(ops) != 1 || ops[0].x != gameAreaSizeX/2+80-3 || ops[0].y != gameAreaSizeY/2-3 || ops[0].w != 7 || ops[0].h != 7 {
		t.Fatalf("breadcrumb overlay = %+v", ops)
	}

	for i := range pictures {
		pictures[i].H -= 20
	}
	update(2, 60)
	if ops := overlay(); len(ops) != 1 || ops[0].x != gameAreaSizeX/2+60-3 {
		t.Fatalf("breadcrumb did not remain anchored to scenery: %+v", ops)
	}
	if !scriptSetConfigValue(owner, "draw-breadcrumbs", false) {
		t.Fatal("could not disable breadcrumb drawing")
	}
	sim.barrier(t)
	if ops := overlay(); len(ops) != 0 {
		t.Fatalf("disabling breadcrumbs left overlay ops: %+v", ops)
	}
	if !scriptSetConfigValue(owner, "draw-breadcrumbs", true) {
		t.Fatal("could not re-enable breadcrumb drawing")
	}
	sim.barrier(t)
	sim.command(t, "follow", "off")
	if ops := overlay(); len(ops) != 0 {
		t.Fatalf("stopping follow left breadcrumb overlay ops: %+v", ops)
	}
}

func TestFollowPlayerAltRightClick(t *testing.T) {
	initFont()
	isolateScriptWorld(t)
	const owner = "follow_click"
	sim := activateBundledProofScript(t, owner, "follow_player.go")
	sim.login(t, "Hero")
	oldSelected := selectedPlayerName
	selectedPlayerName = ""
	t.Cleanup(func() { selectedPlayerName = oldSelected })
	stateMu.Lock()
	state.descriptors = map[uint8]frameDescriptor{
		1: {Index: 1, Name: "Hero", Type: kDescPlayer},
		2: {Index: 2, Name: "Leader", Type: kDescPlayer},
		3: {Index: 3, Name: "Other", Type: kDescPlayer},
		4: {Index: 4, Name: "Rat", Type: kDescMonster},
		5: {Index: 5, Name: "Fallen", Type: kDescPlayer},
		6: {Index: 6, Name: "Gone", Type: kDescPlayer},
	}
	state.liveMobs = []frameMobile{{Index: 1}, {Index: 2, H: 100}, {Index: 3, H: -100}, {Index: 4, V: 100}, {Index: 5, V: -100, State: poseDead}, {Index: 6, H: 100, Persist: true}}
	state.logicalFrame, state.receivedAt = 1, time.Now()
	stateMu.Unlock()
	click := func(mobile Mobile, onMobile bool) bool {
		t.Helper()
		event := makeScriptInputEvent("Alt-RightClick")
		event.OnMobile, event.Mobile = onMobile, mobile
		passed := sim.input(t, event)
		sim.barrier(t)
		return passed
	}
	for _, mobile := range []Mobile{
		{Name: "Hero", Player: true, Self: true},
		{Name: "Rat"},
		{Name: "Fallen", Player: true, Dead: true},
		{Name: "Gone", Player: true, Stale: true},
		{Name: "Gone", Player: true}, // A click snapshot can outlive its mobile.
		{Name: "Missing", Player: true},
		{Player: true},
	} {
		if !click(mobile, true) || scriptMovementSnapshot(owner, time.Now()).Active {
			t.Fatalf("invalid follow target consumed input or started movement: %+v", mobile)
		}
	}
	if !click(Mobile{}, false) {
		t.Fatal("empty-ground click was consumed")
	}
	if click(Mobile{Index: 2, Name: "Leader display label", Player: true}, true) {
		t.Fatal("player follow did not consume right-click")
	}
	if moving := applyScriptMovement(inputState{}, time.Now()); !moving.mouseDown || moving.mouseX != 76 {
		t.Fatalf("click did not follow without a Players selection: %+v", moving)
	}
	if click(Mobile{Index: 3, Name: "Other", Player: true}, true) {
		t.Fatal("switching targets did not consume right-click")
	}
	if moving := applyScriptMovement(inputState{}, time.Now()); !moving.mouseDown || moving.mouseX != -76 {
		t.Fatalf("click did not switch to the other player: %+v", moving)
	}
	sim.command(t, "follow", "off")
	sim.command(t, "follow", " LEA ")
	if moving := applyScriptMovement(inputState{}, time.Now()); !moving.mouseDown || moving.mouseX != 76 {
		t.Fatalf("unique partial name did not start following: %+v", moving)
	}
	sim.command(t, "follow", "off")
	stateMu.Lock()
	descriptor := state.descriptors[3]
	descriptor.Name = "Leader Two"
	state.descriptors[3] = descriptor
	state.receivedAt = time.Now()
	stateMu.Unlock()
	sim.command(t, "follow", "lea")
	if scriptMovementSnapshot(owner, time.Now()).Active {
		t.Fatal("ambiguous partial name chose a player")
	}
	sim.command(t, "follow", "LEADER")
	if moving := applyScriptMovement(inputState{}, time.Now()); !moving.mouseDown || moving.mouseX != 76 {
		t.Fatalf("exact name did not take precedence over prefix match: %+v", moving)
	}
	interruptScriptMovement(time.Now())
	sim.timers(t)
	if scriptMovementSnapshot(owner, time.Now()).Active {
		t.Fatal("manual movement did not cancel click following")
	}
}
