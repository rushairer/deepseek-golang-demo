# 业务请求方法调用路径说明

本文基于当前代码实现，按“HTTP 请求入口 -> Handler -> Service/Agent -> Store/外部依赖 -> 响应”的顺序，详细说明每个业务请求的调用路径。

## 1. 系统总入口

服务启动入口在 `main.go`，整体装配顺序如下：

1. `config.Load()`：加载环境变量配置。
2. `models.NewDB()`：创建数据库连接。
3. `runDatabaseMigrations()`：执行数据库迁移。
4. `models.NewStore()`：构造数据访问层。
5. `deepseek.NewClient()`：构造 DeepSeek 客户端。
6. `notification.NewSender()`：构造通知发送器。
7. `actions.NewExecutor()`：构造工具执行器。
8. `agent.NewRunner()`：构造 Agent 运行器。
9. `api.NewServer()`：构造 HTTP 服务对象。
10. `server.SetupRoutes()`：注册路由、中间件和处理函数。
11. `http.Server.ListenAndServe()`：开始对外提供接口。

可以理解为：

- `api.Server` 负责接 HTTP 请求。
- `models.Store` 负责数据库读写。
- `agent.Runner` 负责驱动大模型多轮分析。
- `actions.Executor` 负责执行模型发起的工具调用。
- `notification.Sender` 负责实际发邮件或调用 Webhook。

---

## 2. 路由总览

`api.Server.SetupRoutes()` 注册了以下接口：

- `GET /healthz`
- `POST /api/records`
- `GET /api/records/:id`
- `POST /api/analyze/:id`
- `GET /api/runs/:id`
- `POST /api/approvals/:id/approve`
- `POST /api/approvals/:id/reject`

其中 `/api/*` 路由统一经过两层中间件：

1. `authMiddleware(authToken)`
   - 校验 `Authorization: Bearer <token>`。
   - 未配置 `API_AUTH_TOKEN` 时直接放行。
   - 校验失败时直接返回 `401 unauthorized`。

2. `rateLimitMiddleware(rateLimit)`
   - 按客户端 IP 做简单限流。
   - 超过限制后直接返回 `429 rate limit exceeded`。

所以所有业务接口的真实入口都不是直接进 Handler，而是：

```text
HTTP Request
  -> Gin Router
  -> authMiddleware
  -> rateLimitMiddleware
  -> 具体 Handler
```

---

## 3. 公共辅助方法

在看各个请求之前，先说明两个所有 Handler 都会复用的方法：

### 3.1 `parseID()`

适用于带 `:id` 的接口。

调用职责：
- 从路径参数读取 `id`。
- 转成 `int64`。
- 校验必须大于 0。
- 失败时调用 `writeError()` 返回 `400 invalid ID`。

### 3.2 `writeError()`

统一错误输出方法。

调用职责：
- 输出 `{ "error": "..." }` JSON。
- 如果请求头里有 `X-Request-ID`，会原样带回 `requestId`。
- 如果内部有真实错误对象，会打印到标准输出，便于排查。

---

## 4. 各业务请求的调用路径

## 4.1 创建记录：`POST /api/records`

### 4.1.1 调用路径

```text
POST /api/records
  -> authMiddleware
  -> rateLimitMiddleware
  -> Server.handleCreateRecord()
     -> http.MaxBytesReader()
     -> json.Decoder.Decode()
     -> 字段清洗与校验
     -> Store.CreateDataRecord()
        -> INSERT INTO data_records ...
     -> 返回 201 Created
```

### 4.1.2 详细说明

#### 第一步：限制请求体大小
`handleCreateRecord()` 先通过 `http.MaxBytesReader()` 限制请求体，防止超大输入直接打穿服务。

#### 第二步：解析并严格校验 JSON
使用 `json.Decoder` 且开启 `DisallowUnknownFields()`，这意味着：

- 未定义字段会直接报错。
- 请求体必须严格符合 `models.DataRecord` 的结构。

#### 第三步：业务字段校验
Handler 内完成如下业务约束：

- `type` 会被转成小写并去掉首尾空格。
- `type` 只能是：`text`、`metric`、`log`。
- `content` 去空格后不能为空。
- `content` 长度不能超过 `500000`。
- `metadata` 如果存在，必须是合法 JSON。

#### 第四步：写库
通过 `s.store.CreateDataRecord()` 持久化：

- 如果 `metadata` 为空，会自动补成 `{}`。
- 生成 `created_at`、`updated_at`。
- 执行 `INSERT INTO data_records`。
- 回填新记录的 `ID` 和时间戳。

