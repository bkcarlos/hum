import SwiftUI

/// 对话气泡：user 右对齐主色底；assistant 左对齐浅底。
struct ChatBubble: View {
    let message: ChatMessage

    var body: some View {
        HStack {
            if message.role == .user { Spacer(minLength: 40) }
            Text(message.text)
                .padding(.horizontal, 12)
                .padding(.vertical, 8)
                .background(background)
                .foregroundStyle(foreground)
                .clipShape(RoundedRectangle(cornerRadius: BrandTheme.cornerRadius))
            if message.role == .assistant { Spacer(minLength: 40) }
        }
    }

    private var background: Color {
        message.role == .user ? BrandTheme.primary : Color(.secondarySystemBackground)
    }
    private var foreground: Color {
        message.role == .user ? .white : .primary
    }
}
