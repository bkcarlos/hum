import Foundation

/// 右侧歌单列表里的一行：歌 + 排序理由 + 状态标签。
/// status: .new 本轮新出现；.kept 上一轮已在、微调后保留（对应前端 new/kept）。
struct PlaylistItem: Identifiable, Hashable {
    enum Status { case new, kept }
    let song: Song
    let reason: String
    var status: Status
    var id: String { song.id }
}
