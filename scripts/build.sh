#!/bin/bash
# Build tgpiler transpiler
# Usage: ./scripts/build.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "=== Building tgpiler ==="
go build -o tgpiler ./cmd/tgpiler

echo "=== Build Successful ==="
echo "Binary: $PROJECT_DIR/tgpiler"
echo ""
echo "Usage: ./tgpiler <input.sql> [output.go]"
