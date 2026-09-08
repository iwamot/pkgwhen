package fetch

import (
	"strconv"
	"testing"
	"time"
)

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

func TestRateLimited(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	epoch := func(d time.Duration) string { return strconv.FormatInt(now.Add(d).Unix(), 10) }
	tests := []struct {
		name string
		resp Response
		want string
	}{
		{"retry-after wins over the reset", Response{RetryAfter: "30", RateLimitRemaining: "0", RateLimitReset: epoch(12 * time.Minute)}, "rate limited; retry after 30s"},
		{"the hourly reset", Response{RateLimitRemaining: "0", RateLimitReset: epoch(12 * time.Minute)}, "rate limited for 12m"},
		{"a reset under a minute", Response{RateLimitRemaining: "0", RateLimitReset: epoch(30 * time.Second)}, "rate limited for 30s"},
		{"no headers", Response{}, "rate limited; wait at least a minute"},
		{"a reset that has passed", Response{RateLimitRemaining: "0", RateLimitReset: epoch(-time.Minute)}, "rate limited; wait at least a minute"},
		{"a reset that does not parse", Response{RateLimitRemaining: "0", RateLimitReset: "soon"}, "rate limited; wait at least a minute"},
		{"a retry-after that does not parse", Response{RetryAfter: "Wed, 21 Oct 2026 07:28:00 GMT"}, "rate limited; wait at least a minute"},
		{"a retry-after of zero", Response{RetryAfter: "0"}, "rate limited; wait at least a minute"},
		{"remaining that is not zero", Response{RateLimitRemaining: "12", RateLimitReset: epoch(12 * time.Minute)}, "rate limited; wait at least a minute"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RateLimited(tt.resp, now); got != tt.want {
				t.Errorf("RateLimited = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatusError(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		resp Response
		want string
	}{
		{"too many requests carries the wait", Response{Status: 429, RetryAfter: "30"}, "rate limited; retry after 30s"},
		{"a server error is worth another call", Response{Status: 500}, "HTTP 500; the registry is failing, retry later"},
		{"a gateway error too", Response{Status: 503}, "HTTP 503; the registry is failing, retry later"},
		{"anything else is a refusal", Response{Status: 451}, "unexpected HTTP 451; retry once, and if it persists the registry is rejecting the request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusError(tt.resp, now).Error(); got != tt.want {
				t.Errorf("StatusError = %q, want %q", got, tt.want)
			}
		})
	}
}
