#!/bin/bash
# Run financial function tests only
# Usage: ./scripts/test-financial.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "=== Building tgpiler ==="
go build -o tgpiler ./cmd/tgpiler

echo ""
echo "=== Running Financial Tests ==="
go test -v ./tests/... -run "TestCompilationFinancial|TestFuture|TestLoan|TestProgressive|TestBreak|TestBond|TestCompound|TestStraight|TestMarkup"

echo ""
echo "=== Financial Tests Passed ==="
