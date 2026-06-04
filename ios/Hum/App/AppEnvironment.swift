import Foundation
import Combine

/// 依赖注入容器：集中创建并持有所有 store/service，注入到 SwiftUI 环境。
@MainActor
final class AppEnvironment: ObservableObject {
    let api: APIClient
    let llm: LLMConfigStore
    let convo: ConversationStore
    let playlist: PlaylistStore
    let music: MusicAuthStore
    let preview: PreviewPlayer
    let reco: RecommendationCoordinator

    init() {
        let api = APIClient()
        let llm = LLMConfigStore(api: api)
        let convo = ConversationStore()
        let playlist = PlaylistStore()
        let music = MusicAuthStore()
        let preview = PreviewPlayer()

        self.api = api
        self.llm = llm
        self.convo = convo
        self.playlist = playlist
        self.music = music
        self.preview = preview
        self.reco = RecommendationCoordinator(api: api, llm: llm, convo: convo,
                                              playlist: playlist, music: music, preview: preview)
    }
}
