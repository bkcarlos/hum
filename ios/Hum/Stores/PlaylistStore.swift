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
    func setRecommendation(_ songs: [Song], rank: RankResult) {
        pool = Dictionary(songs.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        playlistName = rank.playlistName
        playlistDescription = rank.description
        selected = []
        notice = ""
        items = orderItems(rank: rank, previousIds: [])
    }

    /// 微调/重搜：可选替换池；覆盖列表；保留仍在的勾选；提示被移出的勾选。
    func applyRefinement(_ rank: RankResult, songs: [Song]? = nil) {
        let previousIds = Set(items.map { $0.id })
        if let songs {
            pool = Dictionary(songs.map { ($0.id, $0) }, uniquingKeysWith: { a, _ in a })
        }
        if !rank.playlistName.isEmpty { playlistName = rank.playlistName }
        if !rank.description.isEmpty { playlistDescription = rank.description }

        let newItems = orderItems(rank: rank, previousIds: previousIds)
        let presentIds = Set(newItems.map { $0.id })

        let droppedSelected = selected.subtracting(presentIds)
        selected.formIntersection(presentIds)
        notice = droppedSelected.isEmpty ? "" : "有 \(droppedSelected.count) 首已勾选的歌曲不在本轮推荐中。"

        items = newItems
    }

    func toggle(_ id: String) {
        if selected.contains(id) { selected.remove(id) } else { selected.insert(id) }
    }
    func selectAll() { selected = Set(items.map { $0.id }) }
    func clearSelection() { selected = [] }
    func dismissNotice() { notice = "" }

    func reset() {
        pool = [:]; items = []; selected = []
        playlistName = ""; playlistDescription = ""; notice = ""
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
}
