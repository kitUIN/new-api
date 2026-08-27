# Responses API WebSocket 支持实施计划

## 1. 目标

让使用自定义模型供应商的 Codex 客户端可以安全启用：

```toml
[model_providers.newapi]
name = "new-api"
base_url = "https://example.com/v1"
env_key = "NEW_API_KEY"
wire_api = "responses"
supports_websockets = true
```

启用后，网关需要接受 `GET /v1/responses` 的 WebSocket 握手，并在同一连接上处理多条 `response.create` 事件。现有的 `POST /v1/responses` HTTP/SSE 行为必须保持不变。

## 2. 协议边界

Responses WebSocket 与 Realtime WebSocket 是两套不同协议，不能共用 Realtime DTO、事件处理或音频计费逻辑。

客户端建立连接后发送的首条消息形态如下：

```json
{
  "type": "response.create",
  "model": "gpt-5.2-codex",
  "input": []
}
```

后续工具调用或用户输入继续通过同一连接发送：

```json
{
  "type": "response.create",
  "previous_response_id": "resp_123",
  "input": []
}
```

实现需要遵守以下约束：

- `stream` 在 WebSocket 模式下是隐式行为，不应发送给上游。
- `background` 不受 WebSocket 模式支持，应返回明确的参数错误。
- 支持 `generate: false` 预热请求。
- 支持 `previous_response_id` 的连接内连续调用。
- 保留 `stream_id` 等 WebSocket 专用字段。
- 单连接只允许一个正在生成的响应；需要并行时由客户端建立多个连接。
- 所有 JSON 编解码使用 `common/json.go` 中的包装函数。
- 可选标量字段使用指针和 `omitempty`，必须保留客户端显式传入的 `0` 和 `false`。

## 3. 当前缺口

当前代码已有以下能力：

- `POST /v1/responses` 的 HTTP/SSE 转发。
- `GET /v1/realtime` 的 WebSocket 转发。
- WebSocket 上游拨号和双向消息转发的基础代码。

仍缺少：

- `GET /v1/responses` 路由。
- Responses WebSocket 专用的 relay format/transport 标记。
- 从第一条 WebSocket 消息读取模型后再执行渠道分发的流程。
- Responses WebSocket 事件 DTO 和校验。
- 对每条 `response.create` 应用模型映射、参数覆盖和字段过滤。
- 明确的渠道能力开关。
- 持久连接中逐响应预扣、结算、退款和消费日志。
- 对应的单元测试、集成测试和 OpenAPI 文档。

## 4. 总体设计

```text
Codex
  │ GET /v1/responses (Upgrade)
  ▼
Responses WebSocket Controller
  │ 读取并校验首条 response.create
  ▼
按首条消息中的 model 选择渠道
  │
  ├─ 上游不支持 Responses WS → 返回 WebSocket error 并关闭
  │
  ▼
建立 wss://upstream/v1/responses
  │
  ├─ Client → 校验/转换 response.create → 预扣费 → Upstream
  └─ Upstream → 解析 usage/状态 → 结算或退款 → 原样转发 Client
```

连接一旦成功发送首个上游响应事件，就固定到当前渠道和上游连接。后续事件不能跨渠道重试，否则上游连接内的 `previous_response_id` 状态会丢失。

## 5. 分阶段实施

### 阶段一：路由与类型

涉及文件：

- `router/relay-router.go`
- `types/relay_format.go`
- `relay/constant/relay_mode.go`
- `relay/common/relay_info.go`

任务：

1. 增加 `GET /v1/responses` 路由，保留现有 `POST /v1/responses`。
2. 增加独立的 Responses WebSocket relay format 或 transport 字段。
3. 不把该请求标记为 `RelayFormatOpenAIRealtime`。
4. 为 `RelayInfo` 增加 WebSocket transport 状态和当前响应状态，复用现有 `ClientWs`、`TargetWs` 连接字段。
5. 使用独立 upgrader；不要声明 `realtime` 子协议。

验收条件：

- HTTP Responses API 无回归。
- `GET /v1/responses` 可以完成标准 WebSocket 握手。
- `/v1/realtime` 行为保持不变。

### 阶段二：首帧读取与渠道分发

涉及文件：

- `controller/relay.go` 或新增 `controller/relay_responses_websocket.go`
- `middleware/distributor.go`
- `relay/helper/valid_request.go`

任务：

1. Token 鉴权、系统状态检查和模型请求限流仍在握手前执行。
2. 握手成功后设置消息大小上限和首帧读取超时。
3. 读取第一条文本消息，要求 `type == "response.create"` 且 `model` 非空。
4. 把 `Distribute` 中“解析请求体”和“根据模型选择渠道”拆开：
   - HTTP middleware 继续从请求体构造 `ModelRequest`。
   - WebSocket controller 直接使用首帧构造的 `ModelRequest`。
5. 渠道选择失败时，发送 Responses 风格的 WebSocket `error` 事件后关闭连接。
6. 首帧只允许读取一次，并保存原始字节供后续转换和转发。

验收条件：

