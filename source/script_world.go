package main

import (
	"sync"
	"time"

	"gothoom/climg"
	scriptapi "gt2"
)

func scriptCurrentWorld() scriptapi.World {
	primarySession.draw.mu.Lock()
	world := scriptapi.World{
		Width: gameAreaSizeX, Height: gameAreaSizeY, Generation: primarySession.draw.generation.Load(),
		Frame: primarySession.draw.current.logicalFrame, ReceivedAt: primarySession.draw.current.receivedAt,
		CameraShiftX: primarySession.draw.current.picShiftX, CameraShiftY: primarySession.draw.current.picShiftY, Lighting: primarySession.draw.current.lightingFlags,
		Mobiles:  make([]scriptapi.Mobile, 0, len(primarySession.draw.current.liveMobs)),
		Pictures: make([]scriptapi.Picture, 0, len(primarySession.draw.current.pictures)),
	}
	for _, m := range primarySession.draw.current.liveMobs {
		d, ok := primarySession.draw.current.descriptors[m.Index]
		if !ok {
			continue
		}
		world.Mobiles = append(world.Mobiles, scriptapi.Mobile{
			Index: m.Index, Name: d.Name, H: m.H, V: m.V, PictID: d.PictID,
			Colors: m.Colors, Player: d.Type == kDescPlayer, State: m.State,
			Plane: d.Plane, Self: m.Index == playerIndex, Dead: m.State == poseDead, Stale: m.Persist,
		})
	}
	for _, p := range primarySession.draw.current.pictures {
		world.Pictures = append(world.Pictures, scriptapi.Picture{PictID: p.PictID, H: p.H, V: p.V, Plane: p.Plane, Moving: p.Moving, Background: p.Background, Reused: p.Again})
	}
	primarySession.draw.mu.Unlock()
	for i := range world.Mobiles {
		m := &world.Mobiles[i]
		m.Size = mobileSize(m.PictID)
		if m.Self && !m.Stale {
			world.Self, world.HasSelf = *m, true
		}
	}
	if clImages != nil {
		for i := range world.Pictures {
			p := &world.Pictures[i]
			p.Width, p.Height = clImages.Size(uint32(p.PictID))
			if frames := clImages.NumFrames(uint32(p.PictID)); frames > 1 {
				p.Height /= frames
			}
			p.Shadow = clImages.Flags(uint32(p.PictID))&climg.PictDefIsShadow != 0
		}
	}
	scriptLocationMu.RLock()
	world.Location = scriptLocation
	scriptLocationMu.RUnlock()
	return world
}

// scriptCurrentWorldForSession returns a detached snapshot for exactly one
// connection. Script candidates bind this function at compile time, so a
// secondary session can never observe the primary world's mutable state.
func scriptCurrentWorldForSession(session *Session) scriptapi.World {
	if session == nil {
		return scriptapi.World{Width: gameAreaSizeX, Height: gameAreaSizeY}
	}
	selfIndex := session.playerIndexSnapshot()
	session.draw.mu.Lock()
	world := scriptapi.World{
		Width: gameAreaSizeX, Height: gameAreaSizeY, Generation: session.draw.generation.Load(),
		Frame: session.draw.current.logicalFrame, ReceivedAt: session.draw.current.receivedAt,
		CameraShiftX: session.draw.current.picShiftX, CameraShiftY: session.draw.current.picShiftY, Lighting: session.draw.current.lightingFlags,
		Mobiles:  make([]scriptapi.Mobile, 0, len(session.draw.current.liveMobs)),
		Pictures: make([]scriptapi.Picture, 0, len(session.draw.current.pictures)),
	}
	for _, m := range session.draw.current.liveMobs {
		d, ok := session.draw.current.descriptors[m.Index]
		if !ok {
			continue
		}
		mobile := scriptapi.Mobile{
			Index: m.Index, Name: d.Name, H: m.H, V: m.V, PictID: d.PictID,
			Colors: m.Colors, Player: d.Type == kDescPlayer, State: m.State,
			Plane: d.Plane, Self: m.Index == selfIndex, Dead: m.State == poseDead, Stale: m.Persist,
		}
		world.Mobiles = append(world.Mobiles, mobile)
	}
	for _, p := range session.draw.current.pictures {
		world.Pictures = append(world.Pictures, scriptapi.Picture{
			PictID: p.PictID, H: p.H, V: p.V, Plane: p.Plane,
			Moving: p.Moving, Background: p.Background, Reused: p.Again,
		})
	}
	session.draw.mu.Unlock()
	// Metadata lookup requires no GPU image allocation and need not hold the
	// session draw lock.
	for i := range world.Mobiles {
		m := &world.Mobiles[i]
		m.Size = mobileSize(m.PictID)
		if m.Self && !m.Stale {
			world.Self, world.HasSelf = *m, true
		}
	}
	if clImages != nil {
		for i := range world.Pictures {
			p := &world.Pictures[i]
			p.Width, p.Height = clImages.Size(uint32(p.PictID))
			if frames := clImages.NumFrames(uint32(p.PictID)); frames > 1 {
				p.Height /= frames
			}
			p.Shadow = clImages.Flags(uint32(p.PictID))&climg.PictDefIsShadow != 0
		}
	}
	world.Location = scriptLocationForSession(session)
	return world
}

