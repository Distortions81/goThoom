//go:build script

package main

import "gt2"

// Change scriptID to a permanent ID that is unique to your script. Do not
// change it after sharing the script because settings and storage use this ID.
const scriptID = "your-name.my-script"
const scriptName = "My Script"
const scriptAuthor = "Your Name"
const scriptCategory = "Utilities"
const scriptDescription = "A starting point for a goThoom script."
const scriptAPIVersion = 2

func Init() {
	gt2.Command("hello", func(args string) {
		if args == "" {
			args = "world"
		}
		gt2.Print("Hello, " + args + "!")
	})

	gt2.Bind("Ctrl-H", func(event gt2.InputEvent) {
		event.Consume()
		gt2.ShowNotification("Hello from My Script")
	})
}
