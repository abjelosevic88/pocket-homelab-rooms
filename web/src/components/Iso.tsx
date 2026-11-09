// The layout in 3D: walls raised to the ceiling (or cut low to see in),
// doors and windows left open, furniture as simple blocks — drawn with SVG
// polygons, far things first. Drag to turn it round.
import { useMemo, useRef, useState } from 'react'
import type { Drawing, Pt } from '../lib/types'
import { add, edge, mul, wallOf, DEFAULT_WALL, rotate } from '../lib/geom'
import { floorOf, kindOf } from '../lib/catalog'
import { useSize } from './Preview'
import type { Palette } from './PlanSvg'
import { Icon } from './icons'

interface Box { x: number; y: number; w: number; d: number; rot: number; z0: number; z1: number; color: string; kind: 'wall' | 'item' }
interface Face { pts: Pt[]; fill: string; depth: number; stroke: string; op?: number }

function shade(color: string, f: number) {
  let r = 180, g = 180, b = 180
  const rgb = color.match(/^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/)
  if (rgb) { r = +rgb[1]; g = +rgb[2]; b = +rgb[3] } else {
    const m = color.replace('#', '')
    const n = parseInt(m.length === 3 ? m.split('').map(c => c + c).join('') : m, 16)
    if (isFinite(n)) { r = (n >> 16) & 255; g = (n >> 8) & 255; b = n & 255 }
  }
  return `rgb(${Math.min(255, Math.round(r * f))},${Math.min(255, Math.round(g * f))},${Math.min(255, Math.round(b * f))})`
}

/** Splits furniture into a few blocks so a sofa, a bed or a table reads as one. */
function itemBoxes(it: Drawing['items'][number]): Box[] {
  const k = kindOf(it.type)
  const h = it.h ?? k.h
  const e = it.elev || 0
  const col = it.color || k.color
  const b = (lx: number, ly: number, w: number, d: number, z0: number, z1: number, c = col): Box => {
    const p = add(rotate({ x: lx, y: ly }, it.rot || 0), { x: it.x, y: it.y })
    return { x: p.x, y: p.y, w, d, rot: it.rot || 0, z0: e + z0, z1: e + z1, color: c, kind: 'item' }
  }
  const W = it.w, D = it.d
  switch (it.type) {
    case 'sofa': case 'armchair': {
      const back = Math.min(D * 0.25, 22), arm = Math.min(W * 0.12, 18), seat = Math.min(45, h * 0.55)
      return [b(0, 0, W, D, 0, seat), b(0, -D / 2 + back / 2, W, back, seat, h), b(-W / 2 + arm / 2, back / 2, arm, D - back, seat, h * 0.72), b(W / 2 - arm / 2, back / 2, arm, D - back, seat, h * 0.72)]
    }
    case 'sofa_l': {
      const dd = Math.min(95, D * 0.55), cw = Math.min(100, W * 0.38), seat = 44
      return [b(0, -D / 2 + dd / 2, W, dd, 0, seat), b(W / 2 - cw / 2, dd / 2, cw, D - dd, 0, seat), b(0, -D / 2 + 11, W, 22, seat, h), b(W / 2 - 11, 11, 22, D - 22, seat, h)]
    }
    case 'bed_double': case 'bed_single': return [b(0, 0, W, D, 0, Math.min(h, 52)), b(0, -D / 2 + 4, W, 8, 0, Math.max(h, 95), shade(col, 0.8))]
    case 'dining_table': case 'desk': case 'coffee_table': case 'garden_table': case 'table_round': {
      const leg = 5, top = 4
      return [b(0, 0, W, D, h - top, h), ...[[-1, -1], [1, -1], [-1, 1], [1, 1]].map(([sx, sy]) => b(sx * (W / 2 - leg), sy * (D / 2 - leg), leg, leg, 0, h - top))]
    }
    case 'chair': case 'office_chair': return [b(0, 0, W, D, 0, 46), b(0, -D / 2 + 3, W, 6, 46, h)]
    case 'toilet': return [b(0, D * 0.12, W * 0.9, D * 0.75, 0, 42), b(0, -D / 2 + D * 0.13, W, D * 0.26, 0, h)]
    case 'rug': case 'stairs': return [b(0, 0, W, D, 0, Math.max(1, h))]
    case 'lamp': return [b(0, 0, 6, 6, 0, h - 30), b(0, 0, W, D, h - 30, h, '#f3dc8e')]
    case 'plant': return [b(0, 0, W * 0.55, D * 0.55, 0, h * 0.35, '#a0785a'), b(0, 0, W, D, h * 0.35, h)]
  }
  return [b(0, 0, W, D, 0, Math.max(1, h))]
}


