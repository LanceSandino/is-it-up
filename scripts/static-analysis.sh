#!/usr/bin/env bash
set -euo pipefail

# Pinned analysis keeps local and CI results reproducible with Go 1.27.
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

go vet ./...
GOTOOLCHAIN=go1.27.2 go run github.com/securego/gosec/v2/cmd/gosec@v2.29.0 -quiet ./...

echo "static analysis passed"
