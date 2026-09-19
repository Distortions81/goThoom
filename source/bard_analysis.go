package main

import (
	"fmt"
	"strings"
	"time"
)

func validateBardEnsembleScore(score bardScore, partners string) error {
	if _, err := validateBardScore(score); err != nil {
		return err
	}
	if strings.TrimSpace(partners) == "" {
		return nil
	}
	names := strings.Split(partners, ",")
	for _, part := range score.Parts {
		if _, err := bardEnsembleCommands(part.Text, names); err != nil {
			return fmt.Errorf("Part %s with partners %s: %w", part.Name, partners, err)
		}
	}
	return nil
}

func bardPartDuration(part bardPart) (time.Duration, error) {
	tokens, err := bardNoteTokens(part.Text)
	if err != nil {
		return 0, err
	}
	if part.Instrument < 0 || part.Instrument >= len(instruments) {
		return 0, fmt.Errorf("Choose an instrument.")
	}
	_, duration, parseErr := parseClassicTuneTimeline(strings.Join(tokens, ""), instruments[part.Instrument], 120, 100)
	if parseErr != nil {
		return 0, parseErr
	}
	return duration, nil
}

func bardTimingSummary(score bardScore) (string, []string) {
	var lines, warnings []string
	var shortest, longest time.Duration
	for i, part := range score.Parts {
		duration, err := bardPartDuration(part)
		if err != nil {
			lines = append(lines, part.Name+": check notation")
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %.2f seconds", part.Name, duration.Seconds()))
		if i == 0 || duration < shortest {
			shortest = duration
		}
		if duration > longest {
			longest = duration
		}
	}
	if len(score.Parts) > 1 && longest-shortest > time.Millisecond {
		warnings = append(warnings, fmt.Sprintf("Parts end %.2f seconds apart. Check rests and tempo changes if they should finish together.", (longest-shortest).Seconds()))
	}
	return strings.Join(lines, "\n"), warnings
}

func (p *bardPanel) refreshEnsembleAnalysis() {
	share := &p.sharing
	if share.analysis == nil {
		return
	}
	tune := p.tune()
	if tune == nil {
		share.analysis.SetWrappedText("Select a song to inspect its parts.")
		return
	}
	summary, warnings := bardTimingSummary(tune.Score)
	used := map[string]string{p.selectedPart: "you"}
	names := strings.Split(p.partners.Text, ",")
	for i, choice := range share.assignments {
		if i >= len(names) || strings.TrimSpace(names[i]) == "" || choice.Selected <= 0 || choice.Selected > len(tune.Score.Parts) {
			continue
		}
		name := strings.TrimSpace(names[i])
		part := tune.Score.Parts[choice.Selected-1].Name
		if previous, duplicate := used[part]; duplicate {
			warnings = append(warnings, fmt.Sprintf("%s is assigned to both %s and %s. Doubling a part is allowed.", part, previous, name))
		}
		used[part] = name
	}
	if err := validateBardEnsembleScore(tune.Score, p.partners.Text); err != nil {
		warnings = append(warnings, err.Error())
	}
	for _, warning := range warnings {
		summary += "\n\nWarning: " + warning
	}
	summary += "\n\nEach part defaults to 120 BPM. Use @60 through @180 to set tempo; tempo changes and rests apply separately to each part. Different endings and doubled parts can be intentional."
	share.analysis.SetWrappedText(summary)
}
