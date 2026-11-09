import type { ReactElement } from 'react'
// The floor plan editor: draw rooms, walls, doors and windows, place and
// turn furniture, measure — to scale, in centimetres. Changes save by
// themselves; undo and redo go back and forth.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { DItem, DOpening, DRoom, Drawing, Floor, Layout, Pt, Underlay } from '../lib/types'
import { add, checkLayout, corners, dist, drawingBounds, edge, inside, mul, rectRoom, rotate, snap, snapToWall, sub, toSegment, uid, type Issue, wallOf, fixDrawing, dot } from '../lib/geom'
import { kindOf } from '../lib/catalog'
import { len } from '../lib/units'
import { api, ApiError, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { Dim, Handles, PlanSvg, type Sel } from './PlanSvg'
import { fitView, usePalette, useSize } from './Preview'
import { Icon } from './icons'
import { errMsg, modalOpen, useIsMobile, useToast } from './ui'
import { Inspector } from './Inspector'
import { AddFurniture, type Placeable } from './AddFurniture'

export type Tool = 'select' | 'room' | 'shape' | 'wall' | 'door' | 'window' | 'measure' | 'label' | 'scale'

type Drag =
  | { t: 'pan'; sx: number; sy: number; tx: number; ty: number; moved: boolean }
  | { t: 'item'; id: string; start: Pt; orig: DItem; moved: boolean }
  | { t: 'rot'; id: string }
  | { t: 'resize'; id: string; corner: number; opp: Pt }
  | { t: 'room'; id: string; start: Pt; orig: DRoom; items: DItem[]; moved: boolean }
  | { t: 'vertex'; id: string; i: number }
  | { t: 'edge'; id: string; i: number; start: Pt; orig: DRoom }
  | { t: 'opening'; id: string }
  | { t: 'thing'; kind: 'dim' | 'label' | 'wall'; id: string; start: Pt; orig: unknown; moved: boolean }
  | { t: 'end'; kind: 'dimend' | 'wallend'; id: string; i: number }
  | { t: 'draw'; start: Pt; cur: Pt }
  | { t: 'underlay'; start: Pt; ox: number; oy: number }

const MAX_HISTORY = 120

export interface EditorProps {
  layout: Layout
  floor: Floor
  readOnly?: boolean
  onSaved: (l: Layout) => void
  onFloor: (f: Partial<Floor>) => void
  focus?: string | null
}

export function Editor({ layout, floor, readOnly, onSaved, onFloor, focus }: EditorProps) {
  const { settings } = useApp()
  const { toast } = useToast()
  const mobile = useIsMobile()
  const pal = usePalette()
  const [d, setD] = useState<Drawing>(() => fixDrawing(layout.data))
  const [sel, setSel] = useState<Sel[]>([])
  const [tool, setTool] = useState<Tool>('select')
  const [past, setPast] = useState<Drawing[]>([])
  const [future, setFuture] = useState<Drawing[]>([])
  const [ref, W, H, measured] = useSize<HTMLDivElement>([800, 560])
  const [view, setView] = useState<{ k: number; tx: number; ty: number } | null>(null)
  const [draft, setDraft] = useState<{ pts: Pt[]; cur?: Pt } | null>(null)
  const [save, setSave] = useState<'saved' | 'dirty' | 'saving' | 'error'>('saved')
  const [adding, setAdding] = useState(false)
  const [checksOpen, setChecksOpen] = useState(false)
  const [panel, setPanel] = useState(!mobile)
  const [moveContents, setMoveContents] = useState(true)
  const [wallMagnet, setWallMagnet] = useState(true)
  const [scaleDraft, setScaleDraft] = useState<{ a: Pt; b: Pt } | null>(null)
  const svgRef = useRef<SVGSVGElement>(null)
  const drag = useRef<Drag | null>(null)
  const before = useRef<Drawing | null>(null)
  const pointers = useRef(new Map<number, Pt>())
  const pinch = useRef<{ d: number; k: number; cx: number; cy: number; tx: number; ty: number } | null>(null)
  const rev = useRef(layout.revision)
  const lastSaved = useRef(JSON.stringify(fixDrawing(layout.data)))
  const step = settings?.snap ?? 5
  const wallCm = settings?.wall_cm || 12

  // a different layout: start over
  useEffect(() => {
    const nd = fixDrawing(layout.data)
    setD(nd); setPast([]); setFuture([]); setSel([]); setDraft(null); setTool('select')
    rev.current = layout.revision; lastSaved.current = JSON.stringify(nd); setSave('saved'); setView(null)
  }, [layout.id]) // eslint-disable-line react-hooks/exhaustive-deps

  // fit the plan once the box is measured
  useEffect(() => { if (!view && measured && W > 50) setView(fitView(d, W, H, mobile ? 24 : 60)) }, [W, H, view, d, measured, mobile])
  useEffect(() => { if (focus) { const it = d.items.find(i => i.id === focus); if (it) setSel([{ kind: 'item', id: it.id }]) } }, [focus]) // eslint-disable-line react-hooks/exhaustive-deps

  // ---- saving ----
  useEffect(() => {
    if (readOnly) return
    const json = JSON.stringify(d)
    if (json === lastSaved.current) return
    setSave('dirty')
    const t = window.setTimeout(async () => {
      setSave('saving')
      try {
        const l = await api.put<Layout>(`${V1}/layouts/${layout.id}`, { data: d, revision: rev.current })
        rev.current = l.revision; lastSaved.current = json; setSave('saved'); onSaved(l)
      } catch (e) {
        setSave('error')
        toast(e instanceof ApiError && e.status === 409 ? errMsg(e) : 'Could not save: ' + errMsg(e), 'err')
      }
    }, 700)
    return () => window.clearTimeout(t)
  }, [d, layout.id, readOnly]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => { if (JSON.stringify(d) !== lastSaved.current) { e.preventDefault() } }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [d])

  // ---- history ----
  const commit = useCallback((next: Drawing) => {
    setPast(p => [...p.slice(-MAX_HISTORY), d]); setFuture([]); setD(next)
  }, [d])
  const undo = () => { if (!past.length) return; setFuture(f => [d, ...f]); setD(past[past.length - 1]); setPast(p => p.slice(0, -1)) }
  const redo = () => { if (!future.length) return; setPast(p => [...p, d]); setD(future[0]); setFuture(f => f.slice(1)) }

  const issues = useMemo<Issue[]>(() => checkLayout(d, { clearance: settings?.clearance !== false, fmt: cm => len(cm) }), [d, settings?.clearance])
  const issueIDs = useMemo(() => new Set(issues.flatMap(i => i.ids)), [issues])
  const v = view || { k: 1, tx: 0, ty: 0 }

  const toWorld = (cx: number, cy: number): Pt => {
    const r = svgRef.current!.getBoundingClientRect()
    return { x: (cx - r.left - v.tx) / v.k, y: (cy - r.top - v.ty) / v.k }
  }
  const sp = (p: Pt): Pt => ({ x: snap(p.x, step), y: snap(p.y, step) })
  const zoomAt = (factor: number, sx = W / 2, sy = H / 2) => setView(cur => {
    const c = cur || v
    const k = Math.max(0.05, Math.min(12, c.k * factor))
    return { k, tx: sx - (sx - c.tx) * k / c.k, ty: sy - (sy - c.ty) * k / c.k }
  })
  const fit = () => setView(fitView(d, W, H, mobile ? 24 : 60))

  // ---- changing things ----
  const update = (fn: (d: Drawing) => Drawing) => commit(fn(d))
  const setItem = (id: string, patch: Partial<DItem>, history = true) => {
    const next = { ...d, items: d.items.map(i => i.id === id ? { ...i, ...patch } : i) }
    if (history) commit(next); else setD(next)
  }
  const removeSel = () => {
    if (!sel.length || readOnly) return
    const ids = new Set(sel.map(s => s.id))
    update(x => ({
      ...x,
      items: x.items.filter(i => !ids.has(i.id) || i.locked),
      rooms: x.rooms.filter(r => !ids.has(r.id)),
      openings: x.openings.filter(o => !ids.has(o.id) && !ids.has(o.room || '') && !ids.has(o.wall || '')),
      walls: x.walls.filter(w => !ids.has(w.id)),
      dims: x.dims.filter(m => !ids.has(m.id)),
      labels: x.labels.filter(l => !ids.has(l.id)),
    }))
    setSel([])
  }
  const duplicateSel = () => {
    const s = sel[0]
    if (!s || readOnly) return
    if (s.kind === 'item') {
      const it = d.items.find(i => i.id === s.id)
      if (!it) return
      const c = { ...it, id: uid('i'), x: it.x + 20, y: it.y + 20, locked: false }
      commit({ ...d, items: [...d.items, c] }); setSel([{ kind: 'item', id: c.id }])
    } else if (s.kind === 'room') {
      const r = d.rooms.find(x => x.id === s.id)
      if (!r) return
      const b = r.points.reduce((m, p) => Math.max(m, p.x), -Infinity) - r.points.reduce((m, p) => Math.min(m, p.x), Infinity)
      const nr = { ...r, id: uid('r'), name: r.name + ' 2', points: r.points.map(p => ({ x: p.x + b + wallOf(r), y: p.y })) }
      commit({ ...d, rooms: [...d.rooms, nr] }); setSel([{ kind: 'room', id: nr.id }])
    }
  }
  const rotateSel = (deg: number) => {
    const s = sel[0]
    if (s?.kind !== 'item') return
    const it = d.items.find(i => i.id === s.id)
    if (it && !it.locked) setItem(it.id, { rot: (((it.rot || 0) + deg) % 360 + 360) % 360 })
  }
  const nudge = (dx: number, dy: number) => {
    const s = sel[0]
    if (!s) return
    if (s.kind === 'item') { const it = d.items.find(i => i.id === s.id); if (it && !it.locked) setItem(it.id, { x: it.x + dx, y: it.y + dy }) }
    if (s.kind === 'room') update(x => ({ ...x, rooms: x.rooms.map(r => r.id === s.id ? { ...r, points: r.points.map(p => ({ x: p.x + dx, y: p.y + dy })) } : r) }))
  }

  /** Put a new piece in the middle of the view (or of the chosen room). */
  const place = (p: Placeable) => {
    const k = kindOf(p.type)
    let at: Pt = { x: (W / 2 - v.tx) / v.k, y: (H / 2 - v.ty) / v.k }
    const room = sel[0]?.kind === 'room' ? d.rooms.find(r => r.id === sel[0].id) : undefined
    if (room) { const b = room.points.reduce((a, q) => add(a, q), { x: 0, y: 0 }); at = mul(b, 1 / room.points.length) }
    const it: DItem = { id: uid('i'), name: p.name || k.name, type: p.type, cat: k.cat, x: snap(at.x, step), y: snap(at.y, step), w: p.w || k.w, d: p.d || k.d, h: p.h ?? k.h, rot: 0,
      color: p.color || undefined, piece_id: p.piece_id || undefined, price: p.price || undefined, buy: p.buy || undefined, from: p.from || undefined }
    commit({ ...d, items: [...d.items, it] })
    setSel([{ kind: 'item', id: it.id }]); setTool('select'); setAdding(false)
    if (mobile) setPanel(true)
  }

  // ---- pointer ----
  const hitOf = (el: Element | null) => {
    const h = el?.closest('[data-k]') as HTMLElement | SVGElement | null
    return h ? { k: h.getAttribute('data-k')!, id: h.getAttribute('data-id')!, i: Number(h.getAttribute('data-i') || 0) } : null
  }
  const nearestEdge = (p: Pt, reach: number) => {
    let best: { room: DRoom; i: number; at: number; d: number } | null = null
    for (const r of d.rooms) for (let i = 0; i < r.points.length; i++) {
      const e = edge(r, i)
      const s = toSegment(p, e.a, add(e.a, mul(e.u, e.len)))
      // the wall band is outside the edge: measure to its middle too
      const s2 = toSegment(p, add(e.a, mul(e.out, wallOf(r) / 2)), add(e.b, mul(e.out, wallOf(r) / 2)))
      const dd = Math.min(s.d, s2.d)
      if (dd < reach && (!best || dd < best.d)) best = { room: r, i, at: s.t * e.len, d: dd }
    }
    return best
  }

  const onPointerDown = (e: React.PointerEvent<SVGSVGElement>) => {
    if (!view) return
    svgRef.current?.setPointerCapture(e.pointerId)
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
    if (pointers.current.size === 2) {
      const [a, b] = [...pointers.current.values()]
      const r = svgRef.current!.getBoundingClientRect()
      pinch.current = { d: dist(a, b), k: v.k, cx: (a.x + b.x) / 2 - r.left, cy: (a.y + b.y) / 2 - r.top, tx: v.tx, ty: v.ty }
      if (drag.current && before.current) { setD(before.current); before.current = null }
      drag.current = null
      return
    }
    const w = toWorld(e.clientX, e.clientY)
    const hit = hitOf(e.target as Element)
    const pan = () => { drag.current = { t: 'pan', sx: e.clientX, sy: e.clientY, tx: v.tx, ty: v.ty, moved: false } }
    if (e.button === 1) { pan(); return }
    if (readOnly) { if (hit && ['item', 'room', 'opening'].includes(hit.k)) setSel([{ kind: hit.k as Sel['kind'], id: hit.id }]); pan(); return }
    before.current = d
    switch (tool) {
      case 'room': drag.current = { t: 'draw', start: sp(w), cur: sp(w) }; setDraft({ pts: [sp(w)], cur: sp(w) }); return
      case 'measure': drag.current = { t: 'draw', start: sp(w), cur: sp(w) }; setDraft({ pts: [sp(w)], cur: sp(w) }); return
      case 'scale': drag.current = { t: 'draw', start: w, cur: w }; setScaleDraft({ a: w, b: w }); return
      case 'shape': {
        const p = sp(w)
        if (draft && draft.pts.length >= 3 && dist(p, draft.pts[0]) * v.k < 14) { finishShape(draft.pts); return }
        setDraft(dr => ({ pts: [...(dr?.pts || []), p], cur: p }))
        return
      }
      case 'wall': {
        const p = sp(w)
        if (draft && draft.pts.length) {
          const a = draft.pts[draft.pts.length - 1]
          if (dist(a, p) > 5) commit({ ...d, walls: [...d.walls, { id: uid('w'), a, b: p, thick: wallCm }] })
          setDraft({ pts: [p], cur: p })
        } else setDraft({ pts: [p], cur: p })
        return
      }
      case 'door': case 'window': {
        const ne = nearestEdge(w, 60)
        if (!ne) { toast('Click on a wall of a room to put it there'); return }
        const width = tool === 'door' ? (settings?.door_cm || 80) : 120
        const e2 = edge(ne.room, ne.i)
        if (e2.len < width + 4) { toast('That wall is too short for it'); return }
        const o: DOpening = { id: uid('o'), kind: tool === 'door' ? 'door' : 'window', room: ne.room.id, edge: ne.i, at: Math.max(width / 2, Math.min(e2.len - width / 2, snap(ne.at, step))), width,
          height: tool === 'door' ? 205 : 140, sill: tool === 'window' ? 90 : undefined, hinge: 'start' }
        commit({ ...d, openings: [...d.openings, o] }); setSel([{ kind: 'opening', id: o.id }]); setTool('select')
        return
      }
      case 'label': {
        const l = { id: uid('t'), at: sp(w), text: 'Note', size: 13 }
        commit({ ...d, labels: [...d.labels, l] }); setSel([{ kind: 'label', id: l.id }]); setTool('select'); if (mobile) setPanel(true)
        return
      }
    }
    // select tool
    if (!hit) { setSel([]); pan(); if (e.altKey || (floor.underlay?.on && e.shiftKey)) drag.current = { t: 'underlay', start: w, ox: floor.underlay?.x || 0, oy: floor.underlay?.y || 0 }; return }
    switch (hit.k) {
      case 'item': {
        const it = d.items.find(i => i.id === hit.id)!
        setSel([{ kind: 'item', id: hit.id }])
        if (!it.locked) drag.current = { t: 'item', id: it.id, start: w, orig: it, moved: false }
        else pan()
        return
      }
      case 'rot': drag.current = { t: 'rot', id: hit.id }; return
      case 'resize': {
        const it = d.items.find(i => i.id === hit.id)!
        const cs = corners(it)
        drag.current = { t: 'resize', id: it.id, corner: hit.i, opp: cs[(hit.i + 2) % 4] }
        return
      }
      case 'room': {
        const r = d.rooms.find(x => x.id === hit.id)!
        setSel([{ kind: 'room', id: r.id }])
        drag.current = { t: 'room', id: r.id, start: w, orig: r, items: moveContents ? d.items.filter(i => inside(r.points, i)) : [], moved: false }
        return
      }
      case 'vertex': drag.current = { t: 'vertex', id: hit.id, i: hit.i }; return
      case 'edge': {
        if (e.detail >= 2) { // double-click: a new corner in the middle
          const r = d.rooms.find(x => x.id === hit.id)!
          const ed = edge(r, hit.i)
          const pts = [...r.points]
          pts.splice(hit.i + 1, 0, mul(add(ed.a, ed.b), 0.5))
          commit({ ...d, rooms: d.rooms.map(x => x.id === r.id ? { ...x, points: pts } : x), openings: d.openings.map(o => o.room === r.id && o.edge > hit.i ? { ...o, edge: o.edge + 1 } : o) })
          drag.current = null
          return
        }
        const r = d.rooms.find(x => x.id === hit.id)!
        drag.current = { t: 'edge', id: hit.id, i: hit.i, start: w, orig: r }
        return
      }
      case 'opening': setSel([{ kind: 'opening', id: hit.id }]); drag.current = { t: 'opening', id: hit.id }; return
      case 'dim': case 'label': case 'wall': {
        setSel([{ kind: hit.k as Sel['kind'], id: hit.id }])
        const orig = hit.k === 'dim' ? d.dims.find(m => m.id === hit.id) : hit.k === 'label' ? d.labels.find(l => l.id === hit.id) : d.walls.find(x => x.id === hit.id)
        drag.current = { t: 'thing', kind: hit.k as 'dim' | 'label' | 'wall', id: hit.id, start: w, orig, moved: false }
        return
      }
      case 'dimend': case 'wallend': drag.current = { t: 'end', kind: hit.k as 'dimend' | 'wallend', id: hit.id, i: hit.i }; return
    }
  }

  const onPointerMove = (e: React.PointerEvent<SVGSVGElement>) => {
    if (!view) return
    if (pointers.current.has(e.pointerId)) pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
    if (pinch.current && pointers.current.size >= 2) {
      const [a, b] = [...pointers.current.values()]
      const pc = pinch.current
      const k = Math.max(0.05, Math.min(12, pc.k * dist(a, b) / (pc.d || 1)))
      const r = svgRef.current!.getBoundingClientRect()
      const cx = (a.x + b.x) / 2 - r.left, cy = (a.y + b.y) / 2 - r.top
      setView({ k, tx: cx - (pc.cx - pc.tx) * k / pc.k, ty: cy - (pc.cy - pc.ty) * k / pc.k })
      return
    }
    const w = toWorld(e.clientX, e.clientY)
    if (!drag.current) {
      if ((tool === 'shape' || tool === 'wall') && draft) setDraft({ ...draft, cur: sp(w) })
      return
    }
    const g = drag.current
    switch (g.t) {
      case 'pan': setView({ k: v.k, tx: g.tx + e.clientX - g.sx, ty: g.ty + e.clientY - g.sy }); g.moved = true; return
      case 'draw': {
        const cur = tool === 'scale' ? w : sp(w)
        g.cur = cur
        if (tool === 'scale') setScaleDraft({ a: g.start, b: cur }); else setDraft({ pts: [g.start], cur })
        return
      }
      case 'item': {
        const dx = w.x - g.start.x, dy = w.y - g.start.y
        if (!g.moved && Math.hypot(dx, dy) * v.k < 3) return
        g.moved = true
        let next: DItem = { ...g.orig, x: snap(g.orig.x + dx, step), y: snap(g.orig.y + dy, step) }
        if (wallMagnet && !e.altKey && !kindOf(next.type).flat) next = snapToWall(d, next, 18) || next
        setD(cur => ({ ...cur, items: cur.items.map(i => i.id === g.id ? next : i) }))
        return
      }
      case 'rot': {
        const it = d.items.find(i => i.id === g.id)
        if (!it) return
        let a = Math.atan2(w.x - it.x, -(w.y - it.y)) * 180 / Math.PI
        if (!e.shiftKey) a = Math.round(a / 15) * 15
        a = ((a % 360) + 360) % 360
        setD(cur => ({ ...cur, items: cur.items.map(i => i.id === g.id ? { ...i, rot: a } : i) }))
        return
      }
      case 'resize': {
        const it = d.items.find(i => i.id === g.id)
        if (!it) return
        const local = rotate(sub(w, g.opp), -(it.rot || 0))
        const nw = Math.max(5, snap(Math.abs(local.x), e.shiftKey ? 1 : Math.min(step, 5) || 1)), nd = Math.max(5, snap(Math.abs(local.y), e.shiftKey ? 1 : Math.min(step, 5) || 1))
        const sx = Math.sign(local.x) || 1, sy = Math.sign(local.y) || 1
        const c = add(g.opp, rotate({ x: sx * nw / 2, y: sy * nd / 2 }, it.rot || 0))
        setD(cur => ({ ...cur, items: cur.items.map(i => i.id === g.id ? { ...i, w: nw, d: nd, x: c.x, y: c.y } : i) }))
        return
      }
      case 'room': {
        const dx = snap(w.x - g.start.x, step), dy = snap(w.y - g.start.y, step)
        if (!g.moved && Math.hypot(w.x - g.start.x, w.y - g.start.y) * v.k < 3) return
        g.moved = true
        const ids = new Set(g.items.map(i => i.id))
        setD(cur => ({ ...cur,
          rooms: cur.rooms.map(r => r.id === g.id ? { ...g.orig, points: g.orig.points.map(p => ({ x: p.x + dx, y: p.y + dy })) } : r),
          items: cur.items.map(i => ids.has(i.id) ? { ...i, x: g.items.find(o => o.id === i.id)!.x + dx, y: g.items.find(o => o.id === i.id)!.y + dy } : i) }))
        return
      }
      case 'vertex': {
        let p = sp(w)
        // stick to other rooms' corners
        for (const r of d.rooms) for (const q of r.points) if (r.id !== g.id && dist(q, p) * v.k < 10) p = q
        setD(cur => ({ ...cur, rooms: cur.rooms.map(r => r.id === g.id ? { ...r, points: r.points.map((q, i) => i === g.i ? p : q) } : r) }))
        return
      }
      case 'edge': {
        const ed = edge(g.orig, g.i)
        const off = snap(dot(sub(w, g.start), ed.out), step)
        const n = g.orig.points.length
        setD(cur => ({ ...cur, rooms: cur.rooms.map(r => r.id === g.id ? { ...r, points: g.orig.points.map((q, i) => (i === g.i || i === (g.i + 1) % n) ? add(q, mul(ed.out, off)) : q) } : r) }))
        return
      }
      case 'opening': {
        const o = d.openings.find(x => x.id === g.id)
        if (!o) return
        const ne = nearestEdge(w, 60)
        if (!ne) return
        const el = edge(ne.room, ne.i).len
        if (el < o.width + 2) return
        const at = Math.max(o.width / 2, Math.min(el - o.width / 2, snap(ne.at, step)))
        setD(cur => ({ ...cur, openings: cur.openings.map(x => x.id === g.id ? { ...x, room: ne.room.id, edge: ne.i, at, wall: undefined } : x) }))
        return
      }
      case 'thing': {
        const dx = snap(w.x - g.start.x, step), dy = snap(w.y - g.start.y, step)
        g.moved = true
        const mv = (p: Pt) => ({ x: p.x + dx, y: p.y + dy })
        setD(cur => {
          if (g.kind === 'dim') { const o = g.orig as Drawing['dims'][number]; return { ...cur, dims: cur.dims.map(m => m.id === g.id ? { ...o, a: mv(o.a), b: mv(o.b) } : m) } }
          if (g.kind === 'label') { const o = g.orig as Drawing['labels'][number]; return { ...cur, labels: cur.labels.map(l => l.id === g.id ? { ...o, at: mv(o.at) } : l) } }
          const o = g.orig as Drawing['walls'][number]; return { ...cur, walls: cur.walls.map(x => x.id === g.id ? { ...o, a: mv(o.a), b: mv(o.b) } : x) }
        })
        return
      }
      case 'end': {
        const p = sp(w)
        setD(cur => g.kind === 'dimend'
          ? { ...cur, dims: cur.dims.map(m => m.id === g.id ? (g.i ? { ...m, b: p } : { ...m, a: p }) : m) }
          : { ...cur, walls: cur.walls.map(x => x.id === g.id ? (g.i ? { ...x, b: p } : { ...x, a: p }) : x) })
        return
      }
      case 'underlay': onFloor({ underlay: { ...floor.underlay, x: g.ox + w.x - g.start.x, y: g.oy + w.y - g.start.y } }); return
    }
  }

  const onPointerUp = (e: React.PointerEvent<SVGSVGElement>) => {
    pointers.current.delete(e.pointerId)
    if (pointers.current.size < 2) pinch.current = null
    const g = drag.current
    drag.current = null
    if (!g) return
    if (g.t === 'draw') {
      const a = g.start, b = g.cur
      if (tool === 'room') {
        setDraft(null)
        const w = Math.abs(b.x - a.x), h = Math.abs(b.y - a.y)
        if (w < 40 || h < 40) { toast('Drag across the screen to draw a room'); return }
        const n = d.rooms.length + 1
        const r = rectRoom(`Room ${n}`, 'other', Math.min(a.x, b.x), Math.min(a.y, b.y), w, h, 'wood', wallCm)
        commit({ ...d, rooms: [...d.rooms, r] }); setSel([{ kind: 'room', id: r.id }]); setTool('select'); if (mobile) setPanel(true)
      } else if (tool === 'measure') {
        setDraft(null)
        if (dist(a, b) < 5) return
        const m = { id: uid('m'), a, b, off: 0 }
        commit({ ...d, dims: [...d.dims, m] }); setSel([{ kind: 'dim', id: m.id }])
      } else if (tool === 'scale') {
        if (dist(a, b) * v.k < 10) { setScaleDraft(null); return }
        setScaleDraft({ a, b }); if (mobile) setPanel(true)
      }
      return
    }
    if (before.current && g.t !== 'pan' && g.t !== 'underlay') {
      const b0 = before.current
      before.current = null
      setD(cur => {
        if (JSON.stringify(cur) !== JSON.stringify(b0)) { setPast(p => [...p.slice(-MAX_HISTORY), b0]); setFuture([]) }
        return cur
      })
    }
  }

  const finishShape = (pts: Pt[]) => {
    setDraft(null)
    if (pts.length < 3) return
    const r: DRoom = { id: uid('r'), name: `Room ${d.rooms.length + 1}`, kind: 'other', floor: 'wood', wall: wallCm, points: pts }
    commit({ ...d, rooms: [...d.rooms, r] }); setSel([{ kind: 'room', id: r.id }]); setTool('select')
  }

  const onWheel = useCallback((e: WheelEvent) => {
    e.preventDefault()
    const r = svgRef.current!.getBoundingClientRect()
    if (e.ctrlKey || Math.abs(e.deltaY) > 0) {
      const f = Math.exp(-e.deltaY * (e.ctrlKey ? 0.01 : 0.0016))
      setView(cur => {
        if (!cur) return cur
        const k = Math.max(0.05, Math.min(12, cur.k * f))
        const sx = e.clientX - r.left, sy = e.clientY - r.top
        return { k, tx: sx - (sx - cur.tx) * k / cur.k, ty: sy - (sy - cur.ty) * k / cur.k }
      })
    }
  }, [])
  useEffect(() => {
    const el = svgRef.current
    if (!el) return
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [onWheel])

  // ---- keys ----
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement
      if (t.closest('input, textarea, [contenteditable="true"]') || modalOpen() || document.querySelector('.menu, .popover, .sel-pop')) return
      const mod = e.metaKey || e.ctrlKey
      if (mod && e.key.toLowerCase() === 'z') { e.preventDefault(); e.shiftKey ? redo() : undo(); return }
      if (mod && e.key.toLowerCase() === 'y') { e.preventDefault(); redo(); return }
      if (mod && e.key.toLowerCase() === 'd') { e.preventDefault(); duplicateSel(); return }
      if (mod || e.altKey || readOnly) return
      const by = e.shiftKey ? 10 : 1
      switch (e.key) {
        case 'Delete': case 'Backspace': e.preventDefault(); removeSel(); break
        case 'Escape': if (draft && tool === 'shape' && draft.pts.length >= 3) finishShape(draft.pts); else { setDraft(null); setTool('select'); setSel([]); setScaleDraft(null) } break
        case 'Enter': if (draft && tool === 'shape') finishShape(draft.pts); if (tool === 'wall') { setDraft(null); setTool('select') } break
        case 'ArrowLeft': e.preventDefault(); nudge(-by, 0); break
        case 'ArrowRight': e.preventDefault(); nudge(by, 0); break
        case 'ArrowUp': e.preventDefault(); nudge(0, -by); break
        case 'ArrowDown': e.preventDefault(); nudge(0, by); break
        case 'r': rotateSel(e.shiftKey ? -90 : 90); break
        case 'v': setTool('select'); break
        case 'o': setTool('room'); break
        case 'w': setTool('wall'); break
        case 'm': setTool('measure'); break
        case 'f': setAdding(true); break
        case '0': fit(); break
        case '+': case '=': zoomAt(1.25); break
        case '-': zoomAt(0.8); break
      }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  })

  // ---- the grid ----
  const grid = useMemo(() => {
    if (!view) return null
    const x0 = -v.tx / v.k, y0 = -v.ty / v.k, x1 = (W - v.tx) / v.k, y1 = (H - v.ty) / v.k
    const g = v.k * 10 >= 8 ? 10 : v.k * 50 >= 10 ? 50 : 100
    const lines: ReactElement[] = []
    for (let x = Math.floor(x0 / g) * g; x <= x1 && lines.length < 400; x += g) lines.push(<line key={'x' + x} x1={x} y1={y0} x2={x} y2={y1} className={x % 100 === 0 ? 'gl major' : 'gl'} />)
    for (let y = Math.floor(y0 / g) * g; y <= y1 && lines.length < 800; y += g) lines.push(<line key={'y' + y} x1={x0} y1={y} x2={x1} y2={y} className={y % 100 === 0 ? 'gl major' : 'gl'} />)
    return lines
  }, [view, W, H]) // eslint-disable-line react-hooks/exhaustive-deps

  const u = floor.underlay || {}
  const underlay = floor.underlay_photo_id && u.on !== false && u.w_px ? {
    href: `${V1}/photos/${floor.underlay_photo_id}`, x: u.x || 0, y: u.y || 0, w: (u.w_px || 1000) * (u.cm_per_px || 1), h: (u.h_px || 1000) * (u.cm_per_px || 1), opacity: u.opacity ?? 0.5, rot: u.rot || 0,
  } : null

  const tools: { id: Tool; label: string; icon: string; key?: string }[] = [
    { id: 'select', label: 'Select and move', icon: 'move', key: 'V' },
    { id: 'room', label: 'Draw a room', icon: 'grid', key: 'O' },
    { id: 'shape', label: 'Room of any shape', icon: 'layers' },
    { id: 'door', label: 'Door', icon: 'door' },
    { id: 'window', label: 'Window', icon: 'sun' },
    { id: 'wall', label: 'Wall on its own', icon: 'ruler2', key: 'W' },
    { id: 'measure', label: 'Measure', icon: 'ruler', key: 'M' },
    { id: 'label', label: 'Note on the plan', icon: 'text' },
  ]
  const hint = {
    select: mobile ? 'Tap to choose · drag to move · two fingers to zoom' : 'Click to choose · drag to move · scroll to zoom · Alt-drag the picture underneath',
    room: 'Drag to draw a rectangle room', shape: 'Tap each corner; tap the first corner (or press Enter) to close it', door: 'Tap a wall to put a door there',
    window: 'Tap a wall to put a window there', wall: 'Tap where the wall starts and ends; Enter to stop', measure: 'Drag between two points', label: 'Tap where the note goes',
    scale: 'Drag along something on the picture whose length you know',
  }[tool]

  return (
    <div className={`ed ${panel ? 'with-panel' : ''}`}>
      <div className="ed-bar" role="toolbar" aria-label="Drawing tools">
        <div className="ed-tools">
          {tools.map(t => <button key={t.id} type="button" className={`btn ghost icon-only ed-tool ${tool === t.id ? 'on' : ''}`} disabled={readOnly && t.id !== 'select'} aria-pressed={tool === t.id}
            title={t.label + (t.key ? ` (${t.key})` : '')} aria-label={t.label} onClick={() => { setTool(t.id); setDraft(null); if (t.id !== 'select') setSel([]) }}><Icon name={t.icon} size={17} /></button>)}
          <span className="ed-sep" />
          <button type="button" className="btn sm primary ed-add" disabled={readOnly} onClick={() => setAdding(true)} title="Add furniture (F)"><Icon name="plus" size={14} strokeWidth={2.2} />{!mobile && 'Furniture'}</button>
          <span className="ed-sep" />
          <button type="button" className="btn ghost icon-only" disabled={!past.length || readOnly} onClick={undo} title="Undo (⌘Z)" aria-label="Undo"><Icon name="rotate-ccw" size={16} /></button>
          <button type="button" className="btn ghost icon-only" disabled={!future.length || readOnly} onClick={redo} title="Redo (⇧⌘Z)" aria-label="Redo"><Icon name="rotate-cw" size={16} /></button>
          <span className="ed-sep" />
          <button type="button" className="btn ghost icon-only" onClick={() => zoomAt(0.8)} title="Zoom out (−)" aria-label="Zoom out"><Icon name="minus" size={16} /></button>
          <button type="button" className="btn ghost icon-only" onClick={fit} title="Fit the plan (0)" aria-label="Fit the plan"><Icon name="maximize" size={15} /></button>
          <button type="button" className="btn ghost icon-only" onClick={() => zoomAt(1.25)} title="Zoom in (+)" aria-label="Zoom in"><Icon name="plus" size={16} /></button>
          <span className="ed-sep" />
          <button type="button" className={`btn sm ed-checks ${issues.length ? 'has' : ''} ${checksOpen ? 'on' : ''}`} onClick={() => { setChecksOpen(o => !o); if (mobile) setPanel(true) }} title="What the plan checks: overlaps, doors, room to walk">
            <Icon name={issues.length ? 'alert' : 'check-circle'} size={14} />{issues.length ? issues.length : (mobile ? '' : 'All clear')}
          </button>
        </div>
        <span className={`ed-save ${save}`} aria-live="polite">{readOnly ? 'View only' : save === 'saving' ? 'Saving…' : save === 'dirty' ? 'Editing' : save === 'error' ? 'Not saved' : 'Saved'}</span>
        <button type="button" className="btn ghost icon-only ed-panel-btn" onClick={() => setPanel(p => !p)} aria-label={panel ? 'Hide the details' : 'Show the details'} title="Details"><Icon name="sidebar" size={16} /></button>
      </div>
      <div className="ed-body">
        <div className={`ed-canvas tool-${tool}`} ref={ref}>
          <svg ref={svgRef} viewBox={`0 0 ${W} ${H}`} width={W} height={H} onPointerDown={onPointerDown} onPointerMove={onPointerMove} onPointerUp={onPointerUp} onPointerCancel={onPointerUp}
            onDoubleClick={() => { if (tool === 'shape' && draft) finishShape(draft.pts); if (tool === 'wall') { setDraft(null); setTool('select') } }} style={{ touchAction: 'none' }}>
            {view && <g transform={`translate(${v.tx} ${v.ty}) scale(${v.k})`}>
              <g className="ed-grid">{grid}</g>
              <PlanSvg d={d} k={v.k} pal={pal} sel={sel} issues={checksOpen || issues.length < 8 ? issueIDs : undefined} interactive showDims={settings?.show_dims && v.k > 0.55} underlay={underlay} />
              {!readOnly && <Handles d={d} sel={sel} k={v.k} pal={pal} />}
              {draft && tool === 'room' && draft.cur && <rect x={Math.min(draft.pts[0].x, draft.cur.x)} y={Math.min(draft.pts[0].y, draft.cur.y)} width={Math.abs(draft.cur.x - draft.pts[0].x)} height={Math.abs(draft.cur.y - draft.pts[0].y)} className="ed-draft" />}
              {draft && tool === 'room' && draft.cur && <>
                <Dim a={{ x: Math.min(draft.pts[0].x, draft.cur.x), y: Math.min(draft.pts[0].y, draft.cur.y) }} b={{ x: Math.max(draft.pts[0].x, draft.cur.x), y: Math.min(draft.pts[0].y, draft.cur.y) }} off={-18 / v.k} k={v.k} pal={pal} live />
                <Dim a={{ x: Math.max(draft.pts[0].x, draft.cur.x), y: Math.min(draft.pts[0].y, draft.cur.y) }} b={{ x: Math.max(draft.pts[0].x, draft.cur.x), y: Math.max(draft.pts[0].y, draft.cur.y) }} off={-18 / v.k} k={v.k} pal={pal} live />
              </>}
              {draft && (tool === 'shape' || tool === 'wall') && <>
                <polyline points={[...draft.pts, ...(draft.cur ? [draft.cur] : [])].map(p => `${p.x},${p.y}`).join(' ')} className="ed-draft-line" />
                {draft.pts.map((p, i) => <circle key={i} cx={p.x} cy={p.y} r={4 / v.k} className="ed-draft-pt" />)}
                {draft.cur && draft.pts.length > 0 && <Dim a={draft.pts[draft.pts.length - 1]} b={draft.cur} off={-14 / v.k} k={v.k} pal={pal} live />}
              </>}
              {draft && tool === 'measure' && draft.cur && <Dim a={draft.pts[0]} b={draft.cur} off={0} k={v.k} pal={pal} live />}
              {scaleDraft && <Dim a={scaleDraft.a} b={scaleDraft.b} off={0} k={v.k} pal={pal} live />}
            </g>}
          </svg>
          <div className="ed-hint" aria-hidden="true">{hint}</div>
          {mobile && !readOnly && <button type="button" className="ed-fab" onClick={() => setAdding(true)} aria-label="Add furniture"><Icon name="plus" size={22} strokeWidth={2.2} /></button>}
          {!d.rooms.length && !d.items.length && !readOnly && tool === 'select' && <div className="ed-empty">
            <p><b>An empty floor.</b> Draw your first room, or add one by its size in the details.</p>
            <button type="button" className="btn primary" onClick={() => setTool('room')}><Icon name="grid" size={15} />Draw a room</button>
          </div>}
        </div>
        {panel && <aside className="ed-panel" aria-label="Details">
          {checksOpen && <section className="ed-checks-list">
            <div className="ed-panel-head"><h3>Checks</h3><button type="button" className="btn ghost icon-only sm" onClick={() => setChecksOpen(false)} aria-label="Close the checks"><Icon name="x" size={15} /></button></div>
            {!issues.length && <p className="muted small">Nothing in the way: no overlaps, every door opens, and there is room around the furniture.</p>}
            {issues.map((is, i) => <button key={i} type="button" className={`ed-issue ${is.level}`} onClick={() => {
              const first = is.ids[0]; const it = d.items.find(x => x.id === first)
              if (it) { setSel([{ kind: 'item', id: it.id }]); setView({ k: Math.max(v.k, 1), tx: W / 2 - it.x * Math.max(v.k, 1), ty: H / 2 - it.y * Math.max(v.k, 1) }) }
            }}><Icon name={is.level === 'warn' ? 'alert' : 'info'} size={14} /><span>{is.text}</span></button>)}
            {settings?.clearance === false && <p className="muted small">Room around furniture is not checked (Settings → General).</p>}
          </section>}
          <Inspector d={d} sel={sel} setSel={setSel} commit={commit} readOnly={readOnly} floor={floor} onFloor={onFloor}
            moveContents={moveContents} setMoveContents={setMoveContents} wallMagnet={wallMagnet} setWallMagnet={setWallMagnet}
            scaleDraft={scaleDraft} setScaleDraft={setScaleDraft} setTool={setTool} layout={layout} issues={issues}
            onFocusRoom={id => { const r = d.rooms.find(x => x.id === id); if (r) { setSel([{ kind: 'room', id }]); const b = drawingBounds({ ...d, rooms: [r], items: [], walls: [], dims: [], labels: [] }, 80); const k = Math.min(W / b.w, H / b.h); setView({ k, tx: (W - b.w * k) / 2 - b.x0 * k, ty: (H - b.h * k) / 2 - b.y0 * k }) } }}
            onDelete={removeSel} onDuplicate={duplicateSel} onRotate={rotateSel} />
        </aside>}
      </div>
      {adding && <AddFurniture onClose={() => setAdding(false)} onPick={place} />}
    </div>
  )
}

export type { Underlay }
