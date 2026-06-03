# CLAUDE.md

面向 AI 代理的项目工作指南。**部署/平台对比等长篇细节见 [README.md](README.md)；范围、F-编号、里程碑、设计决策以 [docs/requirements.md](docs/requirements.md)（v0.5）为单一事实来源。** 本文件只放日常干活要用的命令、约定与红线。

## 一句话

**Hum · 哼一首**：用户用自然语言描述心情/场景 → LLM 解析意图、从 **Apple Music 真实曲库**检索候选并排序 → 用户试听、勾选、一键建歌单。个人自用、不商业化。

## 不可破坏的红线（改动前必读）

1. **黄金原则——Apple 才是"歌是否存在"的事实来源，LLM 不自行断定。** 自 2026-06-03 起采用**方案 A**：① LLM **提名**真实歌曲（歌名+歌手）并附展示用 `Intent`（`POST /api/suggest` → `llm.SuggestSongs`）；② 后端逐条**去 Apple 解析校验**（`applemusic.Client.ResolveSong`：标题归一化匹配 + 歌手软匹配），**解析不到的直接丢弃** → 得到全是真实曲目的候选池；③ LLM 再对池**排序**（`/api/rank` → `RankSongs`，仍丢弃池外 id）。两道关卡（`ResolveSong` + `RankSongs` 的 id 过滤）杜绝编造/张冠李戴。旧的关键词检索 `/api/apple/search` + `buildSearchTerms` 仍在但 UI 不再调用——它对 vibe 查询会搜空，正是方案 A 取代它的原因。
2. **BYOK key 用完即弃。** 用户 LLM key **只**经 `X-LLM-Api-Key` 请求头传入，单次请求内使用，**绝不落库/缓存/写日志**。日志中间件 [`backend/internal/middleware/logger.go`](backend/internal/middleware/logger.go) 只记录 method/path/status，并维护敏感 header 名单。非敏感配置（provider/baseUrl/model）走 body——让脱敏成为单一 header 规则。Gemini 用 `x-goog-api-key` 头而非 `?key=`。`config` 里**不允许**出现任何 LLM key。
3. **Apple MusicKit ToS。** 不收费/不接广告/不内购；播放须用户主动发起且有标准控件；不下载/转发音频文件；API 建的歌单私有、不可经 API 公开；不绕过订阅。

> 任何编辑触及 LLM 排序、`handlers/`、`middleware/`、日志或新功能时，逐条对照上面三点。

## 常用命令

```bash
# 开发（根目录 Makefile）
make dev-backend     # cd backend && go run ./cmd/server   → :8080
make dev-frontend    # cd frontend && npm run dev           → :5173（Vite 代理 /api → :8080）
make install         # 装前端依赖

# 测试 / 构建
make test            # 后端 go vet ./... && go test ./...  +  前端 vue-tsc --noEmit
make build           # 后端 go build -o bin/server ./cmd/server  +  前端 vite build
cd backend && go test -race ./...   # 并发相关改动务必跑 -race
cd frontend && npm run type-check   # 只做类型检查（build 也会先跑它）

# 本地跑“生产形态”（单服务、同源、无 CORS）
cd frontend && npm run build && cd ..
WEB_DIR="$PWD/frontend/dist" GIN_MODE=release PORT=8080 go -C backend run ./cmd/server
```

## 架构

