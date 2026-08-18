# 0001. Commit the built binary

## Context and problem statement

Quill's thinking is Go: version arithmetic, the branch guard, tagging, and cleanup.
A GitHub Action has no native way to run a Go program.
Something has to get a binary onto the runner before any of that logic can execute, and that something runs on every release in every repository that uses quill.

The runner might be Linux (a GoReleaser release) or macOS (an iOS build going to Fledge), on either `amd64` or `arm64`.

## Considered options

- Commit prebuilt binaries to `dist/`, the way JavaScript actions commit their bundled output
- Build from source at runtime with `actions/setup-go` and `go build`
- Download the binary from quill's own GitHub releases
- Write the action in bash instead

## Decision outcome

**Commit prebuilt binaries** for `linux/amd64`, `linux/arm64`, `darwin/amd64` and `darwin/arm64`, with a shell shim that picks the right one from `uname`.

Building at runtime costs roughly twenty seconds per run and forces a Go toolchain onto a macOS runner whose job is Xcode.
Downloading from quill's own releases is circular: quill cuts quill's releases, so the first release would have nothing to download, and every later run gains a network dependency that can fail at exactly the wrong moment.
Bash was rejected because the version arithmetic is the part most worth testing, and a table-driven Go test is the only reason that logic is trustworthy.

Windows is deliberately absent.
No release runner here is Windows, and each platform costs about 2.4 MB in the repository forever.

## Consequences

**Good.**
The action runs instantly with nothing installed and nothing fetched.
It behaves identically on a Linux release runner and a macOS iOS runner.
There is no bootstrapping problem, so quill can release itself from its first commit.

**Bad.**
About 9.4 MB lands in git for every commit that changes the tool's source.
Git cannot delta compress binaries, so that space is never reclaimed.
This is per source change rather than per release, so it is tens of megabytes a year rather than hundreds.

**The real risk is drift.**
A committed binary that no longer matches the source beside it is a silent lie, and nothing fails.
`make verify-dist` rebuilds and diffs `dist/` in CI on every change, which turns drift into a failed check.

That gate only works if the build is reproducible, so `make dist` pins the Go toolchain exactly with `GOTOOLCHAIN`, passes `-trimpath`, and sets `-buildvcs=false`.
Without the toolchain pin the check fails whenever a laptop and the runner are on different patch releases of Go.
Without `-buildvcs=false` the commit hash is embedded and `dist/` changes on every commit, which defeats the whole arrangement.
