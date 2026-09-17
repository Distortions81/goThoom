package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	scriptapi "gt2"
)

func bardInstrumentIndex(name string) int {
	normalize := func(s string) string {
		return strings.NewReplacer(" ", "", "_", "", "-", "", "è", "e", "é", "e").Replace(strings.ToLower(strings.TrimSpace(s)))
	}
	name = normalize(name)
	for i, n := range classicInstrumentNames {
		if normalize(n) == name {
			return i
		}
	}
	return -1
}
func bardOwnedInstrument(session *Session, index int) (InventoryItem, bool) {
	if session != nil {
		for _, item := range session.inventory.snapshot() {
			if bardInstrumentIndex(item.Base) == index || (item.Base == "" && bardInstrumentIndex(item.Name) == index) {
				return item, true
			}
		}
	}
	return InventoryItem{}, false
}
func bardConnectionGeneration(session *Session) uint64 {
	session.transport.mu.RLock()
	defer session.transport.mu.RUnlock()
	return session.transport.generation
}

// UI-originated commands use the existing cancellable command queue, without
// script ownership or permission checks. Stop removes only this performance.
func queueBardCommands(session *Session, commands []string) []CommandTicket {
	state := session.commands
	state.mu.Lock()
	defer state.mu.Unlock()
	tickets := make([]CommandTicket, 0, len(commands))
	for _, cmd := range commands {
		ticket := CommandTicket{state: &scriptCommandState{commands: state, status: scriptapi.CommandStatus{State: scriptapi.CommandQueued}}}
		state.queue = append(state.queue, queuedCommand{text: cmd, ticket: ticket.state})
		tickets = append(tickets, ticket)
	}
	state.nextLocked()
	return tickets
}

type bardPerformance struct {
	session    *Session
	generation uint64
	instrument InventoryItem
	commands   []string
	tickets    []CommandTicket
	deadline   time.Time
	submitted  bool
	duration   time.Duration
	finishAt   time.Time
}

func startBardPerformance(session *Session, value string, index int) (*bardPerformance, error) {
	if session == nil || !session.transport.connected() {
		return nil, fmt.Errorf("Connect a character to perform.")
	}
	if index < 0 || index >= len(instruments) {
		return nil, fmt.Errorf("Choose an instrument.")
	}
	item, ok := bardOwnedInstrument(session, index)
	if !ok {
		return nil, fmt.Errorf("Take %s out of its case and into inventory before playing.", classicInstrumentNames[index])
	}
	generation := bardConnectionGeneration(session)
	notes, err := validateBardTune(value, index)
	if err != nil {
		return nil, err
	}
	commands, err := bardTuneCommands(value)
	if err != nil {
		return nil, err
	}
	p := &bardPerformance{session: session, generation: generation, instrument: item, commands: commands, deadline: time.Now().Add(10 * time.Second)}
	for _, n := range notes {
		if end := n.Start + n.Duration; end > p.duration {
			p.duration = end
		}
	}
	p.tickets = p.queue([]string{formatEquipCommand(item.ID, item.IDIndex)})
	if len(p.tickets) == 0 {
		return nil, fmt.Errorf("The connection changed; try again.")
	}
	return p, nil
}

func (p *bardPerformance) queue(commands []string) []CommandTicket {
	transport := p.session.transport
	transport.mu.RLock()
	defer transport.mu.RUnlock()
	if transport.generation != p.generation || transport.status != sessionConnected || transport.tcp == nil {
		return nil
	}
	return queueBardCommands(p.session, commands)
}

func (p *bardPerformance) current() bool {
	return p != nil && p.session.transport.connectedGeneration(p.generation)
}
func (p *bardPerformance) stop() {
	if p == nil {
		return
	}
	for _, ticket := range p.tickets {
		ticket.Cancel()
	}
	if p.current() {
		p.queue([]string{"/use /stop"})
	}
}
func (p *bardPerformance) update(now time.Time) error {
	if !p.current() {
		for _, ticket := range p.tickets {
			ticket.Cancel()
		}
		return fmt.Errorf("Performance ended: character disconnected.")
	}
	if p.submitted {
		if p.finishAt.IsZero() && p.tickets[len(p.tickets)-1].Status().State == scriptapi.CommandSent {
			p.finishAt = now.Add(p.duration + time.Second)
		}
		return nil
	}
	for _, item := range p.session.inventory.snapshot() {
		if item.ID == p.instrument.ID && item.IDIndex == p.instrument.IDIndex && item.InstanceID == p.instrument.InstanceID && item.Equipped {
			// An already-equipped item still waits for the queued equip write. It must
			// not bypass unrelated commands ahead of it.
			if p.tickets[0].Status().State != scriptapi.CommandSent || !p.session.commands.idle() {
				break
			}
			// Clear pending song parts before this performance.
			p.tickets = append(p.tickets, p.queue(append([]string{"/use /stop"}, p.commands...))...)
			p.submitted = true
			return nil
		}
	}
	if now.After(p.deadline) {
		p.stop()
		return fmt.Errorf("The instrument was not equipped. Check the game's response and try again.")
	}
	return nil
}

var bardPreviewSequence atomic.Int64

// Each preview has its own identity, so stopping one never stops another bard.
type bardPreview struct {
	who     int
	stopped atomic.Bool
}

func (p *bardPreview) stop() {
	if p != nil {
		p.stopped.Store(true)
		stopMusicFor(p.who)
	}
}
func startBardPreview(value string, index int, finished func(error)) (*bardPreview, error) {
	notes, err := validateBardTune(value, index)
	if err != nil {
		return nil, err
	}
	soundMu.Lock()
	ctx := audioContext
	soundMu.Unlock()
	settings := currentMusicPlaybackSettings()
	if ctx == nil || !settings.enabled {
		return nil, fmt.Errorf("Enable music and turn up the Music mixer to preview.")
	}
	p := &bardPreview{who: -int(bardPreviewSequence.Add(1))}
	go func() {
		err := playMusicGroupWithSettingsAtFrameIf(ctx, []musicPart{{program: instruments[index].program, notes: notes}}, []int{p.who}, nil, nil, settings, 0, func() bool { return !p.stopped.Load() })
		dispatchMainThread(func() {
			if !p.stopped.Load() && finished != nil {
				finished(err)
			}
		})
	}()
	return p, nil
}
