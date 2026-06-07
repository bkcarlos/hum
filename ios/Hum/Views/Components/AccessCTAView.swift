import SwiftUI

/// 空状态顶部的「接入状态 / 引导」块（镜像 web 顶栏 CTA）。按接入模式 × 状态四态：
///   免费档未登录 → Sign in with Apple +「或用自带 Key」
///   免费档已登录 → ✓ 免费额度 · {email}（点开设置）
///   自带 Key 未配置 →「配置自带 Key」+「或用 Apple 登录免费额度」
///   自带 Key 已配置 → ✓ 自带 Key（点开设置）
struct AccessCTAView: View {
    @EnvironmentObject private var session: SessionStore
    @EnvironmentObject private var llm: LLMConfigStore
    @EnvironmentObject private var ui: UIState

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            switch session.mode {
            case .free:
                if session.signedIn {
                    statusRow(icon: "checkmark.circle.fill",
                              text: "免费额度" + (session.email.isEmpty ? "" : " · \(session.email)"))
                } else {
                    AppleSignInButton()
                    if session.loggingIn {
                        HStack(spacing: 6) {
                            ProgressView()
                            Text("登录中…").font(.caption).foregroundStyle(.secondary)
                        }
                    }
                    altLink("或用自带 Key", to: .byok)
                }
            case .byok:
                if llm.configured {
                    statusRow(icon: "checkmark.circle.fill", text: "自带 Key")
                } else {
                    Button { ui.showSettings = true } label: {
                        Text("配置自带 Key").frame(maxWidth: .infinity)
                    }
                    .buttonStyle(.borderedProminent)
                    .tint(BrandTheme.primary)
                    altLink("或用 Apple 登录免费额度", to: .free)
                }
            }
            if !session.loginError.isEmpty {
                Text(session.loginError).font(.caption).foregroundStyle(.red)
            }
        }
        .padding(12)
        .background(Color(.secondarySystemBackground))
        .clipShape(RoundedRectangle(cornerRadius: BrandTheme.cornerRadius))
    }

    private func statusRow(icon: String, text: String) -> some View {
        Button { ui.showSettings = true } label: {
            HStack(spacing: 6) {
                Image(systemName: icon).foregroundStyle(.green)
                Text(text).font(.subheadline).foregroundStyle(.primary).lineLimit(1)
                Spacer()
                Image(systemName: "chevron.right").font(.caption2).foregroundStyle(.secondary)
            }
        }
        .buttonStyle(.plain)
    }

    /// 切到另一种模式并打开设置，让用户继续完成登录/配置（对应 web 的次要链接）。
    private func altLink(_ title: String, to mode: SessionStore.Mode) -> some View {
        Button {
            session.mode = mode
            ui.showSettings = true
        } label: {
            Text(title).font(.caption).foregroundStyle(.secondary)
        }
        .buttonStyle(.plain)
    }
}
