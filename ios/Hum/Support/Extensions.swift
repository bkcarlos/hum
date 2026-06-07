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

extension View {
    /// 在键盘上方加一个「收起」按钮——解决多行输入框（axis:.vertical，回车=换行）
    /// 没有收起入口的问题。只在键盘弹出时显示。
    func keyboardDoneToolbar(_ title: String = "收起") -> some View {
        toolbar {
            ToolbarItemGroup(placement: .keyboard) {
                Spacer()
                Button(title) { hideKeyboard() }
            }
        }
    }
}
