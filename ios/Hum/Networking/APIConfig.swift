import Foundation

/// 后端 API 基址。运行时从 Info.plist 的 `HUMApiBase` 读取
/// （由 project.yml 的 `HUM_API_BASE` 构建设置注入），便于 dev/prod 分环境。
enum APIConfig {
    /// 兜底生产地址（Cloud Run）。
    private static let fallback = "https://hum-github-638000578981.asia-east2.run.app/api"

    static var baseURL: URL {
        let raw = (Bundle.main.object(forInfoDictionaryKey: "HUMApiBase") as? String) ?? ""
        let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        if !trimmed.isEmpty, let url = URL(string: trimmed) {
            return url
        }
        return URL(string: fallback)!
    }
}
