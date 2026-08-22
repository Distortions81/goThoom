package main

import (
	"context"
	"time"

	client "github.com/hugolgst/rich-go/client"
)

var discordStart time.Time
var discordReady bool

func initDiscordRPC(ctx context.Context) {
	if err := client.Login("1406171210240360508"); err != nil {
		logError("discord rpc login: %v", err)
		return
	}
	discordReady = true
	discordStart = time.Now()
	setDiscordStatus("main menu")
	go func() {
		<-ctx.Done()
		client.Logout()
	}()
}

func setDiscordStatus(detail string) {
	if !discordReady {
		return
	}
	if err := client.SetActivity(client.Activity{
		State:   "GoThoom",
		Details: detail,
		Timestamps: &client.Timestamps{
			Start: &discordStart,
		},
	}); err != nil {
		logError("discord rpc activity: %v", err)
	}
}

func updateDiscordMode(mode int) {
	switch mode {
	case GAME_MOVIE:
		setDiscordStatus("watching a visionstone")
	case GAME_PLAYING, GAME_PCAP:
		setDiscordStatus("clanning")
	default:
		setDiscordStatus("main menu")
	}
}
