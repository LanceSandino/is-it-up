#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

echo "Validating shipped example configurations..."
go test ./pkg/checks -run '^TestExampleConfigurationsValidate$' -count=1
echo "example configuration validation: passed"
