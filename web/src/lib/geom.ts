// Plan geometry: centimetres, x to the right, y down. A room's points are
// its inside outline; walls are drawn outside it. An item's front is its
// local +y side (the bottom edge before it is turned).
import type { DItem, DOpening, DRoom, Drawing, Pt } from './types'
import { kindOf } from './catalog'

export const DEFAULT_WALL = 12

export const add = (a: Pt, b: Pt): Pt => ({ x: a.x + b.x, y: a.y + b.y })
export const sub = (a: Pt, b: Pt): Pt => ({ x: a.x - b.x, y: a.y - b.y })
export const mul = (a: Pt, k: number): Pt => ({ x: a.x * k, y: a.y * k })
export const dot = (a: Pt, b: Pt) => a.x * b.x + a.y * b.y
export const dist = (a: Pt, b: Pt) => Math.hypot(a.x - b.x, a.y - b.y)
export const norm = (a: Pt): Pt => { const l = Math.hypot(a.x, a.y) || 1; return { x: a.x / l, y: a.y / l } }
export const rad = (deg: number) => deg * Math.PI / 180
export const rotate = (p: Pt, deg: number): Pt => { const c = Math.cos(rad(deg)), s = Math.sin(rad(deg)); return { x: p.x * c - p.y * s, y: p.x * s + p.y * c } }

export function emptyDrawing(): Drawing { return { rooms: [], walls: [], openings: [], items: [], dims: [], labels: [] } }
export function fixDrawing(d: Partial<Drawing> | undefined | null): Drawing {
  return { rooms: d?.rooms || [], walls: d?.walls || [], openings: d?.openings || [], items: d?.items || [], dims: d?.dims || [], labels: d?.labels || [] }
}

let seq = 0
export function uid(prefix: string) { seq = (seq + 1) % 1e6; return prefix + Date.now().toString(36) + seq.toString(36) }

export function signedArea(pts: Pt[]) {
  let s = 0
  for (let i = 0; i < pts.length; i++) { const a = pts[i], b = pts[(i + 1) % pts.length]; s += a.x * b.y - b.x * a.y }
  return s / 2
}
export const polyArea = (pts: Pt[]) => Math.abs(signedArea(pts))
export function perimeter(pts: Pt[]) { let p = 0; for (let i = 0; i < pts.length; i++) p += dist(pts[i], pts[(i + 1) % pts.length]); return p }

export function inside(pts: Pt[], p: Pt) {
  let c = false
  for (let i = 0, j = pts.length - 1; i < pts.length; j = i++) {
    const a = pts[i], b = pts[j]
    if ((a.y > p.y) !== (b.y > p.y) && p.x < (b.x - a.x) * (p.y - a.y) / (b.y - a.y) + a.x) c = !c
  }
  return c
}

export function bbox(pts: Pt[]) {
  let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity
  for (const p of pts) { x0 = Math.min(x0, p.x); y0 = Math.min(y0, p.y); x1 = Math.max(x1, p.x); y1 = Math.max(y1, p.y) }
  return { x0, y0, x1, y1, w: x1 - x0, h: y1 - y0 }
}

/** An edge of a room: start, end, direction, outward normal, length. */
export function edge(r: DRoom, i: number) {
  const n = r.points.length
  const a = r.points[i % n], b = r.points[(i + 1) % n]
  const u = norm(sub(b, a))
  const s = signedArea(r.points) >= 0 ? 1 : -1
  const out = { x: u.y * s, y: -u.x * s }
  return { a, b, u, out, inn: mul(out, -1), len: dist(a, b) }
}

export const wallOf = (r: DRoom) => r.wall ?? DEFAULT_WALL

/** Distance from p to the segment ab, and the point on it (t in 0..1). */
export function toSegment(p: Pt, a: Pt, b: Pt) {
  const ab = sub(b, a), l2 = dot(ab, ab)
  const t = l2 ? Math.max(0, Math.min(1, dot(sub(p, a), ab) / l2)) : 0
  const q = add(a, mul(ab, t))
  return { d: dist(p, q), t, q }
}

