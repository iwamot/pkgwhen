// Package npm reads the npm registry's packument.
//
// The full packument is fetched, not the abbreviated one, because only the
// full one carries the time map that holds each version's publish date.
// Versions are the keys of "versions"; the time map also holds "created",
// "modified", and unpublished versions, which are not versions to list.
package npm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/iwamot/pkgwhen/internal/release"
)

type packument struct {
	Versions map[string]struct {
		// A string with the notice when deprecated. npm writes "" to
		// undeprecate, and some old documents carry a boolean.
		Deprecated json.RawMessage `json:"deprecated"`
	} `json:"versions"`
	Time map[string]string `json:"time"`
}

// URL is the packument endpoint. A scoped name keeps its "@" and has its
// "/" encoded, which is the form the registry documents.
func URL(name string) string {
	return "https://registry.npmjs.org/" + strings.ReplaceAll(name, "/", "%2F")
}

// Decode turns the packument into releases. A version missing from the
// time map has no known publish date and is left out.
func Decode(data []byte) ([]release.Release, error) {
	var p packument
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	var out []release.Release
	for v, info := range p.Versions {
		stamp, ok := p.Time[v]
		if !ok {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			return nil, fmt.Errorf("version %s: %w", v, err)
		}
		out = append(out, release.Release{
			Version:    v,
			Published:  t,
			Deprecated: isDeprecated(info.Deprecated),
			Prerelease: IsPrerelease(v),
		})
	}
	return out, nil
}

func isDeprecated(raw json.RawMessage) bool {
	switch s := string(raw); s {
	case "", `""`, "false", "null":
		return false
	default:
		return true
	}
}

// IsPrerelease reports whether v carries a semver prerelease part. Build
// metadata after "+" is not one, and may itself contain "-".
func IsPrerelease(v string) bool {
	core, _, _ := strings.Cut(v, "+")
	return strings.Contains(core, "-")
}
