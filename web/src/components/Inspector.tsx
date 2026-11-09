// The details of what is chosen on the plan: sizes, names, which way a door
// opens; with nothing chosen, the layout's rooms and the picture to trace.
import { useRef, useState } from 'react'
import type { DItem, DOpening, DRoom, Drawing, Floor, Layout, OpeningKind, Pt, Underlay } from '../lib/types'
import { add, dist, edge, mul, openingGeom, perimeter, polyArea, rectRoom, rectSize, wallOf, uid, type Issue, bbox } from '../lib/geom'
import { CATEGORIES, FLOORS, KINDS, OPENING_KINDS, ROOM_KINDS, SWATCHES, floorOf, kindOf } from '../lib/catalog'
import { area, len, money, parseMoney } from '../lib/units'
import { useApp } from '../lib/app-context'
import { Select } from './Select'
import { LenInput, NumInput } from './LenInput'
import { Icon } from './icons'
import { Segmented, Switch, errMsg, useToast } from './ui'
import type { Sel } from './PlanSvg'
import type { Tool } from './Editor'
import { shrink, uploadPhoto } from '../lib/photo'
import { useNavigate } from 'react-router-dom'

interface Props {
  d: Drawing; sel: Sel[]; setSel: (s: Sel[]) => void; commit: (d: Drawing) => void; readOnly?: boolean
  floor: Floor; onFloor: (f: Partial<Floor>) => void; layout: Layout; issues: Issue[]
  moveContents: boolean; setMoveContents: (b: boolean) => void; wallMagnet: boolean; setWallMagnet: (b: boolean) => void
  scaleDraft: { a: Pt; b: Pt } | null; setScaleDraft: (s: { a: Pt; b: Pt } | null) => void; setTool: (t: Tool) => void
  onFocusRoom: (id: string) => void; onDelete: () => void; onDuplicate: () => void; onRotate: (deg: number) => void
}

export function Inspector(p: Props) {
  const s = p.sel[0]
  if (p.scaleDraft) return <ScalePanel {...p} />
  if (!s) return <LayoutPanel {...p} />
  if (s.kind === 'item') { const it = p.d.items.find(i => i.id === s.id); return it ? <ItemPanel {...p} it={it} /> : null }
  if (s.kind === 'room') { const r = p.d.rooms.find(i => i.id === s.id); return r ? <RoomPanel {...p} r={r} /> : null }
  if (s.kind === 'opening') { const o = p.d.openings.find(i => i.id === s.id); return o ? <OpeningPanel {...p} o={o} /> : null }
  if (s.kind === 'label') {
    const l = p.d.labels.find(i => i.id === s.id)
    if (!l) return null
    return <div className="insp">
      <Head title="Note" icon="text" onClose={() => p.setSel([])} />
      <label className="insp-f"><span>Text</span><input className="input" autoFocus value={l.text} disabled={p.readOnly} onChange={e => p.commit({ ...p.d, labels: p.d.labels.map(x => x.id === l.id ? { ...x, text: e.target.value } : x) })} /></label>
      <div className="insp-f"><span>Size</span><Segmented value={String(l.size || 13)} onChange={v => p.commit({ ...p.d, labels: p.d.labels.map(x => x.id === l.id ? { ...x, size: Number(v) } : x) })} options={[{ value: '11', label: 'Small' }, { value: '13', label: 'Normal' }, { value: '18', label: 'Big' }]} /></div>
      <Actions {...p} />
    </div>
  }
  if (s.kind === 'dim') {
    const m = p.d.dims.find(i => i.id === s.id)
    if (!m) return null
    return <div className="insp">
      <Head title="Measurement" icon="ruler" onClose={() => p.setSel([])} />
      <p className="insp-big">{len(dist(m.a, m.b))}</p>
      <p className="muted small">Drag its ends to measure something else.</p>
      <Actions {...p} />
    </div>
  }
  if (s.kind === 'wall') {
    const w = p.d.walls.find(i => i.id === s.id)
    if (!w) return null
    const L = dist(w.a, w.b)
    return <div className="insp">
      <Head title="Wall" icon="ruler2" onClose={() => p.setSel([])} />
      <div className="insp-grid">
        <label className="insp-f"><span>Length</span><LenInput value={L} min={5} onChange={v => { const u = mul({ x: w.b.x - w.a.x, y: w.b.y - w.a.y }, 1 / (L || 1)); p.commit({ ...p.d, walls: p.d.walls.map(x => x.id === w.id ? { ...x, b: add(w.a, mul(u, v)) } : x) }) }} /></label>
        <label className="insp-f"><span>Thickness</span><LenInput value={w.thick ?? 12} min={1} max={100} onChange={v => p.commit({ ...p.d, walls: p.d.walls.map(x => x.id === w.id ? { ...x, thick: v } : x) })} /></label>
      </div>
      <Actions {...p} />
    </div>
  }
  return null
}

