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
    @State private var seeds = ""
    @State private var examples: [String] = []
    @State private var showConversation = false
    @State private var visibleIds: Set<String> = []

    /// 当前在放的曲（完整优先），用于「回到正在播放」。
    private var nowPlayingId: String? {
        if !music.fullCurrentId.isEmpty { return music.fullCurrentId }
        if !preview.currentId.isEmpty { return preview.currentId }
        return nil
    }

    /// 当前播放行滚出视野时浮现「正在播放」胶囊，点一下滚回当前曲；可见时自动隐藏。
    @ViewBuilder private func nowPlayingButton(_ proxy: ScrollViewProxy) -> some View {
        if let cur = nowPlayingId, !visibleIds.contains(cur) {
            Button {
                withAnimation { proxy.scrollTo(cur, anchor: .center) }
            } label: {
                Label("正在播放", systemImage: "music.note")
                    .font(.caption).fontWeight(.medium)
                    .padding(.horizontal, 12).padding(.vertical, 7)
                    .background(.regularMaterial, in: Capsule())
                    .overlay(Capsule().strokeBorder(BrandTheme.primary.opacity(0.35), lineWidth: 1))
                    .foregroundStyle(BrandTheme.primary)
                    .shadow(color: .black.opacity(0.12), radius: 5, y: 2)
            }
            .buttonStyle(.plain)
            .padding(.bottom, 10)
        }
    }

    var body: some View {
        VStack(spacing: 0) {
            topBar
            // 点列表/空白区收起键盘（simultaneous 不抢占行的点歌点击）；输入区在 bottomBar 不受影响。
            mainArea
                .simultaneousGesture(TapGesture().onEnded { hideKeyboard() })
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
            ScrollViewReader { proxy in
            List {
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
                ForEach(playlist.items) { item in
                    SongRowView(
                        item: item,
                        isSelected: playlist.selected.contains(item.id),
                        queue: playlist.orderedSongs,
                        onToggleSelect: { playlist.toggle(item.id) }
                    )
                    .id(item.song.id)
                    .onAppear { visibleIds.insert(item.song.id) }
                    .onDisappear { visibleIds.remove(item.song.id) }
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
            }
            .listStyle(.plain)
            .animation(.default, value: playlist.items)
            .scrollDismissesKeyboard(.interactively)
            .overlay(alignment: .bottom) { nowPlayingButton(proxy) }
            }
        } else {
            emptyState
        }
    }

    private var emptyState: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 10) {
                AccessCTAView()
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
        .scrollDismissesKeyboard(.interactively)
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
                    Spacer()
                    Text("已选 \(playlist.selectedCount)")
                        .font(.caption).foregroundStyle(.secondary)
                }
                CreatePlaylistButton()
            }
            if !playlist.hasResult {
                TextField("可选：种子歌手/歌曲（逗号分隔）", text: $seeds)
                    .textFieldStyle(.roundedBorder).font(.caption)
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
        let seedList = seeds
            .split(whereSeparator: { $0 == "," || $0 == "，" })
            .map { String($0).trimmed }
            .filter { !$0.isEmpty }
        input = ""
        hideKeyboard()
        Task { await reco.send(text, seeds: seedList) }
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

    /// 空状态示例：先本地兜底立即有内容，有 key 时再用 LLM 千人千面覆盖。
    private func loadExamples() {
        examples = ExampleProvider.pick(4)
        Task {
            let llmEx = await llm.fetchExamples(count: 4)
            if !llmEx.isEmpty { examples = llmEx }
        }
    }
}
