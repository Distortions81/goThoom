package main

import (
	"math"
	"testing"
	"time"

	scriptapi "gt2"
)

func isolateScriptWorld(t *testing.T) {
	t.Helper()
	primarySession.draw.mu.Lock()
	oldState, oldIndex := primarySession.draw.current, playerIndex
	primarySession.draw.current = drawState{}
	playerIndex = 1
	primarySession.draw.mu.Unlock()
	oldImages := clImages
	clImages = nil
	oldGeneration := primarySession.draw.generation.Load()
	stopScriptMovement("")
	scriptMovement.Lock()
	scriptMovement.manualUntil, scriptMovement.manualAt = time.Time{}, time.Time{}
	scriptMovement.Unlock()
	t.Cleanup(func() {
		stopScriptMovement("")
		primarySession.draw.mu.Lock()
		primarySession.draw.current, playerIndex = oldState, oldIndex
		primarySession.draw.mu.Unlock()
		clImages = oldImages
		primarySession.draw.generation.Store(oldGeneration)
	})
}

func TestScriptWorldSnapshots(t *testing.T) {
	isolateScriptWorld(t)
	now := time.Now()
	primarySession.draw.mu.Lock()
	primarySession.draw.current = drawState{
		descriptors:  map[uint8]frameDescriptor{1: {Index: 1, Name: "Hero", Type: kDescPlayer, Plane: 2}, 2: {Index: 2, Name: "Friend", Type: kDescPlayer}},
		liveMobs:     []frameMobile{{Index: 1, H: 3, V: 4}, {Index: 2, State: poseDead, Persist: true}},
		pictures:     []framePicture{{PictID: 123, H: -40, V: -50, Plane: -1, Again: true}},
		logicalFrame: 17, receivedAt: now, picShiftX: -6, picShiftY: 2, lightingFlags: 3,
	}
	markWorldStateChanged()
	primarySession.draw.mu.Unlock()
	world := scriptCurrentWorld()
	if !world.HasSelf || world.Self.Name != "Hero" || world.Self.Plane != 2 || world.Frame != 17 || world.ReceivedAt != now || world.CameraShiftX != -6 || world.Lighting != 3 {
		t.Fatalf("world snapshot = %+v", world)
	}
	if !world.Mobiles[1].Dead || !world.Mobiles[1].Stale || !world.Pictures[0].Reused || world.Pictures[0].H != -40 {
		t.Fatalf("incomplete scene data: %+v", world)
	}
	world.Mobiles[0].Name = "mutated"
	world.Pictures[0].H = 999
	next := scriptCurrentWorld()
	if next.Self.Name != "Hero" || next.Pictures[0].H != -40 {
		t.Fatal("snapshot aliases client state")
	}
	primarySession.draw.mu.Lock()
	primarySession.draw.current.liveMobs[0].Persist = true
	primarySession.draw.mu.Unlock()
	if scriptCurrentWorld().HasSelf {
		t.Fatal("retained self reported as fresh")
	}
}

func TestScriptMovementLease(t *testing.T) {
	const owner = "movement_test"
	isolateScriptWorld(t)
	resetScriptCallbackTestState(t, owner)
	t.Cleanup(func() { disablescript(owner, "test cleanup") })
	now := time.Now()
	primarySession.draw.mu.Lock()
	primarySession.draw.current.receivedAt = now
	primarySession.draw.mu.Unlock()
	if scriptMove(owner, 100, 0, now) {
		t.Fatal("movement accepted without session")
	}
	scriptSessionLogin("Hero")
	if !scriptMove(owner, 32000, -32000, now) {
		t.Fatal("fresh movement rejected")
	}
	got := applyScriptMovement(inputState{}, now)
	if !got.mouseDown || got.mouseX != int16(fieldCenterX) || got.mouseY != -int16(fieldCenterY) {
		t.Fatalf("unclamped input: %+v", got)
	}
	if applyScriptMovement(inputState{}, now.Add(scriptMovementLease)).mouseDown {
		t.Fatal("expired movement continued")
	}
	if !scriptMove(owner, 80, 5, now) {
		t.Fatal("renewal failed")
	}
	stopScriptMovement("other")
	if !applyScriptMovement(inputState{}, now).mouseDown {
		t.Fatal("another script stopped our movement")
	}
	manual := inputState{mouseX: -70, mouseDown: true}
	if got := applyScriptMovement(manual, now); got != manual {
		t.Fatal("script overrode manual input")
	}
	if scriptMove(owner, 80, 0, now.Add(100*time.Millisecond)) {
		t.Fatal("script reacquired movement during manual grace period")
	}
	if !scriptMovementSnapshot(owner, now).LastManualInput.Equal(now) {
		t.Fatal("manual input not observable")
	}
	now = now.Add(2 * time.Second)
	primarySession.draw.mu.Lock()
	primarySession.draw.current.receivedAt = now
	primarySession.draw.mu.Unlock()
	if !scriptMove(owner, 80, 0, now) {
		t.Fatal("movement did not recover after manual grace period")
	}
	scriptSessionLogout("Hero")
	if applyScriptMovement(inputState{}, now).mouseDown {
		t.Fatal("logout left movement active")
	}
	scriptSessionLogin("Hero")
	if !scriptMove(owner, 80, 0, now) {
		t.Fatal("movement after login failed")
	}
	// Reloading the owner's callback queue invalidates its lease even before cleanup.
	stopScriptEventQueue(owner)
	startScriptEventQueue(owner)
	if applyScriptMovement(inputState{}, now).mouseDown {
		t.Fatal("old queue retained movement after reload")
	}
	primarySession.draw.mu.Lock()
	primarySession.draw.current.receivedAt = now.Add(-2 * time.Second)
	primarySession.draw.mu.Unlock()
	if scriptMove(owner, 80, 0, now) {
		t.Fatal("stale frame permitted movement")
	}
}

