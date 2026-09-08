package github

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/iwamot/pkgwhen/internal/fetch"
)

func TestURLs(t *testing.T) {
	if got := ListURL("jdx/aube"); got != "https://api.github.com/repos/jdx/aube/releases?per_page=100" {
		t.Errorf("ListURL = %q", got)
	}
	if got := TagURL("jdx/aube", "v2.2.12"); got != "https://api.github.com/repos/jdx/aube/releases/tags/v2.2.12" {
		t.Errorf("TagURL = %q", got)
	}
}

func TestAlternateTag(t *testing.T) {
	if got := AlternateTag("2.2.12"); got != "v2.2.12" {
		t.Errorf("AlternateTag = %q", got)
	}
	if got := AlternateTag("v2.2.12"); got != "2.2.12" {
		t.Errorf("AlternateTag = %q", got)
	}
}

func TestDecodeList(t *testing.T) {
	data := []byte(`[
		{"tag_name": "v2.2.13", "draft": true, "prerelease": false, "published_at": null},
		{"tag_name": "v2.2.12", "draft": false, "prerelease": false, "published_at": "2026-09-06T00:44:39Z"},
		{"tag_name": "v2.3.0-rc.1", "draft": false, "prerelease": true, "published_at": "2026-09-05T21:42:28Z"}
	]`)
	got, err := DecodeList(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("DecodeList = %+v", got)
	}
	if got[0].Version != "v2.2.12" || got[0].Prerelease || !got[0].Published.Equal(time.Date(2026, 9, 6, 0, 44, 39, 0, time.UTC)) {
		t.Errorf("release 0 = %+v", got[0])
	}
	if got[1].Version != "v2.3.0-rc.1" || !got[1].Prerelease {
		t.Errorf("release 1 = %+v", got[1])
	}
	if _, err := DecodeList([]byte(`{}`)); err == nil {
		t.Errorf("bad json err = %v", err)
	}
	if _, err := DecodeList([]byte(`[{"tag_name": "v1", "published_at": "soon"}]`)); err == nil || !strings.Contains(err.Error(), "release v1") {
		t.Errorf("bad time err = %v", err)
	}
}

func TestDecodeOne(t *testing.T) {
	got, err := DecodeOne([]byte(`{"tag_name": "v2.2.12", "prerelease": false, "published_at": "2026-09-06T00:44:39Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "v2.2.12" || !got.Published.Equal(time.Date(2026, 9, 6, 0, 44, 39, 0, time.UTC)) {
		t.Errorf("DecodeOne = %+v", got)
	}
	if _, err := DecodeOne([]byte(`[]`)); err == nil {
		t.Error("bad json: no error")
	}
	if _, err := DecodeOne([]byte(`{"tag_name": "v1", "published_at": ""}`)); err == nil {
		t.Error("bad time: no error")
	}
}

func TestStatusError(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	epoch := func(d time.Duration) string { return strconv.FormatInt(now.Add(d).Unix(), 10) }
	tests := []struct {
		name      string
		resp      fetch.Response
		haveToken bool
		want      string
	}{
		{"hourly limit without a token", fetch.Response{Status: 403, RateLimitRemaining: "0", RateLimitReset: epoch(12 * time.Minute)}, false, "rate limited for 12m; set GITHUB_TOKEN or run `gh auth login`"},
		{"hourly limit with a token", fetch.Response{Status: 403, RateLimitRemaining: "0", RateLimitReset: epoch(12 * time.Minute)}, true, "rate limited for 12m"},
		{"retry-after wins over the reset", fetch.Response{Status: 429, RetryAfter: "30", RateLimitRemaining: "0", RateLimitReset: epoch(12 * time.Minute)}, true, "rate limited; retry after 30s"},
		{"429 with no headers", fetch.Response{Status: 429}, true, "rate limited; wait at least a minute"},
		{"a reset that has passed", fetch.Response{Status: 403, RateLimitRemaining: "0", RateLimitReset: epoch(-time.Minute)}, true, "rate limited; wait at least a minute"},
		{"a reset that does not parse", fetch.Response{Status: 403, RateLimitRemaining: "0", RateLimitReset: "soon"}, true, "rate limited; wait at least a minute"},
		{"a retry-after that does not parse", fetch.Response{Status: 403, RetryAfter: "Wed, 21 Oct 2026 07:28:00 GMT"}, true, "rate limited; wait at least a minute"},
		{"403 that is not a limit", fetch.Response{Status: 403, RateLimitRemaining: "12"}, true, "HTTP 403; the token may lack access to this repository"},
		{"a server error is delegated", fetch.Response{Status: 500, RateLimitRemaining: "0"}, true, "HTTP 500; the registry is failing, retry later"},
		{"another status is delegated", fetch.Response{Status: 451, RateLimitRemaining: "0"}, true, "unexpected HTTP 451; retry once, and if it persists the registry is rejecting the request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StatusError(tt.resp, tt.haveToken, now).Error(); got != tt.want {
				t.Errorf("StatusError = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHeaders(t *testing.T) {
	h := Headers("")
	if _, ok := h["Authorization"]; ok || h["Accept"] != "application/vnd.github+json" {
		t.Errorf("Headers(\"\") = %v", h)
	}
	if got := Headers("tok")["Authorization"]; got != "Bearer tok" {
		t.Errorf("Authorization = %q", got)
	}
}
