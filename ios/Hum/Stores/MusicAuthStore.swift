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
    @Published private(set) var fullCurrentId: String = ""   // 完整播放当前曲(catalog id)，空=没在完整播放
    @Published private(set) var fullIsPlaying: Bool = false

    private let music: MusicService
    private var bag = Set<AnyCancellable>()
    /// 用户在 App 内主动「断开」(切回 30s 试听)。会话内有效、不持久——避免回前台
    /// refresh() 因系统仍授权而把状态重置回「已连」。重启 App 后回到系统授权态。
    private var userDisengaged = false

    init(music: MusicService) {
        self.music = music
        authorized = music.authorizationStatus == .authorized
        observeFullPlayer()
    }

    /// 连接：请求授权 → 取真实 storefront + 订阅能力。
    func connect() async {
        userDisengaged = false
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
        userDisengaged = true
        authorized = false
        canPlayFull = false
        // 切回试听模式：停掉正在进行的完整播放（对齐 web 断开即停完整播放）。
        music.pause()
        fullCurrentId = ""
        fullIsPlaying = false
    }

    /// 完整播放（订阅用户）。**在 await 前**乐观标记 fullCurrentId/fullIsPlaying —— 不依赖
    /// ApplicationMusicPlayer.queue.currentEntry（真机上 .item 常为 nil，导致列表不显示「完整」
    /// 标签、且再次点同一行会重播叠加双音轨）。失败再回滚。
    func playFull(catalogIDs: [String], startAt index: Int = 0) async {
        guard !catalogIDs.isEmpty else { return }
        let i = min(max(index, 0), catalogIDs.count - 1)
        fullCurrentId = catalogIDs[i]
        fullIsPlaying = true
        do {
            try await music.playFull(catalogIDs: catalogIDs, startAt: index)
        } catch {
            self.error = "完整播放失败：\(error.localizedDescription)"
            fullCurrentId = ""
            fullIsPlaying = false
        }
    }

    func pauseFull() {
        music.pause()
        fullIsPlaying = false
    }

    /// 完整播放暂停/续播（列表行点当前完整曲时用）。即时更新 fullIsPlaying，不等观察回调。
    func toggleFull() async {
        let p = ApplicationMusicPlayer.shared
        switch p.state.playbackStatus {
        case .playing:
            p.pause()
            fullIsPlaying = false
        case .paused:
            try? await p.play()
            fullIsPlaying = true
        default:
            break
        }
    }

    /// 启动/回前台：若已授权就恢复 storefront + 完整播放能力。修复「重开 App 后
    /// canPlayFull 丢失、订阅用户看不到『完整』按钮」的问题（init 只恢复了 authorized）。
    func refresh() async {
        guard !userDisengaged, music.authorizationStatus == .authorized else { return }
        authorized = true
        if let sf = try? await music.currentStorefront() { storefront = sf }
        canPlayFull = await music.canPlayCatalogContent()
    }

    // MARK: - 完整播放态镜像（ApplicationMusicPlayer → fullCurrentId/fullIsPlaying，给列表行标签/高亮用）

    private func observeFullPlayer() {
        let p = ApplicationMusicPlayer.shared
        // 两个独立订阅，避免依赖 objectWillChange 的具体发布者类型。
        p.state.objectWillChange
            .sink { [weak self] _ in Task { @MainActor in self?.syncFull() } }
            .store(in: &bag)
        p.queue.objectWillChange
            .sink { [weak self] _ in Task { @MainActor in self?.syncFull() } }
            .store(in: &bag)
    }

    private func syncFull() {
        let p = ApplicationMusicPlayer.shared
        let status = p.state.playbackStatus
        fullIsPlaying = (status == .playing)
        if status == .playing || status == .paused,
           case let .song(song)? = p.queue.currentEntry?.item {
            fullCurrentId = song.id.rawValue
        } else if status == .stopped {
            fullCurrentId = ""
        }
    }

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
