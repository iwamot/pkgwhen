package main

import (
	"bytes"
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
		{"min-age", []string{"--min-age", "1d", "pypi:openai-agents"}, with(func(a *cliArgs) { a.spec = pypi; a.window = release.Window{MinAge: day, MinAgeSet: true} }), ""},
		{"since after spec", []string{"pypi:openai-agents", "--since", "2w"}, with(func(a *cliArgs) { a.spec = pypi; a.window = release.Window{Since: 14 * day, SinceSet: true} }), ""},
		{"both windows", []string{"--min-age", "1d", "--since", "30d", "pypi:openai-agents"}, with(func(a *cliArgs) {
			a.spec = pypi
			a.window = release.Window{MinAge: day, MinAgeSet: true, Since: 30 * day, SinceSet: true}
		}), ""},
		{"limit", []string{"-n", "5", "pypi:openai-agents"}, with(func(a *cliArgs) { a.spec = pypi; a.limit = 5 }), ""},
		{"all", []string{"--all", "pypi:openai-agents"}, with(func(a *cliArgs) { a.spec = pypi; a.limit = 0 }), ""},
		{"all then limit", []string{"--all", "-n", "3", "pypi:openai-agents"}, with(func(a *cliArgs) { a.spec = pypi; a.limit = 3 }), ""},
		{"json", []string{"--json", "pypi:openai-agents"}, with(func(a *cliArgs) { a.spec = pypi; a.asJSON = true }), ""},
		{"help short", []string{"-h"}, with(func(a *cliArgs) { a.showHelp = true }), ""},
		{"help long", []string{"--help"}, with(func(a *cliArgs) { a.showHelp = true }), ""},
		{"version short", []string{"-v"}, with(func(a *cliArgs) { a.showVersion = true }), ""},
		{"version long", []string{"--version"}, with(func(a *cliArgs) { a.showVersion = true }), ""},
		{"instructions", []string{"--instructions"}, with(func(a *cliArgs) { a.showInstructions = true }), ""},
		{"no spec", nil, cliArgs{}, "no package given"},
		{"bad spec", []string{"openai-agents"}, cliArgs{}, "want REGISTRY:NAME"},
		{"two specs", []string{"pypi:a", "pypi:b"}, cliArgs{}, "one package per call"},
		{"unknown flag", []string{"--bogus", "pypi:a"}, cliArgs{}, "unknown flag"},
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
		{"usage error", []string{"cargo:serde"}, exitError, "", "pkgwhen: unknown registry"},
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

func TestHelpMatchesInstructions(t *testing.T) {
	// The README quotes both; keep the flags named in the paragraph real.
	for _, flag := range []string{"--min-age", "--since", "@VERSION"} {
		if !strings.Contains(helpText, flag) || !strings.Contains(instructionsText, flag) {
			t.Errorf("%s missing from help or instructions", flag)
		}
	}
}
