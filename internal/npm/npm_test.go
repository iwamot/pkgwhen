package npm

import (
	"strings"
	"testing"
	"time"

	"github.com/iwamot/pkgwhen/internal/release"
)

func TestURL(t *testing.T) {
	if got := URL("fastq"); got != "https://registry.npmjs.org/fastq" {
		t.Errorf("URL = %q", got)
	}
	if got := URL("@types/node"); got != "https://registry.npmjs.org/@types%2Fnode" {
		t.Errorf("URL = %q", got)
	}
}

func TestDecode(t *testing.T) {
	data := []byte(`{
		"versions": {
			"1.20.2": {},
			"1.20.3": {"deprecated": "use 1.21"},
			"1.21.0-beta.1": {"deprecated": ""},
			"0.1.0": {"deprecated": true},
			"9.9.9": {}
		},
		"time": {
			"created": "2020-01-01T00:00:00.000Z",
			"modified": "2026-08-29T11:02:14.000Z",
			"0.1.0": "2020-01-01T00:00:00.000Z",
			"1.20.2": "2026-08-20T09:00:00.000Z",
			"1.20.3": "2026-08-29T11:02:14.000Z",
			"1.21.0-beta.1": "2026-09-01T00:00:00.000Z",
			"8.8.8": "2021-01-01T00:00:00.000Z"
		}
	}`)
	got, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	release.Sort(got)
	want := []release.Release{
		{Version: "1.21.0-beta.1", Published: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Prerelease: true},
		{Version: "1.20.3", Published: time.Date(2026, 8, 29, 11, 2, 14, 0, time.UTC), Deprecated: true},
		{Version: "1.20.2", Published: time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)},
		{Version: "0.1.0", Published: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Deprecated: true},
	}
	if len(got) != len(want) {
		t.Fatalf("Decode = %+v", got)
	}
	for i := range want {
		if !got[i].Published.Equal(want[i].Published) || got[i].Version != want[i].Version || got[i].Deprecated != want[i].Deprecated || got[i].Prerelease != want[i].Prerelease {
			t.Errorf("release %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestDecodeErrors(t *testing.T) {
	if _, err := Decode([]byte(`[]`)); err == nil || !strings.Contains(err.Error(), "npm:") {
		t.Errorf("bad json err = %v", err)
	}
	bad := []byte(`{"versions": {"1.0.0": {}}, "time": {"1.0.0": "yesterday"}}`)
	if _, err := Decode(bad); err == nil || !strings.Contains(err.Error(), "version 1.0.0") {
		t.Errorf("bad time err = %v", err)
	}
}

func TestIsPrerelease(t *testing.T) {
	for v, want := range map[string]bool{
		"1.20.3":           false,
		"1.21.0-beta.1":    true,
		"2.0.0-0":          true,
		"1.0.0+build-1":    false,
		"1.0.0-rc.1+build": true,
		"1.0.0+20260906":   false,
	} {
		if got := IsPrerelease(v); got != want {
			t.Errorf("IsPrerelease(%q) = %v, want %v", v, got, want)
		}
	}
}
