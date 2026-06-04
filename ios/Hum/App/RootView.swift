import SwiftUI

/// 根布局：紧凑宽度（iPhone 竖屏）用 TabView 双 tab；常规宽度（iPad / 横屏）左右双栏。
struct RootView: View {
    @Environment(\.horizontalSizeClass) private var hSize
    @EnvironmentObject private var playlist: PlaylistStore
    @State private var showSettings = false
    @State private var tab = 0

    var body: some View {
        if hSize == .regular {
            splitLayout
        } else {
            tabLayout
        }
    }

    private var tabLayout: some View {
        TabView(selection: $tab) {
            NavigationStack {
                ConversationView()
                    .navigationTitle("Hum")
                    .navigationBarTitleDisplayMode(.inline)
                    .toolbar { settingsButton }
            }
            .tabItem { Label("对话", systemImage: "bubble.left.and.bubble.right") }
            .tag(0)

            NavigationStack {
                PlaylistView()
                    .navigationTitle("歌单")
                    .navigationBarTitleDisplayMode(.inline)
                    .toolbar { settingsButton }
            }
            .tabItem { Label("歌单", systemImage: "music.note.list") }
            .badge(playlist.selectedCount)
            .tag(1)
        }
        .sheet(isPresented: $showSettings) { SettingsView() }
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
            .toolbar { settingsButton }
        }
        .sheet(isPresented: $showSettings) { SettingsView() }
    }

    @ToolbarContentBuilder
    private var settingsButton: some ToolbarContent {
        ToolbarItem(placement: .topBarLeading) { AppleConnectView() }
        ToolbarItem(placement: .topBarTrailing) {
            Button { showSettings = true } label: { Image(systemName: "gearshape") }
        }
    }
}
