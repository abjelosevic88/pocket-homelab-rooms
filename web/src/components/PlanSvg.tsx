import type { ReactElement } from 'react'
// Draws a layout: floors, walls (outside each room's inside outline), doors
// and windows, furniture, measurements and labels. The caller places it
// (a <g> with the view's transform); k is screen pixels per centimetre so
// text and handles keep their size at any zoom.
import { Fragment } from 'react'
import type { DItem, DOpening, DRoom, Drawing, Pt } from '../lib/types'
import { add, corners, dist, edge, labelPoint, mul, openingGeom, polyArea, sub, wallOf, norm, DEFAULT_WALL } from '../lib/geom'
import { floorOf, kindOf } from '../lib/catalog'
import { area, lenShort } from '../lib/units'
import { PieceSymbol } from './Symbols'

export interface Palette { wall: string; line: string; text: string; muted: string; dim: string; accent: string; issue: string; glass: string; bg: string; dark: boolean }
export const LIGHT: Palette = { wall: '#3b3f46', line: '#545a64', text: '#23262b', muted: '#6b717c', dim: '#7b8390', accent: '#56733f', issue: '#c2410c', glass: '#8fbcd9', bg: '#ffffff', dark: false }
export const DARK: Palette = { wall: '#c4c8ce', line: '#a9aeb6', text: '#ecedef', muted: '#9ba1a9', dim: '#8d949e', accent: '#8fb36f', issue: '#f0883e', glass: '#5a8db0', bg: '#1b1c1e', dark: true }

export type Sel = { kind: 'item' | 'room' | 'opening' | 'wall' | 'dim' | 'label'; id: string }

export interface PlanProps {
  d: Drawing
  k: number
  pal: Palette
  sel?: Sel[]
  issues?: Set<string>
  highlight?: Map<string, 'added' | 'moved'>
  showDims?: boolean
  interactive?: boolean
  units?: 'metric' | 'imperial'
  underlay?: { href: string; x: number; y: number; w: number; h: number; opacity: number; rot: number } | null
  pieceIDs?: Set<number>
  mini?: boolean
}

const P = (pts: Pt[]) => pts.map(p => `${r1(p.x)},${r1(p.y)}`).join(' ')
const r1 = (v: number) => Math.round(v * 10) / 10

function itemFill(it: DItem) { return it.color || kindOf(it.type).color }

