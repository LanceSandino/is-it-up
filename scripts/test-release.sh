#!/usr/bin/env bash
set -euo pipefail

# Verifies Linux release packaging, version injection, and checksum output.
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

VERSION=9.8.7 /bin/bash scripts/package-release.sh 9.8.7

for arch in amd64 arm64; do
  archive="dist/release/is-it-up_linux_${arch}.tar.gz"
  test -f "$archive"
  tar -tzf "$archive" | grep -Eq '(^|/)is-it-up$'
  tar -tzf "$archive" | grep -Eq '(^|/)packaging/systemd/is-it-up.service$'
  tar -xOzf "$archive" ./packaging/systemd/is-it-up.service | grep -Fq 'MemoryMax=128M'
done

grep -Fq 'Verify tag targets the current main commit' .github/workflows/release.yml
grep -Fq 'test "$TAG_COMMIT" = "$(git rev-parse origin/main)"' .github/workflows/release.yml
grep -Fq 'mem_limit: 128m' example/docker-compose.is-it-up.yml
grep -Fq 'pids_limit: 64' example/docker-compose.is-it-up.yml

(
  cd dist/release
  shasum -a 256 -c SHA256SUMS
)

if command -v file >/dev/null 2>&1; then
  file dist/is-it-up-linux-amd64 | grep -qi 'ELF 64-bit.*x86-64'
  file dist/is-it-up-linux-arm64 | grep -qi 'ELF 64-bit.*ARM aarch64'
fi

printf 'release packaging: passed\n'
