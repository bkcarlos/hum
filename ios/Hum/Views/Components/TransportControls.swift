import SwiftUI

/// 试听控件：上一首 / 播放暂停 / 下一首（驱动 PreviewPlayer）。
struct TransportControls: View {
    @EnvironmentObject private var preview: PreviewPlayer

    var body: some View {
        HStack(spacing: 20) {
            Button { preview.prev() } label: {
                Image(systemName: "backward.fill")
            }
            Button { preview.playPauseFromTop() } label: {
                Image(systemName: preview.isPlaying ? "pause.circle.fill" : "play.circle.fill")
                    .font(.title)
            }
            Button { preview.next() } label: {
                Image(systemName: "forward.fill")
            }
        }
        .foregroundStyle(BrandTheme.primary)
        .buttonStyle(.plain)
    }
}
