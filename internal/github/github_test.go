package github

import (
	"strings"
	"testing"
	"time"
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
	if _, err := DecodeList([]byte(`{}`)); err == nil || !strings.Contains(err.Error(), "github:") {
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
	tests := []struct {
		status    int
		remaining string
		want      string
	}{
		{403, "0", "rate limited; set GITHUB_TOKEN or run `gh auth login`"},
		{403, "12", "HTTP 403"},
		{403, "", "HTTP 403"},
		{500, "0", "HTTP 500"},
	}
	for _, tt := range tests {
		got := StatusError("https://api.github.com/x", tt.status, tt.remaining).Error()
		if !strings.HasPrefix(got, "github: https://api.github.com/x: ") || !strings.Contains(got, tt.want) {
			t.Errorf("StatusError(%d, %q) = %q, want containing %q", tt.status, tt.remaining, got, tt.want)
		}
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
