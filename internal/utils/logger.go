package utils

import (
	
	"os"

	"github.com/sirupsen/logrus"
)

// Log is the global logger instance
var Log = logrus.New()

func init() {
	// JSON formatted logs (better for production)
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05Z07:00",
		PrettyPrint:     isLocalDev(),
	})

	// Set log level based on environment
	if isLocalDev() {
		Log.SetLevel(logrus.DebugLevel)
	} else {
		Log.SetLevel(logrus.InfoLevel)
	}

	// Output to stdout
	Log.SetOutput(os.Stdout)
}

func isLocalDev() bool {
	env := os.Getenv("ENVIRONMENT")
	return env == "" || env == "development" || env == "dev"
}

// LogInfo logs an info message with fields
func LogInfo(msg string, fields map[string]interface{}) {
	Log.WithFields(logrus.Fields(fields)).Info(msg)
}

// LogError logs an error with fields
func LogError(msg string, err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["error"] = err.Error()
	Log.WithFields(logrus.Fields(fields)).Error(msg)
}

// LogWarn logs a warning with fields
func LogWarn(msg string, fields map[string]interface{}) {
	Log.WithFields(logrus.Fields(fields)).Warn(msg)
}