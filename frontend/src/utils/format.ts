// Shared formatting helpers (previously copy-pasted into three components).

const SYMBOLS: Record<string, string> = { usd: '$', eur: '€', cny: '¥', btc: '₿' }

export function currencySymbol(currency: string | undefined): string {
  const c = (currency || 'usd').toLowerCase()
  return SYMBOLS[c] ?? c.toUpperCase() + ' '
}

/** Price with sensible precision: small prices get more decimals. */
export function formatPrice(n: number | undefined | null, currency?: string): string {
  if (n === undefined || n === null || !isFinite(n) || n === 0) return '—'
  const sym = currencySymbol(currency)
  if (Math.abs(n) < 1) return sym + n.toPrecision(4)
  return sym + n.toLocaleString('en', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

/** Large amounts as $1.23T / $4.56B / $7.89M. */
export function formatLarge(n: number | undefined | null, currency?: string): string {
  const sym = currencySymbol(currency)
  if (!n) return sym + '0'
  if (n >= 1e12) return sym + (n / 1e12).toFixed(2) + 'T'
  if (n >= 1e9) return sym + (n / 1e9).toFixed(2) + 'B'
  if (n >= 1e6) return sym + (n / 1e6).toFixed(2) + 'M'
  return sym + n.toFixed(2)
}

export function formatPct(n: number | undefined | null, digits = 2): string {
  if (n === undefined || n === null || !isFinite(n)) return '—'
  return (n >= 0 ? '+' : '') + n.toFixed(digits) + '%'
}

export function changeColor(n: number | undefined | null): string {
  return (n ?? 0) >= 0 ? '#2ecc71' : '#e74c3c'
}

export function formatTime(t: string | undefined | null): string {
  if (!t) return '—'
  const d = new Date(t)
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return '—'
  return d.toLocaleString('en', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

export function errorText(e: unknown): string {
  if (typeof e === 'string') return e
  if (e instanceof Error) return e.message
  return String(e)
}
