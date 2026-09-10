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
const scriptDescription = "Alt-right-click a player to follow their trail across area edges and doorways. Move manually or /follow off to stop."
const scriptAPIVersion = 2

var targetName string
var followWindow gt2.Window
var followActivity = "Stopped"
var followNote = "Alt-right-click a player in the game view to follow."
var manualAt time.Time
var following bool
var previous gt2.World
var stalled time.Duration
var noProgress time.Duration
var attemptingMove bool
var attemptedDirection followPoint

type blockedSpot struct {
	point   followPoint
	expires time.Time
}

var blockedSpots []blockedSpot
var wiggleStarted time.Time
var wiggleNext time.Time
var wiggleSide = 1.0
var detourSide = 1.0
var startDistance = 72.0
var stopDistance = 44.0

// Breadcrumbs use the current scene's coordinates, shifted with its scenery.
// Never carry these coordinates into a different area.
type followPoint struct{ x, y float64 }

var trail []followPoint
var lastLeader followPoint
var haveLeader bool
var targetLost bool
var leaderDirection followPoint
var leaderMovedAt time.Time
var sceneryHints = true
var mobileClearance = 34.0

func Init() {
	startDistance = gt2.Decimal(gt2.DecimalOption{Key: "start", Label: "Start following distance", Default: 72, Min: 60, Max: 180, Step: 4, OnChange: func(v float64) { startDistance = v }})
	stopDistance = gt2.Decimal(gt2.DecimalOption{Key: "stop", Label: "Stop following distance", Default: 44, Min: 24, Max: 56, Step: 4, OnChange: func(v float64) { stopDistance = v }})
	mobileClearance = gt2.Decimal(gt2.DecimalOption{Key: "clearance", Label: "Preferred mobile clearance", Help: "A soft spacing preference; tight passages may require less room.", Default: 34, Min: 24, Max: 60, Step: 2, OnChange: func(v float64) { mobileClearance = v }})
	sceneryHints = gt2.Bool(gt2.BoolOption{Key: "scenery", Label: "Use scenery hints for detours", Help: "Artwork planes are draw order, not collision geometry. Disable if scenery causes unnecessary detours.", Default: true, OnChange: func(v bool) { sceneryHints = v }})
	followWindow = gt2.CreateWindow(gt2.WindowOptions{
		Title: "Follow Player", Width: 360,
		Text: "Following: None\nStatus: Stopped\nAlt-right-click a player in the game view to follow.",
		Buttons: []gt2.WindowButton{
			{ID: "stop", Label: "Stop Follow", Tooltip: "Stop following immediately", Disabled: true, OnClick: func() { stopFollow("Follow stopped.") }},
		},
		OnClose: func() { stopFollow("Follow stopped: window closed.") },
	})
	gt2.Command("follow", startFollow)
	gt2.Bind("Alt-RightClick", followClickedPlayer)
	gt2.Command("stopfollow", func(args string) { stopFollow("Follow stopped.") })
	gt2.Command("followui", func(args string) { followWindow.Show(); refreshFollowWindow() })

	gt2.OnWorld(followWorld)
	gt2.Repeat(200*time.Millisecond, func() {
		defer refreshFollowWindow()
		if targetName != "" && gt2.Movement().LastManualInput.After(manualAt) {
			stopFollow("Follow stopped: manual movement.")
			return
		}
		if targetName != "" && time.Since(gt2.CurrentWorld().ReceivedAt) > time.Second {
			pauseFollow("Waiting for world updates")
		}
	})
	gt2.OnLogout(func(event gt2.LifecycleEvent) { stopFollow("") })
	gt2.OnCharacterChange(func(event gt2.LifecycleEvent) { stopFollow("") })
}

func followClickedPlayer(event gt2.InputEvent) {
	if !event.OnMobile || !event.Mobile.Player || event.Mobile.Self || event.Mobile.Dead || event.Mobile.Stale || strings.TrimSpace(event.Mobile.Name) == "" {
		return
	}
	// The hit test already identified a mobile. Use its current descriptor name
	// for subsequent scene tracking instead of sending click text to /follow.
	world := gt2.CurrentWorld()
	for _, mobile := range world.Mobiles {
		if mobile.Index == event.Mobile.Index && mobile.Player && !mobile.Self && !mobile.Dead && !mobile.Stale && strings.TrimSpace(mobile.Name) != "" {
			event.Consume()
			beginFollow(world, mobile)
			return
		}
	}
}

