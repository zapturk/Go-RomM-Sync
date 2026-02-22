package utils

import (
	"fmt"
	"time"
)

// ParseTimestamp attempts to parse a string using various common ISO 8601 and RFC 3339 formats.
func ParseTimestamp(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00", // Basic ISO 8601
		"2006-01-02 15:04:05",       // Common DB format
		"2006-01-02T15:04:05",       // ISO 8601 without offset (assume UTC)
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse timestamp %q with any supported format", s)
}
