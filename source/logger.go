package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
)

const (
	diagnosticsDirectoryName = "Diagnostics"
	diagnosticsLogName       = "goThoom.log"
	diagnosticsLogMaxBytes   = int64(4 << 20)
	diagnosticsLogBackups    = 5
)

var (
	errorLogger *log.Logger
	debugLogger *log.Logger

	diagnosticsMu     sync.Mutex
	diagnosticsWriter *rotatingLogWriter
	diagnosticsOutput io.Writer = os.Stdout
	shutdownReasonMu  sync.Mutex
	shutdownReason    string

	// debugPacketDumpLen limits how many bytes of a packet payload are logged.
	// A value of 0 dumps the entire payload.
	debugPacketDumpLen = 256
)

// recordShutdownReason keeps the first reason because later cleanup commonly
// cancels the same context again. The first request is the useful cause.
func recordShutdownReason(reason string) {
	if reason == "" {
		reason = "unspecified"
	}
	shutdownReasonMu.Lock()
	if shutdownReason != "" {
		shutdownReasonMu.Unlock()
		return
	}
	shutdownReason = reason
	shutdownReasonMu.Unlock()
	log.Printf("shutdown reason: %s", reason)
}

// rotatingLogWriter keeps a bounded current log plus numbered backups. It is
// safe for the standard logger and the debug logger to share.
type rotatingLogWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	backups  int
	file     *os.File
	size     int64
}

func newRotatingLogWriter(path string, maxBytes int64, backups int) (*rotatingLogWriter, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("diagnostics log size must be positive")
	}
	if backups < 1 {
		return nil, fmt.Errorf("diagnostics backup count must be positive")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := rotateLogFiles(path, backups); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	return &rotatingLogWriter{
		path:     path,
		maxBytes: maxBytes,
		backups:  backups,
		file:     f,
	}, nil
}

func (w *rotatingLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return 0, os.ErrClosed
	}
	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotateLocked(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingLogWriter) rotateLocked() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	w.file = nil
	if err := rotateLogFiles(w.path, w.backups); err != nil {
		// Keep logging to the current file if an archive cannot be renamed.
		f, openErr := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if openErr == nil {
			w.file = f
			if info, statErr := f.Stat(); statErr == nil {
				w.size = info.Size()
			}
		}
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.size = 0
	return nil
}

func (w *rotatingLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// rotateLogFiles moves path to path.1 and shifts older backups upward. If
// there is no current file, existing backups are left untouched.
func rotateLogFiles(path string, backups int) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	oldest := fmt.Sprintf("%s.%d", path, backups)
	if err := os.Remove(oldest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for i := backups - 1; i >= 1; i-- {
		source := fmt.Sprintf("%s.%d", path, i)
		destination := fmt.Sprintf("%s.%d", path, i+1)
		if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(source, destination); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	first := path + ".1"
	if err := os.Remove(first); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(path, first)
}

func diagnosticsLogDir() string {
	return filepath.Join(dataDirPath, diagnosticsDirectoryName)
}

func diagnosticsLogPath() string {
	return filepath.Join(diagnosticsLogDir(), diagnosticsLogName)
}

func setupLogging(debugEnabled bool) {
	diagnosticsMu.Lock()
	defer diagnosticsMu.Unlock()

	flags := log.LstdFlags | log.Lmicroseconds
	diagnosticsOutput = os.Stdout
	if !isWASM {
		writer, err := newRotatingLogWriter(diagnosticsLogPath(), diagnosticsLogMaxBytes, diagnosticsLogBackups)
		if err != nil {
			log.Printf("could not start diagnostics log: %v", err)
		} else {
			diagnosticsWriter = writer
			diagnosticsOutput = io.MultiWriter(os.Stdout, writer)
		}
	}

	errorLogger = log.New(diagnosticsOutput, "", flags)
	log.SetFlags(flags)
	log.SetOutput(diagnosticsOutput)
	setDebugLoggingLocked(debugEnabled, flags)

	log.Printf("diagnostics started: app=%d cl=%d go=%s platform=%s/%s pid=%d",
		appVersion, clVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH, os.Getpid())
	if diagnosticsWriter != nil {
		log.Printf("diagnostics log: %s", diagnosticsLogPath())
	}
}

func closeDiagnosticsLog() {
	recordShutdownReason("normal application return")
	diagnosticsMu.Lock()
	defer diagnosticsMu.Unlock()

	if diagnosticsWriter == nil {
		return
	}
	log.Printf("diagnostics log closed")
	if err := diagnosticsWriter.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close diagnostics log: %v\n", err)
	}
	diagnosticsWriter = nil
	diagnosticsOutput = os.Stdout
	log.SetOutput(os.Stdout)
	if errorLogger != nil {
		errorLogger.SetOutput(os.Stdout)
	}
	if debugLogger != nil {
		debugLogger.SetOutput(os.Stdout)
	}
}

// logMainPanic copies a main-goroutine panic and stack trace into the
// diagnostics log, then preserves the normal crash behavior.
func logMainPanic() {
	if recovered := recover(); recovered != nil {
		recordShutdownReason(fmt.Sprintf("panic: %v", recovered))
		if errorLogger != nil {
			errorLogger.Printf("panic: %v\n%s", recovered, debug.Stack())
		} else {
			log.Printf("panic: %v\n%s", recovered, debug.Stack())
		}
		panic(recovered)
	}
}

func logError(format string, v ...interface{}) {
	if errorLogger != nil {
		errorLogger.Printf(format, v...)
	}
	if !silent {
		consoleMessage(fmt.Sprintf(format, v...))
	}
}

func logDebug(format string, v ...interface{}) {
	if debugLogger != nil {
		debugLogger.Printf(format, v...)
	}
}

func logWarn(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if errorLogger != nil {
		errorLogger.Printf("warning: %s", msg)
	}
	if !silent {
		consoleMessage("warning: " + msg)
	}
}

func logDebugPacket(prefix string, data []byte) {
	if debugLogger == nil {
		return
	}
	n := len(data)
	dump := data
	if debugPacketDumpLen > 0 && n > debugPacketDumpLen {
		dump = data[:debugPacketDumpLen]
	}
	debugLogger.Printf("%s len=%d payload=% x", prefix, n, dump)
}

func setDebugLogging(enabled bool) {
	diagnosticsMu.Lock()
	defer diagnosticsMu.Unlock()
	setDebugLoggingLocked(enabled, log.LstdFlags|log.Lmicroseconds)
}

func setDebugLoggingLocked(enabled bool, flags int) {
	if enabled {
		debugLogger = log.New(diagnosticsOutput, "debug: ", flags)
	} else {
		debugLogger = nil
	}
}
