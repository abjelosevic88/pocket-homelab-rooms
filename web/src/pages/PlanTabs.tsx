// The plan page's other tabs: rooms and sizes, what to buy, the checklist,
// the way in, photos, details, comparing layouts, AI tips, sending to chat.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { Drawing, FitResult, Layout, LayoutSum, PlanDoc, PlanPhoto, Piece, Route, RouteStep, StepKind, Todo, Plan } from '../lib/types'
import { useApp } from '../lib/app-context'
import { useAppState, announceChange, openAction } from '../lib/data'
import { area, len, money, size } from '../lib/units'
import { diffItems, fixDrawing } from '../lib/geom'
import { FLOORS, kindOf, roomKindName } from '../lib/catalog'
import { Icon } from '../components/icons'
import { Card, CoreHint, Empty, Field, Loading, Modal, Segmented, Switch, errMsg, useToast } from '../components/ui'
import { Select } from '../components/Select'
import { LenInput } from '../components/LenInput'
import { PlanPreview } from '../components/Preview'
import { Markdown } from '../components/Markdown'
import { shrink, uploadPhoto } from '../lib/photo'
import { fmtDate, todayISO } from '../lib/format'

function useSummary(planId: number, layoutId: number) {
  const [s, setS] = useState<LayoutSum[] | null>(null)
  const load = useCallback(() => { api.get<{ floors: LayoutSum[] }>(`${V1}/plans/${planId}/summary?layout=${layoutId}`).then(r => setS(r.floors)).catch(() => setS([])) }, [planId, layoutId])
  useEffect(() => { load() }, [load])
  return [s, load] as const
}

// ---- rooms and sizes ----

export function Summary({ planId, layoutId, currency }: { planId: number; layoutId: number; currency: string }) {
  const [sums] = useSummary(planId, layoutId)
  if (!sums) return <Loading />
  const all = sums.reduce((a, f) => a + f.area_cm2, 0), out = sums.reduce((a, f) => a + f.outside_cm2, 0)
  const paint = sums.reduce((a, f) => a + f.wall_cm2, 0)
  return <div className="page tab-page">
    <div className="stat-row">
      <div className="stat"><span className="stat-v">{area(all)}</span><span className="stat-l">inside{out > 0 ? ` · ${area(out)} outside` : ''}</span></div>
      <div className="stat"><span className="stat-v">{area(paint)}</span><span className="stat-l">walls to paint</span></div>
      <div className="stat"><span className="stat-v">{len(sums.reduce((a, f) => a + f.skirting_cm, 0))}</span><span className="stat-l">skirting board</span></div>
      <div className="stat"><span className="stat-v">{sums.reduce((a, f) => a + f.rooms.length, 0)}</span><span className="stat-l">rooms</span></div>
    </div>
    {sums.map(f => <Card key={f.floor_id} title={`${f.floor_name} · ${f.layout}`} sub={`Ceiling ${len(f.ceiling_cm)}`} flush>
      <div className="table-wrap"><table className="tbl">
        <thead><tr><th>Room</th><th>Size</th><th className="num">Floor</th><th className="num">Walls to paint</th><th className="num col-sm">Daylight</th><th className="col-sm">Floor</th></tr></thead>
        <tbody>{f.rooms.map(r => <tr key={r.id}>
          <td><b>{r.name}</b><div className="muted small">{roomKindName(r.kind)} · {r.doors} {r.doors === 1 ? 'door' : 'doors'}, {r.windows} {r.windows === 1 ? 'window' : 'windows'}</div></td>
          <td>{r.size || '—'}</td><td className="num">{area(r.area_cm2)}</td><td className="num">{area(r.wall_cm2)}</td>
          <td className="num col-sm" title="Window area as a share of the floor: about 10 % or more is bright">{r.windows ? `${r.daylight_pct} %` : '—'}</td>
          <td className="col-sm">{FLOORS.find(x => x.id === r.floor)?.name || '—'}</td>
        </tr>)}</tbody>
        <tfoot><tr><td>All</td><td /><td className="num">{area(f.area_cm2)}</td><td className="num">{area(f.wall_cm2)}</td><td className="col-sm" /><td className="col-sm" /></tr></tfoot>
      </table></div>
      {f.finishes.length > 0 && <div className="finishes">
        <h4>Floors to order <span className="muted small">with 10 % for cuts and waste</span></h4>
        <div className="finish-row">{f.finishes.map(x => <div key={x.floor} className="finish"><b>{FLOORS.find(fl => fl.id === x.floor)?.name || x.floor}</b><span>{area(x.order_cm2)}</span><span className="muted small">{x.room_list}</span></div>)}</div>
      </div>}
      <p className="muted small pad">Paint: about {Math.ceil(f.wall_cm2 / 10000 / 10)} l for one coat at 10 m² per litre ({Math.ceil(f.wall_cm2 / 10000 / 5)} l for two). {currency && ''}</p>
    </Card>)}
  </div>
}

