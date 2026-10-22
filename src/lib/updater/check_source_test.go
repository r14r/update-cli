package updater

import "testing"

func TestCheckSourceKind(t *testing.T) {
	tests := []struct {
		sourceType string
		want       string
	}{
		{"repository", "REPO"},
		{"git", "REPO"},
		{"download", "FILE"},
		{"file", "FILE"},
		{"url", "URL"},
		{"", "none"},
	}
	for _, tt := range tests {
		if got := checkSourceKind(tt.sourceType); got != tt.want {
			t.Errorf("checkSourceKind(%q) = %q, want %q", tt.sourceType, got, tt.want)
		}
	}
}