/** A good spot inside a polygon for its label (far from the edges). */
export function labelPoint(pts: Pt[]): Pt {
  const b = bbox(pts)
  let best = { x: (b.x0 + b.x1) / 2, y: (b.y0 + b.y1) / 2 }, bestD = -1
  const N = 14
  for (let i = 1; i < N; i++) for (let j = 1; j < N; j++) {
    const p = { x: b.x0 + b.w * i / N, y: b.y0 + b.h * j / N }
    if (!inside(pts, p)) continue
    let d = Infinity
    for (let k = 0; k < pts.length; k++) d = Math.min(d, toSegment(p, pts[k], pts[(k + 1) % pts.length]).d)
    // prefer the middle a little
    d -= dist(p, { x: (b.x0 + b.x1) / 2, y: (b.y0 + b.y1) / 2 }) * 0.15
    if (d > bestD) { bestD = d; best = p }
  }
  return best
}

// ---- items ----

/** The four corners of an item (front-left, front-right, back-right, back-left in its own frame). */
export function corners(it: { x: number; y: number; w: number; d: number; rot?: number }, grow = 0): Pt[] {
  const hw = it.w / 2 + grow, hd = it.d / 2 + grow
  return [{ x: -hw, y: -hd }, { x: hw, y: -hd }, { x: hw, y: hd }, { x: -hw, y: hd }].map(p => add(rotate(p, it.rot || 0), { x: it.x, y: it.y }))
}

/** A rectangle next to an item, in its own frame: front zone, a side, the back. */
export function zone(it: DItem, side: 'front' | 'left' | 'right' | 'back', depth: number): Pt[] {
  const hw = it.w / 2, hd = it.d / 2
  let pts: Pt[]
  switch (side) {
    case 'front': pts = [{ x: -hw, y: hd }, { x: hw, y: hd }, { x: hw, y: hd + depth }, { x: -hw, y: hd + depth }]; break
    case 'back': pts = [{ x: -hw, y: -hd - depth }, { x: hw, y: -hd - depth }, { x: hw, y: -hd }, { x: -hw, y: -hd }]; break
    case 'left': pts = [{ x: -hw - depth, y: -hd }, { x: -hw, y: -hd }, { x: -hw, y: hd }, { x: -hw - depth, y: hd }]; break
    case 'right': pts = [{ x: hw, y: -hd }, { x: hw + depth, y: -hd }, { x: hw + depth, y: hd }, { x: hw, y: hd }]; break
  }
  return pts.map(p => add(rotate(p, it.rot || 0), { x: it.x, y: it.y }))
}

/** Separating-axis test for two convex polygons (shrunk a hair so touching is fine). */
export function overlap(a: Pt[], b: Pt[], eps = 0.5) {
  for (const poly of [a, b]) {
    for (let i = 0; i < poly.length; i++) {
      const p = poly[i], q = poly[(i + 1) % poly.length]
      const ax = norm({ x: q.y - p.y, y: p.x - q.x })
      let a0 = Infinity, a1 = -Infinity, b0 = Infinity, b1 = -Infinity
      for (const v of a) { const s = dot(v, ax); a0 = Math.min(a0, s); a1 = Math.max(a1, s) }
      for (const v of b) { const s = dot(v, ax); b0 = Math.min(b0, s); b1 = Math.max(b1, s) }
      if (a1 - eps <= b0 || b1 - eps <= a0) return false
    }
  }
  return true
}

/** Segment intersection test. */
function segX(p1: Pt, p2: Pt, p3: Pt, p4: Pt) {
  const d = (p2.x - p1.x) * (p4.y - p3.y) - (p2.y - p1.y) * (p4.x - p3.x)
  if (Math.abs(d) < 1e-9) return false
  const t = ((p3.x - p1.x) * (p4.y - p3.y) - (p3.y - p1.y) * (p4.x - p3.x)) / d
  const u = ((p3.x - p1.x) * (p2.y - p1.y) - (p3.y - p1.y) * (p2.x - p1.x)) / d
  return t > 0.001 && t < 0.999 && u > 0.001 && u < 0.999
}

