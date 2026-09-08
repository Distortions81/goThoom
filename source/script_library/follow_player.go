//go:build script

package main

import (
	"gt2"
	"math"
	"strings"
	"time"
)

const scriptID = "example-follow-player"
const scriptName = "Follow Player"
const scriptAuthor = "goThoom"
const scriptCategory = "Movement"
const scriptDescription = "Follow a visible player with mobile clearance, local routing and wiggle recovery. /follow name; /follow off."
const scriptAPIVersion = 2

var targetName string
var followWindow gt2.Window
var followActivity = "Stopped"
var followNote = "Select a visible player in Players, then press Follow."
var manualAt time.Time
var following bool
var previous gt2.World
var previousDistance float64
var stalled time.Duration
var wiggleStarted time.Time
var wiggleNext time.Time
var wiggleSide = 1.0
var detourSide = 1.0
var startDistance = 72.0
var stopDistance = 44.0
var sceneryHints = true
var mobileClearance = 34.0

func Init() {
	startDistance = gt2.Decimal(gt2.DecimalOption{Key: "start", Label: "Start following distance", Default: 72, Min: 60, Max: 180, Step: 4, OnChange: func(v float64) { startDistance = v }})
	stopDistance = gt2.Decimal(gt2.DecimalOption{Key: "stop", Label: "Stop following distance", Default: 44, Min: 24, Max: 56, Step: 4, OnChange: func(v float64) { stopDistance = v }})
	mobileClearance = gt2.Decimal(gt2.DecimalOption{Key: "clearance", Label: "Preferred mobile clearance", Help: "A soft spacing preference; tight passages may require less room.", Default: 34, Min: 24, Max: 60, Step: 2, OnChange: func(v float64) { mobileClearance = v }})
	sceneryHints = gt2.Bool(gt2.BoolOption{Key: "scenery", Label: "Use scenery hints for detours", Help: "Artwork planes are draw order, not collision geometry. Disable if scenery causes unnecessary detours.", Default: true, OnChange: func(v bool) { sceneryHints = v }})
	followWindow = gt2.CreateWindow(gt2.WindowOptions{
		Title: "Follow Player", Width: 360,
		Text: "Selected: None\nFollowing: None\nStatus: Stopped\nSelect a visible player in Players, then press Follow.",
		Buttons: []gt2.WindowButton{
			{ID: "follow", Label: "Follow", Tooltip: "Follow the player selected in Players", Disabled: true, OnClick: func() { startFollow("") }},
			{ID: "stop", Label: "Stop Follow", Tooltip: "Stop following immediately", Disabled: true, OnClick: func() { stopFollow("Follow stopped.") }},
		},
		OnClose: func() { stopFollow("Follow stopped: window closed.") },
	})
	gt2.Command("follow", startFollow)
	gt2.Command("stopfollow", func(args string) { stopFollow("Follow stopped.") })
	gt2.Command("followui", func(args string) { followWindow.Show(); refreshFollowWindow() })
	gt2.OnChange(gt2.ChangeSelectedPlayer, func(event gt2.ChangeEvent) { refreshFollowWindow() })

	gt2.OnWorld(followWorld)
	gt2.Repeat(200*time.Millisecond, func() {
		defer refreshFollowWindow()
		if targetName != "" && gt2.Movement().LastManualInput.After(manualAt) {
			stopFollow("Follow stopped: manual movement.")
			return
		}
		if targetName != "" && time.Since(gt2.CurrentWorld().ReceivedAt) > time.Second {
			stopFollow("Follow stopped: world updates lost.")
		}
	})
	gt2.OnLogout(func(event gt2.LifecycleEvent) { stopFollow("") })
	gt2.OnCharacterChange(func(event gt2.LifecycleEvent) { stopFollow("") })
}

