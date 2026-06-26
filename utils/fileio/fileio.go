package fileio

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// WriteFileFromReader reads from r and writes to the given path with the specified permissions.
func WriteFileFromReader(path string, r io.Reader, perm os.FileMode) error {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

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
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
