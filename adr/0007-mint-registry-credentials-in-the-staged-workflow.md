# 0007. Mint registry credentials in the staged workflow

## Context and problem statement

The staged release workflow moves a platform-specific build artifact into one Linux job where Quill publishes GoReleaser artifacts, a Fledge IPA, and a container under one version.
Its original registry contract accepted an optional username and password, then fell back to the GitHub actor and token for GHCR.

GCP Artifact Registry should use a short-lived access token minted through Workload Identity Federation.
A caller cannot run an authentication step before a reusable-workflow job because a job with `uses` cannot also contain `steps`.
It can mint the token in a separate job and relay it through job outputs and the reusable workflow's secrets contract, but that spreads authentication across two jobs and starts the token lifetime before the release job begins.

## Considered options

- Mint the access token in a separate caller job and relay it into Quill as a secret.
- Keep GCP authentication in each consuming repository and copy Quill's staged workflow into those repositories.
- Store a long-lived service-account key or registry password in each consuming repository.
- Add optional GCP Workload Identity inputs to Quill's staged reusable workflow.

## Decision outcome

The staged reusable workflow accepts an optional Workload Identity Provider and service account.
When both are present, it uses `google-github-actions/auth` to mint an access token without writing a credentials file into the checkout.
Quill passes that token to Docker with Artifact Registry's `oauth2accesstoken` username.

Both inputs must be present or both must be absent.
An explicit Docker username and password keep priority over the minted credential.
When the GCP inputs are absent, the existing GitHub actor and token fallback is unchanged.

### Positive consequences

- Staged releases can publish every selected artifact in one Quill transaction while retaining short-lived GCP credentials.
- Consuming repositories do not copy Quill's orchestration or store service-account keys.
- Existing GHCR, Docker Hub, and explicit-password callers remain compatible.
- The checkout stays clean for GoReleaser.

### Negative consequences

- The reusable workflow now has a provider-specific authentication option.
- GCP callers must grant `id-token: write` and configure their workload identity binding outside Quill.
- Other registries with their own workload identity exchange still need an explicit credential or a separate backwards-compatible integration.
