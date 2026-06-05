import SwiftUI

/// iPhone 单屏：结果为主 + 底部常驻输入，不切 tab。
/// 顶部=可折叠的意图/对话；中部=歌单结果 or 空状态；底部=操作 + 输入。
struct CompactHomeView: View {
    @EnvironmentObject private var convo: ConversationStore
    @EnvironmentObject private var playlist: PlaylistStore
    @EnvironmentObject private var preview: PreviewPlayer
    @EnvironmentObject private var music: MusicAuthStore
    @EnvironmentObject private var reco: RecommendationCoordinator
    @EnvironmentObject private var llm: LLMConfigStore
    @EnvironmentObject private var ui: UIState

    @State private var input = ""
    @State private var examples: [String] = []
    @State private var showConversation = false

    var body: some View {
        VStack(spacing: 0) {
            topBar
            mainArea
            bottomBar
        }
        .onAppear { if examples.isEmpty { loadExamples() } }
    }

    // MARK: 顶部：当前意图一行 + 可展开对话 / IntentChips
    @ViewBuilder private var topBar: some View {
        if convo.hasIntent || !convo.messages.isEmpty {
            VStack(spacing: 0) {
                Button {
                    withAnimation { showConversation.toggle() }
                } label: {
                    HStack {
                        Text(intentSummary).font(.caption).foregroundStyle(.secondary).lineLimit(1)
                        Spacer()
                        Image(systemName: showConversation ? "chevron.up" : "chevron.down").font(.caption2)
                    }
                    .padding(.horizontal).padding(.vertical, 6)
                }
                .buttonStyle(.plain)

                if showConversation {
                    ScrollView {
                        VStack(alignment: .leading, spacing: 8) {
                            ForEach(convo.messages) { ChatBubble(message: $0) }
                            IntentChipsView()
                        }
                        .padding(.horizontal)
                    }
                    .frame(maxHeight: 220)
                }
                Divider()
            }
        }
    }

    private var intentSummary: String {
        let i = convo.intent
        let parts = (i.genres + i.moods + i.instruments + i.keywords).prefix(4)
        return parts.isEmpty ? "当前推荐" : parts.joined(separator: " · ")
    }

    // MARK: 主区：歌单 or 空状态
    @ViewBuilder private var mainArea: some View {
        if playlist.hasResult {
            List {
                if !playlist.notice.isEmpty {
                    HStack {
                        Text(playlist.notice).font(.caption).foregroundStyle(.orange)
                        Spacer()
                        Button { playlist.dismissNotice() } label: { Image(systemName: "xmark") }.font(.caption)
                    }
                }
                ForEach(playlist.items) { item in
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
            }
            .listStyle(.plain)
        } else {
            emptyState
        }
    }

    private var emptyState: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 10) {
                Text("用一句话，描述你想听的").font(.title2).fontWeight(.bold)
                Text("Hum 从 Apple Music 真实曲库帮你挑歌、试听、一键建成歌单。")
                    .font(.subheadline).foregroundStyle(.secondary)
                HStack {
                    Text("试试这样说").font(.caption).foregroundStyle(.secondary)
                    Spacer()
                    Button { loadExamples() } label: {
                        Label("换一批", systemImage: "arrow.clockwise").font(.caption)
                    }
                }
                .padding(.top, 4)
                ForEach(examples, id: \.self) { ex in
                    Button { input = ex } label: {
                        Text(ex).frame(maxWidth: .infinity, alignment: .leading)
                            .padding(10)
                            .background(Color(.secondarySystemBackground))
                            .clipShape(RoundedRectangle(cornerRadius: BrandTheme.cornerRadius))
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding()
        }
        .frame(maxHeight: .infinity)
    }

    // MARK: 底部：加载/错误 + 结果操作 + 输入
    private var bottomBar: some View {
        VStack(spacing: 6) {
            if reco.loading {
                HStack(spacing: 8) {
                    ProgressView()
                    Text(reco.stage).font(.caption).foregroundStyle(.secondary)
                }
            }
            if !reco.lastError.isEmpty {
                HStack {
                    Text(reco.lastError).font(.caption).foregroundStyle(.red)
                    Spacer()
                    if reco.errorAction == .openSettings {
                        Button("去设置") { ui.showSettings = true }.font(.caption)
                    } else {
                        Button("重试") { Task { await reco.retry() } }.font(.caption)
                    }
                }
            }
            if playlist.hasResult {
                HStack {
                    Button(playlist.allSelected ? "取消全选" : "全选") {
                        playlist.allSelected ? playlist.clearSelection() : playlist.selectAll()
                    }
                    .font(.caption)
                    .frame(maxWidth: .infinity, alignment: .leading)

                    HStack(spacing: 16) {
                        TransportControls()
                        if music.canPlayFull {
                            Button {
                                Task { await music.playFull(catalogIDs: playlist.orderedSongs.map { $0.id }) }
                            } label: {
                                Image(systemName: "play.fill")
                            }
                            .buttonStyle(.plain).foregroundStyle(BrandTheme.primary)
                        }
                    }

                    Text("已选 \(playlist.selectedCount)")
                        .font(.caption).foregroundStyle(.secondary)
                        .frame(maxWidth: .infinity, alignment: .trailing)
                }
                CreatePlaylistButton()
            }
            ComposerField(
                placeholder: playlist.hasResult ? "继续微调（如“去掉有歌词的”）" : "描述心情/场景…",
                text: $input,
                disabled: reco.loading,
                onSend: send
            )
        }
        .padding(.horizontal).padding(.vertical, 8)
        .background(.bar)
    }

    private func send() {
        let text = input.trimmed
        guard !text.isEmpty else { return }
        input = ""
        Task { await reco.send(text, seeds: []) }
    }

    /// 空状态示例：先本地兜底立即有内容，有 key 时再用 LLM 千人千面覆盖。
    private func loadExamples() {
        examples = ExampleProvider.pick(4)
        Task {
            let llmEx = await llm.fetchExamples(count: 4)
            if !llmEx.isEmpty { examples = llmEx }
        }
    }
}
