# 使用日志中的 API 来源域名

消费日志和错误日志会将用户访问的入口域名记录到 `other.request_host`，并在使用日志详情中显示「API 来源域名」。优先使用 `X-Forwarded-Host` 的第一个值，否则使用 HTTP `Host`（包含请求中原有的端口）。历史日志没有该字段时不显示，无法追溯补全。

## Nginx 反代配置

香港 B（`hk.kituin.fun`）和美西 C（`fast.kituin.fun`）的公网入口，在现有 `location` 中增加：

```nginx
# 覆盖客户端自带的值，记录本入口域名。
proxy_set_header X-Forwarded-Host $host;
```

现有的 `proxy_pass`、`Host` 和 TLS SNI 配置无需因这个日志字段而更改。即使向 A 转发时使用 `Host: ai.kituin.fun`，上述请求头也能保留 B/C 的入口域名。

若 A 前还有 Nginx，需要保留来自 B/C 的域名，但不能直接信任任意公网客户端提交的请求头。以下配置放在 `http` 中，将占位 IP 替换为 A 实际看到的 B/C 出口 IP：

```nginx
geo $remote_addr $is_api_entry_proxy {
    default 0;
    <B出口IP> 1;
    <C出口IP> 1;
}

map "$is_api_entry_proxy:$http_x_forwarded_host" $api_source_host {
    default $host;
    "1:hk.kituin.fun" hk.kituin.fun;
    "1:fast.kituin.fun" fast.kituin.fun;
}
```

A 转发到应用的 `location` 中设置：

```nginx
proxy_set_header X-Forwarded-Host $api_source_host;
```

如果启用了 Nginx realip 模块且重写了 `$remote_addr`，上面的 `geo` 应改用 `$realip_remote_addr` 识别实际连接来源。应用端口应仅允许受控反代访问；应用将此请求头作为日志元数据读取，不用于鉴权或计费。

部署后分别经三个入口发送请求，打开新日志详情，应分别显示 `ai.kituin.fun`、`hk.kituin.fun`、`fast.kituin.fun`。未传递原始域名时，应用只能记录 A 收到的 Host，无法自行推断用户访问了哪个反代入口。
