# Hum · 哼一首

> 哼一段说不出名字的调子，它帮你找到真实的歌。

用自然语言描述心情/场景，让 LLM 从 **Apple Music 真实曲库** 里挑歌、试听、并一键建成歌单。
（"Hum" 为项目代号；内部仓库目录仍为 `apple_music_llm`，对外品牌请勿使用 "Apple Music" 商标。）

> 黄金原则：**LLM 不产歌，只懂人与排序**。歌曲事实层全部来自 Apple Music 官方 API，LLM 只负责
> ①把自然语言解析成结构化意图，②从真实候选池里筛选排序。完整背景见 [docs/requirements.md](docs/requirements.md)。

这是 **M1 就绪的脚手架**：认证、BYOK LLM 抽象、双栏 UI、主链路骨架已打通并通过构建/测试；接入真实
Apple 凭证后即可端到端跑通。

---

## 架构

```
Vue 3 + Vite + TS + Pinia + Naive UI  (frontend/)
  · BYOK LLM Key 仅存浏览器本地，调用时放入 X-LLM-Api-Key 请求头
  · MusicKit JS 授权 / 试听；Music User Token 仅存会话内存
        │ HTTPS（dev 用 Vite 代理 /api → :8080）
        ▼
Go + Gin  (backend/)
  ├─ Apple Music：ES256 签 Developer Token、Catalog 搜索、建歌单
  ├─ BYOK LLM：OpenAI 兼容 / Anthropic / Gemini 三适配器，统一 LLMProvider 接口
  └─ 安全：用户 Key 单次请求用完即弃，绝不落库/缓存/日志（日志中间件显式排除敏感 header）
```

## 目录结构

```
apple_music_llm/
├── docs/requirements.md         # 需求文档（v0.5，单一事实来源）
├── backend/                     # Go + Gin 编排层
│   ├── cmd/server/main.go       # 入口：路由 + 中间件
│   └── internal/
│       ├── config/              # 环境配置（无任何 LLM key）
│       ├── applemusic/          # Developer Token 签发 + 目录搜索 + 建歌单
│       ├── llm/                 # LLMProvider 接口 + 三适配器 + 错误归一化
│       ├── handlers/            # HTTP 处理器（key 从 header 读、用完即弃）
│       ├── httpx/               # 统一响应helper
│       └── middleware/          # 日志（脱敏）/ CORS
└── frontend/                    # Vue 3 单页应用
    └── src/
        ├── api/                 # 后端客户端（注入 key header）
        ├── services/musickit.ts # MusicKit JS 封装
        ├── stores/              # Pinia：llmConfig / apple / conversation / playlist
        ├── composables/         # 推荐编排 / 预览播放 / 响应式
        ├── components/          # 双栏 UI 各组件
        └── data/providers.ts    # BYOK Provider 预设表
```

## 前置条件

- Go ≥ 1.23、Node ≥ 20.19
- （端到端需要）Apple Developer Program 账号、MusicKit Key（`.p8`）、一份 Apple Music 订阅
- 一个你自己的 LLM API Key（OpenAI / DeepSeek / 通义 / Claude / Gemini 等，BYOK）

## 快速开始

```bash
# 1) 后端
cd backend
cp .env.example .env            # 填入 APPLE_TEAM_ID / APPLE_KEY_ID / .p8 路径（没有也能启动，仅 /api/apple/* 返回 503）
go run ./cmd/server             # 监听 :8080

# 2) 前端（另开一个终端）
cd frontend
cp .env.example .env            # 一般无需改动
npm install
npm run dev                     # 打开 http://localhost:5173
```

> 也可以用根目录的 `make dev-backend` / `make dev-frontend`。

使用流程：打开页面 → 右上角「配置 LLM Key」并测试连接 → 「连接 Apple Music」授权 →
左栏描述心情/场景 → 右栏试听、勾选 → 建成歌单。

## 构建与测试

```bash
# 后端
cd backend && go build ./... && go test ./...

# 前端
cd frontend && npm run type-check && npm run build
```

## 部署（方案 1 · 单服务）

一个 Docker 镜像搞定：Node 阶段构建前端，Go 阶段编译静态二进制，最终镜像里**一个 Go 进程同时托管**前端静态文件（`WEB_DIR=/app/web`）和 `/api`。**同源，无需 CORS**。

本地跑"生产形态"自测：
```bash
cd frontend && npm run build && cd ..
WEB_DIR="$PWD/frontend/dist" GIN_MODE=release PORT=8080 go -C backend run ./cmd/server
# 打开 http://localhost:8080 —— 前端与 API 同源
```

构建并运行镜像：
```bash
docker build -t hum .
docker run -p 8080:8080 \
  -e APPLE_TEAM_ID=XXXX -e APPLE_KEY_ID=YYYY \
  -e APPLE_PRIVATE_KEY="$(cat AuthKey_YYYY.p8)" \
  hum
```

**运行时需在平台 Secret/Env 配置：**

