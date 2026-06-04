import Foundation

/// 本地口味记录（UserDefaults，无 PII、不出设备）。从每次成功推荐记录
/// genres+moods+keywords，滚动窗口 40，用于空状态示例的个性化偏置。对应前端 personalize.ts。
enum TasteStore {
    private static let key = "hum.tastes.v1"
    private static let cap = 40

    static func record(_ tokens: [String]) {
        let cleaned = tokens.map { $0.trimmed }.filter { !$0.isEmpty }
        guard !cleaned.isEmpty else { return }
        let merged = Array((cleaned + load()).prefix(cap))
        UserDefaults.standard.set(merged, forKey: key)
    }

    /// 出现频次最高的前 n 个口味 token。
    static func top(_ n: Int) -> [String] {
        var freq: [String: Int] = [:]
        for t in load() { freq[t, default: 0] += 1 }
        return freq.sorted { $0.value > $1.value }.prefix(n).map { $0.key }
    }

    private static func load() -> [String] {
        UserDefaults.standard.stringArray(forKey: key) ?? []
    }
}
