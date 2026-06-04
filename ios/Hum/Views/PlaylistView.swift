import SwiftUI

/// 右侧歌单：通知 / 歌单名 / 试听+完整播放 / 全选 / 列表 / 建歌单。
struct PlaylistView: View {
    @EnvironmentObject private var playlist: PlaylistStore
    @EnvironmentObject private var preview: PreviewPlayer
    @EnvironmentObject private var music: MusicAuthStore

    @State private var creating = false
    @State private var createMsg = ""
    @State private var createdURL: URL?

    var body: some View {
        VStack(spacing: 0) {
            if playlist.hasResult {
                header
                Divider()
                list
                footer
            } else {
                emptyState
            }
        }
    }

    private var header: some View {
        VStack(spacing: 8) {
            if !playlist.notice.isEmpty {
                HStack {
                    Text(playlist.notice).font(.caption).foregroundStyle(.orange)
                    Spacer()
                    Button { playlist.dismissNotice() } label: { Image(systemName: "xmark") }.font(.caption)
                }
            }
            TextField("歌单名", text: $playlist.playlistName)
                .textFieldStyle(.roundedBorder).font(.subheadline)
            HStack(spacing: 12) {
                TransportControls()
                if music.canPlayFull {
                    Button {
                        Task { await music.playFull(catalogIDs: playlist.orderedSongs.map { $0.id }) }
                    } label: {
                        Label("完整播放", systemImage: "play.fill").font(.caption)
                    }
                    .buttonStyle(.plain).foregroundStyle(BrandTheme.primary)
                }
                Spacer()
                Button(playlist.allSelected ? "取消全选" : "全选") {
                    playlist.allSelected ? playlist.clearSelection() : playlist.selectAll()
                }
                .font(.caption)
                Text("已选 \(playlist.selectedCount)/\(playlist.items.count)")
                    .font(.caption).foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal).padding(.vertical, 8)
    }

    private var list: some View {
        List(playlist.items) { item in
            SongRowView(
                item: item,
                isSelected: playlist.selected.contains(item.id),
                isCurrent: preview.currentId == item.id,
                isPlaying: preview.isPlaying,
                onToggleSelect: { playlist.toggle(item.id) },
                onTogglePlay: { preview.toggle(item.song) }
            )
            .listRowInsets(EdgeInsets(top: 2, leading: 12, bottom: 2, trailing: 12))
        }
        .listStyle(.plain)
    }

    private var footer: some View {
        VStack(spacing: 6) {
            if !createMsg.isEmpty {
                Text(createMsg).font(.caption).foregroundStyle(.secondary).multilineTextAlignment(.center)
            }
            if let createdURL {
                Link("在 Apple Music 中打开 ↗", destination: createdURL).font(.caption)
            }
            Button {
                Task { await create() }
            } label: {
                HStack {
                    if creating { ProgressView() }
                    Text("建成歌单（\(playlist.selectedCount) 首）")
                }
                .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)
            .tint(BrandTheme.primary)
            .disabled(playlist.selectedCount == 0 || creating)
        }
        .padding()
    }

    private var emptyState: some View {
        VStack(spacing: 8) {
            Image(systemName: "music.note.list").font(.largeTitle).foregroundStyle(.secondary)
            Text("还没有候选歌曲").font(.headline)
            Text("在左侧描述你想听的，这里会出现可试听、可勾选的真实歌曲。")
                .font(.caption).foregroundStyle(.secondary).multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding()
    }

    /// 建**私有**歌单（MusicKit）。未授权时先连接。
    private func create() async {
        creating = true
        createMsg = ""
        createdURL = nil
        if !music.authorized { await music.connect() }
        guard music.authorized else {
            createMsg = music.error.isEmpty ? "请先连接 Apple Music 再建歌单。" : music.error
            creating = false
            return
        }
        let ids = playlist.selectedSongs.map { $0.id }
        let name = playlist.playlistName.isEmpty ? "我的 AI 歌单" : playlist.playlistName
        let (url, err) = await music.createPlaylist(
            name: name, description: playlist.playlistDescription, catalogIDs: ids
        )
        if let err {
            createMsg = err
        } else {
            createdURL = url
            createMsg = "已在你的 Apple Music 资料库创建「\(name)」（私有）。"
        }
        creating = false
    }
}