/** Whether a convex shape lies inside a (possibly concave) polygon. */
export function within(shape: Pt[], poly: Pt[]) {
  for (const p of shape) if (!inside(poly, p) && !onEdge(poly, p)) return false
  for (let i = 0; i < shape.length; i++) for (let j = 0; j < poly.length; j++) {
    if (segX(shape[i], shape[(i + 1) % shape.length], poly[j], poly[(j + 1) % poly.length])) return false
  }
  return true
}
function onEdge(poly: Pt[], p: Pt) {
  for (let i = 0; i < poly.length; i++) if (toSegment(p, poly[i], poly[(i + 1) % poly.length]).d < 0.6) return true
  return false
}

export function roomAt(d: Drawing, p: Pt): DRoom | undefined { return d.rooms.find(r => inside(r.points, p)) }

// ---- openings ----

/** Where an opening is: centre, the two jambs, the edge frame and wall thickness. */
export function openingGeom(d: Drawing, o: DOpening) {
  const r = d.rooms.find(x => x.id === o.room)
  if (r && r.points.length > 1) {
    const e = edge(r, o.edge)
    const c = add(e.a, mul(e.u, o.at))
    return { c, j0: add(c, mul(e.u, -o.width / 2)), j1: add(c, mul(e.u, o.width / 2)), u: e.u, out: e.out, inn: e.inn, t: wallOf(r), len: e.len, room: r }
  }
  const w = d.walls.find(x => x.id === o.wall)
  if (w) {
    const u = norm(sub(w.b, w.a))
    const out = { x: u.y, y: -u.x }
    const t = w.thick ?? DEFAULT_WALL
    const c = add(add(w.a, mul(u, o.at)), mul(out, -t / 2))
    return { c, j0: add(c, mul(u, -o.width / 2)), j1: add(c, mul(u, o.width / 2)), u, out, inn: mul(out, -1), t, len: dist(w.a, w.b), room: undefined }
  }
  return null
}

const swings = (k: string) => k === 'door' || k === 'double'

/** The floor a door sweeps when it opens (a polygon), or null. */
export function swingArea(d: Drawing, o: DOpening): Pt[] | null {
  if (!swings(o.kind)) return null
  const g = openingGeom(d, o)
  if (!g) return null
  const side = o.out ? g.out : g.inn
  const base = o.out ? add(g.c, mul(g.out, g.t)) : g.c
  const leaf = (hinge: Pt, toward: Pt, r: number) => {
    const pts: Pt[] = [hinge]
    for (let k = 0; k <= 6; k++) {
      const a = (Math.PI / 2) * k / 6
      pts.push(add(hinge, add(mul(toward, Math.cos(a) * r), mul(side, Math.sin(a) * r))))
    }
    return pts
  }
  const j0 = add(base, mul(g.u, -o.width / 2)), j1 = add(base, mul(g.u, o.width / 2))
  if (o.kind === 'double') return [...leaf(j0, g.u, o.width / 2), ...leaf(j1, mul(g.u, -1), o.width / 2).reverse()]
  return o.hinge === 'end' ? leaf(j1, mul(g.u, -1), o.width) : leaf(j0, g.u, o.width)
}

/** Convex hull (for door sweeps, which the overlap test needs convex). */
export function hull(pts: Pt[]): Pt[] {
  const p = [...pts].sort((a, b) => a.x - b.x || a.y - b.y)
  if (p.length < 3) return p
  const cross = (o: Pt, a: Pt, b: Pt) => (a.x - o.x) * (b.y - o.y) - (a.y - o.y) * (b.x - o.x)
  const lo: Pt[] = [], up: Pt[] = []
  for (const q of p) { while (lo.length >= 2 && cross(lo[lo.length - 2], lo[lo.length - 1], q) <= 0) lo.pop(); lo.push(q) }
  for (const q of [...p].reverse()) { while (up.length >= 2 && cross(up[up.length - 2], up[up.length - 1], q) <= 0) up.pop(); up.push(q) }
  return [...lo.slice(0, -1), ...up.slice(0, -1)]
}

// ---- the checks ----

export interface Issue { level: 'warn' | 'info'; text: string; ids: string[]; at: Pt }

