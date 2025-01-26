package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strings"
)

// Logger wraps the standard log.Logger with log level functionality
type Logger struct {
	logger       *log.Logger
	levels       []LogLevel
	apiLevels    []LogLevel
	stdout       bool
	disabled     bool
	debugEnabled bool
	disabledAPI  bool
	colors       bool
}

var stdOutLoggerExists bool

// NewLogger creates a new Logger instance with separate file and stdout loggers
func NewLogger(filepath string, levels, apiLevels []LogLevel, noColors bool) (*Logger, error) {
	var fileWriter io.Writer = io.Discard
	stdout := filepath == ""
	// Configure file logging
	if !stdout {
		file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %v", err)
		}
		fileWriter = file
	}
	var flags int
	if slices.Contains(levels, DEBUG) {
		flags |= log.Lshortfile
	}
	logger := log.New(os.Stdout, "", flags)
	if filepath != "" {
		logger = log.New(fileWriter, "", flags)
	}
	if stdout {
		stdOutLoggerExists = true
	}
	return &Logger{
		logger:       logger,
		levels:       levels,
		apiLevels:    apiLevels,
		disabled:     slices.Contains(levels, DISABLED),
		debugEnabled: slices.Contains(levels, DEBUG),
		disabledAPI:  slices.Contains(apiLevels, DISABLED),
		colors:       !noColors,
		stdout:       stdout,
	}, nil
}

// SetupLogger configures the logger with file and stdout options and their respective log levels
func SetupLogger(output, levels, apiLevels string, noColors bool) error {
	upperLevels := []LogLevel{}
	for _, level := range SplitByMultiple(levels) {
		if level == "" {
			break
		}
		upperLevel := strings.ToUpper(level)
		if upperLevel == "WARNING" || upperLevel == "WARN" {
			upperLevel = "WARN "
		}
		if upperLevel == "INFO" {
			upperLevel = "INFO "
		}
		// Convert level strings to LogLevel
		level, ok := stringToLevel[upperLevel]
		if !ok {
			loggers = []*Logger{}
			return fmt.Errorf("invalid file log level: %s", upperLevel)
		}
		upperLevels = append(upperLevels, level)
	}
	if len(upperLevels) == 0 {
		upperLevels = []LogLevel{INFO, ERROR, WARNING}
	}
	upperApiLevels := []LogLevel{}
	for _, level := range SplitByMultiple(apiLevels) {
		if level == "" {
			break
		}
		upperLevel := strings.ToUpper(level)
		if upperLevel == "WARNING" || upperLevel == "WARN" {
			upperLevel = "WARN "
		}
		if upperLevel == "INFO" {
			upperLevel = "INFO "
		}
		// Convert level strings to LogLevel
		level, ok := stringToLevel[strings.ToUpper(upperLevel)]
		if !ok {
			return fmt.Errorf("invalid api log level: %s", upperLevel)
		}
		upperApiLevels = append(upperApiLevels, level)
	}
	if len(upperApiLevels) == 0 {
		upperApiLevels = []LogLevel{INFO, ERROR, WARNING}
	}
	if slices.Contains(upperLevels, DISABLED) && slices.Contains(upperApiLevels, DISABLED) {
		// both disabled, not creating a logger
		loggers = []*Logger{}
		return nil
	}
	outputStdout := strings.ToUpper(output)
	if outputStdout == "STDOUT" {
		output = ""
	}
	if output == "" && stdOutLoggerExists {
		// stdout logger already exists... don't create another
		return fmt.Errorf("stdout logger already exists, could not set config levels=[%v] apiLevels=[%v] noColors=[%v]", levels, apiLevels, noColors)
	}
	// Create the logger
	logger, err := NewLogger(output, upperLevels, upperApiLevels, noColors)
	if err != nil {
		loggers = []*Logger{}
		return err
	}
	loggers = append(loggers, logger)
	return nil
}

func SplitByMultiple(str string) []string {
	delimiters := []rune{'|', ',', ' '}
	return strings.FieldsFunc(str, func(r rune) bool {
		for _, d := range delimiters {
			if r == d {
				return true
			}
		}
		return false
	})
}