func startFollow(args string) {
	defer refreshFollowWindow()
	name := strings.TrimSpace(args)
	if strings.EqualFold(name, "off") {
		stopFollow("Follow stopped.")
		return
	}
	if name == "" {
		p, ok := gt2.SelectedPlayer()
		if ok {
			name = p.Name
		}
	}
	if name == "" {
		followNote = "Select a player in Players or use /follow Player Name."
		gt2.Print(followNote)
		return
	}
	world := gt2.CurrentWorld()
	_, ok := findLeader(world, name)
	if !ok {
		followNote = "Choose another player who is visible and standing."
		gt2.Print(followNote)
		return
	}
	gt2.StopMoving()
	targetName = name
	followActivity = "Following"
	followNote = ""
	followWindow.Show()
	following = false
	previous = gt2.World{}
	manualAt = gt2.Movement().LastManualInput
	detourSide = 1
	wiggleSide = -1
	stalled = 0
	wiggleStarted = time.Time{}
	wiggleNext = time.Time{}
	gt2.Print("Following " + name + ". Move manually or use /follow off to stop.")
	followWorld(world)
}

// The window and every status/control decision live entirely in this script.
func refreshFollowWindow() {
	selected := "None"
	canFollow := false
	world := gt2.CurrentWorld()
	if p, ok := gt2.SelectedPlayer(); ok {
		selected = p.Name
		_, visible := findLeader(world, p.Name)
		canFollow = visible && world.HasSelf && !world.Self.Dead && time.Since(world.ReceivedAt) < time.Second
	}
	target := targetName
	if target == "" {
		target = "None"
	}
	status := "Selected: " + selected + "\nFollowing: " + target + "\nStatus: " + followActivity
	if followNote != "" {
		status += "\n" + followNote
	}
	followWindow.SetText(status)
	followWindow.SetButtonEnabled("follow", canFollow)
	followWindow.SetButtonEnabled("stop", targetName != "")
}

func stopFollow(message string) {
	followActivity = "Stopped"
	followNote = message
	defer refreshFollowWindow()
	targetName = ""
	following = false
	previous = gt2.World{}
	stalled = 0
	wiggleStarted = time.Time{}
	wiggleNext = time.Time{}
	gt2.StopMoving()
	if message != "" {
		gt2.Print(message)
	}
}

func findLeader(world gt2.World, name string) (gt2.Mobile, bool) {
	for _, m := range world.Mobiles {
		if m.Player && !m.Self && !m.Dead && !m.Stale && strings.EqualFold(strings.ReplaceAll(m.Name, " ", ""), strings.ReplaceAll(name, " ", "")) {
			return m, true
		}
	}
	return gt2.Mobile{}, false
}

