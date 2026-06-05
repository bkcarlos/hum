import Foundation
import Combine

/// BYOK 配置（对应前端 llmConfig store + providers 预设）。
/// 非机密字段存 UserDefaults；**API key 只进 Keychain**（KeychainStore）。
@MainActor
final class LLMConfigStore: ObservableObject {
    struct Preset: Identifiable, Hashable {
        let id: String
        let label: String
        let type: ProviderType
        let baseUrl: String
        let models: [String]
        var isCustom: Bool = false
    }

    /// 预设表（与前端 data/providers.ts 对齐）。
    static let presets: [Preset] = [
        .init(id: "openai", label: "OpenAI", type: .openaiCompat, baseUrl: "https://api.openai.com/v1", models: ["gpt-4o-mini", "gpt-4o"]),
        .init(id: "deepseek", label: "DeepSeek", type: .openaiCompat, baseUrl: "https://api.deepseek.com/v1", models: ["deepseek-chat"]),
        .init(id: "qwen", label: "通义千问 (DashScope 兼容)", type: .openaiCompat, baseUrl: "https://dashscope.aliyuncs.com/compatible-mode/v1", models: ["qwen-plus", "qwen-turbo"]),
        .init(id: "moonshot", label: "Moonshot (Kimi)", type: .openaiCompat, baseUrl: "https://api.moonshot.cn/v1", models: ["moonshot-v1-8k"]),
        .init(id: "zhipu", label: "智谱 GLM", type: .openaiCompat, baseUrl: "https://open.bigmodel.cn/api/paas/v4", models: ["glm-4-flash", "glm-4"]),
        .init(id: "minimax", label: "MiniMax", type: .openaiCompat, baseUrl: "https://api.minimax.chat/v1", models: ["abab6.5s-chat"]),
        .init(id: "openrouter", label: "OpenRouter", type: .openaiCompat, baseUrl: "https://openrouter.ai/api/v1", models: ["openai/gpt-4o-mini"]),
        .init(id: "anthropic", label: "Anthropic Claude", type: .anthropic, baseUrl: "https://api.anthropic.com", models: ["claude-sonnet-4-6", "claude-haiku-4-5-20251001", "claude-opus-4-8"]),
        .init(id: "gemini", label: "Google Gemini", type: .gemini, baseUrl: "https://generativelanguage.googleapis.com", models: ["gemini-2.0-flash", "gemini-1.5-flash", "gemini-1.5-pro"]),
        .init(id: "custom", label: "自定义 OpenAI 兼容", type: .openaiCompat, baseUrl: "", models: [], isCustom: true),
    ]

    @Published var presetId: String
    @Published var provider: ProviderType
    @Published var baseUrl: String
    @Published var model: String
    @Published var apiKey: String
    @Published var tested: Bool = false
    @Published var availableModels: [ModelInfo] = []

    private let api: APIClient
    private let defaults = UserDefaults.standard
    private enum K {
        static let presetId = "hum.llm.presetId"
        static let provider = "hum.llm.provider"
        static let baseUrl = "hum.llm.baseUrl"
        static let model = "hum.llm.model"
    }

    init(api: APIClient) {
        self.api = api
        presetId = defaults.string(forKey: K.presetId) ?? "openai"
        provider = ProviderType(rawValue: defaults.string(forKey: K.provider) ?? "") ?? .openaiCompat
        baseUrl = defaults.string(forKey: K.baseUrl) ?? "https://api.openai.com/v1"
        model = defaults.string(forKey: K.model) ?? "gpt-4o-mini"
        apiKey = KeychainStore.load()
    }

    var configured: Bool {
        !apiKey.trimmed.isEmpty && !model.trimmed.isEmpty && !baseUrl.trimmed.isEmpty
    }

    var body: LlmBody {
        LlmBody(provider: provider, baseUrl: baseUrl.trimmed, model: model.trimmed)
    }

    static func preset(_ id: String) -> Preset? { presets.first { $0.id == id } }

    func applyPreset(_ id: String) {
        guard let p = LLMConfigStore.preset(id) else { return }
        presetId = id
        provider = p.type
        baseUrl = p.isCustom ? "" : p.baseUrl
        model = p.isCustom ? "" : (p.models.first ?? "")
        tested = false
        availableModels = []
        persist()
    }

    /// 持久化非机密字段到 UserDefaults，key 进 Keychain。
    func persist() {
        defaults.set(presetId, forKey: K.presetId)
        defaults.set(provider.rawValue, forKey: K.provider)
        defaults.set(baseUrl, forKey: K.baseUrl)
        defaults.set(model, forKey: K.model)
        KeychainStore.save(apiKey)
    }

    /// 测试连接（/llm/test）。
    func test() async -> Result<Void, APIError> {
        persist()
        do {
            _ = try await api.testLLM(body, apiKey: apiKey)
            tested = true
            return .success(())
        } catch let e as APIError {
            tested = false
            return .failure(e)
        } catch {
            tested = false
            return .failure(.network())
        }
    }

    /// 拉取可选模型（/llm/models，尽力而为；失败回退手动输入）。
    func fetchModels() async -> Result<Int, APIError> {
        persist()
        do {
            availableModels = try await api.listModels(body, apiKey: apiKey)
            return .success(availableModels.count)
        } catch let e as APIError {
            return .failure(e)
        } catch {
            return .failure(.network())
        }
    }

    /// 千人千面：有 key 时让 LLM 现编几条空状态示例；无 key/失败返回空（调用方回退本地池）。
    func fetchExamples(count: Int) async -> [String] {
        guard configured else { return [] }
        return (try? await api.examples(body, apiKey: apiKey,
                                        context: ExampleProvider.nowContext(),
                                        tastes: TasteStore.top(8), count: count)) ?? []
    }

    /// 合并预设 + 拉取的模型（去重、排序），供下拉用。
    var modelOptions: [String] {
        var seen = Set<String>()
        var out: [String] = []
        for m in availableModels where !m.id.isEmpty && seen.insert(m.id).inserted { out.append(m.id) }
        for id in LLMConfigStore.preset(presetId)?.models ?? [] where seen.insert(id).inserted { out.append(id) }
        return out.sorted()
    }

    /// 一键清除本地配置：删 Keychain 里的 key + 清 UserDefaults + 恢复默认预设。
    func wipe() {
        KeychainStore.delete()
        [K.presetId, K.provider, K.baseUrl, K.model].forEach { defaults.removeObject(forKey: $0) }
        apiKey = ""
        tested = false
        availableModels = []
        applyPreset("openai")   // 恢复默认 provider/baseUrl/model（applyPreset 内部已 persist）
    }
}