const isFlat = (it: DItem) => kindOf(it.type).flat || (it.h ?? kindOf(it.type).h) <= 2
const zRange = (it: DItem): [number, number] => { const e = it.elev || 0; return [e, e + (it.h ?? kindOf(it.type).h)] }
const zOverlap = (a: DItem, b: DItem) => { const [a0, a1] = zRange(a), [b0, b1] = zRange(b); return a0 < b1 - 1 && b0 < a1 - 1 }
const nameOf = (it: DItem) => it.name || kindOf(it.type).name
const SEATS = ['chair', 'office_chair', 'bench']
const TABLES = ['dining_table', 'table_round', 'desk', 'island', 'garden_table', 'coffee_table']
const tucks = (a: DItem, b: DItem) => (SEATS.includes(a.type) && TABLES.includes(b.type)) || (SEATS.includes(b.type) && TABLES.includes(a.type))

/** The footprint of a piece as convex parts (a corner sofa is two). */
export function parts(it: DItem, grow = 0): Pt[][] {
  if (it.type !== 'sofa_l') return [corners(it, grow)]
  const dd = Math.min(95, it.d * 0.55), cw = Math.min(100, it.w * 0.38)
  const frame = (x0: number, y0: number, x1: number, y1: number) => [{ x: x0 - grow, y: y0 - grow }, { x: x1 + grow, y: y0 - grow }, { x: x1 + grow, y: y1 + grow }, { x: x0 - grow, y: y1 + grow }]
    .map(p => add(rotate(p, it.rot || 0), { x: it.x, y: it.y }))
  return [frame(-it.w / 2, -it.d / 2, it.w / 2, -it.d / 2 + dd), frame(it.w / 2 - cw, -it.d / 2 + dd, it.w / 2, it.d / 2)]
}
const anyOverlap = (a: Pt[][], b: Pt[][], eps = 1) => a.some(p => b.some(q => overlap(p, q, eps)))