export function PlanSvg({ d, k, pal, sel = [], issues, highlight, showDims, interactive, units, underlay, mini }: PlanProps) {
  const fs = (px: number) => px / k
  const isSel = (kind: Sel['kind'], id: string) => sel.some(s => s.kind === kind && s.id === id)
  const selRoom = sel.length === 1 && sel[0].kind === 'room' ? d.rooms.find(r => r.id === sel[0].id) : undefined
  const hit = (kind: string, id: string, extra: Record<string, string | number> = {}) => interactive ? { 'data-k': kind, 'data-id': id, ...Object.fromEntries(Object.entries(extra).map(([a, b]) => ['data-' + a, b])) } : {}
  const floorFill = (r: DRoom) => r.color || (pal.dark ? floorOf(r.floor).dark : floorOf(r.floor).light)

  // walls: each closed edge, drawn outside the room
  const wallSegs: ReactElement[] = []
  for (const r of d.rooms) {
    const t = wallOf(r)
    if (t <= 0.05 || r.points.length < 2) continue
    for (let i = 0; i < r.points.length; i++) {
      if ((r.open || []).includes(i)) continue
      const e = edge(r, i)
      const o = mul(e.out, t / 2)
      const a = add(e.a, o), b = add(e.b, o)
      wallSegs.push(<line key={`${r.id}-${i}`} x1={a.x} y1={a.y} x2={b.x} y2={b.y} stroke={pal.wall} strokeWidth={t} strokeLinecap="square" />)
    }
  }
  return (
    <g className="plan" fontFamily="system-ui, -apple-system, 'Segoe UI', sans-serif">
      {underlay && <image href={underlay.href} x={underlay.x} y={underlay.y} width={underlay.w} height={underlay.h} opacity={underlay.opacity} preserveAspectRatio="xMidYMid meet"
        transform={underlay.rot ? `rotate(${underlay.rot} ${underlay.x + underlay.w / 2} ${underlay.y + underlay.h / 2})` : undefined} style={{ pointerEvents: 'none' }} />}
      {/* outside spaces and the walls under the floors */}
      <g>{wallSegs}</g>
      {d.walls.map(w => <line key={w.id} {...hit('wall', w.id)} x1={w.a.x} y1={w.a.y} x2={w.b.x} y2={w.b.y} stroke={isSel('wall', w.id) ? pal.accent : pal.wall} strokeWidth={w.thick ?? DEFAULT_WALL} strokeLinecap="square" />)}
      <g>
        {d.rooms.map(r => r.points.length > 2 && <polygon key={r.id} {...hit('room', r.id)} points={P(r.points)} fill={floorFill(r)} stroke="none" />)}
      </g>
      {/* door and window gaps in the walls */}
      <g>
        {d.openings.map(o => <OpeningGap key={o.id} d={d} o={o} pal={pal} fill={floorFill} />)}
      </g>
      <g>
        {d.openings.map(o => <OpeningMark key={o.id} d={d} o={o} pal={pal} k={k} sel={isSel('opening', o.id)} issue={!!issues?.has(o.id)} hitProps={hit('opening', o.id)} />)}
      </g>
      {/* furniture: flat things first, then by height off the floor */}
      <g>
        {[...d.items].sort((a, b) => (kindOf(a.type).flat ? 0 : 1) - (kindOf(b.type).flat ? 0 : 1) || (a.elev || 0) - (b.elev || 0)).map(it => {
          const s = isSel('item', it.id)
          const hl = highlight?.get(it.id)
          return <g key={it.id} {...hit('item', it.id)} transform={`translate(${r1(it.x)} ${r1(it.y)}) rotate(${it.rot || 0})`} opacity={(it.elev || 0) > 100 ? 0.55 : 1} style={interactive ? { cursor: it.locked ? 'default' : 'move' } : undefined}>
            <PieceSymbol type={it.type} w={it.w} d={it.d} fill={itemFill(it)} line={pal.line} detail={Math.min(it.w, it.d) * k > 14} />
            {it.buy && <rect x={-it.w / 2} y={-it.d / 2} width={it.w} height={it.d} fill="none" stroke={pal.accent} strokeWidth={1.2} strokeDasharray="5 3" vectorEffect="non-scaling-stroke" rx={2} />}
            {hl && <rect x={-it.w / 2 - fs(3)} y={-it.d / 2 - fs(3)} width={it.w + fs(6)} height={it.d + fs(6)} fill="none" stroke={hl === 'added' ? pal.accent : '#d79b2b'} strokeWidth={2} vectorEffect="non-scaling-stroke" rx={fs(3)} />}
            {issues?.has(it.id) && <circle cx={it.w / 2 - fs(2)} cy={-it.d / 2 + fs(2)} r={fs(4.5)} fill={pal.issue} stroke={pal.bg} strokeWidth={1.2} vectorEffect="non-scaling-stroke" />}
            {s && <rect x={-it.w / 2} y={-it.d / 2} width={it.w} height={it.d} fill="none" stroke={pal.accent} strokeWidth={2} vectorEffect="non-scaling-stroke" />}
            {!mini && Math.min(it.w, it.d) * k > 46 && it.name && !kindOf(it.type).flat && <text y={fs(4)} textAnchor="middle" fontSize={fs(10)} fill={pal.text} fillOpacity={0.75} transform={`rotate(${-(it.rot || 0)})`} style={{ pointerEvents: 'none' }}>{clipName(it.name, Math.max(it.w, it.d) * k / 6)}</text>}
          </g>
        })}
      </g>
      {/* room names and areas */}
      <g style={{ pointerEvents: 'none' }}>
        {d.rooms.map(r => {
          if (r.points.length < 3) return null
          const p = labelPoint(r.points)
          const a = polyArea(r.points)
          const side = Math.sqrt(a) * k
          if (mini && side < 46) return null
          const small = mini || side < 90
          const halo = { stroke: floorFill(r), strokeWidth: 3.5 / k, strokeLinejoin: 'round' as const, paintOrder: 'stroke' as const }
          return <g key={r.id}>
            <text x={p.x} y={p.y - (small ? 0 : fs(3))} textAnchor="middle" dominantBaseline={small ? 'middle' : undefined} fontSize={fs(mini ? 9.5 : small ? 10.5 : 12.5)} fontWeight={600} fill={pal.text} {...halo}>{r.name}</text>
            {!small && <text x={p.x} y={p.y + fs(12)} textAnchor="middle" fontSize={fs(11)} fill={pal.muted} {...halo}>{area(a, units)}</text>}
          </g>
        })}
      </g>
      {/* measurements and labels */}
      <g>
        {!mini && d.dims.map(m => <Dim key={m.id} a={m.a} b={m.b} off={m.off || 0} k={k} pal={pal} units={units} sel={isSel('dim', m.id)} hitProps={hit('dim', m.id)} />)}
        {!mini && d.labels.map(l => <text key={l.id} {...hit('label', l.id)} x={l.at.x} y={l.at.y} textAnchor="middle" fontSize={fs(l.size || 13)} fontStyle="italic" fill={isSel('label', l.id) ? pal.accent : pal.muted} style={interactive ? { cursor: 'move' } : undefined}>{l.text}</text>)}
      </g>
      {/* lengths along the walls */}
      {(selRoom || showDims) && !mini && <g style={{ pointerEvents: 'none' }}>
        {(selRoom ? [selRoom] : d.rooms).map(r => r.points.map((_, i) => {
          const e = edge(r, i)
          if (e.len * k < (selRoom ? 34 : 60)) return null
          const m = add(mul(add(e.a, e.b), 0.5), mul(e.inn, fs(selRoom ? 11 : 9)))
          let ang = Math.atan2(e.u.y, e.u.x) * 180 / Math.PI
          if (ang > 90 || ang < -90) ang += 180
          return <text key={r.id + i} x={m.x} y={m.y} textAnchor="middle" dominantBaseline="middle" fontSize={fs(selRoom ? 10.5 : 9.5)} fill={selRoom ? pal.accent : pal.dim} transform={`rotate(${ang} ${m.x} ${m.y})`}>{lenShort(e.len, units)}</text>
        }))}
      </g>}
    </g>
  )
}

