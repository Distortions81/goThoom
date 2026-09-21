package main

import (
	"fmt"
	"strings"
	"unicode"

	"gothoom/eui"
)

type bardPlayRequest struct {
	session                       *Session
	generation                    uint64
	character, path, value, title string
	part                          bardPart
	partners                      []string
}

func (p *bardPanel) showPlayConfirmation(request bardPlayRequest) {
	if p.playConfirm != nil {
		p.playConfirm.Close()
	}
	character := request.character
	if character == "" {
		character = "Selected character"
	}
	message := fmt.Sprintf("Play “%s” in game?\n\nCharacter: %s\nPart: %s\nInstrument: %s",
		request.title, character, request.part.Name, classicInstrumentNames[request.part.Instrument])
	if len(request.partners) > 0 {
		message += "\nWith: " + strings.Join(request.partners, ", ")
	}
	message += "\n\nThis equips the instrument and performs for nearby players."
	if _, owned := bardOwnedInstrument(request.session, request.part.Instrument); !owned {
		message += " The client will try to retrieve it from your instrument case, putting one carried instrument away first if space is needed."
	}
	popup := eui.ShowPopup("Confirm In-Game Playback", message, []eui.PopupButton{
		{Text: "Cancel", Action: func() { p.setStatus("In-game playback canceled.", false) }},
		{Text: "Play in Game", Action: func() {
			// The confirmation names a specific song, part and connected character.
			// Switching tabs, reconnecting or editing the file requires a new review.
			if !p.win.IsOpen() || selectedAppSession() != request.session ||
				!request.session.transport.connectedGeneration(request.generation) ||
				request.session.characterName() != request.character || p.selected != request.path ||
				(p.selectedPart != "" && p.selectedPart != request.part.Name) {
				p.setError(fmt.Errorf("The song, part, or character changed. Choose Play in Game again."))
				return
			}
			value, err := readBardTune(request.path)
			if err != nil {
				p.setError(err)
				return
			}
			if value != request.value {
				p.setError(fmt.Errorf("The saved song changed. Choose Play in Game again."))
				return
			}
			if !bardCanPrepareInstrument(request.session, request.part.Instrument) {
				p.setError(fmt.Errorf("Carry the confirmed instrument or an instrument case to perform."))
				return
			}
			p.storage.cancel()
			p.storage = nil
			p.preview.stop()
			p.preview = nil
			p.performance.stop()
			p.performance = nil
			p.performance, err = startBardEnsemblePerformance(request.session, request.part.Text, request.part.Instrument, request.partners)
			p.setError(err)
			p.refreshSelection()
		}},
	})
	// Enter dismisses the prompt safely; starting a performance is deliberate.
	buttons := popup.Contents[0].Contents[len(popup.Contents[0].Contents)-1]
	popup.DefaultButton = buttons.Contents[0]
	popup.OnClose = func() {
		popup.RemoveWindow()
		if p.playConfirm == popup {
			p.playConfirm = nil
		}
	}
	p.playConfirm = popup
}

func bardPartnerCompletion(session *Session, value string) string {
	fields := strings.Split(value, ",")
	if len(fields) > 2 {
		return ""
	}
	prefix := strings.TrimLeftFunc(fields[len(fields)-1], unicode.IsSpace)
	if prefix == "" {
		return ""
	}
	key := func(name string) string {
		options, err := bardWithOptions([]string{name})
		if err != nil {
			return ""
		}
		return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(options, "/with ")))
	}
	self := key(session.characterName())
	var candidates []string
	for _, name := range recentPlayerCompletionNamesForSession(session, inputCompletionCandidateLimit) {
		candidate := key(name)
		if candidate == "" || candidate == self {
			continue
		}
		used := false
		for _, previous := range fields[:len(fields)-1] {
			previousKey := key(previous)
			used = used || previousKey != "" && strings.HasPrefix(candidate, previousKey)
		}
		if !used {
			candidates = append(candidates, name)
		}
	}
	return completionCandidateSuffix(prefix, candidates)
}
