import SwiftUI

/// 右侧歌单（iPad 双栏用）：通知 / 歌单名 / 试听+完整播放 / 全选 / 列表 / 建歌单。
/// iPhone 走 CompactHomeView，不用本视图。
struct PlaylistView: View {
    @EnvironmentObject private var playlist: PlaylistStore
    @EnvironmentObject private var preview: PreviewPlayer

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
            if let removed = playlist.lastRemoved {
                HStack(spacing: 8) {
                    VStack(alignment: .leading, spacing: 1) {
                        Text("已删除「\(removed.item.song.title)」")
                            .font(.caption).foregroundStyle(.secondary).lineLimit(1)
                        if let sim = playlist.similarPrompt {
                            Text("还有 \(sim.ids.count) 首「\(sim.label)」同类")
                                .font(.caption2).foregroundStyle(.secondary)
                        }
                    }
                    Spacer(minLength: 4)
                    if playlist.similarPrompt != nil {
                        Button("一起删") { deleteSimilar() }.font(.caption).tint(.red)
                    }
                    Button("撤销") { undoDelete() }.font(.caption)
                }
            }
            TextField("歌单名", text: $playlist.playlistName)
                .textFieldStyle(.roundedBorder).font(.subheadline)
            HStack(spacing: 12) {
                Button(playlist.allSelected ? "取消全选" : "全选") {
                    playlist.allSelected ? playlist.clearSelection() : playlist.selectAll()
                }
                .font(.caption)
                Spacer()
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
                queue: playlist.orderedSongs,
                onToggleSelect: { playlist.toggle(item.id) }
            )
            .listRowInsets(EdgeInsets(top: 2, leading: 12, bottom: 2, trailing: 12))
            .swipeActions(edge: .leading, allowsFullSwipe: true) {
                Button { playlist.toggle(item.id) } label: {
                    Label(playlist.selected.contains(item.id) ? "取消" : "选择",
                          systemImage: playlist.selected.contains(item.id) ? "circle" : "checkmark.circle.fill")
                }
                .tint(.green)
            }
            .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                Button(role: .destructive) { delete(item) } label: {
                    Label("删除", systemImage: "trash")
                }
            }
        }
        .listStyle(.plain)
        .animation(.default, value: playlist.items)
    }

    /// 左滑删除一行：移除并刷新预览队列；撤销条随 playlist.lastRemoved 出现。
    private func delete(_ item: PlaylistItem) {
        withAnimation { playlist.removeItem(item.id) }
        preview.setQueue(playlist.orderedSongs)
    }
    private func undoDelete() {
        withAnimation { playlist.undoRemove() }
        preview.setQueue(playlist.orderedSongs)
    }
    /// 一起删除"同类"剩余歌曲（同风格/同歌手）。
    private func deleteSimilar() {
        withAnimation { playlist.removeSimilar() }
        preview.setQueue(playlist.orderedSongs)
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
