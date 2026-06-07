import SwiftUI

/// 单行歌曲：勾选 + 封面 + 标题/标签 + 歌手 + 排序理由。
/// 交互：**点整行 = 播放/暂停这首**(订阅→完整，否则 30s 试听)；左侧圆圈 / 左滑 = 勾选。
/// 正在放的行显示「试听 / 完整」标签 + 状态图标；没有单独的播放按钮。
struct SongRowView: View {
    let item: PlaylistItem
    let isSelected: Bool
    let queue: [Song]               // 当前列表顺序，供完整播放从本曲起播
    let onToggleSelect: () -> Void

    @EnvironmentObject private var preview: PreviewPlayer
    @EnvironmentObject private var music: MusicAuthStore

    private var isPreviewCurrent: Bool { preview.currentId == item.song.id }
    private var isFullCurrent: Bool { music.fullCurrentId == item.song.id }
    private var isCurrent: Bool { isPreviewCurrent || isFullCurrent }
    private var isPlaying: Bool {
        (isFullCurrent && music.fullIsPlaying) || (isPreviewCurrent && preview.isPlaying)
    }
    /// 当前在放这首时的模式标签：完整 / 试听；否则不显示。
    private var nowTag: String? { isFullCurrent ? "完整" : (isPreviewCurrent ? "试听" : nil) }
    /// 可播放：有 30s 预览，或订阅用户（可完整播放任意目录曲）。
    private var canPlay: Bool { item.song.hasPreview || music.canPlayFull }

    var body: some View {
        HStack(spacing: 10) {
            Button(action: onToggleSelect) {
                Image(systemName: isSelected ? "checkmark.circle.fill" : "circle")
                    .foregroundStyle(isSelected ? BrandTheme.primary : Color.secondary)
            }
            .buttonStyle(.plain)

            // 点击区 = 封面 + 文字 + 状态图标。整块可点 → 播放/暂停。
            HStack(spacing: 10) {
                artwork

                VStack(alignment: .leading, spacing: 2) {
                    HStack(spacing: 4) {
                        Text(item.song.title).font(.subheadline).fontWeight(.medium).lineLimit(1)
                        if let nowTag { tag(nowTag, color: BrandTheme.previewTag) }
                        tag(item.status == .new ? "新" : "保留",
                            color: item.status == .new ? BrandTheme.primary : BrandTheme.keptTag)
                        if item.song.isExplicit { tag("E", color: .secondary) }
                        if item.song.isInstrumental { tag("纯音乐", color: .secondary) }
                    }
                    Text(item.song.artist).font(.caption).foregroundStyle(.secondary).lineLimit(1)
                    if !item.reason.isEmpty {
                        Text(item.reason).font(.caption2).foregroundStyle(BrandTheme.primary).lineLimit(2)
                    }
                }

                Spacer(minLength: 4)

                if isCurrent {
                    Image(systemName: isPlaying ? "speaker.wave.2.fill" : "pause.fill")
                        .font(.callout)
                        .foregroundStyle(BrandTheme.primary)
                }
            }
            .contentShape(Rectangle())
            .onTapGesture { togglePlay() }
            .opacity(canPlay ? 1 : 0.45)
        }
        .padding(.vertical, 4)
    }

    /// 点整行：订阅 → 完整播放这首（整列从此开始）；当前完整曲再点 → 暂停/续播；否则 30s 试听。
    private func togglePlay() {
        guard canPlay else { return }
        if music.canPlayFull {
            if isFullCurrent {
                Task { await music.toggleFull() }
            } else {
                preview.stop()   // 切完整前停掉 30s 试听
                let idx = queue.firstIndex { $0.id == item.song.id } ?? 0
                Task { await music.playFull(catalogIDs: queue.map(\.id), startAt: idx) }
            }
        } else {
            preview.toggle(item.song)
        }
    }

    private var artwork: some View {
        AsyncImage(url: URL(string: item.song.artworkUrl)) { phase in
            if case .success(let img) = phase {
                img.resizable().scaledToFill()
            } else {
                ZStack {
                    Color(.tertiarySystemFill)
                    Image(systemName: "music.note").foregroundStyle(.secondary)
                }
            }
        }
        .frame(width: 44, height: 44)
        .clipShape(RoundedRectangle(cornerRadius: 6))
    }

    private func tag(_ text: String, color: Color) -> some View {
        Text(text)
            .font(.system(size: 9, weight: .semibold))
            .padding(.horizontal, 4).padding(.vertical, 1)
            .background(color.opacity(0.15))
            .foregroundStyle(color)
            .clipShape(RoundedRectangle(cornerRadius: 3))
    }
}
