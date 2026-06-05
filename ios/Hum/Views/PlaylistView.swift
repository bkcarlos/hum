import SwiftUI

/// 右侧歌单（iPad 双栏用）：通知 / 歌单名 / 试听+完整播放 / 全选 / 列表 / 建歌单。
/// iPhone 走 CompactHomeView，不用本视图。
struct PlaylistView: View {
    @EnvironmentObject private var playlist: PlaylistStore
    @EnvironmentObject private var preview: PreviewPlayer
    @EnvironmentObject private var music: MusicAuthStore

    var body: some View {
        VStack(spacing: 0) {
            if playlist.hasResult {
                header
                Divider()
                list
                CreatePlaylistButton().padding()
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
}
