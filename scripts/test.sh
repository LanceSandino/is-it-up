#!/usr/bin/env bash
set -euo pipefail

# Runs the deterministic local/CI quality gate using only the Go toolchain.
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

if [ -n "$(gofmt -l cmd pkg)" ]; then
  echo "Go files need formatting:" >&2
  gofmt -l cmd pkg >&2
  exit 1
fi

go vet ./...
go test -race ./...
go test ./... -coverprofile=coverage.out

coverage=$(go tool cover -func=coverage.out | awk '/^total:/ {sub(/%/, "", $3); print $3}')
awk -v coverage="$coverage" 'BEGIN { if (coverage + 0 < 85) exit 1 }' || {
  echo "statement coverage ${coverage}% is below 85%" >&2
  exit 1
}

echo "quality gate passed: statement coverage ${coverage}%"
