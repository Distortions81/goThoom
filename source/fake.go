package main

import (
	"context"
	"strings"
	"time"
)

// pnTag wraps a name with -pn markers for name parsing.
func pnTag(name string) []byte {
	b := []byte{0xC2, 'p', 'n'}
	b = append(b, []byte(name)...)
	b = append(b, 0xC2, 'p', 'n')
	return b
}

// bepp wraps plain text with a BEPP prefix and NUL terminator.
func bepp(prefix string, msg []byte) []byte {
	b := []byte{0xC2}
	b = append(b, prefix[0], prefix[1])
	b = append(b, msg...)
	b = append(b, 0)
	return b
}

// runFakeMode injects sample share and fallen messages using real server
// formats captured from PCAP data. It allows testing client behavior without
// connecting to the live server.
func runFakeMode(ctx context.Context) {
	go func() {
		select {
		case <-gameStarted:
		case <-ctx.Done():
			return
		}

		p1, p2 := "Bob", "John"

		// Populate simple player descriptors and mobiles so Hero and Bob
		// appear in the player list and on screen without a server
		// connection.
		updatePlayerAppearance(p1, 447, nil, false)
		updatePlayerAppearance(p2, 447, nil, false)
		stateMu.Lock()
		playerIndex = 0
		state.descriptors[0] = frameDescriptor{Index: 0, Type: kDescPlayer, PictID: 447, Name: p1}
		state.descriptors[1] = frameDescriptor{Index: 1, Type: kDescPlayer, PictID: 447, Name: p2}
		state.mobiles[0] = frameMobile{Index: 0, H: 0, V: 0}
		state.mobiles[1] = frameMobile{Index: 1, H: 32, V: 0}
		prepareRenderCacheLocked()
		stateMu.Unlock()
		playersDirty = true

		// Helper to append a bubble and show corresponding chat message.
		emitBubble := func(idx uint8, typ int, name, verb, txt string) {
			words := len(strings.Fields(txt))
			lifeSeconds := gs.BubbleBaseLife + float64(words)*gs.BubbleLifePerWord
			life := int(lifeSeconds * float64(1000/framems))
			if life < 1 {
				life = 1
			}
			b := bubble{Index: idx, Text: txt, Type: typ, CreatedFrame: frameCounter, LifeFrames: life}
			switch typ & kBubbleTypeMask {
			case kBubbleRealAction, kBubblePlayerAction, kBubbleNarrate:
				b.NoArrow = true
			}
			stateMu.Lock()
			state.bubbles = append(state.bubbles, b)
			stateMu.Unlock()
			switch verb {
			case "", bubbleVerbVerbatim:
				chatMessage(txt)
			case bubbleVerbParentheses:
				chatMessage("(" + name + " " + txt + ")")
			default:
				chatMessage(name + " " + verb + ", " + txt)
			}
		}

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		step := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			switch step {
			case 0: // You share Bob
				msg := append([]byte("You are sharing experiences with "), pnTag(p2)...)
				msg = append(msg, '.')
				handleInfoText(append(bepp("sh", msg), '\r'))
			case 1: // Bob shares you
				msg := append(pnTag(p2), []byte(" is sharing experiences with you.")...)
				handleInfoText(append(bepp("sh", msg), '\r'))
			case 2: // Hero speaks
				emitBubble(0, kBubbleNormal, p1, "says", "Hello there!")
			case 3: // Bob whispers
				emitBubble(1, kBubbleWhisper, p2, "whispers", "psst...")
			case 4: // Hero yells
				emitBubble(0, kBubbleYell, p1, "yells", "Watch out!")
			case 5: // Bob thinks
				emitBubble(1, kBubbleThought, p2, "thinks", "I wonder...")
			case 6: // Bob thinks to you
				emitBubble(1, kBubbleThought, p2, "thinks to you", "Hello Hero")
			case 7: // Bob acts
				emitBubble(1, kBubblePlayerAction, p2, bubbleVerbParentheses, "waves excitedly")
			case 8: // Hero real action
				emitBubble(0, kBubbleRealAction, "", bubbleVerbVerbatim, p1+" strikes!")
			case 9: // Bob ponders
				emitBubble(1, kBubblePonder, p2, "ponders", "Hmm...")
			case 10: // Narration
				emitBubble(0, kBubbleNarrate, "", "", "A narrator speaks")
			case 11: // Monster growls
				emitBubble(1, kBubbleMonster, p2, "growls", "Grrr!")
			case 12: // Off-screen bubble
				words := len(strings.Fields("Over here!"))
				lifeSeconds := gs.BubbleBaseLife + float64(words)*gs.BubbleLifePerWord
				life := int(lifeSeconds * float64(1000/framems))
				if life < 1 {
					life = 1
				}
				b := bubble{Index: 1, H: int16(fieldCenterX + 10), V: 0, Far: true, Text: "Over here!", Type: kBubbleNormal, CreatedFrame: frameCounter, LifeFrames: life}
				stateMu.Lock()
				state.bubbles = append(state.bubbles, b)
				stateMu.Unlock()
				chatMessage(p2 + " says, Over here!")
			case 13: // Bob falls
				msg := append(pnTag(p2), []byte(" has fallen")...)
				handleInfoText(append(bepp("hf", msg), '\r'))
			case 14: // Bob recovers
				msg := append(pnTag(p2), []byte(" is no longer fallen")...)
				handleInfoText(append(bepp("nf", msg), '\r'))
			case 15: // You unshare Bob
				msg := append([]byte("You are no longer sharing experiences with "), pnTag(p2)...)
				msg = append(msg, '.')
				handleInfoText(append(bepp("su", msg), '\r'))
			case 16: // Bob unshares you
				msg := append(pnTag(p2), []byte(" is no longer sharing experiences with you.")...)
				handleInfoText(append(bepp("su", msg), '\r'))
			}
			step = (step + 1) % 17
		}
	}()
}
