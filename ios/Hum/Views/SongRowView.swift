import SwiftUI

/// 单行歌曲：勾选 + 封面 + 标题/标签 + 歌手 + 排序理由 + 预览播放键。
struct SongRowView: View {
    let item: PlaylistItem
    let isSelected: Bool
    let isCurrent: Bool
    let isPlaying: Bool
    let onToggleSelect: () -> Void
    let onTogglePlay: () -> Void

    var body: some View {
        HStack(spacing: 10) {
            Button(action: onToggleSelect) {
                Image(systemName: isSelected ? "checkmark.circle.fill" : "circle")
                    .foregroundStyle(isSelected ? BrandTheme.primary : Color.secondary)
            }
            .buttonStyle(.plain)

            artwork

            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 4) {
                    Text(item.song.title).font(.subheadline).fontWeight(.medium).lineLimit(1)
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

            Button(action: onTogglePlay) {
                Image(systemName: (isCurrent && isPlaying) ? "pause.circle.fill" : "play.circle.fill")
                    .font(.title2)
                    .foregroundStyle(item.song.hasPreview ? BrandTheme.primary : Color.secondary.opacity(0.4))
            }
            .buttonStyle(.plain)
            .disabled(!item.song.hasPreview)
        }
        .padding(.vertical, 4)
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
