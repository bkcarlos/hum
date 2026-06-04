import SwiftUI

/// 顶栏「连接 Apple Music」按钮 + 状态。
struct AppleConnectView: View {
    @EnvironmentObject private var music: MusicAuthStore

    var body: some View {
        Button {
            Task { await music.connect() }
        } label: {
            if music.connecting {
                ProgressView()
            } else {
                Label(music.authorized ? "已连接" : "连接 Apple Music",
                      systemImage: music.authorized ? "checkmark.circle.fill" : "music.note")
                    .font(.caption)
            }
        }
        .disabled(music.connecting)
        .tint(BrandTheme.primary)
    }
}