#### 第五步：返回结果
成功时直接返回数据库写入后的 `record` 对象，HTTP 状态码为 `201`。

### 4.1.3 这个请求的核心职责

- 接收原始业务数据。
- 做最外层输入合法性控制。
- 把待分析记录落库，供后续 Agent 分析使用。

---

## 4.2 查询记录详情：`GET /api/records/:id`

### 4.2.1 调用路径

```text
GET /api/records/:id
  -> authMiddleware
  -> rateLimitMiddleware
  -> parseID()
  -> Server.handleGetRecord()
     -> Store.GetRecordDetail()
        -> Store.GetDataRecord()
           -> SELECT ... FROM data_records
        -> SELECT ... FROM analysis_results
        -> SELECT ... FROM tags
        -> SELECT ... FROM notifications
        -> SELECT ... FROM agent_runs
           -> Store.getRun() [循环调用]
              -> SELECT ... FROM agent_runs WHERE id=?
     -> 返回 200 OK
```

### 4.2.2 详细说明

#### 第一步：解析记录 ID
通过 `parseID()` 完成路径参数校验。

#### 第二步：聚合查询详情
`handleGetRecord()` 调用 `s.store.GetRecordDetail()`，它不是单表查询，而是一个聚合读取流程。

`GetRecordDetail()` 内部链路如下：

1. `GetDataRecord()`
   - 先查 `data_records` 主记录。
   - 如果记录不存在，直接返回 `nil`。

2. 查询最近一条分析结果
   - 从 `analysis_results` 按 `record_id` 倒序取最新一条。
   - 反序列化 `suggestions` JSON。

3. 查询标签列表
   - 从 `tags` 表按 `record_id` 查询全部标签。

4. 查询通知记录
   - 从 `notifications` 表查询该记录关联的通知发送历史。

5. 查询最近 20 次 Agent Run
   - 先从 `agent_runs` 查出最近 20 个 `id`。
   - 再对每个 `id` 调用内部方法 `getRun()` 获取完整运行信息。

#### 第三步：返回聚合结果
最终返回 `RecordDetail`：

- 基础记录 `DataRecord`
- 最新分析结果 `Analysis`
- 标签列表 `Tags`
- 通知列表 `Notifications`
- 最近运行列表 `Runs`

### 4.2.3 这个请求的核心职责

- 提供“单条记录全景视图”。
- 让调用方一次拿到记录、分析、标签、通知和运行历史。

---

## 4.3 触发分析：`POST /api/analyze/:id`

这是整个项目最核心的业务请求。

### 4.3.1 顶层调用路径

```text
POST /api/analyze/:id
  -> authMiddleware
  -> rateLimitMiddleware
  -> parseID()
  -> Server.handleAnalyzeData()
     -> Store.GetDataRecord()
     -> Runner.Run()
        -> Store.CreateAgentRun()
        -> prompts.User()
        -> [循环最多 maxSteps 次]
           -> DeepSeek Client.Complete()
           -> Store.AddAgentStep(kind=model)
           -> 分支 A：无工具调用
              -> parseFinal()
              -> Store.SaveAnalysisResult()
              -> Store.FinishAgentRun(status=completed)
           -> 分支 B：有工具调用
              -> Executor.Execute()
              -> Store.AddAgentStep(kind=tool)
              -> 将工具结果作为 tool message 回传模型
        -> 超步数失败时结束
     -> 返回 200 OK / 502
```

### 4.3.2 Handler 层行为

`handleAnalyzeData()` 的逻辑非常清晰：

1. `parseID()` 校验 `id`。
2. `s.store.GetDataRecord()` 读取待分析记录。
3. 记录不存在则返回 `404`。
4. 调用 `s.runner.Run()` 执行完整 Agent 分析流程。
5. 成功返回分析结果和执行结果；失败返回 `502 agent run failed`。

### 4.3.3 `Runner.Run()` 的完整链路

`Runner.Run()` 是分析主流程，负责把“记录数据 -> 模型推理 -> 工具执行 -> 最终分析结果”串起来。

#### 阶段 1：创建运行记录
先调用：

- `store.CreateAgentRun(record.ID, model, promptVersion, maxSteps)`

作用：
- 在 `agent_runs` 表创建一条状态为 `running` 的运行记录。
- 记录模型名、Prompt 版本、最大步数等信息。

#### 阶段 2：构造 Prompt
调用：

- `prompts.User(record)`

作用：
- 把记录的 `type`、`content`、`metadata` 组装为用户消息。
- 与 `prompts.System` 一起构成大模型输入消息列表。

