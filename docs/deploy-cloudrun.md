# 部署到 Google Cloud Run

Hum 是**单容器**（方案 1：一个 Go 进程同时托管前端 + `/api`）。Cloud Run 直接用仓库根目录的 `Dockerfile`，无需改代码。

## 一、你需要准备（一次性）

1. **Google Cloud 账号 + 一个项目（Project）**，并**开启结算（Billing）**。
   Cloud Run 有较大免费额度（个人自用通常覆盖），但必须绑定结算账号才能用。
2. **安装并登录 gcloud CLI**：
   ```bash
   # 安装见 https://cloud.google.com/sdk/docs/install
   gcloud auth login
   gcloud config set project <你的项目ID>
   ```
3. 手头有 Apple 的 **`AuthKey_XXXX.p8`** 文件，以及 **Team ID**、**Key ID**。

> 用户的 LLM key 不用在这里配——那是 BYOK，运行时从浏览器随请求带上。

## 二、一键部署（推荐）

脚本会：开启所需 API → 把 `.p8` 存入 Secret Manager → 授权运行时账号 → 用 Cloud Build 构建并部署。

```bash
GCP_PROJECT=你的项目ID \
APPLE_TEAM_ID=ABCDE12345 \
APPLE_KEY_ID=KEY1234567 \
P8_FILE=./AuthKey_KEY1234567.p8 \
GCP_REGION=asia-northeast1 \
./deploy/cloudrun.sh
```

首次运行 gcloud 可能提示创建 Artifact Registry 仓库，确认即可。完成后会打印服务 URL。

**区域就近选**（延迟更低）：
`asia-east1`(台湾) · `asia-east2`(香港) · `asia-northeast1`(东京) · `asia-southeast1`(新加坡) · `us-central1` · `europe-west1`

## 三、手动等价命令（脚本做的事）

```bash
gcloud services enable run.googleapis.com cloudbuild.googleapis.com \
  artifactregistry.googleapis.com secretmanager.googleapis.com

# .p8 → Secret Manager
gcloud secrets create apple-p8 --data-file=./AuthKey_KEY1234567.p8 --replication-policy=automatic

# 让 Cloud Run 运行时账号能读这个 secret
PROJNUM=$(gcloud projects describe <项目ID> --format='value(projectNumber)')
gcloud secrets add-iam-policy-binding apple-p8 \
  --member="serviceAccount:${PROJNUM}-compute@developer.gserviceaccount.com" \
  --role=roles/secretmanager.secretAccessor

# 构建 + 部署（从 Dockerfile）
gcloud run deploy hum \
  --source . --region asia-northeast1 --allow-unauthenticated --port 8080 \
  --cpu 1 --memory 512Mi --min-instances 0 --max-instances 3 \
  --set-env-vars "APPLE_TEAM_ID=...,APPLE_KEY_ID=...,GIN_MODE=release" \
  --set-secrets "APPLE_PRIVATE_KEY=apple-p8:latest"
```

- `WEB_DIR` / `PORT` 已在镜像里设好；Cloud Run 注入 `PORT=8080`，服务自动遵循。
- 同源（前端与 `/api` 同一个 `*.run.app` 域名），**无需 CORS**。

## 四、冷启动与成本

- 默认 `--min-instances 0`：闲置缩到 0，**省钱**；首个请求冷启动约 **1–3s**（Go 小镜像）。
- 本应用首个请求要跑两次 LLM + 一次检索（目标 < 8s）。介意首屏延迟就设 `MIN_INSTANCES=1`（常驻一个实例，成本略升但仍便宜）。

## 五、验证

```bash
curl https://<服务URL>/api/health     # 期望 {"data":{"status":"ok"}}
```
然后浏览器打开 `<服务URL>`：配置 LLM key → 连接 Apple Music → 描述 → 试听/勾选 → 建歌单。

## 六、更新 / 回滚

- 改完代码重新部署：再跑一次 `./deploy/cloudrun.sh`（或 `gcloud run deploy hum --source . --region <region>`）。
- 回滚：`gcloud run services update-traffic hum --to-revisions=<旧revision>=100 --region <region>`。

## 七、自动部署（可选 · GitHub Actions）

想"推到 main 自动部署"，推荐用 **Workload Identity Federation**（无长期密钥）：建一个 WIF pool/provider 绑定本仓库，再加一个 `.github/workflows/deploy-cloudrun.yml`。这步命令较多——需要的话我可以把 WIF 初始化脚本和 workflow 一起补上。

## 安全提醒

- `.p8` 只进 Secret Manager；`.gcloudignore` / `.dockerignore` 已排除 `*.p8` 与 `.env`，不会进镜像或上传。
- Team ID / Key ID 是标识符（非高敏），用普通环境变量即可。
- 用户 LLM key 仍是 BYOK：浏览器本地存、随请求 header 转发、后端用完即弃、不落库不记日志。
