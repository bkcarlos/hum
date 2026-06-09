import SwiftUI

/// 圆角一体化输入条：文本框 + 内嵌发送按钮（发送键在圆角容器内右下角）。
struct ComposerField: View {
    let placeholder: String
    @Binding var text: String
    var disabled: Bool = false
    let onSend: () -> Void

    @FocusState private var focused: Bool
    private var sendDisabled: Bool { disabled || text.trimmed.isEmpty }

    var body: some View {
        HStack(alignment: .center, spacing: 6) {
            TextField(placeholder, text: $text, axis: .vertical)
                .textFieldStyle(.plain)
                .focused($focused)
                // 失焦折叠回单行（不输入时紧凑），聚焦展开最多 4 行。
                .lineLimit(focused ? 1...4 : 1...1)
                .animation(.easeInOut(duration: 0.15), value: focused)
                .padding(.leading, 14)
                .padding(.vertical, 9)

            Button(action: onSend) {
                Image(systemName: "arrow.up.circle.fill").font(.title2)
            }
            .disabled(sendDisabled)
            .foregroundStyle(sendDisabled ? Color.secondary : BrandTheme.primary)
            .padding(.trailing, 6)
        }
        .background(
            RoundedRectangle(cornerRadius: 22, style: .continuous)
                .fill(Color(.secondarySystemBackground))
                .overlay(
                    RoundedRectangle(cornerRadius: 22, style: .continuous)
                        .strokeBorder(Color(.separator), lineWidth: 0.5)
                )
        )
    }
}
