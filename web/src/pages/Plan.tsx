// One plan: its floors, the layouts of each floor, and everything about it.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { Floor, Layout, PlanDoc } from '../lib/types'
import { useApp } from '../lib/app-context'
import { useAppState, announceChange, openAction } from '../lib/data'
import { area, money } from '../lib/units'
import { fixDrawing } from '../lib/geom'
import { useCrumb } from '../components/Layout'
import { Editor } from '../components/Editor'
import { Iso } from '../components/Iso'
import { planSVGString, svgToPNG, saveBlob, usePalette } from '../components/Preview'
import { Icon } from '../components/icons'
import { Confirm, Loading, Menu, type MenuItem, Modal, Segmented, errMsg, rectOf, type Anchor, useToast, copyText, Switch, Field } from '../components/ui'
import { Checklist, Compare, Details, Photos, Shopping, Summary, WayIn, AskAI, SendChat } from './PlanTabs'

type Tab = 'plan' | '3d' | 'sizes' | 'shopping' | 'checklist' | 'way' | 'photos' | 'details'

export default function PlanPage() {
  const { id } = useParams()
  const [sp, setSp] = useSearchParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { settings } = useApp()
  const appState = useAppState()
  const pal = usePalette()
  const [doc, setDoc] = useState<PlanDoc | null>(null)
  const [layout, setLayout] = useState<Layout | null>(null)
  const [err, setErr] = useState('')
  const [menu, setMenu] = useState<Anchor | null>(null)
  const [optMenu, setOptMenu] = useState<{ a: Anchor; l: Layout } | null>(null)
  const [rename, setRename] = useState<{ kind: 'layout' | 'floor'; id: number; name: string; ceiling?: number } | null>(null)
  const [confirm, setConfirm] = useState<{ title: string; message: string; run: () => Promise<void> } | null>(null)
  const [share, setShare] = useState(false)
  const [ai, setAI] = useState(false)
  const [chat, setChat] = useState(false)
  const [cut, setCut] = useState(true)
  const tab = (sp.get('tab') as Tab) || 'plan'
  const plan = doc?.plan
  useCrumb(plan?.name, '/')

  const load = useCallback(async () => {
    try { setDoc(await api.get<PlanDoc>(`${V1}/plans/${id}`)) } catch (e) { setErr(errMsg(e)) }
  }, [id])
  useEffect(() => { load() }, [load])

  const floors = plan?.floors || []
  const floor: Floor | undefined = floors.find(f => String(f.id) === sp.get('floor')) || floors.find(f => f.layouts?.some(l => String(l.id) === sp.get('layout'))) || floors[0]
  const layouts = floor?.layouts || []
  const layoutMeta = layouts.find(l => String(l.id) === sp.get('layout')) || layouts.find(l => l.current) || layouts[0]
  useEffect(() => {
    if (!layoutMeta) return
    if (layout?.id === layoutMeta.id) return
    api.get<Layout>(`${V1}/layouts/${layoutMeta.id}`).then(setLayout).catch(e => toast(errMsg(e), 'err'))
  }, [layoutMeta?.id]) // eslint-disable-line react-hooks/exhaustive-deps

  const go = (patch: Record<string, string | null>) => {
    const n = new URLSearchParams(sp)
    for (const [k, v] of Object.entries(patch)) { if (v == null) n.delete(k); else n.set(k, v) }
    setSp(n, { replace: true })
  }
  const setFloorPatch = async (f: Floor, patch: Partial<Floor>) => {
    // the picture moves live; saved a moment later
    setDoc(dc => dc && ({ ...dc, plan: { ...dc.plan, floors: dc.plan.floors!.map(x => x.id === f.id ? { ...x, ...patch, underlay: { ...(patch.underlay ?? x.underlay) } } : x) } }))
    window.clearTimeout((setFloorPatch as unknown as { t?: number }).t)
    ;(setFloorPatch as unknown as { t?: number }).t = window.setTimeout(() => {
      api.put<PlanDoc>(`${V1}/floors/${f.id}`, patch).catch(e => toast(errMsg(e), 'err'))
    }, 400)
  }

  const newOption = async (copy: boolean, shell = false) => {
    if (!floor || !layoutMeta) return
    const l = await api.post<Layout>(`${V1}/floors/${floor.id}/layouts`, copy ? { copy_from: layoutMeta.id, shell } : {})
    await load(); go({ layout: String(l.id), floor: String(floor.id), tab: 'plan' }); toast(`“${l.name}” added`)
  }
  const makeCurrent = async (l: Layout) => { await api.put(`${V1}/floors/${l.floor_id}`, { current_layout_id: l.id }); await load(); toast(`“${l.name}” is now how it is`) }
  const addFloor = async () => {
    const lv = Math.max(-1, ...floors.map(f => f.level)) + 1
    const cur = layoutMeta
    const r = await api.post<PlanDoc>(`${V1}/plans/${id}/floors`, { level: lv, copy_from: cur?.id })
    setDoc(r)
    const f = r.plan.floors!.find(x => x.level === lv)
    if (f) go({ floor: String(f.id), layout: null })
    toast('Floor added, with the same outline to start from')
  }
  const saveRename = async () => {
    if (!rename) return
    try {
      if (rename.kind === 'layout') await api.put(`${V1}/layouts/${rename.id}`, { name: rename.name })
      else await api.put(`${V1}/floors/${rename.id}`, { name: rename.name, ceiling_cm: rename.ceiling })
      setRename(null); await load()
      if (rename.kind === 'layout' && layout?.id === rename.id) setLayout(l => l && ({ ...l, name: rename.name }))
    } catch (e) { toast(errMsg(e), 'err') }
  }

  const drawingNow = useMemo(() => fixDrawing(layout?.data), [layout])
  const exportPicture = async (kind: 'svg' | 'png', toFiles: boolean) => {
    if (!layout || !plan) return
    const svg = planSVGString(drawingNow, { px: 1800, title: `${plan.name} — ${floor?.name} · ${layout.name}`, units: settings?.units, showDims: true })
    const blob = kind === 'svg' ? new Blob([svg], { type: 'image/svg+xml' }) : await svgToPNG(svg)
    if (!toFiles) { saveBlob(blob, `${plan.name} - ${layout.name}.${kind}`); return }
    try {
      const res = await fetch(`${V1}/layouts/${layout.id}/file?type=${kind}`, { method: 'POST', body: blob, credentials: 'same-origin' })
      const j = await res.json().catch(() => ({}))
      if (!res.ok) throw new Error(j.error || res.statusText)
      toast(j.text || 'Saved in Pocket Files', 'ok', openAction(j.url))
    } catch (e) { toast(errMsg(e), 'err') }
  }
  const toNotes = async () => {
    try { const r = await api.post<{ text: string; url: string }>(`${V1}/plans/${id}/note`); toast(r.text, 'ok', openAction(r.url)) } catch (e) { toast(errMsg(e), 'err') }
  }
  const files = appState('files', 'files:write'), notes = appState('notes', 'notes:write'), chatOK = appState('chat', 'chat:write')
  const coreHint = 'Needs Pocket Core and the link switched on (Settings → Connected apps)'
  const menuItems = (): MenuItem[] => [
    { label: 'Share a view-only link', icon: 'share', onSelect: () => setShare(true) },
    { label: 'Ask the household (Pocket Chat)', icon: 'message', disabled: !chatOK.ok, hint: !chatOK.ok ? '' : undefined, onSelect: () => setChat(true) },
    { label: 'Tips from AI for this layout', icon: 'sparkles', disabled: settings?.ai_help === false, onSelect: () => setAI(true) },
    'sep',
    { label: 'Save the summary in Pocket Notes', icon: 'note', disabled: !notes.ok, onSelect: toNotes },
    { label: 'Save a picture in Pocket Files', icon: 'upload', disabled: !files.ok, onSelect: () => exportPicture('png', true) },
    { label: 'Download as PNG', icon: 'image', onSelect: () => exportPicture('png', false) },
    { label: 'Download as SVG', icon: 'download', onSelect: () => exportPicture('svg', false) },
    { label: 'Print', icon: 'file', onSelect: () => window.open(`/plans/${id}/print?layout=${layout?.id || ''}`, '_blank') },
    'sep',
    { label: 'Duplicate the plan', icon: 'copy', onSelect: async () => { const r = await api.post<PlanDoc>(`${V1}/plans/${id}/duplicate`); announceChange(); navigate(`/plans/${r.plan.id}`) } },
    { label: 'Export this plan (JSON)', icon: 'download', onSelect: () => { window.location.href = `${V1}/export.json?plan=${id}` } },
    { label: plan?.archived ? 'Bring back from the archive' : 'Archive', icon: 'archive', onSelect: async () => { await api.put(`${V1}/plans/${id}`, { archived: !plan?.archived }); announceChange(); load() } },
    { label: 'Delete the plan', icon: 'trash', danger: true, onSelect: () => setConfirm({ title: 'Delete this plan?', message: `“${plan?.name}”, its floors, layouts, checklist and photos are deleted. Your measured furniture stays.`, run: async () => { await api.del(`${V1}/plans/${id}`); announceChange(); navigate('/') } }) },
  ]
  if (err) return <div className="page"><p className="error-text">{err}</p></div>
  if (!doc || !plan || !floor) return <Loading />

  const tabs: { id: Tab; label: string; icon: string; n?: number }[] = [
    { id: 'plan', label: 'Plan', icon: 'grid' }, { id: '3d', label: '3D', icon: 'layers' }, { id: 'sizes', label: 'Rooms & sizes', icon: 'ruler' },
    { id: 'shopping', label: 'To buy', icon: 'cart' }, { id: 'checklist', label: 'Checklist', icon: 'check-square', n: plan.open_todos || undefined },
    { id: 'way', label: 'Way in', icon: 'door' }, { id: 'photos', label: 'Photos', icon: 'camera' }, { id: 'details', label: 'Details', icon: 'info' },
  ]
  const full = tab === 'plan' || tab === '3d'
  return <div className={`plan-page ${full ? 'full' : ''}`}>
    <div className="plan-head">
      <div className="plan-title">
        <h2 className="ellipsis">{plan.name}</h2>
        <span className="muted small">{area(plan.area_cm2)}{plan.budget ? ` · ${money(plan.budget, plan.currency)} to buy` : ''}{plan.home_name ? ` · ${plan.home_name}` : ''}</span>
      </div>
      <div className="plan-head-actions">
        <button type="button" className="btn ghost icon-only" onClick={e => setMenu(rectOf(e.currentTarget))} aria-label="More" title="Share, save, print…"><Icon name="more" size={18} /></button>
      </div>
    </div>
    <div className="plan-bar">
      {floors.length > 1 || true ? <div className="chips floors" role="tablist" aria-label="Floors">
        {floors.map(f => <button key={f.id} type="button" role="tab" aria-selected={f.id === floor.id} className={`chip ${f.id === floor.id ? 'on' : ''}`} onClick={() => go({ floor: String(f.id), layout: null })}
          onDoubleClick={() => setRename({ kind: 'floor', id: f.id, name: f.name, ceiling: f.ceiling_cm })}>{f.name}</button>)}
        <button type="button" className="chip add" onClick={addFloor} title="Add a floor (upstairs, basement)"><Icon name="plus" size={13} />Floor</button>
      </div> : null}
      <span className="plan-bar-sep" />
      <div className="chips options" aria-label="Layouts">
        {layouts.map(l => <span key={l.id} className={`chip opt ${l.id === layoutMeta?.id ? 'on' : ''}`}>
          <button type="button" className="opt-name" onClick={() => go({ layout: String(l.id) })} title={l.current ? 'How it is now' : 'Another way to arrange it'}>{l.current && <Icon name="home" size={12} />}{l.name}</button>
          <button type="button" className="opt-more" aria-label={`More for ${l.name}`} onClick={e => setOptMenu({ a: rectOf(e.currentTarget), l })}><Icon name="chevron-down" size={12} /></button>
        </span>)}
        <button type="button" className="chip add" onClick={() => newOption(true)} title="Try another arrangement: a copy of this layout"><Icon name="plus" size={13} />Option</button>
        {layouts.length > 1 && <button type="button" className={`chip ${tab === 'details' && sp.get('compare') ? 'on' : ''}`} onClick={() => go({ tab: 'details', compare: '1' })}><Icon name="columns" size={13} />Compare</button>}
      </div>
    </div>
    <div className="plan-tabs" role="tablist">
      {tabs.map(t => <button key={t.id} type="button" role="tab" aria-selected={tab === t.id} className={`plan-tab ${tab === t.id ? 'on' : ''}`} onClick={() => go({ tab: t.id === 'plan' ? null : t.id, compare: null })}>
        <Icon name={t.icon} size={14} />{t.label}{t.n ? <span className="count">{t.n}</span> : null}</button>)}
    </div>
    <div className="plan-body">
      {!layout ? <Loading /> : <>
        {tab === 'plan' && <Editor key={layout.id} layout={layout} floor={floor} focus={sp.get('item')} onSaved={l => { setLayout(cur => cur && ({ ...cur, ...l, data: cur.data })); setDoc(dc => dc && ({ ...dc, plan: { ...dc.plan, floors: dc.plan.floors!.map(f => ({ ...f, layouts: f.layouts?.map(x => x.id === l.id ? { ...x, ...l, current: x.current } : x) })) } })) }}
          onFloor={patch => setFloorPatch(floor, patch)} />}
        {tab === '3d' && <div className="iso-wrap">
          <div className="iso-bar"><Segmented value={cut ? 'low' : 'full'} onChange={v => setCut(v === 'low')} options={[{ value: 'low', label: 'Low walls' }, { value: 'full', label: 'Full height' }]} /><span className="muted small">Drag to turn it round · {layout.name}</span></div>
          <LiveIso layoutId={layout.id} ceiling={floor.ceiling_cm} cut={cut} pal={pal} fallback={drawingNow} />
        </div>}
        {tab === 'sizes' && <Summary planId={plan.id} layoutId={layout.id} currency={plan.currency} />}
        {tab === 'shopping' && <Shopping planId={plan.id} layoutId={layout.id} currency={plan.currency} onOpenItem={(itemId) => go({ tab: null, item: itemId })} />}
        {tab === 'checklist' && <Checklist planId={plan.id} onChange={load} />}
        {tab === 'way' && <WayIn doc={doc} drawing={drawingNow} onChange={load} />}
        {tab === 'photos' && <Photos planId={plan.id} rooms={drawingNow.rooms.map(r => r.name)} />}
        {tab === 'details' && (sp.get('compare') ? <Compare layouts={layouts} current={layoutMeta!} /> : <Details doc={doc} onChange={setDoc} />)}
      </>}
    </div>
    {menu && <Menu anchor={menu} items={menuItems()} onClose={() => setMenu(null)} align="right" width={300} />}
    {optMenu && <Menu anchor={optMenu.a} onClose={() => setOptMenu(null)} width={260} items={[
      { label: 'Rename', icon: 'edit', onSelect: () => setRename({ kind: 'layout', id: optMenu.l.id, name: optMenu.l.name }) },
      { label: 'This is how it is now', icon: 'home', disabled: optMenu.l.current, onSelect: () => makeCurrent(optMenu.l) },
      { label: 'Copy it as a new option', icon: 'copy', onSelect: () => { go({ layout: String(optMenu.l.id) }); newOption(true) } },
      { label: 'New option with only the walls', icon: 'grid', onSelect: () => { go({ layout: String(optMenu.l.id) }); newOption(true, true) } },
      { label: 'Rename the floor / ceiling height', icon: 'layers', onSelect: () => setRename({ kind: 'floor', id: floor.id, name: floor.name, ceiling: floor.ceiling_cm }) },
      ...(floors.length > 1 ? [{ label: 'Delete this floor', icon: 'trash', danger: true, onSelect: () => setConfirm({ title: `Delete ${floor.name}?`, message: 'Every layout of this floor is deleted.', run: async () => { const r = await api.del<PlanDoc>(`${V1}/floors/${floor.id}`); setDoc(r); go({ floor: null, layout: null }) } }) } as MenuItem] : []),
      'sep',
      { label: 'Delete this layout', icon: 'trash', danger: true, disabled: layouts.length < 2, onSelect: () => setConfirm({ title: `Delete “${optMenu.l.name}”?`, message: 'The layout and everything placed on it is deleted.', run: async () => { const r = await api.del<PlanDoc>(`${V1}/layouts/${optMenu.l.id}`); setDoc(r); go({ layout: null }) } }) },
    ]} />}
    {rename && <Modal title={rename.kind === 'layout' ? 'Rename the layout' : 'The floor'} onClose={() => setRename(null)} footer={<><button type="button" className="btn" onClick={() => setRename(null)}>Cancel</button><button type="button" className="btn primary" onClick={saveRename}>Save</button></>}>
      <Field label="Name"><input className="input" autoFocus value={rename.name} onChange={e => setRename({ ...rename, name: e.target.value })} onKeyDown={e => { if (e.key === 'Enter') saveRename() }} /></Field>
      {rename.kind === 'floor' && <Field label="Ceiling height (cm)" help="Used for walls to paint and the 3D view."><input className="input" inputMode="decimal" value={rename.ceiling || ''} onChange={e => setRename({ ...rename, ceiling: Number(e.target.value) || undefined })} /></Field>}
    </Modal>}
    {confirm && <Confirm title={confirm.title} message={confirm.message} confirmLabel="Delete" onCancel={() => setConfirm(null)} onConfirm={async () => { try { await confirm.run() } catch (e) { toast(errMsg(e), 'err') } setConfirm(null) }} />}
    {share && <ShareModal doc={doc} onClose={() => setShare(false)} onChange={setDoc} />}
    {ai && layout && <AskAI layoutId={layout.id} drawing={drawingNow} onClose={() => setAI(false)} />}
    {chat && layout && <SendChat layoutId={layout.id} onClose={() => setChat(false)} shareOn={plan.share_on} />}
    <span hidden>{coreHint}</span>
  </div>
}