此时初始消息为：

1. `system`: 固定系统约束。
2. `user`: 当前记录内容。

#### 阶段 3：进入多轮 Agent 循环
循环上限由 `maxSteps` 控制。

每一轮会执行以下步骤：

##### 3.3.3.1 调用大模型
调用：

- `client.Complete(ctx, messages, executor.Tools(), maxOutputTokens)`

`deepseek.Client.Complete()` 内部会：

1. 将请求封装成 DeepSeek Chat Completion JSON。
2. 附带工具定义 `executor.Tools()`。
3. 强制要求 `response_format = json_object`。
4. 通过 HTTP POST 调用 `baseURL + /chat/completions`。
5. 解析模型响应。

##### 3.3.3.2 记录模型步
不管模型返回什么，都会先把当前模型输出持久化：

- `store.AddAgentStep(kind=model)`

这里会把完整 `choice.Message` 存入 `agent_steps` 表，作为审计轨迹。

##### 3.3.3.3 分支判断：模型是否调用工具

###### 分支 A：没有工具调用
如果 `choice.Message.ToolCalls` 为空，说明模型已经给出最终结论。

后续链路：

```text
choice.Message.Content
  -> parseFinal()
  -> Store.SaveAnalysisResult()
  -> Store.FinishAgentRun(status=completed)
  -> 返回 Result
```

其中：

- `parseFinal()`：
  - 严格解析模型输出 JSON。
  - 校验 `analysis` 非空。
  - 校验 `confidence` 在 `0~1`。
  - 补齐空的 `suggestions`。

- `SaveAnalysisResult()`：
  - 把分析文本、建议列表、置信度写入 `analysis_results` 表。

- `FinishAgentRun()`：
  - 把 `agent_runs.status` 更新为 `completed`。
  - 写入步数、token 使用量、完成时间。

###### 分支 B：存在工具调用
如果模型返回了 `tool_calls`，说明模型想先执行操作，再继续推理。

后续链路：

```text
for each tool call
  -> Executor.Execute()
  -> Store.AddAgentStep(kind=tool)
  -> 把执行结果包装成 tool message
  -> 追加回 messages
  -> 下一轮继续调用模型
```

每次工具执行结果都会写入 `agent_steps`，因此整个 Agent 推理过程可完整回放。

#### 阶段 4：失败兜底
如果 `Run()` 中途报错：

- defer 会调用 `store.FinishAgentRun(status=failed)`。
- 把错误信息记录到 `agent_runs.error_message`。

如果超过 `maxSteps` 还没完成，会直接返回：

- `agent exceeded maximum of N steps`

### 4.3.4 `Executor.Execute()` 的工具分支

模型当前只能调用 3 个工具：

- `update_record_status`
- `add_record_tag`
- `send_notification`

#### 工具 1：`update_record_status`

调用路径：

```text
Executor.Execute()
  -> decode(arguments)
  -> 校验 status 枚举
  -> store.UpdateStatus(recordID, status)
     -> UPDATE data_records SET metadata=JSON_SET(...)
  -> 返回 succeeded
```

作用：
- 把记录状态写入 `data_records.metadata.status`。
- 这里只更新当前记录，`recordID` 由服务端绑定，模型不能越权指定别的记录。

#### 工具 2：`add_record_tag`

调用路径：

```text
Executor.Execute()
  -> decode(arguments)
  -> strings.TrimSpace(tag)
  -> 校验 tag 长度
  -> store.AddTag(recordID, tag)
     -> INSERT INTO tags ... ON DUPLICATE KEY UPDATE
  -> 返回 succeeded
```

作用：
- 给当前记录打标签。
- 利用数据库唯一约束/幂等写法避免重复标签问题。

#### 工具 3：`send_notification`

这是工具里链路最长的一个。

##### 场景 A：需要审批
如果 `requireApproval = true`，调用路径为：

```text
Executor.Execute()
  -> decode(arguments)
  -> notifier.HasTarget(channel, target)
  -> store.CreateApproval()
     -> INSERT INTO approval_requests ... status='pending'
  -> 返回 pending_approval + approvalId
```

作用：
- 不直接发送通知。
- 先创建审批单，等待人工通过 `/api/approvals/:id/approve` 或 `/reject` 处理。

##### 场景 B：无需审批，直接发送
如果 `requireApproval = false`，调用路径为：

```text
Executor.Execute()
  -> e.send()
     -> store.CreateNotification()
        -> INSERT IGNORE INTO notifications ... status='pending'
     -> notifier.Send()
        -> sendEmail() / sendWebhook()
     -> store.UpdateNotificationStatus(status=sent/failed)
  -> 返回 succeeded/failed
```

