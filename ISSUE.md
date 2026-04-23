# Issue: 增强 dbmate status 命令，添加仪表盘式输出和多种格式支持

## 1. 模拟的用户真实请求

> **[用户]** 先大致分析理解这个仓库，这个 dbmate 仓库是一款数据库迁移工具，能够确保多开发者和生产服务器之间的数据库结构保持同步。我希望增强"dbmate status"命令，提供仪表盘式的输出：1. 我希望有能显示已执行迁移总数和待执行迁移总数的功能 2. 可以列出所有迁移及其状态（已执行/待执行） 3. 显示每个迁移的执行时间（如果已执行）

> **[用户]** *(上下文：用户看到初步实现后追加新需求)* 接下来我想继续增加新功能：4. 支持表格格式输出（带对齐和分隔符） 5. 支持JSON格式输出用于自动化。请注意，你需要增强status命令的输出，并且为总共5个功能添加单元测试。不需要图形化的界面，性能监控或报警系统

## 2. 详细问题描述

**影响文件**：`pkg/dbmate/db.go`（`Status` 函数）、`pkg/dbmate/driver.go`（Driver 接口）、`pkg/dbmate/migration.go`（Migration 结构体）、5 个数据库驱动文件

**现象一**：当前 `dbmate status` 命令只输出简单的迁移文件名列表和 `[X]`/`[ ]` 标记，没有已执行/待执行的总数统计，没有进度条，无法一目了然地了解迁移整体状态

**现象二**：没有记录和应用迁移的执行时间戳（`applied_at`），`schema_migrations` 表只有 `version` 列

**现象三**：status 命令只支持一种输出格式，不支持带边框对齐的表格格式，也不支持 JSON 格式输出，无法满足自动化场景（如 CI/CD 管道中解析迁移状态）

**对用户的实际影响**：
- 无法快速了解数据库迁移的整体进度（需要人工数行）
- 无法追踪迁移执行的时间
- 无法在自动化脚本中解析 status 输出

## 3. 根本原因分析

- `pkg/dbmate/db.go` 的 `Status()` 方法输出格式过于简单，只逐行打印迁移名和状态标记
- `pkg/dbmate/driver.go` 的 Driver 接口只有 `SelectMigrations` 返回 `map[string]bool`，不包含时间戳信息
- `pkg/dbmate/migration.go` 的 `Migration` 结构体没有 `AppliedAt` 字段
- 各数据库驱动的 `schema_migrations` 表只有 `version` 列，缺少 `applied_at` 时间戳列
- `main.go` 的 status 命令没有 `--format` 选项

## 4. 期望结果

| 验收编号 | 辅助 verifier | 评测动作（运行什么） | 检查位置（检查哪里） | 通过标准（应得到什么） | 常见失败表现 |
|----------|---------------|----------------------|----------------------|------------------------|--------------|
| A1 | F1 | 编译后执行 `./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations up && ./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status` | 终端输出 | 输出包含 "Total Migrations:"、"Applied:"、"Pending:" 数量统计行，以及 "Progress:" 进度条 | 输出了迁移列表但没有总数统计和进度条 |
| A2 | F2 | 执行 status 命令后回滚一个迁移（`./dbmate rollback`），再执行 status | 终端输出中迁移行 | 已执行迁移标记为 `[X]` 或 `applied`，待执行迁移标记为 `[ ]` 或 `pending` | 所有迁移都显示相同状态，或没有明确的状态标识 |
| A3 | F3 | 执行 `up` 后执行 `status` | 终端输出 "Applied At" 列 | 已执行迁移显示 `YYYY-MM-DD HH:MM:SS` 格式的时间戳，待执行迁移显示 `-` | 已执行迁移的时间戳显示为空或 `(unknown)` |
| A4 | F4 | 执行 `./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status --format table` | 终端输出 | 输出包含 `+---+` 边框行、`|` 竖线分隔符、表头行，以及底部 `Total: N | Applied: N | Pending: N` 汇总行 | 输出不是带边框的表格格式，或 `--format table` 选项不被识别 |
| A5 | F5 | 执行 `./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status --format json` | 终端输出 | 输出为有效 JSON，包含 `total`、`applied`、`pending`、`migrations` 字段；每个 migration 对象包含 `version`、`file_name`、`applied`、`applied_at` 字段；已执行迁移的 `applied_at` 为 RFC3339 格式字符串，待执行为 `null` | JSON 解析失败，缺少必需字段，或时间戳格式不是 RFC3339 |
| A6 | G1 | 执行 `./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations up` 后执行 `rollback` | 终端输出 | 迁移和回滚正常工作，无报错 | 基本迁移功能被破坏 |
| A7 | G2 | 不执行 up，直接运行 `./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status --exit-code` | 终端退出码 | 退出码为 1（有待执行迁移时） | 退出码为 0 |

