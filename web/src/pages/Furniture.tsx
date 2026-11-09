// Your furniture, measured once: what you own and what you are thinking of
// buying, with where it stands and whether it gets through the door.
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { FitResult, Piece, PieceStatus } from '../lib/types'
import { useApp } from '../lib/app-context'
import { useAppState, openAction, announceChange } from '../lib/data'
import { len, money, parseMoney, size } from '../lib/units'
import { CATEGORIES, KINDS, SWATCHES, kindOf } from '../lib/catalog'
import { useCrumb } from '../components/Layout'
import { Thumb } from '../components/AddFurniture'
import { Icon } from '../components/icons'
import { Card, Confirm, CoreHint, Empty, Field, Loading, Modal, PageHeader, Segmented, Switch, errMsg, useToast } from '../components/ui'
import { Select } from '../components/Select'
import { LenInput } from '../components/LenInput'
import { shrink, uploadPhoto } from '../lib/photo'

const STATES: { value: PieceStatus | ''; label: string }[] = [{ value: '', label: 'All' }, { value: 'own', label: 'Mine' }, { value: 'want', label: 'To buy' }, { value: 'maybe', label: 'Maybe' }, { value: 'gone', label: 'Gone' }]
const stateWord = (s: string) => ({ own: 'Mine', want: 'To buy', maybe: 'Maybe', gone: 'Gone' } as Record<string, string>)[s] || s

export default function Furniture() {
  const [sp, setSp] = useSearchParams()
  const status = sp.get('status') || ''
  const [q, setQ] = useState('')
  const [list, setList] = useState<Piece[] | null>(null)
  const [edit, setEdit] = useState<Partial<Piece> | null>(sp.get('new') ? {} : null)
  const load = useCallback(() => {
    const p = new URLSearchParams()
    if (status) p.set('status', status)
    if (q.trim()) p.set('q', q.trim())
    api.get<Piece[]>(`${V1}/pieces?${p}`).then(setList).catch(() => setList([]))
  }, [status, q])
  useEffect(() => { const t = window.setTimeout(load, 200); return () => window.clearTimeout(t) }, [load])
  return <div className="page">
    <PageHeader title="Furniture" sub="Measure a piece once: it keeps its size on every plan, and the way in checks it" actions={<button type="button" className="btn primary" onClick={() => setEdit({})}><Icon name="plus" size={15} strokeWidth={2.2} />Add a piece</button>} />
    <div className="list-tools">
      <Segmented value={status} onChange={v => setSp(v ? { status: v } : {})} options={STATES.map(s => ({ value: s.value, label: s.label }))} />
      <div className="add-search grow"><Icon name="search" size={15} /><input className="input" placeholder="Search" value={q} onChange={e => setQ(e.target.value)} /></div>
    </div>
    {!list ? <Loading /> : !list.length ? <Empty icon="tag" title={q ? 'Nothing found' : 'No furniture yet'}>{q ? 'Try another word.' : 'Add the pieces you own — the sofa, the bed, the wardrobe — and the ones you are thinking of buying, with their sizes.'}</Empty> :
      <div className="piece-grid">{list.map(p => <Link key={p.id} to={`/furniture/${p.id}`} className="piece-card">
        {p.photo_url ? <img src={p.photo_url + '?thumb=1'} alt="" className="piece-photo" /> : <div className="piece-sym"><Thumb type={p.type} w={p.width} d={p.depth} color={p.color} sizePx={64} /></div>}
        <div className="piece-body"><b className="ellipsis">{p.name}</b><span className="muted small">{size(p.width, p.depth, p.height)}</span>
          <span className="small piece-tags"><span className={`tag st-${p.status}`}>{stateWord(p.status)}</span>{p.price != null && <span>{money(p.price, p.currency)}</span>}{p.placed_in.length > 0 && <span className="muted">on {p.placed_in.length} {p.placed_in.length === 1 ? 'plan' : 'plans'}</span>}</span></div>
      </Link>)}</div>}
    {edit && <PieceEditor piece={edit} onClose={() => { setEdit(null); if (sp.get('new')) setSp({}) }} onSaved={() => { setEdit(null); load(); announceChange() }} />}
  </div>
}