function clipName(s: string, n: number) { n = Math.max(3, Math.floor(n)); return s.length > n ? s.slice(0, n - 1) + '…' : s }

function OpeningGap({ d, o, pal, fill }: { d: Drawing; o: DOpening; pal: Palette; fill: (r: DRoom) => string }) {
  const g = openingGeom(d, o)
  if (!g) return null
  const t = g.t
  const pts = [add(g.j0, mul(g.out, -1)), add(g.j1, mul(g.out, -1)), add(g.j1, mul(g.out, t + 1.5)), add(g.j0, mul(g.out, t + 1.5))]
  const f = g.room ? fill(g.room) : pal.bg
  return <polygon points={P(pts)} fill={o.kind === 'window' ? pal.bg : f} stroke="none" />
}

function OpeningMark({ d, o, pal, k, sel, issue, hitProps }: { d: Drawing; o: DOpening; pal: Palette; k: number; sel: boolean; issue: boolean; hitProps: object }) {
  const g = openingGeom(d, o)
  if (!g) return null
  const col = sel ? pal.accent : issue ? pal.issue : pal.line
  const t = g.t
  const st = { stroke: col, strokeWidth: sel ? 2 : 1.2, vectorEffect: 'non-scaling-stroke' as const, fill: 'none' }
  const jamb = (p: Pt) => <line x1={p.x} y1={p.y} x2={p.x + g.out.x * t} y2={p.y + g.out.y * t} stroke={pal.wall} strokeWidth={1.4} vectorEffect="non-scaling-stroke" />
  const hitBand = [g.j0, g.j1, add(g.j1, mul(g.out, t)), add(g.j0, mul(g.out, t))]
  const side = o.out ? g.out : g.inn
  const base = o.out ? t : 0
  const jb0 = add(g.j0, mul(g.out, base)), jb1 = add(g.j1, mul(g.out, base))
  const leaf = (hinge: Pt, toward: Pt, r: number, key: string) => {
    const tip = add(hinge, mul(side, r))
    const end = add(hinge, mul(toward, r))
    const sweep = cross(toward, side) > 0 ? 1 : 0
    return <g key={key}>
      <line x1={hinge.x} y1={hinge.y} x2={tip.x} y2={tip.y} {...st} strokeWidth={sel ? 2.4 : 1.8} />
      <path d={`M${tip.x},${tip.y} A${r},${r} 0 0 ${sweep ? 0 : 1} ${end.x},${end.y}`} {...st} strokeDasharray="3 3" strokeOpacity={0.8} />
    </g>
  }
  let body: ReactElement
  switch (o.kind) {
    case 'window': {
      const m0 = add(g.j0, mul(g.out, t / 2)), m1 = add(g.j1, mul(g.out, t / 2))
      body = <g>
        <polygon points={P(hitBand)} fill={pal.glass} fillOpacity={0.35} stroke={col} strokeWidth={sel ? 2 : 1} vectorEffect="non-scaling-stroke" />
        <line x1={m0.x} y1={m0.y} x2={m1.x} y2={m1.y} stroke={col} strokeWidth={1} vectorEffect="non-scaling-stroke" />
      </g>
      break
    }
    case 'sliding': case 'pocket': {
      const a0 = add(g.j0, mul(g.out, t * 0.35)), a1 = add(add(g.j0, mul(g.u, o.width * 0.55)), mul(g.out, t * 0.35))
      const b0 = add(add(g.j1, mul(g.u, -o.width * 0.55)), mul(g.out, t * 0.65)), b1 = add(g.j1, mul(g.out, t * 0.65))
      body = <g>{jamb(g.j0)}{jamb(g.j1)}<line x1={a0.x} y1={a0.y} x2={a1.x} y2={a1.y} {...st} strokeWidth={2.2} />{o.kind === 'sliding' && <line x1={b0.x} y1={b0.y} x2={b1.x} y2={b1.y} {...st} strokeWidth={2.2} />}</g>
      break
    }
    case 'opening':
      body = <g>{jamb(g.j0)}{jamb(g.j1)}</g>
      break
    case 'garage': {
      const m0 = add(g.j0, mul(g.out, t / 2)), m1 = add(g.j1, mul(g.out, t / 2))
      body = <g>{jamb(g.j0)}{jamb(g.j1)}<line x1={m0.x} y1={m0.y} x2={m1.x} y2={m1.y} {...st} strokeDasharray="8 5" /></g>
      break
    }
    case 'double':
      body = <g>{jamb(g.j0)}{jamb(g.j1)}{leaf(jb0, g.u, o.width / 2, 'a')}{leaf(jb1, mul(g.u, -1), o.width / 2, 'b')}</g>
      break
    default:
      body = <g>{jamb(g.j0)}{jamb(g.j1)}{o.hinge === 'end' ? leaf(jb1, mul(g.u, -1), o.width, 'a') : leaf(jb0, g.u, o.width, 'a')}</g>
  }
  // a fat invisible band to grab it by
  return <g {...hitProps} style={Object.keys(hitProps).length ? { cursor: 'grab' } : undefined}>
    {body}
    <polygon points={P([add(g.j0, mul(g.out, -Math.max(t, 8 / k))), add(g.j1, mul(g.out, -Math.max(t, 8 / k))), add(g.j1, mul(g.out, t + 8 / k)), add(g.j0, mul(g.out, t + 8 / k))])} fill="transparent" stroke="none" />
  </g>
}