`e.send()` 内部还包含两个关键机制：

1. 幂等键：
   - 格式为 `run:<runID>:tool:<toolCallID>`。
   - 避免同一工具调用重复发通知。

2. 发送状态追踪：
   - 先写 `notifications`。
   - 发送成功后更新为 `sent`。
   - 发送失败则更新为 `failed`。

### 4.3.5 `notification.Sender` 的底层路径

#### 邮件通知

```text
Sender.Send(channel=email)
  -> Sender.sendEmail()
     -> net.Dialer.DialContext()
     -> smtp.NewClient()
     -> 可选 STARTTLS
     -> 可选 Auth
     -> Mail / Rcpt / Data / Quit
```

#### Webhook 通知

```text
Sender.Send(channel=webhook)
  -> Sender.sendWebhook()
     -> url.Parse()
     -> validateWebhookURL()
        -> 校验 scheme
        -> 校验 host
        -> DNS 解析
        -> 拒绝私网/回环/本地地址
     -> http.NewRequestWithContext()
     -> http.Client.Do()
```

这样设计的目的是把“是否允许发”“往哪里发”“怎么发”都牢牢控制在服务端配置里，而不是交给模型决定。

### 4.3.6 这个请求的核心职责

- 读取一条业务记录。
- 运行受约束的 Agent 分析流程。
- 持久化分析轨迹、工具调用轨迹和最终结果。
- 必要时触发标签、状态、通知等副作用。

---

## 4.4 查询运行轨迹：`GET /api/runs/:id`

### 4.4.1 调用路径

```text
GET /api/runs/:id
  -> authMiddleware
  -> rateLimitMiddleware
  -> parseID()
  -> Server.handleGetRun()
     -> Store.GetAgentRun()
        -> Store.getRun()
           -> SELECT ... FROM agent_runs
        -> SELECT ... FROM agent_steps ORDER BY id
     -> 返回 200 OK
```

### 4.4.2 详细说明

`handleGetRun()` 的主要工作是取回一条 Agent Run 的完整执行轨迹。

`Store.GetAgentRun()` 内部分两步：

1. `getRun()`
   - 查 `agent_runs` 主表。
   - 取到这次运行的状态、token、错误信息、完成时间等。

2. 查询 `agent_steps`
   - 按 `run_id` 和 `id` 顺序取出所有步骤。
   - 每一步可能是：
     - `kind=model`
     - `kind=tool`

最终返回：

```json
{
  "run": {...},
  "steps": [...]
}
```

### 4.4.3 这个请求的核心职责

- 提供 Agent 运行审计能力。
- 方便排查模型到底做了什么、何时调用了什么工具、工具是否成功。

---

## 4.5 审批通过：`POST /api/approvals/:id/approve`

这是通知审批链路的“放行”接口。

### 4.5.1 调用路径

```text
POST /api/approvals/:id/approve
  -> authMiddleware
  -> rateLimitMiddleware
  -> parseID()
  -> Server.handleApprove()
     -> ShouldBindJSON()
     -> Store.GetApproval()
        -> SELECT ... FROM approval_requests
     -> Executor.ExecuteApprovedNotification()
        -> decode(approval.Arguments)
        -> Executor.send()
           -> Store.CreateNotification()
           -> notifier.Send()
              -> sendEmail() / sendWebhook()
           -> Store.UpdateNotificationStatus()
     -> Store.DecideApproval(status=approved, reason)
        -> UPDATE approval_requests SET ... WHERE status='pending'
     -> 返回 200 OK
```

### 4.5.2 详细说明

#### 第一步：取审批单
`handleApprove()` 先调用 `store.GetApproval()`：

- 查 `approval_requests`。
- 不存在返回 `404`。
- 如果状态不是 `pending`，返回 `409`。

#### 第二步：执行已批准通知
调用：

- `executor.ExecuteApprovedNotification()`

内部做的事情：

1. 从审批单 `Arguments` 中解析出原始通知参数。
2. 调用 `e.send()` 真正发通知。
3. 完整经过通知落库、幂等判断、发送、状态更新流程。

#### 第三步：更新审批状态
只有通知发送成功后，才继续调用：

- `store.DecideApproval(id, "approved", reason)`

这样会把审批单状态改成 `approved` 并记录决策理由。

#### 第四步：返回执行结果
成功时返回的是通知执行结果，而不是简单的 `{status: approved}`。

### 4.5.3 这个请求的核心职责

