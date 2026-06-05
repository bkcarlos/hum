import SwiftUI

/// 根布局：紧凑宽度（iPhone）= 单屏 CompactHomeView（结果为主 + 底部输入）；
/// 常规宽度（iPad / 横屏）= 左对话 / 右歌单双栏。设置 sheet 由 UIState 统一控制。
struct RootView: View {
    @Environment(\.horizontalSizeClass) private var hSize
    @EnvironmentObject private var ui: UIState

    var body: some View {
        if hSize == .regular {
            splitLayout
        } else {
            compactLayout
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
