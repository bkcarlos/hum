# 从零配置「网页版 Sign in with Apple + 免费额度」

> 目标:让 https://hum-github-638000578981.asia-east2.run.app 的「登录用免费额度」真正能点、能用。
> 代码已全部就位并上线,**这份只讲配置**(Apple 开发者后台 + Cloud Run env),不用改代码。
> 关键前提:Hum 网页登录**只验身份令牌(JWKS 验签)**,**不需要 Sign in with Apple 的 `.p8` 密钥**。

设三处:**A. Apple 标识符** → **B. 域名验证** → **C. Cloud Run env**。最后 D 测试、E 设管理员。

---

## A. Apple 开发者后台

developer.apple.com → Account → **Certificates, Identifiers & Profiles**。

### A1. App ID(必须先有,且开启 Sign in with Apple)
> 你截图里 "No App ID is available" 就是因为这步没做。

1. **Identifiers** → 点 **+** → 选 **App IDs** → Continue → 选 **App** → Continue。
2. **Description**:`Hum`;**Bundle ID** 选 **Explicit**,填 `com.carlosbk.hum`。
3. 往下 **Capabilities**,勾选 **Sign in with Apple**(保持默认 "Enable as a primary App ID")。
4. Continue → Register。
   - 记住这个 Bundle ID:`com.carlosbk.hum` = 后端 **`APPLE_BUNDLE_ID`**。
> 若你已经有这个 App ID,只是没勾 Sign in with Apple:点进去 → Capabilities 勾上 → Save 即可。

### A2. Services ID(网页用的 client_id)
1. **Identifiers** → **+** → 选 **Services IDs** → Continue。
2. **Description**:`Hum Web`;**Identifier**:`com.carlosbk.hum.web`(必须与 App ID 不同)→ Continue → Register。
   - 这个标识符 `com.carlosbk.hum.web` = 后端 **`APPLE_WEB_CLIENT_ID`**。

### A3. 给 Services ID 配 Web 认证
1. 点进刚建的 Services ID → 勾选 **Sign in with Apple** → 点 **Configure**。
2. **Primary App ID**:选 A1 的 `com.carlosbk.hum`(现在能选了,因为它开了 Sign in with Apple)。
3. **Domains and Subdomains**:`hum-github-638000578981.asia-east2.run.app`(不带 https://)
4. **Return URLs**:`https://hum-github-638000578981.asia-east2.run.app`(带 https://,**不带尾斜杠**)
5. Next → 这里 Apple 多半要你**验证域名**(见 B);验证通过后 → Done → Continue → **Save**。

> ⚠️ **Return URL 必须和后端 `APPLE_WEB_REDIRECT_URI` 一字不差**。本指南统一用**无尾斜杠**的根 URL。

---

## B. 域名验证(Apple 给一个文件,挂到网站上)

Apple 会让你下载 `apple-developer-domain-association.txt`,要求能从
`https://hum-github-638000578981.asia-east2.run.app/.well-known/apple-developer-domain-association.txt` 访问到。

Hum 不用改后端:前端静态托管会在 SPA fallback **之前**返回真实存在的文件,而 Vite 会把
`frontend/public/` 原样拷进发布目录。所以:

1. 在 Apple 配置页下载 `apple-developer-domain-association.txt`。
2. 放到仓库 `frontend/public/.well-known/apple-developer-domain-association.txt`。
3. commit + push → 等部署上线(约几分钟)。
4. 浏览器访问上面那个 URL 能看到文件内容 → 回 Apple 点 **Verify**。

> 把文件内容发我,我来放进仓库并提交;或你自己放。

---

## C. Cloud Run(打开免费档后端)

> 自动部署默认**保留**已有 env/secret,只换镜像;这里用 `gcloud run services update` 增量加。
> 前提:你有一把**服务端自己的 LLM key**(OpenAI / DeepSeek / 通义 等,免费档烧的是它)。

```bash
PROJ=hum-runables; SVC=hum-github; REGION=asia-east2
SA=hum-github@hum-runables.iam.gserviceaccount.com

# C1. Firestore(Native)+ 给运行时 SA 读写权限
gcloud firestore databases create --location=$REGION --project $PROJ
gcloud projects add-iam-policy-binding $PROJ \
  --member=serviceAccount:$SA --role=roles/datastore.user

# C2. 两个密钥进 Secret Manager(像现有的 apple-p8 一样),并授权 SA 读取
printf '%s' "<你的服务端 LLM key>" | gcloud secrets create default-llm-key --data-file=- --project $PROJ
printf '%s' "$(openssl rand -hex 32)" | gcloud secrets create session-secret --data-file=- --project $PROJ
for s in default-llm-key session-secret; do
  gcloud secrets add-iam-policy-binding $s --project $PROJ \
    --member=serviceAccount:$SA --role=roles/secretmanager.secretAccessor
done

# C3. 注入 env + secret(按你的 LLM 服务商改 BASE_URL / MODEL)
gcloud run services update $SVC --region $REGION --project $PROJ \
  --update-secrets DEFAULT_LLM_API_KEY=default-llm-key:latest,SESSION_SECRET=session-secret:latest \
  --update-env-vars DEFAULT_LLM_PROVIDER=openai-compat,DEFAULT_LLM_BASE_URL=https://api.openai.com/v1,DEFAULT_LLM_MODEL=gpt-4o-mini,APPLE_BUNDLE_ID=com.carlosbk.hum,APPLE_WEB_CLIENT_ID=com.carlosbk.hum.web,APPLE_WEB_REDIRECT_URI=https://hum-github-638000578981.asia-east2.run.app,FIRESTORE_PROJECT=hum-runables
```

可选额度调节(有默认值,可不设):`FREE_TIER_PER_USER_DAILY`(默认 20)、`FREE_TIER_GLOBAL_DAILY`(默认 0=不限)。

---

## D. 验证

```bash
BASE=https://hum-github-638000578981.asia-east2.run.app
curl -s $BASE/api/auth/apple/web        # 期望 {"data":{"enabled":true,"clientId":"com.carlosbk.hum.web",...}}
```
然后打开 $BASE → 「接入设置」→ **免费额度(Apple 登录)** → **通过 Apple 登录** → 授权 → 回到页面已登录 → 发一句"雨天爵士"应能出歌、可 30s 试听。额度用尽会提示切「自带 Key」。

## E. 把自己设成管理员(可选,用 /admin 后台)
1. 登录后拿 session(浏览器 DevTools → Application → Local Storage → `hum.session`),或:
   ```bash
   curl -s -H "Authorization: Bearer <session>" $BASE/api/auth/me   # 返回你的 sub
   ```
2. Firestore 控制台 → `humQuota/config` 文档 → 给 `admins` 数组加上你的 `sub` → 保存。
3. 打开 $BASE/admin → 用 Apple 登录即可进后台(改额度/看用量/封禁)。

---

## 标识符对照表

| 你创建的 | 值(示例) | 对应后端 env |
|---|---|---|
| App ID(Bundle ID) | `com.carlosbk.hum` | `APPLE_BUNDLE_ID` |
| Services ID | `com.carlosbk.hum.web` | `APPLE_WEB_CLIENT_ID` |
| Return URL | `https://hum-github-638000578981.asia-east2.run.app` | `APPLE_WEB_REDIRECT_URI`(须一字不差) |

> 不需要:Sign in with Apple 的 `.p8` 密钥(Hum 只验身份令牌,不做服务端令牌交换)。
