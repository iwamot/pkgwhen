// Command pkgwhen lists a package's versions with the date each was
// published, for PyPI, npm, and GitHub Releases.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/iwamot/pkgwhen/internal/github"
	"github.com/iwamot/pkgwhen/internal/npm"
	"github.com/iwamot/pkgwhen/internal/period"
	"github.com/iwamot/pkgwhen/internal/pypi"
	"github.com/iwamot/pkgwhen/internal/release"
	"github.com/iwamot/pkgwhen/internal/spec"
)

const (
	exitOK       = 0
	exitNotFound = 1
	exitError    = 2
)

const devVersion = "0.0.0-dev"

var version = devVersion

const defaultLimit = 20

const helpText = `pkgwhen — list a package's versions with the date each was published.

Usage:
  pkgwhen [options] REGISTRY:NAME[@VERSION]

  pkgwhen pypi:openai-agents
  pkgwhen npm:@types/node@22.0.0
  pkgwhen github-releases:jdx/aube --since 30d

REGISTRY is pypi, npm, or github-releases. NAME is the package name, or
OWNER/REPO for GitHub. Versions are printed newest first by publish date,
at most 20 unless -n or --all is given. With @VERSION, only that version
is printed, with the time of day.

Options:
  --min-age DUR   only versions published more than DUR ago (1d, 36h, 2w)
  --since DUR     only versions published within the last DUR
                  (--min-age keeps the older side, --since the newer side)
  -n N            print at most N versions (default 20)
  --all           print every version
  --json          print JSON instead of the table, with ISO 8601 timestamps
  -h, --help      show this help
  -v, --version   show the version
  --instructions  print the paragraph for the agent's instruction file

Marks at the end of a row:
  yanked      the version was yanked (PyPI)
  deprecated  the version is deprecated (npm)
  pre         prerelease

Exit codes:
  0  printed
  1  the package, or the version given with @VERSION, does not exist
  2  usage error, or the registry could not be reached
`

// instructionsText is the paragraph a coding agent needs in order to use
// pkgwhen: when to reach for it, the argument forms, what the marks mean,
// and what exit 1 means. README.md quotes it verbatim.
const instructionsText = "To find which versions of a package exist and when each was published, use `pkgwhen` instead of curl and an ad-hoc script: `pkgwhen pypi:NAME`, `pkgwhen npm:NAME`, or `pkgwhen github-releases:OWNER/REPO`. Add `@VERSION` to print one version, `--min-age 1d` to list only versions old enough to pass a one-day release age, and `--since 30d` for versions published in the last 30 days. Rows marked `yanked`, `deprecated`, or `pre` are versions that dependency updaters usually skip, so a newer version with a mark is not a reason to expect a PR. Exit 1 means the version does not exist yet; to wait for a version you just published, rerun in a loop.\n"

type cliArgs struct {
	showHelp         bool
	showVersion      bool
	showInstructions bool
	window           release.Window
	limit            int
	asJSON           bool
	spec             spec.Spec
}

func parseArgs(argv []string) (cliArgs, error) {
	a := cliArgs{limit: defaultLimit}
	haveSpec := false
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		switch arg {
		case "-h", "--help":
			a.showHelp = true
		case "-v", "--version":
			a.showVersion = true
		case "--instructions":
			a.showInstructions = true
		case "--all":
			a.limit = 0
		case "--json":
			a.asJSON = true
		case "--min-age", "--since", "-n":
			if i+1 >= len(argv) {
				return cliArgs{}, fmt.Errorf("%s needs a value", arg)
			}
			i++
			value := argv[i]
			switch arg {
			case "-n":
				n, err := strconv.Atoi(value)
				if err != nil || n < 1 {
					return cliArgs{}, fmt.Errorf("-n needs a positive integer, got %q", value)
				}
				a.limit = n
			case "--min-age":
				d, err := period.Parse(value)
				if err != nil {
					return cliArgs{}, fmt.Errorf("--min-age: %w", err)
				}
				a.window.MinAge, a.window.MinAgeSet = d, true
			default:
				d, err := period.Parse(value)
				if err != nil {
					return cliArgs{}, fmt.Errorf("--since: %w", err)
				}
				a.window.Since, a.window.SinceSet = d, true
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return cliArgs{}, fmt.Errorf("unknown flag: %s", arg)
			}
			if haveSpec {
				return cliArgs{}, fmt.Errorf("one package per call, got %q and %q", a.spec, arg)
			}
			s, err := spec.Parse(arg)
			if err != nil {
				return cliArgs{}, err
			}
			a.spec, haveSpec = s, true
		}
	}
	if !haveSpec && !a.showHelp && !a.showVersion && !a.showInstructions {
		return cliArgs{}, fmt.Errorf("no package given; want REGISTRY:NAME[@VERSION]")
	}
	return a, nil
}