function Head({ title, icon, onClose, sub }: { title: string; icon: string; onClose: () => void; sub?: string }) {
  return <div className="ed-panel-head"><span className="insp-ico"><Icon name={icon} size={15} /></span><h3 className="grow ellipsis">{title}{sub && <small>{sub}</small>}</h3><button type="button" className="btn ghost icon-only sm" onClick={onClose} aria-label="Done"><Icon name="x" size={15} /></button></div>
}

function Actions({ onDelete, onDuplicate, readOnly, sel }: Props) {
  if (readOnly) return null
  const can = sel[0]?.kind === 'item' || sel[0]?.kind === 'room'
  return <div className="insp-actions">
    {can && <button type="button" className="btn sm" onClick={onDuplicate}><Icon name="copy" size={14} />Duplicate</button>}
    <button type="button" className="btn sm danger-text" onClick={onDelete}><Icon name="trash" size={14} />Remove</button>
  </div>
}

// ---- a piece of furniture ----

function ItemPanel(p: Props & { it: DItem }) {
  const { it, d, commit, readOnly } = p
  const { settings } = useApp()
  const navigate = useNavigate()
  const set = (patch: Partial<DItem>) => commit({ ...d, items: d.items.map(i => i.id === it.id ? { ...i, ...patch } : i) })
  const k = kindOf(it.type)
  const room = d.rooms.find(r => inside(r.points, it))
  const myIssues = p.issues.filter(i => i.ids.includes(it.id))
  const [priceText, setPriceText] = useState(it.price ? (it.price / 100).toFixed(2) : '')
  const dis = readOnly || it.locked
  return <div className="insp">
    <Head title={it.name || k.name} icon="package" onClose={() => p.setSel([])} sub={room ? ` · ${room.name}` : ''} />
    <label className="insp-f"><span>Name</span><input className="input" value={it.name} disabled={readOnly} onChange={e => set({ name: e.target.value })} /></label>
    <div className="insp-f"><span>Looks like</span><Select value={it.type} disabled={dis} ariaLabel="Kind of piece" onChange={v => set({ type: v })}
      options={KINDS.map(x => ({ value: x.type, label: x.name, group: CATEGORIES.find(c => c.id === x.cat)?.name }))} /></div>
    <div className="insp-grid three">
      <label className="insp-f"><span>Width</span><LenInput value={it.w} min={2} disabled={dis} onChange={v => set({ w: v })} /></label>
      <label className="insp-f"><span>Depth</span><LenInput value={it.d} min={2} disabled={dis} onChange={v => set({ d: v })} /></label>
      <label className="insp-f"><span>Height</span><LenInput value={it.h ?? k.h} min={0} disabled={dis} onChange={v => set({ h: v })} /></label>
    </div>
    <div className="insp-f"><span>Turned</span>
      <div className="insp-row">
        <button type="button" className="btn sm" disabled={dis} onClick={() => p.onRotate(-90)} aria-label="Turn left"><Icon name="rotate-ccw" size={14} />90°</button>
        <NumInput value={Math.round(it.rot || 0)} min={0} max={359} suffix="°" label="Angle" onChange={v => set({ rot: ((v % 360) + 360) % 360 })} />
        <button type="button" className="btn sm" disabled={dis} onClick={() => p.onRotate(90)} aria-label="Turn right">90°<Icon name="rotate-cw" size={14} /></button>
      </div>
    </div>
    <details className="insp-more">
      <summary>Position and height off the floor</summary>
      <div className="insp-grid three">
        <label className="insp-f"><span>Across</span><LenInput value={it.x} min={-1e6} disabled={dis} onChange={v => set({ x: v })} /></label>
        <label className="insp-f"><span>Down</span><LenInput value={it.y} min={-1e6} disabled={dis} onChange={v => set({ y: v })} /></label>
        <label className="insp-f"><span>Off the floor</span><LenInput value={it.elev || 0} min={0} disabled={dis} onChange={v => set({ elev: v })} /></label>
      </div>
    </details>
    <div className="insp-f"><span>Colour</span><div className="swatches">{SWATCHES.map(c => <button key={c || 'none'} type="button" disabled={readOnly} className={`swatch ${(it.color || '') === c ? 'on' : ''}`} style={{ background: c || k.color }} onClick={() => set({ color: c || undefined })} aria-label={c ? `Colour ${c}` : 'The usual colour'} title={c ? c : 'The usual colour'} />)}</div></div>
    <div className="insp-sw"><Switch checked={!!it.buy} disabled={readOnly} onChange={v => set({ buy: v || undefined })} label="Still to buy" /><span>Still to buy</span>
      {it.buy && <span className="len-input sm"><input className="input" inputMode="decimal" value={priceText} placeholder="Price" disabled={readOnly} onChange={e => setPriceText(e.target.value)} onBlur={() => set({ price: parseMoney(priceText) || undefined })} /><span className="len-unit">{settings?.currency || 'EUR'}</span></span>}</div>
    <div className="insp-sw"><Switch checked={!!it.locked} disabled={readOnly} onChange={v => set({ locked: v || undefined })} label="Fixed in place" /><span>Fixed in place <small className="muted">(built in: can't be moved by accident)</small></span></div>
    {it.piece_id ? <p className="small muted insp-link"><Icon name="tag" size={13} />One of your measured pieces · <a href={`/furniture/${it.piece_id}`} onClick={e => { e.preventDefault(); navigate(`/furniture/${it.piece_id}`) }}>open it</a></p> : null}
    {it.from && <p className="small muted insp-link"><Icon name="link" size={13} />From {it.from.startsWith('home:') ? 'Pocket Home' : it.from.startsWith('inventory:') ? 'Pocket Inventory' : 'Pocket Wishlist'}</p>}
    {myIssues.length > 0 && <ul className="insp-issues">{myIssues.map((i, n) => <li key={n} className={i.level}><Icon name={i.level === 'warn' ? 'alert' : 'info'} size={13} />{i.text}</li>)}</ul>}
    <div className="insp-actions">
      <button type="button" className="btn sm" onClick={() => navigate(`/fit?w=${Math.round(it.w)}&d=${Math.round(it.d)}&h=${Math.round(it.h ?? k.h)}&name=${encodeURIComponent(it.name || k.name)}&plan=${p.layout.plan_id}`)}><Icon name="door" size={14} />Will it fit through?</button>
    </div>
    <Actions {...p} />
    {!readOnly && <div className="insp-sw"><Switch checked={p.wallMagnet} onChange={p.setWallMagnet} label="Stick to walls" /><span>Stick to walls when moving <small className="muted">(hold Alt to place freely)</small></span></div>}
  </div>
}

function inside(pts: Pt[], q: Pt) {
  let c = false
  for (let i = 0, j = pts.length - 1; i < pts.length; j = i++) {
    const a = pts[i], b = pts[j]
    if ((a.y > q.y) !== (b.y > q.y) && q.x < (b.x - a.x) * (q.y - a.y) / (b.y - a.y) + a.x) c = !c
  }
  return c
}

// ---- a room ----

function RoomPanel(p: Props & { r: DRoom }) {
  const { r, d, commit, readOnly, floor } = p
  const set = (patch: Partial<DRoom>) => commit({ ...d, rooms: d.rooms.map(x => x.id === r.id ? { ...x, ...patch } : x) })
  const rs = rectSize(r)
  const a = polyArea(r.points)
  const setSize = (w: number, dd: number) => {
    if (!rs) return
    const e0 = edge(r, 0), e1 = edge(r, 1)
    const p0 = r.points[0]
    const pts = [p0, add(p0, mul(e0.u, w)), add(add(p0, mul(e0.u, w)), mul(e1.u, dd)), add(p0, mul(e1.u, dd))]
    commit({ ...d, rooms: d.rooms.map(x => x.id === r.id ? { ...x, points: pts } : x),
      openings: d.openings.map(o => o.room === r.id ? { ...o, at: Math.min(o.at, (o.edge % 2 === 0 ? w : dd) - o.width / 2) } : o) })
  }
  const setEdgeLen = (i: number, L: number) => {
    const e = edge(r, i)
    const delta = mul(e.u, L - e.len)
    // move every corner after this wall; the last wall closes the shape
    const pts = r.points.map((q, j) => j > i ? add(q, delta) : q)
    commit({ ...d, rooms: d.rooms.map(x => x.id === r.id ? { ...x, points: pts } : x) })
  }
  const ceiling = r.height || floor.ceiling_cm
  const wall = (() => { let w = 0; for (let i = 0; i < r.points.length; i++) if (!(r.open || []).includes(i)) w += edge(r, i).len; return w * ceiling })()
  const ops = d.openings.filter(o => o.room === r.id)
  return <div className="insp">
    <Head title={r.name || 'Room'} icon="grid" onClose={() => p.setSel([])} />
    <div className="insp-stats"><span><b>{area(a)}</b> floor</span><span><b>{len(perimeter(r.points))}</b> around</span><span><b>{area(Math.max(0, wall - ops.reduce((s, o) => s + o.width * (o.height || (o.kind === 'window' ? 140 : 205)), 0)))}</b> walls</span></div>
    <label className="insp-f"><span>Name</span><input className="input" value={r.name} disabled={readOnly} onChange={e => set({ name: e.target.value })} /></label>
    <div className="insp-grid">
      <div className="insp-f"><span>Room</span><Select value={r.kind || 'other'} disabled={readOnly} ariaLabel="What room" onChange={v => { const k = ROOM_KINDS.find(x => x.id === v); set({ kind: v, ...(!r.floor && k ? { floor: k.floor } : {}) }) }} options={ROOM_KINDS.map(k => ({ value: k.id, label: k.name }))} /></div>
      <div className="insp-f"><span>Floor</span><Select value={r.floor || ''} disabled={readOnly} ariaLabel="Floor" onChange={v => set({ floor: v })} options={FLOORS.map(f => ({ value: f.id, label: f.name }))} /></div>
    </div>
    {rs ? <div className="insp-grid">
      <label className="insp-f"><span>Width</span><LenInput value={rs.w} min={20} disabled={readOnly} onChange={v => setSize(v, rs.d)} /></label>
      <label className="insp-f"><span>Depth</span><LenInput value={rs.d} min={20} disabled={readOnly} onChange={v => setSize(rs.w, v)} /></label>
    </div> : <details className="insp-more" open>
      <summary>Wall lengths</summary>
      {r.points.map((_, i) => i < r.points.length - 1 && <label key={i} className="insp-f row"><span>Wall {i + 1}</span><LenInput value={edge(r, i).len} min={5} disabled={readOnly} onChange={v => setEdgeLen(i, v)} /></label>)}
      <p className="muted small">The last wall closes the shape. Drag a corner to change it on the plan.</p>
    </details>}
    <div className="insp-grid">
      <label className="insp-f"><span>Walls</span><LenInput value={wallOf(r)} min={0} max={100} disabled={readOnly} onChange={v => set({ wall: v })} /></label>
      <label className="insp-f"><span>Ceiling</span><LenInput value={ceiling} min={150} max={1000} disabled={readOnly} onChange={v => set({ height: Math.abs(v - floor.ceiling_cm) < 0.5 ? undefined : v })} /></label>
    </div>
    <details className="insp-more">
      <summary>Open sides (no wall)</summary>
      <p className="muted small">For an open kitchen or a balcony railing: the side is drawn without a wall.</p>
      <div className="chips">{r.points.map((_, i) => { const on = (r.open || []).includes(i); return <button key={i} type="button" disabled={readOnly} className={`chip ${on ? 'on' : ''}`} onClick={() => set({ open: on ? (r.open || []).filter(x => x !== i) : [...(r.open || []), i] })}>{sideName(r, i)} · {len(edge(r, i).len)}</button> })}</div>
    </details>
    <div className="insp-f"><span>Doors and windows</span>
      {ops.length ? <div className="insp-ops">{ops.map(o => <button key={o.id} type="button" className="insp-op" onClick={() => p.setSel([{ kind: 'opening', id: o.id }])}><Icon name={o.kind === 'window' ? 'sun' : 'door'} size={13} />{OPENING_KINDS.find(k => k.id === o.kind)?.name} · {len(o.width)} · {sideName(r, o.edge)}</button>)}</div> : <p className="muted small">None yet: pick Door or Window above and tap a wall.</p>}
    </div>
    <div className="insp-f"><span>Colour on the plan</span><div className="swatches">{['', '#efe3d2', '#e6ebef', '#e7e3dc', '#ebe6ee', '#dfe9d3', '#f3e5d8', '#e2e8f2'].map(c => <button key={c || 'none'} type="button" disabled={readOnly} className={`swatch ${(r.color || '') === c ? 'on' : ''}`} style={{ background: c || floorOf(r.floor).light }} onClick={() => set({ color: c || undefined })} aria-label={c || 'By the floor'} />)}</div></div>
    {!readOnly && <div className="insp-sw"><Switch checked={p.moveContents} onChange={p.setMoveContents} label="Move what's inside" /><span>Move the furniture inside with the room</span></div>}
    <Actions {...p} />
  </div>
}

function sideName(r: DRoom, i: number) {
  const e = edge(r, i)
  if (Math.abs(e.out.y) > Math.abs(e.out.x)) return e.out.y < 0 ? 'Top' : 'Bottom'
  return e.out.x < 0 ? 'Left' : 'Right'
}

// ---- a door or a window ----

function OpeningPanel(p: Props & { o: DOpening }) {
  const { o, d, commit, readOnly } = p
  const set = (patch: Partial<DOpening>) => commit({ ...d, openings: d.openings.map(x => x.id === o.id ? { ...x, ...patch } : x) })
  const g = openingGeom(d, o)
  const isWin = o.kind === 'window'
  const swing = o.kind === 'door' || o.kind === 'double'
  return <div className="insp">
    <Head title={o.name || OPENING_KINDS.find(k => k.id === o.kind)?.name || 'Door'} icon={isWin ? 'sun' : 'door'} onClose={() => p.setSel([])} sub={g?.room ? ` · ${g.room.name}` : ''} />
    <div className="insp-f"><span>Kind</span><Select value={o.kind} disabled={readOnly} ariaLabel="Kind" onChange={v => set({ kind: v as OpeningKind, ...(v === 'window' ? { sill: o.sill ?? 90, height: 140 } : { height: o.kind === 'window' ? 205 : o.height }) })} options={OPENING_KINDS.map(k => ({ value: k.id, label: k.name }))} /></div>
    <label className="insp-f"><span>Name <small className="muted">(for the way in)</small></span><input className="input" value={o.name || ''} placeholder={isWin ? 'Window' : 'Front door, bedroom door…'} disabled={readOnly} onChange={e => set({ name: e.target.value || undefined })} /></label>
    <div className="insp-grid three">
      <label className="insp-f"><span>Width</span><LenInput value={o.width} min={30} max={g ? g.len : 2000} disabled={readOnly} onChange={v => set({ width: v, at: g ? Math.max(v / 2, Math.min(g.len - v / 2, o.at)) : o.at })} /></label>
      <label className="insp-f"><span>Height</span><LenInput value={o.height || (isWin ? 140 : 205)} min={20} disabled={readOnly} onChange={v => set({ height: v })} /></label>
      {isWin ? <label className="insp-f"><span>Sill</span><LenInput value={o.sill ?? 90} min={0} disabled={readOnly} onChange={v => set({ sill: v })} /></label>
        : <label className="insp-f"><span>From corner</span><LenInput value={Math.max(0, o.at - o.width / 2)} min={0} disabled={readOnly} onChange={v => g && set({ at: Math.max(o.width / 2, Math.min(g.len - o.width / 2, v + o.width / 2)) })} /></label>}
    </div>
    {swing && <>
      <div className="insp-f"><span>Opens</span><Segmented value={o.out ? 'out' : 'in'} onChange={v => set({ out: v === 'out' || undefined })} options={[{ value: 'in', label: 'Into the room' }, { value: 'out', label: 'Out of it' }]} /></div>
      {o.kind === 'door' && <div className="insp-f"><span>Hinges</span><Segmented value={o.hinge || 'start'} onChange={v => set({ hinge: v as 'start' | 'end' })} options={[{ value: 'start', label: 'This side' }, { value: 'end', label: 'Other side' }]} /></div>}
    </>}
    <p className="muted small">Drag it along the wall, or onto another wall.</p>
    <Actions {...p} />
  </div>
}

// ---- nothing chosen: the layout ----

function LayoutPanel(p: Props) {
  const { d, commit, readOnly, floor, onFloor } = p
  const { settings } = useApp()
  const { toast } = useToast()
  const [w, setW] = useState(400), [dd, setDd] = useState(350)
  const [name, setName] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const inside2 = d.rooms.filter(r => r.kind !== 'balcony' && r.kind !== 'garden')
  const total = inside2.reduce((s, r) => s + polyArea(r.points), 0)
  const toBuy = d.items.filter(i => i.buy)
  const addRoom = () => {
    // to the right of what is there, a wall apart
    let x = 0, y = 0
    if (d.rooms.length) { const b = bbox(d.rooms.flatMap(r => r.points)); x = b.x1 + (settings?.wall_cm || 12); y = b.y0 }
    const r = rectRoom(name.trim() || `Room ${d.rooms.length + 1}`, 'other', x, y, w, dd, 'wood', settings?.wall_cm || 12)
    commit({ ...d, rooms: [...d.rooms, r] }); p.setSel([{ kind: 'room', id: r.id }]); setName('')
  }
  const u: Underlay = floor.underlay || {}
  const pickPicture = async (f: File) => {
    setBusy(true)
    try {
      const s = await shrink(f, 2000)
      const up = await uploadPhoto(s, 'underlay')
      const b = d.rooms.length ? bbox(d.rooms.flatMap(r => r.points)) : { x0: 0, y0: 0, w: 1000 }
      const cmpx = (b.w || 1000) / (up.width || s.width)
      onFloor({ underlay_photo_id: up.id, underlay: { x: b.x0, y: b.y0, cm_per_px: cmpx, opacity: 0.5, on: true, w_px: up.width || s.width, h_px: up.height || s.height } })
      toast('Picture added under the plan. Now set its scale: measure something on it whose length you know.')
      p.setTool('scale')
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return <div className="insp">
    <div className="ed-panel-head"><span className="insp-ico"><Icon name="layers" size={15} /></span><h3 className="grow ellipsis">{p.layout.name}<small> · {floor.name}</small></h3></div>
    <div className="insp-stats"><span><b>{area(total)}</b> inside</span><span><b>{d.rooms.length}</b> rooms</span><span><b>{d.items.length}</b> pieces</span>{toBuy.length > 0 && <span><b>{money(toBuy.reduce((s, i) => s + (i.price || 0), 0), settings?.currency)}</b> to buy</span>}</div>
    {d.rooms.length > 0 && <div className="insp-rooms">{d.rooms.map(r => <button key={r.id} type="button" className="insp-room" onClick={() => p.onFocusRoom(r.id)}><span className="dot" style={{ background: r.color || floorOf(r.floor).light }} /><span className="grow ellipsis">{r.name}</span><span className="muted">{area(polyArea(r.points))}</span></button>)}</div>}
    {!readOnly && <section className="insp-sec">
      <h4>Add a room by its size</h4>
      <input className="input" value={name} placeholder="Name (Living room, Bedroom…)" onChange={e => setName(e.target.value)} />
      <div className="insp-grid"><label className="insp-f"><span>Width</span><LenInput value={w} min={40} onChange={setW} /></label><label className="insp-f"><span>Depth</span><LenInput value={dd} min={40} onChange={setDd} /></label></div>
      <button type="button" className="btn sm" onClick={addRoom}><Icon name="plus" size={14} />Add the room</button>
    </section>}
    {!readOnly && <section className="insp-sec">
      <h4>Trace over a floor plan picture</h4>
      {floor.underlay_photo_id ? <>
        <div className="insp-sw"><Switch checked={u.on !== false} onChange={v => onFloor({ underlay: { ...u, on: v } })} label="Show the picture" /><span>Show the picture under the plan</span></div>
        <label className="insp-f"><span>See-through</span><input type="range" min={0.1} max={1} step={0.05} value={u.opacity ?? 0.5} onChange={e => onFloor({ underlay: { ...u, opacity: Number(e.target.value) } })} /></label>
        <div className="insp-row wrap">
          <button type="button" className="btn sm" onClick={() => p.setTool('scale')}><Icon name="ruler" size={14} />Set the scale</button>
          <button type="button" className="btn sm" onClick={() => onFloor({ underlay: { ...u, rot: ((u.rot || 0) + 90) % 360 } })}><Icon name="rotate-cw" size={14} />Turn</button>
          <button type="button" className="btn sm danger-text" onClick={() => onFloor({ underlay_photo_id: null, underlay: {} })}><Icon name="trash" size={14} />Remove</button>
        </div>
        <p className="muted small">Alt-drag (or Shift-drag) on an empty spot to move the picture.</p>
      </> : <>
        <p className="muted small">An estate agent's plan, a photo of a printed plan or a sketch: put it under the drawing and draw the rooms over it.</p>
        <button type="button" className="btn sm" disabled={busy} onClick={() => fileRef.current?.click()}><Icon name="image" size={14} />{busy ? 'Adding…' : 'Choose a picture'}</button>
        <input ref={fileRef} type="file" accept="image/*" hidden onChange={e => { const f = e.target.files?.[0]; if (f) pickPicture(f); e.target.value = '' }} />
      </>}
    </section>}
    {d.labels.length + d.dims.length > 0 && <p className="muted small">{d.dims.length} {d.dims.length === 1 ? 'measurement' : 'measurements'}, {d.labels.length} {d.labels.length === 1 ? 'note' : 'notes'} on the plan.</p>}
  </div>
}

// ---- setting the picture's scale ----

function ScalePanel(p: Props) {
  const { scaleDraft, floor, onFloor } = p
  const [real, setReal] = useState(0)
  if (!scaleDraft) return null
  const measured = dist(scaleDraft.a, scaleDraft.b)
  const apply = () => {
    if (real <= 0 || measured <= 0) return
    const u = floor.underlay || {}
    const f = real / measured
    // grow the picture around the first point
    const x = scaleDraft.a.x - (scaleDraft.a.x - (u.x || 0)) * f, y = scaleDraft.a.y - (scaleDraft.a.y - (u.y || 0)) * f
    onFloor({ underlay: { ...u, cm_per_px: (u.cm_per_px || 1) * f, x, y } })
    p.setScaleDraft(null); p.setTool('select')
  }
  return <div className="insp">
    <Head title="Set the picture's scale" icon="ruler" onClose={() => { p.setScaleDraft(null); p.setTool('select') }} />
    <p className="small">You measured a line on the picture. How long is it really? (Look for a size written on the plan, or a door: about 80 cm.)</p>
    <label className="insp-f"><span>Real length</span><LenInput value={real} min={1} onChange={setReal} placeholder="e.g. 4.5 m" /></label>
    <div className="insp-actions"><button type="button" className="btn primary sm" disabled={real <= 0} onClick={apply}>Set the scale</button><button type="button" className="btn sm" onClick={() => p.setScaleDraft(null)}>Measure again</button></div>
  </div>
}

export const _keep = uid
