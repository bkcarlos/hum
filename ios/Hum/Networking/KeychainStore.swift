import Foundation
import Security

/// 本地安全存储：BYOK LLM key（account "byok"）+ 免费档会话令牌（account "session"）。
/// 红线：仅本机、**不同步 iCloud**（kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly），
/// 等价 Web 的 localStorage-only。key 按需读取、只用于 X-LLM-Api-Key 头；会话只用于
/// Authorization: Bearer。绝不进 body、绝不附到不相关的请求。
enum KeychainStore {
    private static let service = "com.example.hum.llmKey"   // 与 bundle id 无关，稳定即可

    static func save(_ value: String, account: String = "byok") {
        delete(account: account) // 幂等：先删后写
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: Data(value.utf8),
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
        ]
        SecItemAdd(query as CFDictionary, nil)
    }

    static func load(account: String = "byok") -> String {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var item: CFTypeRef?
        guard SecItemCopyMatching(query as CFDictionary, &item) == errSecSuccess,
              let data = item as? Data,
              let str = String(data: data, encoding: .utf8) else {
            return ""
        }
        return str
    }

    static func delete(account: String = "byok") {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
        SecItemDelete(query as CFDictionary)
    }
}
