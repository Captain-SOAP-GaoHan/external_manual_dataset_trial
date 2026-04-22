# Issue: 增强 chi 框架的 CORS 中间件

## 1. 模拟的用户真实请求

> **[用户]** 这个仓库是 chi 框架，chi 框架的 CORS 可以增强，已提供更灵活的配置。我希望增强 chi 的 CORS 中间件，添加以下功能：
> 1. 支持基于请求来源的动态 CORS 策略（白名单）
> 2. 支持条件性处理 preflight 请求
> 3. 提供凭证支持配置选项
> 4. 支持自定义暴露的头部列表
> 5. 支持可配置的缓存时间（Max-Age）
>
> 注意：你需要做的：增强 CORS 中间件功能，支持多种配置选项，添加单元测试。你不需要做：与认证系统的集成、性能基准测试、文档生成。你可以新增文件，也可以在已有文件的基础上修改。

## 2. 详细问题描述

- 影响文件：`middleware/` 目录，当前不存在 CORS 中间件
- 现象一：chi 的 `middleware` 包中没有内置 CORS 中间件，开发者只能依赖第三方库（如 `rs/cors`）或手动设置 CORS 头
- 现象二：`middleware/route_headers.go` 中的注释示例使用了外部 `cors.Handler`，说明项目承认需要 CORS 能力但未内置
- 现象三：没有动态来源策略（如基于子域名匹配）的能力，只有静态配置
- 对用户的实际影响：开发者无法通过 chi 原生中间件实现灵活的 CORS 策略，必须引入额外依赖

## 3. 根本原因分析

- chi 的 `middleware` 包没有提供 CORS 中间件实现
- `middleware/route_headers.go` 第 23-41 行的注释示例使用 `cors.Handler(cors.Options{...})` 引用外部库，而非本仓库内置
- 缺少以下关键能力：
  - 动态来源匹配（如 `AllowOriginFunc`）
  - 条件性 preflight 处理（可选择自动处理或透传）
  - 凭证支持（`Access-Control-Allow-Credentials`）
  - 暴露头部配置（`Access-Control-Expose-Headers`）
  - 缓存时间配置（`Access-Control-Max-Age`）

## 4. 期望结果

| 验收编号 | 辅助 verifier | 评测动作（运行什么） | 检查位置（检查哪里） | 通过标准（应得到什么） | 常见失败表现 |
|----------|---------------|----------------------|----------------------|------------------------|--------------|
| A1 | P1 | 在 workspace 中检查 `middleware/cors.go` 是否存在 | 文件系统 | `middleware/cors.go` 文件存在 | 文件不存在 |
| A2 | F1 | 在 workspace 中运行 `go test ./middleware/ -run TestCORS_AllowedOrigins -v` | 终端输出 | 测试通过：白名单中的来源获得 `Access-Control-Allow-Origin` 头，不在白名单中的来源不获得该头 | 测试失败或来源判断不正确 |
| A3 | F2 | 在 workspace 中运行 `go test ./middleware/ -run TestCORS_AllowOriginFunc -v` | 终端输出 | 测试通过：动态函数匹配的来源获得 CORS 头，不匹配的不获得；动态函数优先于静态白名单 | 动态函数未生效或优先级不对 |
| A4 | F3 | 在 workspace 中运行 `go test ./middleware/ -run TestCORS_Preflight -v` | 终端输出 | 测试通过：`HandlePreflight=true` 时 OPTIONS 请求返回 204 并设置 `Access-Control-Allow-Methods`、`Allow-Headers` 等；`HandlePreflight=false` 时 OPTIONS 透传给下游 | preflight 响应码或头部不正确 |
| A5 | F4 | 在 workspace 中运行 `go test ./middleware/ -run TestCORS_Preflight/preflight_with_credentials -v` | 终端输出 | 测试通过：`AllowCredentials=true` 时响应包含 `Access-Control-Allow-Credentials: true` | 凭证头缺失或值不对 |
| A6 | F5 | 在 workspace 中运行 `go test ./middleware/ -run TestCORS_ExposeHeadersOnNormalRequest -v` 和 `TestCORS_MaxAgeNotSetWhenZero -v` | 终端输出 | 测试通过：`ExposedHeaders` 配置的头部出现在 `Access-Control-Expose-Headers` 中；`MaxAge>0` 时 preflight 响应包含 `Access-Control-Max-Age` | 暴露头部或缓存时间头部缺失 |
| A7 | F6 | 在 workspace 中运行 `go test ./middleware/ -run TestCORS_DisallowedOriginNoCORSHeaders -v` | 终端输出 | 测试通过：不允许的来源不获得任何 `Access-Control-*` 头 | 禁止来源错误地获得了 CORS 头 |
| A8 | G1 | 在 workspace 中运行 `go test ./... -v` | 终端输出 | 仓库所有原有测试仍然通过，CORS 功能未破坏现有功能 | 有原有测试失败 |

