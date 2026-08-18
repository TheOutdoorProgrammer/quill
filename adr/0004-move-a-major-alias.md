# 0004. Move a major alias, and let the first release choose its version

## Context and problem statement

Two version questions came out of quill releasing itself.

**Pinning.**
`actions/checkout@v4` works because `v4` is a tag that moves to each new v4 release.
Without that convention a consumer has to write `@v1.4.2` and hand-edit it for every patch, so in practice they pin `@main` and take whatever lands.
The pipeline quill was extracted from never moved a major tag, because it released a CLI that people install rather than an action that people pin.

**Opening at 1.0.0.**
The inherited rule returned `v0.1.0` for a first release whatever the bump said.
That makes `v1.0.0` unreachable: an action repository that wants consumers pinning `@v1` from day one would have to cut a throwaway tag first and then bump major.

## Considered options

For pinning:

- Move a `vN` tag on each release
- Move `vN` and `vN.M`, as some actions do
- Leave it to the consumer to pin an exact version

For the first release:

- Keep `v0.1.0` regardless of bump
- Apply the bump to an implicit `v0.0.0`
- Add a separate `first-version` input

## Decision outcome

**Move a `vN` tag**, and only `vN`.
`vN.M` was rejected as a second moving reference for a case nobody here has asked for.

Three constraints make it safe:

- It never moves for a release candidate, or everyone pinned to `@v1` is silently upgraded to a version that exists to be rehearsed.
- It moves last, after every publisher has succeeded, so a release that failed halfway leaves it pointing where it was.
- The version parser rejects a bare `v1`, so the alias quill creates is never mistaken for a release when computing the next one.

**Apply the bump to an implicit `v0.0.0`** when there are no tags.
`major` gives `v1.0.0`, `minor` gives `v0.1.0`, `patch` gives `v0.0.1`.

A separate `first-version` input was rejected as an input that is meaningless after the first release and wrong forever after that.
The consistent rule is one sentence to document, and the special case it replaces was one sentence plus an exception.

## Consequences

**Good.**
Consumers pin `@v1` and keep getting fixes.
A project can open at `1.0.0` by saying so.
`major-alias: "false"` turns the whole thing off for a repository that does not want a moving tag.

**Bad.**
This changes the inherited behaviour: a first release with the default `patch` is now `v0.0.1` rather than `v0.1.0`.
A project meaning to start at `v0.1.0` has to pick `minor`.
Nothing already released is affected, because the rule only applies when there are no tags at all.

A moving tag is a force push, which is a rewrite of a published reference.
That is the accepted convention for actions and the only mechanism GitHub offers, but it means `@v1` is not immutable and anyone needing that guarantee should pin an exact version.
