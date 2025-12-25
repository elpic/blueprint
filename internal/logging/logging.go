package logging

import (
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel int

const (
	// INFO is the default log level - shows normal output
	INFO LogLevel = iota
	// DEBUG shows additional debug information
	DEBUG
)

var currentLogLevel LogLevel = INFO

// init reads the LOGLEVEL environment variable on package load
func init() {
	level := os.Getenv("LOGLEVEL")
	setLogLevel(strings.ToUpper(level))
}

// setLogLevel sets the log level based on a string value
func setLogLevel(level string) {
	switch level {
	case "DEBUG":
		currentLogLevel = DEBUG
	case "INFO", "":
		currentLogLevel = INFO
	default:
		currentLogLevel = INFO
	}
}

// IsDebug returns true if the current log level is DEBUG or higher
func IsDebug() bool {
	return currentLogLevel >= DEBUG
}

// SetLogLevel allows setting the log level programmatically
func SetLogLevel(level LogLevel) {
	currentLogLevel = level
}

// GetLogLevel returns the current log level
func GetLogLevel() LogLevel {
	return currentLogLevel
}