func startFollow(args string) {
	defer refreshFollowWindow()
	name := strings.TrimSpace(args)
	if strings.EqualFold(name, "off") {
		stopFollow("Follow stopped.")
		return
	}
	if name == "" {
		followNote = "Alt-right-click a player in the game view or use /follow Player Name."
		gt2.Print(followNote)
		return
	}
	world := gt2.CurrentWorld()
	leader, ok := findLeader(world, name)
	if !ok {
		for _, mobile := range world.Mobiles {
			if !mobile.Player || mobile.Self || mobile.Dead || mobile.Stale || !strings.HasPrefix(followName(mobile.Name), followName(name)) {
				continue
			}
			if ok {
				followNote = "More than one visible player matches. Type more of the name or Alt-right-click the player."
				gt2.Print(followNote)
				return
			}
			leader, ok = mobile, true
		}
	}
	if !ok {
		followNote = "No visible, standing player matches that name. Alt-right-click the player in the game view."
		gt2.Print(followNote)
		return
	}
	beginFollow(world, leader)
}

func beginFollow(world gt2.World, leader gt2.Mobile) {
	defer refreshFollowWindow()
	releaseFollowMovement()
	targetName = leader.Name
	followActivity = "Following"
	followNote = ""
	followWindow.Show()
	following = false
	previous = gt2.World{}
	resetTrail()
	manualAt = gt2.Movement().LastManualInput
	detourSide = 1
	wiggleSide = -1
	stalled = 0
	wiggleStarted = time.Time{}
	wiggleNext = time.Time{}
	gt2.Print("Following " + targetName + ". Move manually or use /follow off to stop.")
	followWorld(world)
}

// The window and every status/control decision live entirely in this script.
func refreshFollowWindow() {
	target := targetName
	if target == "" {
		target = "None"
	}
	status := "Following: " + target + "\nStatus: " + followActivity
	if followNote != "" {
		status += "\n" + followNote
	} else {
		status += "\nAlt-right-click a player to follow. Move manually to stop."
	}
	followWindow.SetText(status)
	followWindow.SetButtonEnabled("stop", targetName != "")
}

func stopFollow(message string) {
	followActivity = "Stopped"
	followNote = message
	defer refreshFollowWindow()
	targetName = ""
	following = false
	previous = gt2.World{}
	resetTrail()
	stalled = 0
	wiggleStarted = time.Time{}
	wiggleNext = time.Time{}
	releaseFollowMovement()
	if message != "" {
		gt2.Print(message)
	}
}

func findLeader(world gt2.World, name string) (gt2.Mobile, bool) {
	for _, m := range world.Mobiles {
		if m.Player && !m.Self && !m.Dead && !m.Stale && samePlayer(m.Name, name) {
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
	if world.Self.Dead || time.Since(world.ReceivedAt) > time.Second {
		pauseFollow("Waiting for character and world")
		return
	}
	if !previous.ReceivedAt.IsZero() && world.Frame < previous.Frame {
		pauseFollow("Waiting for world updates")
		return
	}
	// A transition can briefly omit self. Release movement until coordinates
	// are available again, and reacquire rather than steering in the old area.
	if !world.HasSelf {
		pauseFollow("Waiting for character and world")
		return
	}
	if world.Frame == previous.Frame && !previous.ReceivedAt.IsZero() {
		return
	}
	leader, visible := findLeader(world, targetName)
	for _, m := range world.Mobiles {
		if m.Player && !m.Self && !m.Stale && m.Dead && samePlayer(m.Name, targetName) {
			pauseFollow("Waiting for target to stand")
			return
		}
	}
	goal, ready := followTrail(&world, leader, visible)
	if !ready {
		waitForLeader()
		previous = world
		return
	}
	observeFollowProgress(world)
	dx, dy := goal.x-float64(world.Self.H), goal.y-float64(world.Self.V)
	distance := math.Hypot(dx, dy)
	if !visible {
		followBreadcrumb(world, goal)
		return
	}
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
		releaseFollowMovement()
		stalled = 0
		wiggleStarted = time.Time{}
		wiggleNext = time.Time{}
		previous = world
		return
	}
	now := world.ReceivedAt
	wiggling := !wiggleStarted.IsZero() && now.Sub(wiggleStarted) < 1400*time.Millisecond

	direction := math.Atan2(dy, dx)
	if yielding {
		direction = math.Atan2(yieldY, yieldX)
	}
	if stalled > 2*time.Second && !wiggling && !now.Before(wiggleNext) {
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
	// Route around scenery during ordinary travel, before a collision causes
	// a stall. Lost-target breadcrumbs keep their separate doorway handling.
	heading, clear := routeDirection(world, leader, direction, reach, true)
	if !clear {
		followActivity = "Waiting for space"
		// Wait when every local exit would move closer to a mobile already inside
		// the minimum spacing. Keep observing so an opening can be used next frame.
		releaseFollowMovement()
		previous = world
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
	if !moveFollow(world, x, y) {
		pauseFollow("Waiting for movement")
		return
	}
	previous = world
}

func samePlayer(a, b string) bool {
	return strings.EqualFold(followName(a), followName(b))
}

func followName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), ""))
}

