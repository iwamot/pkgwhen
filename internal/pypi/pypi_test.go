package pypi

import (
	"strings"
	"testing"
	"time"

	"github.com/iwamot/pkgwhen/internal/release"
)

func TestURLs(t *testing.T) {
	if got := ProjectURL("openai-agents"); got != "https://pypi.org/pypi/openai-agents/json" {
		t.Errorf("ProjectURL = %q", got)
	}
	if got := VersionURL("openai-agents", "0.21.0"); got != "https://pypi.org/pypi/openai-agents/0.21.0/json" {
		t.Errorf("VersionURL = %q", got)
	}
}

func TestDecodeProject(t *testing.T) {
	data := []byte(`{"releases": {
		"0.0.1": [
			{"upload_time_iso_8601": "2025-03-04T18:16:23.000000Z", "yanked": true},
			{"upload_time_iso_8601": "2025-03-04T18:16:20.000000Z", "yanked": false}
		],
		"0.21.0": [{"upload_time_iso_8601": "2026-09-03T10:00:00.123456Z", "yanked": false}],
		"0.22.0a1": [{"upload_time_iso_8601": "2026-09-05T10:00:00Z", "yanked": false}],
		"1.0.0": []
	}}`)
	got, err := DecodeProject(data)
	if err != nil {
		t.Fatal(err)
	}
	release.Sort(got)
	want := []release.Release{
		{Version: "0.22.0a1", Published: time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC), Prerelease: true},
		{Version: "0.21.0", Published: time.Date(2026, 9, 3, 10, 0, 0, 123456000, time.UTC)},
		{Version: "0.0.1", Published: time.Date(2025, 3, 4, 18, 16, 20, 0, time.UTC), Yanked: true},
	}
	if len(got) != len(want) {
		t.Fatalf("DecodeProject = %+v", got)
	}
	for i := range want {
		if !got[i].Published.Equal(want[i].Published) || got[i].Version != want[i].Version || got[i].Yanked != want[i].Yanked || got[i].Prerelease != want[i].Prerelease {
			t.Errorf("release %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDecodeProjectErrors(t *testing.T) {
	if _, err := DecodeProject([]byte(`not json`)); err == nil {
		t.Errorf("bad json err = %v", err)
	}
	bad := []byte(`{"releases": {"1.0": [{"upload_time_iso_8601": "yesterday"}]}}`)
	if _, err := DecodeProject(bad); err == nil || !strings.Contains(err.Error(), "version 1.0") {
		t.Errorf("bad time err = %v", err)
	}
}

func TestDecodeVersion(t *testing.T) {
	data := []byte(`{"info": {"version": "0.0.1"}, "urls": [
		{"upload_time_iso_8601": "2025-03-04T18:16:20Z", "yanked": true},
		{"upload_time_iso_8601": "2025-03-04T18:16:23Z", "yanked": true}
	]}`)
	got, err := DecodeVersion(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "0.0.1" || !got.Yanked || got.Prerelease || !got.Published.Equal(time.Date(2025, 3, 4, 18, 16, 20, 0, time.UTC)) {
		t.Errorf("DecodeVersion = %+v", got)
	}
	if _, err := DecodeVersion([]byte(`[]`)); err == nil {
		t.Error("bad json: no error")
	}
	if _, err := DecodeVersion([]byte(`{"info": {"version": "1.0"}, "urls": []}`)); err == nil || !strings.Contains(err.Error(), "no files") {
		t.Errorf("no files err = %v", err)
	}
	if _, err := DecodeVersion([]byte(`{"info": {"version": "1.0"}, "urls": [{"upload_time_iso_8601": "x"}]}`)); err == nil {
		t.Error("bad time: no error")
	}
}

func TestIsPrerelease(t *testing.T) {
	tests := []struct {
		v    string
		want bool
	}{
		{"1.0.0", false},
		{"0.21.0", false},
		{"1.0", false},
		{"1.0.post1", false},
		{"1.0.0.post1", false},
		{"1!2.0", false},
		{"v1.0", false},
		{"1.0a1", true},
		{"1.0b2", true},
		{"1.0rc1", true},
		{"1.0.0rc1", true},
		{"1.0-alpha1", true},
		{"1.0.beta.2", true},
		{"1.0c1", true},
		{"1.0pre1", true},
		{"1.0preview", true},
		{"1.0.dev1", true},
		{"1.0dev", true},
		{"1.0.0.dev20260906", true},
		{"1.0rc1.post1.dev2", true},
		{"1.0.post1.dev2", true},
		{"2026.09.06", false},
		{"not-a-version", false},
	}
	for _, tt := range tests {
		if got := IsPrerelease(tt.v); got != tt.want {
			t.Errorf("IsPrerelease(%q) = %v, want %v", tt.v, got, tt.want)
		}
	}
}
