// Package release holds one version's publish facts and turns a list of them
// into the table, the one-line form, and the JSON document. Nothing here does
// I/O; the registries decode their own responses into Release values.
package release

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Release is one version of a package with the moment it was published.
type Release struct {
	Version    string
	Published  time.Time
	Yanked     bool
	Deprecated bool
	Prerelease bool
}

// Marks lists the flags shown at the end of a row, in a fixed order.
func (r Release) Marks() []string {
	var m []string
	if r.Yanked {
		m = append(m, "yanked")
	}
	if r.Deprecated {
		m = append(m, "deprecated")
	}
	if r.Prerelease {
		m = append(m, "pre")
	}
	return m
}

// Window narrows a list to a range of publish dates. MinAge keeps versions
// published at least that long ago; Since keeps versions published within
// that long. The Set flags tell an unset option from a zero one.
type Window struct {
	MinAge    time.Duration
	MinAgeSet bool
	Since     time.Duration
	SinceSet  bool
}

// Keep reports whether r falls inside the window as of now. Both edges are
// inclusive, so a version published exactly one day ago passes --min-age 1d.
func (w Window) Keep(r Release, now time.Time) bool {
	if w.MinAgeSet && r.Published.After(now.Add(-w.MinAge)) {
		return false
	}
	if w.SinceSet && r.Published.Before(now.Add(-w.Since)) {
		return false
	}
	return true
}

// Filter returns the releases inside w.
func Filter(rs []Release, now time.Time, w Window) []Release {
	var out []Release
	for _, r := range rs {
		if w.Keep(r, now) {
			out = append(out, r)
		}
	}
	return out
}

// Sort orders newest first. Versions published at the same instant are
// ordered by version string, descending, so the output is stable.
func Sort(rs []Release) {
	sort.SliceStable(rs, func(i, j int) bool {
		if !rs[i].Published.Equal(rs[j].Published) {
			return rs[i].Published.After(rs[j].Published)
		}
		return rs[i].Version > rs[j].Version
	})
}

// Limit keeps the first n releases and reports how many were cut. n <= 0
// keeps everything.
func Limit(rs []Release, n int) ([]Release, int) {
	if n <= 0 || len(rs) <= n {
		return rs, 0
	}
	return rs[:n], len(rs) - n
}

// Age renders how long ago t was, as of now: days from one day up, hours
// from one hour up, minutes below that. A future t reads as 0m.
func Age(now, t time.Time) string {
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
}

// Table renders the list as aligned columns with a header. Dates are shown
// to the day; the time of day is in the one-line form and in JSON.
func Table(rs []Release, now time.Time) string {
	rows := [][]string{{"VERSION", "PUBLISHED", "AGE"}}
	for _, r := range rs {
		rows = append(rows, []string{r.Version, r.Published.UTC().Format("2006-01-02"), Age(now, r.Published), strings.Join(r.Marks(), " ")})
	}
	width := make([]int, 3)
	for _, row := range rows {
		for i := range 3 {
			width[i] = max(width[i], len(row[i]))
		}
	}
	var b strings.Builder
	for _, row := range rows {
		line := fmt.Sprintf("%-*s  %-*s  %-*s", width[0], row[0], width[1], row[1], width[2], row[2])
		if len(row) > 3 && row[3] != "" {
			line += "  " + row[3]
		}
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// Line renders one release on one line, with the full timestamp.
func Line(r Release, now time.Time) string {
	line := fmt.Sprintf("%s  %s  %s", r.Version, r.Published.UTC().Format(time.RFC3339), Age(now, r.Published))
	if m := r.Marks(); len(m) > 0 {
		line += "  " + strings.Join(m, " ")
	}
	return line + "\n"
}

// More is the trailer printed when Limit cut the list.
func More(n int) string {
	return fmt.Sprintf("... and %d more (pass -n N or --all)\n", n)
}

type document struct {
	Registry string  `json:"registry"`
	Name     string  `json:"name"`
	Versions []entry `json:"versions"`
	More     int     `json:"more"`
}

type entry struct {
	Version    string `json:"version"`
	Published  string `json:"published"`
	AgeSeconds int64  `json:"age_seconds"`
	Yanked     bool   `json:"yanked"`
	Deprecated bool   `json:"deprecated"`
	Prerelease bool   `json:"prerelease"`
}

// JSON renders the list as one document. Timestamps are ISO 8601 in UTC and
// age is in whole seconds, so a caller can compare without parsing the table.
func JSON(registry, name string, rs []Release, more int, now time.Time) string {
	doc := document{Registry: registry, Name: name, Versions: []entry{}, More: more}
	for _, r := range rs {
		age := now.Sub(r.Published)
		if age < 0 {
			age = 0
		}
		doc.Versions = append(doc.Versions, entry{
			Version:    r.Version,
			Published:  r.Published.UTC().Format(time.RFC3339),
			AgeSeconds: int64(age.Seconds()),
			Yanked:     r.Yanked,
			Deprecated: r.Deprecated,
			Prerelease: r.Prerelease,
		})
	}
	// The document holds only strings, numbers, and booleans, so Marshal
	// cannot fail.
	b, _ := json.MarshalIndent(doc, "", "  ")
	return string(b) + "\n"
}
