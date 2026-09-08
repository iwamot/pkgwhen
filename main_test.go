package main

import (
	"bytes"
	"os"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/iwamot/pkgwhen/internal/release"
	"github.com/iwamot/pkgwhen/internal/spec"
)

func TestParseArgs(t *testing.T) {
	pypi := spec.Spec{Registry: "pypi", Name: "openai-agents"}
	def := cliArgs{limit: defaultLimit}
	with := func(f func(*cliArgs)) cliArgs { a := def; f(&a); return a }
	day := 24 * time.Hour
	tests := []struct {
		name    string
		argv    []string
		want    cliArgs
		wantErr string
	}{
		{"spec only", []string{"pypi:openai-agents"}, with(func(a *cliArgs) { a.spec = pypi }), ""},
		{"version in spec", []string{"npm:@types/node@22.0.0"}, with(func(a *cliArgs) { a.spec = spec.Spec{Registry: "npm", Name: "@types/node", Version: "22.0.0"} }), ""},
		{"min-age", []string{"--min-age", "1d", "pypi:openai-agents"}, with(func(a *cliArgs) {
			a.spec = pypi
			a.window = release.Window{MinAge: day, MinAgeSet: true, MinAgeText: "1d"}
		}), ""},
		{"since after spec", []string{"pypi:openai-agents", "--since", "2w"}, with(func(a *cliArgs) {
			a.spec = pypi
			a.window = release.Window{Since: 14 * day, SinceSet: true, SinceText: "2w"}
		}), ""},
		{"both windows", []string{"--min-age", "1d", "--since", "30d", "pypi:openai-agents"}, with(func(a *cliArgs) {
			a.spec = pypi
			a.window = release.Window{MinAge: day, MinAgeSet: true, MinAgeText: "1d", Since: 30 * day, SinceSet: true, SinceText: "30d"}
		}), ""},
		{"equal windows", []string{"--min-age", "1d", "--since", "1d", "pypi:openai-agents"}, with(func(a *cliArgs) {
			a.spec = pypi
			a.window = release.Window{MinAge: day, MinAgeSet: true, MinAgeText: "1d", Since: day, SinceSet: true, SinceText: "1d"}
		}), ""},
		{"contradictory window", []string{"--min-age", "7d", "--since", "1d", "pypi:a"}, cliArgs{}, "--min-age 7d is longer than --since 1d; nothing can match"},
		{"min-age with a version", []string{"--min-age", "1d", "pypi:a@1.2.3"}, cliArgs{}, "--min-age does not apply with @VERSION; drop the flag or the @VERSION"},
		{"limit with a version", []string{"-n", "5", "pypi:a@1.2.3"}, cliArgs{}, "-n does not apply with @VERSION"},
		{"all with a version", []string{"--all", "pypi:a@1.2.3"}, cliArgs{}, "--all does not apply with @VERSION"},
		{"several dead options with a version", []string{"--all", "--since", "30d", "pypi:a@1.2.3"}, cliArgs{}, "--since and --all do not apply with @VERSION; drop them or the @VERSION"},
		{"dead options are refused before the window is compared", []string{"--min-age", "7d", "--since", "1d", "pypi:a@1.2.3"}, cliArgs{}, "--min-age and --since do not apply with @VERSION"},
		{"json still applies with a version", []string{"--json", "pypi:a@1.2.3"}, with(func(a *cliArgs) {
			a.spec = spec.Spec{Registry: "pypi", Name: "a", Version: "1.2.3"}
			a.asJSON = true
		}), ""},
		{"contradictory window across units", []string{"--min-age", "36h", "--since", "1d", "pypi:a"}, cliArgs{}, "--min-age 36h is longer than --since 1d"},
		{"contradictory window without a package", []string{"--min-age", "7d", "--since", "1d"}, cliArgs{}, "no package given"},
		{"contradictory window with help", []string{"--help", "--min-age", "7d", "--since", "1d"}, with(func(a *cliArgs) {
			a.showHelp = true
			a.window = release.Window{MinAge: 7 * day, MinAgeSet: true, MinAgeText: "7d", Since: day, SinceSet: true, SinceText: "1d"}
		}), ""},
		{"limit", []string{"-n", "5", "pypi:openai-agents"}, with(func(a *cliArgs) { a.spec = pypi; a.limit = 5 }), ""},
		{"all", []string{"--all", "pypi:openai-agents"}, with(func(a *cliArgs) { a.spec = pypi; a.limit = 0 }), ""},
		{"all then limit", []string{"--all", "-n", "3", "pypi:openai-agents"}, cliArgs{}, "-n and --all contradict each other; drop one"},
		{"limit then all", []string{"-n", "3", "--all", "pypi:openai-agents"}, cliArgs{}, "-n and --all contradict each other; drop one"},
		{"limit and all without a package", []string{"-n", "3", "--all"}, cliArgs{}, "no package given"},
		{"limit and all with help", []string{"--help", "-n", "3", "--all"}, with(func(a *cliArgs) {
			a.showHelp = true
			a.limit = 0
		}), ""},
		{"json", []string{"--json", "pypi:openai-agents"}, with(func(a *cliArgs) { a.spec = pypi; a.asJSON = true }), ""},
		{"help short", []string{"-h"}, with(func(a *cliArgs) { a.showHelp = true }), ""},
		{"help long", []string{"--help"}, with(func(a *cliArgs) { a.showHelp = true }), ""},
		{"version short", []string{"-v"}, with(func(a *cliArgs) { a.showVersion = true }), ""},
		{"version long", []string{"--version"}, with(func(a *cliArgs) { a.showVersion = true }), ""},
		{"instructions", []string{"--instructions"}, with(func(a *cliArgs) { a.showInstructions = true }), ""},
		{"no spec", nil, cliArgs{}, "no package given"},
		{"bad spec", []string{"openai-agents"}, cliArgs{}, "want REGISTRY:NAME"},
		{"two specs", []string{"pypi:a", "pypi:b"}, cliArgs{}, "one package per call"},
		{"unknown flag", []string{"--bogus", "pypi:a"}, cliArgs{}, "unknown flag \"--bogus\"; run `pkgwhen --help` for the options"},
		{"limit missing value", []string{"pypi:a", "-n"}, cliArgs{}, "-n needs a value"},
		{"limit zero", []string{"-n", "0", "pypi:a"}, cliArgs{}, "positive integer"},
		{"limit not a number", []string{"-n", "x", "pypi:a"}, cliArgs{}, "positive integer"},
		{"min-age missing value", []string{"pypi:a", "--min-age"}, cliArgs{}, "--min-age needs a value"},
		{"min-age bad unit", []string{"--min-age", "1m", "pypi:a"}, cliArgs{}, "--min-age: \"1m\""},
		{"since missing value", []string{"--since"}, cliArgs{}, "--since needs a value"},
		{"since bad", []string{"--since", "soon", "pypi:a"}, cliArgs{}, "--since: \"soon\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.argv)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseArgs err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArgs err = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseArgs = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNotFound(t *testing.T) {
	pypiSpec := spec.Spec{Registry: "pypi", Name: "foo", Version: "1.2.3"}
	npmSpec := spec.Spec{Registry: "npm", Name: "@types/node"}
	ghSpec := spec.Spec{Registry: "github-releases", Name: "iwamot/pkgwhen"}
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	others := []release.Release{
		{Version: "1.2.0", Published: now.Add(-90 * 24 * time.Hour)},
		{Version: "1.3.2", Published: now.Add(-45 * 24 * time.Hour), Yanked: true},
	}
	tests := []struct {
		name      string
		s         spec.Spec
		kind      lookup
		others    []release.Release
		haveToken bool
		want      string
	}{
		{"version", pypiSpec, lookupNoVersion, others, false, "pypi:foo@1.2.3: version not found; latest is 1.3.2 (yanked), published 45d ago; run `pkgwhen pypi:foo` to see the versions that exist"},
		{"version, with nothing to name", pypiSpec, lookupNoVersion, nil, false, "pypi:foo@1.2.3: version not found; run `pkgwhen pypi:foo` to see the versions that exist"},
		{"package on pypi", spec.Spec{Registry: "pypi", Name: "foo"}, lookupNoPackage, nil, false, "pypi:foo: package not found on pypi (or not visible yet); check the name"},
		{"package on npm", npmSpec, lookupNoPackage, nil, false, "npm:@types/node: package not found on npm (or not visible yet); check the name"},
		{"repository without a token", ghSpec, lookupNoPackage, nil, false, "github-releases:iwamot/pkgwhen: repository not found on github (or private; set GITHUB_TOKEN or run `gh auth login`)"},
		{"repository with a token", ghSpec, lookupNoPackage, nil, true, "github-releases:iwamot/pkgwhen: repository not found on github (or private, and the token cannot see it)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notFound(tt.s, tt.kind, tt.others, now, tt.haveToken); got != tt.want {
				t.Errorf("notFound = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMissing(t *testing.T) {
	if got := missing(true); got != lookupNoVersion {
		t.Errorf("missing(true) = %v, want lookupNoVersion", got)
	}
	if got := missing(false); got != lookupNoPackage {
		t.Errorf("missing(false) = %v, want lookupNoPackage", got)
	}
}

func TestDeadOptionsError(t *testing.T) {
	tests := []struct {
		names []string
		want  string
	}{
		{[]string{"--min-age"}, "--min-age does not apply with @VERSION; drop the flag or the @VERSION"},
		{[]string{"--min-age", "-n"}, "--min-age and -n do not apply with @VERSION; drop them or the @VERSION"},
		{[]string{"--min-age", "--since", "-n"}, "--min-age, --since and -n do not apply with @VERSION; drop them or the @VERSION"},
	}
	for _, tt := range tests {
		if got := deadOptionsError(tt.names).Error(); got != tt.want {
			t.Errorf("deadOptionsError(%v) = %q, want %q", tt.names, got, tt.want)
		}
	}
}

func TestNotes(t *testing.T) {
	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	rs := []release.Release{
		{Version: "1.1.0", Published: now.Add(-2 * time.Hour)},
		{Version: "1.0.0", Published: now.Add(-72 * time.Hour)},
	}
	day := 24 * time.Hour
	tests := []struct {
		name     string
		registry string
		all      []release.Release
		cut      int
		window   release.Window
		want     []string
	}{
		{"a full table says nothing", "pypi", rs, 0, release.Window{}, nil},
		{"a cut table says how much was cut", "pypi", rs, 118, release.Window{}, []string{"118 more versions; pass -n N or --all to see them"}},
		{"nothing to list at all", "github-releases", nil, 0, release.Window{}, []string{"no releases published (the repository may have tags but no releases)"}},
		{"nothing to list, on the other registries", "npm", nil, 0, release.Window{}, []string{"no versions with a publish date"}},
		{"an empty list is not blamed on the window", "pypi", nil, 0, release.Window{Since: day, SinceSet: true, SinceText: "1d"}, []string{"no versions with a publish date"}},
		{"a window that dropped everything", "pypi", rs, 0, release.Window{MinAge: 7 * day, MinAgeSet: true, MinAgeText: "7d"}, []string{"no version is older than 7d; newest is 1.1.0, published 2h ago"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := notes(tt.registry, tt.all, tt.cut, now, tt.window); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("notes = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name     string
		injected string
		info     *debug.BuildInfo
		want     string
	}{
		{"injected non-dev wins over build info", "1.2.3", &debug.BuildInfo{Main: debug.Module{Version: "9.9.9"}}, "1.2.3"},
		{"injected non-dev wins with no build info", "1.2.3", nil, "1.2.3"},
		{"dev falls back to build info Main.Version", devVersion, &debug.BuildInfo{Main: debug.Module{Version: "v0.0.3"}}, "v0.0.3"},
		{"dev with (devel) build info falls through to dev", devVersion, &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, devVersion},
		{"dev with empty build info version falls through to dev", devVersion, &debug.BuildInfo{}, devVersion},
		{"dev with nil build info falls through to dev", devVersion, nil, devVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.injected, tt.info); got != tt.want {
				t.Errorf("resolveVersion = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunOffline(t *testing.T) {
	tests := []struct {
		name       string
		argv       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"help", []string{"--help"}, exitOK, "pkgwhen — list", ""},
		{"version", []string{"--version"}, exitOK, devVersion, ""},
		{"instructions", []string{"--instructions"}, exitOK, "use `pkgwhen` instead of curl", ""},
		{"usage error", []string{"cargo:serde"}, exitUsage, "", "pkgwhen: unknown registry"},
		{"contradictory window", []string{"--min-age", "7d", "--since", "1d", "pypi:x"}, exitUsage, "", "nothing can match"},
		{"n with all", []string{"-n", "2", "--all", "pypi:x"}, exitUsage, "", "contradict each other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.argv, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want containing %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want containing %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

// The paragraph lives in three places: here, README.md, and whichever
// instruction file a user pasted it into. The copy outside the repo is a
// release-time chore, but the one in README.md is checkable.
func TestREADMEQuotesInstructions(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), instructionsText) {
		t.Error("README.md does not quote instructionsText verbatim")
	}
}

func TestHelpMatchesInstructions(t *testing.T) {
	// The README quotes both; keep the flags named in the paragraph real.
	for _, flag := range []string{"--min-age", "--since", "@VERSION"} {
		if !strings.Contains(helpText, flag) || !strings.Contains(instructionsText, flag) {
			t.Errorf("%s missing from help or instructions", flag)
		}
	}
}
