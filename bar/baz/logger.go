package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

var logger = log.New(os.Stderr, "", 0)

// Entry defines a log entry.
type Entry struct {
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
	Trace    string `json:"logger.googleapis.com/trace,omitempty"`

	// Logs Explorer allows filtering and display of this as `jsonPayload.component`.
	Component string `json:"component,omitempty"`
}

// String renders an entry structure to the JSON format expected by Cloud Logging.
func (e Entry) String() string {
	if e.Severity == "" {
		e.Severity = "INFO"
	}
	out, err := json.Marshal(e)
	if err != nil {
		log.Printf("json.Marshal: %v", err)
	}
	return string(out)
}

// Debug logs a message at DEBUG level.
func Debug(v ...interface{}) {
	logger.Println(Entry{
		Message: fmt.Sprint(v...),
		Severity: "DEBUG",
	})
}

// Info logs a message at INFO level.
func Info(v ...interface{}) {
	logger.Println(Entry{
		Message: fmt.Sprint(v...),
		Severity: "INFO",
	})
}

// Error logs a message at ERROR level.
func Error(v ...interface{}) {
	logger.Println(Entry{
		Message: fmt.Sprint(v...),
		Severity: "ERROR",
	})
}

// Warning logs a message at WARNING level.
func Warning(v ...interface{}) {
	logger.Println(Entry{
		Message: fmt.Sprint(v...),
		Severity: "WARNING",
	})
}

// Notice logs a message at NOTICE level.
func Notice(v ...interface{}) {
	logger.Println(Entry{
		Message: fmt.Sprint(v...),
		Severity: "NOTICE",
	})
}

// Critical logs a message at CRITICAL level.
func Critical(v ...interface{}) {
	logger.Println(Entry{
		Message: fmt.Sprint(v...),
		Severity: "CRITICAL",
	})
}

// Alert logs a message at ALERT level.
func Alert(v ...interface{}) {
	logger.Println(Entry{
		Message: fmt.Sprint(v...),
		Severity: "ALERT",
	})
}

// Emergency logs a message at EMERGENCY level.
func Emergency(v ...interface{}) {
	logger.Println(Entry{
		Message: fmt.Sprint(v...),
		Severity: "EMERGENCY",
	})
}
