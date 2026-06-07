import SwiftUI

/// 顶栏 Apple Music 连接开关：一个音符图标。状态靠图标本身表达（已连=品牌色+绿点 /
/// 未连=灰色描边），点一下直接切换连/断——对齐 web 的 AppleConnect。连接仅用于完整
/// 播放 + 建歌单；出推荐 / 30s 试听不需要它。断开只是 App 内切回试听模式（系统授权
/// 无法在 App 内撤销）。
struct AppleConnectView: View {
    @EnvironmentObject private var music: MusicAuthStore

    var body: some View {
        Button {
            Task {
                if music.authorized { await music.disconnect() }
                else { await music.connect() }
            }
        } label: {
            ZStack(alignment: .topTrailing) {
                if music.connecting {
                    ProgressView().controlSize(.small)
                } else {
                    Image(systemName: "music.note")
                        .font(.system(size: 17, weight: .semibold))
                        .foregroundStyle(music.authorized ? BrandTheme.primary : Color.secondary)
                    if music.authorized {
                        Circle()
                            .fill(Color.green)
                            .frame(width: 8, height: 8)
                            .overlay(Circle().stroke(Color(.systemBackground), lineWidth: 1.5))
                            .offset(x: 5, y: -3)
                    }
                }
            }
            .frame(width: 28, height: 28)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .disabled(music.connecting)
        .accessibilityLabel(music.authorized
            ? "已连接 Apple Music · \(music.storefront.uppercased())，点击断开"
            : "连接 Apple Music")
    }
}
