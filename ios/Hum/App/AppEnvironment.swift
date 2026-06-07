import Foundation
import Combine

/// 依赖注入容器：集中创建并持有所有 store/service，注入到 SwiftUI 环境。
@MainActor
final class AppEnvironment: ObservableObject {
    let api: APIClient
    let llm: LLMConfigStore
    let session: SessionStore
    let convo: ConversationStore
    let playlist: PlaylistStore
    let music: MusicAuthStore
    let preview: PreviewPlayer
    let reco: RecommendationCoordinator
    let ui = UIState()

    init() {
        let api = APIClient()
        let llm = LLMConfigStore(api: api)
        let session = SessionStore(api: api)
        let convo = ConversationStore()
        let playlist = PlaylistStore()
        let music = MusicAuthStore(music: MusicService())
        let preview = PreviewPlayer()

        self.api = api
        self.llm = llm
        self.session = session
        self.convo = convo
        self.playlist = playlist
        self.music = music
        self.preview = preview
        self.reco = RecommendationCoordinator(api: api, llm: llm, session: session, convo: convo,
                                              playlist: playlist, music: music, preview: preview)
    }
}
