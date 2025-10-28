package config

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type Logger struct {
	std *log.Logger
}

func NewLogger() *Logger {
	return &Logger{
		std: log.New(os.Stdout, "", 0),
	}
}

func (l *Logger) Info(action string, entity string, entityID int64, adminID int64, status string) {
	l.write("info", map[string]any{
		"time":      time.Now().UTC().Format(time.RFC3339),
		"action":    action,
		"entity":    entity,
		"entity_id": entityID,
		"admin_id":  adminID,
		"status":    status,
	})
}

func (l *Logger) Error(err error, action string, entity string, entityID int64, adminID int64) {
	l.write("error", map[string]any{
		"time":      time.Now().UTC().Format(time.RFC3339),
		"action":    action,
		"entity":    entity,
		"entity_id": entityID,
		"admin_id":  adminID,
		"error":     err.Error(),
	})
}

func (l *Logger) write(level string, payload map[string]any) {
	payload["level"] = level
	data, err := json.Marshal(payload)
	if err != nil {
		l.std.Printf(`{"level":"error","error":"logger marshal: %v"}`, err)
		return
	}
	l.std.Println(string(data))
}
