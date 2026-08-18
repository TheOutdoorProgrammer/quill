# 0003. Publishers are a set, and Fledge keeps its own config

## Context and problem statement

Quill publishes through GoReleaser and through Fledge.
The obvious modelling is a choice: one `publish` input holding exactly one backend.

That is wrong for a repository carrying both a Go binary and an iOS app.
Such a repository wants one version, one tag, and one release covering the pair.
Modelling the backend as a choice forces two release workflows, which means two tags for one release, or one workflow silently doing half the job.

A second question sits underneath it.
Fledge needs a server URL, an archive path, notes and a signing policy.
Quill could take all of that as inputs and pass it through, or leave it to Fledge.

## Considered options

For the backend:

- A single-valued `publish` choice
- A set, parsed from a comma or newline separated list
- One boolean input per publisher, such as `goreleaser: true`

For the configuration:

- Quill declares every Fledge input and passes it through
- Quill passes nothing and Fledge reads its own `fledge.yaml`
- Quill declares optional inputs that override `fledge.yaml` when set

## Decision outcome

**A set**, parsed and validated in Go rather than in YAML.
`publish: goreleaser, fledge` runs both against one version and one tag.

A list beats one boolean per publisher because it reads as a single decision, and adding a third backend later does not add another input to every caller's workflow.
Validation is loud: an unknown name is refused rather than quietly publishing nothing, and listing `none` alongside a real publisher is a contradiction the caller is told about rather than having it resolved for them.

**Fledge keeps its own configuration.**
Every `fledge-*` input defaults to empty, and Fledge reads the `fledge.yaml` beside the app.
Quill's inputs exist only as an override for a repository that has no spec file.

Configuration with two homes drifts, and the copy in the workflow file is the one that goes stale, because it is the one nobody looks at while working on the app.

## Consequences

**Good.**
A repository that ships both artefacts gets one tag and one release.
An iOS repository passes a token and nothing else, because everything else is already written down beside the app.
A typo in `publish` fails the run instead of producing a release that published nothing.

**Bad.**
Quill cannot check an archive exists before tagging when the path lives in `fledge.yaml`, because it never sees the path.
The pre-tag check is therefore available only to callers who also pass `fledge-ipa`.
The untag-on-failure step is the safety net for everyone else, so a missing archive costs a failed run rather than a burned version number.

Fledge's own action inputs are pinned at `@v1`, a tag that moves.
`uses:` cannot take an expression, so the reference is hardcoded and cannot be a quill input.
Tracking Fledge's moving major is deliberate: it means a change like `fledge.yaml` support reaches quill's users without quill cutting a release.
It also means a breaking change in Fledge's `v1` would reach them the same way.
