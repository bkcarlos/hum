import Foundation
import Combine

/// 右侧歌单 + 选中态（对应前端 playlist store，含 F10 状态同步）。
@MainActor
final class PlaylistStore: ObservableObject {
    @Published private(set) var items: [PlaylistItem] = []
    @Published var selected: Set<String> = []
    @Published var playlistName: String = ""
    @Published var playlistDescription: String = ""
    @Published var notice: String = ""
    /// 左滑删除后暂存最近一条，供一键撤销；新一轮推荐或 reset 即失效。
    @Published var lastRemoved: RemovedItem?
    /// 删除后即时探测的"同类"剩余歌曲（同风格/同歌手），供"一起删"提示；与 lastRemoved 同生命周期。
    @Published var similarPrompt: SimilarPrompt?
    /// 当前候选池解析所基于的 storefront（区）。建歌单前用它与已连账户的 storefront 比对：
    /// 不一致说明 catalog id 跨区、可能对不上 → 提示重搜（对齐 web 的 builtStorefront 校验）。
    @Published private(set) var builtStorefront: String = ""

    /// 被删除的一行 + 它原来的位置/勾选态，用于精确撤销还原。
    struct RemovedItem { let item: PlaylistItem; let index: Int; let wasSelected: Bool }
    /// 同类提示：可一起删除的 id 列表 + 展示用标签（共享的风格或歌手）。
    struct SimilarPrompt { let ids: [String]; let label: String }

    private var pool: [String: Song] = [:]   // 当前候选池（全部 Song，供 F10 重排）

    var hasResult: Bool { !items.isEmpty }
    var selectedCount: Int { selected.count }
    var allSelected: Bool { !items.isEmpty && selected.count == items.count }

    /// 池里全部候选 → 发往 /rank 的 Candidate。
    var candidatesForRank: [Candidate] { pool.values.map(Candidate.init(song:)) }
    /// 选中的 Song（按当前展示顺序）。
    var selectedSongs: [Song] { items.filter { selected.contains($0.id) }.map { $0.song } }
    /// 展示中的全部 Song（按顺序，供预览队列）。
    var orderedSongs: [Song] { items.map { $0.song } }

