import SwiftUI

/// 单行歌曲：勾选 + 封面 + 标题/标签 + 歌手 + 排序理由 +（当前行）进度条。
/// 交互：**点整行 = 播放/暂停这首**(订阅→完整，否则 30s 试听)；左侧圆圈 / 左滑 = 勾选；
/// 当前行下方显示加载条 / 可拖动进度条。正在放的行显示「试听 / 完整」标签 + 状态图标。
struct SongRowView: View {
    let item: PlaylistItem
    let isSelected: Bool
    let queue: [Song]               // 当前列表顺序，供完整播放从本曲起播
    let onToggleSelect: () -> Void

    @EnvironmentObject private var preview: PreviewPlayer
    @EnvironmentObject private var music: MusicAuthStore

    @State private var dragging = false
    @State private var dragValue: Double = 0

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

    // 当前行的播放进度（完整 / 预览分流）。
    private var activeProgress: Double { isFullCurrent ? music.fullProgress : preview.progress }
    private var activeDuration: Double { isFullCurrent ? music.fullDuration : preview.duration }
    private var activeLoading: Bool { isFullCurrent ? music.fullLoading : preview.loading }

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
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
                .accessibilityElement(children: .combine)
                .accessibilityAddTraits(.isButton)
                .accessibilityLabel("\(item.song.title)，\(item.song.artist)")
                .accessibilityHint(canPlay ? (isPlaying ? "正在播放，点按暂停" : "点按播放") : "无法播放")
            }

            if isCurrent { progressBar }
        }
        .padding(.vertical, 4)
    }

    /// 当前行下方：缓冲时显示不确定加载条，出声后显示可拖动进度条 + mm:ss。
    private var progressBar: some View {
        HStack(spacing: 8) {
            if activeLoading && activeDuration <= 0 {
                ProgressView().progressViewStyle(.linear)
            } else {
                SeekBar(
                    progress: dragging ? dragValue : activeProgress,
                    duration: max(activeDuration, 0.01),
                    onScrub: { dragValue = $0; dragging = true },
                    onCommit: { seekTo($0); dragging = false }
                )
                Text("\(fmt(dragging ? dragValue : activeProgress)) / \(fmt(activeDuration))")
                    .font(.caption2).foregroundStyle(.secondary).monospacedDigit()
            }
        }
        .padding(.horizontal, 2)
    }

    /// 点整行：订阅 → 完整播放这首（整列从此开始）；当前完整曲再点 → 暂停/续播；否则 30s 试听。
    private func togglePlay() {
        guard canPlay else { return }
        if music.canPlayFull {
            if isFullCurrent {
                Task { await music.toggleFull() }
            } else {
                preview.stop()   // 切完整前停掉 30s 试听
                Task { await music.playFull(catalogIDs: queue.map(\.id), startAt: item.song.id) }
            }
        } else {
            preview.toggle(item.song)
        }
    }

    /// 拖动进度条 seek（完整 / 预览分流）。
    private func seekTo(_ t: Double) {
        if isFullCurrent { music.seekFull(to: t) } else { preview.seek(to: t) }
    }

    /// mm:ss。
    private func fmt(_ s: Double) -> String {
        guard s.isFinite, s >= 0 else { return "0:00" }
        let total = Int(s)
        return String(format: "%d:%02d", total / 60, total % 60)
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

/// 紧凑进度条：3px 细轨道 + 10px 小圆点，可点/拖 seek。
/// 取代 SwiftUI Slider —— 它的 thumb 偏大、又没有官方 API 调小。
private struct SeekBar: View {
    let progress: Double
    let duration: Double
    let onScrub: (Double) -> Void
    let onCommit: (Double) -> Void

    var body: some View {
        GeometryReader { geo in
            let w = max(geo.size.width, 1)
            let frac = min(max(progress / duration, 0), 1)
            ZStack(alignment: .leading) {
                Capsule().fill(Color.secondary.opacity(0.25)).frame(height: 3)
                Capsule().fill(BrandTheme.primary).frame(width: w * frac, height: 3)
                Circle().fill(BrandTheme.primary)
                    .frame(width: 10, height: 10)
                    .offset(x: min(max(w * frac - 5, 0), w - 10))
            }
            .frame(maxHeight: .infinity, alignment: .center)
            .contentShape(Rectangle())
            .gesture(
                DragGesture(minimumDistance: 0)
                    .onChanged { v in onScrub(min(max(v.location.x / w, 0), 1) * duration) }
                    .onEnded { v in onCommit(min(max(v.location.x / w, 0), 1) * duration) }
            )
        }
        .frame(height: 18)
    }
}
