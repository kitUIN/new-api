# Responses 请求准备性能

## 优化范围

- `common.DeepCopy` 为 `json.RawMessage` 和 `[]byte` 增加整块复制，避免反射逐字节复制。仍分配独立的字节缓冲区，保留重试隔离以及可选指针字段的显式 `0/false`。
- 禁用字段过滤先扫描顶层字段；不需要过滤时直接返回原请求体。需要过滤时保留其余字段的原始 JSON，避免将大整数转换成 `float64`。
- Responses 工具过滤先检查 `tools`；没有匹配工具时不再解码整份请求到 `RawMessage` map。
- Responses token 元数据直接遍历原始 `input`，避免先复制 `content` 再解码为通用对象。保留已有文本和媒体提取规则，不调整 token 估算公式。

## 本地基准

2026-11-16，Windows amd64，AMD Ryzen AI 7 H 350，Go 1.26.2。同一份合成请求和基准函数，修改前后各运行三次，以下为中位数，单位毫秒。

请求包含 18 个不同的 Base64 资源；资源带 PNG 签名并填充确定性的伪随机字节。大小分别约 2.15 MB、6.45 MB，另有少量 JSON 字段开销。该数据用于测量字节处理和缓存改写，不用于图片解码或模型准确率验证。

| 阶段 | 2.15 MB 优化前 | 2.15 MB 优化后 | 6.45 MB 优化前 | 6.45 MB 优化后 |
| --- | ---: | ---: | ---: | ---: |
| 深拷贝 | 94.22 | 0.15 | 501.03 | 0.54 |
| 禁用字段过滤（无字段需要移除） | 9.43 | 1.42 | 35.57 | 6.77 |
| 工具过滤（无匹配工具） | 14.24 | 2.04 | 36.34 | 6.62 |
| token 元数据提取 | 25.99 | 11.52 | 70.09 | 37.02 |
| 合成准备流程 | 272.97 | 65.74 | 711.89 | 219.42 |

6.45 MB 深拷贝的分配次数由约 645 万次降至 37 次。

合成准备流程包含 token 元数据提取、深拷贝、工具过滤、序列化、禁用字段过滤、18 个资源 URL 改写及 zstd 压缩。每个样本首次填充本地资源缓存，后续迭代复用缓存；不同样本的迭代次数和缓存命中比例不同。该测试不包含入口请求读取/解码、真实 token 计算、数据库/Redis 计费、上游网络、并发压力或线上磁盘争用，不能直接据此推算线上 2.95 秒将降到多少。

复现命令（基准名称中的 `2MB` / `6MB` 对应上表两种大小）：

```sh
go test ./relay -run '^$' -bench '^BenchmarkResponsesPreparation$' -benchmem -benchtime=500ms -count=3
```

## 分阶段日志

现有消费日志的 `other` 和服务端 `relay timing` 日志新增 `prepare_stages_ms`，数值单位为毫秒，保留微秒换算后的小数。原有 `upstream_prepare_ms` 等指标继续使用原来的时间边界。

| 字段 | 范围 |
| --- | --- |
| `request_decode` | 控制器读取/校验 DTO 的耗时，排除首次接收请求体的时间 |
| `token_metadata` | 文本和媒体元数据提取 |
| `sensitive_check` | 启用时的敏感词检查 |
| `token_estimate` | token 估算，包括配置启用时的媒体读取 |
| `pricing` / `preconsume` | 价格计算 / 预扣费 |
| `responses_copy` | Responses 请求深拷贝 |
| `responses_convert` / `responses_marshal` | 渠道转换 / 序列化 |
| `responses_filter` / `param_override` | 字段及工具过滤 / 参数覆盖 |
| `request_snapshot` | 请求日志快照 |
| `request_encode_read` | 压缩前读取内存或暂存请求体 |
| `asset_rewrite` | Base64 解码、资源验证、缓存写入及 URL 替换 |
| `request_compress` | 请求压缩 |
| `request_encode_storage` | 最终请求体暂存 |

这些阶段目前覆盖 HTTP 控制器及 Responses HTTP 准备链路，编码阶段仅在对应功能启用且执行时记录；它们不是所有传输路径的完整追踪。单个请求的阶段值跨重试累计，不包含上游往返等待，不能要求阶段之和等于 `upstream_prepare_ms`。部署后可据此区分 token、计费、资源缓存或压缩是否仍在造成明显延迟。

## 回归验证

```sh
go test ./common ./dto ./relay/... ./service ./controller
```

上述相关后端回归通过；最后补充的字段大小写及重复键兼容性测试也单独通过。覆盖深拷贝缓冲区与指针隔离、显式零值、数字精度、原始字节复用、工具过滤、token 元数据兼容性、耗时累计与日志输出。未部署或验证线上延迟。
