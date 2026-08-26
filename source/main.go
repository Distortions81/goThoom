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
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gothoom/climg"
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

	clmov             string
	pcapPath          string
	fake              bool
	blockSound        bool
	blockBubbles      bool
	blockTTS          bool
	blockMusic        bool
	dumpMusic         bool
	imgDump           bool
	imgDumpScale      int
	imgDumpScaleType  string
	sndDump           bool
	dumpBEPPTags      bool
	musicDebug        bool
	experimental      bool
	brandSpriteOutput string
)

func main() {
	defer shutdownScripts()
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
	flag.BoolVar(&replacementEffectsPreview, "effectsPreview", false, "open the replacement-effects shader preview gallery")
	flag.BoolVar(&eui.CacheCheck, "cacheCheck", false, "display window and item render counts")
	flag.BoolVar(&dumpMusic, "dumpMusic", false, "write played music as a .wav file")
	flag.BoolVar(&imgDump, "imgDump", false, "export all images to dump/img as PNG and exit")
	flag.IntVar(&imgDumpScale, "imgDumpScale", 1, "scale exported images by 1, 2, 3, or 4")
	flag.StringVar(&imgDumpScaleType, "imgDumpScaleType", "nearest", "image export upscale type: nearest, crisp, balanced, smooth, or ultra-smooth")
	flag.BoolVar(&sndDump, "sndDump", false, "export all sounds to dump/snd as WAV and exit")
	flag.BoolVar(&dumpBEPPTags, "dumpBEPPTags", false, "log BEPP tags seen (for empirical analysis)")
	flag.BoolVar(&musicDebug, "musicDebug", false, "show bard music messages in chat")
	flag.BoolVar(&experimental, "experimental", false, "enable experimental features like CL_Images/CL_Sounds patching")
	// Kept for existing launch scripts; UI Scale is now always in Settings.
	_ = flag.Bool("uiscale", false, "deprecated: UI scaling options are always shown in Settings")
	flag.StringVar(&brandSpriteOutput, "exportBrandSprite", "", "render the goThoom brand character to a transparent PNG and exit")
	genPGO := flag.Bool("pgo", false, "create default.pgo from -clmov (or test.clMov) at 30 fps")
	pgoWarmup := flag.Duration("pgoWarmup", 0, "unprofiled warmup duration used with -pgo")
	pgoWarmupMovie := flag.Bool("pgoWarmupMovie", false, "play one complete movie pass before profiling with -pgo")
	pgoDuration := flag.Duration("pgoDuration", 5*time.Minute, "CPU profiling duration used with -pgo")
	pgoOutput := flag.String("pgoOutput", "default.pgo", "CPU profile output path used with -pgo")
	pgoHeapOutput := flag.String("pgoHeapOutput", "", "heap profile output written after -pgo completes")
	verifyPath := flag.String("verifyClmov", "", "verify a .clMov file by re-encoding and comparing")
	flag.Parse()
	if imgDumpScale < 1 || imgDumpScale > 4 {
		log.Fatalf("imgDumpScale must be between 1 and 4, got %d", imgDumpScale)
	}
	if _, ok := imageDumpUpscaleMode(imgDumpScaleType); !ok {
		log.Fatalf("imgDumpScaleType must be nearest, crisp, balanced, smooth, or ultra-smooth, got %q", imgDumpScaleType)
	}

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
		if clmov == "" {
			clmov = filepath.Join("clmovFiles", "test.clMov.zip")
		}
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
	if !isWASM {
		initDiscordRPC(ctx)
	}

	if brandSpriteOutput != "" {
		var err error
		if isWASM && len(wasmCLImagesData) > 0 {
			clImages, err = climg.LoadBytes(wasmCLImagesData)
		} else {
			clImages, err = climg.Load(filepath.Join(dataDirPath, CL_ImagesFile))
		}
		if err != nil {
			log.Fatalf("export brand sprite: load CL_Images: %v", err)
		}
		clImages.SetDenoise(gs.DenoiseImages, gs.DenoiseSharpness, gs.DenoiseAmount)
		clImages.SetGammaCorrection(gs.SpriteGammaCorrection, gs.SpriteGamma, gs.MonitorGamma)
		if err := ReloadSpriteUpscaleShader(); err != nil {
			log.Fatalf("export brand sprite: compile upscale shader: %v", err)
		}
		if clImages == nil {
			log.Fatal("export brand sprite: CL_Images is not available")
		}
		if err := exportBrandSprite(brandSpriteOutput); err != nil {
			log.Fatalf("export brand sprite: %v", err)
		}
		log.Printf("exported brand sprite to %s", brandSpriteOutput)
		return
	}

	go func() {
		if assetDumpMode() {
			select {
			case <-assetDumpComplete:
				cancel()
			case <-ctx.Done():
			}
			return
		}

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
			scriptSessionLogin(playerName)
			defer scriptSessionLogout(playerName)

			mp := newMoviePlayer(frames, clMovFPS, cancel)
			if *genPGO {
				mp.repeat = true
				if *pgoWarmupMovie {
					movieDuration := time.Duration(len(frames)) * time.Second / time.Duration(mp.fps)
					log.Printf("PGO warmup: playing one complete movie pass (%d frames, %s)", len(frames), movieDuration.Round(time.Second))
				}
			}
			if isWASM {
				mp.repeat = true
				gs.PowerSaveAlways = false
				gs.PowerSaveBackground = false
			}
			mp.makePlaybackWindow()

			if gs.PrecacheSounds && !soundsPrecached {
				for !soundsPrecached {
					time.Sleep(time.Millisecond * 100)
				}
			}
			if *genPGO {
				go func() {
					if *pgoWarmupMovie {
						select {
						case <-mp.looped:
						case <-ctx.Done():
							return
						}
					} else if *pgoWarmup > 0 {
						time.Sleep(*pgoWarmup)
					}
					f, err := os.Create(*pgoOutput)
					if err != nil {
						log.Fatalf("create CPU profile: %v", err)
					}
					if err := pprof.StartCPUProfile(f); err != nil {
						f.Close()
						log.Fatalf("start CPU profile: %v", err)
					}
					time.Sleep(*pgoDuration)
					pprof.StopCPUProfile()
					if err := f.Close(); err != nil {
						log.Printf("close CPU profile: %v", err)
					}
					if *pgoHeapOutput != "" {
						runtime.GC()
						heapFile, err := os.Create(*pgoHeapOutput)
						if err != nil {
							log.Printf("create heap profile: %v", err)
						} else {
							if err := pprof.WriteHeapProfile(heapFile); err != nil {
								log.Printf("write heap profile: %v", err)
							}
							if err := heapFile.Close(); err != nil {
								log.Printf("close heap profile: %v", err)
							}
						}
					}
					cancel()
				}()
			}
			go mp.run(ctx)

			<-ctx.Done()
			return
		}

		if pcapPath != "" {
			drawStateEncrypted = false
			if gs.PrecacheSounds && !soundsPrecached {
				for !soundsPrecached {
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
			if gs.PrecacheSounds && !soundsPrecached {
				for !soundsPrecached {
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

func shutdownScripts() {
	stopAllscripts()
	savescriptStores()
}

func exitApplication(code int) {
	shutdownScripts()
	os.Exit(code)
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
