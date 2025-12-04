#!/bin/bash
# Verify all SQL files transpile to valid Go code
# Usage: ./scripts/test-compilation.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "=== Building tgpiler ==="
go build -o tgpiler ./cmd/tgpiler

echo ""
echo "=== Running Compilation Tests ==="
go test -v ./tests/... -run TestCompilation

echo ""
echo "=== Compilation Tests Passed ==="
