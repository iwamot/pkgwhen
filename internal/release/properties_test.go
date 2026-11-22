package release

import (
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/quick"
	"time"
)

// Generated inputs run in ordinary go test, so CI checks the properties as
// well as the explicit examples. A fixed seed makes failures reproducible.
func propertyConfig() *quick.Config {
	return &quick.Config{MaxCount: 2000, Rand: rand.New(rand.NewSource(1))}
}

const day = 24 * time.Hour

// spans are the window lengths a case can use. Ages are drawn from one second
// either side of each of them, so every edge of every window is exercised,
// along with a future publish time.
var spans = []time.Duration{0, time.Hour, day, 3 * day, 30 * day}

func randomAge(r *rand.Rand) time.Duration {
	if r.Intn(10) == 0 {
		return -time.Hour
	}
	return spans[r.Intn(len(spans))] + time.Duration(r.Intn(3)-1)*time.Second
}

// listCase is a list of releases in no particular order, as a registry's map
// hands them over, with versions that are all different and publish times
// that often coincide.
type listCase struct {
	Releases []Release
}

func (listCase) Generate(r *rand.Rand, _ int) reflect.Value {
	n := r.Intn(7)
	rs := make([]Release, n)
	for i := range rs {
		rs[i] = Release{
			Version:    fmt.Sprintf("%d.%d", r.Intn(3), i),
			Published:  now.Add(-randomAge(r)),
			Yanked:     r.Intn(4) == 0,
			Deprecated: r.Intn(4) == 0,
			Prerelease: r.Intn(4) == 0,
		}
	}
	r.Shuffle(n, func(i, j int) { rs[i], rs[j] = rs[j], rs[i] })
	return reflect.ValueOf(listCase{rs})
}

// windowCase is a window the flags can produce: --min-age longer than
// --since is refused before a registry is read.
type windowCase struct {
	Window Window
}

func (windowCase) Generate(r *rand.Rand, _ int) reflect.Value {
	var w Window
	if r.Intn(3) > 0 {
		w.MinAge, w.MinAgeSet, w.MinAgeText = spans[r.Intn(len(spans))], true, "MIN"
	}
	if r.Intn(3) > 0 {
		w.Since, w.SinceSet, w.SinceText = spans[r.Intn(len(spans))], true, "SINCE"
	}
	if w.MinAgeSet && w.SinceSet && w.MinAge > w.Since {
		w.MinAge, w.Since = w.Since, w.MinAge
	}
	return reflect.ValueOf(windowCase{w})
}

func age(r Release) time.Duration { return now.Sub(r.Published) }

// inside is the window read as ages: at least --min-age old, at most --since
// old, both edges included.
func inside(w Window, r Release) bool {
	return (!w.MinAgeSet || age(r) >= w.MinAge) && (!w.SinceSet || age(r) <= w.Since)
}

func unmarked(r Release) bool { return !r.Yanked && !r.Deprecated && !r.Prerelease }

func newest(rs []Release) []Release {
	var out []Release
	for _, r := range rs {
		if len(out) == 0 || r.Published.After(out[0].Published) {
			out = []Release{r}
		} else if r.Published.Equal(out[0].Published) {
			out = append(out, r)
		}
	}
	return out
}

func check(t *testing.T, property any) {
	t.Helper()
	if err := quick.Check(property, propertyConfig()); err != nil {
		t.Fatal(err)
	}
}

func TestFilterKeepsTheReleasesInsideTheWindowInOrder(t *testing.T) {
	check(t, func(c listCase, wc windowCase) bool {
		var want []Release
		for _, r := range c.Releases {
			if inside(wc.Window, r) {
				want = append(want, r)
			}
		}
		return slices.Equal(Filter(c.Releases, now, wc.Window), want)
	})
}

func TestUnmarkedKeepsTheReleasesWithoutMarksInOrder(t *testing.T) {
	check(t, func(c listCase) bool {
		var want []Release
		for _, r := range c.Releases {
			if unmarked(r) {
				want = append(want, r)
			}
		}
		return slices.Equal(Unmarked(c.Releases), want)
	})
}

