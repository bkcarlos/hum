import Foundation

/// /api/rank 里每首被选中曲目的 id + 推荐理由。
struct RankedSong: Codable, Identifiable, Hashable {
    let id: String
    let reason: String
}

/// /api/rank 的输出：歌单名 + 描述 + 有序的选中曲目（对应后端 RankResult）。
struct RankResult: Codable {
    let playlistName: String
    let description: String
    let songs: [RankedSong]

    enum CodingKeys: String, CodingKey {
        case playlistName = "playlist_name"
        case description, songs
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        playlistName = try c.decodeIfPresent(String.self, forKey: .playlistName) ?? ""
        description = try c.decodeIfPresent(String.self, forKey: .description) ?? ""
        songs = try c.decodeIfPresent([RankedSong].self, forKey: .songs) ?? []
    }
}
