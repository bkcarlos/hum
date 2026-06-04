import Foundation

/// 发往 /api/rank 的候选（Song 的排序相关子集，对应前端 toCandidate）。
/// 携带 year/hasLyrics/contentRating，让“去掉有歌词的 / 不要露骨的”等微调作用在真实属性上。
struct Candidate: Encodable {
    let id: String
    let title: String
    let artist: String
    let album: String?
    let genres: [String]?
    let year: String?
    let hasLyrics: Bool?
    let contentRating: String?

    init(song: Song) {
        id = song.id
        title = song.title
        artist = song.artist
        album = song.album
        genres = song.genres
        year = song.year
        hasLyrics = song.hasLyrics
        contentRating = song.contentRating
    }
}
