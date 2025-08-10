package main

import "github.com/Distortions81/EUI/eui"

// initWindow applies common window settings such as size, position,
// pinning and basic flags.
func initWindow(data *eui.WindowData, cfg WindowState, pin eui.Pin) {
	if cfg.Size.X > 0 && cfg.Size.Y > 0 {
		data.Size = eui.Point{X: float32(cfg.Size.X), Y: float32(cfg.Size.Y)}
	}
	if cfg.Position.X != 0 || cfg.Position.Y != 0 {
		data.Position = eui.Point{X: float32(cfg.Position.X), Y: float32(cfg.Position.Y)}
	}
	data.Closable = true
	data.Resizable = true
	data.AutoSize = false
	data.Movable = false
	data.Open = true
	data.PinTo = pin
}
