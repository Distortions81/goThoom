package main

import (
	"testing"

	"gothoom/eui"
)

func TestSessionNotificationCarriesVisibleSource(t *testing.T) {
	initFont()
	originalSettings := gs
	originalWindow := gameWin
	originalNotifications := notifications
	gs = gsdef
	gs.Notifications = true
	gs.NotificationBeep = false
	gs.ChatTTSNotifications = false
	gameWin = eui.NewWindow()
	notifications = nil
	t.Cleanup(func() {
		gs = originalSettings
		gameWin = originalWindow
		notifications = originalNotifications
	})

	session := mustNewSession(2)
	session.setCharacterName("Second Hero")
	showSessionNotification(session, "Alice has fallen", 72, 69, 65)

	if len(notifications) != 1 {
		t.Fatalf("notifications = %d, want 1", len(notifications))
	}
	notification := notifications[0]
	if notification.source != 2 {
		t.Fatalf("notification source = %d, want 2", notification.source)
	}
	if got, want := notification.item.Text, "Second Hero: Alice has fallen"; got != want {
		t.Fatalf("notification text = %q, want %q", got, want)
	}
}

func TestSecondaryFallenNotificationUsesOwningSession(t *testing.T) {
	initFont()
	originalSettings := gs
	originalWindow := gameWin
	originalNotifications := notifications
	gs = gsdef
	gs.Notifications = true
	gs.NotificationBeep = false
	gs.NotifyFallen = true
	gameWin = eui.NewWindow()
	notifications = nil
	t.Cleanup(func() {
		gs = originalSettings
		gameWin = originalWindow
		notifications = originalNotifications
	})

	session := mustNewSession(2)
	session.setCharacterName("Second Hero")
	raw := append([]byte{0xc2, 'p', 'n'}, []byte("Alice")...)
	if !session.parseFallenText(raw, "Alice has fallen") {
		t.Fatal("secondary fallen message was not handled")
	}
	if len(notifications) != 1 || notifications[0].source != 2 {
		t.Fatalf("secondary notifications = %#v", notifications)
	}
}
