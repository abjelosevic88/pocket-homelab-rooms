// A length field: shows the length in the person's units, reads what they type.
import { useEffect, useState } from 'react'
import { lenInput, parseLen, units } from '../lib/units'

export function LenInput({ value, onChange, min = 0, max = 100000, label, className = '', placeholder, disabled }: { value: number; onChange: (cm: number) => void; min?: number; max?: number; label?: string; className?: string; placeholder?: string; disabled?: boolean }) {
  const [text, setText] = useState(lenInput(value))
  const [focus, setFocus] = useState(false)
  useEffect(() => { if (!focus) setText(lenInput(value)) }, [value, focus])
  const apply = () => {
    const v = parseLen(text)
    if (v == null || v < min || v > max) { setText(lenInput(value)); return }
    if (Math.abs(v - value) > 0.04) onChange(Math.round(v * 10) / 10)
  }
  return <span className={`len-input ${className}`}>
    <input type="text" inputMode="decimal" className="input" value={text} aria-label={label} placeholder={placeholder} disabled={disabled}
      onFocus={e => { setFocus(true); e.target.select() }} onBlur={() => { setFocus(false); apply() }}
      onChange={e => setText(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') { apply(); (e.target as HTMLInputElement).blur() } if (e.key === 'Escape') { setText(lenInput(value)); (e.target as HTMLInputElement).blur() } }} />
    <span className="len-unit">{units() === 'imperial' ? 'ft/in' : 'cm'}</span>
  </span>
}

export function NumInput({ value, onChange, min, max, step = 1, suffix, label }: { value: number; onChange: (v: number) => void; min?: number; max?: number; step?: number; suffix?: string; label?: string }) {
  const [text, setText] = useState(String(value))
  const [focus, setFocus] = useState(false)
  useEffect(() => { if (!focus) setText(String(Math.round(value * 10) / 10)) }, [value, focus])
  const apply = () => {
    let v = parseFloat(text.replace(',', '.'))
    if (!isFinite(v)) { setText(String(value)); return }
    if (min != null) v = Math.max(min, v)
    if (max != null) v = Math.min(max, v)
    if (v !== value) onChange(v)
  }
  return <span className="len-input">
    <input type="text" inputMode="decimal" className="input" value={text} aria-label={label} step={step}
      onFocus={e => { setFocus(true); e.target.select() }} onBlur={() => { setFocus(false); apply() }}
      onChange={e => setText(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') { apply(); (e.target as HTMLInputElement).blur() } }} />
    {suffix && <span className="len-unit">{suffix}</span>}
  </span>
}
