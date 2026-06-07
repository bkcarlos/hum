import SwiftUI

/// 根布局：紧凑宽度（iPhone）= 单屏 CompactHomeView（结果为主 + 底部输入）；
/// 常规宽度（iPad / 横屏）= 左对话 / 右歌单双栏。设置 sheet 由 UIState 统一控制。
struct RootView: View {
    @Environment(\.horizontalSizeClass) private var hSize
    @Environment(\.scenePhase) private var scenePhase
    @EnvironmentObject private var ui: UIState
    @EnvironmentObject private var music: MusicAuthStore

    var body: some View {
        Group {
            if hSize == .regular {
                splitLayout
            } else {
                compactLayout
            }
        }
        .task { await music.refresh() }   // 启动恢复完整播放能力（订阅用户重开 App 后仍显示「完整」）
        .onChange(of: scenePhase) { phase in
            if phase == .active { Task { await music.refresh() } }   // 回前台再刷一次（订阅态可能变）
        }
    }

    private var compactLayout: some View {
        NavigationStack {
            CompactHomeView()
                .navigationTitle("Hum")
                .navigationBarTitleDisplayMode(.inline)
                .toolbar { toolbar }
        }
        .sheet(isPresented: $ui.showSettings) { SettingsView() }
    }

    private var splitLayout: some View {
        NavigationStack {
            HStack(spacing: 0) {
                ConversationView().frame(maxWidth: .infinity)
                Divider()
                PlaylistView().frame(maxWidth: .infinity)
            }
            .navigationTitle("Hum · 哼一首")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar { toolbar }
        }
        .sheet(isPresented: $ui.showSettings) { SettingsView() }
    }

    @ToolbarContentBuilder
    private var toolbar: some ToolbarContent {
        ToolbarItem(placement: .topBarLeading) { AppleConnectView() }
        ToolbarItem(placement: .topBarTrailing) {
            Button { ui.showSettings = true } label: { Image(systemName: "gearshape") }
        }
    }
}
