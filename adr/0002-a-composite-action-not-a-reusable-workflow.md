# 0002. A composite action, not a reusable workflow

## Context and problem statement

The release pipeline being extracted is a whole job: check out, guard the branch, work out the version, cross-build, tag, publish, clean up.
GitHub offers two ways to share that.

A **reusable workflow** is called as a whole job (`jobs.release.uses`).
It owns the runner, the permissions and the checkout, so the caller writes almost nothing.

A **composite action** is called as a step.
The caller still declares the job, the runner, the permissions and the checkout, and quill slots in among their own steps.

## Considered options

- A composite action
- A reusable workflow
- Both, with the reusable workflow wrapping the composite action

## Decision outcome

**A composite action.**

The deciding case is Fledge.
An iOS release has to import a signing certificate, install a provisioning profile, run `xcodebuild archive` and `xcodebuild -exportArchive` on a macOS runner, and only then publish the archive.
All of that is repository-specific and has to happen in the same job as the publish step, because it produces the file being published.
A reusable workflow cannot host it: the caller cannot inject steps into a workflow it does not own.

Shipping both was considered and rejected as a second surface to keep in step for a saving of about fifteen lines in the callers that could use it.

Neither option can move the dropdowns.
GitHub requires `workflow_dispatch` inputs to be declared in the workflow file being dispatched, so that block stays with the caller whatever we do here.
That removes most of the reusable workflow's advantage before the Fledge argument is even reached.

## Consequences

**Good.**
One surface.
The same action serves a Go repository, an iOS repository, and a repository that is both, because the caller controls what happens around it.
Callers can put steps before and after it, which is what an iOS build requires.

**Bad.**
Every caller repeats the job declaration, the runner, `permissions: contents: write`, and `actions/checkout` with `fetch-depth: 0`.
That is roughly ten lines that a reusable workflow would have absorbed.

Forgetting `fetch-depth: 0` is the failure this invites, so quill detects a shallow checkout and refuses rather than silently restarting the version line.
