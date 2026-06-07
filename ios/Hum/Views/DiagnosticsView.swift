import SwiftUI
import UIKit

/// 诊断日志页：查看最近的请求/登录日志，一键**复制**或**分享/提交**（不含任何密钥/令牌）。
/// 快照式（复现问题后进来「刷新」即可），避免实时刷新带来的并发复杂度。
struct DiagnosticsView: View {
    @State private var entries: [AppLog.Entry] = []
    @State private var shareText = ""
    @State private var copied = false

    private static let tf: DateFormatter = {
        let f = DateFormatter(); f.dateFormat = "HH:mm:ss.SSS"; return f
    }()

    var body: some View {
        List {
            if entries.isEmpty {
                Text("暂无日志。复现一次问题后回到这里点「刷新」。")
                    .font(.callout).foregroundStyle(.secondary)
            } else {
                ForEach(entries) { e in
                    VStack(alignment: .leading, spacing: 2) {
                        Text(e.message)
                            .font(.system(.caption, design: .monospaced))
                            .foregroundStyle(color(for: e.level))
                            .textSelection(.enabled)
                        Text("\(Self.tf.string(from: e.date)) · \(e.category)")
                            .font(.caption2).foregroundStyle(.secondary)
                    }
                    .listRowInsets(EdgeInsets(top: 4, leading: 12, bottom: 4, trailing: 12))
                }
            }
        }
        .listStyle(.plain)
        .navigationTitle("诊断日志")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar {
            ToolbarItem(placement: .topBarLeading) {
                Button("刷新") { refresh() }
            }
            ToolbarItemGroup(placement: .topBarTrailing) {
                ShareLink(item: shareText) { Image(systemName: "square.and.arrow.up") }
                Button {
                    UIPasteboard.general.string = shareText
                    copied = true
                } label: {
                    Image(systemName: copied ? "checkmark" : "doc.on.doc")
                }
                Button(role: .destructive) { AppLog.shared.clear(); refresh() } label: {
                    Image(systemName: "trash")
                }
            }
        }
        .onAppear { refresh() }
    }

    private func refresh() {
        entries = AppLog.shared.snapshot().reversed()   // 最新在上
        shareText = deviceHeader() + "\n\n" + AppLog.shared.exportText()
        copied = false
    }

    /// 设备/版本头——便于「提交日志」时定位环境（均为非敏感信息）。
    private func deviceHeader() -> String {
        let info = Bundle.main.infoDictionary
        let v = info?["CFBundleShortVersionString"] as? String ?? "?"
        let b = info?["CFBundleVersion"] as? String ?? "?"
        return "Hum \(v) (\(b)) · iOS \(UIDevice.current.systemVersion) · \(UIDevice.current.model)"
    }

    private func color(for level: String) -> Color {
        switch level {
        case "ERROR": return .red
        case "INFO": return .primary
        default: return .secondary
        }
    }
}
