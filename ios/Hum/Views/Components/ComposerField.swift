import SwiftUI

/// 圆角一体化输入条：文本框 + 内嵌发送按钮（发送键在圆角容器内右下角）。
struct ComposerField: View {
    let placeholder: String
    @Binding var text: String
    var disabled: Bool = false
    let onSend: () -> Void

    private var sendDisabled: Bool { disabled || text.trimmed.isEmpty }

    var body: some View {
        HStack(alignment: .center, spacing: 6) {
            TextField(placeholder, text: $text, axis: .vertical)
                .textFieldStyle(.plain)
                .lineLimit(1...4)
                .padding(.leading, 14)
                .padding(.vertical, 9)
                .keyboardDoneToolbar()   // 多行框回车=换行，需显式「收起」入口

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
