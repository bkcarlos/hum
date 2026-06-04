import Foundation

/// 空状态示例（千人千面）。M-i1：本地静态池 + 上下文/口味偏置（无 LLM）。
/// 有 LLM key 时改走 /api/examples（M-i4 接入），无 key 时用 `pick(_:)` 本地挑选。
enum ExampleProvider {
    /// 静态示例池（与前端 EXAMPLE_POOL 对齐的中文场景）。
    static let pool: [String] = [
        "深夜一个人开车，放空的氛围电子",
        "睡前放松，纯钢琴轻音乐",
        "周末早晨做早餐，轻快有活力",
        "咖啡馆看书，慵懒的 bossa nova",
        "失恋后一个人，温柔的抒情慢歌",
        "健身房撸铁，带劲的节奏",
        "雨天加班，不吵的器乐爵士",
        "通勤路上，提神的英伦摇滚",
        "周末派对，热闹的舞曲",
        "专注写代码，无人声的 lo-fi",
    ]

    /// 当前时刻的人读上下文（时段/工作日/语言），喂给 /api/examples。
    static func nowContext() -> String {
        let cal = Calendar.current, now = Date()
        let h = cal.component(.hour, from: now)
        let part: String
        switch h {
        case 0..<6: part = "深夜"
        case 6..<11: part = "早晨"
        case 11..<14: part = "中午"
        case 14..<18: part = "下午"
        case 18..<23: part = "晚上"
        default: part = "深夜"
        }
        let weekday = cal.component(.weekday, from: now)
        let dayType = (weekday == 1 || weekday == 7) ? "周末" : "工作日"
        let lang = Locale.preferredLanguages.first ?? "zh"
        return "\(dayType)\(part)，地区语言：\(lang)"
    }

    /// 无 LLM key 时本地挑选：口味重叠（强）+ 当前时段关键词（弱）+ 抖动。冷启动≈随机。
    static func pick(_ n: Int) -> [String] {
        let tastes = TasteStore.top(12)
        let ctx = contextKeywords()
        return pool
            .map { ex -> (text: String, score: Double) in
                var score = Double.random(in: 0..<0.9)   // 抖动 < 1：仅打破并列/增加变化
                for t in tastes where t.count >= 2 && ex.contains(t) { score += 2 }
                for k in ctx where ex.contains(k) { score += 1 }
                return (ex, score)
            }
            .sorted { $0.score > $1.score }
            .prefix(n)
            .map { $0.text }
    }

    private static func contextKeywords() -> [String] {
        let cal = Calendar.current, now = Date()
        let h = cal.component(.hour, from: now)
        var keys: [String]
        switch h {
        case 0..<6, 23...: keys = ["深夜", "夜", "睡前", "放空", "放松"]
        case 6..<11: keys = ["早晨", "早餐", "通勤", "起床"]
        case 11..<18: keys = ["专注", "写代码", "加班", "咖啡馆"]
        default: keys = ["夜", "放松", "派对", "失恋"]
        }
        let weekday = cal.component(.weekday, from: now)
        if weekday == 1 || weekday == 7 { keys.append("周末") }
        return keys
    }
}
