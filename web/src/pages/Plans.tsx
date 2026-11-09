// The start page: your plans as little floor plans, what is due on their
// checklists, the furniture you want — and starting a new plan.
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { Drawing, Piece, Plan, PlanDoc, Todo } from '../lib/types'
import { useApp } from '../lib/app-context'
import { useAppState, announceChange, useOnChange } from '../lib/data'
import { area, money, size } from '../lib/units'
import { fixDrawing } from '../lib/geom'
import { PlanPreview } from '../components/Preview'
import { Icon } from '../components/icons'
import { CoreHint, Empty, Field, Loading, Modal, PageHeader, Segmented, errMsg, useToast } from '../components/ui'
import { Select } from '../components/Select'
import { LenInput } from '../components/LenInput'
import { fmtDate } from '../lib/format'
import { shrink, uploadPhoto } from '../lib/photo'

interface Home { plans: Plan[]; todos: Todo[]; wanted: Piece[]; pieces: number }

const STATUS: Record<string, string> = { living: 'Home', moving: 'Moving in', idea: 'Idea', done: 'Done' }

export default function Plans() {
  const [home, setHome] = useState<Home | null>(null)
  const [arch, setArch] = useState<Plan[] | null>(null)
  const [sp, setSp] = useSearchParams()
  const navigate = useNavigate()
  const load = useCallback(() => { api.get<Home>(`${V1}/home`).then(setHome).catch(() => setHome({ plans: [], todos: [], wanted: [], pieces: 0 })) }, [])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  const creating = sp.get('new') === '1'
  if (!home) return <Loading />
  return <div className="page">
    <PageHeader title="Plans" sub="Your rooms, drawn to scale" actions={<button type="button" className="btn primary" onClick={() => setSp({ new: '1' })}><Icon name="plus" size={15} strokeWidth={2.2} />New plan</button>} />
    {!home.plans.length ? <div className="welcome">
      <Empty icon="grid" title="Draw your first plan">Start from a ready-made flat, from a picture of a floor plan, or from an empty floor. Then place your furniture by its real size and see what fits.</Empty>
      <button type="button" className="btn primary lg" onClick={() => setSp({ new: '1' })}><Icon name="plus" size={16} />New plan</button>
    </div> : <div className="plan-cards">
      {home.plans.map(p => <Link key={p.id} to={`/plans/${p.id}`} className="plan-card">
        <div className="plan-card-img">{p.preview?.data ? <PlanPreview d={fixDrawing(p.preview.data)} pad={14} label={`Plan of ${p.name}`} /> : <div className="plan-card-none"><Icon name="grid" size={28} /></div>}</div>
        <div className="plan-card-body">
          <div className="plan-card-title"><b className="ellipsis">{p.name}</b>{p.pinned && <Icon name="pin" size={13} className="muted" />}<span className={`tag st-${p.status}`}>{STATUS[p.status]}</span></div>
          <span className="muted small">{area(p.area_cm2)} · {p.rooms_n} {p.rooms_n === 1 ? 'room' : 'rooms'} · {p.items_n} {p.items_n === 1 ? 'piece' : 'pieces'}{(p.floors?.length || 0) > 1 ? ` · ${p.floors!.length} floors` : ''}</span>
          {(p.budget > 0 || p.open_todos > 0) && <span className="small plan-card-more">{p.budget > 0 && <span><Icon name="cart" size={12} />{money(p.budget, p.currency)} to buy</span>}{p.open_todos > 0 && <span><Icon name="check-square" size={12} />{p.open_todos} to do</span>}</span>}
        </div>
      </Link>)}
      <button type="button" className="plan-card new" onClick={() => setSp({ new: '1' })}><Icon name="plus" size={26} /><span>New plan</span></button>
    </div>}
    {(home.todos.length > 0 || home.wanted.length > 0) && <div className="home-cols">
      {home.todos.length > 0 && <section className="card"><div className="card-head"><h3>Coming up</h3></div><div className="rows">
        {home.todos.map(t => <Link key={t.id} to={`/plans/${t.plan_id}?tab=checklist`} className="row"><Icon name="circle" size={15} className="muted" /><span className="grow">{t.title}<span className="muted small">{t.plan_name}</span></span>{t.due && <span className="muted small">{fmtDate(t.due, false)}</span>}</Link>)}
      </div></section>}
      {home.wanted.length > 0 && <section className="card"><div className="card-head"><h3>Furniture you want</h3><Link to="/furniture?status=want" className="btn ghost sm">All</Link></div><div className="rows">
        {home.wanted.map(p => <Link key={p.id} to={`/furniture/${p.id}`} className="row"><Icon name="tag" size={15} className="muted" /><span className="grow">{p.name}<span className="muted small">{size(p.width, p.depth, p.height)}</span></span>{p.price != null && <span className="muted small">{money(p.price, p.currency)}</span>}</Link>)}
      </div></section>}
    </div>}
    <div className="home-foot">
      <button type="button" className="btn ghost sm" onClick={() => { if (arch) setArch(null); else api.get<Plan[]>(`${V1}/plans?archived=1&lean=1`).then(setArch) }}><Icon name="archive" size={14} />{arch ? 'Hide the archive' : 'Archived plans'}</button>
    </div>
    {arch && (arch.length ? <div className="rows card">{arch.map(p => <Link key={p.id} to={`/plans/${p.id}`} className="row"><span className="grow">{p.name}</span><span className="muted small">{area(p.area_cm2)}</span></Link>)}</div> : <p className="muted small">Nothing archived.</p>)}
    {creating && <NewPlan onClose={() => setSp({})} onDone={id => { announceChange(); navigate(`/plans/${id}`) }} />}
  </div>
}

