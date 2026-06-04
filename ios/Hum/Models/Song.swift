import Foundation

/// 一首 Apple Music 真实曲目（对应后端 Song / 前端 types.ts 的 Song）。
/// 全部来自 /api/suggest 已校验的候选；`id` 是 storefront 相关的 catalog id，
/// 原生 MusicKit 用它做完整播放/建歌单，`previewUrl` 用于 30s 试听。
struct Song: Codable, Identifiable, Hashable {
    let id: String
    let title: String
    let artist: String
    let album: String
    let genres: [String]
    let durationMs: Int
    let artworkUrl: String
    let previewUrl: String
    let releaseDate: String?   // "1959-08-17" 或 "1959"
    let contentRating: String? // "clean" | "explicit"
    let hasLyrics: Bool?       // false ⇒ 可能是纯音乐
    let isrc: String?
    let composer: String?

    enum CodingKeys: String, CodingKey {
        case id, title, artist, album, genres, durationMs, artworkUrl, previewUrl
        case releaseDate, contentRating, hasLyrics, isrc, composer
    }

    // 容错解码：后端少给某些字段也不致整体解码失败。
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        id = try c.decode(String.self, forKey: .id)
        title = try c.decodeIfPresent(String.self, forKey: .title) ?? ""
        artist = try c.decodeIfPresent(String.self, forKey: .artist) ?? ""
        album = try c.decodeIfPresent(String.self, forKey: .album) ?? ""
        genres = try c.decodeIfPresent([String].self, forKey: .genres) ?? []
        durationMs = try c.decodeIfPresent(Int.self, forKey: .durationMs) ?? 0
        artworkUrl = try c.decodeIfPresent(String.self, forKey: .artworkUrl) ?? ""
        previewUrl = try c.decodeIfPresent(String.self, forKey: .previewUrl) ?? ""
        releaseDate = try c.decodeIfPresent(String.self, forKey: .releaseDate)
        contentRating = try c.decodeIfPresent(String.self, forKey: .contentRating)
        hasLyrics = try c.decodeIfPresent(Bool.self, forKey: .hasLyrics)
        isrc = try c.decodeIfPresent(String.self, forKey: .isrc)
        composer = try c.decodeIfPresent(String.self, forKey: .composer)
    }
}

extension Song {
    /// 发行年份（releaseDate 前 4 位）。
    var year: String? {
        guard let d = releaseDate, d.count >= 4 else { return releaseDate }
        return String(d.prefix(4))
    }
    var isExplicit: Bool { contentRating == "explicit" }
    var isInstrumental: Bool { hasLyrics == false }   // 仅当后端明确给出 false
    var hasPreview: Bool { !previewUrl.isEmpty }
}
