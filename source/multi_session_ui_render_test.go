package main

import (
	"fmt"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"testing"

	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

// Run alone because it starts Ebitengine's game loop. Besides exporting a
// review image, it validates the final rendered action rectangles rather than
// relying only on the logical flow structure.
func TestRenderSessionTabActions(t *testing.T) {
	dir := os.Getenv("GOTHOOM_SESSION_TAB_RENDER_DIR")
	if dir == "" {
		t.Skip("set GOTHOOM_SESSION_TAB_RENDER_DIR to export the session tab review image")
	}
	initFont()
	eui.SetScreenSize(1000, 180)
	eui.SetUIScale(2)
	if err := eui.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	if err := eui.LoadStyle("Default"); err != nil {
		t.Fatal(err)
	}
	fake, clmov, pcapPath = false, "", ""
	firstSession := mustNewSession(primarySessionID)
	primarySession = firstSession
	appSessions = newSessionManager(firstSession)
	appViewports = newViewportManager()
	names := []string{"Mossy Boots", "Puddle Jumper", "Moon Bean", "Fern Friend"}
	var connections []net.Conn
	for index, character := range names {
		var session *Session
		if index == 0 {
			session = firstSession
		} else {
			var ok bool
			session, ok = appSessions.addSession()
			if !ok {
				t.Fatalf("add session %d", index+1)
			}
		}
		session.setCharacterName(character)
		client, server := net.Pipe()
		connections = append(connections, client, server)
		session.transport.mu.Lock()
		session.transport.tcp = client
		session.transport.status = sessionConnected
		session.transport.mu.Unlock()
	}
	t.Cleanup(func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	})
	appSessions.selectSession(2)
	sessionMusicIndicators = [maxSessions]bool{}
	sessionMusicIndicators[2] = true

	gameWin = newGameRenderWindow()
	gameWin.Title = "Session tabs"
	gameWin.Size = eui.Point{X: 1000, Y: 160}
	gameWin.AddWindow(false)
	gameWin.MarkOpen()
	sessionTabBar = nil
	refreshSessionTabs()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	game := &sessionTabRenderGame{path: filepath.Join(dir, "session-tabs.png")}
	if err := ebiten.RunGame(game); err != nil {
		t.Fatal(err)
	}
	if game.err != nil {
		t.Fatal(game.err)
	}
}

type sessionTabRenderGame struct {
	done bool
	err  error
	path string
}

func (g *sessionTabRenderGame) Layout(_, _ int) (int, int) { return 1000, 180 }

func (g *sessionTabRenderGame) Update() error {
	if g.done {
		return ebiten.Termination
	}
	return nil
}

func (g *sessionTabRenderGame) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	g.done = true
	loadMaterialIcons()
	refreshSessionTabs()
	screen.Fill(eui.NewColor(32, 32, 32, 255).ToRGBA())
	eui.Draw(screen)
	for index, segment := range sessionTabBar.Contents[:appSessions.count()] {
		if len(segment.Contents) < 3 {
			g.err = fmt.Errorf("tab %d has %d controls, want surface, label, and close", index+1, len(segment.Contents))
			return
		}
		for _, action := range segment.Contents[2:] {
			if action.DrawRect.X0 < segment.DrawRect.X0 || action.DrawRect.X1 > segment.DrawRect.X1 ||
				action.DrawRect.Y0 < segment.DrawRect.Y0 || action.DrawRect.Y1 > segment.DrawRect.Y1 {
				g.err = fmt.Errorf("tab %d action escaped tab: action=%+v tab=%+v", index+1, action.DrawRect, segment.DrawRect)
				return
			}
		}
	}
	file, err := os.Create(g.path)
	if err != nil {
		g.err = err
		return
	}
	g.err = png.Encode(file, screen)
	_ = file.Close()
}