- 模型权限、分组、自动分组、渠道亲和和指定渠道限制与 HTTP 请求一致。
- 缺少模型、模型无权限、无可用渠道时返回稳定的错误事件。
- 首帧不会在分发和转发阶段被重复消费或丢失。

### 阶段三：事件 DTO 与请求转换

涉及文件：

- `dto/openai_request.go` 或新增 `dto/openai_responses_websocket.go`
- `relay/responses_handler.go`
- `relay/common/override.go`

建议新增 DTO：

```go
type OpenAIResponsesWebSocketEvent struct {
    Type     string `json:"type"`
    Generate *bool  `json:"generate,omitempty"`
    StreamID string `json:"stream_id,omitempty"`

    // Responses 请求字段需要与 OpenAIResponsesRequest 保持一致。
}
```

具体实现可以采用“类型化请求 + `map[string]json.RawMessage` 信封”的方式，避免在重新序列化时丢失 WebSocket 专用字段。`json.RawMessage` 只作为类型使用，实际 marshal/unmarshal 必须调用 `common.*`。

每条 `response.create` 都执行：

1. 校验事件类型和连接状态。
2. 继承首帧模型，或校验事件中显式模型与连接模型一致。
3. 执行模型映射。
4. 执行 adaptor 的 Responses 请求转换。
5. 执行禁用字段过滤和参数覆盖。
6. 执行 Responses 工具过滤。
7. 移除 `stream`；拒绝不受支持的 `background`。
8. 把 `type`、`generate`、`stream_id` 合并回最终事件。
9. 记录参数覆盖审计，但不记录完整敏感请求内容。

验收条件：

- `generate: false` 不会被 `omitempty` 丢弃。
- 请求中的显式 `0`、`0.0` 和 `false` 能到达上游。
- `previous_response_id` 和 `stream_id` 原样保留。
- HTTP 和 WebSocket 路径的模型映射、参数覆盖结果一致。

### 阶段四：渠道能力与上游连接

涉及文件：

- `dto/channel_settings.go`
- `relay/channel/api_request.go`
- `relay/channel/openai/adaptor.go`
- 对应的前端渠道设置组件和 i18n 文案

任务：

1. 在渠道设置中增加显式能力开关，例如：

   ```go
   SupportsResponsesWebSocket *bool `json:"supports_responses_websocket,omitempty"`
   ```

2. 默认值为 `true`；缺少该字段或显式设为 `true` 时支持 Responses WebSocket。
3. 仅显式设为 `false` 的渠道禁止进入 WebSocket 转发。
4. 上游地址转换不能原地修改 `ChannelBaseUrl`：
   - `https://` 转为 `wss://`。
   - `http://` 转为 `ws://`。
5. 使用现有请求头构造与 Header Override 逻辑，但禁止透传：
   - 下游 `Authorization`。
   - `Sec-WebSocket-Key`。
   - `Sec-WebSocket-Version`。
   - `Sec-WebSocket-Extensions`。
6. 保留渠道自身的认证头、组织头和管理员配置的 Header Override。
7. 上游连接失败时，在尚未向客户端发送上游事件前允许按现有重试策略更换渠道；一旦响应开始则禁止重试。

验收条件：

- 未声明能力的渠道不会被误用。
- 上游能收到正确的认证信息和 `/v1/responses` 路径。
- 下游凭据不会泄露给上游。
- HTTP 渠道 Base URL 不会因 WebSocket 请求被永久改写。

### 阶段五：持久连接与状态机

建议新增文件：

- `relay/responses_websocket.go`
- `relay/channel/openai/relay_responses_websocket.go`

任务：

1. 建立 client-to-upstream 和 upstream-to-client 两个读取循环。
2. WebSocket 写操作使用单写者或互斥锁，避免并发写入同一连接。
3. 实现状态机：
   - `idle`
   - `warming`
   - `generating`
   - `closing`
4. `generating` 状态下收到新的生成请求时返回错误，不做并行复用。
5. 收到 `response.completed`、`response.failed` 或终止 `error` 后回到 `idle`。
6. 正确转发文本帧、Ping、Pong 和 Close。
7. 设置读取超时、Pong 处理和连接生命周期上限。
8. 客户端断开时关闭上游；上游断开时向客户端发送 Close。
9. 不把 Responses 事件解析为 Realtime 事件，也不调用 Realtime token 统计。

验收条件：

- 一个连接可顺序执行多次 `response.create`。
- 工具调用输出可以通过 `previous_response_id` 在同一连接继续。
- 任意一侧断开后没有遗留 goroutine 或连接。
- `go test -race` 不报告 WebSocket 并发写问题。

### 阶段六：逐响应计费

涉及文件：

- `service/billing.go`
- `service/text_quota.go`
- `relay/responses_websocket.go`
- `relay/common/relay_info.go`

任务：

1. 将一条 `response.create` 定义为一个独立计费回合。
2. 每个生成回合创建独立的计费状态，不能在同一 `RelayInfo.Billing` 上重复结算。
3. 在事件发送给上游前：
   - 估算本次新增输入 token。
   - 计算价格。
   - 执行预扣费。
