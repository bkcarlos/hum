import Foundation

/// BYOK 服务商类型（与后端 ProviderType 字符串一致）。
enum ProviderType: String, Codable, CaseIterable, Identifiable {
    case openaiCompat = "openai-compat"
    case anthropic
    case gemini
    var id: String { rawValue }
}

/// 请求体里携带的**非机密** LLM 配置。API Key **不在这里**，单独走 X-LLM-Api-Key 头。
struct LlmBody: Codable {
    let provider: ProviderType
    let baseUrl: String
    let model: String
}

/// /api/llm/models 返回的可选模型（live 目录，配置页用）。
struct ModelInfo: Codable, Identifiable, Hashable {
    let id: String
    let displayName: String?
}