// ---- what to buy ----

export function Shopping({ planId, layoutId, currency, onOpenItem }: { planId: number; layoutId: number; currency: string; onOpenItem: (id: string) => void }) {
  const [sums] = useSummary(planId, layoutId)
  const { toast } = useToast()
  const navigate = useNavigate()
  const appState = useAppState()
  const wish = appState('wishlist', 'wishes:write')
  if (!sums) return <Loading />
  const items = sums.flatMap(f => f.items.map(i => ({ ...i, floor: f.floor_name })))
  const buy = items.filter(i => i.buy), own = items.filter(i => !i.buy)
  const total = buy.reduce((a, i) => a + i.price, 0)
  const toWish = async (pieceId: number) => {
    try { const r = await api.post<{ text: string; url: string }>(`${V1}/pieces/${pieceId}/wish`); toast(r.text, 'ok', openAction(r.url)) } catch (e) { toast(errMsg(e), 'err') }
  }
  return <div className="page tab-page">
    <div className="stat-row">
      <div className="stat"><span className="stat-v">{buy.length}</span><span className="stat-l">to buy</span></div>
      <div className="stat"><span className="stat-v">{money(total, currency) || '—'}</span><span className="stat-l">altogether</span></div>
      <div className="stat"><span className="stat-v">{own.length}</span><span className="stat-l">you have</span></div>
    </div>
    <Card title="Still to buy" sub="Pieces marked “Still to buy” on this layout, and your wanted furniture placed on it" flush>
      {!buy.length ? <p className="muted pad">Nothing to buy on this layout. Mark a piece “Still to buy” in its details to add it here.</p> :
        <div className="rows">{buy.map(i => <div key={i.id} className="row">
          <span className="grow"><b>{i.name}</b><span className="muted small">{i.room || i.floor} · {i.size}</span></span>
          <span className="num">{money(i.price, currency) || <span className="muted">no price</span>}</span>
          <button type="button" className="btn ghost sm" onClick={() => onOpenItem(i.id)} title="Show it on the plan"><Icon name="map" size={14} /></button>
          {i.piece_id > 0 && <button type="button" className="btn ghost sm" disabled={!wish.ok} title={wish.ok ? 'Put it on your wishlist' : 'Needs Pocket Wishlist (Settings → Connected apps)'} onClick={() => toWish(i.piece_id)}><Icon name="gift" size={14} /></button>}
          {i.piece_id > 0 && <button type="button" className="btn ghost sm" onClick={() => navigate(`/furniture/${i.piece_id}`)} title="Open the piece"><Icon name="chevron-right" size={14} /></button>}
        </div>)}</div>}
    </Card>
    <Card title="Already yours" flush>
      {!own.length ? <p className="muted pad">No furniture on this layout yet.</p> :
        <div className="rows">{own.map(i => <div key={i.id} className="row"><span className="grow">{i.name}<span className="muted small">{i.room || i.floor} · {i.size}</span></span>
          <button type="button" className="btn ghost sm" onClick={() => onOpenItem(i.id)} title="Show it on the plan"><Icon name="map" size={14} /></button></div>)}</div>}
    </Card>
  </div>
}

// ---- the checklist ----