func resetTrail() {
	trail = nil
	haveLeader = false
	targetLost = false
	leaderDirection = followPoint{}
	leaderMovedAt = time.Time{}
	resetRouting()
}

// Unavailable movement or coordinates suspend pursuit, not the chosen target.
func pauseFollow(activity string) {
	releaseFollowMovement()
	followActivity = activity
	following = false
	previous = gt2.World{}
	resetTrail()
	stalled = 0
	wiggleStarted = time.Time{}
	wiggleNext = time.Time{}
}

func waitForLeader() {
	releaseFollowMovement()
	stalled = 0
	wiggleStarted = time.Time{}
	wiggleNext = time.Time{}
	followActivity = "Waiting for target"
}

// Keep the breadcrumb as the destination while routing around known mobiles
// and places where attempted movement failed. Artwork alone cannot close a door.
func followBreadcrumb(world gt2.World, goal followPoint) {
	following = true
	wiggleStarted = time.Time{}
	wiggleNext = time.Time{}
	dx, dy := goal.x-float64(world.Self.H), goal.y-float64(world.Self.V)
	desired, reach := math.Atan2(dy, dx), math.Hypot(dx, dy)
	heading, clear := routeDirection(world, gt2.Mobile{}, desired, reach, false)
	followActivity = "Following breadcrumbs"
	if !clear {
		releaseFollowMovement()
		followActivity = "Waiting for space on trail"
	} else {
		if math.Abs(heading-desired) > 0.01 {
			reach = math.Min(80, math.Max(45, reach))
			followActivity = "Routing to breadcrumb"
		}
		if !moveFollow(world, float64(world.Self.H)+math.Cos(heading)*reach, float64(world.Self.V)+math.Sin(heading)*reach) {
			releaseFollowMovement()
			followActivity = "Waiting for movement"
		}
	}
	previous = world
}

func releaseFollowMovement() {
	gt2.StopMoving()
	attemptingMove = false
}

func moveFollow(world gt2.World, x, y float64) bool {
	attemptingMove = gt2.Move(int16(math.Round(x)), int16(math.Round(y)))
	// Track the clamped mouse command, rather than an off-screen requested aim.
	movement := gt2.Movement()
	dx, dy := float64(movement.H)-float64(world.Self.H), float64(movement.V)-float64(world.Self.V)
	distance := math.Max(1, math.Hypot(dx, dy))
	attemptedDirection = followPoint{dx / distance, dy / distance}
	return attemptingMove
}

func resetRouting() {
	blockedSpots = nil
	noProgress = 0
	stalled = 0
	attemptingMove = false
	wiggleStarted = time.Time{}
	wiggleNext = time.Time{}
}

// Failed movement supplies local collision evidence even when artwork does not.
// Measure progress along the command we actually sent, including detours, so
// walking around an obstacle need not shorten the distance to the final target.
func observeFollowProgress(world gt2.World) {
	kept := blockedSpots[:0]
	for _, spot := range blockedSpots {
		if world.ReceivedAt.Before(spot.expires) {
			kept = append(kept, spot)
		}
	}
	blockedSpots = kept
	elapsed := world.ReceivedAt.Sub(previous.ReceivedAt)
	if !attemptingMove || !previous.HasSelf || world.Frame != previous.Frame+1 || elapsed <= 0 || elapsed >= time.Second {
		noProgress, stalled = 0, 0
		wiggleStarted = time.Time{}
		return
	}
	dx := float64(int(world.Self.H) - int(previous.Self.H) - world.CameraShiftX)
	dy := float64(int(world.Self.V) - int(previous.Self.V) - world.CameraShiftY)
	if math.Hypot(dx, dy) <= 4*elapsed.Seconds() {
		stalled += elapsed
	} else {
		stalled = 0
		wiggleStarted = time.Time{}
	}
	if dx*attemptedDirection.x+dy*attemptedDirection.y <= 4*elapsed.Seconds() {
		noProgress += elapsed
	} else {
		noProgress = 0
	}
	if noProgress < 750*time.Millisecond || (!wiggleStarted.IsZero() && world.ReceivedAt.Sub(wiggleStarted) < 1400*time.Millisecond) {
		return
	}
	point := followPoint{float64(world.Self.H) + attemptedDirection.x*24, float64(world.Self.V) + attemptedDirection.y*24}
	for i := range blockedSpots {
		if math.Hypot(point.x-blockedSpots[i].point.x, point.y-blockedSpots[i].point.y) < 16 {
			blockedSpots[i].expires = world.ReceivedAt.Add(10 * time.Second)
			noProgress = 0
			return
		}
	}
	blockedSpots = append(blockedSpots, blockedSpot{point, world.ReceivedAt.Add(10 * time.Second)})
	if len(blockedSpots) > 8 {
		blockedSpots = blockedSpots[len(blockedSpots)-8:]
	}
	noProgress = 0
}

