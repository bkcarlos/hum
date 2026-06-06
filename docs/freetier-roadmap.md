# Sign in with Apple 免费档 · 配额系统 · 实施路线

> 版本：v0.1
> 日期：2026-06-07
> 文档状态：进行中（C0/C1/C2 已完成代码，免费档默认关闭）
> 一句话：让 App Store 用户**无需自带 LLM key** 也能用——用 Sign in with Apple 识别用户，在**服务端自有 key** 上按配额提供免费额度；重度用户仍可 BYOK（自带 key、不限量）。

---

## 1. 背景与目标

公开上架的最大障碍是 **BYOK**：用户不填第三方 LLM key 时 app 几乎空白，审核（Guideline 2.1 完整性）很可能打回。
方案：**双轨**——
- **BYOK**：请求带 `X-LLM-Api-Key` → 用用户自己的 key，**不限量**。
- **免费档**：带 `Authorization: Bearer <session>`（由 Sign in with Apple 换取）→ 用**服务端默认 key**，按**每人/全局每日配额**限量。

> 红线对齐：服务端默认 key 是**运营密钥**（Secret Manager 注入、绝不写日志、不随请求），是"config 不放 LLM key"的**唯一例外**（仅限服务自有、非用户 key）。详见 `CLAUDE.md` 红线 #2。

## 2. 架构（已落地部分）

- 网关：`internal/handlers/freetier.go` 的 `resolveProvider` —— 有 key 走 BYOK 不限量；否则验 session → 查配额 → 用默认 key，失败 `refund`。
- 计费：仅 `suggest` + `rank` 计数（**一次推荐 ≈ 2 次**）；`intent`/`examples`/`test`/`models` 仍仅 BYOK。超额返回 **429 `quota_exceeded`**（带 `reason` + 计数）。
- 身份：`POST /api/auth/apple` 验 Apple identityToken（JWKS：iss/aud=bundleId/exp → `sub`）→ 签发 HS256 session（30 天）。
- 存储：`internal/quota` 的 `Store` 接口；`MemoryStore`（本地/测试，单实例）/ `FirestoreStore`（生产，事务化、配置热更）。
- **动态配置**：`humQuota/config` 文档实时读取（~30s 缓存）→ 在 GCP 控制台改额度/开关/默认 key，**无需重启**。

## 3. 进度看板

| 编号 | 内容 | 状态 | 备注 |
|---|---|---|---|
| C0 | 免费档后端：quota 引擎 + Apple 验签/session + 接线（MemoryStore） | ✅ `520d078` | `go test -race` 全过 + 实跑 |
| C1 | Firestore 配额存储（事务化、配置热更、boot 失败回退内存） | ✅ `0888066` | 编译+接口校验+回退实测；**运行时待 GCP 验** |
| C2 | 管理员身份（`/auth/me` + admin 白名单 + `AdminOnly` 中间件） | ✅ `4168c4d` | `go test -race` 全过 + 实跑（anon 401 / 非管理员 403 / 管理员 200） |
| C3 | 管理 API（用量/用户列表、封禁/解禁、读写配额配置） | ⬜ 待做（下一步） | 可测 |
| C4 | 管理后台页面（受保护 UI） | ⬜ 待做 | 前端 |
| C5 | iOS 接入 Sign in with Apple + 免费档 + 429 文案 | ⬜ 待做 | **需 Xcode 验证** |
| C-GCP | GCP 准备（你来做） | ⬜ 待做 | 见 §6 |

## 4. 待完成详情

### C2 · 管理员身份 ✅（`4168c4d`）
- **目标**：定义"谁是管理员"并可动态修改，不重启。
- **设计（已落地）**：管理员 = Apple `sub` 白名单，**存在 live 配额配置文档 `humQuota/config` 的 `admins[]` 字段里**（不是单独的 `config/admins` 集合）——直接复用现有 `GetConfig` 的 ~30s 缓存与"控制台改、不重启"路径，`IsAdmin` 只读缓存配置、零额外存储方法。管理员也走 Sign in with Apple，中间件校验 `sub` 是否在白名单。
- **改动（已实现）**：
  - `GET /api/auth/me`（任意有效 session，含非管理员）：返回 `{sub, isAdmin}`。`sub` 用于 bootstrap 第一个管理员；`isAdmin` 供前端决定是否露出管理入口（best-effort，真正的门是 `AdminOnly`）。
  - `quota.Config` 新增 `Admins []string` + `IsAdmin(sub)`；`config` 新增 `ADMIN_APPLE_SUBS`（CSV）首启种子。
  - `handlers.AdminOnly()` 中间件：校验 Bearer session 的 `sub` ∈ `admins`，否则 401（无 session）/ 403（非管理员）；**fail closed**（存储出错即拒绝），并把 `sub` 写进 gin context 供 C3 用。
  - `GET /api/admin/me`：过 `AdminOnly` 的"探针"，管理后台加载时调它确认权限。
- **bootstrap**：登录后用 `/auth/me` 拿到自己的 `sub` → 写进 `humQuota/config` 文档的 `admins[]`（或首启设 `ADMIN_APPLE_SUBS`）→ 即为管理员。
- **验收（已过）**：非管理员访问 admin 路由 403；白名单内 200；改白名单即时生效（内存即时；Firestore ≤30s 配置缓存）。

