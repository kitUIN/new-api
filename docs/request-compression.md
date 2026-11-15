# 请求体压缩

客户端请求可使用 `Content-Encoding: gzip`、`br` 或 `zstd`。网关在解析和选择渠道前解压请求，并按 `MAX_REQUEST_BODY_MB` 限制解压后的大小。此入站能力不受渠道压缩开关影响。

渠道编辑页的额外设置提供“不压缩 / gzip 压缩 / zstd 压缩”三选一，默认选择 zstd。发送给上游的 JSON 请求压缩方式保存在渠道 `setting` JSON 的 `request_body_compression` 字段中：

| 配置值 | 行为 |
| --- | --- |
| `none` | 不压缩 |
| `gzip` | 使用 gzip 压缩上游 JSON 请求 |
| `zstd` | 使用 zstd 压缩上游 JSON 请求（默认） |

请根据上游支持的请求解压格式选择。旧渠道显式保存的 `request_body_gzip: true` 映射为 gzip，`false` 映射为不压缩；未配置时使用 zstd。新字段优先于旧字段，编辑保存后使用新字段。

压缩发生在提供商格式转换和资源 URL 替换之后，仅处理至少 1 KiB 且压缩后更小的 JSON 请求。较小或无法缩小的请求保留原始内容。multipart 和 AWS/Tencent 等已签名请求跳过自动压缩。启用自动压缩时，不可同时通过请求头覆盖手动设置 `Content-Encoding`。

压缩后的请求携带相应的 `Content-Encoding` 和正确的内容长度，并支持请求重放。此设置仅控制上游 HTTP 请求体；Responses WebSocket 消息不经过 HTTP 请求体压缩。
