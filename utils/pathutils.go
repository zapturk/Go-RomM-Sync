package utils

import (
	"path/filepath"
	"strings"
)

// SanitizePath ensures a path received from a server is safe for use in local operations.
// It removes any directory traversal segments (..) and ensures the path is clean.
func SanitizePath(path string) string {
	// 1. Clean the path to resolve any internal .. or .
	p := filepath.Clean(path)

	// 2. Remove Windows volume names
	if vol := filepath.VolumeName(p); vol != "" {
		p = strings.TrimPrefix(p, vol)
	}

	// 3. Convert to forward slashes for consistent handling during sanitization
	p = filepath.ToSlash(p)

	// 4. If the path starts with .. or /, it's trying to escape.
	// Clean will keep a leading .. if the path is relative and starts with it.
	// We want to treat the path as a relative path from our own root.
	for strings.HasPrefix(p, "../") || p == ".." {
		p = strings.TrimPrefix(p, "../")
		if p == ".." {
			p = "."
		}
	}

	// Also remove leading slash to force relativity
	p = strings.TrimPrefix(p, "/")

	if p == "" || p == "." {
		return "."
	}

	return filepath.FromSlash(p)
}
