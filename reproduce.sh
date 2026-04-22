#!/bin/bash
set -e

ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_DIR="$HOME/.tools"
ENV_FILE="$HOME/.issue_env"

# ──────────────────────────────────────────
# 1. Install Go if not available (conda or manual)
# ──────────────────────────────────────────
if ! command -v go &> /dev/null; then
    echo "Go not found, installing via conda..."
    if command -v conda &> /dev/null; then
        if ! conda env list | grep -q "^cors-enhance "; then
            conda env create -f "$ISSUE_DIR/environment.yml"
        fi
        eval "$(conda shell.bash hook)"
        conda activate cors-enhance
    else
        # Manual Go install for non-conda environments
        mkdir -p "$TOOLS_DIR"
        GO_VERSION="1.23.0"
        GO_TAR="go${GO_VERSION}.linux-amd64.tar.gz"
        if [ ! -d "$TOOLS_DIR/go" ]; then
            echo "Downloading Go $GO_VERSION..."
            curl -fsSL "https://go.dev/dl/${GO_TAR}" -o "/tmp/${GO_TAR}"
            tar -C "$TOOLS_DIR" -xzf "/tmp/${GO_TAR}"
            rm -f "/tmp/${GO_TAR}"
        fi
        export GOROOT="$TOOLS_DIR/go"
        export PATH="$GOROOT/bin:$PATH"
        echo "export GOROOT=$GOROOT" >> "$ENV_FILE"
        echo "export PATH=$GOROOT/bin:\$PATH" >> "$ENV_FILE"
    fi
fi

echo "Go version: $(go version)"

# Persist env variables if any
[ -f "$ENV_FILE" ] && grep -qF "source $ENV_FILE" "$HOME/.bashrc" || echo "source $ENV_FILE" >> "$HOME/.bashrc"

# ──────────────────────────────────────────
# 2. Copy init/ to workspace/ (this step must succeed)
# ──────────────────────────────────────────
[ -d "$ISSUE_DIR/workspace" ] && rm -rf "$ISSUE_DIR/workspace"
cp -r "$ISSUE_DIR/init" "$ISSUE_DIR/workspace"

echo "Workspace created at $ISSUE_DIR/workspace"
echo "Run 'cd $ISSUE_DIR/workspace && go mod download' to fetch dependencies"
