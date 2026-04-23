#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 激活 conda 环境
eval "$(conda shell.bash hook)"
conda activate dbmate-status

cd "$ISSUE_DIR/workspace"

# 下载依赖（非 CGo 模式，不需要 GCC）
export CGO_ENABLED=0
go mod download

# 编译
go build -o dbmate .

# 运行验证测试
python "$ISSUE_DIR/tests/test_outputs.py"
