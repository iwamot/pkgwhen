package github

import (
	"os"
	"os/exec"
	"strings"

	"github.com/iwamot/pkgwhen/internal/fetch"
	"github.com/iwamot/pkgwhen/internal/release"
)

// Token returns a GitHub token from GITHUB_TOKEN or GH_TOKEN, else from
// `gh auth token`, else "". Without one the API allows 60 requests an hour
// and hides draft releases, which is enough for a public repository.
func Token() string {
	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// List fetches releases of repo, newest created first as GitHub orders them,
// following pages until at least want releases are in hand or the pages run
// out. want <= 0 reads every page. found is false when the repository does
// not exist, or is private and the token cannot see it.
func List(repo, token string, want int) (rs []release.Release, found bool, err error) {
	url := ListURL(repo)
	for url != "" {
		resp, err := fetch.Get(url, Headers(token))
		if err != nil {
			return nil, false, err
		}
		switch resp.Status {
		case 200:
		case 404:
			return nil, false, nil
		default:
			return nil, false, StatusError(url, resp.Status, resp.RateLimitRemaining)
		}
		page, err := DecodeList(resp.Body)
		if err != nil {
			return nil, false, err
		}
		rs = append(rs, page...)
		if want > 0 && len(rs) >= want {
			break
		}
		url = resp.Next
	}
	return rs, true, nil
}

// One fetches the release attached to tag, trying the other spelling of
// the version ("v1.2.3" for "1.2.3", and the reverse) when the first is not
// found. found is false when neither exists.
func One(repo, tag, token string) (r release.Release, found bool, err error) {
	for _, candidate := range []string{tag, AlternateTag(tag)} {
		url := TagURL(repo, candidate)
		resp, err := fetch.Get(url, Headers(token))
		if err != nil {
			return release.Release{}, false, err
		}
		switch resp.Status {
		case 200:
			r, err = DecodeOne(resp.Body)
			return r, true, err
		case 404:
			continue
		default:
			return release.Release{}, false, StatusError(url, resp.Status, resp.RateLimitRemaining)
		}
	}
	return release.Release{}, false, nil
}

// Exists reports whether the repository can be read with this token. GitHub
// answers 404 for a private repository as well as a missing one, so a false
// here means "not visible", which is what the message has to say.
func Exists(repo, token string) (bool, error) {
	url := ListURL(repo)
	resp, err := fetch.Get(url, Headers(token))
	if err != nil {
		return false, err
	}
	switch resp.Status {
	case 200:
		return true, nil
	case 404:
		return false, nil
	default:
		return false, StatusError(url, resp.Status, resp.RateLimitRemaining)
	}
}
