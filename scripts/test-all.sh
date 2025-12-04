#!/bin/bash
# Run all tests for tgpiler
# Usage: ./scripts/test-all.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "=== Building tgpiler ==="
go build -o tgpiler ./cmd/tgpiler

echo ""
echo "=== Running All Tests ==="
go test -v ./tests/... 2>&1 | tee /tmp/test-results.txt

echo ""
echo "=== Summary ==="
PASS_COUNT=$(grep -c "PASS:" /tmp/test-results.txt || echo "0")
FAIL_COUNT=$(grep -c "FAIL:" /tmp/test-results.txt || echo "0")

echo "Passed: $PASS_COUNT"
echo "Failed: $FAIL_COUNT"

if grep -q "^FAIL" /tmp/test-results.txt; then
    echo ""
    echo "SOME TESTS FAILED"
    exit 1
else
    echo ""
    echo "ALL TESTS PASSED"
    exit 0
fi