export function checkLayout(d: Drawing, opts: { clearance: boolean; fmt: (cm: number) => string }): Issue[] {
  const out: Issue[] = []
  const items = d.items
  const polys = items.map(it => parts(it))
  const solid = items.map(it => !isFlat(it))
  // 1. pieces on top of each other (a chair may tuck under its table)
  for (let i = 0; i < items.length; i++) for (let j = i + 1; j < items.length; j++) {
    if (!solid[i] || !solid[j] || !zOverlap(items[i], items[j]) || tucks(items[i], items[j])) continue
    if (anyOverlap(polys[i], polys[j], 1)) out.push({ level: 'warn', text: `${nameOf(items[i])} and ${nameOf(items[j])} overlap`, ids: [items[i].id, items[j].id], at: { x: (items[i].x + items[j].x) / 2, y: (items[i].y + items[j].y) / 2 } })
  }
  // 2. pieces through a wall
  for (let i = 0; i < items.length; i++) {
    if (!d.rooms.length || isFlat(items[i])) continue
    const host = d.rooms.find(r => inside(r.points, items[i]))
    if (!host) { out.push({ level: 'warn', text: `${nameOf(items[i])} is outside every room`, ids: [items[i].id], at: items[i] }); continue }
    if (!parts(items[i], -0.6).every(p => within(p, host.points))) out.push({ level: 'warn', text: `${nameOf(items[i])} goes through a wall of the ${host.name.toLowerCase()}`, ids: [items[i].id], at: items[i] })
  }
  // 3. doors that cannot open
  for (const o of d.openings) {
    const sw = swingArea(d, o)
    if (!sw) continue
    const h = hull(sw)
    for (let i = 0; i < items.length; i++) {
      if (!solid[i] || (items[i].elev || 0) > (o.height || 205)) continue
      if (anyOverlap([h], polys[i], 2)) out.push({ level: 'warn', text: `${nameOf(items[i])} is in the way of ${o.name || (o.kind === 'double' ? 'a double door' : 'a door')}`, ids: [items[i].id, o.id], at: items[i] })
    }
  }
  if (!opts.clearance) return out
  // 4. room to use things: in front of wardrobes, around beds and tables…
  for (let i = 0; i < items.length; i++) {
    const it = items[i]
    const c = kindOf(it.type).clear
    if (!c || (it.elev || 0) > 40) continue
    const sides: { side: 'front' | 'left' | 'right' | 'back'; need: number }[] = []
    if (c.all) sides.push({ side: 'front', need: c.all }, { side: 'back', need: c.all }, { side: 'left', need: c.all }, { side: 'right', need: c.all })
    if (c.front) sides.push({ side: 'front', need: c.front })
    if (c.sides) sides.push({ side: 'left', need: c.sides }, { side: 'right', need: c.sides })
    const results = sides.map(s => ({ ...s, free: freeDepth(d, it, i, s.side, s.need, c.allow || [], polys, solid) }))
    if (c.all) {
      const short = results.filter(r => r.free < r.need - 0.5)
      // a table needs room on the sides people sit; against a wall on one side is fine
      if (short.length >= 2 || short.some(r => r.free < 45)) {
        const worst = short.sort((a, b) => a.free - b.free)[0]
        if (worst) out.push({ level: 'info', text: `${nameOf(it)}: ${opts.fmt(Math.max(0, worst.free))} to pull a chair out on one side (${opts.fmt(worst.need)} is comfortable)`, ids: [it.id], at: it })
      }
      continue
    }
    if (c.oneSide) {
      const lr = results.filter(r => r.side === 'left' || r.side === 'right')
      if (lr.length && lr.every(r => r.free < r.need - 0.5)) {
        const best = Math.max(...lr.map(r => r.free))
        out.push({ level: 'info', text: `${nameOf(it)}: ${opts.fmt(Math.max(0, best))} to get in beside it (${opts.fmt(c.sides!)} is comfortable)`, ids: [it.id], at: it })
      }
      continue
    }
    for (const r of results) {
      if (r.free >= r.need - 0.5) continue
      const where = r.side === 'front' ? (it.type.startsWith('bed') ? 'at the foot' : 'in front') : `on the ${r.side}`
      out.push({ level: 'info', text: `${nameOf(it)}: ${opts.fmt(Math.max(0, r.free))} free ${where} (it needs about ${opts.fmt(r.need)})`, ids: [it.id], at: it })
    }
  }
  return out
}

/** How deep the zone beside an item can go before it meets something (up to need). */
function freeDepth(d: Drawing, it: DItem, idx: number, side: 'front' | 'left' | 'right' | 'back', need: number, allow: string[], polys: Pt[][][], solid: boolean[]) {
  const host = d.rooms.find(r => inside(r.points, it))
  const clear = (depth: number) => {
    if (depth <= 0.5) return true
    const z = zone(it, side, depth)
    const zs = [z[0], z[1], z[2], z[3]].map(p => p)
    const shrunk = shrink(zs, 0.8)
    if (host && !within(shrunk, host.points)) {
      // an open edge (no wall) to a neighbouring room is not a wall
      if (!d.rooms.some(r => r !== host && within(shrunk, r.points)) && !roomsCover(d, shrunk)) return false
    }
    for (let j = 0; j < d.items.length; j++) {
      if (j === idx || !solid[j] || allow.includes(d.items[j].type)) continue
      if ((d.items[j].elev || 0) > 60) continue
      if (anyOverlap([shrunk], polys[j], 1)) return false
    }
    return true
  }
  if (clear(need)) return need
  let lo = 0, hi = need
  for (let k = 0; k < 12; k++) { const m = (lo + hi) / 2; if (clear(m)) lo = m; else hi = m }
  return lo
}

function shrink(pts: Pt[], by: number) {
  const c = mul(pts.reduce((a, b) => add(a, b), { x: 0, y: 0 }), 1 / pts.length)
  return pts.map(p => { const v = sub(p, c); const l = Math.hypot(v.x, v.y) || 1; return add(c, mul(v, Math.max(0, l - by) / l)) })
}

