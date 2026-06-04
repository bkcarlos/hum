import Foundation

/// 自然语言解析出的结构化意图（对应后端 Intent / 前端 types.ts）。
/// 注意 `seed_artists` 是 snake_case；其余为小写同名。
struct Intent: Codable, Equatable {
    var moods: [String]
    var genres: [String]
    var instruments: [String]
    var tempo: String          // "slow" | "medium" | "fast" | ""
    var keywords: [String]
    var seedArtists: [String]

    enum CodingKeys: String, CodingKey {
        case moods, genres, instruments, tempo, keywords
        case seedArtists = "seed_artists"
    }

    init(moods: [String] = [], genres: [String] = [], instruments: [String] = [],
         tempo: String = "", keywords: [String] = [], seedArtists: [String] = []) {
        self.moods = moods
        self.genres = genres
        self.instruments = instruments
        self.tempo = tempo
        self.keywords = keywords
        self.seedArtists = seedArtists
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        moods = try c.decodeIfPresent([String].self, forKey: .moods) ?? []
        genres = try c.decodeIfPresent([String].self, forKey: .genres) ?? []
        instruments = try c.decodeIfPresent([String].self, forKey: .instruments) ?? []
        tempo = try c.decodeIfPresent(String.self, forKey: .tempo) ?? ""
        keywords = try c.decodeIfPresent([String].self, forKey: .keywords) ?? []
        seedArtists = try c.decodeIfPresent([String].self, forKey: .seedArtists) ?? []
    }

    static func empty() -> Intent { Intent() }

    var isEmpty: Bool {
        moods.isEmpty && genres.isEmpty && instruments.isEmpty
            && tempo.isEmpty && keywords.isEmpty && seedArtists.isEmpty
    }
}
