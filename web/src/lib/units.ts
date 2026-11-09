// Lengths are kept in centimetres; this shows them and reads what people type.
import type { Units } from './types'

let UNITS: Units = 'metric'
export function setUnits(u: Units) { UNITS = u === 'imperial' ? 'imperial' : 'metric' }
export function units(): Units { return UNITS }

const trim = (n: number, d: number) => n.toFixed(d).replace(/\.?0+$/, '')

/** A length: "2.45 m", "85 cm", or 8′ 2″. */
export function len(cm: number, u: Units = UNITS): string {
  if (!isFinite(cm)) return '—'
  if (u === 'imperial') {
    const inch = cm / 2.54
    let ft = Math.floor(inch / 12)
    let rest = Math.round(inch - ft * 12)
    if (rest === 12) { ft++; rest = 0 }
    return ft ? `${ft}′ ${rest}″` : `${rest}″`
  }
  return Math.abs(cm) >= 100 ? `${trim(cm / 100, 2)} m` : `${Math.round(cm)} cm`
}

/** A short length for the plan: "245" in cm, or 8′2″. */
export function lenShort(cm: number, u: Units = UNITS): string {
  if (u === 'imperial') {
    const inch = cm / 2.54
    let ft = Math.floor(inch / 12)
    let rest = Math.round(inch - ft * 12)
    if (rest === 12) { ft++; rest = 0 }
    return ft ? `${ft}′${rest}″` : `${rest}″`
  }
  return cm >= 100 ? trim(cm / 100, 2) : `${Math.round(cm)}`
}

export function size(w: number, d: number, h?: number, u: Units = UNITS): string {
  const p = [w, d, ...(h ? [h] : [])].map(x => u === 'imperial' ? lenShort(x, u) : `${Math.round(x)}`)
  return u === 'imperial' ? p.join(' × ') : p.join(' × ') + ' cm'
}

/** An area: "12.4 m²" or "133 sq ft". */
export function area(cm2: number, u: Units = UNITS): string {
  if (u === 'imperial') return `${Math.round(cm2 / 929.0304).toLocaleString()} sq ft`
  const m2 = cm2 / 10000
  return `${m2 >= 100 ? Math.round(m2) : trim(m2, 1)} m²`
}

/** Reads "2.4 m", "240", "240cm", "2,4m", "7'10", "7′ 10″", "94 in". Bare numbers are cm (inches in imperial). */
export function parseLen(text: string, u: Units = UNITS): number | null {
  const t = text.trim().toLowerCase().replace(',', '.').replace(/[’′]/g, "'").replace(/[”″]/g, '"')
  if (!t) return null
  const ftIn = t.match(/^(\d+(?:\.\d+)?)\s*(?:'|ft|feet)\s*(?:(\d+(?:\.\d+)?)\s*(?:"|in|inch(?:es)?)?)?$/)
  if (ftIn) return (parseFloat(ftIn[1]) * 12 + (ftIn[2] ? parseFloat(ftIn[2]) : 0)) * 2.54
  const m = t.match(/^(-?\d+(?:\.\d+)?)\s*(mm|cm|m|in|inch|inches|"|)$/)
  if (!m) return null
  const v = parseFloat(m[1])
  switch (m[2]) {
    case 'mm': return v / 10
    case 'cm': return v
    case 'm': return v * 100
    case 'in': case 'inch': case 'inches': case '"': return v * 2.54
  }
  return u === 'imperial' ? v * 2.54 : v
}

/** The value to put in an input for a length. */
export function lenInput(cm: number, u: Units = UNITS): string {
  if (u === 'imperial') return lenShort(cm, u).replace('′', "'").replace('″', '"')
  return `${Math.round(cm * 10) / 10}`
}

export function money(minor: number | null | undefined, cur = 'EUR'): string {
  if (minor == null) return ''
  try { return new Intl.NumberFormat(undefined, { style: 'currency', currency: cur, maximumFractionDigits: minor % 100 === 0 ? 0 : 2 }).format(minor / 100) } catch { return `${cur} ${(minor / 100).toFixed(2)}` }
}

export function parseMoney(text: string): number | null {
  const t = text.trim().replace(/[^\d.,-]/g, '')
  if (!t) return null
  const norm = t.includes(',') && !t.includes('.') ? t.replace(',', '.') : t.replace(/,/g, '')
  const v = parseFloat(norm)
  return isFinite(v) ? Math.round(v * 100) : null
}
