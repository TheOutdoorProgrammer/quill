# quill

The shared GitHub Action that cuts a release: version, tag, publish, clean up.
See [README.md](README.md) for how to use it, and [adr/](adr/) for the decisions worth not re-litigating.

## Build and test

- `make test` runs the suite with the race detector.
- `make lint` runs gofmt, vet and golangci-lint.
- `make dist` rebuilds the committed binaries in `dist/`.
- `make verify-dist` rebuilds and fails if anything changed.

**`dist/` is committed, not generated at release time.**
Any change under `cmd/` or `internal/` needs `make dist` and the result committed in the same commit, or CI fails.

## Layout

- `action.yml` is the single-runner composite release action.
- `stage/action.yml` stages caller-built files for a later release job.
- `.github/workflows/staged-release.yml` moves staged files onto Linux and runs `action.yml` there.
- `cmd/quill/` is one file per subcommand.
- `internal/plan/` is the version arithmetic and the publisher set.
- `internal/gitrepo/` is the git plumbing.
- `internal/actions/` is the runner boundary: workflow commands, outputs, summary.
- `scripts/run.sh` picks the `dist/` binary matching the runner.
- `dist/` is the committed cross-compiled output.

## Conventions

Go does the thinking, YAML does the wiring.
Anything worth a test belongs in `internal/`, not in a `run:` block.

**Inputs reach every script through `env:`, never `${{ }}` interpolation into a `run:` body.**
An interpolated body is assembled before bash sees it, so a value containing shell syntax executes.

Everything before the tag is side-effect free.
A guard that can only fail after tagging is a guard in the wrong place, because a tag cut for a failed release burns that version number.

This repo eats the house style it enforces elsewhere:

- Comments explain why, not what.
A block over three lines, or a line over eighty characters, is a smell.
- One sentence per line in markdown, and let the renderer wrap it.
- No em-dashes. A comma, colon, period, or parentheses does the job.
