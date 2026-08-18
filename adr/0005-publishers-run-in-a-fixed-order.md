# 0005. Publishers run in a fixed order

## Context and problem statement

Adding Docker as a third publisher raised the question of sequencing.
The initial requirement was that a caller should choose the order: `docker, fledge, goreleaser` and `goreleaser, fledge, docker` should each run as written.

This collides with a platform limit.
A composite action's steps are static YAML, and `uses:` cannot take an expression, so a step cannot be chosen at run time.
The supported keys on a composite action step are `run`, `shell`, `if`, `name`, `id`, `env`, `working-directory`, `uses`, `with` and `continue-on-error`.
`strategy`, `matrix` and `needs` are all job-level, and a composite action has no jobs.

## Considered options

- **Fixed order**, with the caller's list read as a set
- **A step per publisher per position**, gated on what the planner assigned to each position
- **A reusable workflow with a matrix job per publisher**, ordered with `max-parallel: 1`
- **One dispatcher step** looping over the publishers in a shell

## Decision outcome

**Fixed order: GoReleaser, then Fledge, then Docker.**
The `publish` input stays a list for readability, but it is read as a set and reported back in the order it will actually run.

The ordering rationale is that only one edge is plausible.
GoReleaser produces the GitHub release and its archives, which is the thing another publisher might consume.
Fledge publishes an iOS archive and depends on neither of the others.
An image is the artefact most likely to want something already published, so Docker goes last.
Nobody could name a case needing a different order.

**The position-per-slot version was built, tested, and removed.**
It worked: three positions times three publishers, each gated on a `slot-N` output, with a test that normalised the copies and diffed them so they could not drift.
That test was verified to fail on real drift.
It was removed anyway, because roughly 130 lines of mechanical YAML is a real cost to carry for flexibility nobody could find a use for.
The commit history holds it if the requirement ever comes back.

**Matrix was rejected because it does not do what it appears to.**
It would require quill to become a reusable workflow, which breaks the iOS case outright: a matrix job is a fresh runner, and the `.ipa` is built by `xcodebuild` in the caller's own job (adr/0002).
Worse, it would not deliver ordering anyway.
`max-parallel` is documented as a concurrency cap, not an execution order, and matrix entries have no specified order.
"Usually runs in the listed order" is not a property to hang a release pipeline on.

**The shell dispatcher was rejected on ownership.**
A single step looping in the caller's order is only possible if every publisher is a shell invocation, which means dropping `docker/build-push-action` and hand-rolling the Actions cache token plumbing, dropping `goreleaser-action`, and reimplementing Fledge's own publish action inside quill.
That trades three maintained things for bash we would own forever, and it duplicates a sibling repository (see the reuse rule in `CLAUDE.md`).

## Consequences

**Good.**
Three publishing steps instead of nine, and no `slot-N` outputs.
The sequence is stated once, in `plan.Order`, and `TestPublishStepsFollowTheDeclaredOrder` fails if `action.yml` disagrees with it.
The upstream actions keep doing the work they are good at.

**Bad.**
A caller who needs Docker before GoReleaser cannot have it, and would have to run two jobs or send a patch.
Listing publishers in a particular order is silently ignored rather than rejected, which could mislead.
The run summary states the real sequence to make that visible rather than surprising.

Adding a fourth publisher means deciding where it sits in `plan.Order`, which is a decision worth making deliberately rather than one the caller makes per release.
