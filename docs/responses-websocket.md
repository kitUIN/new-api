# Responses WebSocket 接入

`GET /v1/responses` 提供 Responses API 的 WebSocket 传输，原有 `POST /v1/responses` HTTP/SSE 接口保持不变。该协议与 `GET /v1/realtime` 的 Realtime 协议相互独立。

## 启用渠道

在 OpenAI、Azure 或 Codex 渠道的设置中启用“Responses WebSocket”，或在渠道 `settings` JSON 中设置：

```json
{
  "supports_responses_websocket": true
}
```

该能力默认开启；旧渠道未保存该字段时也视为开启，只有显式设为 `false` 才会关闭。上游地址会按连接临时从 `https/http` 转换为 `wss/ws`，不会修改渠道保存的 Base URL。

## Codex 配置

```toml
[model_providers.newapi]
name = "new-api"
base_url = "https://example.com/v1"
env_key = "NEW_API_KEY"
wire_api = "responses"
supports_websockets = true
```

## 客户端事件

连接建立后的第一条文本消息必须包含模型：

```json
{
  "type": "response.create",
  "model": "gpt-5.2-codex",
  "input": []
}
```

后续回合可以省略模型并通过 `previous_response_id` 续接：

```json
{
  "type": "response.create",
  "previous_response_id": "resp_123",
  "input": []
}
```

- `stream` 会被移除，WebSocket 传输本身即为流式。
- `background` 不受支持；只要传入该字段就会返回参数错误。
- `generate: false` 用于预热，并保留显式 `false`。
- `stream_id` 及未识别的 WebSocket 信封字段会原样保留。
- 单连接同一时刻只允许一个活动回合；并行请求需建立多个连接。
- 单连接最长保持 60 分钟，到期后客户端需重新建立连接。

握手后的错误使用 Responses WebSocket 事件返回：

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

一个连接可以顺序执行多个回合。`response.completed` 和 `response.incomplete` 会按实际 usage 结算，`response.failed`、终止 `error` 或连接中断会退回尚未结算的预扣额度。每个生成回合单独记录消费日志；零 usage 的 `generate: false` 预热回合不生成消费结算记录。
