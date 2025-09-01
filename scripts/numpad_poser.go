//go:build plugin

package main

import "gt"

// Numpad Poser – hit keypad numbers to strike poses quickly.
//
// Notes for non‑technical players:
// - Press 1–9 on the numeric keypad (NumLock on).
// - Each key sends a /pose command so nearby players can see it.
//
// Plugin metadata
const PluginName = "Numpad Poser"
const PluginAuthor = "Examples"
const PluginCategory = "Fun"
const PluginAPIVersion = 1

// Init binds each number key on the keypad to a fun pose.
func Init() {
	gt.AddHotkeyFn("Numpad1", npPose1)
	gt.AddHotkeyFn("Numpad2", npPose2)
	gt.AddHotkeyFn("Numpad3", npPose3)
	gt.AddHotkeyFn("Numpad4", npPose4)
	gt.AddHotkeyFn("Numpad5", npPose5)
	gt.AddHotkeyFn("Numpad6", npPose6)
	gt.AddHotkeyFn("Numpad7", npPose7)
	gt.AddHotkeyFn("Numpad8", npPose8)
	gt.AddHotkeyFn("Numpad9", npPose9)
}

func npPose1(e gt.HotkeyEvent) { gt.RunCommand("/pose leanleft") }
func npPose2(e gt.HotkeyEvent) { gt.RunCommand("/pose akimbo") }
func npPose3(e gt.HotkeyEvent) { gt.RunCommand("/pose leanright") }
func npPose4(e gt.HotkeyEvent) { gt.RunCommand("/pose kneel") }
func npPose5(e gt.HotkeyEvent) { gt.RunCommand("/pose sit") }
func npPose6(e gt.HotkeyEvent) { gt.RunCommand("/pose angry") }
func npPose7(e gt.HotkeyEvent) { gt.RunCommand("/pose lie") }
func npPose8(e gt.HotkeyEvent) { gt.RunCommand("/pose seated") }
func npPose9(e gt.HotkeyEvent) { gt.RunCommand("/pose celebrate") }