func followWorld(world gt2.World) {
	defer refreshFollowWindow()
	if targetName == "" {
		return
	}
	if gt2.Movement().LastManualInput.After(manualAt) {
		stopFollow("Follow stopped: manual movement.")
		return
	}
	if !world.HasSelf || world.Self.Dead || time.Since(world.ReceivedAt) > time.Second {
		stopFollow("Follow stopped: character or world unavailable.")
		return
	}
	leader, ok := findLeader(world, targetName)
	if !ok {
		stopFollow("Follow stopped: target left view.")
		return
	}
	if previous.Frame != 0 && (world.Frame < previous.Frame || world.Location != previous.Location) {
		stopFollow("Follow stopped: scene changed.")
		return
	}
	if world.Frame == previous.Frame && !previous.ReceivedAt.IsZero() {
		return
	}
	dx, dy := float64(leader.H-world.Self.H), float64(leader.V-world.Self.V)
	distance := math.Hypot(dx, dy)
	if following {
		following = distance > stopDistance
	} else {
		following = distance > startDistance
	}
	// Give nearby standing mobiles space even inside the resting distance band.
	// Use a smaller emergency radius while following; normal routing supplies
	// the wider, soft clearance without repeatedly abandoning the target.
	yieldX, yieldY := 0.0, 0.0
	for _, m := range world.Mobiles {
		if m.Self || m.Dead || m.Stale {
			continue
		}
		x, y := float64(world.Self.H-m.H), float64(world.Self.V-m.V)
		d := math.Hypot(x, y)
		radius := 22.0
		if !following && m.Index != leader.Index {
			radius = mobileClearance
		}
		if d < radius {
			if d < 1 {
				x, y, d = 0, -1, 1
			}
			yieldX += x / d * (radius - d)
			yieldY += y / d * (radius - d)
		}
	}
	yielding := math.Hypot(yieldX, yieldY) > 1
	if !following && !yielding {
		followActivity = "Staying"
		gt2.StopMoving()
		stalled = 0
		wiggleStarted = time.Time{}
		wiggleNext = time.Time{}
		previous = world
		previousDistance = distance
		return
	}
	now := world.ReceivedAt
	wiggling := !wiggleStarted.IsZero() && now.Sub(wiggleStarted) < 1400*time.Millisecond
	// Subtract camera motion and measure forward progress. Lateral motion during
	// a wiggle must not reset the retry budget while still caught on an object.
	if previous.HasSelf && world.Frame == previous.Frame+1 {
		elapsed := now.Sub(previous.ReceivedAt)
		travelX := float64(int(world.Self.H) - int(previous.Self.H) - world.CameraShiftX)
		travelY := float64(int(world.Self.V) - int(previous.Self.V) - world.CameraShiftY)
		forward := (travelX*dx + travelY*dy) / math.Max(1, distance)
		if yielding {
			forward = (travelX*yieldX + travelY*yieldY) / math.Max(1, math.Hypot(yieldX, yieldY))
		}
		if elapsed > 0 && elapsed < time.Second {
			if wiggling || (forward < 2 && distance >= previousDistance-2) {
				stalled += elapsed
			} else {
				stalled = 0
				wiggleStarted = time.Time{}
			}
		}
	} else {
		// CameraShift describes only the preceding frame. Never infer progress
		// across skipped frames; cancel an incomplete recovery on a gap.
		stalled = 0
		wiggleStarted = time.Time{}
		wiggling = false
	}
	if stalled > 8*time.Second {
		stopFollow("Follow stopped: blocked. Reposition and /follow again.")
		return
	}
	direction := math.Atan2(dy, dx)
	if yielding {
		direction = math.Atan2(yieldY, yieldX)
	}
	if stalled > 750*time.Millisecond && !wiggling && !now.Before(wiggleNext) {
		wiggleStarted = now
		wiggleNext = now.Add(2 * time.Second)
		wiggleSide = -wiggleSide
		wiggling = true
	}
	// Normally aim 24 world pixels short of the leader. Mouse distance controls
	// speed naturally. Only nearby obstacles or a recovery change that heading.
	reach := math.Max(0, distance-24)
	if yielding {
		reach = 65
	}
	if wiggling {
		direction += wiggleOffset(now.Sub(wiggleStarted), wiggleSide)
		reach = 65
	}
	checkScenery := stalled > 750*time.Millisecond || distance > startDistance+40
	heading, clear := routeDirection(world, leader, direction, reach, checkScenery)
	if !clear {
		followActivity = "Waiting for space"
		// Wait when every local exit would move closer to a mobile already inside
		// the minimum spacing. Keep observing so an opening can be used next frame.
		gt2.StopMoving()
		previous = world
		previousDistance = distance
		return
	}
	if math.Abs(heading-direction) > 0.01 {
		reach = math.Min(100, math.Max(65, reach))
	}
	followActivity = "Following"
	if math.Abs(heading-direction) > 0.01 {
		followActivity = "Routing"
	}
	if yielding {
		followActivity = "Giving space"
	}
	if wiggling {
		followActivity = "Wiggling"
	}
	direction = heading
	x := float64(world.Self.H) + math.Cos(direction)*reach
	y := float64(world.Self.V) + math.Sin(direction)*reach
	if !gt2.Move(int16(x), int16(y)) {
		stopFollow("Follow stopped: movement overridden or unavailable.")
		return
	}
	previous = world
	previousDistance = distance
}

