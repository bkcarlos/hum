import Foundation
import Combine

/// Apple Music 授权态。
/// **M-i1 为桩**：storefront 固定 "us"、未授权（试听走 AVPlayer 预览，无需授权）。
/// M-i2 接入原生 MusicKit：`MusicAuthorization.request()` + 真实 storefront + 订阅能力检测。
@MainActor
final class MusicAuthStore: ObservableObject {
    @Published private(set) var authorized: Bool = false
    @Published private(set) var storefront: String = "us"
    @Published private(set) var connecting: Bool = false
    @Published var error: String = ""
    @Published private(set) var canPlayFull: Bool = false   // 是否可完整播放（订阅用户，M-i2）

    /// M-i1 占位。真实授权在 M-i2 用 MusicKit 实现。
    func connect() async {
        error = "完整 Apple Music 接入（授权 / 完整播放 / 建歌单）将在下一里程碑提供；当前可试听 30s 预览。"
    }

    func disconnect() async {
        authorized = false
        canPlayFull = false
    }
}