interface Template { key: string; name: string; about: string; drawing: Drawing }

export function NewPlan({ onClose, onDone }: { onClose: () => void; onDone: (id: number) => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const appState = useAppState()
  const [how, setHow] = useState<'template' | 'picture' | 'file'>('template')
  const [templates, setTemplates] = useState<Template[] | null>(null)
  const [tpl, setTpl] = useState('one_bed')
  const [name, setName] = useState('')
  const [kind, setKind] = useState('apartment')
  const [status, setStatus] = useState('living')
  const [w, setW] = useState(400), [d, setD] = useState(350)
  const [homeID, setHomeID] = useState('')
  const [rentalID, setRentalID] = useState('')
  const [homes, setHomes] = useState<{ id: number; name: string; address: string; area_m2: number }[]>([])
  const [units, setUnits] = useState<{ id: number; name: string; area_m2: number }[]>([])
  const [busy, setBusy] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [hint, setHint] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)
  const home = appState('home', 'home:read'), rentals = appState('rentals', 'rentals:read')
  useEffect(() => { api.get<Template[]>(`${V1}/templates`).then(setTemplates).catch(() => setTemplates([])) }, [])
  useEffect(() => { if (home.ok) api.get<{ homes: typeof homes }>(`${V1}/apps/homes`).then(r => setHomes(r.homes)).catch(() => {}) }, [home.ok])
  useEffect(() => { if (rentals.ok) api.get<{ units: typeof units }>(`${V1}/apps/rentals`).then(r => setUnits(r.units)).catch(() => {}) }, [rentals.ok])
  const hm = homes.find(h => String(h.id) === homeID), un = units.find(u => String(u.id) === rentalID)
  const finalName = name.trim() || hm?.name || un?.name || (templates?.find(t => t.key === tpl)?.name ?? 'My plan')
  const base = () => ({ name: finalName, kind, status, home_id: hm?.id || 0, home_name: hm?.name || '', rental_id: un?.id || 0, rental_name: un?.name || '' })
  const create = async () => {
    try {
      if (how === 'template') {
        setBusy('Making the plan…')
        const r = await api.post<PlanDoc>(`${V1}/plans`, { ...base(), template: tpl, width: w, depth: d })
        onDone(r.plan.id)
      } else if (how === 'picture') {
        if (!file) return
        setBusy('Reading the plan… (this can take a minute)')
        const s = await shrink(file, 2000)
        const up = await uploadPhoto(s, 'underlay')
        const read = await api.post<{ drawing: Drawing; note: string; rooms: number }>(`${V1}/ai/read-plan`, { photo_id: up.id, hint })
        const r = await api.post<PlanDoc>(`${V1}/plans`, { ...base(), drawing: read.drawing })
        // keep the picture underneath to check and trace
        const f = r.plan.floors?.[0]
        if (f) {
          const dr = read.drawing
          const xs = dr.rooms.flatMap(x => x.points.map(p => p.x)), ys = dr.rooms.flatMap(x => x.points.map(p => p.y))
          const bw = xs.length ? Math.max(...xs) - Math.min(...xs) : 1000
          await api.put(`${V1}/floors/${f.id}`, { underlay_photo_id: up.id, underlay: { x: xs.length ? Math.min(...xs) : 0, y: ys.length ? Math.min(...ys) : 0, cm_per_px: bw / (up.width || s.width) * 1.0, opacity: 0.45, on: true, w_px: up.width || s.width, h_px: up.height || s.height } })
        }
        toast(read.rooms ? `Read ${read.rooms} rooms. Check the sizes against the picture underneath.` : (read.note || 'No rooms found: draw them over the picture.'))
        onDone(r.plan.id)
      } else {
        if (!file) return
        setBusy('Bringing it in…')
        const fd = new FormData(); fd.append('file', file)
        const res = await fetch(`${V1}/import`, { method: 'POST', body: fd, credentials: 'same-origin' })
        const j = await res.json().catch(() => ({}))
        if (!res.ok) throw new Error(j.error || res.statusText)
        if (j.plan_id) onDone(j.plan_id); else { toast('Imported'); onClose(); announceChange() }
      }
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const aiOff = settings?.ai_help === false
  return <Modal title="New plan" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={!!busy || (how !== 'template' && !file)} onClick={create}>{busy || 'Create the plan'}</button></>}>
    <Segmented value={how} onChange={setHow} options={[{ value: 'template', label: 'Start drawing' }, { value: 'picture', label: 'From a picture' }, { value: 'file', label: 'From a file' }]} />
    {how === 'template' && <>
      <div className="tpl-grid">{(templates || []).map(t => <button key={t.key} type="button" className={`tpl ${tpl === t.key ? 'on' : ''}`} onClick={() => setTpl(t.key)}>
        <div className="tpl-img">{t.drawing.rooms.length ? <PlanPreview d={t.drawing} pad={8} /> : <Icon name="grid" size={26} />}</div>
        <b>{t.name}</b><span className="muted small">{t.about}</span>
      </button>)}</div>
      {tpl === 'room' && <div className="grid two"><Field label="Width"><LenInput value={w} min={50} onChange={setW} /></Field><Field label="Depth"><LenInput value={d} min={50} onChange={setD} /></Field></div>}
    </>}
    {how === 'picture' && <div className="np-pic">
      <p className="small">An estate agent's floor plan, a photo of a printed plan or a sketch with sizes. The AI reads the rooms and their sizes; the picture stays underneath so you can check and correct it.</p>
      {aiOff ? <p className="muted small">AI help is switched off (Settings → AI). You can still start empty and trace over the picture from the plan's details.</p> : <>
        <button type="button" className="btn" onClick={() => fileRef.current?.click()}><Icon name="image" size={14} />{file ? file.name : 'Choose the picture'}</button>
        <input ref={fileRef} type="file" accept="image/*" hidden onChange={e => setFile(e.target.files?.[0] || null)} />
        <Field label="Anything that helps (optional)"><input className="input" value={hint} onChange={e => setHint(e.target.value)} placeholder="The flat is 62 m²; the living room is 5.2 m wide" /></Field>
      </>}
    </div>}
    {how === 'file' && <div className="np-pic">
      <p className="small">A <b>Sweet Home 3D</b> file (.sh3d — rooms, walls, doors, windows and furniture come in), or a Pocket Rooms export (.json).</p>
      <button type="button" className="btn" onClick={() => fileRef.current?.click()}><Icon name="upload" size={14} />{file ? file.name : 'Choose the file'}</button>
      <input ref={fileRef} type="file" accept=".sh3d,.xml,.json,application/json,application/zip" hidden onChange={e => setFile(e.target.files?.[0] || null)} />
    </div>}
    {how !== 'file' && <div className="np-details">
      <div className="grid two">
        <Field label="Name"><input className="input" value={name} placeholder={finalName} onChange={e => setName(e.target.value)} /></Field>
        <Field label="It is"><Select value={kind} onChange={setKind} options={[{ value: 'apartment', label: 'A flat' }, { value: 'house', label: 'A house' }, { value: 'studio', label: 'A studio' }, { value: 'office', label: 'An office' }, { value: 'garden', label: 'A garden' }, { value: 'other', label: 'Something else' }]} /></Field>
      </div>
      <div className="grid two">
        <Field label="Status"><Select value={status} onChange={setStatus} options={[{ value: 'living', label: 'I live here' }, { value: 'moving', label: 'Moving in' }, { value: 'idea', label: 'An idea' }]} /></Field>
        <Field label="In Pocket Home" help={!home.ok ? undefined : 'Its appliances and plants can go on the plan'}>
          {home.ok ? <Select value={homeID} onChange={setHomeID} placeholder="Not tied to a home" clearLabel="Not tied to a home" options={homes.map(h => ({ value: String(h.id), label: h.name, hint: h.address }))} /> : <><Select value="" onChange={() => {}} disabled options={[]} placeholder="Pocket Home not connected" /><CoreHint /></>}
        </Field>
      </div>
      {rentals.ok && units.length > 0 && <Field label="A flat you let (Pocket Rentals)"><Select value={rentalID} onChange={setRentalID} placeholder="Not a rental" clearLabel="Not a rental" options={units.map(u => ({ value: String(u.id), label: u.name, hint: u.area_m2 ? `${u.area_m2} m²` : '' }))} /></Field>}
    </div>}
    <span hidden>{settings?.units}</span>
  </Modal>
}
