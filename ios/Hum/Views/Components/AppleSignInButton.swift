import SwiftUI
import AuthenticationServices

/// 原生 Sign in with Apple 按钮：取 identity token → SessionStore.completeSignIn。
/// SwiftUI 的 SignInWithAppleButton 自带系统弹窗与 presentation anchor，无需自定义 delegate/anchor。
/// 只请求 email scope（用于展示；token 的 email claim 每次都带，后端据此识别）。
struct AppleSignInButton: View {
    @EnvironmentObject private var session: SessionStore
    @Environment(\.colorScheme) private var colorScheme

    var body: some View {
        SignInWithAppleButton(.signIn) { request in
            request.requestedScopes = [.email]
        } onCompletion: { result in
            switch result {
            case .success(let auth):
                guard
                    let cred = auth.credential as? ASAuthorizationAppleIDCredential,
                    let tokenData = cred.identityToken,
                    let token = String(data: tokenData, encoding: .utf8)
                else {
                    AppLog.shared.error("auth", "Apple 授权成功但未取到 identityToken")
                    Task { @MainActor in session.loginError = "未能取得 Apple 身份令牌，请重试。" }
                    return
                }
                AppLog.shared.info("auth", "已取得 Apple identityToken，换取会话中…")
                Task { await session.completeSignIn(identityToken: token) }
            case .failure(let error):
                // 用户主动取消不算错误，静默忽略。
                if let e = error as? ASAuthorizationError, e.code == .canceled {
                    AppLog.shared.debug("auth", "用户取消了 Apple 登录")
                    return
                }
                AppLog.shared.error("auth", "Apple 授权失败：code=\((error as? ASAuthorizationError)?.code.rawValue.description ?? "未知")")
                Task { @MainActor in session.loginError = "Apple 登录失败，请重试。" }
            }
        }
        .signInWithAppleButtonStyle(colorScheme == .dark ? .white : .black)
        .frame(height: 44)
    }
}