/** The 3D view of the layout as saved (re-read when you come to it). */
function LiveIso({ layoutId, ceiling, cut, pal, fallback }: { layoutId: number; ceiling: number; cut: boolean; pal: ReturnType<typeof usePalette>; fallback: ReturnType<typeof fixDrawing> }) {
  const [d, setD] = useState(fallback)
  useEffect(() => { api.get<Layout>(`${V1}/layouts/${layoutId}`).then(l => setD(fixDrawing(l.data))).catch(() => {}) }, [layoutId])
  return <Iso d={d} ceiling={ceiling} pal={pal} cut={cut} />
}

function ShareModal({ doc, onClose, onChange }: { doc: PlanDoc; onClose: () => void; onChange: (d: PlanDoc) => void }) {
  const { toast } = useToast()
  const [busy, setBusy] = useState(false)
  const set = async (on: boolean, fresh = false) => {
    setBusy(true)
    try { onChange(await api.post<PlanDoc>(`${V1}/plans/${doc.plan.id}/share`, { on, new: fresh })) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return <Modal title="Share a view-only link" onClose={onClose}>
    <p className="small">Anyone with the link sees the floors and every layout of this plan, in 2D and 3D, with the room sizes. Not your notes, prices, checklist or photos.</p>
    <div className="insp-sw"><Switch checked={doc.plan.share_on} disabled={busy} onChange={v => set(v)} label="Share" /><span>{doc.plan.share_on ? 'Shared' : 'Not shared'}</span></div>
    {doc.plan.share_on && doc.share_url && <>
      <div className="share-url"><input className="input" readOnly value={doc.share_url} onFocus={e => e.target.select()} /><button type="button" className="btn" onClick={async () => { if (await copyText(doc.share_url!)) toast('Link copied') }}><Icon name="copy" size={14} />Copy</button></div>
      <button type="button" className="btn sm" disabled={busy} onClick={() => set(true, true)}><Icon name="refresh" size={14} />Make a new link (the old one stops working)</button>
    </>}
  </Modal>
}