func scriptLocationForSession(session *Session) string {
	if session == nil {
		return ""
	}
	return session.scriptLocationSnapshot()
}

const scriptMovementLease = 500 * time.Millisecond

type scriptMovementState struct {
	sync.Mutex
	owner       string
	queue       *scriptEventQueue
	input       inputState
	expires     time.Time
	manualUntil time.Time
	manualAt    time.Time
}

var scriptMovement scriptMovementState

func scriptMovementStateForSession(session *Session) *scriptMovementState {
	if session == nil || session.automation == nil {
		return &scriptMovement
	}
	return &session.automation.movement
}

func scriptMovementQueueCurrent(session *Session, owner string, queue *scriptEventQueue) bool {
	if session == nil {
		return scriptEventQueueIsCurrent(owner, queue)
	}
	return queue != nil && currentSessionScriptEventQueue(session, owner) == queue
}

func scriptMove(owner string, x, y int16, now time.Time) bool {
	return scriptMoveForSession(nil, owner, x, y, now)
}

func scriptMoveForSession(session *Session, owner string, x, y int16, now time.Time) bool {
	var queue *scriptEventQueue
	drawSession := session
	if session == nil {
		queue = currentScriptEventQueue(owner)
		drawSession = primarySession
		scriptSessionMu.Lock()
		active := scriptSessionActive
		scriptSessionMu.Unlock()
		if !active {
			return false
		}
	} else {
		queue = currentSessionScriptEventQueue(session, owner)
		if !session.transport.connected() {
			return false
		}
	}
	if queue == nil {
		return false
	}
	drawSession.draw.mu.Lock()
	fresh := !drawSession.draw.current.receivedAt.IsZero() && now.Sub(drawSession.draw.current.receivedAt) < time.Second
	drawSession.draw.mu.Unlock()
	if !fresh {
		return false
	}
	movement := scriptMovementStateForSession(session)
	movement.Lock()
	defer movement.Unlock()
	if now.Before(movement.manualUntil) ||
		(movement.owner != "" && movement.owner != owner && now.Before(movement.expires)) {
		return false
	}
	movement.owner, movement.queue = owner, queue
	movement.input = inputState{mouseX: int16(max(-fieldCenterX, min(fieldCenterX, int(x)))), mouseY: int16(max(-fieldCenterY, min(fieldCenterY, int(y)))), mouseDown: true}
	movement.expires = now.Add(scriptMovementLease)
	return true
}

func stopScriptMovement(owner string) {
	stopScriptMovementForSession(nil, owner)
}

func stopScriptMovementForSession(session *Session, owner string) {
	movement := scriptMovementStateForSession(session)
	movement.Lock()
	if owner == "" || movement.owner == owner {
		movement.owner, movement.queue = "", nil
		movement.expires = time.Time{}
	}
	movement.Unlock()
}

// Called from ordinary input handling, including legacy macros. Never inject
// script requests into a session input queue: an expired/reloaded request must not linger.
func interruptScriptMovement(now time.Time) {
	interruptScriptMovementForSession(nil, now)
}

func interruptScriptMovementForSession(session *Session, now time.Time) {
	movement := scriptMovementStateForSession(session)
	movement.Lock()
	movement.owner, movement.queue = "", nil
	movement.expires = time.Time{}
	movement.manualUntil = now.Add(time.Second)
	movement.manualAt = now
	movement.Unlock()
}

func applyScriptMovement(input inputState, now time.Time) inputState {
	return applyScriptMovementForSession(nil, input, now)
}

func applyScriptMovementForSession(session *Session, input inputState, now time.Time) inputState {
	if input.mouseDown {
		interruptScriptMovementForSession(session, now)
		return input
	}
	movement := scriptMovementStateForSession(session)
	movement.Lock()
	owner, queue, request, expires := movement.owner, movement.queue, movement.input, movement.expires
	movement.Unlock()
	if owner != "" && now.Before(expires) && scriptMovementQueueCurrent(session, owner, queue) {
		return request
	}
	return input
}

func scriptMovementSnapshot(owner string, now time.Time) scriptapi.MovementState {
	return scriptMovementSnapshotForSession(nil, owner, now)
}

func scriptMovementSnapshotForSession(session *Session, owner string, now time.Time) scriptapi.MovementState {
	movement := scriptMovementStateForSession(session)
	movement.Lock()
	currentOwner, queue := movement.owner, movement.queue
	snapshot := scriptapi.MovementState{
		H: movement.input.mouseX, V: movement.input.mouseY,
		ExpiresAt: movement.expires, LastManualInput: movement.manualAt,
	}
	movement.Unlock()
	snapshot.Active = currentOwner != "" && now.Before(snapshot.ExpiresAt) && scriptMovementQueueCurrent(session, currentOwner, queue)
	snapshot.Owned = snapshot.Active && currentOwner == owner
	return snapshot
}
