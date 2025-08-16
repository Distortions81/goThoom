package main

import (
	"context"
	"time"

	client "github.com/hugolgst/rich-go/client"
)

func initDiscordRPC(ctx context.Context) {
	if err := client.Login("1406171210240360508"); err != nil {
		logError("discord rpc login: %v", err)
		return
	}
	now := time.Now()
	if err := client.SetActivity(client.Activity{
		State:   "GoThoom",
		Details: "In game",
		Timestamps: &client.Timestamps{
			Start: &now,
		},
	}); err != nil {
		logError("discord rpc activity: %v", err)
	}
	go func() {
		<-ctx.Done()
		client.Logout()
	}()
}
