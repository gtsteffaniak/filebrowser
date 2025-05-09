package logger

import (
	"fmt"
	"log"
	"slices"
	"time"
)

type LogLevel int

const (
	DISABLED LogLevel = 0
	ERROR    LogLevel = 1
	FATAL    LogLevel = 1
	WARNING  LogLevel = 2
	INFO     LogLevel = 3
	DEBUG    LogLevel = 4
	API      LogLevel = 10
	// COLORS
	RED    = "\033[31m"
	GREEN  = "\033[32m"
	YELLOW = "\033[33m"
	GRAY   = "\033[37m"
)

var (
	loggers []*Logger
)

type levelConsts struct {
	INFO     string
	FATAL    string
	ERROR    string
	WARNING  string
	DEBUG    string
	API      string
	DISABLED string
}

var levels = levelConsts{
	INFO:     "INFO ", // with consistent space padding
	FATAL:    "FATAL",
	ERROR:    "ERROR",
	WARNING:  "WARN ", // with consistent space padding
	DEBUG:    "DEBUG",
	DISABLED: "DISABLED",
	API:      "API",
}

// stringToLevel maps string representation to LogLevel
var stringToLevel = map[string]LogLevel{
	"DEBUG":    DEBUG,
	"INFO ":    INFO, // with consistent space padding
	"ERROR":    ERROR,
	"DISABLED": DISABLED,
	"WARN ":    WARNING, // with consistent space padding
	"FATAL":    FATAL,
	"API":      API,
}

// Log prints a log message if its level is greater than or equal to the logger's levels
func Log(level string, msg string, prefix, api bool, color string) {
	LEVEL := stringToLevel[level]
	for _, logger := range loggers {
		if api {
			if logger.disabledAPI || !slices.Contains(logger.apiLevels, LEVEL) {
				continue
			}
		} else {
			if logger.disabled || !slices.Contains(logger.levels, LEVEL) {
				continue
			}
		}
		if logger.stdout && level == "FATAL" {
			continue
		}
		writeOut := msg
		formattedTime := time.Now().Format("2006/01/02 15:04:05")
		if logger.colors && color != "" {
			formattedTime = formattedTime + color
		}
		if prefix || logger.debugEnabled {
			logger.logger.SetPrefix(fmt.Sprintf("%s [%s] ", formattedTime, level))
		} else {
			logger.logger.SetPrefix(formattedTime + " ")
		}
		if logger.colors && color != "" {
			writeOut = writeOut + "\033[0m"
		}
		err := logger.logger.Output(3, writeOut) // 3 skips this function for correct file:line
		if err != nil {
			log.Printf("failed to log message '%v' with error `%v`", msg, err)
		}
	}
}

func Api(msg string, statusCode int) {
	// redirects are not warnings anymore
	// content not modified is not a warning anymore
	if statusCode > 304 && statusCode < 500 {
		Log(levels.WARNING, msg, false, true, YELLOW)
	} else if statusCode >= 500 {
		Log(levels.ERROR, msg, false, true, RED)
	} else {
		Log(levels.INFO, msg, false, true, GREEN)
	}
}

// Helper methods for specific log levels
func Debug(msg string) {
	if len(loggers) > 0 {
		Log(levels.DEBUG, msg, true, false, GRAY)
	} else {
		log.Println("[DEBUG] ", msg)
	}
}

func Info(msg string) {
	if len(loggers) > 0 {
		Log(levels.INFO, msg, false, false, "")
	} else {
		log.Println(msg)
	}
}

func Warning(msg string) {
	if len(loggers) > 0 {
		Log(levels.WARNING, msg, true, false, YELLOW)
	} else {
		log.Println("[WARN ] ", msg)
	}
}

func Error(msg string) {
	if len(loggers) > 0 {
		Log(levels.ERROR, msg, true, false, RED)
	} else {
		log.Println("[ERROR]", msg)
	}
}

func Fatal(msg string) {
	if len(loggers) > 0 {
		Log(levels.FATAL, msg, true, false, RED)
	}
	log.Fatal("[FATAL] ", msg)
}
