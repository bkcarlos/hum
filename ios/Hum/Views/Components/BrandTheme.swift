import SwiftUI

/// 品牌样式 token（主色 #fa2d48 等），集中管理避免散落硬编码。
enum BrandTheme {
    static let primary = Color(red: 250.0 / 255, green: 45.0 / 255, blue: 72.0 / 255)
    static let cornerRadius: CGFloat = 12
    static let keptTag = Color.secondary
}
