import Foundation

/// 本地口味记录（UserDefaults，无 PII、不出设备）。从每次成功推荐记录
/// genres+moods+keywords，滚动窗口 40，用于空状态示例的个性化偏置。对应前端 personalize.ts。
enum TasteStore {
    private static let key = "hum.tastes.v1"
    private static let dislikeKey = "hum.dislikes.v1"   // 负向口味：删除歌曲时记录的风格/歌手
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

    // MARK: - 负向口味（删除信号）

    /// 记录"不喜欢"的 token（删除歌曲的风格/歌手），滚动窗口同样 cap。
    static func recordDislike(_ tokens: [String]) {
        let cleaned = tokens.map { $0.trimmed }.filter { !$0.isEmpty }
        guard !cleaned.isEmpty else { return }
        let merged = Array((cleaned + loadDislikes()).prefix(cap))
        UserDefaults.standard.set(merged, forKey: dislikeKey)
    }

    /// 撤销删除时回收对应的负向 token（每个最多移除一次出现），保持与实际删除一致。
    static func unrecordDislike(_ tokens: [String]) {
        var list = loadDislikes()
        for t in tokens.map({ $0.trimmed }).filter({ !$0.isEmpty }) {
            if let idx = list.firstIndex(of: t) { list.remove(at: idx) }
        }
        UserDefaults.standard.set(list, forKey: dislikeKey)
    }

    /// 出现频次最高的前 n 个负向 token，用于下次推荐时提示"尽量避免"。
    static func topDislikes(_ n: Int) -> [String] {
        var freq: [String: Int] = [:]
        for t in loadDislikes() { freq[t, default: 0] += 1 }
        return freq.sorted { $0.value > $1.value }.prefix(n).map { $0.key }
    }

    private static func load() -> [String] {
        UserDefaults.standard.stringArray(forKey: key) ?? []
    }
    private static func loadDislikes() -> [String] {
        UserDefaults.standard.stringArray(forKey: dislikeKey) ?? []
    }
}
