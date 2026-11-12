package pypi

import (
	"context"
	"net/http"
	"time"

	"github.com/iwamot/pkgwhen/internal/fetch"
	"github.com/iwamot/pkgwhen/internal/release"
)

// Archived reports whether the Simple Index confirms that the project is
// archived. The lookup is advisory and has its own short deadline.
func Archived(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := fetch.GetContext(ctx, indexURL(name), map[string]string{
		"Accept": "application/vnd.pypi.simple.v1+json",
	})
	return err == nil && resp.Status == http.StatusOK && isArchived(resp.Body)
}

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
		return nil, false, fetch.StatusError(resp, time.Now().UTC())
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
		return release.Release{}, false, fetch.StatusError(resp, time.Now().UTC())
	}
}
