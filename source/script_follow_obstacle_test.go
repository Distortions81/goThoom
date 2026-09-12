package main

import (
	"math"
	"testing"
	"time"
)

// The obstacle is deliberately absent from picture metadata. Only rejected
// movement reveals it. A scrolling camera must produce the same world route.
func TestFollowRoutesAroundUnmappedObstacle(t *testing.T) {
	initFont()
	for _, visible := range []bool{true, false} {
		var reference [][2]int16
		for _, scrolling := range []bool{false, true} {
			name := "breadcrumb"
			if visible {
				name = "player"
			}
			if scrolling {
				name += "-scrolling"
			}
			t.Run(name, func(t *testing.T) {
				isolateScriptWorld(t)
				const owner = "follow_obstacle"
				sim := activateBundledProofScript(t, owner, "follow_player.go")
				sim.login(t, "Hero")
				base := time.Now()
				frame := 0
				var x, y, oldCameraX, oldCameraY int16
				update := func(showLeader bool) inputState {
					frame++
					var cameraX, cameraY int16
					if scrolling {
						cameraX, cameraY = x, y
					}
					primarySession.draw.mu.Lock()
					primarySession.draw.current.descriptors = map[uint8]frameDescriptor{1: {Index: 1, Name: "Hero", Type: kDescPlayer}, 2: {Index: 2, Name: "Leader", Type: kDescPlayer}}
					primarySession.draw.current.liveMobs = []frameMobile{{Index: 1, H: x - cameraX, V: y - cameraY}}
					if showLeader {
						primarySession.draw.current.liveMobs = append(primarySession.draw.current.liveMobs, frameMobile{Index: 2, H: 160 - cameraX, V: -cameraY})
					}
					primarySession.draw.current.pictures = []framePicture{{PictID: 100, H: -200 - cameraX, V: -200 - cameraY, Background: true}, {PictID: 101, H: -150 - cameraX, V: -100 - cameraY}}
					primarySession.draw.current.logicalFrame, primarySession.draw.current.receivedAt = frame, base.Add(time.Duration(frame)*250*time.Millisecond)
					primarySession.draw.current.picShiftX, primarySession.draw.current.picShiftY = int(oldCameraX-cameraX), int(oldCameraY-cameraY)
					oldCameraX, oldCameraY = cameraX, cameraY
					markWorldStateChanged()
					primarySession.draw.mu.Unlock()
					dispatchScriptChange(ChangeEvent{Type: ChangeWorld})
					sim.barrier(t)
					return applyScriptMovement(inputState{}, time.Now())
				}
				update(true)
				sim.command(t, "follow", "Leader")
				reached, wentAround := false, false
				var route [][2]int16
				for i := 0; i < 100; i++ {
					input := update(visible)
					remaining := math.Hypot(160-float64(x), float64(y))
					if !input.mouseDown && ((visible && remaining <= 44) || (!visible && remaining <= 14)) {
						reached = true
						break
					}
					if input.mouseDown {
						dx, dy := float64(input.mouseX-x), float64(input.mouseY-y)
						if scrolling {
							dx, dy = float64(input.mouseX), float64(input.mouseY)
						}
						distance := math.Max(1, math.Hypot(dx, dy))
						nx, ny := x+int16(math.Round(dx/distance*6)), y+int16(math.Round(dy/distance*6))
						// The server refuses entry to a small obstacle at (40, 0).
						if math.Hypot(float64(nx)-40, float64(ny)) >= 20 {
							x, y = nx, ny
						}
					}
					wentAround = wentAround || math.Abs(float64(y)) >= 20
					route = append(route, [2]int16{x, y})
				}
				if !reached || !wentAround {
					t.Fatalf("failed to detour and resume destination: reached=%v around=%v at=(%d,%d), route=%v", reached, wentAround, x, y, route)
				}
				if !scrolling {
					reference = route
				} else {
					if len(route) != len(reference) {
						t.Fatalf("scrolling changed route length: %d vs %d", len(route), len(reference))
					}
					for i := range route {
						if route[i] != reference[i] {
							t.Fatalf("world anchoring drifted at step %d: %v vs %v", i, route[i], reference[i])
						}
					}
				}
			})
		}
	}
}