- 把“待审批通知”转换成“已实际发送通知”。
- 在审批通过后补写审批决策结果。

---

## 4.6 审批拒绝：`POST /api/approvals/:id/reject`

### 4.6.1 调用路径

```text
POST /api/approvals/:id/reject
  -> authMiddleware
  -> rateLimitMiddleware
  -> parseID()
  -> Server.handleReject()
     -> ShouldBindJSON()
     -> Store.DecideApproval(status=rejected, reason)
        -> UPDATE approval_requests SET ... WHERE status='pending'
     -> 返回 200 OK
```

### 4.6.2 详细说明

这个接口链路比 approve 简单很多：

- 不查通知发送结果。
- 不做任何外部通知动作。
- 只把审批单从 `pending` 改成 `rejected`。

如果审批单已被处理，再次拒绝会因为 `WHERE status='pending'` 不满足而失败，最终返回 `409`。

### 4.6.3 这个请求的核心职责

- 显式阻止待审批通知的实际发送。
- 给审批链路留下拒绝审计记录。

---

## 5. 非业务但常用接口：`GET /healthz`

### 5.1 调用路径

```text
GET /healthz
  -> Gin Router
  -> inline handler
  -> 返回 {"status":"ok"}
```

### 5.2 说明

这个接口没有经过认证和限流中间件，也不访问数据库，不调用任何业务逻辑，主要用于：

- 健康检查
- 容器探针
- 负载均衡存活检测

---

## 6. 从架构角度看，请求是如何分层流转的

为了更容易记忆，可以把项目中的请求调用路径归纳为 4 层：

### 6.1 接入层：`api`

代表方法：
- `SetupRoutes()`
- `handleCreateRecord()`
- `handleGetRecord()`
- `handleAnalyzeData()`
- `handleGetRun()`
- `handleApprove()`
- `handleReject()`

职责：
- 接 HTTP 请求。
- 解析参数。
- 做基础输入校验。
- 调用下游服务。
- 组织 HTTP 响应。

### 6.2 核心编排层：`agent` / `actions`

代表方法：
- `Runner.Run()`
- `Executor.Execute()`
- `Executor.ExecuteApprovedNotification()`
- `Executor.send()`

职责：
- 驱动模型多轮执行。
- 执行模型工具调用。
- 协调审批与通知行为。

### 6.3 数据访问层：`models.Store`

代表方法：
- `CreateDataRecord()`
- `GetRecordDetail()`
- `CreateAgentRun()`
- `AddAgentStep()`
- `SaveAnalysisResult()`
- `CreateApproval()`
- `DecideApproval()`
- `CreateNotification()`

职责：
- 负责所有数据库读写。
- 把业务状态变更落到具体表结构上。

### 6.4 外部集成层：`deepseek` / `notification`

代表方法：
- `Client.Complete()`
- `Sender.Send()`
- `sendEmail()`
- `sendWebhook()`

职责：
- 调用 DeepSeek 模型。
- 发送邮件或 Webhook。
- 控制外部访问边界与安全限制。

---

## 7. 一张图看懂最核心请求

项目里最复杂的请求是 `POST /api/analyze/:id`，其主链路可以简化成：

```text
HTTP 请求
  -> API Handler
  -> 读取 record
  -> 创建 agent_run
  -> 组装 prompt
  -> 调用 DeepSeek
  -> 记录 model step
  -> 如果要调工具：执行工具并记录 tool step
  -> 把工具结果回传模型
  -> 循环直到输出最终 JSON
  -> 保存 analysis_result
  -> 完成 agent_run
  -> 返回 run + analysis + executions
```

这个接口本质上是整个系统的“业务编排中心”。

---

## 8. 总结

如果按请求的重要程度排序，这个项目的业务主链路可以理解为：

1. `POST /api/records`
   - 录入待分析业务数据。
2. `POST /api/analyze/:id`
   - 驱动 Agent 对业务数据做分析与动作执行。
3. `GET /api/records/:id`
   - 汇总查看单条业务记录的完整状态。
4. `GET /api/runs/:id`
   - 审计一次 Agent 执行过程。
5. `POST /api/approvals/:id/approve|reject`
   - 管理通知审批闭环。

其中最值得重点关注的是：

- `api.handleAnalyzeData()`：业务请求入口。
- `agent.Runner.Run()`：核心编排。
- `actions.Executor.Execute()`：工具执行中心。
- `models.Store`：持久化中心。
- `notification.Sender`：外部通知出口。

如果你后续需要，我还可以继续补一版“按源码文件逐个解释的方法关系图”或者“带 Mermaid 时序图的版本”。
