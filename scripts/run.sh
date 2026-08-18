#!/usr/bin/env bash
# Run the committed quill binary that matches this runner.
set -euo pipefail

fail() {
  echo "::error::$*"
  exit 1
}

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) fail "quill ships no binary for $(uname -s). Linux and macOS runners only." ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "quill ships no binary for $(uname -m). amd64 and arm64 only." ;;
esac

binary="${here}/../dist/quill-${os}-${arch}"

[ -f "$binary" ] || fail "missing ${binary##*/}. The action was checked out without its dist/."

# Git records the executable bit, but an archive export or a filesystem without
# permissions loses it, and the failure otherwise reads as a missing binary.
[ -x "$binary" ] || chmod +x "$binary" || fail "cannot make ${binary##*/} executable"

exec "$binary" "$@"
