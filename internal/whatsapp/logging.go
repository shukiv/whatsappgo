package whatsapp

import (
	"log"
	"os"
	"strings"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// logLevelVar names the environment variable that turns on what the WhatsApp
// library has to say. The daemon writes its own log lines to stderr and the
// library writes to stdout; the desktop client forwards both when it is asked
// to show backend logs.
const logLevelVar = "WHATSAPPGO_LOG_LEVEL"

// libraryLog reports where the WhatsApp library should write what it sees.
//
// It stays silent unless the environment asks otherwise, and not only to keep
// the output small: at DEBUG the library prints the contents of every message
// it handles, so turning this on writes the conversation into a log file in
// plain text. That is worth having while a problem is being chased and is not
// something to leave running.
func libraryLog() waLog.Logger {
	level := strings.ToUpper(strings.TrimSpace(os.Getenv(logLevelVar)))
	switch level {
	case "":
		return waLog.Noop
	case "DEBUG", "INFO", "WARN", "ERROR":
		return waLog.Stdout("whatsmeow", level, false)
	default:
		log.Printf("ignoring %s=%q: expected DEBUG, INFO, WARN or ERROR", logLevelVar, level)
		return waLog.Noop
	}
}