    /// 首次推荐：建池 + 按 rank 排序，全部标 new，清空选择。
    func setRecommendation(_ songs: [Song], rank: RankResult, storefront: String = "") {
        pool = Dictionary(songs.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        builtStorefront = storefront
        playlistName = rank.playlistName
        playlistDescription = rank.description
        selected = []
        notice = ""
        lastRemoved = nil; similarPrompt = nil
        items = orderItems(rank: rank, previousIds: [])
    }

    /// 微调/重搜：可选替换池；覆盖列表；保留仍在的勾选；提示被移出的勾选。
    func applyRefinement(_ rank: RankResult, songs: [Song]? = nil, storefront: String? = nil) {
        let previousIds = Set(items.map { $0.id })
        if let songs {
            pool = Dictionary(songs.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
            if let storefront { builtStorefront = storefront }
        }
        if !rank.playlistName.isEmpty { playlistName = rank.playlistName }
        if !rank.description.isEmpty { playlistDescription = rank.description }

        let newItems = orderItems(rank: rank, previousIds: previousIds)
        let presentIds = Set(newItems.map { $0.id })

        let droppedSelected = selected.subtracting(presentIds)
        selected.formIntersection(presentIds)
        notice = droppedSelected.isEmpty ? "" : "有 \(droppedSelected.count) 首已勾选的歌曲不在本轮推荐中。"

        items = newItems
        lastRemoved = nil; similarPrompt = nil   // 新一轮列表使上一条撤销/同类提示失效
    }

    func toggle(_ id: String) {
        if selected.contains(id) { selected.remove(id) } else { selected.insert(id) }
    }
    func selectAll() { selected = Set(items.map { $0.id }) }
    func clearSelection() { selected = [] }
    func dismissNotice() { notice = "" }

    /// 左滑删除：从列表/池/选择中移除，暂存以便撤销，记录负向口味，并探测同类。
    func removeItem(_ id: String) {
        guard let index = items.firstIndex(where: { $0.id == id }) else { return }
        let item = items[index]
        let wasSelected = selected.contains(id)
        items.remove(at: index)
        selected.remove(id)
        pool[id] = nil   // 同时移出池，避免之后重排把它捞回来
        lastRemoved = RemovedItem(item: item, index: index, wasSelected: wasSelected)

        TasteStore.recordDislike(dislikeTokens(item.song))   // 负向口味（撤销时回收）
        similarPrompt = detectSimilar(to: item.song)         // 当场查同类
    }

    /// 撤销最近一次删除：还原到原位置、原勾选态、放回池，并回收负向口味。
    func undoRemove() {
        guard let last = lastRemoved else { return }
        pool[last.item.id] = last.item.song
        items.insert(last.item, at: min(last.index, items.count))
        if last.wasSelected { selected.insert(last.item.id) }
        TasteStore.unrecordDislike(dislikeTokens(last.item.song))
        lastRemoved = nil
        similarPrompt = nil
    }

    /// "一起删除同类"：批量移除探测到的同类并记负向口味。批量删除不进单条撤销。
    func removeSimilar() {
        guard let p = similarPrompt else { return }
        for id in p.ids {
            if let idx = items.firstIndex(where: { $0.id == id }) {
                TasteStore.recordDislike(dislikeTokens(items[idx].song))
                items.remove(at: idx)
            }
            selected.remove(id)
            pool[id] = nil
        }
        similarPrompt = nil
        lastRemoved = nil   // 批量删除后单条撤销已不连贯，清掉
    }

    func clearRemoved() { lastRemoved = nil; similarPrompt = nil }
    func dismissSimilar() { similarPrompt = nil }

    func reset() {
        pool = [:]; items = []; selected = []
        playlistName = ""; playlistDescription = ""; notice = ""
        lastRemoved = nil; similarPrompt = nil
        builtStorefront = ""
    }

    /// 按 rank.songs 顺序构造 items（id 必须在 pool 中，池外 id 丢弃 = 黄金原则），标 new/kept。
    private func orderItems(rank: RankResult, previousIds: Set<String>) -> [PlaylistItem] {
        var result: [PlaylistItem] = []
        for r in rank.songs {
            guard let song = pool[r.id] else { continue }
            let status: PlaylistItem.Status = previousIds.contains(r.id) ? .kept : .new
            result.append(PlaylistItem(song: song, reason: r.reason, status: status))
        }
        return result
    }

    // MARK: - 同类探测 / 负向口味 token

    /// 有意义的风格（剔除空值与过于宽泛的 "Music"，避免同类匹配命中全部）。
    private func meaningfulGenres(_ s: Song) -> Set<String> {
        Set(s.genres.filter { !$0.isEmpty && $0 != "Music" })
    }

    /// 一首歌的负向口味 token：有意义的风格 + 歌手。
    private func dislikeTokens(_ s: Song) -> [String] {
        Array(meaningfulGenres(s)) + (s.artist.isEmpty ? [] : [s.artist])
    }

    /// 在当前列表里找与给定歌曲同风格或同歌手的剩余歌曲。
    private func detectSimilar(to song: Song) -> SimilarPrompt? {
        let genres = meaningfulGenres(song)
        let artist = song.artist
        let matched = items.filter { other in
            (!artist.isEmpty && other.song.artist == artist) ||
            !meaningfulGenres(other.song).isDisjoint(with: genres)
        }
        guard !matched.isEmpty else { return nil }
        // 标签优先用被共享的具体风格，否则用歌手名。
        let sharedGenre = genres.first { g in matched.contains { meaningfulGenres($0.song).contains(g) } }
        let label = sharedGenre ?? artist
        return SimilarPrompt(ids: matched.map { $0.id }, label: label)
    }
}
