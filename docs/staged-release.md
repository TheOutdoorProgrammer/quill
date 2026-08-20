# Staged releases

A composite action cannot choose a runner for one of its steps.
If a release needs macOS to build an iOS archive and Linux to build a container image, keep only the platform-specific build on macOS and hand its output to Quill.

The staged path has two pieces:

- `TheOutdoorProgrammer/quill/stage@v1` uploads caller-built files as a short-lived workflow artifact.
- `TheOutdoorProgrammer/quill/.github/workflows/staged-release.yml@v1` downloads those files on Ubuntu and runs the normal Quill action there.

The reusable workflow still runs GoReleaser, Fledge, and Docker in Quill's fixed order.
GoReleaser and Docker therefore share a filesystem, so a Dockerfile can consume files GoReleaser wrote to `dist/` without another upload/download cycle.

## Example

```yaml
jobs:
  ios:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v5

      # The repository still owns its certificate, provisioning, archive, and
      # export commands. Quill does not try to describe an arbitrary iOS build.
      - run: ./scripts/build-ios.sh

      - id: stage
        uses: TheOutdoorProgrammer/quill/stage@v1
        with:
          path: ${{ runner.temp }}/export/MyApp.ipa

    outputs:
      artifact-name: ${{ steps.stage.outputs.artifact-name }}

  release:
    needs: ios
    uses: TheOutdoorProgrammer/quill/.github/workflows/staged-release.yml@v1
    with:
      artifact-name: ${{ needs.ios.outputs.artifact-name }}
      fledge-ipa: MyApp.ipa
      bump: ${{ inputs.bump }}
      scope: ${{ inputs.scope }}
      publish: goreleaser,fledge,docker
    secrets:
      fledge-server: ${{ secrets.FLEDGE_URL }}
    permissions:
      contents: write
      packages: write
      id-token: write
```

`fledge-ipa` is relative to the root of the staged artifact.
The example stages one file, so the downloaded path is simply `MyApp.ipa`.

The caller owns permissions.
Grant `contents: write` for tags and GitHub releases, `packages: write` when Docker pushes to GHCR, and `id-token: write` when Fledge or cosign uses workload identity.
Add the attestation permissions when the corresponding Quill inputs are enabled.

A dry run still crosses the runner boundary and downloads the artifact.
Quill then performs its normal pre-tag builds and checks without cutting a tag or publishing anything.
