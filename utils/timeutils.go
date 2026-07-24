package utils

import (
	"fmt"
	"time"
)

func ParseTimestamp(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05", // Common DB format
		"2006-01-02T15:04:05", // ISO 8601 without offset
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("failed to parse timestamp %q", s)
}
