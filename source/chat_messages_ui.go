//go:build !test

package main

import (
	"context"
	"sync/atomic"
	"time"

	"gothoom/eui"

	clipboard "golang.design/x/clipboard"
)

var chatWin *eui.WindowData
var chatList *eui.ItemData
var chatHighlighted *eui.ItemData
var chatWindowUpdateQueued atomic.Bool
var chatWindowMessages messageWindowState
var chatWindowForceFull bool

// queueChatWindowUpdate coalesces message-driven refreshes and keeps EUI tree
// mutations on the game loop. Chat can arrive on a network goroutine while the
// existing window is being drawn.
func queueChatWindowUpdate() {
	if !chatWindowUpdateQueued.CompareAndSwap(false, true) {
		return
	}
	dispatchMainThread(func() {
		chatWindowUpdateQueued.Store(false)
		updateChatWindow()
	})
}

func updateChatWindow() {
	if chatWin == nil || !chatWin.IsOpen() {
		return
	}

	scrollit := chatList.ScrollAtBottom()

	format := gs.TimestampFormat
	msgs, types, firstChanged := chatWindowMessages.Sync(&chatLog, format, gs.ChatTimestamps, chatWindowForceFull)
	chatWindowForceFull = false
	if chatWindowMessages.dropped > 0 && !chatWindowMessages.reset && chatWindowMessages.dropped <= len(chatList.Contents) {
		chatList.SetItems(chatList.Contents[chatWindowMessages.dropped:])
	}
	updateTextWindowFrom(chatWin, chatList, nil, msgs, gs.ChatFontSize, "", nil, gs.ChatAlternatingRowColors, &chatTextWrapCache, firstChanged)
	if chatWin.SearchText != "" {
		applyTextWindowSearch(chatList, chatWin.SearchText)
	}
	if chatList != nil {
		styleStart := firstChanged
		if chatWindowMessages.reset {
			styleStart = 0
		}
		for i := styleStart; i < len(msgs); i++ {
			msg := msgs[i]
			if i < len(types) {
				chatList.Contents[i].TextColor, chatList.Contents[i].ForceTextColor = messageTextStyleForMessage(types[i], msg)
			}
			if !gs.ClassicMessageColors && chatHasPlayerTag(msg) {
				chatList.Contents[i].TextColor = eui.AccentColor()
				chatList.Contents[i].ForceTextColor = true
			}
		}
		if chatWindowMessages.dropped > 0 && chatWin.SearchText == "" {
			for i, row := range chatList.Contents {
				restoreAlternateTextRow(row, i, gs.ChatAlternatingRowColors)
			}
		}
		// Auto-scroll list to bottom on new messages
		if scrollit {
			chatList.Scroll.Y = 1e9
		}
		chatWin.RefreshWithReason("chat content")
	}
}

func makeChatWindow() error {
	if gs.MessagesToConsole {
		return nil
	}
	if chatWin != nil {
		return nil
	}
	chatWin, chatList, _ = newTextWindow("Chat", eui.HZoneRight, eui.VZoneBottom, false, updateChatWindow)
	chatWin.Searchable = true
	chatWin.OnSearch = func(s string) { searchTextWindow(chatWin, chatList, s) }
	chatWin.OnOpen = updateChatWindow
	updateChatWindow()
	return nil
}

// handleChatCopyRightClick copies the clicked chat line to the clipboard,
// highlights it, and optionally shows a notification. Returns true if a line
// was found under the cursor.
func handleChatCopyRightClick(mx, my int) bool {
	if chatWin == nil || chatList == nil || !chatWin.IsOpen() {
		return false
	}
	pos := eui.Point{X: float32(mx), Y: float32(my)}
	for _, row := range chatList.Contents {
		r := row.DrawRect
		if pos.X >= r.X0 && pos.X <= r.X1 && pos.Y >= r.Y0 && pos.Y <= r.Y1 {
			// Clear previous highlights in chat list.
			for i, it := range chatList.Contents {
				restoreAlternateTextRow(it, i, gs.ChatAlternatingRowColors)
				it.Focused = false
			}
			// Highlight selected line briefly and copy the text.
			row.Filled = true
			row.Focused = true
			chatHighlighted = row
			chatWin.Refresh()
			scheduleChatUnhighlight(row)
			if row.Text != "" {
				_, _ = clipboard.Write(context.Background(), clipboard.FmtText, []byte(row.Text))
				if gs.NotifyCopyText {
					showNotification("text copied")
				}
			}
			return true
		}
	}
	return false
}

func scheduleChatUnhighlight(row *eui.ItemData) {
	go func(target *eui.ItemData) {
		time.Sleep(1200 * time.Millisecond)
		dispatchMainThread(func() {
			if chatHighlighted == target {
				for i, it := range chatList.Contents {
					if it == target {
						restoreAlternateTextRow(it, i, gs.ChatAlternatingRowColors)
						break
					}
				}
				target.Focused = false
				if chatWin != nil {
					chatWin.Refresh()
				}
				chatHighlighted = nil
			}
		})
	}(row)
}
