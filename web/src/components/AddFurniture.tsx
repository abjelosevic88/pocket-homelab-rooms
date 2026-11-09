// Adding furniture: from the catalogue (usual sizes), your own measured
// pieces, or what your other Pocket apps already know about.
import { useEffect, useMemo, useState } from 'react'
import { api, V1 } from '../lib/api'
import type { Piece } from '../lib/types'
import { CATEGORIES, KINDS, kindOf } from '../lib/catalog'
import { size, money } from '../lib/units'
import { useAppState } from '../lib/data'
import { Icon } from './icons'
import { CoreHint, Empty, Loading, Modal, Tabs, errMsg } from './ui'
import { PieceSymbol } from './Symbols'
import { usePalette } from './Preview'

export interface Placeable { type: string; name?: string; w?: number; d?: number; h?: number; color?: string; piece_id?: number; price?: number; buy?: boolean; from?: string }

type Tab = 'catalogue' | 'mine' | 'apps'

export function Thumb({ type, w, d, color, sizePx = 46 }: { type: string; w: number; d: number; color?: string; sizePx?: number }) {
  const pal = usePalette()
  const k = (sizePx - 8) / Math.max(w, d)
  return <svg width={sizePx} height={sizePx} viewBox={`${-sizePx / 2} ${-sizePx / 2} ${sizePx} ${sizePx}`} aria-hidden="true" className="thumb">
    <g transform={`scale(${k})`}><PieceSymbol type={type} w={w} d={d} fill={color || kindOf(type).color} line={pal.line} detail={Math.min(w, d) * k > 10} /></g>
  </svg>
}

export function AddFurniture({ onClose, onPick }: { onClose: () => void; onPick: (p: Placeable) => void }) {
  const [tab, setTab] = useState<Tab>('catalogue')
  const [q, setQ] = useState('')
  const [cat, setCat] = useState('')
  const [pieces, setPieces] = useState<Piece[] | null>(null)
  useEffect(() => { api.get<Piece[]>(`${V1}/pieces`).then(setPieces).catch(() => setPieces([])) }, [])
  const kinds = useMemo(() => {
    const s = q.trim().toLowerCase()
    return KINDS.filter(k => (!cat || k.cat === cat) && (!s || (k.name + ' ' + (k.words || '') + ' ' + k.type).toLowerCase().includes(s)))
  }, [q, cat])
  const mine = (pieces || []).filter(p => !q.trim() || p.name.toLowerCase().includes(q.trim().toLowerCase()))
  return <Modal title="Add furniture" onClose={onClose} size="lg" className="add-modal">
    <Tabs value={tab} onChange={setTab} tabs={[{ id: 'catalogue', label: 'Catalogue', icon: 'grid' }, { id: 'mine', label: 'My furniture', icon: 'tag' }, { id: 'apps', label: 'From my apps', icon: 'link' }]} />
    {tab !== 'apps' && <div className="add-search"><Icon name="search" size={15} /><input className="input" autoFocus placeholder={tab === 'catalogue' ? 'Sofa, bed, wardrobe…' : 'Search your furniture'} value={q} onChange={e => setQ(e.target.value)} /></div>}
    {tab === 'catalogue' && <>
      <div className="chips add-cats">
        <button type="button" className={`chip ${!cat ? 'on' : ''}`} onClick={() => setCat('')}>Everything</button>
        {CATEGORIES.map(c => <button key={c.id} type="button" className={`chip ${cat === c.id ? 'on' : ''}`} onClick={() => setCat(c.id)}><Icon name={c.icon} size={13} />{c.name}</button>)}
      </div>
      <div className="add-grid">
        {kinds.map(k => <button key={k.type} type="button" className="add-tile" onClick={() => onPick({ type: k.type })}>
          <Thumb type={k.type} w={k.w} d={k.d} />
          <span className="add-name">{k.name}</span><span className="add-size">{size(k.w, k.d, k.h || undefined)}</span>
        </button>)}
        {!kinds.length && <p className="muted">Nothing called that. Try “Box (anything)” and give it your own size.</p>}
      </div>
    </>}
    {tab === 'mine' && (pieces === null ? <Loading /> : !mine.length ? <Empty icon="tag" title="No measured furniture yet">Measure your own pieces under Furniture: they keep their size on every plan.</Empty> :
      <div className="add-list">{mine.map(p => <button key={p.id} type="button" className="add-row" onClick={() => onPick({ type: p.type, name: p.name, w: p.width, d: p.depth, h: p.height, color: p.color || undefined, piece_id: p.id, price: p.price || undefined, buy: p.status === 'want' || p.status === 'maybe' })}>
        <Thumb type={p.type} w={p.width} d={p.depth} color={p.color} />
        <span className="grow"><b>{p.name}</b><span className="muted small">{size(p.width, p.depth, p.height)}{p.status !== 'own' ? ` · ${p.status === 'want' ? 'to buy' : 'maybe'}` : ''}{p.price ? ` · ${money(p.price, p.currency)}` : ''}</span></span>
        {p.placed_in.length > 0 && <span className="muted small">on {p.placed_in.length} {p.placed_in.length === 1 ? 'plan' : 'plans'}</span>}
      </button>)}</div>)}
    {tab === 'apps' && <FromApps onPick={onPick} />}
  </Modal>
}

