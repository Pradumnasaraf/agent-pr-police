package ghclient

import "testing"

func TestFindSticky(t *testing.T) {
	marker := "<!-- agent-pr-guard:sticky-comment -->"
	comments := []Comment{
		{ID: 1, Body: "first comment"},
		{ID: 2, Body: marker + "\n## report"},
		{ID: 3, Body: "another"},
	}
	if got := FindSticky(comments, marker); got != 2 {
		t.Errorf("FindSticky = %d, want 2", got)
	}
	if got := FindSticky([]Comment{{ID: 1, Body: "no marker"}}, marker); got != 0 {
		t.Errorf("FindSticky with no match = %d, want 0", got)
	}
}

func TestURLPathEscape(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Dockerfile", "Dockerfile"},
		{"app/Dockerfile", "app/Dockerfile"},
		{"dir with space/file.tf", "dir%20with%20space/file.tf"},
	}
	for _, tt := range tests {
		if got := urlPathEscape(tt.in); got != tt.want {
			t.Errorf("urlPathEscape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
