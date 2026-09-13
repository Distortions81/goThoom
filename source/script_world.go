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
	if session == primarySession {
		scriptLocationMu.RLock()
		location := scriptLocation
		scriptLocationMu.RUnlock()
		return location
	}
	return session.scriptLocationSnapshot()
}

const scriptMovementLease = 500 * time.Millisecond

var scriptMovement struct {
	sync.Mutex
	owner       string
	queue       *scriptEventQueue
	input       inputState
	expires     time.Time
	manualUntil time.Time
	manualAt    time.Time
}

func scriptMove(owner string, x, y int16, now time.Time) bool {
	if scriptIsDisabled(owner) {
		return false
	}
	queue := currentScriptEventQueue(owner)
	if queue == nil {
		return false
	}
	scriptSessionMu.Lock()
	defer scriptSessionMu.Unlock()
	if !scriptSessionActive {
		return false
	}
	primarySession.draw.mu.Lock()
	fresh := !primarySession.draw.current.receivedAt.IsZero() && now.Sub(primarySession.draw.current.receivedAt) < time.Second
	primarySession.draw.mu.Unlock()
	if !fresh {
		return false
	}
	scriptMovement.Lock()
	defer scriptMovement.Unlock()
	if now.Before(scriptMovement.manualUntil) ||
		(scriptMovement.owner != "" && scriptMovement.owner != owner && now.Before(scriptMovement.expires)) {
		return false
	}
	scriptMovement.owner, scriptMovement.queue = owner, queue
	scriptMovement.input = inputState{mouseX: int16(max(-fieldCenterX, min(fieldCenterX, int(x)))), mouseY: int16(max(-fieldCenterY, min(fieldCenterY, int(y)))), mouseDown: true}
	scriptMovement.expires = now.Add(scriptMovementLease)
	return true
}

func stopScriptMovement(owner string) {
	scriptMovement.Lock()
	if owner == "" || scriptMovement.owner == owner {
		scriptMovement.owner, scriptMovement.queue = "", nil
		scriptMovement.expires = time.Time{}
	}
	scriptMovement.Unlock()
}

// Called from ordinary input handling, including legacy macros. Never inject
// script requests into inputQueue: an expired/reloaded request must not linger.
func interruptScriptMovement(now time.Time) {
	scriptMovement.Lock()
	scriptMovement.owner, scriptMovement.queue = "", nil
	scriptMovement.expires = time.Time{}
	scriptMovement.manualUntil = now.Add(time.Second)
	scriptMovement.manualAt = now
	scriptMovement.Unlock()
}

func applyScriptMovement(input inputState, now time.Time) inputState {
	if input.mouseDown {
		interruptScriptMovement(now)
		return input
	}
	scriptMovement.Lock()
	owner, queue, request, expires := scriptMovement.owner, scriptMovement.queue, scriptMovement.input, scriptMovement.expires
	scriptMovement.Unlock()
	if owner != "" && now.Before(expires) && scriptEventQueueIsCurrent(owner, queue) {
		return request
	}
	return input
}

func scriptMovementSnapshot(owner string, now time.Time) scriptapi.MovementState {
	scriptMovement.Lock()
	currentOwner, queue := scriptMovement.owner, scriptMovement.queue
	snapshot := scriptapi.MovementState{
		H: scriptMovement.input.mouseX, V: scriptMovement.input.mouseY,
		ExpiresAt: scriptMovement.expires, LastManualInput: scriptMovement.manualAt,
	}
	scriptMovement.Unlock()
	snapshot.Active = currentOwner != "" && now.Before(snapshot.ExpiresAt) && scriptEventQueueIsCurrent(currentOwner, queue)
	snapshot.Owned = snapshot.Active && currentOwner == owner
	return snapshot
}
