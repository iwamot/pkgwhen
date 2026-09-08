// Package github reads GitHub Releases through the REST API.
//
// A release's date is published_at, which is what Renovate uses and which
// can trail created_at by the time it took to finish the draft. Drafts are
// visible only with a token and have no published_at, so they are dropped.
package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iwamot/pkgwhen/internal/fetch"
	"github.com/iwamot/pkgwhen/internal/release"
)

type doc struct {
	TagName     string `json:"tag_name"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
}

// PerPage is the largest page the releases endpoint serves.
const PerPage = 100

// ListURL is the first page of the releases endpoint.
func ListURL(repo string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=%d", repo, PerPage)
}

// TagURL is the endpoint for the release attached to one tag.
func TagURL(repo, tag string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, tag)
}

// AlternateTag is the other spelling of a version: with a leading "v" when
// it has none, without it when it has one. Release tags are usually v1.2.3
// while the argument is usually 1.2.3.
func AlternateTag(v string) string {
	if strings.HasPrefix(v, "v") {
		return v[1:]
	}
	return "v" + v
}

// DecodeList turns one page of releases into Release values, dropping drafts.
func DecodeList(data []byte) ([]release.Release, error) {
	var docs []doc
	if err := json.Unmarshal(data, &docs); err != nil {
		return nil, err
	}
	var out []release.Release
	for _, d := range docs {
		if d.Draft {
			continue
		}
		r, err := fromDoc(d)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// DecodeOne turns a single release document into a Release.
func DecodeOne(data []byte) (release.Release, error) {
	var d doc
	if err := json.Unmarshal(data, &d); err != nil {
		return release.Release{}, err
	}
	return fromDoc(d)
}

func fromDoc(d doc) (release.Release, error) {
	t, err := time.Parse(time.RFC3339, d.PublishedAt)
	if err != nil {
		return release.Release{}, fmt.Errorf("release %s: %w", d.TagName, err)
	}
	return release.Release{Version: d.TagName, Published: t, Prerelease: d.Prerelease}, nil
}

// StatusError describes a response that is neither the document nor a 404.
// A 429 is always a rate limit; a 403 is one only when a header says so,
// because GitHub answers 403 for a token that lacks access as well. That
// other 403 is what a scope, an unapproved SSO session, or an IP allow list
// looks like, and all three are answered at the token, so the message says
// to look there. A caller with no token sees 200 or 404 instead, so there
// is nothing to tell them apart from.
func StatusError(resp fetch.Response, haveToken bool, now time.Time) error {
	limited := resp.Status == http.StatusTooManyRequests ||
		(resp.Status == http.StatusForbidden && (resp.RetryAfter != "" || resp.RateLimitRemaining == "0"))
	if !limited {
		if resp.Status == http.StatusForbidden {
			return errors.New("HTTP 403; the token may lack access to this repository")
		}
		return fetch.StatusError(resp, now)
	}
	msg := fetch.RateLimited(resp, now)
	if !haveToken {
		msg += "; set GITHUB_TOKEN or run `gh auth login`"
	}
	return errors.New(msg)
}

// Headers builds the request headers, adding the token when there is one.
func Headers(token string) map[string]string {
	h := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
	if token != "" {
		h["Authorization"] = "Bearer " + token
	}
	return h
}
