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

// Only name text is accepted here; file metadata can never inject commands.
func bardWithOptions(partners []string) (string, error) {
	if len(partners) > 2 {
		return "", fmt.Errorf("Clan Lord supports up to two other performers per ensemble.")
	}
	var result strings.Builder
	seen := make(map[string]bool)
	for _, partner := range partners {
		var name strings.Builder
		for _, c := range strings.TrimSpace(partner) {
			switch {
			case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
				name.WriteRune(c)
			case c == ' ', c == '\'', c == '-':
				// Clan Lord names in commands omit spaces and punctuation.
			default:
				return "", fmt.Errorf("Enter performer names separated by commas, without commands.")
			}
		}
		key := strings.ToLower(name.String())
		if key == "" || len(key) > 32 || seen[key] {
			return "", fmt.Errorf("Enter distinct performer names or unique name prefixes.")
		}
		seen[key] = true
		result.WriteString("/with " + name.String() + " ")
	}
	return result.String(), nil
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
	session     *Session
	generation  uint64
	instrument  InventoryItem
	commands    []string
	tickets     []CommandTicket
	deadline    time.Time
	submitted   bool
	duration    time.Duration
	finishAt    time.Time
	ensemble    bool
	preparation *bardCaseOperation
}

func startBardPerformance(session *Session, value string, index int) (*bardPerformance, error) {
	return startBardEnsemblePerformance(session, value, index, nil)
}

func startBardEnsemblePerformance(session *Session, value string, index int, partners []string) (*bardPerformance, error) {
	if session == nil || !session.transport.connected() {
		return nil, fmt.Errorf("Connect a character to perform.")
	}
	score, err := parseBardScore(value, index)
	if err != nil {
		return nil, err
	}
	if len(score.Parts) != 1 {
		return nil, fmt.Errorf("Choose one part to perform in-game.")
	}
	value, index = score.Parts[0].Text, score.Parts[0].Instrument
	item, ok := bardOwnedInstrument(session, index)
	if !ok && !bardCanPrepareInstrument(session, index) {
		return nil, fmt.Errorf("Carry %s or an instrument case to perform.", classicInstrumentNames[index])
	}
	generation := bardConnectionGeneration(session)
	notes, err := validateBardTune(value, index)
	if err != nil {
		return nil, err
	}
	commands, err := bardEnsembleCommands(value, partners)
	if err != nil {
		return nil, err
	}
	p := &bardPerformance{session: session, generation: generation, instrument: item, commands: commands, deadline: time.Now().Add(10 * time.Second), ensemble: len(partners) > 0}
	for _, n := range notes {
		if end := n.Start + n.Duration; end > p.duration {
			p.duration = end
		}
	}
	if !ok {
		p.preparation, err = startBardCaseOperation(session, index)
		if err != nil {
			return nil, err
		}
		return p, nil
	}
	p.tickets = p.queue([]string{formatEquipCommand(item.ID, item.IDIndex)})
	if len(p.tickets) == 0 {
		return nil, fmt.Errorf("The connection changed; try again.")
	}
	return p, nil
}

func (p *bardPerformance) queue(commands []string) []CommandTicket {
	return queueBardSessionCommands(p.session, p.generation, commands)
}

func (p *bardPerformance) current() bool {
	return p != nil && p.session.transport.connectedGeneration(p.generation)
}
func (p *bardPerformance) stop() {
	if p == nil {
		return
	}
	p.preparation.cancel()
	for _, ticket := range p.tickets {
		ticket.Cancel()
	}
	if p.current() {
		p.queue([]string{"/use /stop"})
	}
}
func (p *bardPerformance) update(now time.Time) error {
	if !p.current() {
		p.preparation.cancel()
		for _, ticket := range p.tickets {
			ticket.Cancel()
		}
		return fmt.Errorf("Performance ended: character disconnected.")
	}
	if p.preparation != nil {
		if err := p.preparation.update(now); err != nil {
			p.stop()
			return err
		}
		if !p.preparation.done {
			return nil
		}
		item, ok := bardOwnedInstrument(p.session, p.preparation.target)
		if !ok {
			p.stop()
			return fmt.Errorf("The requested instrument is no longer in inventory.")
		}
		p.instrument, p.preparation = item, nil
		p.tickets = p.queue([]string{formatEquipCommand(item.ID, item.IDIndex)})
		if len(p.tickets) == 0 {
			return fmt.Errorf("The connection changed; try again.")
		}
		p.deadline = now.Add(10 * time.Second)
	}
	if p.submitted {
		// Ensemble playback waits for remote performers after our commands are
		// sent. Keep Stop available instead of guessing when that playback ends.
		if !p.ensemble && p.finishAt.IsZero() && p.tickets[len(p.tickets)-1].Status().State == scriptapi.CommandSent {
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
	score, err := parseBardScore(value, index)
	if err != nil {
		return nil, err
	}
	return startBardScorePreview(score, -1, finished)
}

func startBardScorePreview(score bardScore, selected int, finished func(error)) (*bardPreview, error) {
	if selected >= 0 {
		if selected >= len(score.Parts) {
			return nil, fmt.Errorf("Choose a part.")
		}
		score.Parts = score.Parts[selected : selected+1]
	}
	parts, err := validateBardScore(score)
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
		err := playMusicGroupWithSettingsAtFrameIf(ctx, parts, []int{p.who}, nil, nil, settings, 0, func() bool { return !p.stopped.Load() })
		dispatchMainThread(func() {
			if !p.stopped.Load() && finished != nil {
				finished(err)
			}
		})
	}()
	return p, nil
}