export function Checklist({ planId, onChange }: { planId: number; onChange: () => void }) {
  const [list, setList] = useState<Todo[] | null>(null)
  const [title, setTitle] = useState('')
  const [due, setDue] = useState('')
  const { toast } = useToast()
  const appState = useAppState()
  const tasks = appState('tasks', 'tasks:write')
  const load = useCallback(() => { api.get<Todo[]>(`${V1}/todos?plan_id=${planId}`).then(setList).catch(() => setList([])) }, [planId])
  useEffect(() => { load() }, [load])
  const add = async () => {
    if (!title.trim()) return
    try { await api.post(`${V1}/plans/${planId}/todos`, { title, due }); setTitle(''); setDue(''); load(); onChange() } catch (e) { toast(errMsg(e), 'err') }
  }
  const tick = async (t: Todo) => { setList(l => l?.map(x => x.id === t.id ? { ...x, done: !x.done } : x) || null); await api.put(`${V1}/todos/${t.id}`, { done: !t.done }).catch(() => {}); load(); onChange() }
  const send = async () => { try { const r = await api.post<{ text: string; url: string }>(`${V1}/plans/${planId}/tasks`); toast(r.text, 'ok', openAction(r.url)); load() } catch (e) { toast(errMsg(e), 'err') } }
  if (!list) return <Loading />
  const open = list.filter(t => !t.done), done = list.filter(t => t.done)
  return <div className="page tab-page narrow">
    <div className="todo-add">
      <input className="input grow" placeholder="Measure the alcove, book the movers…" value={title} onChange={e => setTitle(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') add() }} />
      <input className="input date" type="date" value={due} onChange={e => setDue(e.target.value)} aria-label="Due" />
      <button type="button" className="btn primary" onClick={add} disabled={!title.trim()}><Icon name="plus" size={14} />Add</button>
    </div>
    <div className="todo-tools">
      <button type="button" className="btn sm" disabled={!tasks.ok || !open.some(t => !t.task_id)} onClick={send} title={tasks.ok ? '' : 'Needs Pocket Tasks (Settings → Connected apps)'}><Icon name="check-square" size={14} />Send to Pocket Tasks</button>
      {!tasks.ok && <CoreHint />}
    </div>
    {!list.length && <Empty icon="check-square" title="Nothing on the checklist">What needs doing for this plan: measuring, buying, booking. You get a reminder the day before one is due.</Empty>}
    <div className="todos">{[...open, ...done].map(t => <div key={t.id} className={`todo ${t.done ? 'done' : ''}`}>
      <button type="button" className="todo-box" onClick={() => tick(t)} aria-label={t.done ? 'Not done' : 'Done'}><Icon name={t.done ? 'check-circle' : 'circle'} size={18} /></button>
      <span className="grow">{t.title}{t.due && <span className={`muted small ${!t.done && t.due < todayISO() ? 'late' : ''}`}> · {fmtDate(t.due, false)}</span>}</span>
      {t.task_url && <a className="btn ghost sm" href={t.task_url} target="_blank" rel="noreferrer" title="In Pocket Tasks"><Icon name="external" size={13} /></a>}
      <button type="button" className="btn ghost sm" aria-label="Remove" onClick={async () => { await api.del(`${V1}/todos/${t.id}`); load(); onChange() }}><Icon name="x" size={14} /></button>
    </div>)}</div>
  </div>
}

// ---- the way in ----

const STEP_KINDS: { id: StepKind; name: string; help: string }[] = [
  { id: 'door', name: 'Door', help: 'Clear width between the frames (door open), and height' },
  { id: 'hall', name: 'Hallway', help: 'Width wall to wall, straight' },
  { id: 'corner', name: 'Corner', help: 'A hallway that turns: the width you turn into, and the width you come from' },
  { id: 'stairs', name: 'Stairs', help: 'Width, the lowest headroom, and the landing width if they turn' },
  { id: 'lift', name: 'Lift', help: 'Inside width, depth and height' },
]

export function WayIn({ doc, drawing, onChange }: { doc: PlanDoc; drawing: Drawing; onChange: () => void }) {
  const { toast } = useToast()
  const [routes, setRoutes] = useState<Route[]>(doc.routes)
  const [pieces, setPieces] = useState<Piece[]>([])
  const [active, setActive] = useState<number | null>(doc.routes[0]?.id ?? null)
  useEffect(() => { setRoutes(doc.routes) }, [doc.routes])
  useEffect(() => { api.get<Piece[]>(`${V1}/pieces`).then(setPieces).catch(() => {}) }, [])
  const rt = routes.find(r => r.id === active)
  const save = async (r: Route) => {
    setRoutes(rs => rs.map(x => x.id === r.id ? r : x))
    try { await api.put(`${V1}/routes/${r.id}`, { name: r.name, steps: r.steps.filter(s => s.width > 0) }) } catch (e) { toast(errMsg(e), 'err') }
  }
  const create = async (fromPlan: boolean) => {
    const steps: RouteStep[] = fromPlan ? drawing.openings.filter(o => o.kind !== 'window').slice(0, 4).map(o => ({ kind: 'door', name: o.name || 'Door', width: Math.max(40, o.width - 6), height: (o.height || 205) - 3 })) : [{ kind: 'door', name: 'Front door', width: 86, height: 203 }]
    try { const r = await api.post<Route>(`${V1}/plans/${doc.plan.id}/routes`, { name: routes.length ? `Way in ${routes.length + 1}` : 'The way in', steps }); setRoutes(rs => [...rs, r]); setActive(r.id); onChange() } catch (e) { toast(errMsg(e), 'err') }
  }
  return <div className="page tab-page">
    <p className="muted">Every door, hallway, corner, stairs and lift a piece of furniture has to get through. Measure each once; then every piece you add is checked against them.</p>
    <div className="chips">
      {routes.map(r => <button key={r.id} type="button" className={`chip ${r.id === active ? 'on' : ''}`} onClick={() => setActive(r.id)}>{r.name}</button>)}
      <button type="button" className="chip add" onClick={() => create(false)}><Icon name="plus" size={13} />Way in</button>
      {drawing.openings.some(o => o.kind !== 'window') && !routes.length && <button type="button" className="chip add" onClick={() => create(true)}><Icon name="door" size={13} />From the doors on the plan</button>}
    </div>
    {!rt ? <Empty icon="door" title="No way in yet">Add one: start with the building door, then the stairs or lift, the front door, and the hallway.</Empty> : <div className="way-grid">
      <Card title={<input className="input title-input" value={rt.name} onChange={e => setRoutes(rs => rs.map(x => x.id === rt.id ? { ...x, name: e.target.value } : x))} onBlur={() => save(rt)} aria-label="Name" />} actions={<button type="button" className="btn ghost sm danger-text" onClick={async () => { await api.del(`${V1}/routes/${rt.id}`); setRoutes(rs => rs.filter(x => x.id !== rt.id)); setActive(null); onChange() }}><Icon name="trash" size={14} />Remove</button>}>
        <ol className="steps">{rt.steps.map((s, i) => <li key={i} className="step">
          <div className="step-head">
            <span className="step-n">{i + 1}</span>
            <Select value={s.kind} ariaLabel="What it is" onChange={v => save({ ...rt, steps: rt.steps.map((x, j) => j === i ? { ...x, kind: v as StepKind } : x) })} options={STEP_KINDS.map(k => ({ value: k.id, label: k.name }))} />
            <input className="input grow" value={s.name} placeholder="Name" onChange={e => setRoutes(rs => rs.map(x => x.id === rt.id ? { ...x, steps: x.steps.map((y, j) => j === i ? { ...y, name: e.target.value } : y) } : x))} onBlur={() => save(rt)} />
            <button type="button" className="btn ghost icon-only sm" disabled={i === 0} onClick={() => { const st = [...rt.steps]; [st[i - 1], st[i]] = [st[i], st[i - 1]]; save({ ...rt, steps: st }) }} aria-label="Earlier"><Icon name="arrow-up" size={14} /></button>
            <button type="button" className="btn ghost icon-only sm" onClick={() => save({ ...rt, steps: rt.steps.filter((_, j) => j !== i) })} aria-label="Remove the step"><Icon name="x" size={14} /></button>
          </div>
          <p className="muted small">{STEP_KINDS.find(k => k.id === s.kind)?.help}</p>
          <div className="step-fields">
            <label className="insp-f"><span>Width</span><LenInput value={s.width} min={20} onChange={v => save({ ...rt, steps: rt.steps.map((x, j) => j === i ? { ...x, width: v } : x) })} /></label>
            {s.kind !== 'hall' && s.kind !== 'corner' && <label className="insp-f"><span>{s.kind === 'stairs' ? 'Headroom' : 'Height'}</span><LenInput value={s.height || (s.kind === 'lift' ? 210 : 200)} min={50} onChange={v => save({ ...rt, steps: rt.steps.map((x, j) => j === i ? { ...x, height: v } : x) })} /></label>}
            {s.kind === 'lift' && <label className="insp-f"><span>Depth</span><LenInput value={s.depth || 130} min={40} onChange={v => save({ ...rt, steps: rt.steps.map((x, j) => j === i ? { ...x, depth: v } : x) })} /></label>}
            {s.kind !== 'lift' && s.kind !== 'hall' && <label className="insp-f"><span>{s.kind === 'corner' ? 'Coming from' : 'Turn from a hallway'}</span><LenInput value={s.turn || 0} min={0} onChange={v => save({ ...rt, steps: rt.steps.map((x, j) => j === i ? { ...x, turn: v || undefined } : x) })} placeholder="—" /></label>}
          </div>
        </li>)}</ol>
        <button type="button" className="btn sm" onClick={() => save({ ...rt, steps: [...rt.steps, { kind: 'door', name: '', width: 80, height: 200 }] })}><Icon name="plus" size={14} />Add a step</button>
      </Card>
      <Card title="Does it fit?" sub="Your measured furniture, with 2 cm for hands (Settings)" flush>
        {!pieces.length ? <p className="muted pad">Measure your furniture under Furniture to see it here.</p> : <FitList pieces={pieces} route={rt} />}
      </Card>
    </div>}
  </div>
}

function FitList({ pieces, route }: { pieces: Piece[]; route: Route }) {
  const [res, setRes] = useState<Record<number, FitResult>>({})
  const navigate = useNavigate()
  const key = JSON.stringify(route.steps)
  useEffect(() => {
    let live = true
    const steps = route.steps.filter(s => s.width > 0)
    Promise.all(pieces.map(p => api.post<{ result: FitResult }>(`${V1}/fit`, { piece_id: p.id, steps }).then(r => [p.id, r.result] as const).catch(() => null)))
      .then(all => { if (live) setRes(Object.fromEntries(all.filter(Boolean) as [number, FitResult][])) })
    return () => { live = false }
  }, [key, pieces]) // eslint-disable-line react-hooks/exhaustive-deps
  return <div className="rows">{pieces.map(p => {
    const r = res[p.id]
    return <button key={p.id} type="button" className="row fit-row" onClick={() => navigate(`/fit?piece=${p.id}&route=${route.id}`)}>
      <Icon name={!r ? 'circle' : r.ok ? 'check-circle' : 'x-circle'} size={16} className={r ? (r.ok ? 'ok' : 'bad') : ''} />
      <span className="grow"><b>{p.name}</b><span className="muted small">{size(p.width, p.depth, p.height)}{r && !r.ok ? ' · ' + r.summary : ''}</span></span>
      <Icon name="chevron-right" size={14} />
    </button>
  })}</div>
}

// ---- photos ----

export function Photos({ planId, rooms }: { planId: number; rooms: string[] }) {
  const [list, setList] = useState<PlanPhoto[] | null>(null)
  const [room, setRoom] = useState('')
  const [busy, setBusy] = useState(false)
  const [pick, setPick] = useState(false)
  const [big, setBig] = useState<PlanPhoto | null>(null)
  const { toast } = useToast()
  const appState = useAppState()
  const photos = appState('photos', 'photos:read')
  const load = useCallback(() => { api.get<PlanPhoto[]>(`${V1}/plans/${planId}/photos`).then(setList).catch(() => setList([])) }, [planId])
  useEffect(() => { load() }, [load])
  const upload = async (files: FileList) => {
    setBusy(true)
    try {
      for (const f of Array.from(files)) { const s = await shrink(f); const up = await uploadPhoto(s, 'room'); await api.post(`${V1}/plans/${planId}/photos`, { photo_id: up.id, room }) }
      load()
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  if (!list) return <Loading />
  const byRoom = [...new Set(list.map(p => p.room))]
  return <div className="page tab-page">
    <div className="photo-tools">
      <Select value={room} onChange={setRoom} ariaLabel="Room" placeholder="Which room?" options={[{ value: '', label: 'No room' }, ...rooms.map(r => ({ value: r, label: r }))]} />
      <label className={`btn primary ${busy ? 'disabled' : ''}`}><Icon name="camera" size={14} />{busy ? 'Adding…' : 'Add photos'}<input type="file" accept="image/*" multiple hidden onChange={e => { if (e.target.files?.length) upload(e.target.files); e.target.value = '' }} /></label>
      <button type="button" className="btn" disabled={!photos.ok} onClick={() => setPick(true)} title={photos.ok ? '' : 'Needs Pocket Photos (Settings → Connected apps)'}><Icon name="image" size={14} />From Pocket Photos</button>
      {!photos.ok && <CoreHint />}
    </div>
    {!list.length && <Empty icon="camera" title="No photos yet">Photos of each room — before, during, after — kept with the plan.</Empty>}
    {byRoom.map(r => <section key={r || 'none'}><h4 className="add-h">{r || 'No room'}</h4><div className="photo-grid">
      {list.filter(p => p.room === r).map(p => <button key={p.id} type="button" className="photo" onClick={() => setBig(p)}><img src={p.thumb_url} alt={p.caption || r} loading="lazy" /></button>)}
    </div></section>)}
    {pick && <PickPocketPhoto onClose={() => setPick(false)} onPick={async id => { await api.post(`${V1}/plans/${planId}/photos`, { pocket_id: id, room }); setPick(false); load() }} />}
    {big && <Modal title={big.room || 'Photo'} onClose={() => setBig(null)} size="lg" footer={<><button type="button" className="btn danger-text" onClick={async () => { await api.del(`${V1}/plan-photos/${big.id}`); setBig(null); load() }}><Icon name="trash" size={14} />Remove</button><span className="grow" /><button type="button" className="btn" onClick={() => setBig(null)}>Close</button></>}>
      <img src={big.url} alt="" className="photo-big" />
      <div className="photo-meta"><Select value={big.room} ariaLabel="Room" onChange={async v => { const r = await api.put<PlanPhoto>(`${V1}/plan-photos/${big.id}`, { room: v }); setBig(r); load() }} options={[{ value: '', label: 'No room' }, ...rooms.map(x => ({ value: x, label: x }))]} />
        <span className="muted small">{big.pocket_id ? 'From Pocket Photos · ' : ''}{big.taken_on && fmtDate(big.taken_on)}</span></div>
    </Modal>}
  </div>
}

export function PickPocketPhoto({ onClose, onPick }: { onClose: () => void; onPick: (id: string) => void }) {
  const [list, setList] = useState<{ id: string; name: string; date: string; thumb_url: string }[] | null>(null)
  const [q, setQ] = useState('')
  const [err, setErr] = useState('')
  useEffect(() => {
    const t = window.setTimeout(() => {
      api.get<{ photos: { id: string; name: string; date: string; thumb_url: string }[] }>(`${V1}/apps/photos${q ? `?q=${encodeURIComponent(q)}` : ''}`).then(r => { setList(r.photos); setErr('') }).catch(e => { setErr(errMsg(e)); setList([]) })
    }, 300)
    return () => window.clearTimeout(t)
  }, [q])
  return <Modal title="Pick from Pocket Photos" onClose={onClose} size="lg">
    <div className="add-search"><Icon name="search" size={15} /><input className="input" placeholder="Search: living room, floor plan, sofa…" value={q} onChange={e => setQ(e.target.value)} /></div>
    {err && <p className="error-text">{err}</p>}
    {!list ? <Loading /> : <div className="photo-grid">{list.map(p => <button key={p.id} type="button" className="photo" onClick={() => onPick(p.id)} title={p.name}><img src={p.thumb_url} alt={p.name} loading="lazy" /></button>)}</div>}
  </Modal>
}

// ---- details ----

export function Details({ doc, onChange }: { doc: PlanDoc; onChange: (d: PlanDoc) => void }) {
  const { toast } = useToast()
  const appState = useAppState()
  const home = appState('home', 'home:read'), rentals = appState('rentals', 'rentals:read')
  const [p, setP] = useState<Plan>(doc.plan)
  const [homes, setHomes] = useState<{ id: number; name: string; address: string; url: string }[]>([])
  const [units, setUnits] = useState<{ id: number; name: string; url: string }[]>([])
  useEffect(() => { setP(doc.plan) }, [doc.plan])
  useEffect(() => { if (home.ok) api.get<{ homes: typeof homes }>(`${V1}/apps/homes`).then(r => setHomes(r.homes)).catch(() => {}) }, [home.ok])
  useEffect(() => { if (rentals.ok) api.get<{ units: typeof units }>(`${V1}/apps/rentals`).then(r => setUnits(r.units)).catch(() => {}) }, [rentals.ok])
  const save = async (patch: Partial<Plan>) => {
    try { onChange(await api.put<PlanDoc>(`${V1}/plans/${p.id}`, patch)); announceChange() } catch (e) { toast(errMsg(e), 'err') }
  }
  return <div className="page tab-page narrow">
    <Card title="About this plan">
      <Field label="Name"><input className="input" value={p.name} onChange={e => setP({ ...p, name: e.target.value })} onBlur={() => p.name !== doc.plan.name && save({ name: p.name })} /></Field>
      <Field label="Address"><input className="input" value={p.address} onChange={e => setP({ ...p, address: e.target.value })} onBlur={() => p.address !== doc.plan.address && save({ address: p.address })} /></Field>
      <div className="grid two">
        <Field label="It is"><Select value={p.kind} onChange={v => save({ kind: v as Plan['kind'] })} options={[{ value: 'apartment', label: 'A flat' }, { value: 'house', label: 'A house' }, { value: 'studio', label: 'A studio' }, { value: 'office', label: 'An office' }, { value: 'garden', label: 'A garden' }, { value: 'other', label: 'Something else' }]} /></Field>
        <Field label="Status"><Select value={p.status} onChange={v => save({ status: v as Plan['status'] })} options={[{ value: 'living', label: 'I live here' }, { value: 'moving', label: 'Moving in' }, { value: 'idea', label: 'An idea' }, { value: 'done', label: 'Done / moved out' }]} /></Field>
      </div>
      <Field label="Notes"><textarea className="input" rows={5} value={p.notes} onChange={e => setP({ ...p, notes: e.target.value })} onBlur={() => p.notes !== doc.plan.notes && save({ notes: p.notes })} placeholder="What the landlord allows, the radiators, the light…" /></Field>
      <div className="insp-sw"><Switch checked={p.pinned} onChange={v => save({ pinned: v })} label="Pin" /><span>Pin it to the top of your plans</span></div>
    </Card>
    <Card title="Pocket Home" sub="The home this plan is of: its appliances and house plants can go on the plan">
      {!home.ok ? <><p className="muted small">{home.app?.fix || 'Connect Pocket Home in Settings → Connected apps.'}</p><CoreHint /></> : <>
        <Select value={p.home_id ? String(p.home_id) : ''} onChange={v => { const h = homes.find(x => String(x.id) === v); save({ home_id: h ? h.id : 0, home_name: h?.name || '' } as Partial<Plan>) }} placeholder="Pick a home" clearLabel="Not tied to a home" options={homes.map(h => ({ value: String(h.id), label: h.name, hint: h.address }))} />
        {doc.home_url && <a className="btn ghost sm" href={doc.home_url} target="_blank" rel="noreferrer"><Icon name="external" size={13} />Open in Pocket Home</a>}
      </>}
    </Card>
    <Card title="Pocket Rentals" sub="A flat you let: draw its plan here and keep it with the unit">
      {!rentals.ok ? <><p className="muted small">{rentals.app?.fix || 'Connect Pocket Rentals in Settings → Connected apps.'}</p><CoreHint /></> : <>
        <Select value={p.rental_id ? String(p.rental_id) : ''} onChange={v => { const u = units.find(x => String(x.id) === v); save({ rental_id: u ? u.id : 0, rental_name: u?.name || '' } as Partial<Plan>) }} placeholder="Pick a unit" clearLabel="Not a rental" options={units.map(u => ({ value: String(u.id), label: u.name }))} />
        {p.rental_id && doc.rental_url && <a className="btn ghost sm" href={doc.rental_url} target="_blank" rel="noreferrer"><Icon name="external" size={13} />Open in Pocket Rentals</a>}
      </>}
    </Card>
  </div>
}

// ---- comparing layouts ----

export function Compare({ layouts, current }: { layouts: Layout[]; current: Layout }) {
  const [a, setA] = useState(String(layouts.find(l => l.current)?.id || layouts[0].id))
  const [b, setB] = useState(String(current.id === Number(a) ? (layouts.find(l => String(l.id) !== a)?.id || current.id) : current.id))
  const [la, setLa] = useState<Layout | null>(null), [lb, setLb] = useState<Layout | null>(null)
  const { settings } = useApp()
  useEffect(() => { api.get<Layout>(`${V1}/layouts/${a}`).then(setLa).catch(() => {}) }, [a])
  useEffect(() => { api.get<Layout>(`${V1}/layouts/${b}`).then(setLb).catch(() => {}) }, [b])
  const da = useMemo(() => fixDrawing(la?.data), [la]), db = useMemo(() => fixDrawing(lb?.data), [lb])
  const diff = useMemo(() => diffItems(da, db), [da, db])
  const hl = useMemo(() => new Map<string, 'added' | 'moved'>([...diff.added.map(i => [i.id, 'added'] as [string, 'added']), ...diff.moved.map(i => [i.id, 'moved'] as [string, 'moved'])]), [diff])
  const opts = layouts.map(l => ({ value: String(l.id), label: l.name }))
  return <div className="page tab-page">
    <div className="compare">
      {[{ v: a, set: setA, l: la, d: da, h: undefined }, { v: b, set: setB, l: lb, d: db, h: hl }].map((s, i) => <div key={i} className="compare-col">
        <Select value={s.v} onChange={s.set} options={opts} ariaLabel={i ? 'Second layout' : 'First layout'} />
        {s.l ? <><PlanPreview d={s.d} className="compare-plan" highlight={s.h} units={settings?.units} />
          <p className="muted small">{area(s.l.area_cm2)} · {s.l.items_n} pieces{s.l.budget ? ` · ${money(s.l.budget)} to buy` : ''}</p>
          {s.l.notes && <p className="small">{s.l.notes}</p>}</> : <Loading />}
      </div>)}
    </div>
    <Card title="What changes">
      {!diff.added.length && !diff.moved.length && !diff.removed.length ? <p className="muted">The furniture is the same in both.</p> : <ul className="diff">
        {diff.added.map(i => <li key={i.id} className="added"><Icon name="plus" size={13} />{i.name || kindOf(i.type).name}{i.buy && i.price ? ` · ${money(i.price)}` : ''}</li>)}
        {diff.moved.map(i => <li key={i.id} className="moved"><Icon name="move" size={13} />{i.name || kindOf(i.type).name} moves</li>)}
        {diff.removed.map(i => <li key={i.id} className="removed"><Icon name="minus" size={13} />{i.name || kindOf(i.type).name} goes</li>)}
      </ul>}
      {la && lb && <p className="muted small">To buy: {money(la.budget) || '0'} → {money(lb.budget) || '0'}. On the right, new pieces are outlined green and moved ones orange.</p>}
    </Card>
  </div>
}

// ---- AI tips ----

export function AskAI({ layoutId, drawing, onClose }: { layoutId: number; drawing: Drawing; onClose: () => void }) {
  const [q, setQ] = useState('')
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const { settings } = useApp()
  const ask = async () => {
    setBusy(true); setErr('')
    try {
      const { checkLayout } = await import('../lib/geom')
      const issues = checkLayout(drawing, { clearance: settings?.clearance !== false, fmt: cm => len(cm) }).map(i => i.text)
      const r = await api.post<{ text: string }>(`${V1}/ai/tips`, { layout_id: layoutId, issues, question: q })
      setText(r.text)
    } catch (e) { setErr(errMsg(e)) } finally { setBusy(false) }
  }
  return <Modal title="Tips from AI" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Close</button><button type="button" className="btn primary" onClick={ask} disabled={busy}><Icon name="sparkles" size={14} />{busy ? 'Thinking…' : text ? 'Ask again' : 'Get tips'}</button></>}>
    <p className="muted small">The AI sees the rooms, doors, windows and furniture of this layout (sizes and places, no photos) and the problems the checks found.</p>
    <Field label="Anything in particular? (optional)"><input className="input" value={q} onChange={e => setQ(e.target.value)} placeholder="Where should the desk go? Is the living room too full?" onKeyDown={e => { if (e.key === 'Enter') ask() }} /></Field>
    {err && <p className="error-text">{err}</p>}
    {text && <div className="ai-text"><Markdown text={text} /></div>}
  </Modal>
}

// ---- send to chat ----

export function SendChat({ layoutId, onClose, shareOn }: { layoutId: number; onClose: () => void; shareOn: boolean }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const [chats, setChats] = useState<{ id: string; name: string; emoji: string }[] | null>(null)
  const [to, setTo] = useState(settings?.chat_to || '')
  const [text, setText] = useState('What do you think of this layout?')
  useEffect(() => { api.get<{ conversations: { id: string; name: string; emoji: string }[] }>(`${V1}/apps/chats`).then(r => setChats(r.conversations)).catch(e => { toast(errMsg(e), 'err'); setChats([]) }) }, []) // eslint-disable-line react-hooks/exhaustive-deps
  const send = async () => {
    try { const r = await api.post<{ text: string; url: string }>(`${V1}/apps/chat`, { layout_id: layoutId, to, text }); toast(r.text, 'ok', openAction(r.url)); onClose() } catch (e) { toast(errMsg(e), 'err') }
  }
  return <Modal title="Ask the household" onClose={onClose} footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={send}><Icon name="send" size={14} />Send</button></>}>
    <Field label="To">{chats ? <Select value={to} onChange={setTo} placeholder="Notes to myself" options={chats.map(c => ({ value: c.id, label: `${c.emoji ? c.emoji + ' ' : ''}${c.name}` }))} /> : <Loading />}</Field>
    <Field label="Message"><textarea className="input" rows={3} value={text} onChange={e => setText(e.target.value)} /></Field>
    {!shareOn && <p className="muted small">The card links to the plan in Pocket Rooms. Turn on the view-only link (Share) so people without an account can open it too.</p>}
  </Modal>
}

export { Segmented }