func TestFollowPlayerProof(t *testing.T) {
	initFont()
	for _, scenario := range []string{"hysteresis", "blocked", "camera", "manual", "yield", "lost", "stale", "wiggle", "wiggle-lateral-release", "near-still", "camera-only-stuck", "lateral", "retreat", "scenery", "avoid-mobile"} {
		t.Run(scenario, func(t *testing.T) {
			isolateScriptWorld(t)
			const owner = "follow_proof"
			sim := activateBundledProofScript(t, owner, "follow_player.go")
			sim.login(t, "Hero")
			base := time.Now()
			frame := 0
			var blockers []frameMobile
			cameraY := 0
			var selfH int16
			update := func(distance int16, camera int, visible bool) {
				frame++
				primarySession.draw.mu.Lock()
				primarySession.draw.current.descriptors = map[uint8]frameDescriptor{1: {Index: 1, Name: "Hero", Type: kDescPlayer}, 2: {Index: 2, Name: "Leader", Type: kDescPlayer}}
				primarySession.draw.current.liveMobs = []frameMobile{{Index: 1, H: selfH}}
				if visible {
					primarySession.draw.current.liveMobs = append(primarySession.draw.current.liveMobs, frameMobile{Index: 2, H: distance})
				}
				for _, blocker := range blockers {
					primarySession.draw.current.descriptors[blocker.Index] = frameDescriptor{Index: blocker.Index, Name: "Bystander", Type: kDescPlayer}
					primarySession.draw.current.liveMobs = append(primarySession.draw.current.liveMobs, blocker)
				}
				primarySession.draw.current.logicalFrame, primarySession.draw.current.receivedAt, primarySession.draw.current.picShiftX = frame, base.Add(time.Duration(frame)*250*time.Millisecond), camera
				primarySession.draw.current.picShiftY = cameraY
				markWorldStateChanged()
				primarySession.draw.mu.Unlock()
				dispatchScriptChange(ChangeEvent{Type: ChangeWorld})
				sim.barrier(t)
			}
			moving := func() inputState { return applyScriptMovement(inputState{}, time.Now()) }
			update(100, 0, true)
			sim.command(t, "follow", "Leader")
			if !moving().mouseDown {
				t.Fatal("follow did not begin")
			}
			if moving().mouseX != 76 || moving().mouseY != 0 {
				t.Fatalf("mouse should aim 24 pixels behind the target: %+v", moving())
			}
			switch scenario {
			case "hysteresis":
				update(40, 0, true)
				if moving().mouseDown {
					t.Fatal("did not stop at inner threshold")
				}
				update(60, 0, true)
				if moving().mouseDown {
					t.Fatal("restarted inside hysteresis band")
				}
				update(90, 0, true)
				if !moving().mouseDown {
					t.Fatal("did not restart at outer threshold")
				}
			case "blocked":
				for i := 0; i < 5; i++ {
					update(100, 0, true)
				}
				if moving().mouseY == 0 {
					t.Fatal("stalled follower did not attempt a detour")
				}
				for i := 0; i < 30; i++ {
					update(100, 0, true)
				}
				if !moving().mouseDown {
					t.Fatal("blocked follower gave up instead of retrying its route")
				}
				update(-100, 0, true)
				if !moving().mouseDown || moving().mouseX >= 0 {
					t.Fatal("blocked follower forgot target after route changed")
				}
			case "scenery":
				// Deliver a world snapshot with explicit artwork dimensions, as the
				// client supplies after looking up its image metadata.
				world := scriptCurrentWorld()
				world.Frame++
				world.ReceivedAt = world.ReceivedAt.Add(250 * time.Millisecond)
				world.Pictures = []scriptapi.Picture{{PictID: 100, H: 20, V: -64, Width: 40, Height: 64}}
				queue := currentScriptEventQueue(owner)
				value, err := queue.interpreter.Eval("followWorld")
				if err != nil {
					t.Fatal(err)
				}
				handler := value.Interface().(func(scriptapi.World))
				if !queueScriptCallbackWaitOn(queue, owner, "Test scenery", func() { handler(world) }) {
					t.Fatal("scenery callback did not run")
				}
				sim.barrier(t)
				if !moving().mouseDown || moving().mouseX <= 0 || moving().mouseY == 0 {
					t.Fatalf("did not route around nearby scenery before becoming stuck: %+v", moving())
				}
			case "avoid-mobile":
				blockers = []frameMobile{{Index: 3, H: 40}}
				update(100, -6, true) // Moving normally, with no stall or large gap.
				if moving().mouseY == 0 {
					t.Fatal("did not route around a mobile during ordinary following")
				}
				firstSide := moving().mouseY
				update(100, -6, true)
				if moving().mouseY*firstSide <= 0 {
					t.Fatal("changed sides without a new obstacle")
				}
				blockers = nil
				update(100, -6, true)
				if moving().mouseY != 0 {
					t.Fatal("did not resume direct following after path cleared")
				}
			case "lateral", "retreat":
				cameraX := 0
				if scenario == "lateral" {
					cameraY = -6
				} else {
					cameraX = 6
				}
				for i := 0; i < 3; i++ {
					update(100, cameraX, true)
				}
				if !moving().mouseDown || moving().mouseY == 0 {
					t.Fatalf("failed progress along attempted heading did not trigger a detour: %+v", moving())
				}
			case "camera-only-stuck":
				for i := 0; i < 4; i++ {
					selfH += 6 // Self shifts with the background: no actual travel.
					update(100, 6, true)
				}
				if !moving().mouseDown || moving().mouseY == 0 {
					t.Fatalf("background scrolling concealed a stationary character: %+v", moving())
				}
			case "near-still":
				for i := 0; i < 4; i++ {
					cameraY = 1 - 2*(i%2)
					update(100, 0, true)
				}
				if !moving().mouseDown || moving().mouseY == 0 {
					t.Fatalf("near-zero motion did not trigger an obstacle detour: %+v", moving())
				}
			case "wiggle", "wiggle-lateral-release":
				activity := func() string {
					value, err := currentScriptEventQueue(owner).interpreter.Eval("followActivity")
					if err != nil {
						t.Fatal(err)
					}
					return value.Interface().(string)
				}
				for i := 0; i < 4; i++ {
					update(100, 0, true)
				}
				if activity() != "Routing" {
					t.Fatalf("did not try routing before wiggle: %s", activity())
				}
				for i := 0; i < 5; i++ {
					update(100, 0, true)
				}
				if activity() != "Wiggling" {
					t.Fatalf("failed detours with no movement did not permit recovery: %s", activity())
				}
				if scenario == "wiggle-lateral-release" {
					cameraY = -6
					update(100, 0, true)
				} else {
					update(100, -8, true)
				}
				if activity() == "Wiggling" {
					t.Fatal("wiggle continued after movement resumed")
				}

			case "camera":
				for i := 0; i < 12; i++ {
					update(100, -6, true)
				}
				if !moving().mouseDown || moving().mouseY != 0 {
					t.Fatalf("camera motion mistaken for stall: %+v", moving())
				}
			case "manual":
				update(40, 0, true) // Manual input must cancel even while resting.
				interruptScriptMovement(time.Now())
				sim.timers(t)
				scriptMovement.Lock()
				scriptMovement.manualUntil = time.Time{}
				scriptMovement.Unlock()
				update(100, 0, true)
				if moving().mouseDown {
					t.Fatal("follow resumed after manual cancellation")
				}
			case "yield":
				update(10, 0, true)
				if !moving().mouseDown || moving().mouseX >= 0 {
					t.Fatalf("did not give leader space: %+v", moving())
				}
			case "lost":
				update(100, 0, false)
				if !moving().mouseDown || moving().mouseX != 100 {
					t.Fatalf("did not pursue last known position: %+v", moving())
				}
				update(100, 0, true)
				if !moving().mouseDown || moving().mouseX != 76 {
					t.Fatal("did not resume visible following after reacquiring target")
				}
			case "stale":
				primarySession.draw.mu.Lock()
				primarySession.draw.current.receivedAt = time.Now().Add(-2 * time.Second)
				primarySession.draw.mu.Unlock()
				sim.timers(t)
				if moving().mouseDown {
					t.Fatal("watchdog left stale movement active")
				}
				update(100, 0, true)
				if !moving().mouseDown {
					t.Fatal("did not resume after world updates returned")
				}
			}
		})
	}
}

