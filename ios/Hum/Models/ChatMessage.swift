import Foundation

/// 左侧对话流里的一条消息（纯本地，不持久化）。
struct ChatMessage: Identifiable, Hashable {
    enum Role { case user, assistant }
    let id: Int
    let role: Role
    let text: String
}