/** The zone is covered by rooms joined by open edges (no wall between them). */
function roomsCover(d: Drawing, z: Pt[]) {
  return z.every(p => d.rooms.some(r => inside(r.points, p)))
    && !d.rooms.some(r => {
      // a wall between the rooms the zone covers
      for (let i = 0; i < r.points.length; i++) {
        if ((r.open || []).includes(i)) continue
        const a = r.points[i], b = r.points[(i + 1) % r.points.length]
        for (let k = 0; k < z.length; k++) if (segX(z[k], z[(k + 1) % z.length], a, b)) return true
      }
      return false
    })
}

// ---- placing things ----

/** Snap an item's back against the nearest wall, if one is close. */
export function snapToWall(d: Drawing, it: DItem, reach = 25): DItem | null {
  let best: { dd: number; rot: number; x: number; y: number } | null = null
  for (const r of d.rooms) {
    for (let i = 0; i < r.points.length; i++) {
      if ((r.open || []).includes(i)) continue
      const e = edge(r, i)
      if (e.len < it.w * 0.6) continue
      const rot = (Math.atan2(e.out.x, -e.out.y) * 180) / Math.PI
      const back = add({ x: it.x, y: it.y }, mul(e.out, it.d / 2))
      const s = toSegment(back, e.a, e.b)
      if (s.d > reach) continue
      // keep it on the edge, and on the inside
      const along = Math.max(it.w / 2, Math.min(e.len - it.w / 2, s.t * e.len))
      const p = add(add(e.a, mul(e.u, along)), mul(e.inn, it.d / 2))
      if (!inside(r.points, p)) continue
      if (!best || s.d < best.dd) best = { dd: s.d, rot: ((Math.round(rot) % 360) + 360) % 360, x: p.x, y: p.y }
    }
  }
  return best ? { ...it, x: best.x, y: best.y, rot: best.rot } : null
}

export function snap(v: number, step: number) { return step > 0 ? Math.round(v / step) * step : v }

export function drawingBounds(d: Drawing, pad = 60) {
  const pts: Pt[] = []
  for (const r of d.rooms) for (const p of r.points) pts.push(p)
  for (const w of d.walls) pts.push(w.a, w.b)
  for (const it of d.items) pts.push(...corners(it))
  for (const m of d.dims) pts.push(m.a, m.b)
  for (const l of d.labels) pts.push(l.at)
  if (!pts.length) return { x0: -pad, y0: -pad, x1: 600 + pad, y1: 400 + pad, w: 600 + 2 * pad, h: 400 + 2 * pad }
  const b = bbox(pts)
  return { x0: b.x0 - pad, y0: b.y0 - pad, x1: b.x1 + pad, y1: b.y1 + pad, w: b.w + 2 * pad, h: b.h + 2 * pad }
}

/** A rectangle room at x, y of w × d. */
export function rectRoom(name: string, kind: string, x: number, y: number, w: number, d: number, floor: string, wall: number): DRoom {
  return { id: uid('r'), name, kind, floor, wall, points: [{ x, y }, { x: x + w, y }, { x: x + w, y: y + d }, { x, y: y + d }] }
}

/** Width and depth of a rectangular room, or null. */
export function rectSize(r: DRoom): { w: number; d: number } | null {
  if (r.points.length !== 4) return null
  for (let i = 0; i < 4; i++) {
    const a = r.points[i], b = r.points[(i + 1) % 4], c = r.points[(i + 2) % 4]
    if (Math.abs(dot(sub(b, a), sub(c, b))) > 1) return null
  }
  return { w: dist(r.points[0], r.points[1]), d: dist(r.points[1], r.points[2]) }
}

/** Item ids that changed between two layouts (for compare). */
export function diffItems(a: Drawing, b: Drawing) {
  const ma = new Map(a.items.map(i => [i.id, i])), mb = new Map(b.items.map(i => [i.id, i]))
  const added = b.items.filter(i => !ma.has(i.id)), removed = a.items.filter(i => !mb.has(i.id))
  const moved = b.items.filter(i => { const o = ma.get(i.id); return o && (dist(o, i) > 2 || Math.abs((o.rot || 0) - (i.rot || 0)) > 1 || o.w !== i.w || o.d !== i.d) })
  return { added, removed, moved }
}
