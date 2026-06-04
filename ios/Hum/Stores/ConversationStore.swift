import Foundation
import Combine

/// 左侧对话流 + 当前 Intent（对应前端 conversation store）。
@MainActor
final class ConversationStore: ObservableObject {
    @Published var messages: [ChatMessage] = []
    @Published var intent: Intent = .empty()
    @Published var hasIntent: Bool = false

    private var nextId = 0

    func addUser(_ text: String) { append(.user, text) }
    func addAssistant(_ text: String) { append(.assistant, text) }

    func setIntent(_ i: Intent) {
        intent = i
        hasIntent = !i.isEmpty
    }

    func reset() {
        messages = []
        intent = .empty()
        hasIntent = false
        nextId = 0
    }

    private func append(_ role: ChatMessage.Role, _ text: String) {
        let t = text.trimmed
        guard !t.isEmpty else { return }
        messages.append(ChatMessage(id: nextId, role: role, text: t))
        nextId += 1
    }
}