const cross = (a: Pt, b: Pt) => a.x * b.y - a.y * b.x

export function Dim({ a, b, off, k, pal, units, sel, hitProps = {}, live }: { a: Pt; b: Pt; off: number; k: number; pal: Palette; units?: 'metric' | 'imperial'; sel?: boolean; hitProps?: object; live?: boolean }) {
  const L = dist(a, b)
  if (L < 0.5) return null
  const u = norm(sub(b, a)), n = { x: -u.y, y: u.x }
  const A = add(a, mul(n, off)), B = add(b, mul(n, off))
  const tick = 6 / k
  const col = sel || live ? pal.accent : pal.dim
  const m = mul(add(A, B), 0.5)
  let ang = Math.atan2(u.y, u.x) * 180 / Math.PI
  if (ang > 90 || ang < -90) ang += 180
  const st = { stroke: col, strokeWidth: sel ? 1.8 : 1.1, vectorEffect: 'non-scaling-stroke' as const }
  const lab = lenShort(L, units)
  const tw = (lab.length * 6.4 + 8) / k
  return <g {...hitProps} style={Object.keys(hitProps).length ? { cursor: 'pointer' } : undefined}>
    {off !== 0 && <><line x1={a.x} y1={a.y} x2={A.x} y2={A.y} {...st} strokeOpacity={0.5} /><line x1={b.x} y1={b.y} x2={B.x} y2={B.y} {...st} strokeOpacity={0.5} /></>}
    <line x1={A.x} y1={A.y} x2={B.x} y2={B.y} {...st} />
    {[A, B].map((p, i) => <line key={i} x1={p.x - (u.x + n.x) * tick / 1.6} y1={p.y - (u.y + n.y) * tick / 1.6} x2={p.x + (u.x + n.x) * tick / 1.6} y2={p.y + (u.y + n.y) * tick / 1.6} {...st} />)}
    <g transform={`rotate(${ang} ${m.x} ${m.y})`}>
      <rect x={m.x - tw / 2} y={m.y - 8 / k} width={tw} height={16 / k} rx={3 / k} fill={pal.bg} fillOpacity={0.92} />
      <text x={m.x} y={m.y} textAnchor="middle" dominantBaseline="central" fontSize={11 / k} fontWeight={600} fill={col}>{lab}</text>
    </g>
    <line x1={A.x} y1={A.y} x2={B.x} y2={B.y} stroke="transparent" strokeWidth={14} vectorEffect="non-scaling-stroke" />
  </g>
}

