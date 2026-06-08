import Foundation
import MusicKit

/// 原生 MusicKit 封装：授权 / storefront / 订阅 / 完整播放 / 建歌单。
/// 红线：播放由用户主动发起；建的歌单私有；不下载或转发音频文件。
@MainActor
final class MusicService {
    var authorizationStatus: MusicAuthorization.Status { MusicAuthorization.currentStatus }

    /// 请求授权（弹系统弹窗）。
    func requestAuthorization() async -> MusicAuthorization.Status {
        await MusicAuthorization.request()
    }

    /// 用户所在 storefront 国家码（如 "us" / "cn"）。
    func currentStorefront() async throws -> String {
        try await MusicDataRequest.currentCountryCode
    }

    /// 是否可完整播放目录内容（有效 Apple Music 订阅）。
    func canPlayCatalogContent() async -> Bool {
        guard let sub = try? await MusicSubscription.current else { return false }
        return sub.canPlayCatalogContent
    }

    /// 用 catalog id 完整播放（订阅用户），从 startAtID 这首开始。
    func playFull(catalogIDs: [String], startAtID: String) async throws {
        let songs = try await catalogSongs(for: catalogIDs)
        guard !songs.isEmpty else { return }
        // 按点击曲的 catalog id 在解析结果里定位起始曲：即使有曲在本区解析不到、
        // songs 比 catalogIDs 短，也不会像用原始 index 那样错位到邻近曲。
        let start = songs.first { $0.id.rawValue == startAtID } ?? songs[0]
        let player = ApplicationMusicPlayer.shared
        player.queue = ApplicationMusicPlayer.Queue(for: songs, startingAt: start)
        try await player.play()
    }

    func pause() {
        ApplicationMusicPlayer.shared.pause()
    }

    /// 建**私有**歌单（iOS 16 MusicLibrary）。返回 (id, 可选打开 URL)。
    func createPlaylist(name: String, description: String, catalogIDs: [String]) async throws -> (id: String, url: URL?) {
        let songs = try await catalogSongs(for: catalogIDs)
        let playlist = try await MusicLibrary.shared.createPlaylist(
            name: name, description: description, items: songs
        )
        return (playlist.id.rawValue, playlist.url)
    }

    /// 按传入顺序，用 id 批量取 catalog Song。
    private func catalogSongs(for catalogIDs: [String]) async throws -> [MusicKit.Song] {
        guard !catalogIDs.isEmpty else { return [] }
        let ids = catalogIDs.map { MusicItemID($0) }
        var request = MusicCatalogResourceRequest<MusicKit.Song>(matching: \.id, memberOf: ids)
        request.limit = 100
        let items = try await request.response().items
        let order = Dictionary(catalogIDs.enumerated().map { ($1, $0) }, uniquingKeysWith: { a, _ in a })
        return items.sorted { (order[$0.id.rawValue] ?? Int.max) < (order[$1.id.rawValue] ?? Int.max) }
    }
}
