import SwiftUI

/// F3：展示已解析的 Intent 条件芯片，可逐个删除，并用编辑后的条件重搜。
struct IntentChipsView: View {
    @EnvironmentObject private var convo: ConversationStore
    @EnvironmentObject private var reco: RecommendationCoordinator

    var body: some View {
        if convo.hasIntent {
            VStack(alignment: .leading, spacing: 6) {
                chipRow("情绪", \.moods)
                chipRow("曲风", \.genres)
                chipRow("乐器", \.instruments)
                chipRow("关键词", \.keywords)
                chipRow("歌手", \.seedArtists)
                if !convo.intent.tempo.isEmpty {
                    HStack(spacing: 4) {
                        Text("节奏").font(.caption2).foregroundStyle(.secondary).frame(width: 36, alignment: .leading)
                        chip(tempoText) { convo.intent.tempo = ""; syncHasIntent() }
                    }
                }
                Button { Task { await reco.researchFromIntent() } } label: {
                    Label("用编辑后的条件重搜", systemImage: "arrow.triangle.2.circlepath").font(.caption)
                }
                .buttonStyle(.bordered)
                .tint(BrandTheme.primary)
                .padding(.top, 2)
            }
            .padding(.vertical, 6)
        }
    }

    private var tempoText: String {
        switch convo.intent.tempo {
        case "slow": return "慢"
        case "medium": return "适中"
        case "fast": return "快"
        default: return convo.intent.tempo
        }
    }

    @ViewBuilder
    private func chipRow(_ label: String, _ keyPath: WritableKeyPath<Intent, [String]>) -> some View {
        let values = convo.intent[keyPath: keyPath]
        if !values.isEmpty {
            HStack(alignment: .center, spacing: 4) {
                Text(label).font(.caption2).foregroundStyle(.secondary).frame(width: 36, alignment: .leading)
                ScrollView(.horizontal, showsIndicators: false) {
                    HStack(spacing: 4) {
                        ForEach(Array(values.enumerated()), id: \.offset) { idx, v in
                            chip(v) {
                                convo.intent[keyPath: keyPath].remove(at: idx)
                                syncHasIntent()
                            }
                        }
                    }
                }
            }
        }
    }

    private func chip(_ text: String, onDelete: @escaping () -> Void) -> some View {
        HStack(spacing: 2) {
            Text(text).font(.caption2)
            Button(action: onDelete) { Image(systemName: "xmark.circle.fill").font(.system(size: 10)) }
                .buttonStyle(.plain)
        }
        .padding(.horizontal, 6).padding(.vertical, 3)
        .background(BrandTheme.primary.opacity(0.1))
        .foregroundStyle(BrandTheme.primary)
        .clipShape(Capsule())
    }

    private func syncHasIntent() {
        convo.hasIntent = !convo.intent.isEmpty
    }
}
