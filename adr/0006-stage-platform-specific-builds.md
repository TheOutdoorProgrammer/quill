# 0006. Stage platform-specific builds before publishing

## Context and problem statement

A composite action runs every step on the caller job's runner.
That is useful when a repository needs to prepare an iOS archive beside Quill, but it also means selecting Docker on a macOS release job asks Docker to run on macOS.
GitHub-hosted macOS runners do not provide the Linux Docker engine that `docker/build-push-action` expects.

The conflict is at the job boundary, not between publishers.
Fledge's publishing action itself runs on Linux as well as macOS.
Only the repository-specific Xcode archive, signing, and export work actually needs the macOS runner.

GoReleaser and Docker should still share a runner and filesystem.
A Dockerfile is the publisher most likely to consume something GoReleaser put in `dist/`, and the fixed GoReleaser, Fledge, Docker ordering exists for exactly that dependency.

## Considered options

- Make the composite action change runners between steps
- Make Quill own the repository's Xcode build in a reusable workflow
- Split every publisher into an independent job and pass all outputs as artifacts
- Stage only platform-specific build outputs, then run Quill on Linux

## Decision outcome

**Stage platform-specific build outputs, then run the existing Quill action on Linux.**

`stage/action.yml` uploads files produced by a caller-owned job as a short-lived workflow artifact.
The caller keeps its signing setup, Xcode commands, Fastlane invocation, or any other platform-specific preparation exactly where it already lives.

`.github/workflows/staged-release.yml` is a reusable workflow that downloads that artifact on `ubuntu-latest` and invokes the normal Quill composite action there.
The workflow owns only the runner transition and artifact handoff.
Publisher behavior continues to have one implementation in `action.yml`.

The staged workflow interprets `fledge-ipa` relative to the downloaded artifact root.
Other release inputs keep the same meaning they have on the composite action.
The caller still owns permissions because a called workflow cannot increase the permissions granted by its caller.

The existing composite action remains the default public surface for releases whose preparation and publishers can share one runner.
The staged path is opt-in for releases that need a platform-specific preparation job and a Linux publisher job.

## Consequences

**Good.**
Existing iOS build logic stays in the consuming repository rather than becoming Quill configuration.
Docker always runs on Linux in the staged path.
GoReleaser and Docker are back in one job, so a Docker build can consume GoReleaser's `dist/` directly with no second artifact handoff.
The tag, rollback, major alias, Docker preflight, and publisher ordering remain owned by the existing composite action.
A dry run exercises the same handoff without publishing or cutting a tag.

**Bad.**
The caller has two jobs instead of one: its platform-specific build job and the reusable release job.
The staged files are copied once through GitHub's artifact service.
A caller that needs another publisher to execute on a third operating system still needs another explicit handoff rather than a magical runner switch inside a composite action.

ADR 0002 still applies to the composite action itself.
This decision adds a second orchestration surface for the case ADR 0002 could not solve without giving Quill ownership of the caller's iOS build.