### C3 · 管理 API（挂 `/api/admin/*`，过 `AdminOnly` 中间件——C2 已就绪）
- `GET /admin/config` / `PUT /admin/config`：读/写配额配置（开关、每人/全局额度、默认 LLM provider/model/baseUrl）。⚠️ `admins[]` 也在这同一个 config 文档里——`PUT` 必须**保留**它（或显式管理），别让前端漏传字段把管理员名单清空。
- `GET /admin/usage?day=YYYY-MM-DD`：全局当日用量 + Top 用户。
- `GET /admin/users` / `POST /admin/users/{sub}/ban` / `unban`。
- **验收**：改配置后 `resolveProvider` 即时按新值执行；封禁用户立刻 429 `banned`。

### C4 · 管理后台页面
- 一个受保护页面（可挂现有前端 `/admin`，或独立小页）：管理员用 Sign in with Apple 登录 → 看当日用量曲线 + 用户列表 + 改额度/开关 + 封禁。
- **验收**：非管理员进不去；改额度/封禁的效果与 C3 API 一致。

### C5 · iOS 接入
- Xcode 加 **Sign in with Apple** capability（entitlement `com.apple.developer.applesignin`）+ 开发者后台开启。
- 登录按钮/流程 → 拿 identityToken `POST /auth/apple` → session 存 **Keychain**。
- LLM 请求按"免费档(Bearer) / 自带 key(BYOK)"二选一发送；设置页加切换。
- 429 文案："今日免费额度用完，明天再来，或在设置填自己的 key"。
- **验收**：免费档登录后能出推荐；额度用尽提示切 BYOK；BYOK 不受限。*（需真机 + Xcode 编译验证）*

## 5. 配置项（env）速查

免费档需以下**全部**设置才开启（`config.FreeTierConfigured()`）：

| env | 说明 |
|---|---|
| `DEFAULT_LLM_API_KEY` | 服务端自有 LLM key（Secret Manager 注入；非用户 key） |
| `DEFAULT_LLM_PROVIDER` / `DEFAULT_LLM_BASE_URL` / `DEFAULT_LLM_MODEL` | 默认 LLM provider/baseUrl/model（seed 进 Firestore config） |
| `SESSION_SECRET` | session JWT 的 HMAC 密钥 |
| `APPLE_BUNDLE_ID` | Apple identityToken 期望的 `aud`（iOS bundle id，如 `com.carlosbk.hum`） |
| `FREE_TIER_ENABLED` | 总开关（默认 true；上线后以 Firestore config 文档为准） |
| `FREE_TIER_PER_USER_DAILY` | 每人每日额度（默认 20；1 推荐≈2） |
| `FREE_TIER_GLOBAL_DAILY` | 全局每日额度（默认 0=不限；用于护账单） |
| `FIRESTORE_PROJECT` | 设了用 Firestore，否则内存（单实例，仅 dev） |
| `ADMIN_APPLE_SUBS` | 管理员 Apple sub 白名单种子（CSV，**仅首启** seed 进 `humQuota/config` 的 `admins[]`；之后以控制台文档为准） |

## 6. C-GCP · 你需要做的（部署免费档前）

1. 开启 **Firestore（Native 模式）**。
2. 给 Cloud Run 服务账号授 `roles/datastore.user`。
3. 把 `DEFAULT_LLM_API_KEY` 放 **Secret Manager**，在 Cloud Run 以环境变量注入；同时设置 `SESSION_SECRET`、`APPLE_BUNDLE_ID`、`DEFAULT_LLM_*`、`FREE_TIER_*`、`FIRESTORE_PROJECT`。
4. 部署后：可在 Firestore 控制台直接改 `humQuota/config` 文档调额度/开关——即时生效、不重启。

## 7. Firestore 数据布局

```
humQuota/config            → 配额配置 + 管理员白名单（Config，含 admins[]；控制台改、不重启）
humQuotaGlobal/{day}       → { count }   全局当日计数
humQuotaUser/{day}__{sub}  → { count }   每用户当日计数
humQuotaBans/{sub}         → { banned }  封禁标记（不存在=未封）
```

## 8. 已定的设计决策

- 计费单位 = `suggest`+`rank`（一次推荐≈2）；examples/intent 不计费（仅 BYOK）。
- 失败/空结果的免费请求 **退款**，不烧额度。
- BYOK 永远不限量、不经配额。
- 配额"动态不重启"由 **Firestore 配置文档 + 实时读取** 实现，管理后台只是它的 UI。
- 管理员身份用 **Apple sub 白名单**（非共享口令），改名单即时生效。名单存 `humQuota/config` 的 `admins[]` 字段（复用 live config 缓存，不另建集合）；`ADMIN_APPLE_SUBS` 仅作首启种子。

## 9. 验证与上线顺序

1. 后端每块：`go build/vet/test`（含 `-race`）。
2. 配好 §6 的 GCP env → 在 staging/prod 验 `/auth/apple` + 一次 `suggest` 计数 + 429。
3. iOS（C5）在 Xcode 真机验证登录 + 免费档 + 429。
4. 上架前再过一遍 §1 红线与 Apple MusicKit ToS。

> 相关：`CLAUDE.md`（红线/约定）、`docs/requirements.md`（范围）、`docs/deploy-cloudrun.md`（部署）。代码入口见 `internal/handlers/freetier.go`、`internal/quota/`、`internal/auth/`。
