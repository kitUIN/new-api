# QQ 开放接口

本文档描述面向 QQ 服务端调用的开放接口，用于按 QQ 号查询绑定用户的 API Key、查询用户当前可用分组，以及修改该用户 API Key 的分组。

## 鉴权

所有接口都需要携带 QQ 服务鉴权 token。token 使用系统配置中的 `QQCallbackAccessToken`。

支持以下任意一种请求头：

```http
X-Access-Token: <QQCallbackAccessToken>
Authorization: Bearer <QQCallbackAccessToken>
Authorization: <QQCallbackAccessToken>
```

未配置或鉴权失败时，接口会返回失败响应；鉴权失败的 HTTP 状态码为 `401`。

## 通用响应格式

成功响应：

```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

失败响应：

```json
{
  "success": false,
  "message": "错误信息"
}
```

## 1. 查询 QQ 号对应用户的 API Key 列表

```http
GET /api/qq/users/:qq_id/tokens
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `qq_id` | string | 是 | 用户绑定的 QQ 号 |

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `qq_id` | string | QQ 号 |
| `user_id` | number | 绑定用户 ID |
| `tokens` | array | API Key 列表 |

`tokens` 中的 `key` 为脱敏后的 API Key，不返回完整密钥。

### 响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "qq_id": "123456",
    "user_id": 1,
    "tokens": [
      {
        "id": 10,
        "name": "默认令牌",
        "key": "abcd**********wxyz",
        "status": 1,
        "created_time": 1710000000,
        "accessed_time": 1710000000,
        "expired_time": -1,
        "remain_quota": 100000,
        "used_quota": 0,
        "unlimited_quota": false,
        "model_limits_enabled": false,
        "model_limits": "",
        "group": "default",
        "cross_group_retry": false
      }
    ]
  }
}
```

### curl 示例

```bash
curl -X GET "https://example.com/api/qq/users/123456/tokens" \
  -H "X-Access-Token: <QQCallbackAccessToken>"
```

## 2. 查询 QQ 号对应用户当前可用分组

```http
GET /api/qq/users/:qq_id/groups
```

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `qq_id` | string | 是 | 用户绑定的 QQ 号 |

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `qq_id` | string | QQ 号 |
| `user_id` | number | 绑定用户 ID |
| `user_group` | string | 用户自身分组 |
| `usable_groups` | object | 当前用户可使用的分组 |

`usable_groups` 的 key 为分组名称；value 中包含：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `ratio` | number/string | 分组倍率；`auto` 分组返回 `"自动"` |
| `desc` | string | 分组描述 |

### 响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "qq_id": "123456",
    "user_id": 1,
    "user_group": "default",
    "usable_groups": {
      "default": {
        "ratio": 1,
        "desc": "默认分组"
      },
      "vip": {
        "ratio": 1,
        "desc": "VIP 分组"
      },
      "auto": {
        "ratio": "自动",
        "desc": "自动选择可用分组"
      }
    }
  }
}
```

### curl 示例

```bash
curl -X GET "https://example.com/api/qq/users/123456/groups" \
  -H "X-Access-Token: <QQCallbackAccessToken>"
```

## 3. 修改 QQ 号对应用户 API Key 的分组

```http
PUT /api/qq/users/:qq_id/tokens/:token_id/group
```

接口只允许修改该 QQ 号绑定用户自己的 API Key。如果 `token_id` 不属于该用户，会返回失败响应。

### 路径参数

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `qq_id` | string | 是 | 用户绑定的 QQ 号 |
| `token_id` | number | 是 | API Key ID |

### 请求体

```json
{
  "group": "default"
}
```

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `group` | string | 是 | 目标分组名称 |

说明：

- `group` 必须在该用户当前可用分组内。
- `group` 可传空字符串 `""`，表示清空 API Key 自身分组，回退使用用户自身分组。
- 修改成功后只更新 API Key 的分组，不改变名称、额度、过期时间等其它字段。