/** Handles for the selection: a room's corners and walls, an item's corners and turn knob. */
export function Handles({ d, sel, k, pal }: { d: Drawing; sel: Sel[]; k: number; pal: Palette }) {
  if (sel.length !== 1) return null
  const s = sel[0]
  const R = 6 / k
  if (s.kind === 'room') {
    const r = d.rooms.find(x => x.id === s.id)
    if (!r) return null
    return <g>
      {r.points.map((_, i) => {
        const e = edge(r, i)
        if (e.len * k < 40) return null
        const m = mul(add(e.a, e.b), 0.5)
        const ang = Math.atan2(e.u.y, e.u.x) * 180 / Math.PI
        return <rect key={'e' + i} data-k="edge" data-id={r.id} data-i={i} x={m.x - 11 / k} y={m.y - 4 / k} width={22 / k} height={8 / k} rx={4 / k} fill={pal.bg} stroke={pal.accent} strokeWidth={1.6} vectorEffect="non-scaling-stroke"
          transform={`rotate(${ang} ${m.x} ${m.y})`} style={{ cursor: Math.abs(e.u.x) > Math.abs(e.u.y) ? 'ns-resize' : 'ew-resize' }}><title>Drag to move this wall · double-click to add a corner</title></rect>
      })}
      {r.points.map((p, i) => <circle key={'v' + i} data-k="vertex" data-id={r.id} data-i={i} cx={p.x} cy={p.y} r={R} fill={pal.accent} stroke={pal.bg} strokeWidth={2} vectorEffect="non-scaling-stroke" style={{ cursor: 'grab' }} />)}
    </g>
  }
  if (s.kind === 'item') {
    const it = d.items.find(x => x.id === s.id)
    if (!it || it.locked) return null
    const cs = corners(it)
    const knob = add({ x: it.x, y: it.y }, rotatePt({ x: 0, y: -it.d / 2 - 22 / k }, it.rot || 0))
    const backMid = add({ x: it.x, y: it.y }, rotatePt({ x: 0, y: -it.d / 2 }, it.rot || 0))
    return <g>
      <line x1={backMid.x} y1={backMid.y} x2={knob.x} y2={knob.y} stroke={pal.accent} strokeWidth={1.4} vectorEffect="non-scaling-stroke" />
      <circle data-k="rot" data-id={it.id} cx={knob.x} cy={knob.y} r={R * 1.1} fill={pal.bg} stroke={pal.accent} strokeWidth={2} vectorEffect="non-scaling-stroke" style={{ cursor: 'grab' }}><title>Drag to turn (hold Shift for any angle)</title></circle>
      {cs.map((p, i) => <rect key={i} data-k="resize" data-id={it.id} data-i={i} x={p.x - R * 0.8} y={p.y - R * 0.8} width={R * 1.6} height={R * 1.6} fill={pal.bg} stroke={pal.accent} strokeWidth={1.8} vectorEffect="non-scaling-stroke" style={{ cursor: 'nwse-resize' }} />)}
    </g>
  }
  if (s.kind === 'dim') {
    const m = d.dims.find(x => x.id === s.id)
    if (!m) return null
    return <g>{[m.a, m.b].map((p, i) => <circle key={i} data-k="dimend" data-id={m.id} data-i={i} cx={p.x} cy={p.y} r={R} fill={pal.accent} stroke={pal.bg} strokeWidth={2} vectorEffect="non-scaling-stroke" style={{ cursor: 'grab' }} />)}</g>
  }
  if (s.kind === 'wall') {
    const w = d.walls.find(x => x.id === s.id)
    if (!w) return null
    return <g>{[w.a, w.b].map((p, i) => <circle key={i} data-k="wallend" data-id={w.id} data-i={i} cx={p.x} cy={p.y} r={R} fill={pal.accent} stroke={pal.bg} strokeWidth={2} vectorEffect="non-scaling-stroke" style={{ cursor: 'grab' }} />)}</g>
  }
  return null
}

function rotatePt(p: Pt, deg: number): Pt { const c = Math.cos(deg * Math.PI / 180), s = Math.sin(deg * Math.PI / 180); return { x: p.x * c - p.y * s, y: p.x * s + p.y * c } }

export { Fragment }
