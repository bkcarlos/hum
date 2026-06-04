import Foundation

/// 后端 LLM 链路客户端（选项 A 的边界 —— 未来纯客户端化只需替换本类内部）。
/// 所有方法把 BYOK key 注入 `X-LLM-Api-Key` 头，body 只放非机密的 `{llm, ...}`。
actor APIClient {
    private let baseURL: URL
    private let session: URLSession

    init(baseURL: URL = APIConfig.baseURL, session: URLSession? = nil) {
        self.baseURL = baseURL
        self.session = session ?? APIClient.makeSession()
    }

    private static func makeSession() -> URLSession {
        let cfg = URLSessionConfiguration.default
        cfg.timeoutIntervalForRequest = 60   // 首轮含两次 LLM 调用 + 后端解析，给足时间
        cfg.waitsForConnectivity = true
        return URLSession(configuration: cfg)
    }

    // MARK: - Endpoints

    func testLLM(_ llm: LlmBody, apiKey: String) async throws -> Bool {
        struct Body: Encodable { let llm: LlmBody }
        struct Resp: Decodable { let ok: Bool }
        let r: Resp = try await post("/llm/test", body: Body(llm: llm), apiKey: apiKey)
        return r.ok
    }

    func listModels(_ llm: LlmBody, apiKey: String) async throws -> [ModelInfo] {
        struct Body: Encodable { let llm: LlmBody }
        struct Resp: Decodable { let models: [ModelInfo] }
        let r: Resp = try await post("/llm/models", body: Body(llm: llm), apiKey: apiKey)
        return r.models
    }

    func suggest(_ llm: LlmBody, apiKey: String, storefront: String,
                 text: String, seedArtists: [String]) async throws -> SuggestResult {
        struct Body: Encodable {
            let llm: LlmBody; let storefront: String; let text: String; let seedArtists: [String]
        }
        return try await post("/suggest",
                              body: Body(llm: llm, storefront: storefront, text: text, seedArtists: seedArtists),
                              apiKey: apiKey)
    }

    func rank(_ llm: LlmBody, apiKey: String, intent: Intent,
              candidates: [Candidate], instruction: String = "") async throws -> RankResult {
        struct Body: Encodable {
            let llm: LlmBody; let intent: Intent; let candidates: [Candidate]; let instruction: String
        }
        return try await post("/rank",
                              body: Body(llm: llm, intent: intent, candidates: candidates, instruction: instruction),
                              apiKey: apiKey)
    }

    func examples(_ llm: LlmBody, apiKey: String, context: String,
                  tastes: [String], count: Int) async throws -> [String] {
        struct Body: Encodable {
            let llm: LlmBody; let context: String; let tastes: [String]; let count: Int
        }
        struct Resp: Decodable { let examples: [String] }
        let r: Resp = try await post("/examples",
                                     body: Body(llm: llm, context: context, tastes: tastes, count: count),
                                     apiKey: apiKey)
        return r.examples
    }

    // MARK: - Core

    /// POST `{path}`，注入 key 头，成功解 `{data:T}`；失败抛 APIError（解 `{error}` 或合成 network）。
    private func post<B: Encodable, T: Decodable>(_ path: String, body: B, apiKey: String?) async throws -> T {
        let segment = path.hasPrefix("/") ? String(path.dropFirst()) : path
        var req = URLRequest(url: baseURL.appendingPathComponent(segment))
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let apiKey, !apiKey.isEmpty {
            req.setValue(apiKey, forHTTPHeaderField: "X-LLM-Api-Key")  // BYOK：唯一携带 key 的地方
        }
        do {
            req.httpBody = try JSONEncoder().encode(body)
        } catch {
            throw APIError(code: "bad_request", message: "请求体编码失败。")
        }

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: req)
        } catch {
            throw APIError.network(error.localizedDescription)
        }

        guard let http = response as? HTTPURLResponse else { throw APIError.network() }

        if (200..<300).contains(http.statusCode) {
            do {
                return try JSONDecoder().decode(DataEnvelope<T>.self, from: data).data
            } catch {
                throw APIError(code: "upstream", message: "无法解析服务器响应。")
            }
        } else {
            if let env = try? JSONDecoder().decode(ErrorEnvelope.self, from: data) {
                throw env.error   // 后端 {error:{code,message}}，message 已是中文
            }
            throw APIError(code: "upstream", message: "服务器错误（HTTP \(http.statusCode)）。")
        }
    }
}