| 变量 | 说明 |
|------|------|
| `APPLE_TEAM_ID` / `APPLE_KEY_ID` | Apple 开发者凭证 |
| `APPLE_PRIVATE_KEY` | .p8 的 PEM 内容（云平台用它，而非文件路径） |
| `CORS_ALLOWED_ORIGINS` | 单服务可不设（同源）；前后端分离时设为前端域名 |

`WEB_DIR` / `GIN_MODE` / `PORT` 镜像已设好；多数平台会注入自己的 `PORT`，服务已自动遵循。HTTPS 由平台提供（MusicKit JS 与 key 传输都强制要求）。

### 平台对比（跑这个单容器 · 个人自用）

| 平台 | 成本 / 免费档 | 冷启动 | 部署方式 | 备注 |
|------|--------------|--------|----------|------|
| **Render** | 有免费档；付费 Starter ~$7/月 | 免费档闲置 ~15 分钟休眠，**冷启动 30–60s** | 连 GitHub 自动部署 / Dockerfile | 最省心；免费档冷启动会拖慢首个请求，建议付费档常驻 |
| **Railway** | 无长期免费，约 $5/月用量额度起 | 基本常驻（按用量计费） | 连 GitHub / Dockerfile / Nixpacks | DX 最佳，小额可预期；适合"花点小钱省事" |
| **Fly.io** | 按量计费，可缩到 0 | 唤醒**冷启动数秒**（比 Render 快） | `flyctl` + Dockerfile（`fly.toml`） | **可选区域**（东京/香港/新加坡等）→ 亚洲延迟更好；偏运维 |
| **Cloud Run** | 免费额度大（个人自用大概率覆盖），缩到 0 | Go 小镜像**冷启动 ~1–3s**；可设 `min-instances=1` 免冷启动 | `gcloud` / Cloud Build + Dockerfile | 最省钱、冷启动最快；GCP 初始配置略多 |

**怎么选：**
- 图省心、能接受小额月费 → **Railway**（DX 最好）或 **Render**（付费档常驻）
- 想最省钱 / 缩到 0 且冷启动要快 → **Cloud Run**
- 在意亚洲访问延迟、想指定区域 → **Fly.io**（选东京/香港节点）

> 注意：① 本应用首个请求会触发两次 LLM + 一次检索（目标 < 8s），所以**冷启动越短越好**——这点 Cloud Run / Fly 优于 Render 免费档。② 各家定价与免费政策时常变动，以官网为准。③ 若从中国大陆访问，几家平台连通性都可能有波动，必要时自备可达的域名/线路。

## 安全要点（BYOK · 方案 B）

- 用户 LLM Key 仅存浏览器 localStorage；调用时随 `X-LLM-Api-Key` 头发给后端
- 后端**单次请求内使用、用完即弃**：不落库、不缓存、不写日志
- 日志中间件 [`middleware/logger.go`](backend/internal/middleware/logger.go) 只记录 method/path/status，并维护「永不记录」敏感 header 名单
- Gemini 用 `x-goog-api-key` 头而非 `?key=` 查询参数，避免 key 进入 URL/访问日志
- 传输安全依赖 HTTPS（TLS），不自行做应用层加密
- 上线前自查：`grep` 访问日志确认无 key；排查 CDN/WAF/APM 的请求头快照

## 已实现 vs 待办（对照里程碑）

**已实现并通过构建/测试**
- M1：Developer Token（ES256）签发与缓存（含单测）；MusicKit JS 授权封装；BYOK 配置 + 测试连接
- M2：意图解析 → 候选检索 → 排序主链路与数据契约；候选检索**并发拉取 + 去重 + 池上限(150)**；搜索结果 **TTL 缓存**（key 含 storefront）；左右双栏 + 窄屏 tab 降级
- M3：右栏试听（预览片段）+ 勾选 + 建歌单请求；左栏 F10 多轮微调与「覆盖+保留已选+提示」状态同步
- M4：Gin release 模式 + 关闭代理信任；失败「重试」；F3 编辑意图 chip 后「重搜」
- 安全：三 Provider 错误归一化；golden-rule 过滤（丢弃池外/编造 id）；日志脱敏（实测 key 不入日志）
- 测试：后端 24 个单测（三 Provider mock 请求/解析/错误归一化、Apple 请求构造与缓存、并发去重、Developer Token 签发、golden-rule），`go test` 与 `-race` 全过

**待办（按文档路线图）**
- 真实凭证联调：Apple 授权弹窗 / 播放 / 搜索 / 建单在目标浏览器（含移动 Safari）实测（M1/M4）
- LLM/检索**限流**（M4）；候选检索**相关性调优**（关键词/种子权重）
- 订阅用户的 MusicKit **完整播放**（当前仅 30s 预览）
- F10 深化：rerank 目前在现有候选池内重排，可扩展为按指令重新检索
- 掉出选择的「是否保留」交互（当前为提示 + 暂移）
- F8 歌单历史 / F9 个性化（M5，非 MVP）

## 设计取舍（脚手架阶段已定）

- UI 库选 **Naive UI**，后端框架选 **Gin**（文档中的二选一）
- LLM key 只走请求头，非敏感配置（provider/baseUrl/model）走 body —— 让日志脱敏成为单一 header 规则
- 数据库暂不接入（F8 非 MVP），保留接入位
