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
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gothoom/climg"
	"gothoom/clsnd"
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
	clipboard "golang.design/x/clipboard"

	_ "embed"
)

const (
	defaultServerHostName = "server.deltatao.com"
	fallbackServerIP      = "172.236.246.13"
)

var (
	//go:embed logo.png
	windowIconPNG []byte

	// Default movie playback FPS; classic client updates ~10Hz.
	clMovFPS int = 5

	host     string = defaultServerHostName + ":5010"
	name     string
	pass     string
	passHash string

	clmov        string
	pcapPath     string
	fake         bool
	blockSound   bool
	blockBubbles bool
	blockTTS     bool
	blockMusic   bool
	dumpMusic    bool
	imgDump      bool
	sndDump      bool
	dumpBEPPTags bool
	musicDebug   bool
	experimental bool
	showUIScale  bool
	measureLoads bool
)

func main() {
	// Ensure any active recording is finalized on exit.
	defer func() {
		if recorder != nil {
			stopRecording()
		}
	}()
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
	flag.BoolVar(&measureLoads, "measure", false, "report asset load times and metadata (sounds/images)")
	genPGO := flag.Bool("pgo", false, "create default.pgo using test.clMov at 30 fps for 30s")
	verifyPath := flag.String("verifyClmov", "", "verify a .clMov file by re-encoding and comparing")
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

	if *verifyPath != "" {
		if err := verifyClmov(*verifyPath, clVersion); err != nil {
			log.Fatalf("verifyClmov: %v", err)
		}
		log.Printf("verifyClmov: OK")
		return
	}

	if *genPGO {
		clmov = filepath.Join("clmovFiles", "test.clMov.zip")
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

	clmovPath := ""
	if clmov != "" {
		clmovPath = clmov
	}

	loadStats()
	defer saveStats()

	ctx, cancel := signal.NotifyContext(context.Background(), shutdownSignals()...)
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
			time.Sleep(300 * time.Second)
			cancel()
		}()
	}

	if !isWASM {
		initDiscordRPC(ctx)
	}

	imgStart := time.Now()
	if isWASM && len(wasmCLImagesData) > 0 {
		clImages, err = climg.LoadBytes(wasmCLImagesData)
	} else {
		clImages, err = climg.Load(filepath.Join(dataDirPath, CL_ImagesFile))
	}
	if err != nil {
		logError("failed to load CL_Images: %v", err)
		// Do not exit; allow UI to open download window.
	} else {
		clImages.SetDenoise(gs.DenoiseImages, gs.DenoiseSharpness, gs.DenoiseAmount)
		clImages.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
		if measureLoads {
			dtms := float64(time.Since(imgStart).Nanoseconds()) / 1e6
			log.Printf("measure: CL_Images archive loaded in %.2fms frame=%d", dtms, frameCounter)
		}
		// Build/restore the splash image according to settings.
		prepareClassicSplash()
	}

	sndStart := time.Now()
	if isWASM && len(wasmCLSoundsData) > 0 {
		clSounds, err = clsnd.LoadBytes(wasmCLSoundsData)
	} else {
		clSounds, err = clsnd.Load(filepath.Join(dataDirPath, CL_SoundsFile))
	}
	if err != nil {
		logError("failed to load CL_Sounds: %v", err)
		// Do not exit; allow UI to open download window.
	} else if measureLoads {
		dtms := float64(time.Since(sndStart).Nanoseconds()) / 1e6
		log.Printf("measure: CL_Sounds archive loaded in %.2fms frame=%d", dtms, frameCounter)
	}

	if gs.PrecacheSounds || gs.PrecacheImages {
		go precacheAssets()
	}

	go func() {
		if clmovPath != "" || (isWASM && len(wasmMovieZipData) > 0) {
			if isWASM {
				enterWasmPrivacyMode()
				defer exitWasmPrivacyMode()
			}
			if !waitForMovieAssets(ctx) {
				return
			}
			if loginWin != nil {
				loginWin.Close()
			}
			drawStateEncrypted = false
			var (
				frames []movieFrame
				err    error
			)
			if clmovPath != "" {
				frames, err = parseMovie(clmovPath, clVersion)
			} else {
				frames, err = parseMovieZipBytes(wasmMovieZipData, clVersion)
			}
			if err != nil {
				log.Fatalf("parse movie: %v", err)
			}

			playerName = extractMoviePlayerName(frames)
			if wasmPrivacyActive() {
				playerName = ""
			}
			updateGameWindowTitle()
			applyEnabledScripts()

			mp := newMoviePlayer(frames, clMovFPS, cancel)
			if isWASM {
				mp.repeat = true
				gs.PowerSaveAlways = false
				gs.PowerSaveBackground = false
			}
			mp.makePlaybackWindow()

			if (gs.PrecacheSounds || gs.PrecacheImages) && !assetsPrecached {
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
			if (gs.PrecacheSounds || gs.PrecacheImages) && !assetsPrecached {
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
			if (gs.PrecacheSounds || gs.PrecacheImages) && !assetsPrecached {
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

func waitForMovieAssets(ctx context.Context) bool {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return false
		}

		dlMutex.Lock()
		needImages := status.NeedImages
		needSounds := status.NeedSounds
		dlMutex.Unlock()

		imagesReady := clImages != nil
		soundsReady := clSounds != nil

		if imagesReady && soundsReady && !needImages && !needSounds {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

func extractMoviePlayerName(frames []movieFrame) string {
	for _, m := range frames {
		if len(m.data) >= 2 && binary.BigEndian.Uint16(m.data[:2]) == 2 {
			data := append([]byte(nil), m.data[2:]...)
			if n := validMoviePlayerName(playerFromDrawState(data)); n != "" {
				return n
			}
			simpleEncrypt(data)
			if n := validMoviePlayerName(playerFromDrawState(data)); n != "" {
				return n
			}
		}
	}

	for _, m := range frames {
		if len(m.data) >= 2 && binary.BigEndian.Uint16(m.data[:2]) == 2 {
			data := append([]byte(nil), m.data[2:]...)
			if n := validMoviePlayerName(firstDescriptorName(data)); n != "" {
				return n
			}
			simpleEncrypt(data)
			if n := validMoviePlayerName(firstDescriptorName(data)); n != "" {
				return n
			}
		}
	}
	return ""
}

// validMoviePlayerName rejects names obtained by accidentally interpreting an
// encrypted or non-draw frame as a descriptor. Those values must never reach
// the Clan Lord window title.
func validMoviePlayerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 48 {
		return ""
	}
	hasLetter := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r), r == ' ', r == '\'', r == '-':
		default:
			return ""
		}
	}
	if !hasLetter {
		return ""
	}
	return name
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
			name := utfFold(decodeServerText(data[p : p+off]))
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
		return utfFold(decodeServerText(data[p : p+idx]))
	}
	return ""
}
