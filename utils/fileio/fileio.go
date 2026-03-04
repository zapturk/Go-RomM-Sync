package fileio

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// LogFunc matches the signature of logging functions used across the app (like LogErrorf).
type LogFunc func(format string, args ...interface{})

// Close closes the given io.Closer and logs any error that occurs.
// If logFunc is nil, the error is ignored (silent fallback).
// This is useful for following 'errcheck' linting rules without cluttering call sites.
func Close(c io.Closer, logFunc LogFunc, msg string) {
	if c == nil {
		return
	}
	if err := c.Close(); err != nil {
		if logFunc != nil {
			formatted := fmt.Sprintf("%s: %v", msg, err)
			logFunc(formatted)
		}
	}
}

// MkdirAll is a wrapper for os.MkdirAll that logs any error.
func MkdirAll(path string, perm os.FileMode, logFunc LogFunc) {
	if err := os.MkdirAll(path, perm); err != nil {
		if logFunc != nil {
			formatted := fmt.Sprintf("MkdirAll failed for %s: %v", path, err)
			logFunc(formatted)
		}
	}
}

// Remove is a wrapper for os.Remove that logs any error.
func Remove(path string, logFunc LogFunc) {
	if err := os.Remove(path); err != nil {
		if logFunc != nil {
			formatted := fmt.Sprintf("Remove failed for %s: %v", path, err)
			logFunc(formatted)
		}
	}
}

// RemoveAll is a wrapper for os.RemoveAll that logs any error.
func RemoveAll(path string, logFunc LogFunc) {
	if err := os.RemoveAll(path); err != nil {
		if logFunc != nil {
			formatted := fmt.Sprintf("RemoveAll failed for %s: %v", path, err)
			logFunc(formatted)
		}
	}
}

// WriteFileFromReader reads from r and writes to the given path with the specified permissions.
func WriteFileFromReader(path string, r io.Reader, perm os.FileMode) error {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer Close(out, nil, "WriteFileFromReader: Failed to close file")

	if _, err := io.Copy(out, r); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

// GetMD5 returns the MD5 hash of the file at the given path.
func GetMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer Close(f, nil, "GetMD5: Failed to close file")

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
