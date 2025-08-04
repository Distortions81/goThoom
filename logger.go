package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	errorLogger *log.Logger
	debugLogger *log.Logger
)

func setupLogging(debug bool) {
	logDir := filepath.Join(baseDir, "logs", "errors")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("could not create log directory: %v\n", err)
	}
	ts := time.Now().Format("20060102-150405")

	errPath := filepath.Join(logDir, fmt.Sprintf("error-%s.log", ts))
	errFile, err := os.Create(errPath)
	var errWriter io.Writer = os.Stdout
	if err == nil {
		errWriter = io.MultiWriter(os.Stdout, errFile)
	}
	errorLogger = log.New(errWriter, "", log.LstdFlags)
	log.SetOutput(errWriter)

	setDebugLogging(debug)
}

func logError(format string, v ...interface{}) {
	if errorLogger != nil {
		errorLogger.Printf(format, v...)
	}
	if !silent {
		addMessage(fmt.Sprintf(format, v...))
	}
}

func logDebug(format string, v ...interface{}) {
	if debugLogger != nil {
		debugLogger.Printf(format, v...)
	}
}

func setDebugLogging(enabled bool) {
	if enabled {
		logDir := filepath.Join(baseDir, "logs", "errors")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("could not create log directory: %v\n", err)
		}
		ts := time.Now().Format("20060102-150405")
		dbgPath := filepath.Join(logDir, fmt.Sprintf("debug-%s.log", ts))
		dbgFile, err := os.Create(dbgPath)
		var dbgWriter io.Writer
		if err == nil {
			dbgWriter = io.MultiWriter(os.Stdout, dbgFile)
		} else {
			dbgWriter = os.Stdout
		}
		debugLogger = log.New(dbgWriter, "", log.LstdFlags)
	} else {
		debugLogger = nil
	}
}
