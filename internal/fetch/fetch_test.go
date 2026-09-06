package fetch

import "testing"

func TestNextLink(t *testing.T) {
	tests := []struct {
		header, want string
	}{
		{"", ""},
		{`<https://api.github.com/repositories/1/releases?per_page=100&page=2>; rel="next", <https://api.github.com/repositories/1/releases?per_page=100&page=5>; rel="last"`, "https://api.github.com/repositories/1/releases?per_page=100&page=2"},
		{`<https://api.github.com/repositories/1/releases?per_page=100&page=1>; rel="prev", <https://api.github.com/repositories/1/releases?per_page=100&page=1>; rel="first"`, ""},
	}
	for _, tt := range tests {
		if got := NextLink(tt.header); got != tt.want {
			t.Errorf("NextLink(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}
