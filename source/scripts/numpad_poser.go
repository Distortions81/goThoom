//go:build script

package main

import "gt"

// Numpad Poser – hit keypad numbers to strike poses quickly.
//
// Notes for non‑technical players:
// - Press 1–9 on the numeric keypad (NumLock on).
// - Each key sends a /pose command so nearby players can see it.
//
// script metadata
const scriptName = "Numpad Poser"
const scriptAuthor = "Examples"
const scriptCategory = "Fun"
const scriptAPIVersion = 1

// Init binds each number key on the keypad to a fun pose.
func Init() {
	gt.Bind("Numpad1", npPose1)
	gt.Bind("Numpad2", npPose2)
	gt.Bind("Numpad3", npPose3)
	gt.Bind("Numpad4", npPose4)
	gt.Bind("Numpad5", npPose5)
	gt.Bind("Numpad6", npPose6)
	gt.Bind("Numpad7", npPose7)
	gt.Bind("Numpad8", npPose8)
	gt.Bind("Numpad9", npPose9)
}

func npPose1(gt.InputEvent) { gt.Send("/pose leanleft") }
func npPose2(gt.InputEvent) { gt.Send("/pose akimbo") }
func npPose3(gt.InputEvent) { gt.Send("/pose leanright") }
func npPose4(gt.InputEvent) { gt.Send("/pose kneel") }
func npPose5(gt.InputEvent) { gt.Send("/pose sit") }
func npPose6(gt.InputEvent) { gt.Send("/pose angry") }
func npPose7(gt.InputEvent) { gt.Send("/pose lie") }
func npPose8(gt.InputEvent) { gt.Send("/pose seated") }
func npPose9(gt.InputEvent) { gt.Send("/pose celebrate") }
