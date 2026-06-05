import Foundation
import Combine

/// 推荐编排（对应前端 useRecommendation）：
/// submit(full) / refine(rerank) / researchFromIntent / retry，串起 suggest→rank。
@MainActor
final class RecommendationCoordinator: ObservableObject {
    @Published private(set) var loading: Bool = false
    @Published private(set) var stage: String = ""      // "AI 选歌…" / "智能排序…" / "重新挑选…"
    @Published var lastError: String = ""
    @Published var errorAction: ErrorAction = .retry    // 决定错误提示后跟「重试」还是「去设置」

    /// 错误后建议的操作：可重试（网络/上游）还是去设置（配置类）。
    enum ErrorAction { case retry, openSettings }

    private let api: APIClient
    private let llm: LLMConfigStore
    private let convo: ConversationStore
    private let playlist: PlaylistStore
    private let music: MusicAuthStore
    private let preview: PreviewPlayer

    private enum LastAction {
        case full(text: String, seeds: [String])
        case refine(instruction: String)
        case research
    }
    private var lastAction: LastAction?

    init(api: APIClient, llm: LLMConfigStore, convo: ConversationStore,
         playlist: PlaylistStore, music: MusicAuthStore, preview: PreviewPlayer) {
        self.api = api; self.llm = llm; self.convo = convo
        self.playlist = playlist; self.music = music; self.preview = preview
    }

    /// 输入框入口：无结果→full；有结果→refine（对应前端 send 分流）。
    func send(_ text: String, seeds: [String]) async {
        let t = text.trimmed
        guard !t.isEmpty else { return }
        if playlist.hasResult { await refine(t) } else { await submit(t, seeds: seeds) }
    }

    func submit(_ text: String, seeds: [String]) async {
        lastAction = .full(text: text, seeds: seeds)
        convo.addUser(text)
        await runFull(text: text, seeds: seeds)
    }

    func refine(_ instruction: String) async {
        lastAction = .refine(instruction: instruction)
        convo.addUser(instruction)
        if playlist.hasResult { await runRerank(instruction: instruction) }
        else { await runFull(text: instruction, seeds: []) }
    }

    /// 用编辑后的 intent 重搜（F3）。retry 不写新的对话气泡。
    func researchFromIntent() async {
        lastAction = .research
        await runResearch()
    }

    func retry() async {
        switch lastAction {
        case let .full(text, seeds): await runFull(text: text, seeds: seeds)
        case let .refine(instruction):
            if playlist.hasResult { await runRerank(instruction: instruction) }
            else { await runFull(text: instruction, seeds: []) }
        case .research: await runResearch()
        case .none: break
        }
    }

    // MARK: - Pipelines

    private func runFull(text: String, seeds: [String]) async {
        guard precondition() else { return }
        begin("AI 选歌…")
        do {
            let s = try await api.suggest(llm.body, apiKey: llm.apiKey,
                                          storefront: music.storefront, text: text, seedArtists: seeds)
            guard !s.candidates.isEmpty else {
                fail("AI 推荐的歌在 Apple Music 上都没匹配到，换个说法或更具体些。"); return
            }
            convo.setIntent(s.intent)
            TasteStore.record(s.intent.genres + s.intent.moods + s.intent.keywords)

            stage = "智能排序…"
            let rank = try await api.rank(llm.body, apiKey: llm.apiKey, intent: s.intent,
                                          candidates: s.candidates.map(Candidate.init(song:)))
            playlist.setRecommendation(s.candidates, rank: rank)
            preview.setQueue(playlist.orderedSongs)
            convo.addAssistant("为你挑了 \(playlist.items.count) 首：「\(playlist.playlistName)」。试听、勾选后可一键建歌单。")
            end()
        } catch let e as APIError { fail(e.userMessage, action: e.needsSetup ? .openSettings : .retry) }
        catch { fail("出错了，请重试。") }
    }

    private func runRerank(instruction: String) async {
        guard precondition() else { return }
        begin("重新挑选…")
        do {
            let rank = try await api.rank(llm.body, apiKey: llm.apiKey, intent: convo.intent,
                                          candidates: playlist.candidatesForRank, instruction: instruction)
            playlist.applyRefinement(rank)
            preview.setQueue(playlist.orderedSongs)
            convo.addAssistant("已按「\(instruction)」重新挑选。")
            end()
        } catch let e as APIError { fail(e.userMessage, action: e.needsSetup ? .openSettings : .retry) }
        catch { fail("出错了，请重试。") }
    }

    private func runResearch() async {
        guard precondition() else { return }
        begin("AI 选歌…")
        do {
            let s = try await api.suggest(llm.body, apiKey: llm.apiKey, storefront: music.storefront,
                                          text: intentToText(convo.intent), seedArtists: convo.intent.seedArtists)
            guard !s.candidates.isEmpty else {
                fail("按当前条件没匹配到歌曲，调整一下条件再试。"); return
            }
            stage = "智能排序…"
            let rank = try await api.rank(llm.body, apiKey: llm.apiKey, intent: convo.intent,
                                          candidates: s.candidates.map(Candidate.init(song:)))
            playlist.applyRefinement(rank, songs: s.candidates)
            preview.setQueue(playlist.orderedSongs)
            convo.addAssistant("已按编辑后的条件重新挑选。")
            end()
        } catch let e as APIError { fail(e.userMessage, action: e.needsSetup ? .openSettings : .retry) }
        catch { fail("出错了，请重试。") }
    }

    // MARK: - Helpers

    /// M-i1 只要求 LLM 配好（storefront 用默认 "us"）；M-i2 起再加 Apple 授权前置。
    private func precondition() -> Bool {
        guard llm.configured else {
            lastError = "请先在「设置」里配置并测试 LLM。"
            errorAction = .openSettings
            return false
        }
        return true
    }

    private func begin(_ s: String) { loading = true; stage = s; lastError = "" }
    private func end() { loading = false; stage = "" }
    private func fail(_ msg: String, action: ErrorAction = .retry) {
        loading = false; stage = ""; lastError = msg; errorAction = action
    }

    /// 把编辑后的 intent 转回自然语言（对应前端 intentToText）。
    private func intentToText(_ i: Intent) -> String {
        var parts: [String] = []
        let all = i.moods + i.genres + i.instruments + i.keywords
        if !all.isEmpty { parts.append("请推荐符合这些条件的歌：" + all.joined(separator: "、")) }
        switch i.tempo {
        case "slow": parts.append("节奏偏慢")
        case "fast": parts.append("节奏偏快")
        case "medium": parts.append("节奏适中")
        default: break
        }
        if !i.seedArtists.isEmpty { parts.append("可参考歌手：" + i.seedArtists.joined(separator: "、")) }
        return parts.isEmpty ? "随便推荐一些好听的" : parts.joined(separator: "，")
    }
}
