#!/bin/bash
# End-to-end tests: transpile SQL -> compile Go -> execute functions
# Usage: ./scripts/test-e2e.sh [compile|execute|all]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo "=== Building tgpiler ==="
go build -o tgpiler ./cmd/tgpiler

MODE="${1:-all}"

case "$MODE" in
    compile)
        echo ""
        echo "=== E2E Compile Tests ==="
        go test -v ./tests/... -run TestE2ECompileAll -count=1
        ;;
    execute)
        echo ""
        echo "=== E2E Execute Tests ==="
        go test -v ./tests/... -run "TestE2EExecuteBasic|TestE2EExecuteFinancial|TestE2EExecuteNontrivial" -count=1
        ;;
    all)
        echo ""
        echo "=== E2E Compile Tests (all 55 files) ==="
        go test -v ./tests/... -run TestE2ECompileAll -count=1
        
        echo ""
        echo "=== E2E Execute Tests (selected functions) ==="
        go test -v ./tests/... -run "TestE2EExecuteBasic|TestE2EExecuteFinancial|TestE2EExecuteNontrivial" -count=1
        ;;
    *)
        echo "Usage: $0 [compile|execute|all]"
        echo ""
        echo "  compile  - Transpile and compile all 55 SQL files"
        echo "  execute  - Transpile, compile, and run selected functions"
        echo "  all      - Run both (default)"
        exit 1
        ;;
esac

echo ""
echo "=== E2E Tests Passed ==="
