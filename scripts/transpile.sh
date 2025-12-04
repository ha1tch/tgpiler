#!/bin/bash
# Transpile a SQL file to Go and optionally run it
# Usage: ./scripts/transpile.sh <input.sql> [--run]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

if [ -z "$1" ]; then
    echo "Usage: $0 <input.sql> [--run]"
    echo ""
    echo "Examples:"
    echo "  $0 tsql_basic/01_simple_add.sql"
    echo "  $0 tsql_financial/04_loan_payment.sql --run"
    exit 1
fi

INPUT_FILE="$1"
RUN_FLAG="$2"

cd "$PROJECT_DIR"

# Build if needed
if [ ! -f "./tgpiler" ]; then
    echo "=== Building tgpiler ==="
    go build -o tgpiler ./cmd/tgpiler
fi

# Get base name
BASENAME=$(basename "$INPUT_FILE" .sql)
OUTPUT_FILE="/tmp/${BASENAME}.go"

echo "=== Transpiling $INPUT_FILE ==="
./tgpiler "$INPUT_FILE" > "$OUTPUT_FILE"

echo "Generated: $OUTPUT_FILE"
echo ""

# Check syntax
if gofmt -e "$OUTPUT_FILE" > /dev/null 2>&1; then
    echo "Syntax: OK"
else
    echo "Syntax: ERROR"
    gofmt -e "$OUTPUT_FILE"
    exit 1
fi

# Show generated code
echo ""
echo "=== Generated Go Code ==="
cat "$OUTPUT_FILE"

# Run if requested
if [ "$RUN_FLAG" = "--run" ]; then
    echo ""
    echo "=== Running ==="
    cd /tmp
    
    # Create a simple main wrapper if needed
    if ! grep -q "func main()" "$OUTPUT_FILE"; then
        echo "Note: No main() function - cannot run directly"
    else
        go run "$OUTPUT_FILE"
    fi
fi
