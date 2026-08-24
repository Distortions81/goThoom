//go:build script

package main

import (
	"fmt"
	"time"

	"gt2"
)

const scriptName = "Daily Reminder"
const scriptID = "daily-reminder"
const scriptAuthor = "Examples"
const scriptCategory = "Quality Of Life"
const scriptAPIVersion = 2

func Init() {
	gt2.Repeat(24*time.Hour, remind)
}

func remind() {
	count := gt2.LoadInteger("reminder-count", 0) + 1
	gt2.Store("reminder-count", count)
	gt2.Print(fmt.Sprintf("Daily reminder %d", count))
}
