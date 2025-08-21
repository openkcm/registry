package model

import "time"

// formatTime converts a time.Time value to its UTC string representation using the RFC3339Nano format.
// This ensures a stable serialized representation.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