func TestFollowSceneryHints(t *testing.T) {
	src, err := scriptScripts.ReadFile("script_library/follow_player.go")
	if err != nil {
		t.Fatal(err)
	}
	grantScriptPermissionsForTest(t, "follow_path_test")
	prepared, err := compileScriptSource("follow_path_test", src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	defer disposePreparedScript(prepared)
	value, err := prepared.interpreter.Eval("sceneryPathPenalty")
	if err != nil {
		t.Fatal(err)
	}
	penalty := value.Interface().(func(scriptapi.World, float64, float64) float64)
	world := scriptapi.World{Self: scriptapi.Mobile{Index: 1}, Pictures: []scriptapi.Picture{{H: 20, V: -64, Width: 40, Height: 64}}}
	if penalty(world, 0, 64) <= penalty(world, math.Pi/2, 64) {
		t.Fatal("path through sprite base not penalized")
	}
	world.Pictures[0].Plane = -1
	if penalty(world, 0, 64) != 0 {
		t.Fatal("ground artwork treated as obstacle")
	}
	world.Pictures[0].Plane = 0
	world.Pictures[0].Shadow = true
	if penalty(world, 0, 64) != 0 {
		t.Fatal("shadow treated as obstacle")
	}
}

func TestFollowMobileClearance(t *testing.T) {
	src, err := scriptScripts.ReadFile("script_library/follow_player.go")
	if err != nil {
		t.Fatal(err)
	}
	grantScriptPermissionsForTest(t, "follow_clearance")
	prepared, err := compileScriptSource("follow_clearance", src, restrictedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	defer disposePreparedScript(prepared)
	value, err := prepared.interpreter.Eval("mobilePathPenalty")
	if err != nil {
		t.Fatal(err)
	}
	penalty := value.Interface().(func(scriptapi.World, scriptapi.Mobile, float64, float64) (float64, bool))
	value, err = prepared.interpreter.Eval("routeDirection")
	if err != nil {
		t.Fatal(err)
	}
	route := value.Interface().(func(scriptapi.World, scriptapi.Mobile, float64, float64, bool) (float64, bool))
	leader := scriptapi.Mobile{Index: 2, H: 100}
	world := scriptapi.World{Self: scriptapi.Mobile{Index: 1, Self: true}, Mobiles: []scriptapi.Mobile{{Index: 3, H: 40, V: 26}}}
	cost, clear := penalty(world, leader, 0, 76)
	if !clear || cost <= 0 {
		t.Fatal("comfortable spacing should be a soft cost beyond the minimum")
	}
	heading, clear := route(world, leader, 0, 76, false)
	if !clear || heading >= 0 {
		t.Fatal("did not give a nearby mobile extra clearance")
	}
	world.Mobiles[0].V = 0
	if _, clear := penalty(world, leader, 0, 76); clear {
		t.Fatal("segment through a mobile accepted because its endpoint was clear")
	}
	world.Mobiles[0].Dead = true
	if heading, clear := route(world, leader, 0, 76, false); !clear || heading != 0 {
		t.Fatal("fallen mobile blocked route")
	}
	world.Mobiles[0].Dead = false
	world.Mobiles[0].Stale = true
	if heading, clear := route(world, leader, 0, 76, false); !clear || heading != 0 {
		t.Fatal("retained edge mobile blocked route")
	}
	world.Mobiles = []scriptapi.Mobile{{Index: 3, H: 10}}
	if _, clear := penalty(world, leader, math.Pi, 65); !clear {
		t.Fatal("escape from an existing overlap was rejected")
	}
	if _, clear := penalty(world, leader, 0, 65); clear {
		t.Fatal("movement deeper into overlap was accepted")
	}
	world.Mobiles = []scriptapi.Mobile{{Index: 3, H: 10}, {Index: 4, H: -10}, {Index: 5, V: 10}, {Index: 6, V: -10}}
	if _, clear := route(world, leader, 0, 65, false); clear {
		t.Fatal("surrounded follower should wait for an opening")
	}
}
