import SwiftUI

/// 建私有歌单按钮（含状态/反馈），iPhone 单屏与 iPad 双栏共用。
struct CreatePlaylistButton: View {
    @EnvironmentObject private var playlist: PlaylistStore
    @EnvironmentObject private var music: MusicAuthStore

    @State private var creating = false
    @State private var msg = ""
    @State private var url: URL?

    var body: some View {
        VStack(spacing: 4) {
            if !msg.isEmpty {
                Text(msg).font(.caption).foregroundStyle(.secondary).multilineTextAlignment(.center)
            }
            if let url {
                Link("在 Apple Music 中打开 ↗", destination: url).font(.caption)
            }
            Button {
                Task { await create() }
            } label: {
                HStack {
                    if creating { ProgressView() }
                    Text("建成歌单（\(playlist.selectedCount)）")
                }
                .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)
            .tint(BrandTheme.primary)
            .disabled(playlist.selectedCount == 0 || creating)
        }
    }

    private func create() async {
        creating = true; msg = ""; url = nil
        if !music.authorized { await music.connect() }
        guard music.authorized else {
            msg = music.error.isEmpty ? "请先连接 Apple Music 再建歌单。" : music.error
            creating = false
            return
        }
        // 候选池区 ≠ 账户区：catalog id 跨区可能对不上 → 提示重搜（对齐 web）。
        if !playlist.builtStorefront.isEmpty, playlist.builtStorefront != music.storefront {
            msg = "当前结果基于 \(playlist.builtStorefront.uppercased()) 区，你的 Apple Music 是 \(music.storefront.uppercased()) 区。请用编辑后的条件重搜后再建歌单，以匹配你的曲库。"
            creating = false
            return
        }
        let ids = playlist.selectedSongs.map { $0.id }
        let name = playlist.playlistName.isEmpty ? "我的 AI 歌单" : playlist.playlistName
        let (u, err) = await music.createPlaylist(
            name: name, description: playlist.playlistDescription, catalogIDs: ids
        )
        if let err { msg = err } else { url = u; msg = "已创建「\(name)」（私有）。" }
        creating = false
    }
}
