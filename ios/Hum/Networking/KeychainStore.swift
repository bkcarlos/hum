import Foundation
import Security

/// BYOK LLM key 的本地安全存储。
/// 红线：仅本机、**不同步 iCloud**（kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly），
/// 等价 Web 的 localStorage-only。按需读取、只用于 X-LLM-Api-Key 头，绝不进 body、绝不附到其他请求。
enum KeychainStore {
    private static let service = "com.example.hum.llmKey"   // 与 bundle id 无关，稳定即可
    private static let account = "byok"

    static func save(_ key: String) {
        delete() // 幂等：先删后写
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
            kSecValueData as String: Data(key.utf8),
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
        ]
        SecItemAdd(query as CFDictionary, nil)
    }

    static func load() -> String {
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

    static func delete() {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
        SecItemDelete(query as CFDictionary)
    }
}
