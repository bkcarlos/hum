import Foundation

/// /api/suggest（方案 A）的输出：已落地的真实候选池 + 展示用 intent + 解析统计。
/// candidates 每首都已被后端去 Apple 解析校验过（黄金原则）。
struct SuggestResult: Codable {
    let storefront: String
    let intent: Intent
    let candidates: [Song]
    let suggested: Int       // LLM 提名的总数
    let resolved: Int        // 实际在 Apple Music 命中的数
    let unresolved: [String] // 未命中的“歌手 - 歌名”（透明度）

    enum CodingKeys: String, CodingKey {
        case storefront, intent, candidates, suggested, resolved, unresolved
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        storefront = try c.decodeIfPresent(String.self, forKey: .storefront) ?? ""
        intent = try c.decodeIfPresent(Intent.self, forKey: .intent) ?? .empty()
        candidates = try c.decodeIfPresent([Song].self, forKey: .candidates) ?? []
        suggested = try c.decodeIfPresent(Int.self, forKey: .suggested) ?? 0
        resolved = try c.decodeIfPresent(Int.self, forKey: .resolved) ?? 0
        unresolved = try c.decodeIfPresent([String].self, forKey: .unresolved) ?? []
    }
}
