// The view-only page behind a plan's share link: no account needed.
import { useEffect, useMemo, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { V1 } from '../lib/api'
import type { Drawing } from '../lib/types'
import { area, setUnits } from '../lib/units'
import { fixDrawing, polyArea } from '../lib/geom'
import { PlanPreview, usePalette } from '../components/Preview'
import { Iso } from '../components/Iso'
import { Loading, Segmented } from '../components/ui'

interface PubLayout { id: number; name: string; notes: string; data: Drawing; area_cm2: number; rooms_n: number; items_n: number; current: boolean }
interface Pub { name: string; kind: string; units: 'metric' | 'imperial'; floors: { id: number; name: string; ceiling_cm: number; layouts: PubLayout[] }[] }

export default function Shared() {
  const { token } = useParams()
  const [sp] = useSearchParams()
  const [doc, setDoc] = useState<Pub | null>(null)
  const [err, setErr] = useState('')
  const [fi, setFi] = useState(0)
  const [li, setLi] = useState<number | null>(null)
  const [mode, setMode] = useState<'2d' | '3d'>('2d')
  const pal = usePalette()
  useEffect(() => {
    fetch(`${V1}/public/s/${token}`).then(async r => { const j = await r.json(); if (!r.ok) throw new Error(j.error || 'not found'); setUnits(j.units); setDoc(j)
      const want = Number(sp.get('layout'))
      if (want) j.floors.forEach((f: Pub['floors'][number], i: number) => { const k = f.layouts.findIndex(l => l.id === want); if (k >= 0) { setFi(i); setLi(k) } })
    }).catch(e => setErr(e.message))
  }, [token]) // eslint-disable-line react-hooks/exhaustive-deps
  const floor = doc?.floors[fi]
  const lay = floor ? floor.layouts[li ?? Math.max(0, floor.layouts.findIndex(l => l.current))] : undefined
  const d = useMemo(() => fixDrawing(lay?.data), [lay])
  if (err) return <div className="shared-page"><div className="shared-msg"><img src="/favicon.svg" alt="" width={48} /><h2>This plan is not shared</h2><p className="muted">The link was switched off or replaced.</p></div></div>
  if (!doc || !floor || !lay) return <Loading />
  return <div className="shared-page">
    <header className="shared-head"><img src="/favicon.svg" alt="" width={28} /><div className="grow"><h1>{doc.name}</h1><span className="muted small">{area(lay.area_cm2)} · {lay.rooms_n} rooms · shared from Pocket Rooms</span></div>
      <Segmented value={mode} onChange={setMode} options={[{ value: '2d', label: 'Plan' }, { value: '3d', label: '3D' }]} /></header>
    <div className="chips">{doc.floors.length > 1 && doc.floors.map((f, i) => <button key={f.id} type="button" className={`chip ${i === fi ? 'on' : ''}`} onClick={() => { setFi(i); setLi(null) }}>{f.name}</button>)}
      {floor.layouts.length > 1 && floor.layouts.map((l, i) => <button key={l.id} type="button" className={`chip ${l.id === lay.id ? 'on' : ''}`} onClick={() => setLi(i)}>{l.name}</button>)}</div>
    {lay.notes && <p className="small">{lay.notes}</p>}
    <div className="shared-view">{mode === '2d' ? <PlanPreview d={d} className="shared-plan" units={doc.units} /> : <Iso d={d} ceiling={floor.ceiling_cm} pal={pal} cut />}</div>
    <div className="shared-rooms">{d.rooms.map(r => <span key={r.id} className="chip">{r.name} · {area(polyArea(r.points))}</span>)}</div>
  </div>
}
