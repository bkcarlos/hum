import SwiftUI

/// 左侧对话（iPad 双栏用）：空状态（hero + 示例）/ 聊天流 / Intent 芯片 / 错误 / 输入区。
struct ConversationView: View {
    @EnvironmentObject private var convo: ConversationStore
    @EnvironmentObject private var reco: RecommendationCoordinator
    @EnvironmentObject private var llm: LLMConfigStore
    @EnvironmentObject private var ui: UIState

    @State private var input = ""
    @State private var seeds = ""
    @State private var examples: [String] = []

    var body: some View {
        VStack(spacing: 0) {
            ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                    if convo.messages.isEmpty {
                        emptyState
                    } else {
                        ForEach(convo.messages) { ChatBubble(message: $0) }
                    }
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
                    IntentChipsView()
                }
                .padding()
            }
            .scrollDismissesKeyboard(.interactively)
            composer
        }
        .onAppear { if examples.isEmpty { loadExamples() } }
    }

    private var emptyState: some View {
        VStack(alignment: .leading, spacing: 10) {
            AccessCTAView()
            Text("用一句话，描述你想听的").font(.title2).fontWeight(.bold)
            Text("Hum 从 Apple Music 真实曲库帮你挑歌、试听、一键建成歌单。")
                .font(.subheadline).foregroundStyle(.secondary)
            HStack {
                Text("试试这样说").font(.caption).foregroundStyle(.secondary)
                Spacer()
                Button { loadExamples() } label: { Label("换一批", systemImage: "arrow.clockwise").font(.caption) }
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
    }

    private var composer: some View {
        VStack(spacing: 6) {
            TextField("可选：种子歌手/歌曲（用逗号分隔）", text: $seeds)
                .textFieldStyle(.roundedBorder).font(.caption)
            ComposerField(
                placeholder: "描述心情/场景，或追加微调（如“去掉有歌词的”）",
                text: $input,
                disabled: reco.loading,
                onSend: send
            )
        }
        .padding()
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
        Task { await reco.send(text, seeds: seedList) }
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
