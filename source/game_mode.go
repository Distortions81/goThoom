package main

import "time"

const IdleLimit = time.Minute * 1

const (
	GAME_BOOT = iota
	GAME_MAIN
	GAME_DOWNLOAD
	GAME_ACCOUNT

	//Here or higher is "playing"
	GAME_PLAYING
	GAME_MOVIE
	GAME_PCAP
)

var gameMode int
var lastActive time.Time

var gameModeNames []string = []string{
	"goThoom loading",
	"Main menu",
	"Downloading updates",
	"Account management",

	//Here or higher is "playing"
	"Clanning",
	"Watching a visionstone",
	"goThoom dev",
}

func CLANNING() bool {
	return gameMode >= GAME_PLAYING
}

func MODE_NAME() string {
	if time.Since(lastActive) > IdleLimit {
		return "Idle"
	}
	return gameModeNames[gameMode]
}

func CHANGE_MODE(mode int) {
	gameMode = mode
	updateDiscordMode(mode)
}
