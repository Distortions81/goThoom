package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

var (
	chatTTSMu    sync.Mutex
	ttsPlayers   = make(map[*audio.Player]struct{})
	ttsPlayersMu sync.Mutex

	chatTTSQueue  chan string
	chatTTSCtx    context.Context
	chatTTSCancel func()

	pendingTTSMu    sync.Mutex
	pendingTTS      int
	playChatTTSFunc func(context.Context, string)

	piperPath   string
	piperModel  string
	piperConfig string

	lastTTSSpeaker string
	lastTTSTime    time.Time
)

func init() {
	playChatTTSFunc = playChatTTS
	resetChatTTSWorker()
}

func resetChatTTSWorker() {
	if chatTTSCancel != nil {
		chatTTSCancel()
	}
	chatTTSCtx, chatTTSCancel = context.WithCancel(context.Background())
	chatTTSQueue = make(chan string, 10)
	go chatTTSWorker(chatTTSCtx, chatTTSQueue)
}

func stopAllTTS() {
	ttsPlayersMu.Lock()
	for p := range ttsPlayers {
		_ = p.Close()
		delete(ttsPlayers, p)
	}
	ttsPlayersMu.Unlock()
	resetChatTTSWorker()
	pendingTTSMu.Lock()
	pendingTTS = 0
	pendingTTSMu.Unlock()
}

func disableTTS() {
	gs.ChatTTS = false
	settingsDirty = true
	stopAllTTS()
	updateSoundVolume()
	if ttsMixCB != nil {
		ttsMixCB.Checked = false
		ttsMixCB.Dirty = true
	}
	if ttsMixSlider != nil {
		ttsMixSlider.Disabled = true
		ttsMixSlider.Dirty = true
	}
}

func chatTTSWorker(ctx context.Context, queue <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-queue:
			msgs := []string{msg}
			timer := time.NewTimer(200 * time.Millisecond)
		collect:
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					// On cancellation, do not adjust pendingTTS here; stopAllTTS
					// is responsible for resetting the counter to 0.
					return
				case m := <-queue:
					msgs = append(msgs, m)
				case <-timer.C:
					break collect
				}
			}
			timer.Stop()
			select {
			case <-ctx.Done():
				// Do not adjust pendingTTS on cancellation; it will be reset elsewhere.
				return
			default:
			}
			playChatTTSFunc(ctx, strings.Join(msgs, ". "))
			pendingTTSMu.Lock()
			pendingTTS -= len(msgs)
			pendingTTSMu.Unlock()
		}
	}
}

func ensurePiper() bool {
	if piperPath != "" && piperModel != "" {
		return true
	}
	var err error
	piperPath, piperModel, piperConfig, err = preparePiper(dataDirPath)
	if err != nil {
		logError("chat tts init: %v", err)
		return false
	}
	return true
}

