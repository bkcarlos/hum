import Foundation
import AVFoundation
import Combine

/// 30s 预览播放器（AVPlayer）。一次一首，队列 = 列表顺序，播完自动续播，
/// `previewUrl` 为空的曲目不可播放且会被跳过。**不需要 MusicKit 授权**（M-i1 即可用）。
@MainActor
final class PreviewPlayer: ObservableObject {
    @Published private(set) var currentId: String = ""
    @Published private(set) var isPlaying: Bool = false

    private var queue: [Song] = []
    private var player: AVPlayer?
    private var endObserver: NSObjectProtocol?

    init() {
        try? AVAudioSession.sharedInstance().setCategory(.playback, mode: .default)
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

    /// 顶部播放键：有当前曲→切换；否则从第一首可播放的开始。
    func playPauseFromTop() {
        if !currentId.isEmpty {
            isPlaying ? pause() : resume()
        } else if let first = queue.first(where: { $0.hasPreview }) {
            start(first)
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
    }

    // MARK: - Private

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
    }

    private func pause() {
        player?.pause()
        isPlaying = false
    }

    private func resume() {
        player?.play()
        isPlaying = true
    }

    private func removeEndObserver() {
        if let endObserver {
            NotificationCenter.default.removeObserver(endObserver)
            self.endObserver = nil
        }
    }
}
