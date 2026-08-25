# 0008. Share persistent Apple signing setup as an optional action

## Context and problem statement

Coop, Planty, and Tiller all publish Ad Hoc iOS applications from ephemeral GitHub-hosted macOS runners.
Automatic Xcode signing creates a new certificate when the runner has no private key, which eventually exhausts Apple's certificate quota.

Tiller solved this by importing a persistent Apple Distribution identity and creating or reusing an Ad Hoc profile whose name records the exact certificate and enabled-device membership.
Keeping that implementation in Tiller would require Coop and Planty to copy security-sensitive code.

ADR 0006 keeps platform-specific archive and export work in the caller rather than making the staged release workflow own Xcode.
The repository already exposes the main release action and the independent `stage` action, so that boundary does not require every future capability to live outside Quill.

## Considered options

- Copy Tiller's signing and profile code into every iOS application repository
- Create a separate public repository containing one provisioning action
- Add an optional sibling action to Quill while keeping archive and export commands in callers
- Make `quill/stage` perform signing, archive, and export

## Decision outcome

**Add `quill/apple-signing` as a third, optional action.**

The action imports a caller-provided persistent PKCS#12 identity through the Apple-Actions community certificate-import action.
It then creates or reuses `IOS_APP_ADHOC` profiles tied to that exact certificate and the current enabled iOS device set, and installs them on the runner.

Callers continue to own their Xcode project, archive command, export options, entitlements, and signed-archive verification.
The staged release workflow remains an artifact handoff and does not call the signing action implicitly.

Because the action handles signing and App Store Connect credentials, callers pin it to an immutable commit rather than the moving `v1` alias.

## Consequences

**Good.**
Certificate creation is removed from routine release jobs, so ephemeral runners no longer consume Apple's certificate quota.
Profile membership refreshes automatically when the registered-device set changes.
Certificate matching and profile reconciliation have one tested implementation.
Applications retain explicit control of bundle-specific archive, export, and entitlement policy.

**Bad.**
Quill now carries Apple-specific code and committed macOS binaries even though its primary release action is platform-neutral.
Consumers must update an immutable Quill commit pin deliberately when the signing action changes.
The action depends on App Store Connect profile APIs and Apple's certificate-import action in addition to Xcode itself.
