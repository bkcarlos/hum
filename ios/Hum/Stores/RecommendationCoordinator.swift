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
    private let session: SessionStore
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

    init(api: APIClient, llm: LLMConfigStore, session: SessionStore, convo: ConversationStore,
         playlist: PlaylistStore, music: MusicAuthStore, preview: PreviewPlayer) {
        self.api = api; self.llm = llm; self.session = session; self.convo = convo
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
        guard let auth = resolveAuth() else { return }
        begin("AI 选歌…")
        do {
            let s = try await api.suggest(llm.body, auth: auth,
                                          storefront: music.storefront, text: withAvoidHint(text), seedArtists: seeds)
            guard !s.candidates.isEmpty else {
                fail("AI 推荐的歌在 Apple Music 上都没匹配到，换个说法或更具体些。"); return
            }
            convo.setIntent(s.intent)
            TasteStore.record(s.intent.genres + s.intent.moods + s.intent.keywords)

            stage = "智能排序…"
            let rank = try await api.rank(llm.body, auth: auth, intent: s.intent,
                                          candidates: s.candidates.map(Candidate.init(song:)))
            playlist.setRecommendation(s.candidates, rank: rank)
            preview.setQueue(playlist.orderedSongs)
            convo.addAssistant("为你挑了 \(playlist.items.count) 首：「\(playlist.playlistName)」。试听、勾选后可一键建歌单。")
            end()
        } catch let e as APIError { fail(e.userMessage, action: e.needsSetup ? .openSettings : .retry) }
        catch { fail("出错了，请重试。") }
    }

    private func runRerank(instruction: String) async {
        guard let auth = resolveAuth() else { return }
        begin("重新挑选…")
        do {
            let rank = try await api.rank(llm.body, auth: auth, intent: convo.intent,
                                          candidates: playlist.candidatesForRank, instruction: instruction)
            playlist.applyRefinement(rank)
            preview.setQueue(playlist.orderedSongs)
            convo.addAssistant("已按「\(instruction)」重新挑选。")
            end()
        } catch let e as APIError { fail(e.userMessage, action: e.needsSetup ? .openSettings : .retry) }
        catch { fail("出错了，请重试。") }
    }

    private func runResearch() async {
        guard let auth = resolveAuth() else { return }
        begin("AI 选歌…")
        do {
            let s = try await api.suggest(llm.body, auth: auth, storefront: music.storefront,
                                          text: withAvoidHint(intentToText(convo.intent)), seedArtists: convo.intent.seedArtists)
            guard !s.candidates.isEmpty else {
                fail("按当前条件没匹配到歌曲，调整一下条件再试。"); return
            }
            stage = "智能排序…"
            let rank = try await api.rank(llm.body, auth: auth, intent: convo.intent,
                                          candidates: s.candidates.map(Candidate.init(song:)))
            playlist.applyRefinement(rank, songs: s.candidates)
            preview.setQueue(playlist.orderedSongs)
            convo.addAssistant("已按编辑后的条件重新挑选。")
            end()
        } catch let e as APIError { fail(e.userMessage, action: e.needsSetup ? .openSettings : .retry) }
        catch { fail("出错了，请重试。") }
    }

    // MARK: - Helpers

    /// 鉴权前置（对应前端 effectiveAuth + precondition）：按接入模式给出本次请求的鉴权。
    /// 免费档需已登录（→ .session）；自带 Key 需配置完成（→ .byok）。未满足则写错误 + 返回 nil。
    /// storefront 仍用 music.storefront（未授权时为推断的默认区）——出推荐 + 预览不需连 Apple Music。
    private func resolveAuth() -> LlmAuth? {
        switch session.mode {
        case .free:
            guard session.signedIn else {
                lastError = "请先用 Apple 登录以使用免费额度（在「设置」里登录），或改用自带 Key。"
                errorAction = .openSettings
                AppLog.shared.info("reco", "已拦截：免费档未登录")
                return nil
            }
            AppLog.shared.debug("reco", "鉴权：免费档会话")
            return .session(session.session)
        case .byok:
            guard llm.configured else {
                lastError = "请先在「设置」里配置并测试自带 Key，或改用免费额度登录。"
                errorAction = .openSettings
                AppLog.shared.info("reco", "已拦截：自带 Key 未配置")
                return nil
            }
            AppLog.shared.debug("reco", "鉴权：自带 Key")
            return .byok(llm.apiKey)
        }
    }

    private func begin(_ s: String) { loading = true; stage = s; lastError = "" }
    private func end() { loading = false; stage = "" }
    private func fail(_ msg: String, action: ErrorAction = .retry) {
        loading = false; stage = ""; lastError = msg; errorAction = action
        AppLog.shared.error("reco", "推荐失败：\(msg)")
    }

    /// 把本地"负向口味"（删除歌曲累积的风格/歌手）软性附到推荐请求上。
    /// 只影响 LLM 提名，候选仍逐条走 Apple 解析校验，不破黄金原则；措辞让本次明确需求优先。
    private func withAvoidHint(_ text: String) -> String {
        let avoid = TasteStore.topDislikes(5)
        guard !avoid.isEmpty else { return text }
        return text + "（口味提示：尽量避免这些风格/歌手：\(avoid.joined(separator: "、"))；若与本次要求冲突，以本次为准。）"
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
