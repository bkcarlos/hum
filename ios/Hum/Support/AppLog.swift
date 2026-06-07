import Foundation
import os

/// 轻量诊断日志：同时写**系统统一日志**（Xcode 控制台 / Console.app 可见）+ **内存环形缓冲**
/// （供 App 内「诊断日志」页查看 / 复制 / 分享）。线程安全，可从任意线程/actor 调用。
///
/// 红线（对齐后端 `middleware/logger.go`）：只记 method / path / status / 错误码等**非敏感**信息；
/// **绝不**记 API key、会话令牌、identityToken、Apple 邮箱原文、请求体。打点处自行保证传入文案安全。
final class AppLog: @unchecked Sendable {
    static let shared = AppLog()

    struct Entry: Identifiable {
        let id = UUID()
        let date: Date
        let category: String
        let level: String
        let message: String
    }

    enum Level: String { case debug = "DEBUG", info = "INFO", error = "ERROR" }

    private let subsystem = "com.carlosbk.hum"
    private let lock = NSLock()
    private var buffer: [Entry] = []
    private let cap = 500

    private init() {}

    func log(_ level: Level, _ category: String, _ message: String) {
        // 1) 系统统一日志。message 标 .public（已确保非敏感），否则 Console 里会显示 <private>。
        let logger = Logger(subsystem: subsystem, category: category)
        switch level {
        case .debug: logger.debug("\(message, privacy: .public)")
        case .info:  logger.info("\(message, privacy: .public)")
        case .error: logger.error("\(message, privacy: .public)")
        }
        // 2) 内存环形缓冲（供复制/分享）。
        let entry = Entry(date: Date(), category: category, level: level.rawValue, message: message)
        lock.lock()
        buffer.append(entry)
        if buffer.count > cap { buffer.removeFirst(buffer.count - cap) }
        lock.unlock()
    }

    func debug(_ category: String, _ message: String) { log(.debug, category, message) }
    func info(_ category: String, _ message: String)  { log(.info, category, message) }
    func error(_ category: String, _ message: String) { log(.error, category, message) }

    /// 当前缓冲快照（最旧在前）。
    func snapshot() -> [Entry] {
        lock.lock(); defer { lock.unlock() }
        return buffer
    }

    func clear() {
        lock.lock(); buffer.removeAll(); lock.unlock()
    }

    /// 缓冲导出为纯文本（供复制/分享）。设备/版本头由调用方（DiagnosticsView）拼接。
    func exportText() -> String {
        let f = DateFormatter()
        f.dateFormat = "MM-dd HH:mm:ss.SSS"
        let lines = snapshot().map { "\(f.string(from: $0.date)) [\($0.category)] \($0.level) \($0.message)" }
        return lines.isEmpty ? "（无日志）" : lines.joined(separator: "\n")
    }
}