// resolveVersion picks the most authoritative version string available.
//
// Priority:
//  1. injected (set via `-ldflags '-X main.version=...'` during a GoReleaser
//     build) when it differs from devVersion.
//  2. info.Main.Version when present and not "(devel)" or "" — this is what
//     `go install module@vX.Y.Z` records, even though ldflags don't apply.
//  3. injected (devVersion) as the final fallback.
func resolveVersion(injected string, info *debug.BuildInfo) string {
	if injected != devVersion {
		return injected
	}
	if info != nil {
		v := info.Main.Version
		if v != "" && v != "(devel)" {
			return v
		}
	}
	return injected
}

// list fetches every version the registry knows for s. found is false when
// the package itself does not exist.
func list(s spec.Spec, want int) ([]release.Release, bool, error) {
	switch s.Registry {
	case "pypi":
		return pypi.List(s.Name)
	case "npm":
		return npm.List(s.Name)
	default:
		return github.List(s.Name, github.Token(), want)
	}
}

// one fetches the version named in s. found is false when the package or
// the version does not exist.
func one(s spec.Spec) (release.Release, bool, error) {
	switch s.Registry {
	case "pypi":
		return pypi.One(s.Name, s.Version)
	case "npm":
		rs, found, err := npm.List(s.Name)
		if err != nil || !found {
			return release.Release{}, found, err
		}
		for _, r := range rs {
			if r.Version == s.Version {
				return r, true, nil
			}
		}
		return release.Release{}, false, nil
	default:
		return github.One(s.Name, s.Version, github.Token())
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(argv []string, stdout, stderr io.Writer) int {
	a, err := parseArgs(argv)
	if err != nil {
		fmt.Fprintln(stderr, "pkgwhen:", err)
		return exitError
	}
	if a.showHelp {
		fmt.Fprint(stdout, helpText)
		return exitOK
	}
	if a.showVersion {
		info, _ := debug.ReadBuildInfo()
		fmt.Fprintln(stdout, resolveVersion(version, info))
		return exitOK
	}
	if a.showInstructions {
		fmt.Fprint(stdout, instructionsText)
		return exitOK
	}

	now := time.Now().UTC()
	if a.spec.Version != "" {
		r, found, err := one(a.spec)
		if err != nil {
			fmt.Fprintln(stderr, "pkgwhen:", err)
			return exitError
		}
		if !found {
			fmt.Fprintf(stderr, "pkgwhen: %s: not found\n", a.spec)
			return exitNotFound
		}
		if a.asJSON {
			fmt.Fprint(stdout, release.JSON(a.spec.Registry, a.spec.Name, []release.Release{r}, 0, now))
		} else {
			fmt.Fprint(stdout, release.Line(r, now))
		}
		return exitOK
	}

	// A window can drop any number of the newest releases, so GitHub, the
	// one registry read page by page, is read to the end when one is set.
	want := a.limit
	if a.window.MinAgeSet || a.window.SinceSet {
		want = 0
	}
	rs, found, err := list(a.spec, want)
	if err != nil {
		fmt.Fprintln(stderr, "pkgwhen:", err)
		return exitError
	}
	if !found {
		fmt.Fprintf(stderr, "pkgwhen: %s: not found\n", a.spec)
		return exitNotFound
	}
	rs = release.Filter(rs, now, a.window)
	release.Sort(rs)
	rs, more := release.Limit(rs, a.limit)
	if a.asJSON {
		fmt.Fprint(stdout, release.JSON(a.spec.Registry, a.spec.Name, rs, more, now))
		return exitOK
	}
	fmt.Fprint(stdout, release.Table(rs, now))
	if more > 0 {
		fmt.Fprint(stdout, release.More(more))
	}
	return exitOK
}
