// A layout drawn to fit its box (plan cards, compare, the shared page), and
// the same drawing as a standalone SVG file (download, Pocket Files).
import { useCallback, useRef, useState } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import type { Drawing } from '../lib/types'
import { drawingBounds } from '../lib/geom'
import { DARK, LIGHT, PlanSvg, type Palette } from './PlanSvg'
import { useApp } from '../lib/app-context'
import { useMediaQuery } from './ui'

export function useSize<T extends Element>(fallback: [number, number]) {
  const [size, setSize] = useState<[number, number, boolean]>([fallback[0], fallback[1], false])
  const ro = useRef<ResizeObserver | null>(null)
  const ref = useCallback((el: T | null) => {
    ro.current?.disconnect(); ro.current = null
    if (!el) return
    const measure = () => { const r = el.getBoundingClientRect(); if (r.width > 0 && r.height > 0) setSize([Math.round(r.width), Math.round(r.height), true]) }
    measure(); ro.current = new ResizeObserver(measure); ro.current.observe(el)
  }, [])
  return [ref, size[0], size[1], size[2]] as const
}

export function useDark() {
  const { themeMode } = useApp()
  const sys = useMediaQuery('(prefers-color-scheme: dark)')
  return themeMode === 'dark' || (themeMode === 'system' && sys)
}

export function usePalette(): Palette { return useDark() ? DARK : LIGHT }

export function fitView(d: Drawing, W: number, H: number, pad = 40) {
  const b = drawingBounds(d, pad)
  const k = Math.min(W / b.w, H / b.h)
  return { k, tx: (W - b.w * k) / 2 - b.x0 * k, ty: (H - b.h * k) / 2 - b.y0 * k }
}

export function PlanPreview({ d, className = '', pad = 30, highlight, units, label, mini }: { d: Drawing; className?: string; pad?: number; highlight?: Map<string, 'added' | 'moved'>; units?: 'metric' | 'imperial'; label?: string; mini?: boolean }) {
  const [ref, W, H] = useSize<HTMLDivElement>([320, 200])
  const pal = usePalette()
  const v = fitView(d, W, H, pad)
  return <div ref={ref} className={`plan-preview ${className}`} role="img" aria-label={label || 'Floor plan'}>
    <svg viewBox={`0 0 ${W} ${H}`} width={W} height={H}>
      <g transform={`translate(${v.tx} ${v.ty}) scale(${v.k})`}><PlanSvg d={d} k={v.k} pal={pal} highlight={highlight} units={units} mini={mini ?? v.k < 0.45} /></g>
    </svg>
  </div>
}

/** The layout as an SVG file, about `px` wide (light colours, white paper). */
export function planSVGString(d: Drawing, opts: { px?: number; title?: string; units?: 'metric' | 'imperial'; showDims?: boolean } = {}) {
  const b = drawingBounds(d, 80)
  const W = opts.px || 1600
  const k = W / b.w
  const H = Math.round(b.h * k) + (opts.title ? 46 : 0)
  const top = opts.title ? 46 : 0
  const body = renderToStaticMarkup(<g transform={`translate(${-b.x0 * k} ${-b.y0 * k + top}) scale(${k})`}><PlanSvg d={d} k={k} pal={LIGHT} units={opts.units} showDims={opts.showDims} /></g>)
  const title = opts.title ? `<text x="24" y="32" font-family="system-ui, sans-serif" font-size="20" font-weight="600" fill="#23262b">${esc(opts.title)}</text>` : ''
  return `<?xml version="1.0" encoding="UTF-8"?>\n<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}"><rect width="100%" height="100%" fill="#ffffff"/>${title}${body}</svg>`
}

const esc = (s: string) => s.replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]!))

/** The SVG turned into a PNG (for people who want a picture). */
export async function svgToPNG(svg: string): Promise<Blob> {
  const img = new Image()
  const url = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }))
  try {
    await new Promise<void>((res, rej) => { img.onload = () => res(); img.onerror = () => rej(new Error('could not draw the picture')); img.src = url })
    const c = document.createElement('canvas')
    c.width = img.naturalWidth || img.width; c.height = img.naturalHeight || img.height
    const ctx = c.getContext('2d')!
    ctx.drawImage(img, 0, 0)
    return await new Promise<Blob>((res, rej) => c.toBlob(b => b ? res(b) : rej(new Error('could not make the PNG')), 'image/png'))
  } finally { URL.revokeObjectURL(url) }
}

export function saveBlob(blob: Blob, name: string) {
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = name
  document.body.appendChild(a); a.click(); a.remove()
  setTimeout(() => URL.revokeObjectURL(a.href), 2000)
}
