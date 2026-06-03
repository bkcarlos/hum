// Local, no-account personalization signals for empty-state examples ("千人千面").
// Everything lives in the browser (localStorage) — no PII ever leaves the device,
// and there is no cross-device sync (that would need a backend, i.e. F8).

const TASTE_KEY = 'hum.tastes.v1'
const TASTE_CAP = 40 // rolling window of recent taste tokens

function loadTastes(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(TASTE_KEY) || '[]')
    return Array.isArray(v) ? v : []
  } catch {
    return []
  }
}

/** Record taste tokens (genres/moods/keywords) from a successful recommendation. */
export function recordTastes(tokens: string[]) {
  const cleaned = tokens.map((t) => t.trim()).filter(Boolean)
  if (!cleaned.length) return
  try {
    localStorage.setItem(TASTE_KEY, JSON.stringify([...cleaned, ...loadTastes()].slice(0, TASTE_CAP)))
  } catch {
    /* storage full/disabled — non-fatal, personalization just stays cold */
  }
}

/** The user's most frequent recent tastes, for biasing generated examples. */
export function topTastes(n: number): string[] {
  const freq = new Map<string, number>()
  for (const t of loadTastes()) freq.set(t, (freq.get(t) ?? 0) + 1)
  return [...freq.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([t]) => t)
    .slice(0, n)
}

/** A short human-readable context string: time of day, weekday/weekend, locale. */
export function nowContext(): string {
  const d = new Date()
  const h = d.getHours()
  const part = h < 6 ? '凌晨' : h < 11 ? '早晨' : h < 14 ? '中午' : h < 18 ? '下午' : h < 23 ? '晚上' : '深夜'
  const weekend = d.getDay() === 0 || d.getDay() === 6 ? '周末' : '工作日'
  return `${weekend}${part}，地区语言：${navigator.language || 'zh'}`
}