func playChatTTS(ctx context.Context, text string) {
	if audioContext == nil || blockTTS || gs.Mute || focusMuted || !gs.ChatTTS {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}
	if !ensurePiper() {
		logError("chat tts: piper not initialized")
		disableTTS()
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	text = substituteTTS(text)
	wavData, err := synthesizeWithPiper(text)
	if err != nil {
		logError("chat tts synthesize: %v", err)
		disableTTS()
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}
	stream, err := wav.DecodeWithSampleRate(audioContext.SampleRate(), bytes.NewReader(wavData))
	if err != nil {
		logError("chat tts decode: %v", err)
		disableTTS()
		return
	}

	chatTTSMu.Lock()
	defer chatTTSMu.Unlock()

	select {
	case <-ctx.Done():
		return
	default:
	}
	p, err := audioContext.NewPlayer(stream)
	if err != nil {
		logError("chat tts player: %v", err)
		disableTTS()
		return
	}

	ttsPlayersMu.Lock()
	ttsPlayers[p] = struct{}{}
	ttsPlayersMu.Unlock()

	vol := gs.MasterVolume * gs.ChatTTSVolume
	if gs.Mute || focusMuted {
		vol = 0
	}
	p.SetVolume(vol)
	p.Play()
	for p.IsPlaying() {
		select {
		case <-ctx.Done():
			_ = p.Close()
			ttsPlayersMu.Lock()
			delete(ttsPlayers, p)
			ttsPlayersMu.Unlock()
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
	_ = p.Close()

	ttsPlayersMu.Lock()
	delete(ttsPlayers, p)
	ttsPlayersMu.Unlock()
}

func speakChatMessage(msg string) {
	if audioContext == nil || blockTTS || gs.Mute || focusMuted || !gs.ChatTTS {
		if audioContext == nil {
			logError("chat tts: audio context is nil")
		}
		if blockTTS {
			logDebug("chat tts: tts blocked")
		}
		if gs.Mute || focusMuted {
			logDebug("chat tts: client muted")
		}
		if !gs.ChatTTS {
			logDebug("chat tts: disabled in settings")
		}
		return
	}

	speaker := chatSpeaker(msg)
	ttsMsg := msg
	if speaker != "" {
		now := time.Now()
		if speaker == lastTTSSpeaker && now.Sub(lastTTSTime) <= 10*time.Second {
			content := msg
			if i := strings.Index(msg, ","); i >= 0 && i+1 < len(msg) {
				content = strings.TrimSpace(msg[i+1:])
			}
			ttsMsg = "and then said " + content
		}
		lastTTSSpeaker = speaker
		lastTTSTime = now
	}

	pendingTTSMu.Lock()
	if pendingTTS >= 10 {
		pendingTTSMu.Unlock()
		logError("chat tts: too many pending messages, dropping message")
		return
	}
	pendingTTS++
	pendingTTSMu.Unlock()
	select {
	case chatTTSQueue <- ttsMsg:
	default:
		pendingTTSMu.Lock()
		pendingTTS--
		pendingTTSMu.Unlock()
		logError("chat tts: queue full, dropping message")
	}
}

func preparePiper(dataDir string) (string, string, string, error) {
	if isWASM {
		return "", "", "", fmt.Errorf("chat tts unavailable in wasm")
	}
	piperDir := filepath.Join(dataDir, "piper")
	binDir := filepath.Join(piperDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", "", "", err
	}

	var archiveName, binName string
	switch runtime.GOOS {
	case "linux":
		binName = "piper"
		switch runtime.GOARCH {
		case "amd64":
			archiveName = "piper_linux_x86_64.tar.gz"
		case "arm64":
			archiveName = "piper_linux_aarch64.tar.gz"
		case "arm":
			archiveName = "piper_linux_armv7l.tar.gz"
		default:
			return "", "", "", fmt.Errorf("unsupported arch %s", runtime.GOARCH)
		}
	case "darwin":
		binName = "piper"
		switch runtime.GOARCH {
		case "amd64":
			archiveName = "piper_macos_x64.tar.gz"
		case "arm64":
			archiveName = "piper_macos_aarch64.tar.gz"
		default:
			return "", "", "", fmt.Errorf("unsupported arch %s", runtime.GOARCH)
		}
	case "windows":
		binName = "piper.exe"
		archiveName = "piper_windows_amd64.zip"
	default:
		return "", "", "", fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}

	findBin := func() (string, error) {
		candidates := []string{
			filepath.Join(binDir, binName),
			filepath.Join(binDir, "piper", binName),
		}
		if runtime.GOOS == "windows" {
			candidates = append(candidates, filepath.Join(binDir, "piper", "piper"))
		}
		for _, p := range candidates {
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				return p, nil
			}
		}
		return "", os.ErrNotExist
	}

	binPath, err := findBin()
	if err != nil {
		archivePath := filepath.Join(piperDir, archiveName)
		if _, err := os.Stat(archivePath); err != nil {
			if err := downloadFile(extraDataBase+archiveName, archivePath); err != nil {
				return "", "", "", err
			}
		}
		if err := extractArchive(archivePath, binDir); err != nil {
			return "", "", "", err
		}
		if binPath, err = findBin(); err != nil {
			return "", "", "", fmt.Errorf("piper binary missing: %w", err)
		}
	}
	_ = os.Chmod(binPath, 0o755)

	voicesDir := filepath.Join(piperDir, "voices")
	// Automatically extract any voice archives placed in the voices directory.
	if archives, _ := filepath.Glob(filepath.Join(voicesDir, "*.tar.gz")); len(archives) > 0 {
		if err := os.MkdirAll(voicesDir, 0o755); err != nil {
			return "", "", "", err
		}
		for _, arch := range archives {
			if err := extractArchive(arch, voicesDir); err != nil {
				return "", "", "", err
			}
			_ = os.Remove(arch)
		}
	}
	voice := gs.ChatTTSVoice
	if voice == "" {
		voice = "en_US-hfc_female-medium"
	}

	model := filepath.Join(voicesDir, voice, voice+".onnx")
	cfg := filepath.Join(voicesDir, voice, voice+".onnx.json")
	if _, err := os.Stat(model); err != nil {
		// Try voice files directly in voicesDir
		model = filepath.Join(voicesDir, voice+".onnx")
		cfg = filepath.Join(voicesDir, voice+".onnx.json")
		if _, err := os.Stat(model); err != nil {
			// Search subdirectories for the voice files
			matches, _ := filepath.Glob(filepath.Join(voicesDir, "*", voice+".onnx"))
			found := false
			for _, m := range matches {
				c := filepath.Join(filepath.Dir(m), voice+".onnx.json")
				if _, err2 := os.Stat(c); err2 == nil {
					model = m
					cfg = c
					found = true
					break
				}
			}
			if !found {
				return "", "", "", fmt.Errorf("missing piper voice model: %w", err)
			}
		}
	}
	if _, err := os.Stat(cfg); err != nil {
		return "", "", "", fmt.Errorf("missing piper voice config: %w", err)
	}
	return binPath, model, cfg, nil
}

func listPiperVoices() ([]string, error) {
	if isWASM {
		return nil, fmt.Errorf("chat tts unavailable in wasm")
	}
	voicesDir := filepath.Join(dataDirPath, "piper", "voices")
	entries, err := os.ReadDir(voicesDir)
	if err != nil {
		return nil, err
	}
	voiceSet := map[string]struct{}{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			subdir := filepath.Join(voicesDir, name)
			files, err := os.ReadDir(subdir)
			if err != nil {
				continue
			}
			for _, f := range files {
				fname := f.Name()
				if f.IsDir() || !strings.HasSuffix(fname, ".onnx") {
					continue
				}
				base := strings.TrimSuffix(fname, ".onnx")
				cfg := filepath.Join(subdir, base+".onnx.json")
				if _, err := os.Stat(cfg); err != nil {
					continue
				}
				voiceSet[base] = struct{}{}
			}
		} else if strings.HasSuffix(name, ".onnx") {
			base := strings.TrimSuffix(name, ".onnx")
			cfg := filepath.Join(voicesDir, base+".onnx.json")
			if _, err := os.Stat(cfg); err != nil {
				continue
			}
			voiceSet[base] = struct{}{}
		}
	}
	voices := make([]string, 0, len(voiceSet))
	for v := range voiceSet {
		voices = append(voices, v)
	}
	sort.Strings(voices)
	return voices, nil
}

func extractArchive(src, dst string) error {
	if isWASM {
		return fmt.Errorf("archive extraction unavailable in wasm")
	}
	clean := func(base, name string) (string, error) {
		target := filepath.Join(base, name)
		target = filepath.Clean(target)
		rel, err := filepath.Rel(base, target)
		if err != nil {
			return "", err
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("invalid path: %s", name)
		}
		return target, nil
	}
	if strings.HasSuffix(src, ".tar.gz") {
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			target, err := clean(dst, hdr.Name)
			if err != nil {
				return err
			}
			switch hdr.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
					return err
				}
				continue
			case tar.TypeSymlink:
				if _, err := clean(dst, filepath.Join(filepath.Dir(hdr.Name), hdr.Linkname)); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				if err := os.Symlink(hdr.Linkname, target); err != nil {
					return err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
			_ = os.Chmod(target, os.FileMode(hdr.Mode))
		}
		return nil
	}
	if strings.HasSuffix(src, ".zip") {
		zr, err := zip.OpenReader(src)
		if err != nil {
			return err
		}
		defer zr.Close()
		for _, f := range zr.File {
			target, err := clean(dst, f.Name)
			if err != nil {
				return err
			}
			if f.FileInfo().IsDir() {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return err
			}
			if f.Mode()&os.ModeSymlink != 0 {
				linkBytes, err := io.ReadAll(rc)
				rc.Close()
				if err != nil {
					return err
				}
				link := string(linkBytes)
				if _, err := clean(dst, filepath.Join(filepath.Dir(f.Name), link)); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				if err := os.Symlink(link, target); err != nil {
					return err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				rc.Close()
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				rc.Close()
				return err
			}
			if _, err := io.Copy(out, rc); err != nil {
				rc.Close()
				out.Close()
				return err
			}
			rc.Close()
			out.Close()
			_ = os.Chmod(target, f.Mode())
		}
		return nil
	}
	return fmt.Errorf("unknown archive format: %s", src)
}

// synthesizeWithPiper invokes the piper binary to generate speech from text.
//
// On Windows the piper binary cannot stream audio to stdout, so the output is
// written to a temporary file which is read back after the process completes.
func synthesizeWithPiper(text string) ([]byte, error) {
	if piperPath == "" || piperModel == "" {
		return nil, fmt.Errorf("piper not initialized")
	}

	dir := filepath.Dir(piperPath)
	args := []string{
		"--model", piperModel,
		"--config", piperConfig,
		"--espeak_data", filepath.Join(dir, "espeak-ng-data"),
		"--length_scale", fmt.Sprintf("%f", 1/gs.ChatTTSSpeed),
	}
	var stderr bytes.Buffer

	if runtime.GOOS == "windows" {
		tmp, err := os.CreateTemp("", "piper-*.wav")
		if err != nil {
			return nil, fmt.Errorf("piper temp file: %v", err)
		}
		tmpName := tmp.Name()
		_ = tmp.Close()
		defer os.Remove(tmpName)

		args = append(args, "--output_file", tmpName)
		cmd := exec.Command(piperPath, args...)
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(text)
		cmd.Stderr = &stderr
		if attr := piperSysProcAttr(); attr != nil {
			cmd.SysProcAttr = attr
		}
		if err := cmd.Run(); err != nil {
			if os.IsPermission(err) {
				if info, statErr := os.Stat(piperPath); statErr == nil {
					return nil, fmt.Errorf("piper run: %v (file mode %v): %s", err, info.Mode(), stderr.String())
				}
			}
			return nil, fmt.Errorf("piper run: %v: %s", err, stderr.String())
		}
		data, err := os.ReadFile(tmpName)
		if err != nil {
			return nil, fmt.Errorf("read piper output: %v", err)
		}
		return data, nil
	}

	var out bytes.Buffer
	args = append(args, "--output_file", "-")
	cmd := exec.Command(piperPath, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(text)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if attr := piperSysProcAttr(); attr != nil {
		cmd.SysProcAttr = attr
	}
	if err := cmd.Run(); err != nil {
		if os.IsPermission(err) {
			if info, statErr := os.Stat(piperPath); statErr == nil {
				return nil, fmt.Errorf("piper run: %v (file mode %v): %s", err, info.Mode(), stderr.String())
			}
		}
		return nil, fmt.Errorf("piper run: %v: %s", err, stderr.String())
	}
	return out.Bytes(), nil
}
