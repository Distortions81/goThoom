package main

import (
	"context"
	"os"
	"time"

	client "github.com/hugolgst/rich-go/client"
)

func initDiscordRPC(ctx context.Context) {
	appID := os.Getenv("DISCORD_APP_ID")
	if appID == "" {
		return
	}
	if err := client.Login(appID); err != nil {
		logError("discord rpc login: %v", err)
		return
	}
	if err := client.SetActivity(client.Activity{
		State:   "GoThoom",
		Details: "In game",
		Timestamps: &client.Timestamps{
			Start: time.Now(),
		},
	}); err != nil {
		logError("discord rpc activity: %v", err)
	}
	go func() {
		<-ctx.Done()
		client.Logout()
	}()
}
