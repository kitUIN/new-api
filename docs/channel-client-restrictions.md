# 渠道客户端限制

在渠道新增或编辑页面的「高级设置」中开启「禁止 Claude Code 客户端」，即可限制该渠道供 Claude Code 使用。默认关闭，适用于所有渠道类型。

配置保存在现有渠道 `settings` JSON 的 `disable_claude_code` 布尔字段中，无需数据库迁移。例如：

```json
{"disable_claude_code": true}
```

- 自动选路、重试、自动分组、组合分组及令牌模型组合会跳过受限渠道，继续寻找允许的渠道。全部不可用时沿用无可用渠道错误。
- 渠道亲和命中受限渠道时，重新选择允许的渠道。
- 显式指定渠道（包括旧版固定渠道组合）时，返回 HTTP 403，错误码为 `claude_code_not_allowed`。
- 关闭开关后恢复正常使用，不影响其他渠道。

识别依据为请求 User-Agent 中的 `claude-cli`、`claude-code` 或 `claudecode` 产品标识（不区分大小写）；也识别同时携带 `X-App: cli` 和 `Anthropic-Beta: claude-code-*` 的请求。普通 Anthropic SDK、只有通用 CLI 标识或只有 beta 头的请求不会因此被拦截，不检查用户消息内容。

这是基于客户端自报请求头的识别；删除或改写所有相关标识的客户端无法可靠识别。
