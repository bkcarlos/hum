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

## 七、自动部署（推到 main 自动发布）

### 方式 A · Cloud Run 内置「持续部署」（最简单，推荐）

全程控制台，不用写 CI、不碰密钥文件：

1. **先手动部署一次**（`./deploy/cloudrun.sh`），确保服务、`apple-p8` 密钥、Artifact Registry 仓库都已就绪。
2. Cloud Run 控制台 → 打开服务 `hum` → 顶部 **「设置持续部署 / Set up continuous deployment」**。
3. 连接 GitHub（首次授权 Cloud Build 的 GitHub App）→ 选仓库 `bkcarlos/hum`、分支 `^main$`、**Build type 选 Dockerfile**。保存后会自动建一个 Cloud Build 触发器。
4. 确认服务的「变量与密钥」已设：`APPLE_TEAM_ID`、`APPLE_KEY_ID`、`GIN_MODE=release`（环境变量），`APPLE_PRIVATE_KEY` ← 引用 Secret Manager 的 `apple-p8`。

此后每次 `git push` 到 main → 自动构建镜像、发布新版本。
**关键点**：自动部署只更新镜像，`gcloud run deploy` 默认**保留**服务已有的环境变量/密钥，所以上面那些设一次就够。

### 方式 B · 配置进 Git（`cloudbuild.yaml` + 触发器，可复现）

想把部署参数纳入版本控制，用仓库根目录的 [`cloudbuild.yaml`](../cloudbuild.yaml)：

1. **一次性**：连接 GitHub 到 Cloud Build（Console: Cloud Build → 触发器 → 连接仓库，授权 `bkcarlos/hum`）。
2. **给构建服务账号授权**（部署 Cloud Run + 以运行时账号身份部署）：
   ```bash
   PROJNUM=$(gcloud projects describe <项目ID> --format='value(projectNumber)')
   BUILD_SA="${PROJNUM}@cloudbuild.gserviceaccount.com"   # 若用 Compute 默认 SA 构建则换成它
   gcloud projects add-iam-policy-binding <项目ID> \
     --member="serviceAccount:${BUILD_SA}" --role=roles/run.admin
   gcloud iam service-accounts add-iam-policy-binding \
     "${PROJNUM}-compute@developer.gserviceaccount.com" \
     --member="serviceAccount:${BUILD_SA}" --role=roles/iam.serviceAccountUser
   ```
3. **建触发器**，指向 `cloudbuild.yaml` 并填替换变量：
   ```bash
   gcloud builds triggers create github \
     --name=hum-deploy --repo-name=hum --repo-owner=bkcarlos \
     --branch-pattern='^main$' --build-config=cloudbuild.yaml \
     --substitutions=_REGION=asia-northeast1,_APPLE_TEAM_ID=ABCDE12345,_APPLE_KEY_ID=KEY1234567
   ```
   `.p8` 仍只在 Secret Manager，由 `cloudbuild.yaml` 的 `--set-secrets` 引用，不进仓库。

> 方式 B 的构建账号 IAM 因项目而异（经典 Cloud Build SA vs Compute 默认 SA）；若报权限错，按提示给对应 SA 补 `run.admin` / `iam.serviceAccountUser`，并确认运行时 SA 有 `secretmanager.secretAccessor`。嫌麻烦就用方式 A。

### GitHub Actions（想在 GitHub 侧跑 CI 时）

也可用 `google-github-actions/auth`（推荐 Workload Identity Federation，免长期密钥）跑 `gcloud run deploy`。需先建 WIF 并绑仓库——需要的话我把 workflow + WIF 初始化命令补上。

## 安全提醒

- `.p8` 只进 Secret Manager；`.gcloudignore` / `.dockerignore` 已排除 `*.p8` 与 `.env`，不会进镜像或上传。
- Team ID / Key ID 是标识符（非高敏），用普通环境变量即可。
- 用户 LLM key 仍是 BYOK：浏览器本地存、随请求 header 转发、后端用完即弃、不落库不记日志。
