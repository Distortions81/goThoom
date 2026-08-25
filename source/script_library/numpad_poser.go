//go:build script

package main

import "gt2"

// Numpad Poser – hit keypad numbers to strike poses quickly.
//
// Notes for non‑technical players:
// - Press 1–9 on the numeric keypad (NumLock on).
// - Each key sends a /pose command so nearby players can see it.
//
// script metadata
const scriptName = "Numpad Poser"
const scriptID = "numpad-poser"
const scriptAuthor = "Examples"
const scriptCategory = "Fun"
const scriptAPIVersion = 2

// Init binds each number key on the keypad to a fun pose.
func Init() {
	gt2.Bind("Numpad1", npPose1)
	gt2.Bind("Numpad2", npPose2)
	gt2.Bind("Numpad3", npPose3)
	gt2.Bind("Numpad4", npPose4)
	gt2.Bind("Numpad5", npPose5)
	gt2.Bind("Numpad6", npPose6)
	gt2.Bind("Numpad7", npPose7)
	gt2.Bind("Numpad8", npPose8)
	gt2.Bind("Numpad9", npPose9)
}

func npPose1(gt2.InputEvent) { gt2.Send("/pose leanleft") }
func npPose2(gt2.InputEvent) { gt2.Send("/pose akimbo") }
func npPose3(gt2.InputEvent) { gt2.Send("/pose leanright") }
func npPose4(gt2.InputEvent) { gt2.Send("/pose kneel") }
func npPose5(gt2.InputEvent) { gt2.Send("/pose sit") }
func npPose6(gt2.InputEvent) { gt2.Send("/pose angry") }
func npPose7(gt2.InputEvent) { gt2.Send("/pose lie") }
func npPose8(gt2.InputEvent) { gt2.Send("/pose seated") }
func npPose9(gt2.InputEvent) { gt2.Send("/pose celebrate") }