### 响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "qq_id": "123456",
    "user_id": 1,
    "token": {
      "id": 10,
      "name": "默认令牌",
      "key": "abcd**********wxyz",
      "status": 1,
      "created_time": 1710000000,
      "accessed_time": 1710000000,
      "expired_time": -1,
      "remain_quota": 100000,
      "used_quota": 0,
      "unlimited_quota": false,
      "model_limits_enabled": false,
      "model_limits": "",
      "group": "default",
      "cross_group_retry": false
    }
  }
}
```

### curl 示例

```bash
curl -X PUT "https://example.com/api/qq/users/123456/tokens/10/group" \
  -H "X-Access-Token: <QQCallbackAccessToken>" \
  -H "Content-Type: application/json" \
  -d '{"group":"default"}'
```

## 4. 查询分组成功率

```http
GET /api/qq/group-health
```

该接口返回当前系统分组维度的成功率和健康状态，数据来源与后台分组健康/性能指标一致。

### 查询参数

| 参数 | 类型 | 必填 | 默认值 | 最大值 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `hours` | number | 否 | `24` | `168` | 统计窗口，单位为小时 |
| `interval_minutes` | number | 否 | `10` | `60` | 分桶间隔，单位为分钟 |

### 响应字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `window_hours` | number | 实际统计窗口小时数 |
| `interval_minutes` | number | 实际分桶间隔分钟数 |
| `bucket_count` | number | 分桶数量 |
| `series_schema` | string | 分桶时间格式配置 |
| `groups` | array | 分组健康列表 |

`groups` 中常用字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `group` | string | 分组名称 |
| `ratio` | number | 分组倍率 |
| `provider_count` | number | 分组可用供应商/渠道聚合数量 |
| `balance_level` | number | 余额等级，`0` 低、`1` 中、`2` 高 |
| `request_count` | number | 统计窗口内请求数 |
| `success_rate` | number | 统计窗口内成功率，百分比 |
| `avg_ttft_ms` | number | 平均首 token 延迟，毫秒 |
| `avg_latency_ms` | number | 平均请求延迟，毫秒 |
| `avg_tps` | number | 平均输出 TPS |
| `recent_request_count` | number | 近 10 分钟或 20 分钟请求数 |
| `recent_success_rate` | number | 近 10 分钟或 20 分钟成功率，百分比 |
| `recent_window_minutes` | number | 近期成功率使用的窗口分钟数 |
| `buckets` | array | 分桶序列 |

`buckets` 中常用字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `ts` | number | 分桶开始时间戳，秒 |
| `end_ts` | number | 分桶结束时间戳，秒 |
| `request_count` | number | 分桶请求数 |
| `success_count` | number | 分桶成功数 |
| `success_rate` | number | 分桶成功率，百分比 |
| `avg_ttft_ms` | number | 分桶平均首 token 延迟，毫秒 |
| `avg_latency_ms` | number | 分桶平均请求延迟，毫秒 |
| `avg_tps` | number | 分桶平均输出 TPS |
| `status` | string | 分桶状态：`empty`、`ok`、`warning`、`error` |

### 响应示例

```json
{
  "success": true,
  "message": "",
  "data": {
    "window_hours": 24,
    "interval_minutes": 10,
    "bucket_count": 144,
    "series_schema": "bucket_start",
    "groups": [
      {
        "group": "default",
        "ratio": 1,
        "provider_count": 3,
        "balance_level": 2,
        "request_count": 1000,
        "success_rate": 99.2,
        "avg_ttft_ms": 520.5,
        "avg_latency_ms": 2300.8,
        "avg_tps": 38.4,
        "recent_request_count": 80,
        "recent_success_rate": 98.75,
        "recent_window_minutes": 10,
        "buckets": [
          {
            "ts": 1710000000,
            "end_ts": 1710000600,
            "request_count": 20,
            "success_count": 20,
            "test_request_count": 0,
            "non_test_request_count": 20,
            "success_rate": 100,
            "avg_ttft_ms": 500,
            "avg_latency_ms": 2100,
            "avg_tps": 40,
            "status": "ok"
          }
        ]
      }
    ]
  }
}
```

### curl 示例

```bash
curl -X GET "https://example.com/api/qq/group-health?hours=24&interval_minutes=10" \
  -H "X-Access-Token: <QQCallbackAccessToken>"
```
