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

func TestWindowKeep(t *testing.T) {
	day := 24 * time.Hour
	tests := []struct {
		name string
		w    Window
		at   time.Time
		want bool
	}{
		{"unset keeps everything", Window{}, at(0), true},
		{"min-age keeps older", Window{MinAge: day, MinAgeSet: true}, at(2 * day), true},
		{"min-age keeps the edge", Window{MinAge: day, MinAgeSet: true}, at(day), true},
		{"min-age drops newer", Window{MinAge: day, MinAgeSet: true}, at(day - time.Second), false},
		{"since keeps newer", Window{Since: day, SinceSet: true}, at(time.Hour), true},
		{"since keeps the edge", Window{Since: day, SinceSet: true}, at(day), true},
		{"since drops older", Window{Since: day, SinceSet: true}, at(day + time.Second), false},
		{"both form a range", Window{MinAge: day, MinAgeSet: true, Since: 3 * day, SinceSet: true}, at(2 * day), true},
		{"range drops outside", Window{MinAge: day, MinAgeSet: true, Since: 3 * day, SinceSet: true}, at(4 * day), false},
		{"zero min-age set keeps now", Window{MinAgeSet: true}, at(0), true},
		{"zero since set drops the past", Window{SinceSet: true}, at(time.Second), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.Keep(Release{Published: tt.at}, now); got != tt.want {
				t.Errorf("Keep = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	rs := []Release{{Version: "new", Published: at(time.Hour)}, {Version: "old", Published: at(48 * time.Hour)}}
	got := Filter(rs, now, Window{MinAge: 24 * time.Hour, MinAgeSet: true})
	if len(got) != 1 || got[0].Version != "old" {
		t.Errorf("Filter = %+v", got)
	}
	if got := Filter(nil, now, Window{}); got != nil {
		t.Errorf("Filter(nil) = %+v", got)
	}
}

func TestSort(t *testing.T) {
	rs := []Release{
		{Version: "1.0.0", Published: at(3 * time.Hour)},
		{Version: "1.0.1", Published: at(time.Hour)},
		{Version: "0.9.9", Published: at(time.Hour)},
		{Version: "1.1.0", Published: at(2 * time.Hour)},
	}
	Sort(rs)
	var got []string
	for _, r := range rs {
		got = append(got, r.Version)
	}
	want := []string{"1.0.1", "0.9.9", "1.1.0", "1.0.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Sort = %v, want %v", got, want)
	}
}

func TestLimit(t *testing.T) {
	rs := []Release{{Version: "a"}, {Version: "b"}, {Version: "c"}}
	if got, more := Limit(rs, 2); len(got) != 2 || more != 1 {
		t.Errorf("Limit(2) = %d rows, %d more", len(got), more)
	}
	if got, more := Limit(rs, 3); len(got) != 3 || more != 0 {
		t.Errorf("Limit(3) = %d rows, %d more", len(got), more)
	}
	if got, more := Limit(rs, 0); len(got) != 3 || more != 0 {
		t.Errorf("Limit(0) = %d rows, %d more", len(got), more)
	}
}

func TestAge(t *testing.T) {
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{0, "0m"},
		{-time.Hour, "0m"},
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
	if got := More(3796); got != "... and 3796 more (pass -n N or --all)\n" {
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