/** The faces to draw, far first. */
export function buildFaces(d: Drawing, ceiling: number, cut: boolean, yaw: number, dark: boolean): Face[] {
  const pal = { dark }
  const pitch = 38 * Math.PI / 180
  const wallTop = cut ? Math.min(110, ceiling) : ceiling

    const boxes: Box[] = []
    // walls, cut where the doors and windows are
    const holes = new Map<string, { a: number; b: number; z0: number; z1: number }[]>()
    for (const o of d.openings) {
      if (!o.room) continue
      const key = o.room + ':' + o.edge
      const lo = o.kind === 'window' ? (o.sill ?? 90) : 0
      const hi = o.kind === 'window' ? lo + (o.height ?? 140) : (o.height ?? 205)
      holes.set(key, [...(holes.get(key) || []), { a: o.at - o.width / 2, b: o.at + o.width / 2, z0: lo, z1: hi }])
    }
    const wallBox = (p0: Pt, p1: Pt, out: Pt, t: number, z0: number, z1: number) => {
      const len = Math.hypot(p1.x - p0.x, p1.y - p0.y)
      if (len < 0.5 || z1 - z0 < 0.5) return
      const n = Math.max(1, Math.ceil(len / 60))
      const ang = Math.atan2(p1.y - p0.y, p1.x - p0.x) * 180 / Math.PI
      for (let i = 0; i < n; i++) {
        const a = add(p0, mul({ x: (p1.x - p0.x) / len, y: (p1.y - p0.y) / len }, len * (i + 0.5) / n))
        const c = add(a, mul(out, t / 2))
        boxes.push({ x: c.x, y: c.y, w: len / n + 0.2, d: t, rot: ang, z0, z1, color: pal.dark ? '#c9ccd1' : '#f1efea', kind: 'wall' })
      }
    }
    for (const r of d.rooms) {
      const t = wallOf(r)
      if (t <= 0.05) continue
      const rh = r.height || ceiling
      const top = cut ? Math.min(wallTop, rh) : rh
      for (let i = 0; i < r.points.length; i++) {
        if ((r.open || []).includes(i)) continue
        const e = edge(r, i)
        const hs = (holes.get(r.id + ':' + i) || []).sort((x, y) => x.a - y.a)
        let s = -t
        const at = (v: number) => add(e.a, mul(e.u, v))
        for (const h of hs) {
          wallBox(at(s), at(Math.max(s, h.a)), e.out, t, 0, top)
          if (h.z0 > 0) wallBox(at(h.a), at(h.b), e.out, t, 0, Math.min(h.z0, top))
          if (h.z1 < top) wallBox(at(h.a), at(h.b), e.out, t, h.z1, top)
          s = Math.max(s, h.b)
        }
        wallBox(at(s), at(e.len + t), e.out, t, 0, top)
      }
    }
    for (const w of d.walls) {
      const t = w.thick ?? DEFAULT_WALL
      const u = { x: (w.b.x - w.a.x), y: (w.b.y - w.a.y) }
      const l = Math.hypot(u.x, u.y) || 1
      const o = { x: u.y / l, y: -u.x / l }
      wallBox(add(w.a, mul(o, -t / 2)), add(w.b, mul(o, -t / 2)), o, t, 0, wallTop)
    }
    for (const it of d.items) boxes.push(...itemBoxes(it))
    const c = Math.cos(yaw * Math.PI / 180), s = Math.sin(yaw * Math.PI / 180)
    const proj = (x: number, y: number, z: number): [number, number, number] => {
      const X = x * c - y * s, Y = x * s + y * c
      return [X, Y * Math.sin(pitch) - z * Math.cos(pitch), Y * Math.cos(pitch) + z * Math.sin(pitch)]
    }
    const out: Face[] = []
    // floors
    for (const r of d.rooms) {
      if (r.points.length < 3) continue
      const pts = r.points.map(p => proj(p.x, p.y, 0))
      out.push({ pts: pts.map(p => ({ x: p[0], y: p[1] })), fill: r.color || (pal.dark ? floorOf(r.floor).dark : floorOf(r.floor).light), depth: -1e9, stroke: 'none' })
    }
    for (const bx of boxes) {
      const hw = bx.w / 2, hd = bx.d / 2
      const loc = [{ x: -hw, y: -hd }, { x: hw, y: -hd }, { x: hw, y: hd }, { x: -hw, y: hd }].map(p => add(rotate(p, bx.rot), { x: bx.x, y: bx.y }))
      const bottom = loc.map(p => proj(p.x, p.y, bx.z0)), top = loc.map(p => proj(p.x, p.y, bx.z1))
      const center = proj(bx.x, bx.y, (bx.z0 + bx.z1) / 2)
      const stroke = bx.kind === 'wall' ? (pal.dark ? '#8b9097' : '#b7b3aa') : shade(bx.color, 0.62)
      // sides facing the viewer
      for (let i = 0; i < 4; i++) {
        const a = loc[i], b2 = loc[(i + 1) % 4]
        const nx = (b2.y - a.y), ny = -(b2.x - a.x)
        const ny2 = nx * s + ny * c // rotated normal's y: towards the viewer when > 0
        // outward normal: check against the box centre
        const mx = (a.x + b2.x) / 2 - bx.x, my = (a.y + b2.y) / 2 - bx.y
        const sign = mx * nx + my * ny > 0 ? 1 : -1
        if (ny2 * sign <= 0) continue
        const lit = 0.78 + 0.18 * Math.abs((nx * c - ny * s) * sign) / (Math.hypot(nx, ny) || 1)
        const fill = shade(bx.color, lit)
        out.push({ pts: [bottom[i], bottom[(i + 1) % 4], top[(i + 1) % 4], top[i]].map(p => ({ x: p[0], y: p[1] })), fill, depth: center[2], stroke: bx.kind === 'wall' ? fill : stroke })
      }
      out.push({ pts: top.map(p => ({ x: p[0], y: p[1] })), fill: bx.kind === 'wall' ? (pal.dark ? '#5b5f66' : '#4a4e55') : shade(bx.color, 1.04), depth: center[2] + 0.01 + (bx.z1 - bx.z0) * 0.001, stroke })
    }
    out.sort((a, b) => a.depth - b.depth)
    return out
}

