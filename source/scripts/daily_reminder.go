//go:build script

package main

import (
	"fmt"
	"strings"
	"time"

	"gt2"
)

const scriptName = "Daily Reminder"
const scriptID = "daily-reminder"
const scriptAuthor = "Examples"
const scriptCategory = "Quality Of Life"
const scriptDescription = "Shows how to make a daily task survive reloads and app restarts."
const scriptAPIVersion = 2

const reminderDateFormat = "2006-01-02"

var reminderCharacter string

func Init() {
	setReminderCharacter(gt2.Self().Name)
	gt2.OnLogin(func(event gt2.LifecycleEvent) {
		setReminderCharacter(event.Character)
	})
	gt2.OnLogout(func(event gt2.LifecycleEvent) {
		if event.Character == "" || strings.EqualFold(event.Character, reminderCharacter) {
			reminderCharacter = ""
		}
	})
	gt2.Repeat(time.Minute, func() {
		checkReminder(reminderCharacter)
	})
}

func setReminderCharacter(character string) {
	reminderCharacter = strings.TrimSpace(character)
	checkReminder(reminderCharacter)
}

func reminderKey(name, character string) string {
	return name + ":" + strings.ToLower(strings.TrimSpace(character))
}

func checkReminder(character string) {
	character = strings.TrimSpace(character)
	if character == "" {
		return
	}
	dateKey := reminderKey("last-reminder-date", character)
	today := time.Now().Format(reminderDateFormat)
	lastReminder := gt2.LoadString(dateKey, "")
	if lastReminder == "" {
		// Start counting from today instead of firing immediately when the script
		// is first installed.
		gt2.Store(dateKey, today)
		return
	}
	if lastReminder == today {
		return
	}

	// Store the date before running the task so another timer callback cannot
	// repeat it during this session. Storage is restored on the next launch.
	gt2.Store(dateKey, today)
	remind(character)
}

func remind(character string) {
	countKey := reminderKey("reminder-count", character)
	count := gt2.LoadInteger(countKey, 0) + 1
	gt2.Store(countKey, count)
	gt2.Print(fmt.Sprintf("Daily reminder for %s: %d", character, count))
}
