# Issue: 为 chi 路由框架实现增强的请求 ID 中间件

## 1. 模拟的用户真实请求

> **[用户]** 看下这个仓库的 README.md 文件，这是 chi 仓库，但是该框架目前没有内置的请求 ID 中间件。请你帮我实现一个请求 ID 中间件，为每个传入的 HTTP 请求自动生成唯一的 ID，或者从请求头读取现有 ID。

> **[用户]** *(上下文：看到初始实现后)* 我对 request-id 中间件还有需求：将请求 ID 添加到响应头中，看看目前是否满足或者是否需要修改。

> **[用户]** *(上下文：确认响应头需求后)* 我还有个需求：将请求头 ID 存储在 request context 中，这样方便后续 handler 访问。帮我看看是否满足这个需求。

> **[用户]** *(上下文：最终汇总需求)* 好的，那我总结一下所有的需求：
> 1. 检查请求头中的 `X-Request-ID`。如果存在，则使用该值
> 2. 如果不存在，生成一个新的 UUID 作为请求 ID
> 3. 将请求 ID 添加到响应头中
> 4. 将请求 ID 存储在 request context 中，方便后续 handler 访问
>
> 需要实现：RequestID 中间件函数；支持自定义的请求 ID 头名称；提供 context key 用于访问请求 ID；添加单元测试

## 2. 详细问题描述

- **影响文件**：`middleware/request_id.go`，`middleware/request_id_test.go`
- **现象一**：原始 `RequestID` 中间件不会将请求 ID 写入响应头，下游服务无法从响应中获取请求 ID 进行追踪
- **现象二**：不支持自定义 ID 生成器，无法注入 UUID 等策略
- **现象三**：不支持多请求头回退查找，在分布式追踪场景中无法从 `X-Trace-Id`、`X-Correlation-Id` 等头部获取已有 ID
- **对用户的实际影响**：无法实现完整的请求追踪链路，且无法灵活配置请求 ID 的生成和读取策略

## 3. 根本原因分析

原始 `RequestID` 中间件（第67-79行）仅做了两件事：
1. 从 `X-Request-Id` 请求头读取或自动生成 ID
2. 将 ID 存入 request context

缺少以下能力：
- 不将 ID 写入响应头（缺少 `w.Header().Set` 调用）
- 不支持自定义 ID 生成器（硬编码了 hostname/random-counter 格式）
- 不支持多请求头回退查找（只检查一个固定头）
- 不支持通过配置结构体灵活控制行为

## 4. 期望结果

| 验收编号 | 辅助 verifier | 评测动作（运行什么） | 检查位置（检查哪里） | 通过标准（应得到什么） | 常见失败表现 |
|----------|---------------|----------------------|----------------------|------------------------|--------------|
| A1 | F1 | 在 workspace 运行 `go test ./middleware/ -run TestRequestID -v -count=1` | 终端输出 | 所有测试通过，且输出包含 `PASS`；测试验证了从请求头读取已有 ID、自动生成 ID、自定义头名等功能 | 测试失败或缺少测试覆盖 |
| A2 | F2 | 在 workspace 运行 `go test ./middleware/ -run TestRequestIDWithConfig_ResponseHeader -v -count=1` | 终端输出 | 测试通过；验证了请求 ID 被写入响应头，且响应头值与 context 中值一致 | 响应头中无请求 ID，或与 context 值不一致 |
| A3 | F3 | 在 workspace 运行 `go test ./middleware/ -run TestRequestIDWithConfig_CustomGenerator -v -count=1` | 终端输出 | 测试通过；验证了可通过配置传入自定义 ID 生成函数 | 不支持自定义生成器 |
| A4 | F4 | 在 workspace 运行 `go test ./middleware/ -run TestRequestIDWithConfig_ExtraHeaders -v -count=1` | 终端输出 | 测试通过；验证了多请求头回退查找优先级：主头 > 第一额外头 > 第二额外头 > 自动生成 | 回退顺序不正确或无法从额外头读取 |
| A5 | F5 | 在 workspace 运行 `go test ./middleware/ -run TestGetReqID -v -count=1` | 终端输出 | 测试通过；验证了从 context 读取请求 ID 的辅助函数，包括 nil context 和无 ID context 的边界处理 | GetReqID 对边界情况处理不正确 |

