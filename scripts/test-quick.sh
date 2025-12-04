#!/bin/bash
# Quick smoke test - runs a subset of tests for fast feedback
# Usage: ./scripts/test-quick.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "=== Building tgpiler ==="
go build -o tgpiler ./cmd/tgpiler

echo ""
echo "=== Quick Smoke Tests ==="

# Test a few key functions from each category
go test -v ./tests/... -run "TestFactorial|TestGcd|TestLevenshtein|TestFutureValue|TestLoanPayment" -count=1

echo ""
echo "=== Quick Tests Passed ==="