- **frontend/**：Vue 3 + Vite + TS + Pinia + Naive UI + axios。双栏 UI（左对话/右歌单），窄屏降级为 tab。
- **backend/**：Go 1.23 + Gin（module `github.com/bkcarlos/hum`，入口 `cmd/server/main.go`）。
- **生产=单进程**：同一个 Go 进程既托管前端静态文件（`WEB_DIR`，SPA fallback）又提供 `/api`，**同源、无 CORS**。`WEB_DIR` 为空时即开发模式（前端由 Vite 提供）。
- Apple 凭证在启动时**可选**：未配置也能启动，`/api/apple/*` 返回 **503**，前端 + LLM 接口照常工作。
- 默认 `gin.ReleaseMode`（设 `GIN_MODE=debug` 才回到 verbose）；默认**不信任任何代理头**（`SetTrustedProxies(nil)`）。

## 后端约定

**响应封装**（[`internal/httpx/respond.go`](backend/internal/httpx/respond.go)）：成功 `{"data": ...}`，失败 `{"error":{"code","message"}}`。`code` 是稳定的机器可读串（如 `auth`/`rate_limit`），`message` 面向用户、**不得含密钥**。前端 [`api/client.ts`](frontend/src/api/client.ts) 会自动拆 `.data.data`、把错误归一化成 `{code,message}`。

**API 路由**（全部挂在 `/api`，仅 `GET`/`POST`）：

| Method | Path | 处理器 | 说明 |
|---|---|---|---|
| GET  | `/api/health` | — | 健康检查 |
| GET  | `/api/apple/developer-token` | `DeveloperToken` | 签发 ES256 Developer Token（含缓存） |
| POST | `/api/apple/search` | `Search` | `{storefront, intent}` → 真实候选池（并发拉取+去重，池上限 150，TTL 缓存 key 含 storefront） |
| POST | `/api/apple/playlists` | `CreatePlaylist` | 建歌单；需 `Music-User-Token` 头 |
| POST | `/api/llm/test` | `TestLLM` | 测试 BYOK key/连通性（Provider.Ping） |
| POST | `/api/intent` | `ParseIntent` | 自然语言 → `Intent` |
| POST | `/api/rank` | `Rank` | 候选池 → `RankResult`（排序+理由+歌单名） |
| POST | `/api/suggest` | `Suggest` | **方案 A**：LLM 提名歌曲 → 逐条 Apple 解析校验 → 真实候选池（+ intent + 未命中清单）；前端主链路用它取代 `/apple/search` |

**LLM 层**（`internal/llm/`）：统一接口 `Provider{ ParseIntent, SuggestSongs, RankSongs, Ping }`（`SuggestSongs` 为方案 A 新增），三适配器 `openai-compat` / `anthropic` / `gemini`，由 `llm.New(Config)` 工厂按 provider 填默认 BaseURL/Model。`Config.APIKey` 是请求作用域密钥，随 Provider 实例随请求销毁。数据契约：`Intent{moods,genres,instruments,tempo,keywords,seed_artists}`、`Candidate{id,title,artist,album,genres,year,hasLyrics,contentRating}`、`RankResult{playlist_name,description,songs[]{id,reason}}`。`Song` 还带 `releaseDate/contentRating/hasLyrics/isrc/composer`（同一次 Apple 响应白送，喂排序+落地"去掉有歌词的""不要露骨的"等过滤；Apple 无 BPM/energy）。错误三家归一化见 `errors.go`。

**配置/env**（[`internal/config/config.go`](backend/internal/config/config.go)，可读 `.env`，进程已有 env 优先）：

| 变量 | 默认 | 说明 |
|---|---|---|
| `APPLE_TEAM_ID` / `APPLE_KEY_ID` | — | Apple 开发者凭证 |
| `APPLE_PRIVATE_KEY` | — | .p8 的 PEM 内容（云平台首选；支持 `\n` 转义粘贴） |
| `APPLE_PRIVATE_KEY_PATH` | — | 或本地 .p8 文件路径 |
| `APPLE_TOKEN_TTL_HOURS` | 4320 | Developer Token 有效期 |
| `APPLE_API_BASE` | `https://api.music.apple.com` | 测试/区域代理可覆盖 |
| `PORT` | 8080 | 多数云平台会注入自己的 PORT |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | 单服务同源可不设 |
| `UPSTREAM_TIMEOUT_SECONDS` | 30 | 上游 HTTP 超时 |
| `SEARCH_CACHE_TTL_SECONDS` | 600 | 检索缓存（0 关闭） |
| `WEB_DIR` | — | 设了就托管前端静态文件（单服务） |
| `GIN_MODE` | release | 设 `debug` 回到 verbose |

## 前端约定

- Pinia stores：`llmConfig`（BYOK 配置，key 存浏览器 localStorage）/ `apple`（Music User Token 仅存会话内存，不持久化）/ `conversation` / `playlist`。
- 所有后端调用走 [`api/client.ts`](frontend/src/api/client.ts)；BYOK key 经 `keyHeader()` 注入 `X-LLM-Api-Key`，**别在别处缓存 key**。
- MusicKit JS 封装在 `services/musickit.ts`；推荐编排/预览播放在 `composables/`（`useRecommendation` / `usePreviewPlayer`）。当前仅 30s 预览，完整播放待办。
- BYOK Provider 预设表在 `data/providers.ts`。

## 关键文件地图

```
backend/cmd/server/main.go          入口：路由 / CORS / release / 托管前端
backend/internal/applemusic/        Developer Token 签发 + Catalog 搜索 + 建歌单
backend/internal/llm/               Provider 接口 + 三适配器 + 错误归一化
backend/internal/handlers/          HTTP 处理器（从 header 读 key、用完即弃）
backend/internal/{cache,httpx,middleware,config}/
frontend/src/{api,stores,composables,components,services,data}/
```

## 沟通

项目文档与代码注释均为中文；默认用中文回复（见用户记忆）。
