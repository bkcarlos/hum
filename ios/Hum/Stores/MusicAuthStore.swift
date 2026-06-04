import Foundation
import Combine
import MusicKit

/// Apple Music 授权态 + MusicKit 操作代理（授权 / storefront / 完整播放 / 建歌单）。
@MainActor
final class MusicAuthStore: ObservableObject {
    @Published private(set) var authorized: Bool = false
    @Published var storefront: String = "us"          // 未授权时的默认；授权后取真实值
    @Published private(set) var connecting: Bool = false
    @Published var error: String = ""
    @Published private(set) var canPlayFull: Bool = false

    private let music: MusicService

    init(music: MusicService) {
        self.music = music
        authorized = music.authorizationStatus == .authorized
    }

    /// 连接：请求授权 → 取真实 storefront + 订阅能力。
    func connect() async {
        connecting = true
        error = ""
        let status = await music.requestAuthorization()
        authorized = (status == .authorized)
        if authorized {
            if let sf = try? await music.currentStorefront() { storefront = sf }
            canPlayFull = await music.canPlayCatalogContent()
        } else {
            error = "未获得 Apple Music 授权。可在系统「设置 > Hum」里允许，或继续用 30s 预览。"
        }
        connecting = false
    }

    func disconnect() async {
        authorized = false
        canPlayFull = false
    }

    /// 完整播放（订阅用户）。
    func playFull(catalogIDs: [String], startAt index: Int = 0) async {
        do {
            try await music.playFull(catalogIDs: catalogIDs, startAt: index)
        } catch {
            self.error = "完整播放失败：\(error.localizedDescription)"
        }
    }

    func pauseFull() { music.pause() }

    /// 建歌单。返回 (打开 URL, 错误文案)。
    func createPlaylist(name: String, description: String, catalogIDs: [String]) async -> (url: URL?, error: String?) {
        do {
            let r = try await music.createPlaylist(name: name, description: description, catalogIDs: catalogIDs)
            return (r.url, nil)
        } catch {
            return (nil, "建歌单失败：\(error.localizedDescription)")
        }
    }
}