func blockedPathPenalty(world gt2.World, angle, reach float64) float64 {
	penalty := 0.0
	ux, uy := math.Cos(angle), math.Sin(angle)
	for _, spot := range blockedSpots {
		x, y := spot.point.x-float64(world.Self.H), spot.point.y-float64(world.Self.V)
		along := math.Max(0, math.Min(reach, x*ux+y*uy))
		nearest := math.Hypot(x-ux*along, y-uy*along)
		// Permit escape if an approximate footprint already contains us.
		radius := math.Min(20, math.Hypot(x, y))
		if nearest < radius {
			penalty += 200 * (radius - nearest) / math.Max(1, radius)
		}
	}
	return penalty
}

// Unique stationary artwork supplies landmarks even with motion smoothing off
// or skipped callbacks. Repeated tiles are ambiguous; moving sprites and shadows
// are not landmarks. Reused artwork can still be part of the current scene.
func sceneAnchors(world gt2.World) map[uint16]gt2.Picture {
	anchors := map[uint16]gt2.Picture{}
	counts := map[uint16]int{}
	for _, p := range world.Pictures {
		if p.Moving || p.Shadow {
			continue
		}
		counts[p.PictID]++
		if counts[p.PictID] == 1 {
			anchors[p.PictID] = p
		} else {
			delete(anchors, p.PictID)
		}
	}
	return anchors
}

// Returns a translation between these snapshots and whether the old trail is
// still usable. A location change, replaced scenery, or coordinate jump starts
// a new scene. CameraShift alone describes only the immediately preceding frame.
func trailShift(before, world gt2.World) (float64, float64, bool) {
	if before.Location != world.Location {
		return 0, 0, false
	}
	a, b := sceneAnchors(before), sceneAnchors(world)
	votes := map[[2]int]int{}
	best := [2]int{}
	bestCount := 0
	background := false
	for id, p := range a {
		q, ok := b[id]
		if !ok {
			continue
		}
		shift := [2]int{int(q.H) - int(p.H), int(q.V) - int(p.V)}
		votes[shift]++
		if votes[shift] > bestCount {
			best, bestCount = shift, votes[shift]
			background = p.Background && q.Background
		}
	}
	count := math.Min(float64(len(a)), float64(len(b)))
	matched := float64(bestCount)*2 > count && (bestCount >= 2 || (count == 1 && background))
	if bestCount == 0 && len(a) > 0 && len(b) > 0 {
		for _, p := range a {
			for _, q := range b {
				if p.Background && q.Background {
					return 0, 0, false
				}
			}
		}
	}
	dx, dy := float64(world.CameraShiftX), float64(world.CameraShiftY)
	if matched {
		dx, dy = float64(best[0]), float64(best[1])
	} else if (len(a) >= 2 && len(b) >= 2) || world.Frame != before.Frame+1 {
		return 0, 0, false
	}
	gap := math.Max(1, float64(world.Frame-before.Frame))
	travel := math.Hypot(float64(world.Self.H)-float64(before.Self.H)-dx, float64(world.Self.V)-float64(before.Self.V)-dy)
	if math.Hypot(dx, dy) > 160*gap || travel > 96*gap {
		return 0, 0, false
	}
	return dx, dy, true
}

