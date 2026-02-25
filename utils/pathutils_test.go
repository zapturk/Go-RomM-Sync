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
