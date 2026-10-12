#!/usr/bin/env bash
set -euo pipefail

# Builds Linux release archives and their SHA-256 checksum manifest.
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
VERSION=${1:-$(tr -d '[:space:]' <"$ROOT_DIR/VERSION")}
RELEASE_DIR="$ROOT_DIR/dist/release"

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid semantic version: $VERSION" >&2
  exit 2
fi

mkdir -p "$RELEASE_DIR"
rm -f "$RELEASE_DIR/is-it-up_linux_amd64.tar.gz" \
  "$RELEASE_DIR/is-it-up_linux_arm64.tar.gz" \
  "$RELEASE_DIR/SHA256SUMS"

VERSION="$VERSION" /bin/bash "$ROOT_DIR/scripts/build.sh" linux/amd64,linux/arm64

for arch in amd64 arm64; do
  stage=$(mktemp -d "${TMPDIR:-/tmp}/is-it-up-release.XXXXXX")
  trap 'rm -rf "${stage:-}"' EXIT
  install -m 0755 "$ROOT_DIR/dist/is-it-up-linux-$arch" "$stage/is-it-up"
  printf '%s\n' "$VERSION" >"$stage/VERSION"
  install -m 0644 "$ROOT_DIR/CHANGELOG.md" "$stage/CHANGELOG.md"
  install -m 0644 "$ROOT_DIR/README.md" "$stage/README.md"
  install -m 0644 "$ROOT_DIR/example/config.example.yaml" "$stage/config.example.yaml"
  install -d -m 0755 "$stage/packaging/systemd"
  install -m 0644 "$ROOT_DIR/packaging/systemd/is-it-up.service" "$stage/packaging/systemd/is-it-up.service"
  install -m 0644 "$ROOT_DIR/packaging/systemd/is-it-up.timer" "$stage/packaging/systemd/is-it-up.timer"
  if [ -f "$ROOT_DIR/LICENSE" ]; then
    install -m 0644 "$ROOT_DIR/LICENSE" "$stage/LICENSE"
  fi
  tar -C "$stage" -czf "$RELEASE_DIR/is-it-up_linux_${arch}.tar.gz" .
  rm -rf "$stage"
  trap - EXIT
done

(
  cd "$RELEASE_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum is-it-up_linux_amd64.tar.gz is-it-up_linux_arm64.tar.gz >SHA256SUMS
  else
    shasum -a 256 is-it-up_linux_amd64.tar.gz is-it-up_linux_arm64.tar.gz >SHA256SUMS
  fi
)

echo "Release archives written to $RELEASE_DIR"