func followTrail(snapshot *gt2.World, leader gt2.Mobile, visible bool) (followPoint, bool) {
	world := *snapshot
	fromX, fromY := float64(world.Self.H), float64(world.Self.V)
	if previous.HasSelf {
		sx, sy, sameScene := trailShift(previous, world)
		if !sameScene {
			// No reliable coordinate mapping remains for the recorded trail.
			resetRouting()
			trail = nil
			haveLeader = false
			leaderDirection = followPoint{}
			leaderMovedAt = time.Time{}
			stalled = 0
			wiggleStarted = time.Time{}
			wiggleNext = time.Time{}
			previous = gt2.World{}
		} else {
			fromX, fromY = float64(previous.Self.H)+sx, float64(previous.Self.V)+sy
			snapshot.CameraShiftX, snapshot.CameraShiftY = int(sx), int(sy)
			for i := range trail {
				trail[i].x += sx
				trail[i].y += sy
			}
			for i := range blockedSpots {
				blockedSpots[i].point.x += sx
				blockedSpots[i].point.y += sy
			}
			lastLeader.x += sx
			lastLeader.y += sy
		}
	}
	if visible {
		point := followPoint{float64(leader.H), float64(leader.V)}
		if targetLost {
			stalled = 0
			wiggleStarted = time.Time{}
			wiggleNext = time.Time{}
			trail = nil
			leaderDirection = followPoint{}
			leaderMovedAt = time.Time{}
		} else if haveLeader {
			dx, dy := point.x-lastLeader.x, point.y-lastLeader.y
			distance := math.Hypot(dx, dy)
			// lastLeader has already been aligned with stationary scenery, so
			// scrolling cannot supply a false exit direction. Ignore teleports.
			if distance > 80 {
				leaderDirection = followPoint{}
				leaderMovedAt = time.Time{}
			} else if distance >= 2 {
				leaderDirection = followPoint{dx / distance, dy / distance}
				leaderMovedAt = world.ReceivedAt
			}
		}
		targetLost = false
		lastLeader, haveLeader = point, true
		if len(trail) == 0 || math.Hypot(point.x-trail[len(trail)-1].x, point.y-trail[len(trail)-1].y) >= 6 {
			trail = append(trail, point)
			if len(trail) > 64 {
				trail = trail[len(trail)-64:]
			}
		}
	} else {
		if !targetLost && haveLeader {
			trail = append(trail, lastLeader)
			joinTrail(world)
			// The final visible sample can fall just short of an area boundary.
			// Continue once along recent observed travel to cross it, keeping the
			// corner samples before this endpoint. A new scene discards it.
			if !leaderMovedAt.IsZero() && world.ReceivedAt.Sub(leaderMovedAt) <= 1500*time.Millisecond {
				trail = append(trail, followPoint{lastLeader.x + leaderDirection.x*96, lastLeader.y + leaderDirection.y*96})
			}
		}
		targetLost = true
	}
	// Drop breadcrumbs we reached or passed on the latest movement segment.
	// Keeping the newest reached point avoids walking back around an old bend.
	sx, sy := float64(world.Self.H), float64(world.Self.V)
	vx, vy := sx-fromX, sy-fromY
	for i := len(trail) - 1; i >= 0; i-- {
		along := math.Max(0, math.Min(1, ((trail[i].x-fromX)*vx+(trail[i].y-fromY)*vy)/math.Max(1, vx*vx+vy*vy)))
		if math.Hypot(trail[i].x-fromX-vx*along, trail[i].y-fromY-vy*along) <= 14 {
			trail = trail[i+1:]
			break
		}
	}
	if visible {
		return lastLeader, true
	}
	if len(trail) == 0 {
		return followPoint{}, false
	}
	return trail[0], true
}

// Visible following can cut a corner without stepping on its breadcrumbs.
// Rejoin the nearest observed segment rather than walking back to old samples.
func joinTrail(world gt2.World) {
	nearest := math.Inf(1)
	index := 0
	point := trail[0]
	for i := 0; i < len(trail)-1; i++ {
		a, b := trail[i], trail[i+1]
		dx, dy := b.x-a.x, b.y-a.y
		along := math.Max(0, math.Min(1, ((float64(world.Self.H)-a.x)*dx+(float64(world.Self.V)-a.y)*dy)/math.Max(1, dx*dx+dy*dy)))
		p := followPoint{a.x + along*dx, a.y + along*dy}
		d := math.Hypot(p.x-float64(world.Self.H), p.y-float64(world.Self.V))
		if d <= nearest {
			nearest, index, point = d, i, p
		}
	}
	trail = append([]followPoint{point}, trail[index+1:]...)
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

// Route visible following and recovery on every fresh frame. Clearance
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
		score := penalty + blockedPathPenalty(world, angle, lookahead) + math.Abs(offset)*12
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
