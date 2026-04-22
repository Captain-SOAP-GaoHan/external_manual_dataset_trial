#!/bin/bash
set -e

ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE="$ISSUE_DIR/workspace"

echo "=== Running CORS middleware verifier ==="

# P1: Check that CORS middleware exists in workspace (init state check is manual)
if [ ! -f "$WORKSPACE/middleware/cors.go" ]; then
    echo "FAIL P1: middleware/cors.go does not exist in workspace"
    exit 1
fi
echo "PASS P1: middleware/cors.go exists in workspace"

# Build and run the verifier
echo ""
echo "=== Building and running functional verifier ==="
cd "$WORKSPACE"
cp "$ISSUE_DIR/tests/verifier.go" "$WORKSPACE/verifier_test_main.go"
go run verifier_test_main.go
rm -f verifier_test_main.go

echo ""
echo "=== Running project's own CORS tests ==="
go test ./middleware/ -run TestCORS -v

echo ""
echo "=== All verifier tests completed ==="
