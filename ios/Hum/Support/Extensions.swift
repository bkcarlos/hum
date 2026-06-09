import Foundation
import SwiftUI
import UIKit

extension String {
    /// 去首尾空白与换行。
    var trimmed: String { trimmingCharacters(in: .whitespacesAndNewlines) }
}

/// 收起当前键盘（resign first responder）。
@MainActor
func hideKeyboard() {
    UIApplication.shared.sendAction(#selector(UIResponder.resignFirstResponder), to: nil, from: nil, for: nil)
}

// 键盘收起改用手势（发送后自动收起 + 点空白/滚动收起），不再放键盘上方的 toolbar 按钮。
// 收起动作仍走上面的 hideKeyboard()。