## 5. 改动方案

### `middleware/request_id.go`

**改动1：默认 RequestID 中间件添加响应头写入**

```diff
+		w.Header().Set(RequestIDHeader, requestID)
 		ctx = context.WithValue(ctx, RequestIDKey, requestID)
```

在将请求 ID 存入 context 之前，先将 ID 写入响应头。

**改动2：新增 RequestIDGeneratorFunc 类型**

```go
type RequestIDGeneratorFunc func(r *http.Request) string
```

**改动3：新增 RequestIDConfig 配置结构体**

```go
type RequestIDConfig struct {
    Header         string
    ExtraHeaders   []string
    Generator      RequestIDGeneratorFunc
    ResponseHeader bool
}
```

**改动4：新增 RequestIDWithConfig 函数**

支持通过配置自定义：请求头名、额外回退头、自定义生成器、是否写响应头。

### `middleware/request_id_test.go`

新增 9 个测试用例覆盖所有新功能，加上 2 个边界测试。

## 6. 复现步骤

### 第一步：安装初始环境

```bash
bash $ISSUE_ROOT/reproduce.sh
```

### 第二步：激活环境，运行 init 版本测试，观察输出

```bash
cd $ISSUE_ROOT/workspace
go test ./middleware/ -run "TestRequestID|TestGetReqID" -v -count=1 2>&1 | tee /tmp/init_output.txt
```

### 第三步：验证初始现象

在 init 版本中：
- `TestRequestID` 测试通过（原有功能正常）
- 不存在 `TestRequestIDWithConfig_*` 系列测试（新功能未实现）
- 原始 `RequestID` 中间件不将 ID 写入响应头

可验证 init 不将 ID 写入响应头：

```bash
cd $ISSUE_ROOT/workspace
# 运行原有测试，检查响应头未被设置
go test ./middleware/ -run TestRequestID -v -count=1
# 在 init 版本中，ResponseHeader 行为未被测试（因为该功能不存在）
```

### 第四步：验证改动后效果

```bash
# 将 final 中的改动文件覆盖到 workspace
cp $ISSUE_ROOT/final/middleware/request_id.go $ISSUE_ROOT/workspace/middleware/request_id.go
cp $ISSUE_ROOT/final/middleware/request_id_test.go $ISSUE_ROOT/workspace/middleware/request_id_test.go

# 按 A1-A5 逐条验收
cd $ISSUE_ROOT/workspace

# A1: 基本功能测试
go test ./middleware/ -run TestRequestID -v -count=1

# A2: 响应头写入测试
go test ./middleware/ -run TestRequestIDWithConfig_ResponseHeader -v -count=1

# A3: 自定义生成器测试
go test ./middleware/ -run TestRequestIDWithConfig_CustomGenerator -v -count=1

# A4: 多请求头回退测试
go test ./middleware/ -run TestRequestIDWithConfig_ExtraHeaders -v -count=1

# A5: GetReqID 边界测试
go test ./middleware/ -run TestGetReqID -v -count=1

# 或一次性运行所有测试
bash $ISSUE_ROOT/run-tests.sh
```

## 7. 元信息

- 仓库：https://github.com/Captain-SOAP-GaoHan/gochi.git
- Base commit：`a54874f0e2f12647a19e82ee70dfa8185014100c`（2025-04-23）
- 题型：新功能实现
- 难度：中

## 8. 文件清单

| 文件/目录           | 用途说明                     |
| --------------- | ------------------------ |
| ISSUE.md        | Issue 描述、复现步骤、验证方法       |
| reproduce.sh    | 一键安装脚本                   |
| environment.yml | conda 环境定义               |
| run-tests.sh    | 自动化测试脚本                  |
| changes.diff    | unified diff             |
| init/           | base commit 的代码库快照       |
| final/          | 实现功能后的代码库快照              |
| workspace/      | 由 reproduce.sh 自动生成，用于复现 |
