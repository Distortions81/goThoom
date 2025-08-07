package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	errorLogger  *log.Logger
	errorLogPath string
	errorLogOnce sync.Once

	debugLogger  *log.Logger
	debugLogPath string
	debugLogOnce sync.Once
	// debugPacketDumpLen limits how many bytes of a packet payload are logged.
	// A value of 0 dumps the entire payload.
	debugPacketDumpLen = 256
)

func setupLogging(debug bool) {
	logDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("could not create log directory: %v\n", err)
	}
	ts := time.Now().Format("20060102-150405")

	errorLogPath = filepath.Join(logDir, fmt.Sprintf("error-%s.log", ts))
	errorLogOnce = sync.Once{}
	errorLogger = log.New(os.Stdout, "", log.LstdFlags)
	log.SetOutput(errorLogger.Writer())

	setDebugLogging(debug)
}

func logError(format string, v ...interface{}) {
	if errorLogger != nil {
		errorLogOnce.Do(func() {
			if f, err := os.Create(errorLogPath); err == nil {
				errorLogger.SetOutput(io.MultiWriter(os.Stdout, f))
				log.SetOutput(errorLogger.Writer())
			}
		})
		errorLogger.Printf(format, v...)
	}
	if !silent {
		addMessage(fmt.Sprintf(format, v...))
	}
}

func logDebug(format string, v ...interface{}) {
	if debugLogger != nil {
		debugLogOnce.Do(func() {
			if f, err := os.Create(debugLogPath); err == nil {
				debugLogger.SetOutput(io.MultiWriter(os.Stdout, f))
			}
		})
		debugLogger.Printf(format, v...)
	}
}

func logDebugPacket(prefix string, data []byte) {
	if debugLogger == nil {
		return
	}
	debugLogOnce.Do(func() {
		if f, err := os.Create(debugLogPath); err == nil {
			debugLogger.SetOutput(io.MultiWriter(os.Stdout, f))
		}
	})
	n := len(data)
	dump := data
	if debugPacketDumpLen > 0 && n > 0 {
		dump = data[:0]
	}
	debugLogger.Printf("%s len=%d payload=% x", prefix, n, dump)
}

func setDebugLogging(enabled bool) {
	if enabled {
		logDir := filepath.Join(baseDir, "logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("could not create log directory: %v\n", err)
		}
		ts := time.Now().Format("20060102-150405")
		debugLogPath = filepath.Join(logDir, fmt.Sprintf("debug-%s.log", ts))
		debugLogOnce = sync.Once{}
		debugLogger = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		debugLogger = nil
	}
}