改动后终端输出样例（运行 `go test ./middleware/ -run TestCORS -v`）：
```
=== RUN   TestCORS_AllowedOrigins
--- PASS: TestCORS_AllowedOrigins (0.00s)
=== RUN   TestCORS_AllowOriginFunc
--- PASS: TestCORS_AllowOriginFunc (0.00s)
=== RUN   TestCORS_Preflight
--- PASS: TestCORS_Preflight (0.00s)
...
PASS
ok      github.com/go-chi/chi/v5/middleware
```

## 5. 改动方案

### 新增文件

**`middleware/cors.go`**：CORS 中间件核心实现
- 定义 `CORSConfig` 结构体，包含 `AllowedOrigins`、`AllowOriginFunc`、`AllowedMethods`、`AllowedHeaders`、`ExposedHeaders`、`AllowCredentials`、`MaxAge`、`HandlePreflight` 字段
- 实现 `CORS(cfg CORSConfig) func(next http.Handler) http.Handler` 中间件函数
- 支持 `AllowedOrigins` 白名单精确匹配和 `"*"` 通配
- 支持 `AllowOriginFunc` 动态来源判断（优先于白名单）
- 支持 `HandlePreflight` 控制是否自动处理 OPTIONS preflight
- 支持 `AllowCredentials` 设置凭证头
- 支持 `ExposedHeaders` 设置暴露头部
- 支持 `MaxAge` 设置 preflight 缓存时间
- 当 `AllowCredentials=true` 且使用通配符来源时，回退到具体 Origin 值（符合 CORS 规范）

**`middleware/cors_test.go`**：13 个测试用例，覆盖所有配置选项

**`_examples/cors/main.go`**：CORS 使用示例

### 修改文件

**`middleware/route_headers.go`**：将 CORS 示例注释从外部 `cors.Handler` 改为使用内置 `middleware.CORS`

**`_examples/README.md`**：添加 cors 示例入口

## 6. 复现步骤

### 第一步：安装初始环境
```bash
bash $ISSUE_ROOT/reproduce.sh
```

### 第二步：激活环境，运行 init 状态，观察输出
```bash
cd $ISSUE_ROOT/workspace
go mod download
go test ./middleware/ -v 2>&1 | tee /tmp/init_output.txt
```

### 第三步：验证初始现象
- 运行 `ls middleware/cors.go`，应显示文件不存在
- 运行 `grep -r "func CORS" middleware/`，应无匹配结果
- 运行 `go test ./middleware/ -v`，所有原有测试通过，但没有 CORS 相关测试

### 第四步：验证改动后效果
```bash
# 将 final/ 的改动应用到 workspace
cp -r $ISSUE_ROOT/final/middleware/cors.go $ISSUE_ROOT/workspace/middleware/
cp -r $ISSUE_ROOT/final/middleware/cors_test.go $ISSUE_ROOT/workspace/middleware/
cp -r $ISSUE_ROOT/final/middleware/route_headers.go $ISSUE_ROOT/workspace/middleware/
cp -r $ISSUE_ROOT/final/_examples/ $ISSUE_ROOT/workspace/_examples/

# 按 A1-A8 逐条验证
ls $ISSUE_ROOT/workspace/middleware/cors.go                           # A1: 文件存在
go test ./middleware/ -run TestCORS_AllowedOrigins -v                 # A2: 白名单测试通过
go test ./middleware/ -run TestCORS_AllowOriginFunc -v                # A3: 动态策略测试通过
go test ./middleware/ -run TestCORS_Preflight -v                      # A4: preflight 测试通过
go test ./middleware/ -run TestCORS_Preflight/preflight_with_credentials -v  # A5: 凭证测试通过
go test ./middleware/ -run TestCORS_ExposeHeadersOnNormalRequest -v   # A6: 暴露头部测试通过
go test ./middleware/ -run TestCORS_DisallowedOriginNoCORSHeaders -v  # A7: 禁止来源测试通过
go test ./... -v                                                      # A8: 全部测试通过

# 若提供了 run-tests.sh，作为辅助验证
bash $ISSUE_ROOT/run-tests.sh
```

## 7. 元信息

- 仓库：https://github.com/go-chi/chi
- Base commit：`a54874f0e2f12647a19e82ee70dfa8185014100c`（2026-02-19）
- 题型：新功能实现
- 难度：中

## 8. 文件清单

| 文件/目录 | 用途说明 |
|-----------|----------|
| ISSUE.md | Issue 描述、复现步骤、验证方法 |
| reproduce.sh | 一键安装脚本 |
| environment.yml | conda 环境定义（Go 1.23） |
| run-tests.sh | 自动化验证脚本 |
| changes.diff | unified diff |
| tests/verifier.go | 独立 verifier 测试程序 |
| init/ | base commit 的代码库快照（无 CORS 中间件） |
| final/ | 实现功能后的代码库快照（含 CORS 中间件） |
| workspace/ | 由 reproduce.sh 自动生成，用于复现 |
