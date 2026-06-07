# 查日志速查（Hum）

服务：Cloud Run `hum-github` · 区域 `asia-east2` · 项目 `hum-runables`。
后端日志：`slog` JSON 结构化（生产；`GIN_MODE=debug` 为可读文本）→ Cloud Logging。
**红线：日志从不含 API key / 会话令牌 / 请求体**，只有 method/path/status/latency/ip/request_id。

## 三种来源

| 来源 | 看哪 | 适合 |
|---|---|---|
| 后端 | Cloud Logging / `gcloud`（JSON 字段可查） | 服务端请求、错误、配额、登录校验 |
| iOS 客户端 | App 内「设置 → 诊断日志」复制/分享；或 Xcode / Console.app | App 运行时、登录每一步 |
| 关联 | `request_id`（= 客户端日志里的 `rid`） | iOS↔后端串起来 |

## 后端常用命令

```bash
# 最近 N 条
gcloud run services logs read hum-github --region=asia-east2 --limit=50

# 实时跟（需 gcloud components install beta）
gcloud beta run services logs tail hum-github --region=asia-east2

# 只看错误（近 1 小时）
gcloud logging read 'resource.labels.service_name="hum-github" AND severity>=ERROR' --freshness=1h --limit=30

# 按某次请求 id（从 iOS 诊断日志 / web 网络面板拿到的 rid）—— 串两端
gcloud logging read 'resource.labels.service_name="hum-github" AND jsonPayload.request_id="RID"' --freshness=2h

# 按接口 + 状态
gcloud logging read 'resource.labels.service_name="hum-github" AND jsonPayload.path="/api/suggest" AND jsonPayload.status>=400' --freshness=1h
```

网页版（推荐，能存查询/看时间线/看完整 JSON）：console → Logging → **Logs Explorer**，
过滤框 `resource.labels.service_name="hum-github"`，再叠 `severity>=ERROR` 或
`jsonPayload.request_id="xxx"` / `jsonPayload.path="..."`。

可查的 JSON 字段：`jsonPayload.method` `jsonPayload.path` `jsonPayload.status`
`jsonPayload.latency_ms` `jsonPayload.request_id` `severity`。

## iOS 客户端日志

- App 内：**设置 → 诊断日志** → 复制 / 分享（带「App 版本 · iOS · 机型」头，不含密钥）。
  每行带 `rid=xxxx`，拿去后端按 `jsonPayload.request_id` 查。
- 开发时：Xcode 跑 → 底部控制台；或 `Console.app` 按 subsystem **`com.carlosbk.hum`**
  （category `net` / `auth` / `reco`）过滤。

## 排查范式

1. 用户报问题 → 让其在 iOS「诊断日志」复制/分享，拿到一行的 `rid`。
2. `gcloud logging read '... jsonPayload.request_id="<rid>"'` 看服务端那一条的 status/错误。
3. 典型：`/auth/apple 401` ≈ aud 不匹配（后端 `APPLE_BUNDLE_ID` ≠ bundle id）；
   `/suggest 429 quota_exceeded` ≈ 免费额度用尽；`5xx` ≈ 上游 LLM 故障/超时。