export function Iso({ d, ceiling, pal, cut }: { d: Drawing; ceiling: number; pal: Palette; cut: boolean }) {
  const [ref, W, H] = useSize<HTMLDivElement>([640, 420])
  const [yaw, setYaw] = useState(-30)
  const drag = useRef<{ x: number; yaw: number } | null>(null)
  const faces = useMemo(() => buildFaces(d, ceiling, cut, yaw, pal.dark), [d, yaw, pal.dark, ceiling, cut])

  // fit
  let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity
  for (const f of faces) for (const p of f.pts) { x0 = Math.min(x0, p.x); y0 = Math.min(y0, p.y); x1 = Math.max(x1, p.x); y1 = Math.max(y1, p.y) }
  if (!isFinite(x0)) { x0 = 0; y0 = 0; x1 = 100; y1 = 100 }
  const k = Math.min((W - 40) / (x1 - x0 || 1), (H - 40) / (y1 - y0 || 1))
  const tx = (W - (x1 - x0) * k) / 2 - x0 * k, ty = (H - (y1 - y0) * k) / 2 - y0 * k
  return <div ref={ref} className="iso" onPointerDown={e => { drag.current = { x: e.clientX, yaw }; (e.target as Element).setPointerCapture?.(e.pointerId) }}
    onPointerMove={e => { if (drag.current) setYaw(drag.current.yaw + (e.clientX - drag.current.x) * 0.4) }} onPointerUp={() => { drag.current = null }} onPointerCancel={() => { drag.current = null }}>
    <svg viewBox={`0 0 ${W} ${H}`} width={W} height={H} role="img" aria-label="The layout in 3D">
      <g transform={`translate(${tx} ${ty}) scale(${k})`}>
        {faces.map((f, i) => <polygon key={i} points={f.pts.map(p => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')} fill={f.fill} stroke={f.stroke} strokeWidth={0.7} vectorEffect="non-scaling-stroke" strokeLinejoin="round" />)}
      </g>
    </svg>
    <div className="iso-turn">
      <button type="button" className="btn sm" onPointerDown={e => e.stopPropagation()} onClick={() => setYaw(y => y - 45)} aria-label="Turn left"><Icon name="rotate-ccw" size={14} />Turn</button>
      <button type="button" className="btn sm" onPointerDown={e => e.stopPropagation()} onClick={() => setYaw(y => y + 45)} aria-label="Turn right">Turn<Icon name="rotate-cw" size={14} /></button>
    </div>
  </div>
}

