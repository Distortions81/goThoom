package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"syscall"
	"time"

	"gothoom/climg"
	"gothoom/clsnd"
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	clipboard "golang.design/x/clipboard"

	_ "embed"
)

var (
	//go:embed goThoom.png
	windowIconPNG []byte

	// Default movie playback FPS; classic client updates ~10Hz.
	clMovFPS int = 5

	host     string = "server.deltatao.com:5010"
	name     string
	pass     string
	passHash string

	clmov         string
	pcapPath      string
	fake          bool
	blockSound    bool
	blockBubbles  bool
	blockTTS      bool
	blockMusic    bool
	dumpMusic     bool
	imgDump       bool
	sndDump       bool
	dumpBEPPTags  bool
	musicDebug    bool
	clientVersion int
	experimental  bool
	showUIScale   bool
	hdTextures    bool
)

func main() {
	clientVersion = clVersion
    dumpTune := flag.String("dumpTune", "", "dump parsed note timings for the given tune string and exit")
    dumpTempo := flag.Int("dumpTempo", 120, "tempo for -dumpTune (BPM)")
    dumpInst := flag.Int("dumpInst", defaultInstrument, "instrument index for -dumpTune")
	flag.StringVar(&clmov, "clmov", "", "play back a .clMov file")
	flag.StringVar(&pcapPath, "pcap", "", "replay network frames from a .pcap/.pcapng file")
	flag.BoolVar(&fake, "fake", false, "simulate server messages without connecting")
	flag.BoolVar(&doDebug, "debug", false, "verbose/debug logging")
	flag.BoolVar(&eui.CacheCheck, "cacheCheck", false, "display window and item render counts")
	flag.BoolVar(&dumpMusic, "dumpMusic", false, "write played music as a .wav file")
	flag.BoolVar(&imgDump, "imgDump", false, "dump images to dump/img as PNG")
	flag.BoolVar(&sndDump, "sndDump", false, "dump sounds to dump/snd as WAV")
	flag.BoolVar(&dumpBEPPTags, "dumpBEPPTags", false, "log BEPP tags seen (for empirical analysis)")
	flag.BoolVar(&musicDebug, "musicDebug", false, "show bard music messages in chat")
	flag.BoolVar(&experimental, "experimental", false, "enable experimental features like CL_Images/CL_Sounds patching")
	flag.BoolVar(&showUIScale, "uiscale", false, "show UI scaling options")
	flag.BoolVar(&hdTextures, "hd", false, "enable HD texture loading from data/hd")
	genPGO := flag.Bool("pgo", false, "create default.pgo using test.clMov at 30 fps for 30s")
	flag.Parse()

    // Classic timing and parser are always enabled; flags removed.

    if *dumpTune != "" {
        // Minimal dump path: no window/audio init needed.
        notes := *dumpTune
        tempo := *dumpTempo
        inst := *dumpInst
        if inst < 0 || inst >= len(instruments) {
            inst = defaultInstrument
        }
        ns := classicNotesFromTune(notes, instruments[inst], tempo, 100)
        var end time.Duration
        for i, n := range ns {
            s := n.Start.Milliseconds()
            d := n.Duration.Milliseconds()
            println(fmt.Sprintf("%02d: key=%3d start=%6dms dur=%6dms", i, n.Key, s, d))
            if e := n.Start + n.Duration; e > end {
                end = e
            }
        }
        println(fmt.Sprintf("total end: %dms (tempo=%d inst=%d)", end.Milliseconds(), tempo, inst))
        return
    }

	if err := clipboard.Init(); err != nil {
		log.Printf("clipboard init: %v", err)
	}

	if *genPGO {
		clmov = filepath.Join("clmovFiles", "test.clMov")
		clMovFPS = 30
	}

	loadSettings()
	if gs.WindowWidth < 512 {
		gs.WindowWidth = initialWindowW
	}
	if gs.WindowHeight < 384 {
		gs.WindowHeight = initialWindowH
	}
	ebiten.SetWindowSize(gs.WindowWidth, gs.WindowHeight)

	if img, err := png.Decode(bytes.NewReader(windowIconPNG)); err == nil {
		ebiten.SetWindowIcon([]image.Image{img})
	} else {
		log.Printf("decode icon: %v", err)
	}

	var err error

	loadCharacters()
	initSoundContext()

	applySettings()
	setupLogging(doDebug)
	go versionCheckLoop()
	defer func() {
		if r := recover(); r != nil {
			logPanic(r)
		}
	}()

	clmovPath := ""
	if clmov != "" {
		clmovPath = clmov
	}

	loadStats()
	defer saveStats()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)
	if *genPGO {
		f, err := os.Create("default.pgo")
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

	initDiscordRPC(ctx)

	clImages, err = climg.Load(filepath.Join(dataDirPath, CL_ImagesFile))
	if err != nil {
		logError("failed to load CL_Images: %v", err)
		// Do not exit; allow UI to open download window.
	} else {
		clImages.Denoise = gs.DenoiseImages
		clImages.DenoiseSharpness = gs.DenoiseSharpness
		clImages.DenoiseAmount = gs.DenoiseAmount
	}

	clSounds, err = clsnd.Load(filepath.Join("data/CL_Sounds"))
	if err != nil {
		logError("failed to load CL_Sounds: %v", err)
		// Do not exit; allow UI to open download window.
	}

	if (gs.precacheSounds || gs.precacheImages) && !gs.NoCaching {
		go precacheAssets()
	}

	go func() {
		if clmovPath != "" {
			drawStateEncrypted = false
			frames, err := parseMovie(clmovPath, clientVersion)
			if err != nil {
				log.Fatalf("parse movie: %v", err)
			}

			playerName = extractMoviePlayerName(frames)
			applyEnabledPlugins()

			mp := newMoviePlayer(frames, clMovFPS, cancel)
			mp.makePlaybackWindow()

			if (gs.precacheSounds || gs.precacheImages) && !assetsPrecached {
				for !assetsPrecached {
					time.Sleep(time.Millisecond * 100)
				}
			}
			go mp.run(ctx)

			<-ctx.Done()
			return
		}

		if pcapPath != "" {
			drawStateEncrypted = false
			if (gs.precacheSounds || gs.precacheImages) && !assetsPrecached {
				for !assetsPrecached {
					time.Sleep(time.Millisecond * 100)
				}
			}
			go func() {
				if err := replayPCAP(ctx, pcapPath); err != nil {
					log.Printf("replay PCAP: %v", err)
				} else {
					log.Print("PCAP replay complete")
				}
			}()
			<-ctx.Done()
			return
		}

		if fake {
			drawStateEncrypted = false
			if (gs.precacheSounds || gs.precacheImages) && !assetsPrecached {
				for !assetsPrecached {
					time.Sleep(time.Millisecond * 100)
				}
			}
			runFakeMode(ctx)
			<-ctx.Done()
			return
		}
	}()
	runGame(ctx)
	cancel()

	<-ctx.Done()
}

func extractMoviePlayerName(frames []movieFrame) string {
	for _, m := range frames {
		if len(m.data) >= 2 && binary.BigEndian.Uint16(m.data[:2]) == 2 {
			data := append([]byte(nil), m.data[2:]...)
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
		if len(m.data) >= 2 && binary.BigEndian.Uint16(m.data[:2]) == 2 {
			data := append([]byte(nil), m.data[2:]...)
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
			name := utfFold(decodeMacRoman(data[p : p+off]))
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
		return utfFold(decodeMacRoman(data[p : p+idx]))
	}
	return ""
}
