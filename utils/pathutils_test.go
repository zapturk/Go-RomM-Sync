package utils

import (
	"path/filepath"
	"testing"
)

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal/path/file.txt", "normal/path/file.txt"},
		{"../../etc/passwd", "etc/passwd"},
		{"/abs/path", "abs/path"},
		{"path/../with/traversal", "with/traversal"},
		{"C:/Users/test", "Users/test"},
		{"..", "."},
		{"./././", "."},
		{"", "."},
	}

	for _, tt := range tests {
		result := filepath.ToSlash(SanitizePath(tt.input))
		if result != tt.expected {
			t.Errorf("SanitizePath(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsSafePath(t *testing.T) {
	tests := []struct {
		baseDir    string
		targetPath string
		expected   bool
	}{
		{"/var/lib", "/var/lib/some/file.txt", true},
		{"/var/lib", "/var/lib", true},
		{"/var/lib", "/var/lib/../lib/file.txt", true},
		{"/var/lib", "/var/file.txt", false},
		{"/var/lib", "/var/lib/../../etc/passwd", false},
		{"/var/lib", "/etc/passwd", false},
		{"relative", "relative/sub/path", true},
		{"relative", "other/path", false},
	}

	for _, tt := range tests {
		result := IsSafePath(tt.baseDir, tt.targetPath)
		if result != tt.expected {
			t.Errorf("IsSafePath(%q, %q) = %t, expected %t", tt.baseDir, tt.targetPath, result, tt.expected)
		}
	}
}

