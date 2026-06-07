import Foundation
import AVFoundation
import MediaPlayer
import UIKit
import Combine

/// 30s 预览播放器（AVPlayer）。一次一首，队列 = 列表顺序，播完自动续播，
/// `previewUrl` 为空的曲目不可播放且会被跳过。**不需要 MusicKit 授权**。
/// 接入锁屏/控制中心/AirPods（MPNowPlayingInfoCenter + MPRemoteCommandCenter）+ 音频中断处理。
/// 注：订阅用户走 MusicKit 完整播放（系统播放器自带锁屏控件），本类只服务非订阅的 30s 试听。
@MainActor
final class PreviewPlayer: ObservableObject {
    @Published private(set) var currentId: String = ""
    @Published private(set) var isPlaying: Bool = false

    private var queue: [Song] = []
    private var player: AVPlayer?
    private var endObserver: NSObjectProtocol?
    private var interruptionObserver: NSObjectProtocol?

    init() {
        try? AVAudioSession.sharedInstance().setCategory(.playback, mode: .default)
        setupRemoteCommands()
        observeInterruptions()
    }

    /// 同步队列为当前展示顺序（每次推荐/重排后调用）。若在播的曲已不在新队列则停止。
    func setQueue(_ songs: [Song]) {
        queue = songs
        if !currentId.isEmpty, !songs.contains(where: { $0.id == currentId }) {
            stop()
        }
    }

    /// 点某一行：正在播这首→暂停；否则从这首开始。`previewUrl` 空则忽略。
    func toggle(_ song: Song) {
        guard song.hasPreview else { return }
        if song.id == currentId {
            isPlaying ? pause() : resume()
        } else {
            start(song)
        }
    }

    func next() {
        guard let idx = queue.firstIndex(where: { $0.id == currentId }) else {
            if let first = queue.first(where: { $0.hasPreview }) { start(first) }
            return
        }
        if let nextSong = queue[(idx + 1)...].first(where: { $0.hasPreview }) {
            start(nextSong)
        } else {
            stop()
        }
    }

    func prev() {
        guard let idx = queue.firstIndex(where: { $0.id == currentId }) else { return }
        if let prevSong = queue[..<idx].last(where: { $0.hasPreview }) {
            start(prevSong)
        }
    }

    func stop() {
        player?.pause()
        player = nil
        isPlaying = false
        currentId = ""
        removeEndObserver()
        updateNowPlaying(for: nil)
    }

    // MARK: - Private playback

    private func start(_ song: Song) {
        guard song.hasPreview, let url = URL(string: song.previewUrl) else { return }
        player?.pause()
        removeEndObserver()

        let item = AVPlayerItem(url: url)
        player = AVPlayer(playerItem: item)
        try? AVAudioSession.sharedInstance().setActive(true)
        endObserver = NotificationCenter.default.addObserver(
            forName: .AVPlayerItemDidPlayToEndTime, object: item, queue: .main
        ) { [weak self] _ in
            Task { @MainActor in self?.next() }   // 播完自动下一首
        }
        currentId = song.id
        player?.play()
        isPlaying = true
        updateNowPlaying(for: song)
    }

    private func pause() {
        player?.pause()
        isPlaying = false
        updateNowPlayingPlaybackState()
    }

    private func resume() {
        guard !currentId.isEmpty else { return }
        try? AVAudioSession.sharedInstance().setActive(true)
        player?.play()
        isPlaying = true
        updateNowPlayingPlaybackState()
    }

    private func removeEndObserver() {
        if let endObserver {
            NotificationCenter.default.removeObserver(endObserver)
            self.endObserver = nil
        }
    }

    // MARK: - 锁屏 / 控制中心 / AirPods（远程控制 + 正在播放信息）

    private func setupRemoteCommands() {
        let c = MPRemoteCommandCenter.shared()
        c.playCommand.addTarget { [weak self] _ in
            Task { @MainActor in self?.resume() }; return .success
        }
        c.pauseCommand.addTarget { [weak self] _ in
            Task { @MainActor in self?.pause() }; return .success
        }
        c.togglePlayPauseCommand.addTarget { [weak self] _ in
            Task { @MainActor in
                guard let self else { return }
                self.isPlaying ? self.pause() : self.resume()
            }
            return .success
        }
        c.nextTrackCommand.addTarget { [weak self] _ in
            Task { @MainActor in self?.next() }; return .success
        }
        c.previousTrackCommand.addTarget { [weak self] _ in
            Task { @MainActor in self?.prev() }; return .success
        }
    }

    /// 设置/清空正在播放信息（锁屏与控制中心据此显示标题/歌手/封面 + 播放态）。
    private func updateNowPlaying(for song: Song?) {
        guard let song else {
            MPNowPlayingInfoCenter.default().nowPlayingInfo = nil
            return
        }
        let info: [String: Any] = [
            MPMediaItemPropertyTitle: song.title,
            MPMediaItemPropertyArtist: song.artist,
            MPMediaItemPropertyAlbumTitle: song.album,
            MPNowPlayingInfoPropertyElapsedPlaybackTime: 0,
            MPNowPlayingInfoPropertyPlaybackRate: isPlaying ? 1.0 : 0.0,
        ]
        MPNowPlayingInfoCenter.default().nowPlayingInfo = info
        loadArtwork(song.artworkUrl, for: song.id)
    }

    /// 仅更新播放速率/进度（暂停/续播时）。
    private func updateNowPlayingPlaybackState() {
        guard var info = MPNowPlayingInfoCenter.default().nowPlayingInfo else { return }
        info[MPNowPlayingInfoPropertyPlaybackRate] = isPlaying ? 1.0 : 0.0
        if let t = player?.currentTime() {
            info[MPNowPlayingInfoPropertyElapsedPlaybackTime] = CMTimeGetSeconds(t)
        }
        MPNowPlayingInfoCenter.default().nowPlayingInfo = info
    }

    /// 异步拉封面塞进锁屏（拉到时仍在放这首才设，避免错图）。
    private func loadArtwork(_ urlStr: String, for songId: String) {
        guard let url = URL(string: urlStr) else { return }
        Task { [weak self] in
            guard let (data, _) = try? await URLSession.shared.data(from: url),
                  let image = UIImage(data: data) else { return }
            guard let self, self.currentId == songId,
                  var info = MPNowPlayingInfoCenter.default().nowPlayingInfo else { return }
            info[MPMediaItemPropertyArtwork] = MPMediaItemArtwork(boundsSize: image.size) { _ in image }
            MPNowPlayingInfoCenter.default().nowPlayingInfo = info
        }
    }

    // MARK: - 音频中断（来电 / 其他 App 抢占）

    private func observeInterruptions() {
        interruptionObserver = NotificationCenter.default.addObserver(
            forName: AVAudioSession.interruptionNotification, object: nil, queue: .main
        ) { [weak self] note in
            Task { @MainActor in self?.handleInterruption(note) }
        }
    }

    private func handleInterruption(_ note: Notification) {
        guard let info = note.userInfo,
              let raw = info[AVAudioSessionInterruptionTypeKey] as? UInt,
              let type = AVAudioSession.InterruptionType(rawValue: raw) else { return }
        switch type {
        case .began:
            if isPlaying { pause() }
        case .ended:
            let optRaw = (info[AVAudioSessionInterruptionOptionKey] as? UInt) ?? 0
            if AVAudioSession.InterruptionOptions(rawValue: optRaw).contains(.shouldResume),
               !currentId.isEmpty {
                resume()
            }
        @unknown default:
            break
        }
    }
}
