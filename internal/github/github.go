// Package github reads GitHub Releases through the REST API.
//
// A release's date is published_at, which is what Renovate uses and which
// can trail created_at by the time it took to finish the draft. Drafts are
// visible only with a token and have no published_at, so they are dropped.
package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
		return nil, fmt.Errorf("github: %w", err)
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
		return release.Release{}, fmt.Errorf("github: %w", err)
	}
	return fromDoc(d)
}

func fromDoc(d doc) (release.Release, error) {
	t, err := time.Parse(time.RFC3339, d.PublishedAt)
	if err != nil {
		return release.Release{}, fmt.Errorf("github: release %s: %w", d.TagName, err)
	}
	return release.Release{Version: d.TagName, Published: t, Prerelease: d.Prerelease}, nil
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
