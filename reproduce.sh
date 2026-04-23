#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_DIR="$HOME/.tools"
ENV_FILE="$HOME/.issue_env"

# ──────────────────────────────────────────
# 1. 安装运行环境
# ──────────────────────────────────────────

# conda 环境
if ! conda env list | grep -q "^dbmate-status "; then
    conda env create -f "$ISSUE_DIR/environment.yml"
fi

# 持久化环境变量
touch "$ENV_FILE"
grep -qF "source $ENV_FILE" "$HOME/.bashrc" 2>/dev/null || echo "source $ENV_FILE" >> "$HOME/.bashrc"

# ──────────────────────────────────────────
# 2. 将 init/ 复制为 workspace/（此步必须成功）
# ──────────────────────────────────────────
[ -d "$ISSUE_DIR/workspace" ] && rm -rf "$ISSUE_DIR/workspace"
cp -r "$ISSUE_DIR/init" "$ISSUE_DIR/workspace"
