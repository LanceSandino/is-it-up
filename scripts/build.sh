#!/usr/bin/env bash
set -euo pipefail

# build.sh - build reproducible is-it-up binaries with embedded version metadata
# Usage: ./scripts/build.sh [linux/amd64,linux/arm64]

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT_DIR=$(cd "$SCRIPT_DIR/.." && pwd)
DIST_DIR="$ROOT_DIR/dist"
MAIN_PKG="./cmd/is-it-up"

mkdir -p "$DIST_DIR"

BASE_VERSION=$(tr -d '[:space:]' <"$ROOT_DIR/VERSION")
COMMIT=${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD 2>/dev/null || printf 'unknown')}
BUILD_DATE=${BUILD_DATE:-$(git -C "$ROOT_DIR" show -s --format=%cI HEAD 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')}

if [ -z "${VERSION:-}" ]; then
  if tag=$(git -C "$ROOT_DIR" describe --tags --exact-match 2>/dev/null); then
    VERSION=${tag#v}
  else
    dirty=""
    if ! git -C "$ROOT_DIR" diff --quiet --ignore-submodules -- 2>/dev/null; then
      dirty=".dirty"
    fi
    VERSION="${BASE_VERSION}-dev+${COMMIT}${dirty}"
  fi
fi

targets=("linux/amd64" "linux/arm64")
if [ "$#" -ge 1 ]; then
  IFS=',' read -ra targets <<<"$1"
fi

echo "Building targets: ${targets[*]}"

build_target() {
  local target=$1
  if [ "$target" = "host" ]; then
    goos=$(go env GOOS)
    goarch=$(go env GOARCH)
  else
    goos=${target%%/*}
    goarch=${target##*/}
  fi

  out="$DIST_DIR/is-it-up-${goos}-${goarch}"
  echo "  -> building $goos/$goarch -> $out"
  ldflags="-s -w -X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildDate=$BUILD_DATE"
  env CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -buildvcs=true -ldflags "$ldflags" -o "$out" "$MAIN_PKG"
  chmod +x "$out" || true

  # if this matches host, also create a convenience copy 'dist/is-it-up'
  if [ "$(go env GOOS)" = "$goos" ] && [ "$(go env GOARCH)" = "$goarch" ]; then
    cp -f "$out" "$DIST_DIR/is-it-up"
    chmod +x "$DIST_DIR/is-it-up" || true
  fi
}

for t in "${targets[@]}"; do
  build_target "$t"
done

echo "Build complete: version=$VERSION commit=$COMMIT date=$BUILD_DATE"
ls -la "$DIST_DIR"
