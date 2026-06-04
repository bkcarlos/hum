import Foundation

extension String {
    /// 去首尾空白与换行。
    var trimmed: String { trimmingCharacters(in: .whitespacesAndNewlines) }
}