interface Found { key: string; name: string; sub: string; type: string; w: number; d: number; h: number; from: string; price?: number; buy?: boolean; group: string }

function FromApps({ onPick }: { onPick: (p: Placeable) => void }) {
  const appState = useAppState()
  const [rows, setRows] = useState<Found[] | null>(null)
  const [errs, setErrs] = useState<string[]>([])
  const home = appState('home', 'home:read'), inv = appState('inventory', 'items:read'), wish = appState('wishlist', 'wishes:read')
  useEffect(() => {
    let live = true
    const out: Found[] = [], bad: string[] = []
    const jobs: Promise<void>[] = []
    if (home.ok) jobs.push(api.get<{ things: { from: string; name: string; room: string; type: string; w: number; d: number; h: number; subtitle: string; kind: string }[] }>(`${V1}/apps/home-things`).then(r => {
      for (const t of r.things) out.push({ key: t.from, name: t.name, sub: [t.room, t.subtitle].filter(Boolean).join(' · '), type: t.type, w: t.w, d: t.d, h: t.h, from: t.from, group: t.kind === 'plant' ? 'House plants (Pocket Home)' : 'Appliances (Pocket Home)' })
    }).catch(e => { bad.push('Pocket Home: ' + errMsg(e)) }))
    if (inv.ok) jobs.push(api.get<{ items: { id: number; name: string; location: string; category: string; type: string; w: number; d: number; h: number; price: number }[] }>(`${V1}/apps/inventory`).then(r => {
      for (const t of r.items) out.push({ key: 'inv' + t.id, name: t.name, sub: [t.location, t.category].filter(Boolean).join(' · ') + ' · usual size, measure it', type: t.type, w: t.w, d: t.d, h: t.h, from: `inventory:item:${t.id}`, price: t.price || undefined, group: 'Pocket Inventory' })
    }).catch(e => { bad.push('Pocket Inventory: ' + errMsg(e)) }))
    if (wish.ok) jobs.push(api.get<{ wishes: { id: number; name: string; shop: string; type: string; w: number; d: number; h: number; price: number; currency: string }[] }>(`${V1}/apps/wishes`).then(r => {
      for (const t of r.wishes) out.push({ key: 'w' + t.id, name: t.name, sub: [t.shop, t.price ? money(t.price, t.currency) : ''].filter(Boolean).join(' · '), type: t.type, w: t.w, d: t.d, h: t.h, from: `wishlist:wish:${t.id}`, price: t.price || undefined, buy: true, group: 'Pocket Wishlist (to buy)' })
    }).catch(e => { bad.push('Pocket Wishlist: ' + errMsg(e)) }))
    Promise.all(jobs).then(() => { if (live) { setRows(out); setErrs(bad) } })
    return () => { live = false }
  }, [home.ok, inv.ok, wish.ok])
  if (home.loading) return <Loading />
  const off = [home, inv, wish].filter(a => !a.ok)
  const groups = [...new Set((rows || []).map(r => r.group))]
  return <div className="add-apps">
    {off.length > 0 && <div className="add-off">
      {off.map(a => <p key={a.app?.slug || Math.random()} className="muted small"><Icon name="link" size={13} /><b>{a.app?.name || 'An app'}</b>: {a.app?.fix || a.app?.detail || 'not connected'}</p>)}
      {off.some(a => a.app?.state === 'no_core') && <CoreHint />}
    </div>}
    {rows === null ? <Loading /> : !rows.length ? (off.length < 3 && <Empty icon="link" title="Nothing to bring in">Your connected apps have no furniture, appliances or plants yet.</Empty>) :
      groups.map(g => <section key={g}><h4 className="add-h">{g}</h4><div className="add-list">{rows.filter(r => r.group === g).map(r => <button key={r.key} type="button" className="add-row" onClick={() => onPick({ type: r.type, name: r.name, w: r.w, d: r.d, h: r.h, from: r.from, price: r.price, buy: r.buy })}>
        <Thumb type={r.type} w={r.w} d={r.d} /><span className="grow"><b>{r.name}</b><span className="muted small">{r.sub}</span></span><span className="muted small">{size(r.w, r.d, r.h)}</span>
      </button>)}</div></section>)}
    {errs.map(e => <p key={e} className="muted small">{e}</p>)}
  </div>
}
