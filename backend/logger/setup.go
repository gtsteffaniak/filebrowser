package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// LogLevel defines the severity levels
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
	DISABLED
)

var (
	logger *Logger
)

// levelToString maps LogLevel to string representation
var levelToString = map[LogLevel]string{
	DEBUG:    "DEBUG",
	INFO:     "INFO",
	ERROR:    "ERROR",
	DISABLED: "DISABLED",
}

// stringToLevel maps string representation to LogLevel
var stringToLevel = map[string]LogLevel{
	"DEBUG":    DEBUG,
	"INFO":     INFO,
	"ERROR":    ERROR,
	"DISABLED": DISABLED,
}

// Logger wraps the standard log.Logger with log level functionality
type Logger struct {
	fileLogger   *log.Logger
	stdoutLogger *log.Logger
	fileLevel    LogLevel
	stdoutLevel  LogLevel
}

// NewLogger creates a new Logger instance with separate file and stdout loggers
func NewLogger(fileWriter, stdoutWriter io.Writer, fileLevel, stdoutLevel LogLevel) *Logger {
	// Determine the log flags for file and stdout based on the debug level
	fileFlags := log.Ldate | log.Ltime
	if fileLevel == DEBUG {
		fileFlags |= log.Lshortfile
	}

	stdoutFlags := log.Ldate | log.Ltime
	if stdoutLevel == DEBUG {
		stdoutFlags |= log.Lshortfile
	}

	return &Logger{
		fileLogger:   log.New(fileWriter, "", fileFlags),
		stdoutLogger: log.New(stdoutWriter, "", stdoutFlags),
		fileLevel:    fileLevel,
		stdoutLevel:  stdoutLevel,
	}
}

// Log prints a log message if its level is greater than or equal to the logger's levels
func (l *Logger) Log(level LogLevel, msg string) {
	withPrefix := fmt.Sprintf("[%s] ", levelToString[level])
	if l.fileLevel != DISABLED && level >= l.fileLevel {
		if l.fileLevel == INFO {
			l.fileLogger.Output(3, msg) // 3 skips this function for correct file:line
		} else {
			l.fileLogger.Output(3, withPrefix+msg) // 3 skips this function for correct file:line
		}
	}
	if l.stdoutLevel != DISABLED && level >= l.stdoutLevel {
		if l.stdoutLevel == INFO {
			l.stdoutLogger.Output(3, msg) // 3 skips this function for correct file:line
		} else {
			l.stdoutLogger.Output(3, withPrefix+msg) // 3 skips this function for correct file:line
		}
	}
}

// Helper methods for specific log levels
func Debug(msg string) {
	if logger != nil {
		logger.Log(DEBUG, msg)
	} else {
		log.Println("DEBUG : " + msg)
	}
}

func Info(msg string) {
	if logger != nil {
		logger.Log(INFO, msg)
	} else {
		log.Println("INFO : " + msg)
	}
}

func Error(msg string) {
	if logger != nil {
		logger.Log(ERROR, msg)
	} else {
		log.Println("ERROR : " + msg)
	}
}

// SetupLogger configures the logger with file and stdout options and their respective log levels
func SetupLogger(logFile, fileLevel, stdoutLevel string) error {
	fileLevel = strings.ToUpper(fileLevel)
	stdoutLevel = strings.ToUpper(stdoutLevel)
	if fileLevel == "" && logFile == "" {
		fileLevel = "DISABLED"
	} else if logFile == "" {
		logFile = "filebrowser.log"
	}
	if fileLevel == "" {
		fileLevel = "INFO"
	}
	if stdoutLevel == "" {
		stdoutLevel = "INFO"
	}
	var fileWriter, stdoutWriter io.Writer = io.Discard, io.Discard

	// Convert level strings to LogLevel
	fileLogLevel, ok := stringToLevel[strings.ToUpper(fileLevel)]
	if !ok {
		return fmt.Errorf("invalid file log level: %s", fileLevel)
	}
	stdoutLogLevel, ok := stringToLevel[strings.ToUpper(stdoutLevel)]
	if !ok {
		return fmt.Errorf("invalid stdout log level: %s", stdoutLevel)
	}

	// Configure file logging
	if fileLevel != "DISABLED" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file: %v", err)
		}
		fileWriter = file
	}

	// Configure stdout logging
	if stdoutLevel != "DISABLED" {
		stdoutWriter = os.Stdout
	}

	// Create the logger
	logger = NewLogger(fileWriter, stdoutWriter, fileLogLevel, stdoutLogLevel)
	return nil
}
