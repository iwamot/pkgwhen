// Package spec parses the REGISTRY:NAME[@VERSION] argument.
package spec

import (
	"fmt"
	"strings"
)

// Registries lists the accepted registry names, in help order.
var Registries = []string{"pypi", "npm", "github-releases"}

// Spec is one parsed argument. Version is empty when no @VERSION was given.
type Spec struct {
	Registry string
	Name     string
	Version  string
}

// String renders the spec back in argument form, for messages.
func (s Spec) String() string {
	if s.Version == "" {
		return s.Registry + ":" + s.Name
	}
	return s.Registry + ":" + s.Name + "@" + s.Version
}

// Parse splits REGISTRY:NAME[@VERSION]. The version is cut at the last "@"
// that is not the first character of NAME, so npm scoped packages such as
// @types/node@22.0.0 keep their leading "@". GitHub names must be OWNER/REPO.
func Parse(arg string) (Spec, error) {
	registry, rest, ok := strings.Cut(arg, ":")
	if !ok || registry == "" {
		return Spec{}, fmt.Errorf("%q: want REGISTRY:NAME[@VERSION], where REGISTRY is %s", arg, strings.Join(Registries, ", "))
	}
	known := false
	for _, r := range Registries {
		if r == registry {
			known = true
		}
	}
	if !known {
		return Spec{}, fmt.Errorf("unknown registry %q: want %s", registry, strings.Join(Registries, ", "))
	}
	s := Spec{Registry: registry, Name: rest}
	if at := strings.LastIndex(rest, "@"); at > 0 {
		s.Name, s.Version = rest[:at], rest[at+1:]
		if s.Version == "" {
			return Spec{}, fmt.Errorf("%q: nothing after @", arg)
		}
	}
	if s.Name == "" {
		return Spec{}, fmt.Errorf("%q: no package name after %s:", arg, registry)
	}
	if registry == "github-releases" {
		owner, repo, ok := strings.Cut(s.Name, "/")
		if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
			return Spec{}, fmt.Errorf("%q: github-releases wants OWNER/REPO", arg)
		}
	}
	return s, nil
}
