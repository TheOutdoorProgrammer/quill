---
dusk: v1alpha1
namespace: stout
kind: repository
name: quill
title: Quill
attributes:
  language: go
  visibility: public
  install: uses TheOutdoorProgrammer/quill@v1
---

The shared GitHub Action that cuts a release, extracted from the pipeline that lived in ollie-hooks.

A caller declares the `workflow_dispatch` dropdowns (`Release` or `Release Candidate`, and `patch`, `minor` or `major`) and calls quill in one step.
Quill works out the version from the tag list, refuses to release off the wrong branch or from a shallow checkout, cross-builds before committing to a number, tags, publishes, moves the `vN` alias, and deletes the tag if anything after tagging fails.

Publishing is a set rather than a choice, so a repository carrying more than one artefact gets one version and one tag for all of them (adr/0003).
`goreleaser` runs GoReleaser, `fledge` hands the archive to `service:stout/fledge`, `docker` builds and pushes a multi-platform image, and `none` still tags and writes a GitHub release.
They run in a fixed sequence, GoReleaser then Fledge then Docker, and a caller listing them differently does not change it (adr/0005).
Every `fledge-*` input defaults to empty because Fledge reads the `fledge.yaml` beside the app, so configuration has one home.

Cross-runner releases use `quill/stage@v1` to move a caller-built artifact into `.github/workflows/staged-release.yml@v1`, where every selected publisher runs in one Linux release transaction.
The staged workflow can mint a short-lived GCP Artifact Registry token from optional `gcp-workload-identity-provider` and `gcp-service-account` inputs.
Both inputs are required together, explicit Docker credentials retain priority, and callers that omit them keep the existing GHCR actor/token defaults.

Optional `tap-app-id` and `tap-private-key` inputs mint a scoped, hour-long token and export `HOMEBREW_TAP_GITHUB_TOKEN`, which is the boilerplate every repository publishing a cask to `repository:stout/homebrew-tap` used to carry itself.

The logic is Go in `cmd/quill` and `internal/`, cross-compiled and **committed** to `dist/` for linux and darwin on amd64 and arm64 (adr/0001).
Nothing is downloaded and no toolchain is installed at run time.
`scripts/run.sh` picks the binary from `uname`.

Quill releases itself with itself: its `release.yml` uses `uses: ./` rather than a pinned tag.

## Gotchas

**`dist/` is committed and must never be gitignored.**
A committed binary that no longer matches its source is a silent lie, so `make verify-dist` rebuilds and diffs it in CI.
That gate only holds because the build is reproducible: `make dist` pins the Go toolchain exactly with `GOTOOLCHAIN`, passes `-trimpath`, and sets `-buildvcs=false`.
Change the Go version in `go.mod` without changing the Makefile pin and the check fails with a confusing binary diff.

**A caller that forgets `fetch-depth: 0` gets a refusal, not a wrong version.**
The default checkout is depth 1 and carries no tag history, which would restart the version line at the beginning.

**The moving `vN` alias never moves for a candidate, and moves last.**
The version parser rejects a bare `v1` so the alias is never mistaken for a release when computing the next one.

**A first release with the default `patch` bump is `v0.0.1`, not `v0.1.0`.**
The bump applies to an implicit `v0.0.0`, which is the only way to open a project at `1.0.0` (adr/0004).
This differs from the `internal/nextversion` rule quill was extracted from.

**A release candidate never takes the Docker `latest` tag, nor the major or minor tags.**
An unguarded `latest` means `docker pull` with no tag hands people a candidate.
The version reaches the tagger as an explicit value rather than being read off the ref, because the tag does not exist yet when the image is built.

**Publisher order is fixed and lives in `plan.Order`.**
A caller-chosen order was built as a step per publisher per position and then removed, because nobody could name a case needing it (adr/0005).
Matrix and `needs` are not reachable: they are job-level, and a composite action has no jobs.

**Fledge's action is pinned at `@v1`, a moving tag, and cannot be an input.**
`uses:` does not accept expressions.
Tracking Fledge's major is deliberate, so its changes reach quill's users without quill cutting a release.

**GCP OIDC authentication belongs inside the staged reusable workflow.**
A caller job that invokes a reusable workflow cannot also run an authentication step.
Minting the token in a separate job would relay credential material across the job boundary and start its lifetime before the publisher job begins.
Pass the workload identity provider and service account as normal workflow inputs, grant the called job `id-token: write`, and let Quill mint the access token where Docker consumes it.
