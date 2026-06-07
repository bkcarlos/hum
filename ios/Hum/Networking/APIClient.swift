import Foundation

/// 后端 LLM 链路客户端（选项 A 的边界 —— 未来纯客户端化只需替换本类内部）。
/// 计费端点按 `LlmAuth` 注入鉴权头：BYOK → `X-LLM-Api-Key`；免费档会话 → `Authorization: Bearer`。
/// body 只放非机密的 `{llm, ...}`，绝不带 key。
actor APIClient {
    private let baseURL: URL
    private let urlSession: URLSession

    init(baseURL: URL = APIConfig.baseURL, session: URLSession? = nil) {
        self.baseURL = baseURL
        self.urlSession = session ?? APIClient.makeSession()
    }

    private static func makeSession() -> URLSession {
        let cfg = URLSessionConfiguration.default
        cfg.timeoutIntervalForRequest = 60   // 首轮含两次 LLM 调用 + 后端解析，给足时间
        cfg.waitsForConnectivity = true
        return URLSession(configuration: cfg)
    }

    // MARK: - LLM 端点（BYOK 头；免费档无对应——examples 失败时调用方回退本地池）

    func testLLM(_ llm: LlmBody, apiKey: String) async throws -> Bool {
        struct Body: Encodable { let llm: LlmBody }
        struct Resp: Decodable { let ok: Bool }
        let r: Resp = try await post("/llm/test", body: Body(llm: llm), auth: .byok(apiKey))
        return r.ok
    }

    func listModels(_ llm: LlmBody, apiKey: String) async throws -> [ModelInfo] {
        struct Body: Encodable { let llm: LlmBody }
        struct Resp: Decodable { let models: [ModelInfo] }
        let r: Resp = try await post("/llm/models", body: Body(llm: llm), auth: .byok(apiKey))
        return r.models
    }

    func examples(_ llm: LlmBody, apiKey: String, context: String,
                  tastes: [String], count: Int) async throws -> [String] {
        struct Body: Encodable {
            let llm: LlmBody; let context: String; let tastes: [String]; let count: Int
        }
        struct Resp: Decodable { let examples: [String] }
        let r: Resp = try await post("/examples",
                                     body: Body(llm: llm, context: context, tastes: tastes, count: count),
                                     auth: .byok(apiKey))
        return r.examples
    }

    // MARK: - 计费端点（BYOK 或 免费档会话；后端 resolveProvider 二选一）

    func suggest(_ llm: LlmBody, auth: LlmAuth, storefront: String,
                 text: String, seedArtists: [String]) async throws -> SuggestResult {
        struct Body: Encodable {
            let llm: LlmBody; let storefront: String; let text: String; let seedArtists: [String]
        }
        return try await post("/suggest",
                              body: Body(llm: llm, storefront: storefront, text: text, seedArtists: seedArtists),
                              auth: auth)
    }

    func rank(_ llm: LlmBody, auth: LlmAuth, intent: Intent,
              candidates: [Candidate], instruction: String = "") async throws -> RankResult {
        struct Body: Encodable {
            let llm: LlmBody; let intent: Intent; let candidates: [Candidate]; let instruction: String
        }
        return try await post("/rank",
                              body: Body(llm: llm, intent: intent, candidates: candidates, instruction: instruction),
                              auth: auth)
    }

    // MARK: - Sign in with Apple / 会话

    /// 用 Apple identity token 换我们自己的会话令牌（POST /auth/apple，无鉴权头）。
    /// identityToken 用完即弃，不缓存、不回传。
    func exchangeAppleToken(_ identityToken: String) async throws -> AppleAuthResult {
        struct Body: Encodable { let identityToken: String }
        return try await post("/auth/apple", body: Body(identityToken: identityToken), auth: nil)
    }

    /// 当前会话的 sub/email/isAdmin（GET /auth/me，Bearer）。展示用。
    func getMe(session token: String) async throws -> MeInfo {
        try await get("/auth/me", auth: .session(token))
    }

    // MARK: - Core

    /// POST `{path}`，按 auth 注入鉴权头，成功解 `{data:T}`；失败抛 APIError。
    private func post<B: Encodable, T: Decodable>(_ path: String, body: B, auth: LlmAuth?) async throws -> T {
        var req = makeRequest(path, method: "POST", auth: auth)
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        do {
            req.httpBody = try JSONEncoder().encode(body)
        } catch {
            throw APIError(code: "bad_request", message: "请求体编码失败。")
        }
        return try await send(req)
    }

    /// GET `{path}`，按 auth 注入鉴权头。
    private func get<T: Decodable>(_ path: String, auth: LlmAuth?) async throws -> T {
        let req = makeRequest(path, method: "GET", auth: auth)
        return try await send(req)
    }

    /// 构造请求并注入鉴权头：BYOK→X-LLM-Api-Key（唯一携带 key 的地方）/ session→Bearer / nil→无。
    private func makeRequest(_ path: String, method: String, auth: LlmAuth?) -> URLRequest {
        let segment = path.hasPrefix("/") ? String(path.dropFirst()) : path
        var req = URLRequest(url: baseURL.appendingPathComponent(segment))
        req.httpMethod = method
        // 关联 id：每请求一个短随机串，发给后端(slog 记 request_id) + 写进本地诊断日志，
        // 「提交日志」里的 rid 就能在 Cloud Logging 里对上服务端那一行。非敏感、不透明。
        req.setValue(String(UUID().uuidString.prefix(8)).lowercased(), forHTTPHeaderField: "X-Request-Id")
        switch auth {
        case .byok(let key)?:
            if !key.isEmpty { req.setValue(key, forHTTPHeaderField: "X-LLM-Api-Key") }
        case .session(let token)?:
            if !token.isEmpty { req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization") }
        case nil:
            break
        }
        return req
    }

    /// 发请求 + 解包：2xx → `{data:T}`；否则解 `{error:{code,message}}` 或合成 network/upstream。
    private func send<T: Decodable>(_ req: URLRequest) async throws -> T {
        // 诊断日志：只记 method/path/status/错误码 + 关联 rid，绝不记 header(key/token)/body。
        let method = req.httpMethod ?? "?"
        let path = req.url?.path ?? "?"
        let rid = req.value(forHTTPHeaderField: "X-Request-Id") ?? "-"
        AppLog.shared.debug("net", "→ \(method) \(path) rid=\(rid)")

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await urlSession.data(for: req)
        } catch {
            AppLog.shared.error("net", "✗ \(method) \(path) rid=\(rid) 传输失败：\(error.localizedDescription)")
            throw APIError.network(error.localizedDescription)
        }

        guard let http = response as? HTTPURLResponse else {
            AppLog.shared.error("net", "✗ \(method) \(path) rid=\(rid) 非 HTTP 响应")
            throw APIError.network()
        }

        if (200..<300).contains(http.statusCode) {
            do {
                let decoded = try JSONDecoder().decode(DataEnvelope<T>.self, from: data).data
                AppLog.shared.info("net", "← \(method) \(path) \(http.statusCode) rid=\(rid)")
                return decoded
            } catch {
                AppLog.shared.error("net", "✗ \(method) \(path) \(http.statusCode) rid=\(rid) 响应解析失败")
                throw APIError(code: "upstream", message: "无法解析服务器响应。")
            }
        } else {
            if let env = try? JSONDecoder().decode(ErrorEnvelope.self, from: data) {
                AppLog.shared.error("net", "✗ \(method) \(path) \(http.statusCode) rid=\(rid) \(env.error.code)")
                throw env.error   // 后端 {error:{code,message}}，message 已是中文（额度/会话错误同此形）
            }
            AppLog.shared.error("net", "✗ \(method) \(path) \(http.statusCode) rid=\(rid)")
            throw APIError(code: "upstream", message: "服务器错误（HTTP \(http.statusCode)）。")
        }
    }
}