func TestSortOrdersNewestFirstThenByVersionDescending(t *testing.T) {
	check(t, func(c listCase) bool {
		got := slices.Clone(c.Releases)
		Sort(got)
		// The same releases come out, marks and all, whatever their order.
		byVersion := func(a, b Release) int { return strings.Compare(a.Version, b.Version) }
		if !slices.Equal(slices.SortedFunc(slices.Values(got), byVersion), slices.SortedFunc(slices.Values(c.Releases), byVersion)) {
			return false
		}
		for i := 1; i < len(got); i++ {
			a, b := got[i-1], got[i]
			if !(a.Published.After(b.Published) || a.Published.Equal(b.Published) && a.Version > b.Version) {
				return false
			}
		}
		return true
	})
}

func TestLimitKeepsTheFirstNAndCountsTheRest(t *testing.T) {
	check(t, func(c listCase, pick uint8) bool {
		n := int(pick%10) - 2
		got, more := Limit(c.Releases, n)
		keep := len(c.Releases)
		if n > 0 {
			keep = min(n, keep)
		}
		return slices.Equal(got, c.Releases[:keep]) && more == len(c.Releases)-keep
	})
}

func TestLatestIsAReleasePublishedLast(t *testing.T) {
	check(t, func(c listCase) bool {
		got, ok := Latest(c.Releases)
		if len(c.Releases) == 0 {
			return !ok
		}
		return ok && slices.Contains(newest(c.Releases), got)
	})
}

func TestUnmarkedNoteNamesTheNewestWhenEveryReleaseIsMarked(t *testing.T) {
	check(t, func(c listCase) bool {
		got := UnmarkedNote(c.Releases, now)
		if len(c.Releases) == 0 || slices.ContainsFunc(c.Releases, unmarked) {
			return got == ""
		}
		for _, r := range newest(c.Releases) {
			if got == "every version left is marked; newest is "+Describe(r, now)+"; drop --unmarked to see them" {
				return true
			}
		}
		return false
	})
}

// Note is asked about a list the registry returned with something in it; an
// empty list is worded by NoVersions instead.
func TestNoteNamesTheNewestReleaseTheWindowDropped(t *testing.T) {
	check(t, func(c listCase, wc windowCase) bool {
		w, rs := wc.Window, c.Releases
		if len(rs) == 0 {
			return true
		}
		got := Note(rs, now, w)
		if !w.MinAgeSet && !w.SinceSet || slices.ContainsFunc(rs, func(r Release) bool { return inside(w, r) }) {
			return got == ""
		}
		for _, r := range newest(rs) {
			// The newest release is too new when --min-age dropped it, which a
			// caller can wait out; otherwise --since dropped it as too old.
			want := "no version published in the last SINCE; latest is " + Describe(r, now)
			if w.MinAgeSet && age(r) < w.MinAge {
				want = "no version is older than MIN; newest is " + Describe(r, now)
				if w.SinceSet {
					want = "no version published in the last SINCE is older than MIN; newest is " + Describe(r, now)
				}
			}
			if got == want {
				return true
			}
		}
		return false
	})
}

// What the note says about the list holds for the list.
func TestNoteDescribesTheListTruthfully(t *testing.T) {
	check(t, func(c listCase, wc windowCase) bool {
		w, rs := wc.Window, c.Releases
		if len(rs) == 0 {
			return true
		}
		none := func(match func(time.Duration) bool) bool {
			return !slices.ContainsFunc(rs, func(r Release) bool { return match(age(r)) })
		}
		got := Note(rs, now, w)
		switch {
		case strings.HasPrefix(got, "no version published in the last SINCE is older than MIN;"):
			return none(func(a time.Duration) bool { return a >= w.MinAge && a <= w.Since })
		case strings.HasPrefix(got, "no version is older than MIN;"):
			return none(func(a time.Duration) bool { return a >= w.MinAge })
		case strings.HasPrefix(got, "no version published in the last SINCE;"):
			return none(func(a time.Duration) bool { return a <= w.Since })
		}
		return got == ""
	})
}
