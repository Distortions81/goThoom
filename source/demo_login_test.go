package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestNextDemoCandidateIndexRetriesOnlineCharacter(t *testing.T) {
	err := fmt.Errorf("login candidate: %w", &loginResultError{result: loginResultCharacterAlreadyOnline})
	if next, ok := nextDemoCandidateIndex(err, 0, 3); !ok || next != 1 {
		t.Fatalf("next demo candidate = %d, %v; want 1, true", next, ok)
	}
	if next, ok := nextDemoCandidateIndex(err, 2, 3); ok || next != 3 {
		t.Fatalf("exhausted demo candidates = %d, %v; want 3, false", next, ok)
	}
}

func TestNextDemoCandidateIndexDoesNotHideOtherLoginErrors(t *testing.T) {
	err := &loginResultError{result: -30998}
	if next, ok := nextDemoCandidateIndex(err, 0, 3); ok || next != 0 {
		t.Fatalf("non-online login error advanced to %d, %v; want 0, false", next, ok)
	}
}

func TestDemoLoginExhaustedError(t *testing.T) {
	if got, want := errDemoSlotsUsed.Error(), "Sorry, all demo slots seem to be used."; got != want {
		t.Fatalf("exhausted demo error = %q, want %q", got, want)
	}
}

func TestParseDemoCharacterNamesDeduplicatesNames(t *testing.T) {
	data := append(make([]byte, 12), []byte("Agratis One\x00agratis one\x00 Agratis Two \x00\x00")...)
	if got, want := parseDemoCharacterNames(data), []string{"Agratis One", "Agratis Two"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("demo character names = %v, want %v", got, want)
	}
}

func TestDemoLoginRetriesOnlineCandidates(t *testing.T) {
	server := newFakeServerWithResults(t, loginResultCharacterAlreadyOnline, 0)
	defer server.close()

	originalHost := host
	originalDataDir := dataDirPath
	originalName := name
	originalPass := pass
	originalPassHash := passHash
	host = server.addr()
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		host = originalHost
		dataDirPath = originalDataDir
		name = originalName
		pass = originalPass
		passHash = originalPassHash
	})

	candidates := []string{"Agratis One", "Agratis Two", "Agratis Three"}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := loginWithDemoCandidates(ctx, 1, candidates); err != nil {
		t.Fatalf("demo login: %v", err)
	}

	if got, want := server.attemptedLoginNames(), candidates[:2]; !reflect.DeepEqual(got, want) {
		t.Fatalf("attempted demo characters = %v, want %v", got, want)
	}
	if name != candidates[1] {
		t.Fatalf("successful demo character = %q, want %q", name, candidates[1])
	}
}

func TestDemoLoginTriesEachCandidateOnceBeforeExhaustedError(t *testing.T) {
	server := newFakeServerWithResults(t, loginResultCharacterAlreadyOnline)
	defer server.close()

	originalHost := host
	originalDataDir := dataDirPath
	originalName := name
	originalPass := pass
	originalPassHash := passHash
	host = server.addr()
	dataDirPath = t.TempDir()
	t.Cleanup(func() {
		host = originalHost
		dataDirPath = originalDataDir
		name = originalName
		pass = originalPass
		passHash = originalPassHash
	})

	candidates := []string{"Agratis One", "Agratis Two", "Agratis Three"}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := loginWithDemoCandidates(ctx, 1, candidates)
	if !errors.Is(err, errDemoSlotsUsed) {
		t.Fatalf("demo login error = %v, want %v", err, errDemoSlotsUsed)
	}
	if got := server.attemptedLoginNames(); !reflect.DeepEqual(got, candidates) {
		t.Fatalf("attempted demo characters = %v, want each once: %v", got, candidates)
	}
}
