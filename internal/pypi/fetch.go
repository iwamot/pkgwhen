package pypi

import (
	"fmt"

	"github.com/iwamot/pkgwhen/internal/fetch"
	"github.com/iwamot/pkgwhen/internal/release"
)

// List fetches every version of name. found is false when PyPI has no such
// project.
func List(name string) (rs []release.Release, found bool, err error) {
	resp, err := fetch.Get(ProjectURL(name), nil)
	if err != nil {
		return nil, false, err
	}
	switch resp.Status {
	case 200:
		rs, err = DecodeProject(resp.Body)
		return rs, true, err
	case 404:
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("pypi: %s: HTTP %d", ProjectURL(name), resp.Status)
	}
}

// One fetches a single version. found is false when the project or the
// version does not exist.
func One(name, v string) (r release.Release, found bool, err error) {
	resp, err := fetch.Get(VersionURL(name, v), nil)
	if err != nil {
		return release.Release{}, false, err
	}
	switch resp.Status {
	case 200:
		r, err = DecodeVersion(resp.Body)
		return r, true, err
	case 404:
		return release.Release{}, false, nil
	default:
		return release.Release{}, false, fmt.Errorf("pypi: %s: HTTP %d", VersionURL(name, v), resp.Status)
	}
}

// Exists reports whether PyPI has the project at all. One asks for a single
// version, so its 404 does not say whether the project or only the version is
// missing; this answers that, and is only called on that path.
func Exists(name string) (bool, error) {
	resp, err := fetch.Get(ProjectURL(name), nil)
	if err != nil {
		return false, err
	}
	switch resp.Status {
	case 200:
		return true, nil
	case 404:
		return false, nil
	default:
		return false, fmt.Errorf("pypi: %s: HTTP %d", ProjectURL(name), resp.Status)
	}
}
