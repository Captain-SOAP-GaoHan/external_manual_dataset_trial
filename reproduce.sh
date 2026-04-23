#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_DIR="$HOME/.tools"
ENV_FILE="$HOME/.issue_env"

# ──────────────────────────────────────────
# 1. 安装运行环境
# ──────────────────────────────────────────

# Install Go via conda if not already available
if ! command -v go &> /dev/null; then
    if ! conda env list | grep -q "^gochi-requestid "; then
        conda env create -f "$ISSUE_DIR/environment.yml"
    fi
    eval "$(conda shell.bash hook)"
    conda activate gochi-requestid
fi

# ──────────────────────────────────────────
# 2. 将 init/ 复制为 workspace/（此步必须成功）
# ──────────────────────────────────────────
[ -d "$ISSUE_DIR/workspace" ] && rm -rf "$ISSUE_DIR/workspace"
cp -r "$ISSUE_DIR/init" "$ISSUE_DIR/workspace"

# Download Go module dependencies in workspace
cd "$ISSUE_DIR/workspace"
go mod download 2>/dev/null || true

echo "Environment setup complete. Workspace is at: $ISSUE_DIR/workspace"
