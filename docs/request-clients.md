# 使用日志客户端图标

系统设置 → 维护设置 → 客户端识别（`/system-settings/operations/request-clients`）提供有序客户端列表，支持新增、删除、禁用、排序以及修改显示名称、图标、链接和请求头正则。

- 配置保存在 `RequestClientRules` option 中，无新增数据库列；支持 SQLite、MySQL、PostgreSQL。
- 按列表顺序选择第一个启用且匹配的客户端；同一客户端内多条规则是 OR 关系。每条规则匹配指定请求头的单个值，缺失或空请求头不匹配。
- 使用 Go RE2 正则语法，可用 `(?i)` 忽略大小写。保存前编译验证；无效配置不写入数据库、不替换运行中的规则。最多 100 个客户端，每个最多 16 条规则，单条正则最多 1024 字节。
- 图标支持内置标识 `codex`、`codex-desktop`、`codex-tui`、`codex-vscode`、`codex-tui-vscode`、`claude-code`、`pi`、`opencode`、`codebuddy`、`deepseek-harness`，或站内绝对路径、HTTP(S) 图片 URL。跳转链接可留空，仅允许 HTTP(S)。链接在新标签页打开。
- 成功/失败日志保存 `other.request_client` 标识及原始 `other.user_agent`；客户端识别功能不保存其他请求头。日志详情展示完整 User-Agent 并支持复制，即使客户端无法识别也保留 User-Agent。未记录 User-Agent 的历史日志无法补回。
- `/api/status` 只返回已启用客户端的显示信息；正则仅在管理员配置中可见。图标和名称变更在刷新配置后也适用于带标识的历史日志。修改标识、禁用或删除客户端会隐藏对应旧日志的图标。
- 没有标识的旧日志、无法识别的请求、未配置/禁用的客户端均不显示图标。模型名、运行时、SDK 和请求正文不用于猜测客户端；本规则不改变渠道客户端限制或鉴权。
- 图标位置保留固定宽度，未识别客户端的请求体大小仍然对齐。客户端图标、请求体大小、图片数量分别使用页面统一的悬浮提示；图片图标只提示数量。DeepSeek Harness 使用随浅色/深色主题切换的黑白图标。

## prod 调查（2026-11-16）

只读查询 sub2api PostgreSQL 的 `usage_logs.user_agent` 与 `ops_error_logs.user_agent`，先按完整 UA 去重，再合并版本/平台。初始使用日志快照共 **677,482** 条、**407** 种 UA，覆盖 2026-08-18 至 2026-11-16；生产持续写入，后续聚合数量会增加。未导出用户、密钥、IP 或请求正文。

| 去重后客户端/标识 | 处理 |
| --- | --- |
| Codex Desktop | Codex 图标 + 显示器角标 |
| codex-tui、codex-cli、codex_cli_rs | Codex 图标 + 终端角标 |
| codex_vscode | Codex 图标 + VS Code 角标 |
| codex-tui / codex-cli / codex_cli_rs，UA 同时包含 vscode/ 或 code-server/ | Codex 图标 + 蓝色终端角标，名称为 Codex TUI (VS Code) |
| codex_exec、codex_sdk_ts | 通用 Codex 图标，不推断运行环境 |
| pi（有版本号和仅平台信息两种格式） | Pi Agent 图标 |
| opencode | OpenCode 图标 |
| VSCode/... CodeBuddy/... | CodeBuddy 图标 |
| deepseek-harness | DeepSeek 图标 |
| RikkaHub-Android | 官方 RikkaHub 图标 |
| magpie | 官方 Magpie 图标 |
| cc-pocket | 官方 Pairlet 图标，名称保留 CC Pocket |
| omp | 官方 Oh My Pi 图标 |
| Go-http-client、undici、Bun、OpenAI/JS、ai-sdk | 仅 SDK/运行时，无法确定应用，跳过 |
| orca_desktop、pvf_editor、ucp、codex-chrome-extension-sidepanel | 联网未找到足够明确的产品对应，跳过 |
| new-api-ws-warmup-diagnostic | 诊断请求，跳过 |

默认另外包含项目原有支持的 Claude Code UA；此次 prod 样本没有出现 Claude Code。

Codex 统一使用与 ChatGPT 相同的 OpenAI 图标，通过角标区分运行方式。具体规则排在通用 Codex 规则之前，VS Code 内的 TUI 排在普通 TUI 之前。例如 `codex-tui/0.167.0 (Windows 10.0.26200; x86_64) vscode/1.145.0 (codex-tui; 0.166.0)` 会识别为 VS Code 内的 TUI；`codex_vscode/...` 则为 VS Code 扩展。仅尾部包含 `VS Code` 字样的 Desktop UA 仍识别为 Desktop。UA 未携带运行环境时，无法进一步区分。已自定义的规则不会被默认配置覆盖，可在设置中调整规则顺序。

可复查聚合 SQL（仅用于 sub2api 的 PostgreSQL 调查，不属于应用运行 SQL）：

```sql
BEGIN READ ONLY;
SET LOCAL statement_timeout = '30s';
SELECT COUNT(*), MIN(created_at), MAX(created_at), COUNT(DISTINCT user_agent)
FROM usage_logs;
SELECT user_agent, COUNT(*) FROM usage_logs GROUP BY user_agent ORDER BY COUNT(*) DESC;
SELECT user_agent, COUNT(*) FROM ops_error_logs GROUP BY user_agent ORDER BY COUNT(*) DESC;
COMMIT;
```

身份核对与图标来源见 [客户端图标来源](../web/public/images/clients/README.md)。

## 本次验证

- `go test ./common ./model -run 'Test(DetectRequestClient|RequestClient|RecordLogsPreservesRequest)' -count=1`：通过，包含配置验证、顺序/禁用、日志写入及 SQLite 配置持久化；未连接生产数据库执行写入。
- `go test ./controller -run '^$'`：编译通过。
- `bun test src/features/usage-logs/components/request-client-icon.test.tsx`：4 项通过。
- `bun run build:check`：类型检查及生产构建通过。
- 本次修改的 TS/TSX 文件 ESLint 检查通过。标准 ESLint 命令被现有 `brace-expansion@2` 与 ESLint 10/minimatch 的依赖冲突阻断；本地检查将原配置的 glob 等价展开后执行，未禁用规则、未修改项目依赖。
- 浏览器验证：未知客户端无图标且大小对齐、图标链接新标签页打开、无效正则拒绝保存、有效配置保存后名称刷新、浅色/深色图标可见；Codex 运行方式分别识别，日志详情可查看和复制 User-Agent，客户端/大小/图片数量提示独立展示。

本地示例网站为 `http://127.0.0.1:5180/`，复用实际组件和 Go 识别/校验代码，数据为示例；未部署 prod。
