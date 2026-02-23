package fileio

import (
	"fmt"
	"io"
	"log"
)

// LogFunc matches the signature of logging functions used across the app (like LogErrorf).
type LogFunc func(format string, args ...interface{})

// Close closes the given io.Closer and logs any error that occurs.
// If logFunc is nil, it falls back to the standard log package.
// This is useful for following 'errcheck' linting rules without cluttering call sites.
func Close(c io.Closer, logFunc LogFunc, msg string) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		formatted := fmt.Sprintf("%s: %v", msg, err)
		if logFunc != nil {
			logFunc(formatted)
		} else {
			log.Println(formatted)
		}
	}
}
