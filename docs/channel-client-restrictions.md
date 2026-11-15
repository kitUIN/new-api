# 渠道客户端限制

在渠道新增或编辑页面的「高级设置 → 特定客户端访问」中，分别为 Codex 和 Claude Code 选择「不限制」「禁止」或「只允许」。默认不限制，适用于所有渠道类型。

- 禁止：拒绝该客户端；未设置「只允许」时，其他客户端仍可访问。
- 只允许：启用白名单，只允许匹配的客户端访问，普通 SDK 和无法识别的客户端均不可访问。
- 两个客户端均为「只允许」时，匹配任意一个即可；两者也可以同时禁止或混合设置。
- 同一个请求同时携带两个客户端标识时，禁止规则优先。

配置保存在现有渠道 `settings` JSON 的 `client_restrictions` 对象中，无需数据库迁移。字段值为 `unrestricted`、`deny`、`allow_only`，省略字段或空字符串表示不限制。例如，仅允许 Codex，同时禁止 Claude Code：

```json
{"client_restrictions": {"codex": "allow_only", "claude_code": "deny"}}
```

- 自动选路、重试、自动分组、组合分组及令牌模型组合会跳过受限渠道，继续寻找允许的渠道。全部不可用时沿用无可用渠道错误。
- 渠道亲和命中受限渠道时，重新选择允许的渠道。
- 显式指定渠道（包括旧版固定渠道组合）时，返回 HTTP 403，错误码为 `client_not_allowed`。
- 两个客户端都恢复「不限制」后，解除该渠道的客户端限制。

兼容旧配置 `{"disable_claude_code": true}`，其效果仍为禁止 Claude Code，固定渠道拒绝时保留旧错误码 `claude_code_not_allowed`。存在非 null 的 `client_restrictions` 对象时以新配置为准（包括空对象）；界面加载旧配置后展示为 Claude Code「禁止」，保存时转换为新配置并删除旧字段。

识别规则（均不区分大小写，不检查用户消息内容）：

- Claude Code：User-Agent 中的 `claude-cli`、`claude-code` 或 `claudecode` 产品标识；也识别同时携带 `X-App: cli` 和 `Anthropic-Beta: claude-code-*` 的请求。普通 Anthropic SDK、只有通用 CLI 标识或只有 beta 头，不会被识别为 Claude Code。
- Codex：User-Agent 产品名或 `Originator` 中的 `codex`、`codex-cli`、`codex_cli_rs`、`codex_exec`、`codex_vscode`、`codex_desktop`、`codex_app`；`Originator: Codex CLI` 也可识别。普通 OpenAI SDK 不会被识别为 Codex。

这是基于客户端自报请求头的识别；删除或改写所有相关标识的客户端无法可靠识别。
