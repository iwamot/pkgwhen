// Package pypi reads pypi.org's JSON API.
//
// A version's publish date is the earliest upload among its files, which is
// what uv's exclude-newer effectively uses, since it filters file by file.
// A version is yanked when any of its files is, which is how Renovate reads
// it.
package pypi

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/iwamot/pkgwhen/internal/release"
)

type file struct {
	UploadTime string `json:"upload_time_iso_8601"`
	Yanked     bool   `json:"yanked"`
}

type project struct {
	Releases map[string][]file `json:"releases"`
}

type version struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	URLs []file `json:"urls"`
}

// ProjectURL is the endpoint that lists every version with its files.
func ProjectURL(name string) string {
	return "https://pypi.org/pypi/" + name + "/json"
}

// VersionURL is the endpoint for one version. PyPI normalizes the version
// on its side, so 2.32 and 2.32.0 both resolve.
func VersionURL(name, v string) string {
	return "https://pypi.org/pypi/" + name + "/" + v + "/json"
}

// DecodeProject turns the project document into releases. Versions with no
// files have no upload date and are left out.
func DecodeProject(data []byte) ([]release.Release, error) {
	var p project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("pypi: %w", err)
	}
	var out []release.Release
	for v, files := range p.Releases {
		r, ok, err := fromFiles(v, files)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, r)
		}
	}
	return out, nil
}

// DecodeVersion turns the one-version document into a release.
func DecodeVersion(data []byte) (release.Release, error) {
	var d version
	if err := json.Unmarshal(data, &d); err != nil {
		return release.Release{}, fmt.Errorf("pypi: %w", err)
	}
	r, ok, err := fromFiles(d.Info.Version, d.URLs)
	if err != nil {
		return release.Release{}, err
	}
	if !ok {
		return release.Release{}, fmt.Errorf("pypi: version %s has no files, so no upload date", d.Info.Version)
	}
	return r, nil
}

func fromFiles(v string, files []file) (release.Release, bool, error) {
	if len(files) == 0 {
		return release.Release{}, false, nil
	}
	r := release.Release{Version: v, Prerelease: IsPrerelease(v)}
	for _, f := range files {
		t, err := time.Parse(time.RFC3339Nano, f.UploadTime)
		if err != nil {
			return release.Release{}, false, fmt.Errorf("pypi: version %s: %w", v, err)
		}
		if r.Published.IsZero() || t.Before(r.Published) {
			r.Published = t
		}
		if f.Yanked {
			r.Yanked = true
		}
	}
	return r, true, nil
}

// PEP 440: release segment, then optional pre (a/b/c/rc/alpha/beta/pre/
// preview), post, and dev segments, each with an optional separator.
var pep440 = regexp.MustCompile(`(?i)^v?(?:\d+!)?\d+(?:\.\d+)*(?P<pre>[-._]?(?:a|b|c|rc|alpha|beta|pre|preview)[-._]?\d*)?(?:[-._]?(?:post|rev|r)[-._]?\d*)?(?P<dev>[-._]?dev[-._]?\d*)?$`)

// IsPrerelease reports whether v has a PEP 440 pre or dev segment. A version
// that does not parse as PEP 440 at all is reported as a final release.
func IsPrerelease(v string) bool {
	m := pep440.FindStringSubmatch(v)
	if m == nil {
		return false
	}
	return m[pep440.SubexpIndex("pre")] != "" || m[pep440.SubexpIndex("dev")] != ""
}
