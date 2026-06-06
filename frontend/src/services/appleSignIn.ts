// Browser Sign in with Apple (Apple JS SDK). Used by the admin login and the
// main app's free-tier login. The SDK is loaded lazily (only when web Apple
// login is actually offered) and talks to Apple directly; we only ever forward
// the returned identity token to our own POST /api/auth/apple.

const SDK_URL = 'https://appleid.cdn-apple.com/appleauth/static/jsapi/appleid/1/en_US/appleid.auth.js'

let sdkPromise: Promise<void> | null = null

function loadSdk(): Promise<void> {
  if ((window as any).AppleID) return Promise.resolve()
  if (sdkPromise) return sdkPromise
  sdkPromise = new Promise<void>((resolve, reject) => {
    const s = document.createElement('script')
    s.src = SDK_URL
    s.async = true
    s.onload = () => resolve()
    s.onerror = () => {
      sdkPromise = null
      reject(new Error('无法加载 Apple 登录脚本，请检查网络后重试。'))
    }
    document.head.appendChild(s)
  })
  return sdkPromise
}

export interface AppleWebConfig {
  clientId: string
  redirectUri: string
  scope: string
}

/** True when the user dismissed the Apple popup (so callers can stay silent). */
export function isAppleCancel(e: unknown): boolean {
  const err = (e as { error?: string })?.error
  return err === 'popup_closed_by_user' || err === 'user_cancelled_authorize' || err === 'user_trigger_new_signin_flow'
}

/** Runs the Sign in with Apple popup and resolves the Apple identity token
 *  (JWT). Throws on failure; use isAppleCancel() to detect a user dismissal. */
export async function appleSignIn(cfg: AppleWebConfig): Promise<string> {
  await loadSdk()
  const AppleID = (window as any).AppleID
  if (!AppleID?.auth) throw new Error('Apple 登录不可用。')
  AppleID.auth.init({
    clientId: cfg.clientId,
    scope: cfg.scope || '',
    redirectURI: cfg.redirectUri || `${window.location.origin}/admin`,
    usePopup: true,
  })
  const res = await AppleID.auth.signIn()
  const idToken = res?.authorization?.id_token
  if (!idToken) throw new Error('Apple 未返回身份令牌。')
  return idToken as string
}
