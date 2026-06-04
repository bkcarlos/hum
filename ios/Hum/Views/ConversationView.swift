import SwiftUI

/// 左侧对话：空状态（hero + 示例）/ 聊天流 / Intent 芯片 / 错误重试 / 输入区。
struct ConversationView: View {
    @EnvironmentObject private var convo: ConversationStore
    @EnvironmentObject private var reco: RecommendationCoordinator

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
                            Button("重试") { Task { await reco.retry() } }.font(.caption)
                        }
                    }
                    IntentChipsView()
                }
                .padding()
            }
            composer
        }
        .onAppear { if examples.isEmpty { loadExamples() } }
    }

    private var emptyState: some View {
        VStack(alignment: .leading, spacing: 10) {
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
            HStack(alignment: .bottom, spacing: 8) {
                TextField("描述心情/场景，或追加微调（如“去掉有歌词的”）", text: $input, axis: .vertical)
                    .textFieldStyle(.roundedBorder)
                    .lineLimit(1...4)
                Button { send() } label: {
                    Image(systemName: "arrow.up.circle.fill").font(.title)
                }
                .disabled(input.trimmed.isEmpty || reco.loading)
                .foregroundStyle(BrandTheme.primary)
            }
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

    /// M-i1：本地挑选示例（无 LLM key 也能用）。M-i4 接 /api/examples 个性化。
    private func loadExamples() {
        examples = ExampleProvider.pick(4)
    }
}
