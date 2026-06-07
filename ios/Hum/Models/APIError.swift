import Foundation

/// 归一化的后端错误（对应 {"error":{"code,message}}）。code 为稳定机器串，
/// message 面向用户、已是中文、不含密钥。传输层失败合成为 code="network"。
struct APIError: Error, Codable, Equatable {
    let code: String
    let message: String

    static func network(_ message: String = "网络错误，请稍后重试。") -> APIError {
        APIError(code: "network", message: message)
    }

    /// 面向用户的文案：后端 message 已是中文则直接用，空则按 code 兜底。
    var userMessage: String {
        if !message.isEmpty { return message }
        switch code {
        case "no_key": return "未配置 LLM API Key，请先在设置里完成配置。"
        case "auth": return "API Key 无效或无权限，请检查 Key 是否正确。"
        case "quota": return "额度不足或计费异常，请检查账户余额。"
        case "rate_limit": return "请求过于频繁（限流），请稍后重试。"
        case "model_not_found": return "模型不存在或当前 Key 无权访问。"
        case "bad_request": return "请求无效。"
        case "network": return "无法连接服务器，请检查网络。"
        default: return "出错了（\(code)）。"
        }
    }

    /// 配置/接入类错误——应引导去「设置」（配置/切模式/登录）而非无意义重试：
    /// key 缺失或无效、模型不存在、请求被拒，以及免费档的额度用尽 / 未登录。
    var needsSetup: Bool {
        ["no_key", "auth", "bad_request", "model_not_found", "quota_exceeded", "no_session"].contains(code)
    }
}