export function PieceEditor({ piece, onClose, onSaved }: { piece: Partial<Piece>; onClose: () => void; onSaved: (p: Piece) => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const appState = useAppState()
  const isNew = !piece.id
  const k0 = kindOf(piece.type || 'sofa')
  const [p, setP] = useState<Partial<Piece>>({ type: 'sofa', category: k0.cat, width: k0.w, depth: k0.d, height: k0.h, status: 'own', currency: settings?.currency || 'EUR', name: '', ...piece })
  const [priceText, setPriceText] = useState(piece.price != null ? (piece.price / 100).toFixed(2) : '')
  const [aiText, setAIText] = useState('')
  const [busy, setBusy] = useState('')
  const [from, setFrom] = useState<'' | 'inventory' | 'wishlist'>('')
  const set = (patch: Partial<Piece>) => setP(x => ({ ...x, ...patch }))
  const pickType = (t: string) => {
    const k = kindOf(t)
    const untouched = isNew && p.width === kindOf(p.type || 'box').w && p.depth === kindOf(p.type || 'box').d
    set({ type: t, category: k.cat, ...(untouched ? { width: k.w, depth: k.d, height: k.h } : {}) })
  }
  const readAI = async () => {
    setBusy('Reading…')
    try {
      const r = await api.post<{ name?: string; type?: string; category?: string; width?: number; depth?: number; height?: number; price?: number; currency?: string; shop?: string; note?: string }>(`${V1}/ai/sizes`, { text: aiText })
      set({ ...(r.name && !p.name ? { name: r.name } : {}), ...(r.type && KINDS.some(k => k.type === r.type) ? { type: r.type, category: kindOf(r.type).cat } : {}),
        ...(r.width ? { width: r.width } : {}), ...(r.depth ? { depth: r.depth } : {}), ...(r.height ? { height: r.height } : {}), ...(r.shop ? { shop: r.shop } : {}), ...(r.currency ? { currency: r.currency } : {}) })
      if (r.price) setPriceText((r.price / 100).toFixed(2))
      toast(r.note ? `Read it. ${r.note}` : 'Read the size — check it before saving.')
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const save = async () => {
    if (!p.name?.trim()) { toast('Give the piece a name', 'err'); return }
    setBusy('Saving…')
    try {
      const body = { name: p.name, type: p.type, category: p.category, width: p.width, depth: p.depth, height: p.height, color: p.color || '', status: p.status, price: parseMoney(priceText),
        currency: p.currency, url: p.url || '', shop: p.shop || '', notes: p.notes || '', photo_id: p.photo_id || 0, inventory_id: p.inventory_id || 0, inventory_name: p.inventory_name || '', wish_id: p.wish_id || 0, wish_name: p.wish_name || '' }
      const r = isNew ? await api.post<Piece>(`${V1}/pieces`, body) : await api.put<Piece>(`${V1}/pieces/${piece.id}`, body)
      onSaved(r)
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const photo = async (f: File) => {
    setBusy('Adding the photo…')
    try { const s = await shrink(f); const up = await uploadPhoto(s, 'piece'); set({ photo_id: up.id, photo_url: up.url, color: p.color || s.colors[0] || '' }) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const inv = appState('inventory', 'items:read'), wish = appState('wishlist', 'wishes:read')
  return <Modal title={isNew ? 'Add a piece of furniture' : `Edit ${piece.name}`} onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={!!busy} onClick={save}>{busy || 'Save'}</button></>}>
    {isNew && <div className="pe-from">
      <span className="muted small">Bring it in from</span>
      <button type="button" className={`chip ${from === 'inventory' ? 'on' : ''}`} disabled={!inv.ok} onClick={() => setFrom(from === 'inventory' ? '' : 'inventory')}><Icon name="package" size={13} />Pocket Inventory</button>
      <button type="button" className={`chip ${from === 'wishlist' ? 'on' : ''}`} disabled={!wish.ok} onClick={() => setFrom(from === 'wishlist' ? '' : 'wishlist')}><Icon name="gift" size={13} />Pocket Wishlist</button>
      {(!inv.ok || !wish.ok) && <CoreHint />}
    </div>}
    {from && <ImportPick from={from} onPick={x => { setFrom(''); set(x); if (x.price) setPriceText((x.price / 100).toFixed(2)) }} />}
    <div className="pe-grid">
      <div className="pe-main">
        <Field label="Name"><input className="input" autoFocus={isNew} value={p.name || ''} onChange={e => set({ name: e.target.value })} placeholder="Grey three-seater, our bed…" /></Field>
        <div className="grid two">
          <Field label="Looks like"><Select value={p.type || 'box'} onChange={pickType} options={KINDS.map(x => ({ value: x.type, label: x.name, group: CATEGORIES.find(c => c.id === x.cat)?.name }))} /></Field>
          <Field label="Status"><Select value={p.status || 'own'} onChange={v => set({ status: v as PieceStatus })} options={[{ value: 'own', label: 'Mine' }, { value: 'want', label: 'To buy' }, { value: 'maybe', label: 'Maybe' }, { value: 'gone', label: 'Gone (sold, given away)' }]} /></Field>
        </div>
        <div className="grid three">
          <Field label="Width" help="side to side"><LenInput value={p.width || 0} min={1} onChange={v => set({ width: v })} /></Field>
          <Field label="Depth" help="front to back"><LenInput value={p.depth || 0} min={1} onChange={v => set({ depth: v })} /></Field>
          <Field label="Height"><LenInput value={p.height || 0} min={1} onChange={v => set({ height: v })} /></Field>
        </div>
        <div className="grid two">
          <Field label="Price"><span className="len-input"><input className="input" inputMode="decimal" value={priceText} onChange={e => setPriceText(e.target.value)} placeholder="0.00" /><span className="len-unit">{p.currency}</span></span></Field>
          <Field label="Shop"><input className="input" value={p.shop || ''} onChange={e => set({ shop: e.target.value })} /></Field>
        </div>
        <Field label="Link"><input className="input" value={p.url || ''} onChange={e => set({ url: e.target.value })} placeholder="https://…" /></Field>
        <Field label="Notes"><textarea className="input" rows={2} value={p.notes || ''} onChange={e => set({ notes: e.target.value })} placeholder="Legs screw off, comes in two parts…" /></Field>
      </div>
      <div className="pe-side">
        <div className="pe-preview">{p.photo_id ? <img src={`${V1}/photos/${p.photo_id}?thumb=1`} alt="" /> : <Thumb type={p.type || 'box'} w={p.width || 50} d={p.depth || 50} color={p.color} sizePx={120} />}</div>
        <label className="btn sm"><Icon name="camera" size={14} />{p.photo_id ? 'Another photo' : 'Add a photo'}<input type="file" accept="image/*" hidden onChange={e => { const f = e.target.files?.[0]; if (f) photo(f); e.target.value = '' }} /></label>
        <div className="swatches">{SWATCHES.map(c => <button key={c || 'n'} type="button" className={`swatch ${(p.color || '') === c ? 'on' : ''}`} style={{ background: c || kindOf(p.type || 'box').color }} onClick={() => set({ color: c })} aria-label={c || 'Usual colour'} />)}</div>
        {settings?.ai_help !== false && <div className="pe-ai">
          <span className="small"><Icon name="sparkles" size={13} /> Paste the shop's description; the AI reads the size.</span>
          <textarea className="input" rows={3} value={aiText} onChange={e => setAIText(e.target.value)} placeholder="W 228 x D 95 x H 83 cm, seat height 45…" />
          <button type="button" className="btn sm" disabled={!aiText.trim() || !!busy} onClick={readAI}>Read the size</button>
        </div>}
      </div>
    </div>
  </Modal>
}

function ImportPick({ from, onPick }: { from: 'inventory' | 'wishlist'; onPick: (p: Partial<Piece>) => void }) {
  const [rows, setRows] = useState<{ id: number; name: string; sub: string; price: number; type: string; w: number; d: number; h: number; link?: string; shop?: string }[] | null>(null)
  const [q, setQ] = useState('')
  const [err, setErr] = useState('')
  useEffect(() => {
    const t = window.setTimeout(() => {
      const url = from === 'inventory' ? `${V1}/apps/inventory?q=${encodeURIComponent(q)}` : `${V1}/apps/wishes?q=${encodeURIComponent(q)}`
      api.get<{ items?: { id: number; name: string; location: string; price: number; type: string; w: number; d: number; h: number }[]; wishes?: { id: number; name: string; shop: string; price: number; type: string; w: number; d: number; h: number; link: string }[] }>(url)
        .then(r => setRows(from === 'inventory' ? (r.items || []).map(i => ({ ...i, sub: i.location })) : (r.wishes || []).map(w => ({ ...w, sub: w.shop }))))
        .catch(e => { setErr(errMsg(e)); setRows([]) })
    }, 250)
    return () => window.clearTimeout(t)
  }, [from, q])
  return <div className="pe-import">
    <div className="add-search"><Icon name="search" size={15} /><input className="input" autoFocus placeholder="Search" value={q} onChange={e => setQ(e.target.value)} /></div>
    {err && <p className="error-text">{err}</p>}
    {!rows ? <Loading /> : <div className="rows">{rows.slice(0, 30).map(r => <button key={r.id} type="button" className="row" onClick={() => onPick({ name: r.name, type: r.type, category: kindOf(r.type).cat, width: r.w, depth: r.d, height: r.h, price: r.price || null, shop: r.shop || '', url: r.link || '',
      ...(from === 'inventory' ? { inventory_id: r.id, inventory_name: r.name, status: 'own' as PieceStatus } : { wish_id: r.id, wish_name: r.name, status: 'want' as PieceStatus }) })}>
      <Thumb type={r.type} w={r.w} d={r.d} /><span className="grow"><b>{r.name}</b><span className="muted small">{r.sub}</span></span></button>)}</div>}
    <p className="muted small">The size is a usual one for its kind: measure the real piece and correct it.</p>
  </div>
}

// ---- one piece ----

export function PiecePage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { settings } = useApp()
  const appState = useAppState()
  const [data, setData] = useState<{ piece: Piece; fits: { route_id: number; route: string; plan_id: number; plan: string; result: FitResult }[]; inventory_url?: string; wish_url?: string; money_url?: string } | null>(null)
  const [edit, setEdit] = useState(false)
  const [del, setDel] = useState(false)
  const [buy, setBuy] = useState(false)
  const load = useCallback(() => { api.get<typeof data>(`${V1}/pieces/${id}`).then(setData).catch(e => toast(errMsg(e), 'err')) }, [id]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { load() }, [load])
  useCrumb(data?.piece.name, '/furniture')
  if (!data) return <Loading />
  const p = data.piece
  const wish = appState('wishlist', 'wishes:write')
  const toWish = async () => { try { const r = await api.post<{ text: string; url: string }>(`${V1}/pieces/${p.id}/wish`); toast(r.text, 'ok', openAction(r.url)); load() } catch (e) { toast(errMsg(e), 'err') } }
  return <div className="page narrow">
    <div className="piece-head">
      <div className="piece-head-img">{p.photo_url ? <img src={p.photo_url} alt="" /> : <Thumb type={p.type} w={p.width} d={p.depth} color={p.color} sizePx={110} />}</div>
      <div className="grow">
        <h2>{p.name}</h2>
        <p className="piece-size">{size(p.width, p.depth, p.height)} <span className="muted small">width × depth × height</span></p>
        <p className="small piece-tags"><span className={`tag st-${p.status}`}>{stateWord(p.status)}</span>{p.price != null && <span>{money(p.price, p.currency)}</span>}{p.shop && <span className="muted">{p.shop}</span>}{p.url && <a href={p.url} target="_blank" rel="noreferrer"><Icon name="external" size={12} /> shop page</a>}</p>
        <div className="row-actions">
          <button type="button" className="btn sm" onClick={() => setEdit(true)}><Icon name="edit" size={14} />Edit</button>
          <button type="button" className="btn sm" onClick={() => navigate(`/fit?piece=${p.id}`)}><Icon name="door" size={14} />Will it fit?</button>
          {p.status !== 'own' && <button type="button" className="btn sm primary" onClick={() => setBuy(true)}><Icon name="check" size={14} />I bought it</button>}
          {p.status !== 'own' && !p.wish_id && <button type="button" className="btn sm" disabled={!wish.ok} title={wish.ok ? '' : 'Needs Pocket Wishlist (Settings → Connected apps)'} onClick={toWish}><Icon name="gift" size={14} />To my wishlist</button>}
          <button type="button" className="btn sm danger-text" onClick={() => setDel(true)}><Icon name="trash" size={14} />Delete</button>
        </div>
      </div>
    </div>
    {p.notes && <Card><p className="pre">{p.notes}</p></Card>}
    <Card title="The way in" sub={`Through each plan's doors, hallways and stairs, leaving ${len(settings?.hands_cm ?? 2)} for hands`} flush>
      {!data.fits.length ? <p className="muted pad">No way in measured yet. Open a plan → Way in, and add its doors and hallways.</p> : <div className="rows">
        {data.fits.map(f => <Link key={f.route_id} to={`/fit?piece=${p.id}&route=${f.route_id}`} className="row">
          <Icon name={f.result.ok ? 'check-circle' : 'x-circle'} size={16} className={f.result.ok ? 'ok' : 'bad'} />
          <span className="grow"><b>{f.plan} · {f.route}</b><span className="muted small">{f.result.summary}</span></span><Icon name="chevron-right" size={14} />
        </Link>)}</div>}
    </Card>
    <Card title="Where it stands" flush>
      {!p.placed_in.length ? <p className="muted pad">Not on any plan yet: open a plan, Furniture → My furniture.</p> : <div className="rows">{p.placed_in.map((w, i) => <div key={i} className="row"><Icon name="map" size={15} className="muted" />{w}</div>)}</div>}
    </Card>
    {(data.inventory_url || data.wish_url || data.money_url) && <Card title="In your other apps"><div className="row-actions">
      {data.inventory_url && <a className="btn sm" href={data.inventory_url} target="_blank" rel="noreferrer"><Icon name="package" size={14} />Pocket Inventory</a>}
      {data.wish_url && <a className="btn sm" href={data.wish_url} target="_blank" rel="noreferrer"><Icon name="gift" size={14} />Pocket Wishlist</a>}
      {data.money_url && <a className="btn sm" href={data.money_url} target="_blank" rel="noreferrer"><Icon name="wallet" size={14} />Pocket Money</a>}
    </div></Card>}
    {edit && <PieceEditor piece={p} onClose={() => setEdit(false)} onSaved={() => { setEdit(false); load(); announceChange() }} />}
    {del && <Confirm title={`Delete ${p.name}?`} message="It stays on your plans as a plain piece, without its link here." confirmLabel="Delete" onCancel={() => setDel(false)} onConfirm={async () => { await api.del(`${V1}/pieces/${p.id}`); announceChange(); navigate('/furniture') }} />}
    {buy && <Bought piece={p} onClose={() => setBuy(false)} onDone={() => { setBuy(false); load() }} />}
  </div>
}

function Bought({ piece, onClose, onDone }: { piece: Piece; onClose: () => void; onDone: () => void }) {
  const { toast } = useToast()
  const appState = useAppState()
  const moneyOK = appState('money', 'transactions:write')
  const [price, setPrice] = useState(piece.price != null ? (piece.price / 100).toFixed(2) : '')
  const [book, setBook] = useState(moneyOK.ok)
  const [busy, setBusy] = useState(false)
  const go = async () => {
    setBusy(true)
    try { const r = await api.post<{ text: string; money_url?: string }>(`${V1}/pieces/${piece.id}/bought`, { price: parseMoney(price), book: book && moneyOK.ok }); toast(r.text, 'ok', openAction(r.money_url)); onDone() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return <Modal title={`You bought ${piece.name}`} onClose={onClose} footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={busy} onClick={go}>Done</button></>}>
    <Field label="What it cost"><span className="len-input"><input className="input" inputMode="decimal" value={price} onChange={e => setPrice(e.target.value)} /><span className="len-unit">{piece.currency}</span></span></Field>
    <div className="insp-sw"><Switch checked={book && moneyOK.ok} disabled={!moneyOK.ok} onChange={setBook} label="Book it" /><span>Book it as an expense in Pocket Money</span></div>
    {!moneyOK.ok && <CoreHint />}
    <p className="muted small">It becomes one of your pieces; on your plans it is no longer “to buy”{piece.wish_id ? ', and the wish is ticked off in Pocket Wishlist' : ''}.</p>
  </Modal>
}
