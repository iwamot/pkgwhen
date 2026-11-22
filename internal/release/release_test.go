package release

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func at(d time.Duration) time.Time { return now.Add(-d) }

func TestMarks(t *testing.T) {
	tests := []struct {
		r    Release
		want []string
	}{
		{Release{}, nil},
		{Release{Yanked: true}, []string{"yanked"}},
		{Release{Deprecated: true}, []string{"deprecated"}},
		{Release{Prerelease: true}, []string{"pre"}},
		{Release{Yanked: true, Deprecated: true, Prerelease: true}, []string{"yanked", "deprecated", "pre"}},
	}
	for _, tt := range tests {
		if got := tt.r.Marks(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Marks(%+v) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestAge(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{0, "0s"},
		{-time.Hour, "0s"},
		{30 * time.Second, "30s"},
		{time.Minute, "1m"},
		{59 * time.Minute, "59m"},
		{time.Hour, "1h"},
		{23*time.Hour + 59*time.Minute, "23h"},
		{24 * time.Hour, "1d"},
		{551*24*time.Hour + 5*time.Hour, "551d"},
	}
	for _, tt := range tests {
		if got := Age(now, at(tt.ago)); got != tt.want {
			t.Errorf("Age(%v ago) = %q, want %q", tt.ago, got, tt.want)
		}
	}
}

func TestFind(t *testing.T) {
	rs := []Release{{Version: "1.0.0"}, {Version: "v1.1.0"}}
	if got, ok := Find(rs, "v1.1.0"); !ok || got.Version != "v1.1.0" {
		t.Errorf("Find(v1.1.0) = %+v, %v", got, ok)
	}
	if _, ok := Find(rs, "1.1.0"); ok {
		t.Error("Find(1.1.0) matched a differently spelled version")
	}
	if _, ok := Find(nil, "1.0.0"); ok {
		t.Error("Find on an empty list reported a release")
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct {
		r    Release
		want string
	}{
		{Release{Version: "1.3.2", Published: at(45 * 24 * time.Hour)}, "1.3.2, published 45d ago"},
		{Release{Version: "1.3.2", Published: at(3 * time.Hour), Yanked: true}, "1.3.2 (yanked), published 3h ago"},
		{Release{Version: "2.0.0-rc.1", Published: at(90 * time.Second), Deprecated: true, Prerelease: true}, "2.0.0-rc.1 (deprecated, pre), published 1m ago"},
	}
	for _, tt := range tests {
		if got := Describe(tt.r, now); got != tt.want {
			t.Errorf("Describe = %q, want %q", got, tt.want)
		}
	}
}

func TestTable(t *testing.T) {
	rs := []Release{
		{Version: "0.21.0", Published: at(3 * 24 * time.Hour)},
		{Version: "0.0.1", Published: time.Date(2025, 3, 4, 18, 16, 20, 0, time.UTC), Yanked: true, Prerelease: true},
	}
	want := strings.Join([]string{
		"VERSION  PUBLISHED   AGE",
		"0.21.0   2026-09-03  3d",
		"0.0.1    2025-03-04  550d  yanked pre",
		"",
	}, "\n")
	if got := Table(rs, now); got != want {
		t.Errorf("Table =\n%s\nwant\n%s", got, want)
	}
	if got := Table(nil, now); got != "VERSION  PUBLISHED  AGE\n" {
		t.Errorf("Table(nil) = %q", got)
	}
}

func TestLine(t *testing.T) {
	r := Release{Version: "1.20.3", Published: time.Date(2026, 8, 29, 11, 2, 14, 0, time.UTC)}
	if got, want := Line(r, now), "1.20.3  2026-08-29T11:02:14Z  8d\n"; got != want {
		t.Errorf("Line = %q, want %q", got, want)
	}
	r.Deprecated = true
	if got, want := Line(r, now), "1.20.3  2026-08-29T11:02:14Z  8d  deprecated\n"; got != want {
		t.Errorf("Line = %q, want %q", got, want)
	}
}

func TestMore(t *testing.T) {
	if got := More(3796); got != "3796 more versions; pass -n N or --all to see them" {
		t.Errorf("More = %q", got)
	}
}

func TestJSON(t *testing.T) {
	rs := []Release{{Version: "0.21.0", Published: at(3 * 24 * time.Hour), Prerelease: true}, {Version: "future", Published: at(-time.Hour)}}
	want := strings.Join([]string{
		"{",
		`  "registry": "pypi",`,
		`  "name": "openai-agents",`,
		`  "versions": [`,
		"    {",
		`      "version": "0.21.0",`,
		`      "published": "2026-09-03T12:00:00Z",`,
		`      "age_seconds": 259200,`,
		`      "yanked": false,`,
		`      "deprecated": false,`,
		`      "prerelease": true`,
		"    },",
		"    {",
		`      "version": "future",`,
		`      "published": "2026-09-06T13:00:00Z",`,
		`      "age_seconds": 0,`,
		`      "yanked": false,`,
		`      "deprecated": false,`,
		`      "prerelease": false`,
		"    }",
		"  ],",
		`  "more": 7`,
		"}",
		"",
	}, "\n")
	if got := JSON("pypi", "openai-agents", rs, 7, now); got != want {
		t.Errorf("JSON =\n%s\nwant\n%s", got, want)
	}
	if got := JSON("npm", "x", nil, 0, now); !strings.Contains(got, `"versions": []`) {
		t.Errorf("JSON(nil) = %s", got)
	}
}

func TestNoVersions(t *testing.T) {
	tests := []struct{ registry, want string }{
		{"github-releases", "no releases published (the repository may have tags but no releases)"},
		{"pypi", "no versions with a publish date"},
		{"npm", "no versions with a publish date"},
	}
	for _, tt := range tests {
		if got := NoVersions(tt.registry); got != tt.want {
			t.Errorf("NoVersions(%q) = %q, want %q", tt.registry, got, tt.want)
		}
	}
}
