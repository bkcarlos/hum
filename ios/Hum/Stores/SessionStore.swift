import Foundation
import Combine

/// 接入模式 + 免费档会话（对应前端 stores/session.ts + composables/useAppleLogin.ts）。
/// 两种模式：
///   .byok — 用户自带 key（LLMConfigStore），不限量。
///   .free — Sign in with Apple → 服务端 key，按每日配额。
/// 会话令牌是凭据（同 BYOK key）：存本机 Keychain（不同步 iCloud），不进 body、不记日志。
@MainActor
final class SessionStore: ObservableObject {
    enum Mode: String { case byok, free }

    @Published var mode: Mode { didSet { defaults.set(mode.rawValue, forKey: K.mode) } }
    @Published private(set) var session: String
    @Published private(set) var email: String
    @Published private(set) var sub: String
    @Published var loggingIn: Bool = false
    @Published var loginError: String = ""

    var signedIn: Bool { !session.isEmpty }

    private let api: APIClient
    private let defaults = UserDefaults.standard
    private enum K {
        static let mode = "hum.authMode"
        static let email = "hum.session.email"
    }
    private static let sessionAccount = "session"

    init(api: APIClient) {
        self.api = api
        session = KeychainStore.load(account: SessionStore.sessionAccount)
        email = defaults.string(forKey: K.email) ?? ""
        sub = ""
        // 默认模式：已存模式优先；否则新装默认免费档，但已配置 BYOK key 的老用户保持
        // BYOK（不被推到登录墙，贯彻 web 的「不打扰已配置用户」）。
        if let raw = defaults.string(forKey: K.mode), let m = Mode(rawValue: raw) {
            mode = m
        } else {
            mode = KeychainStore.load().isEmpty ? .free : .byok
        }
    }

    /// 原生 Sign in with Apple 完成后调用：identityToken → 会话令牌 → 拉 me（best-effort
    /// 取 email/sub）→ 落地 + 切到免费档。对应 useAppleLogin.login 的后半段。
    func completeSignIn(identityToken: String) async {
        guard !loggingIn else { return }
        loggingIn = true
        loginError = ""
        AppLog.shared.info("auth", "开始 Apple 登录：换取会话…")
        do {
            let result = try await api.exchangeAppleToken(identityToken)
            var who = ""
            var mail = ""
            if let me = try? await api.getMe(session: result.session) {
                who = me.sub
                mail = me.email
            }
            setSession(result.session, sub: who, email: mail)
            mode = .free
            AppLog.shared.info("auth", "Apple 登录成功（\(mail.isEmpty ? "未取到邮箱" : "已取邮箱")）")
        } catch let e as APIError {
            loginError = e.userMessage
            AppLog.shared.error("auth", "Apple 登录失败：\(e.code)")
        } catch {
            loginError = "Apple 登录失败，请重试。"
            AppLog.shared.error("auth", "Apple 登录失败：未知错误")
        }
        loggingIn = false
    }

    /// 退出免费档登录：清会话 + 邮箱（不动 BYOK key）。
    func signOut() {
        setSession("", sub: "", email: "")
    }

    private func setSession(_ token: String, sub who: String, email mail: String) {
        session = token
        sub = who
        email = mail
        if token.isEmpty {
            KeychainStore.delete(account: SessionStore.sessionAccount)
            defaults.removeObject(forKey: K.email)
        } else {
            KeychainStore.save(token, account: SessionStore.sessionAccount)
            if mail.isEmpty {
                defaults.removeObject(forKey: K.email)
            } else {
                defaults.set(mail, forKey: K.email)
            }
        }
    }
}
