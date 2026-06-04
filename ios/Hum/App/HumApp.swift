import SwiftUI

@main
struct HumApp: App {
    @StateObject private var env = AppEnvironment()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(env)
                .environmentObject(env.llm)
                .environmentObject(env.convo)
                .environmentObject(env.playlist)
                .environmentObject(env.music)
                .environmentObject(env.preview)
                .environmentObject(env.reco)
        }
    }
}
