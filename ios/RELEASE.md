# Hum · iOS 发布手册

> iOS **不像 web 那样 push 自动部署**——必须在 Xcode 打包并上传到 App Store Connect。
> 本手册覆盖：前提 → 打包 → 分发（直装 / TestFlight / 上架）→ 本项目卡点 → 上架细节。
> 相关：根 [`CLAUDE.md`](../CLAUDE.md)（红线）、[`docs/freetier-roadmap.md`](../docs/freetier-roadmap.md)（免费档 / 上架障碍）、[`project.yml`](project.yml)（工程清单）。

## 0. 一次性前提

| 项 | 说明 |
|---|---|
| 付费开发者账号 | Apple Developer Program（$99/年）。免费账号只能直装、签名 7 天过期、**不能** TestFlight |
| App ID 能力 | 开发者后台给 `com.carlosbk.hum` 开 **Sign in with Apple**（不开 → 免费档登录 `/auth/apple` 返 401）。签名 Automatic，Xcode 自动管 Provisioning |
| 后端 env | 按 `docs/freetier-roadmap.md` §6.A 配好；尤其 `APPLE_BUNDLE_ID=com.carlosbk.hum`（= bundle id，原生 token 的 `aud`），否则免费档登录全失败 |
| App Store Connect | 走 TestFlight / 上架需先建 App 记录，bundle id 选 `com.carlosbk.hum` |

## 1. 选择发布方式

| 方式 | 适合 | 设备 | 更新 | 审核 | 账号 |
|---|---|---|---|---|---|
| Xcode 直装 | 单机快速验证 | 已注册真机 | 连线重装 | 无 | 免费/付费 |
| **TestFlight 内部测试** ⭐ | 自用 + 亲友 | ≤100 内部成员 | OTA 推新版 | 内部组**免审核** | 付费 |
| App Store 正式 | 公开分发 | 不限 | OTA | App Review | 付费 |

个人自用首选 **TestFlight 内部测试**：OTA 更新、装多台、内部组不过审。「公开上架」与本项目「个人自用 / 非商业」定位不符，除非确实要给陌生人用。

## 2. 每次发版（通用 5 步）

1. 改过 `.swift` / `project.yml` / 新增文件后：`cd ios && xcodegen generate`
2. 递增版本（改 `project.yml` 后再 `xcodegen generate`）：
   - `MARKETING_VERSION`：面向用户的版本（如 `0.6.0`）
   - `CURRENT_PROJECT_VERSION`：build 号，**每次上传 App Store Connect 必须唯一递增**（`2`→`3`…）
3. `open Hum.xcodeproj`，顶部目标选 **Any iOS Device (arm64)**（不能是模拟器）
4. **Product → Archive**
5. Organizer 窗口 → **Distribute App**

## 3. 分发（接 Distribute）

- **TestFlight / App Store**：选 *App Store Connect* → Upload → 等处理（几~几十分钟）→ App Store Connect → TestFlight 页 → 加内部测试组（你自己的 Apple ID）→ 手机装 TestFlight App 安装。
- **直装**：目标直接选连线真机 → Run（最快）；或 Distribute → *Release (Ad Hoc)* 导出 `.ipa` 装到已注册设备。

## 4. 本项目卡点（务必）

- **真机必测**：Apple Music 完整播放 / 建歌单 / MusicKit 在模拟器全不可用。顺带在真机走一遍后台音频、锁屏控件、Apple 连接开关（连/断）、建歌单 storefront 拦截。
- **Sign in with Apple**：App ID 开能力 + 后端 `APPLE_BUNDLE_ID == com.carlosbk.hum`，缺一免费档登录就失败。
- **导出合规**：`project.yml` 已加 `ITSAppUsesNonExemptEncryption: false`（仅 HTTPS 标准加密），上传不再被追问出口合规。

## 5. App Store 正式上架细节（仅公开发布需要）

> ⚠️ 本项目 PolyForm Noncommercial License + `CLAUDE.md` 红线 = 不收费 / 不内购 / 不接广告。App Store **免费分发**可以；任何变现都违反 License 与 Apple MusicKit ToS。

1. **完整性（Guideline 2.1）—— 最大的坎**：审核员不会填 LLM key，App 若空白会被打回。**必须先把免费档后端配好并默认开启**（`docs/freetier-roadmap.md` §6），让审核员一登录就能出推荐。这正是免费档存在的理由。
2. **MusicKit ToS**：不收费/内购/广告；播放须用户主动发起且有标准控件；不下载/转发音频；API 建的歌单私有、不经 API 公开；不绕过订阅。审核会查 Apple Music 用法。
3. **隐私营养标签**（App Store Connect → App 隐私）：声明收集 **邮箱地址**（Sign in with Apple，用途 = App 功能 / 账号管理），关联到用户身份。BYOK 的 LLM key 不上传、不收集，无需申报。
4. **App Review 备注**：给一条可登录的演示路径（Sign in with Apple 免费档即可），并说明「Apple Music 完整播放需审核员自己的订阅，无订阅时降级为 30s 试听」——避免因「播放不出来」被误判。
5. **元数据**：App 名称 / 副标题 / 关键词 / 描述 / 截图（至少 6.7" 与 6.5" iPhone；若支持 iPad 另加 iPad 截图）/ 隐私政策 URL（收集邮箱即需要）/ 年龄分级。
6. 提交 → App Review（通常 1–3 天）→ 通过后手动或自动发布。

## 6. project.yml 与版本号约定（重要）

`ios/project.yml` 在工作树里**长期带本地 diff、不入库**，因为它含你的私密签名信息：

- `DEVELOPMENT_TEAM`（真实 Apple Team ID）
- `bundleIdPrefix` / `PRODUCT_BUNDLE_IDENTIFIER`（你的 reverse-DNS）

仓库保留的是占位模板（`com.example.hum` / 空 Team）。因此 **`MARKETING_VERSION` / `CURRENT_PROJECT_VERSION` 也随 `project.yml` 在本地维护**——发版前本地 bump + 本地 `xcodegen generate`，不提交。

> 若希望 `project.yml` 干净入库、版本号纳入版本管理：把上述 3 个签名字段分离到一个 gitignore 的本地覆盖文件（XcodeGen `include:` 合并 / 或 `.xcconfig`）。当前未做——需要时再说。

## 7. 排查

| 现象 | 原因 / 处理 |
|---|---|
| 上传报「build number 已存在」 | `CURRENT_PROJECT_VERSION` 没递增 |
| 免费档登录 401 | App ID 没开 Sign in with Apple，或后端 `APPLE_BUNDLE_ID` ≠ bundle id |
| 完整播放 / 建歌单按钮灰着 | 未连 Apple Music 或无订阅（设计如此，降级 30s 试听）；模拟器不支持 |
| `git pull` / 改文件后 Xcode 找不到新文件 | 忘了 `cd ios && xcodegen generate`（`.xcodeproj` 不入库，靠它从 `project.yml` 重建） |
