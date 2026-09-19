package main

import "time"

// Protected by sessionMusicState.mu. Server /me identifies the performer;
// matching the assembled notes avoids mistaking an older song for this one.
type bardMusicWatch struct {
	notes            []Note
	program          int
	who              int
	ownSeen          bool
	started          time.Time
	duration         time.Duration
	expectedDuration time.Duration
	stopped          bool
}

func (s *sessionMusicState) observeBardCommand(mp MusicParams) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.bardWatch
	if w == nil {
		return false
	}
	if mp.Stop {
		if w.ownSeen && (mp.Who == 0 || mp.Who == w.who) {
			w.stopped = true
		}
	} else if mp.Me && mp.Who != 0 {
		w.who, w.ownSeen = mp.Who, true
	}
	return true
}

func (s *sessionMusicState) observeBardStart(jobs []tuneJob, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.bardWatch
	if w == nil || !w.ownSeen || w.stopped || !w.started.IsZero() {
		return
	}
	for _, job := range jobs {
		if job.who != w.who || job.program != w.program || job.duration != w.expectedDuration || len(job.notes) != len(w.notes) {
			continue
		}
		matches := true
		for i, note := range job.notes {
			want := w.notes[i]
			matches = matches && note.Key == want.Key && note.Start == want.Start && note.Duration == want.Duration
		}
		if matches {
			w.started = now
			w.duration = job.duration
			return
		}
	}
}

func (p *bardPerformance) clearMusicWatch() {
	if p == nil || p.watch == nil {
		return
	}
	state := p.session.music
	state.mu.Lock()
	if state.bardWatch == p.watch {
		state.bardWatch = nil
	}
	state.mu.Unlock()
}