4. 收到 `response.completed` 时，从 `response.usage` 构造 `dto.Usage`，调用文本 Responses 计费结算。
5. 收到 `response.failed`、不可恢复 `error` 或连接中断时，退款尚未结算的预扣费。
6. `generate: false` 预热回合不预扣生成额度；如果上游返回非零 usage，则按照实际 usage 结算。
7. 每个生成回合独立记录消费日志、首 token 时间和渠道亲和结果。
8. 连接级日志只记录连接建立、关闭和异常，不重复记费。

验收条件：

- 多回合连接会产生正确的逐回合消费记录。
- 失败或断开的回合不会遗留预扣额度。
- 缓存 token、输入 token、输出 token 和内置工具费用与 HTTP Responses 计费一致。
- 余额不足时，在向上游发送新一轮事件前拒绝该回合。

### 阶段七：错误处理、可观测性与安全

任务：

1. 统一 WebSocket 错误事件：

   ```json
   {
     "type": "error",
     "status": 400,
     "error": {
       "code": "invalid_request",
       "message": "..."
     }
   }
   ```

2. 区分握手前 HTTP 错误和握手后 WebSocket 错误。
3. 错误消息附带 request ID，但不包含渠道 Key、Authorization 或完整请求体。
4. 增加指标：
   - 活跃 Responses WebSocket 连接数。
   - 连接时长。
   - 上游拨号失败数。
   - WebSocket 回合数及成功率。
   - WebSocket 到 HTTP 回退数（如果未来实现回退）。
5. 对首帧和后续消息应用统一大小限制。
6. 复核 `CheckOrigin` 策略；API 客户端没有 `Origin` 时应允许，浏览器来源应按部署配置处理。

## 6. 测试计划

### 单元测试

- GET 和 POST `/v1/responses` 路由互不冲突。
- 首帧 `response.create` 解析及模型提取。
- 非法事件、缺少模型和二进制首帧处理。
- `generate: false`、显式零值和 `previous_response_id` 的序列化。
- `https/http` 到 `wss/ws` 的 URL 转换。
- WebSocket 请求头过滤和 Header Override。
- 模型映射、参数覆盖及工具过滤。
- 渠道能力开关默认开启，显式关闭时禁用。
- `response.completed` usage 转换。
- 成功、失败和断开情况下的预扣/结算/退款。

### 集成测试

使用本地 mock WebSocket upstream 覆盖：

1. 单次生成并收到 `response.completed`。
2. 同连接多次生成。
3. 工具调用后使用 `previous_response_id` 继续。
4. `generate: false` 后继续生成。
5. 上游拨号失败并在首事件前切换渠道。
6. 首个上游事件后断开，确认不再切换渠道。
7. 上游返回错误事件。
8. 客户端提前断开。
9. 上游提前断开。
10. 超大消息、读取超时和连接超时。
11. 多连接并发及 race 检查。

### 回归测试

```powershell
go test ./...
go test -race ./relay/... ./controller/... ./middleware/... ./service/...
```

前端如增加渠道能力开关，还需执行：

```powershell
Set-Location web
bun run i18n:lint
bun run build
```

## 7. 文档更新

涉及文件：

- `docs/openapi/relay.json`
- 渠道设置说明文档
- 部署或客户端接入说明

需要记录：

- `GET /v1/responses` 是 WebSocket 端点。
- 支持的客户端事件和服务端事件。
- 单连接单并发限制。
- 渠道必须显式启用 Responses WebSocket 能力。
- Codex 客户端配置示例。
- 不支持 `background`，`stream` 不应发送。

## 8. 交付拆分

建议拆为以下提交，便于审查和回滚：

1. `responses-ws: add route, transport types, and first-frame parsing`
2. `responses-ws: refactor channel distribution for websocket bootstrap`
3. `responses-ws: add upstream capability and request conversion`
4. `responses-ws: implement persistent proxy and lifecycle handling`
5. `responses-ws: add per-response billing and usage accounting`
6. `responses-ws: add tests, metrics, frontend setting, and docs`

## 9. 最终验收标准

- Codex 自定义 provider 启用 `supports_websockets = true` 后可以建立连接。
- 首次请求、工具调用续接和多回合连续请求均成功。
- `previous_response_id` 在同一上游连接内有效。
- HTTP/SSE Responses API 与 Realtime WebSocket 无回归。
- 仅显式支持的渠道可承载 Responses WebSocket。
- 每个生成回合均正确预扣、结算或退款。
- 所有 JSON 操作遵守项目 JSON 包装规则。
- SQLite、MySQL、PostgreSQL 均不需要数据库专用迁移；如未来新增持久字段，必须补齐三数据库兼容方案。
- 后端测试、race 测试、前端构建和 i18n 检查通过。

## 10. 上线策略

1. 服务端功能上线后渠道能力默认开启，不支持的渠道需显式关闭。
2. 使用测试渠道和 Codex 完成多轮工具调用验证。
3. 观察连接失败率、未结算计费回合和 goroutine 数量。
4. 确认不支持 Responses WebSocket 的渠道已显式关闭该能力。
5. 服务端验证稳定后，客户端再配置 `supports_websockets = true`。
