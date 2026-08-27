#!/usr/bin/env bash
set -euo pipefail

umask 077
signing_dir="$(mktemp -d "$RUNNER_TEMP/quill-apple-signing.XXXXXX")"
trap 'rm -rf "$signing_dir"' EXIT
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

printf '%s' "$P12_FILE_BASE64" \
  | tr -d '\r\n' \
  | openssl base64 -d -A -out "$signing_dir/distribution.p12"
openssl pkcs12 -legacy \
  -in "$signing_dir/distribution.p12" \
  -clcerts \
  -nokeys \
  -passin env:P12_PASSWORD \
  -out "$signing_dir/distribution.pem"
security unlock-keychain -p "$SIGNING_KEYCHAIN_PASSWORD" "$SIGNING_KEYCHAIN"
security import "$here/AppleWWDRCAG3.pem" \
  -k "$SIGNING_KEYCHAIN" \
  -f pemseq \
  -t cert >/dev/null 2>&1 || true
security find-certificate -a -Z "$SIGNING_KEYCHAIN" \
  | grep -Fq DCF21878C77F4198E4B4614F03D696D89C66C66008D4244E1B99161AAC91601F \
  || { echo "::error::the Apple WWDR G3 intermediate was not imported"; exit 1; }
security verify-cert \
  -c "$signing_dir/distribution.pem" \
  -p codeSign \
  -k "$SIGNING_KEYCHAIN" \
  -q \
  || { echo "::error::the imported certificate chain is not trusted"; exit 1; }
printf '%s' "$ASC_PRIVATE_KEY" > "$signing_dir/AuthKey_$ASC_KEY_ID.p8"

fingerprint="$(openssl x509 \
  -in "$signing_dir/distribution.pem" \
  -noout \
  -fingerprint \
  -sha1 \
  | cut -d= -f2 \
  | tr -d ':')"
security find-identity \
  -v \
  -p codesigning \
  "$SIGNING_KEYCHAIN" > "$signing_dir/identities"
grep -q "$fingerprint" "$signing_dir/identities" \
  || { echo "::error::the imported certificate has no usable private key"; exit 1; }

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "::error::quill apple signing supports amd64 and arm64 only"; exit 1 ;;
esac
binary="$here/dist/quill-apple-signing-darwin-$arch"
[ -f "$binary" ] || { echo "::error::missing ${binary##*/}"; exit 1; }
[ -x "$binary" ] || chmod +x "$binary"

QUILL_APPLE_CERTIFICATE_PATH="$signing_dir/distribution.pem" \
QUILL_ASC_ISSUER_ID="$ASC_ISSUER_ID" \
QUILL_ASC_KEY_ID="$ASC_KEY_ID" \
QUILL_ASC_PRIVATE_KEY_PATH="$signing_dir/AuthKey_$ASC_KEY_ID.p8" \
QUILL_BUNDLE_IDENTIFIERS="$BUNDLE_IDENTIFIERS" \
  "$binary"
