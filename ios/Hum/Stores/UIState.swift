import Foundation
import Combine

/// 轻量 UI 导航状态。目前只有「是否展示设置页」，供任意子视图触发打开设置
/// （如错误提示里的「去设置」按钮），由根视图持有 sheet。
@MainActor
final class UIState: ObservableObject {
    @Published var showSettings = false
}
