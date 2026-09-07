# pkgwhen

[![release](https://img.shields.io/github/v/release/iwamot/pkgwhen)](https://github.com/iwamot/pkgwhen/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/iwamot/pkgwhen)](https://pkg.go.dev/github.com/iwamot/pkgwhen)

List a package's versions with the date each was published. Built for coding agents that keep dependencies current from the shell.

```
$ pkgwhen pypi:openai-agents
VERSION  PUBLISHED   AGE
0.22.0   2026-08-19  18d
0.21.1   2026-08-16  20d
0.21.0   2026-08-15  22d
...
0.0.1    2025-03-04  550d  yanked
```

One argument names the registry and the package. PyPI, npm, and GitHub Releases come back in the same table, newest first, with the age of each version and a mark on the ones dependency updaters usually skip.

## Why

An agent that keeps dependencies current keeps asking the same question: when did this version come out? A release-age rule (Renovate's `minimumReleaseAge`, uv's `exclude-newer`, pnpm's `minimumReleaseAge`) holds a version back until it is a day old, so "why is there no PR yet" and "is this pin the latest" both come down to a publish date.

The registries answer it, but not in one shape. npm has `npm view PKG time --json`, which returns every version in one dictionary. PyPI has a JSON API and no command. GitHub has `gh release list`. So the agent fetches the JSON with curl and writes five to ten lines of Python to pull out the dates. In one month of the author's Claude Code sessions, that happened 132 times.

`pkgwhen` is those lines, made into a command. For GitHub it is not a replacement for `gh release list`; it is the same answer in the same table as the other two.

## Setup

Install it where the agent runs:

```bash
brew install iwamot/tap/pkgwhen
```

Or with Go:

```bash
go install github.com/iwamot/pkgwhen@latest
```

Or download a prebuilt binary from the [Releases page](https://github.com/iwamot/pkgwhen/releases).

Then tell the agent to use it, in `CLAUDE.md`, `AGENTS.md`, or whichever file your agent reads:

```markdown
To find which versions of a package exist and when each was published, use `pkgwhen` instead of curl and an ad-hoc script: `pkgwhen pypi:NAME`, `pkgwhen npm:NAME`, or `pkgwhen github-releases:OWNER/REPO`. Add `@VERSION` to print one version, `--min-age 1d` to list only versions old enough to pass a one-day release age, and `--since 30d` for versions published in the last 30 days. Rows marked `yanked`, `deprecated`, or `pre` are versions that dependency updaters usually skip, so a newer version with a mark is not a reason to expect a PR. Exit 1 means the package or version does not exist (yet); rerun while it exits 1, and stop and read stderr on any other exit code.
```

That paragraph is all the agent needs. `pkgwhen --instructions` prints the same paragraph, for setup scripts and machines where this page is not at hand.

## What the agent sees

Checking whether a version has passed a one-day release age. Only versions published more than a day ago are listed, so the newest one being absent is the answer:

```
$ pkgwhen --min-age 1d -n 3 npm:@types/node
VERSION  PUBLISHED   AGE
26.4.1   2026-09-01  4d
26.4.0   2026-08-27  10d
26.3.0   2026-08-24  12d
... and 2356 more (pass -n N or --all)
```

One version, with the time of day. The tag on GitHub is `v2.2.12`, and the plain version finds it too:

```
$ pkgwhen github-releases:jdx/aube@2.2.12
v2.2.12  2026-09-06T00:44:39Z  13h
```

A version that is not there yet. Nothing is printed on stdout, and the exit code is 1, so a loop can wait for a package that was just published. The `|| [ $? -ne 1 ]` ends the loop on any other exit code, so a rate limit or an unreachable registry stops it instead of retrying forever:

```
$ pkgwhen npm:welt-io-x@1.2.3
pkgwhen: npm:welt-io-x@1.2.3: not found
$ until pkgwhen npm:welt-io-x@1.2.3 || [ $? -ne 1 ]; do sleep 30; done
```

The same list as JSON, for a script that compares rather than reads:

```
$ pkgwhen --json -n 1 pypi:openai-agents
{
  "registry": "pypi",
  "name": "openai-agents",
  "versions": [
    {
      "version": "0.22.0",
      "published": "2026-08-19T13:45:19Z",
      "age_seconds": 1557272,
      "yanked": false,
      "deprecated": false,
      "prerelease": false
    }
  ],
  "more": 118
}
```

## Reference

```
$ pkgwhen --help
pkgwhen — list a package's versions with the date each was published.

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
```

- Versions are ordered by publish date, not by version number, so a patch to an older line appears where it was published. To compare two versions, ask for each with `@VERSION`.
- Ages are measured from the current UTC time. The table shows whole days, hours below a day, and minutes below an hour; `--min-age` and `--since` are compared to the second.
- npm: the full packument is fetched, because only it carries publish dates. For a large package that is a few megabytes, compressed in transit.
- PyPI: a version's date is the earliest upload among its files, which is the moment uv's `exclude-newer` treats it as available. A version is marked `yanked` when any of its files is, which is how Renovate reads it. Versions with no files are left out.
- GitHub: the date is `published_at`, which is what Renovate uses and which can trail the draft's creation by as long as the draft took to finish. Draft releases are dropped. `GITHUB_TOKEN`, `GH_TOKEN`, or `gh auth token` is used when available; without one, the API allows 60 requests an hour, and hitting that limit is reported as such rather than as a bare 403. Releases are read in the order GitHub returns them, newest created first, and only as many pages as the requested count needs unless a date option or `--all` is given.
- Nothing is cached and nothing is written. Every call asks the registry.

## Out of scope

- Sorting by version number. The question here is when, and the registries already order by version.
- Other registries. Go, Cargo, and the rest can be added when a use for them turns up; the shape is one small package per registry.
- Aggregators such as deps.dev. They lag the registries by minutes and miss yanks, which is exactly where a publish date matters.
- Waiting. Exit 1 and a shell loop cover it without a flag.
- Installing anything. `pkgwhen` only reads.

## License

MIT
