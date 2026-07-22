# DeepSeek Golang Agent Demo

这是一个使用 DeepSeek Function Calling 构建的受约束数据分析 Agent。

## 项目概览

下图集中展示了项目架构、Agent 多轮规划与工具调用的数据流程，以及安全护栏和核心能力。

![DeepSeek Golang Agent 项目架构与数据流程总览](docs/readme-overview.png)

## 核心能力

- 模型规划、工具调用、工具结果回传和再次规划的闭环。
- `record_id` 由服务端上下文绑定，模型无法修改其他记录。
- 严格工具 Schema 与 Go 强类型参数校验。
- 通知目标仅允许服务端配置的别名。
- Webhook 默认仅允许 HTTPS，并拒绝内网、回环和链路本地地址。
- 通知默认需要人工审批，并使用幂等键避免重复发送。
- 持久化 Agent Run、每一步调用、结果、错误和 Token 用量。
- Bearer Token、请求体限制、基础限流、请求超时和优雅停机。

## 启动

```bash
docker run -d --name deepseek-mariadb \
  -e MYSQL_ROOT_PASSWORD=root \
  -e MYSQL_DATABASE=deepseek_demo \
  -e MYSQL_USER=deepseek \
  -e MYSQL_PASSWORD=deepseek123 \
  -p 3306:3306 mariadb:11

cp .env.example .env
# 编辑 .env，至少填写 DEEPSEEK_API_KEY、DB_DSN 和 API_AUTH_TOKEN
go run .
```

默认模型为 `deepseek-v4-flash`，可通过 `DEEPSEEK_MODEL` 修改。

## API

除 `/healthz` 外，配置 `API_AUTH_TOKEN` 后需要：

```http
Authorization: Bearer <token>
```

创建记录：

```bash
curl -X POST http://localhost:8080/api/records \
  -H 'Authorization: Bearer replace_with_a_long_random_token' \
  -H 'Content-Type: application/json' \
  -d '{"type":"text","content":"客户反馈系统持续报错","metadata":{"source":"feedback"}}'
```

运行 Agent：

```bash
curl -X POST http://localhost:8080/api/analyze/1 \
  -H 'Authorization: Bearer replace_with_a_long_random_token'
```

查询记录和运行轨迹：

```bash
curl http://localhost:8080/api/records/1 -H 'Authorization: Bearer replace_with_a_long_random_token'
curl http://localhost:8080/api/runs/1 -H 'Authorization: Bearer replace_with_a_long_random_token'
```

审批通知：

```bash
curl -X POST http://localhost:8080/api/approvals/1/approve \
  -H 'Authorization: Bearer replace_with_a_long_random_token' \
  -H 'Content-Type: application/json' \
  -d '{"reason":"人工确认需要通知"}'
```

拒绝时使用 `/api/approvals/:id/reject`。

## 通知目标

```env
EMAIL_TARGETS_JSON={"support":"support@example.com"}
WEBHOOK_TARGETS_JSON={"ops":"https://example.com/hooks/deepseek"}
```

不要在不了解风险时开启 `ALLOW_HTTP_WEBHOOKS` 或 `ALLOW_PRIVATE_WEBHOOKS`。

## 验证

```bash
gofmt -w .
go vet ./...
go test -race ./...
```
