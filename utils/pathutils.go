package utils

import (
	"path/filepath"
	"strings"
)

func SanitizePath(path string) string {
	p := filepath.Clean(path)
	if vol := filepath.VolumeName(p); vol != "" {
		p = strings.TrimPrefix(p, vol)
	} else if len(p) >= 2 && p[1] == ':' {
		p = p[2:]
	}

	p = filepath.ToSlash(p)
	for strings.HasPrefix(p, "../") || p == ".." {
		p = strings.TrimPrefix(p, "../")
		if p == ".." {
			p = "."
		}
	}
	p = strings.TrimPrefix(p, "/")

	if p == "" || p == "." {
		return "."
	}
	return filepath.FromSlash(p)
}

// IsSafePath checks if targetPath is safely contained within baseDir.
// It returns true if safe, and false if a path traversal is detected or if an error occurs.
func IsSafePath(baseDir, targetPath string) bool {
	cleanBase := filepath.Clean(baseDir)
	cleanTarget := filepath.Clean(targetPath)
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	return true
}
