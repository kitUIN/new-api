# 工单

控制台侧边栏「个人」区域的「工单」入口位于 `/tickets`。普通用户可发起工单并查看自己的工单，管理员可查看、回复及关闭所有工单。

## 对话与关闭

- 创建时需要标题（1 至 120 字），每条消息需要文字或图片，也可同时包含两者。
- 每条消息最多 10000 字、4 张图片；每张图片最多 2 MB、2500 万像素，支持 PNG、JPEG、GIF、WebP。服务端先检查尺寸，再尝试解码，拒绝损坏或截断的图片。
- 支持多轮对话、状态筛选、列表分页、历史消息加载及图片预览。打开的工单会定期刷新；检测到关闭时会等待正在进行的消息请求完成，再补一次刷新，获取关闭前的最后回复。
- 附件接近可见区域时才下载。发送消息、关闭和列表刷新不会重复下载已经加载的附件。
- 工单发起人或管理员均可关闭工单。关闭不可撤销，历史仍可查看，双方都无法再发送消息。
- 关闭与回复通过对同一工单行的事务更新串行处理，消息或图片写入失败时整体回滚。

## 未读消息

- 侧边栏「工单」显示未读消息条数，0 条时隐藏；超过 99 条显示 `99+`，悬停说明保留完整数量。收起侧边栏后角标仍显示在图标旁。
- 普通用户只统计自己工单中由其他账号发送的消息；管理员统计所有工单中由其他账号发送的消息。每个账号（包括不同管理员）的阅读进度独立保存，刷新页面或更换设备后保留。
- 未读数每 15 秒刷新。仅进入列表、后台预取或关闭工单不会标记已读；在可见的工单详情中获取消息后，将该次获取的最新消息及其之前的消息标记为已读，并立即更新角标。之后到达的新消息仍为未读。
- 已读进度只会前进，多标签页的旧请求不会恢复已经读过的消息。首次上线时，没有阅读记录的历史消息也会计入未读。

## QQ 通知

复用上游分组变更通知使用的 `/api/nachoai/send_message` 接口及现有公共封装 `common.SendQQAdminMessage`。

- 复用 `QQCallbackAddress`、`QQCallbackAccessToken`、`QQAdminNumber`，无需单独配置。
- 新建成功及普通用户追问成功后通知管理员；管理员回复、关闭、校验失败和写入失败不发送通知。
- 通知包含工单编号、标题、用户、消息摘要、图片数量，以及基于 `ServerAddress` 生成的工单链接。
- 与现有通知一致，采用异步发送；未配置时跳过，网络失败记录日志，不影响已经保存的工单。

## 存储与接口

启动时自动创建 `tickets`、`ticket_messages`、`ticket_attachments`、`ticket_reads`，两种迁移路径均已注册。图片作为二进制数据保存到主数据库，备份主数据库即包含工单附件，不依赖本地上传目录。使用 GORM 的通用类型映射兼容 SQLite、MySQL 和 PostgreSQL。

所有接口均使用 `UserAuth`，需要登录凭证和 `New-Api-User` 请求头。普通用户访问他人工单或图片返回 404；关闭后的回复返回 409。

创建与回复共用独立的用户限流，默认每用户每 60 秒 20 次，通过 `TICKET_RATE_LIMIT`、`TICKET_RATE_LIMIT_DURATION` 配置，不占用登录等敏感操作的限额。附件下载使用独立的用户限流，默认每用户每 60 秒 240 次，通过 `TICKET_ATTACHMENT_RATE_LIMIT`、`TICKET_ATTACHMENT_RATE_LIMIT_DURATION` 配置，不占用控制台的全局 API 限额。超限返回 429；时间单位均为秒。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/ticket/` | 列表，支持 `p`、`page_size`、`status`（`open` / `closed` / 空） |
| GET | `/api/ticket/unread` | 当前账号的未读消息总数，响应 `data.unread_count` |
| POST | `/api/ticket/` | 创建，multipart 字段 `title`、`content`、重复的 `images` 文件字段 |
| GET | `/api/ticket/:id` | 工单详情 |
| GET | `/api/ticket/:id/messages` | 最近 50 条消息，`before` 为历史游标，响应含 `items`、`has_more` |
| POST | `/api/ticket/:id/messages` | 回复，multipart 字段 `content`、`images` |
| POST | `/api/ticket/:id/close` | 关闭，重复操作保留首次关闭人和时间 |
| POST | `/api/ticket/:id/read` | 保存阅读进度，JSON 字段 `message_id` 必须属于该工单 |
| GET | `/api/ticket/:id/attachments/:attachment_id` | 经过权限校验的图片内容，禁止缓存 |

除图片接口外，响应沿用 `{ success, message, data }` 格式。消息按 ID 升序返回，不在消息列表中内嵌图片二进制内容。