// Four short pulses pull away, rock to the other side, then try forward again.
// Timed phases keep packet/frame rate from turning the wiggle into rapid jitter.
func wiggleOffset(elapsed time.Duration, side float64) float64 {
	phase := int(elapsed / (350 * time.Millisecond))
	if phase == 0 {
		return side * 2.2
	}
	if phase == 1 {
		return -side * 2.2
	}
	if phase == 2 {
		return side * 0.9
	}
	return -side * 0.9
}

// Route on every fresh frame, including normal following and recovery. Clearance
// is a preference outside an 18-pixel minimum; sprite size is not collision size.
func routeDirection(world gt2.World, leader gt2.Mobile, desired, reach float64, scenery bool) (float64, bool) {
	lookahead := math.Min(96, math.Max(16, reach))
	best := desired
	bestOffset := 0.0
	bestScore := math.Inf(1)
	found := false
	angles := []float64{0, detourSide * 0.4, detourSide * 0.8, detourSide * 1.2, detourSide * 1.6, -detourSide * 0.4, -detourSide * 0.8, -detourSide * 1.2, -detourSide * 1.6, detourSide * 2.2, -detourSide * 2.2, math.Pi}
	for _, offset := range angles {
		angle := desired + offset
		penalty, clear := mobilePathPenalty(world, leader, angle, lookahead)
		if !clear {
			continue
		}
		score := penalty + math.Abs(offset)*12
		if offset*detourSide < 0 {
			score += 5
		}
		if scenery && sceneryHints {
			score += sceneryPathPenalty(world, angle, lookahead)
		}
		if score < bestScore {
			best = angle
			bestOffset = offset
			bestScore = score
			found = true
		}
	}
	if found && bestOffset != 0 {
		detourSide = math.Copysign(1, bestOffset)
	}
	return best, found
}

// Check the whole segment, not just its endpoint. Mobiles already too close
// may be escaped, but a candidate may not reduce that existing separation.
func mobilePathPenalty(world gt2.World, leader gt2.Mobile, angle, reach float64) (float64, bool) {
	penalty := 0.0
	ux, uy := math.Cos(angle), math.Sin(angle)
	for _, m := range world.Mobiles {
		if m.Self || m.Dead || m.Stale {
			continue
		}
		x, y := float64(int(m.H)-int(world.Self.H)), float64(int(m.V)-int(world.Self.V))
		start := math.Hypot(x, y)
		along := math.Max(0, math.Min(reach, x*ux+y*uy))
		nearest := math.Hypot(x-ux*along, y-uy*along)
		if nearest < math.Min(18, start)-0.1 {
			return 0, false
		}
		clearance := mobileClearance
		if m.Index == leader.Index {
			clearance = 24
		}
		if nearest < clearance {
			penalty += 70 * (clearance - nearest) / clearance
		}
		if start < clearance {
			end := math.Hypot(x-ux*reach, y-uy*reach)
			penalty += 3 * math.Max(0, clearance-end)
		}
	}
	return penalty, true
}

// Scenery remains a soft hint: the lower middle of upright artwork is only
// an estimate of its footprint, not authoritative server collision geometry.
func sceneryPathPenalty(world gt2.World, angle, reach float64) float64 {
	penalty := 0.0
	for step := 8.0; step <= reach; step += 8 {
		x := float64(world.Self.H) + math.Cos(angle)*step
		y := float64(world.Self.V) + math.Sin(angle)*step
		for _, p := range world.Pictures {
			if p.Plane != 0 || p.Moving || p.Shadow || p.Width <= 0 || p.Height <= 0 || p.Width > 240 || p.Height > 240 {
				continue
			}
			left := float64(p.H) + float64(p.Width)*0.25 - 8
			right := float64(p.H) + float64(p.Width)*0.75 + 8
			top := float64(p.V) + float64(p.Height)*0.75 - 8
			bottom := float64(p.V) + float64(p.Height) + 4
			if x >= left && x <= right && y >= top && y <= bottom {
				penalty += 15
			}
		}
	}
	return penalty
}
