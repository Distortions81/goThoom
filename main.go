package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"syscall"
	"time"

	"go_client/climg"
	"go_client/clsnd"
)

var (
	clMovFPS int = 5
	dataDir  string

	host        string = "server.deltatao.com:5010"
	account     string
	accountPass string
	name        string
	pass        string
	passHash    string

	demo          bool
	clmov         string
	baseDir       string
	blockSound    bool
	blockBubbles  bool
	clientVersion int
)

func main() {
	flag.StringVar(&clmov, "clmov", "", "play back a .clMov file")
	clientVer := flag.Int("client-version", 1445, "client version number (for testing)")
	flag.BoolVar(&debug, "debug", false, "verbose/debug logging")
	genPGO := flag.Bool("pgo", false, "create default.pgo using test.clMov at 60 fps for 30s")
	flag.Parse()
	clientVersion = *clientVer

	if *genPGO {
		clmov = filepath.Join("clmovFiles", "test.clMov")
		clMovFPS = 30
	}

	baseDir = os.Getenv("PWD")
	if baseDir == "" {
		var err error
		if baseDir, err = os.Getwd(); err != nil {
			log.Fatalf("get working directory: %v", err)
		}
	}

	loadSettings()
	loadCharacters()
	initSoundContext()
	applySettings()
	setupLogging(debug)
	defer func() {
		if r := recover(); r != nil {
			logError("panic: %v\n%s", r, debug.Stack())
		}
	}()

	clmovPath := ""
	if clmov != "" {
		if filepath.IsAbs(clmov) {
			clmovPath = clmov
		} else {
			clmovPath = filepath.Join(baseDir, clmov)
		}
	}

	dataDir = filepath.Join(baseDir, "data")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if *genPGO {
		f, err := os.Create(baseDir + "/default.pgo")
		if err != nil {
			log.Fatalf("create default.pgo: %v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatalf("start CPU profile: %v", err)
		}
		defer func() {
			pprof.StopCPUProfile()
			f.Close()
		}()
		go func() {
			time.Sleep(30 * time.Second)
			cancel()
		}()
	}

	go func() {
		runGame(ctx)
		cancel()
	}()
	addMessage("Starting...")

	var err error
	clImages, err = climg.Load(filepath.Join(baseDir + "/data/CL_Images"))
	if err != nil {
		logError("failed to load CL_Images: %v", err)
	}

	clSounds, err = clsnd.Load(filepath.Join(baseDir + "/data/CL_Sounds"))
	if err != nil {
		logError("failed to load CL_Sounds: %v", err)
	}

	if gs.PrecacheAssets {
		go precacheAssets()
	}

	if clmovPath != "" {
		drawStateEncrypted = false
		frames, err := parseMovie(clmovPath, *clientVer)
		if err != nil {
			log.Fatalf("parse movie: %v", err)
		}

		playerName = extractMoviePlayerName(frames)

		mp := newMoviePlayer(frames, clMovFPS, cancel)
		mp.initUI()

		if gs.PrecacheAssets && !assetsPrecached {
			for !assetsPrecached {
				time.Sleep(time.Millisecond * 100)
			}
		}
		go mp.run(ctx)

		<-ctx.Done()
		return
	}

	<-ctx.Done()
}

func extractMoviePlayerName(frames [][]byte) string {
	for _, m := range frames {
		if len(m) >= 2 && binary.BigEndian.Uint16(m[:2]) == 2 {
			data := append([]byte(nil), m[2:]...)
			if n := playerFromDrawState(data); n != "" {
				return n
			}
			simpleEncrypt(data)
			if n := playerFromDrawState(data); n != "" {
				return n
			}
		}
	}

	for _, m := range frames {
		if len(m) >= 2 && binary.BigEndian.Uint16(m[:2]) == 2 {
			data := append([]byte(nil), m[2:]...)
			if n := firstDescriptorName(data); n != "" {
				return n
			}
			simpleEncrypt(data)
			if n := firstDescriptorName(data); n != "" {
				return n
			}
		}
	}
	return ""
}

func playerFromDrawState(data []byte) string {
	if len(data) < 9 {
		return ""
	}
	p := 9
	if len(data) <= p {
		return ""
	}
	descCount := int(data[p])
	p++
	descs := make(map[uint8]struct {
		Type uint8
		Name string
	}, descCount)
	for i := 0; i < descCount && p < len(data); i++ {
		if p+4 > len(data) {
			return ""
		}
		idx := data[p]
		typ := data[p+1]
		p += 4
		if off := bytes.IndexByte(data[p:], 0); off >= 0 {
			name := string(data[p : p+off])
			p += off + 1
			if p >= len(data) {
				return ""
			}
			cnt := int(data[p])
			p++
			if p+cnt > len(data) {
				return ""
			}
			p += cnt
			descs[idx] = struct {
				Type uint8
				Name string
			}{typ, name}
		} else {
			return ""
		}
	}
	if len(data) < p+7 {
		return ""
	}
	p += 7
	if len(data) <= p {
		return ""
	}
	pictCount := int(data[p])
	p++
	if pictCount == 255 {
		if len(data) < p+2 {
			return ""
		}
		// skip pictAgain
		pictCount = int(data[p+1])
		p += 2
	}
	br := bitReader{data: data[p:]}
	for i := 0; i < pictCount; i++ {
		if _, ok := br.readBits(14); !ok {
			return ""
		}
		if _, ok := br.readBits(11); !ok {
			return ""
		}
		if _, ok := br.readBits(11); !ok {
			return ""
		}
	}
	p += br.bitPos / 8
	if br.bitPos%8 != 0 {
		p++
	}
	if len(data) <= p {
		return ""
	}
	mobileCount := int(data[p])
	p++
	for i := 0; i < mobileCount && p+7 <= len(data); i++ {
		idx := data[p]
		h := int16(binary.BigEndian.Uint16(data[p+2:]))
		v := int16(binary.BigEndian.Uint16(data[p+4:]))
		p += 7
		if h == 0 && v == 0 {
			if d, ok := descs[idx]; ok && d.Type == kDescPlayer {
				playerIndex = idx
				return d.Name
			}
		}
	}
	return ""
}

func firstDescriptorName(data []byte) string {
	if len(data) < 10 {
		return ""
	}
	p := 9
	if len(data) <= p {
		return ""
	}
	descCount := int(data[p])
	p++
	if descCount == 0 || p >= len(data) {
		return ""
	}
	if p+4 > len(data) {
		return ""
	}
	p += 4
	if idx := bytes.IndexByte(data[p:], 0); idx >= 0 {
		return string(data[p : p+idx])
	}
	return ""
}