## 5. 改动方案

核心改动分布在以下文件：

1. **`pkg/dbmate/driver.go`**：新增 `MigrationRecord` 结构体（含 `Version` 和 `AppliedAt` 字段）和 `SelectMigrationsWithDetails` 方法到 Driver 接口
2. **`pkg/dbmate/migration.go`**：`Migration` 结构体新增 `AppliedAt *time.Time` 字段
3. **`pkg/dbmate/db.go`**：
   - `DB` 结构体新增 `StatusFormat string` 字段
   - `FindMigrations()` 使用 `SelectMigrationsWithDetails` 替代 `SelectMigrations` 填充 `AppliedAt`
   - `Status()` 方法重构为格式分发器，根据 `StatusFormat` 调用不同输出方法
   - 新增 `printStatusDashboard()`（默认仪表盘格式）、`printStatusTable()`（表格格式）、`printStatusJSON()`（JSON 格式）
   - 新增 `StatusJSONOutput` 和 `StatusJSONMigration` 结构体
4. **5 个数据库驱动**：各实现 `SelectMigrationsWithDetails`，`CreateMigrationsTable` 添加 `applied_at` 列（带向后兼容的 fallback）
5. **`main.go`**：status 命令新增 `--format` 标志
6. **SQLite 驱动拆分**：`sqlite.go` 移除 CGo 构建标签，新增 `sqlite_cgo.go` 和 `sqlite_nocgo.go` 支持纯 Go SQLite 驱动

## 6. 复现步骤

### 第一步：安装初始环境
```bash
bash $ISSUE_ROOT/reproduce.sh
```

### 第二步：激活环境，运行 init 版本，观察输出
```bash
eval "$(conda shell.bash hook)"
conda activate dbmate-status
cd $ISSUE_ROOT/workspace
export CGO_ENABLED=0
go mod download
go build -o dbmate .

./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations up
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status 2>&1 | tee /tmp/init_output.txt
```

### 第三步：验证初始现象
- status 输出没有 Applied/Pending 总数统计
- status 输出没有 Applied At 时间戳列
- `--format table` 和 `--format json` 不生效或报错
- 可选：运行 `VERIFY_MODE=init ISSUE_DIR=$ISSUE_ROOT python3 $ISSUE_ROOT/tests/test_outputs.py` 执行 P* 测试

### 第四步：验证改动后效果
```bash
# 替换为 final 版本
rm -rf $ISSUE_ROOT/workspace
cp -r $ISSUE_ROOT/final $ISSUE_ROOT/workspace
cd $ISSUE_ROOT/workspace
export CGO_ENABLED=0
go mod download
go build -o dbmate .

# A1: 验证仪表盘计数
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations up
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status 2>&1 | tee /tmp/final_output.txt
# 检查：输出应包含 "Total Migrations:"、"Applied:"、"Pending:"、"Progress:"

# A2: 验证迁移状态列表
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations rollback
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status
# 检查：应有 [X] 和 [ ] 两种状态标记

# A3: 验证执行时间戳
# 检查 status 输出中的 "Applied At" 列，已执行迁移应有时间戳

# A4: 验证表格格式
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status --format table
# 检查：输出应包含 +---+ 边框和 | 分隔符

# A5: 验证 JSON 格式
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status --format json
# 检查：输出应为有效 JSON，含 total/applied/pending/migrations 字段

# A6: 验证非回归（migrate/rollback 正常）
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations up
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations rollback

# A7: 验证 exit-code
rm -f test.db
./dbmate --url "sqlite:test.db" --migrations-dir testdata/db/migrations status --exit-code
echo "Exit code: $?"
# 应返回 1（有待执行迁移）

# 可选：运行自动化 verifier
rm -f test.db
VERIFY_MODE=final ISSUE_DIR=$ISSUE_ROOT python3 $ISSUE_ROOT/tests/test_outputs.py
```

## 7. 元信息

- 仓库：https://github.com/amacneil/dbmate
- Base commit：`2036c594de2cf49e357d14561665f03b7df108c7`（2026-04-04）
- 题型：新功能实现
- 难度：中

## 8. 文件清单

| 文件/目录 | 用途说明 |
|-----------|----------|
| ISSUE.md | Issue 描述、复现步骤、验证方法 |
| reproduce.sh | 一键安装脚本（conda 环境 + workspace 生成） |
| environment.yml | conda 环境定义（Go + Python） |
| run-tests.sh | 自动化验证脚本 |
| changes.diff | unified diff（patch 格式） |
| init/ | base commit 的代码库快照（功能未实现） |
| final/ | 实现功能后的代码库快照 |
| workspace/ | 由 reproduce.sh 自动生成，用于复现 |
| tests/test_outputs.py | 自动化 verifier（P*/F*/G* 测试） |
