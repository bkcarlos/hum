import SwiftUI
import MusicKit

/// 完整播放控件（订阅用户）：观察 ApplicationMusicPlayer 实时播放态，play/pause toggle。
/// 仅在 music.canPlayFull 时展示。预览(30s)仍走 TransportControls。
struct FullPlaybackButton: View {
    let catalogIDs: [String]
    @EnvironmentObject private var music: MusicAuthStore
    @ObservedObject private var playerState = ApplicationMusicPlayer.shared.state

    var body: some View {
        Button {
            Task { await toggle() }
        } label: {
            Image(systemName: isPlaying ? "pause.circle.fill" : "play.circle.fill")
        }
        .buttonStyle(.plain)
        .foregroundStyle(BrandTheme.primary)
        .accessibilityLabel(isPlaying ? "暂停完整播放" : "完整播放")
    }

    private var isPlaying: Bool { playerState.playbackStatus == .playing }

    private func toggle() async {
        let player = ApplicationMusicPlayer.shared
        switch player.state.playbackStatus {
        case .playing:
            player.pause()
        case .paused:
            try? await player.play()           // resume
        default:
            await music.playFull(catalogIDs: catalogIDs)  // 从头起播（复用 MusicService）
        }
    }
}
