#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Ensure Go is available
if command -v conda &> /dev/null; then
    eval "$(conda shell.bash hook)"
    conda activate gochi-requestid 2>/dev/null || true
fi

cd "$ISSUE_DIR/workspace"

echo "=== Running RequestID middleware tests ==="
go test ./middleware/ -run "TestRequestID|TestGetReqID" -v -count=1

echo ""
echo "=== All tests passed ==="
