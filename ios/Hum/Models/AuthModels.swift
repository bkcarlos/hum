import Foundation

/// 计费端点（/suggest·/rank）的鉴权方式：自带 Key 或 免费档会话。
/// 对应前端 client.ts 的 `LlmAuth = {apiKey} | {session}` 与 `authConfig()`。
enum LlmAuth {
    case byok(String)      // → X-LLM-Api-Key（用户自带 key，不限量）
    case session(String)   // → Authorization: Bearer（免费档会话，按配额）
}

/// GET /auth/me 的返回：当前会话的 Apple sub + 邮箱（授权 email scope 时有）+ 是否管理员。
/// iOS 只用 sub/email 做展示，admin 后台不进 App。
struct MeInfo: Decodable {
    let sub: String
    let email: String
    let isAdmin: Bool
}

/// POST /auth/apple 的返回：服务端会话令牌 + 有效期（秒）。identityToken 用完即弃、不回传。
struct AppleAuthResult: Decodable {
    let session: String
    let expiresInSeconds: Int
}
