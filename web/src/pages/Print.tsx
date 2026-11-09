// A page to print: the layout on A4 with its rooms and sizes.
import { useEffect, useMemo, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { Layout, LayoutSum, PlanDoc } from '../lib/types'
import { area, len, money } from '../lib/units'
import { fixDrawing } from '../lib/geom'
import { planSVGString } from '../components/Preview'
import { Loading } from '../components/ui'
import { useApp } from '../lib/app-context'

export default function Print() {
  const { id } = useParams()
  const [sp] = useSearchParams()
  const { settings } = useApp()
  const [doc, setDoc] = useState<PlanDoc | null>(null)
  const [l, setL] = useState<Layout | null>(null)
  const [sum, setSum] = useState<LayoutSum | null>(null)
  useEffect(() => {
    (async () => {
      const d = await api.get<PlanDoc>(`${V1}/plans/${id}`)
      setDoc(d)
      const lid = sp.get('layout') || String(d.plan.floors?.[0]?.layouts?.find(x => x.current)?.id || d.plan.floors?.[0]?.layouts?.[0]?.id)
      const lay = await api.get<Layout>(`${V1}/layouts/${lid}`)
      setL(lay)
      const s = await api.get<{ floors: LayoutSum[] }>(`${V1}/plans/${id}/summary?layout=${lid}`)
      setSum(s.floors.find(f => f.layout_id === lay.id) || null)
    })().catch(() => {})
  }, [id]) // eslint-disable-line react-hooks/exhaustive-deps
  const svg = useMemo(() => l ? planSVGString(fixDrawing(l.data), { px: 1400, units: settings?.units, showDims: true }) : '', [l, settings?.units])
  useEffect(() => { if (svg && sum) { const t = window.setTimeout(() => window.print(), 500); return () => window.clearTimeout(t) } }, [svg, sum])
  if (!doc || !l) return <Loading />
  const floor = doc.plan.floors?.find(f => f.id === l.floor_id)
  return <div className="print-page">
    <h1>{doc.plan.name}</h1>
    <p className="muted">{floor?.name} · {l.name} · {area(l.area_cm2)}{doc.plan.address ? ` · ${doc.plan.address}` : ''}</p>
    <div className="print-plan" dangerouslySetInnerHTML={{ __html: svg.replace(/^<\?xml[^>]*>/, '') }} />
    {sum && <table className="tbl print-tbl"><thead><tr><th>Room</th><th>Size</th><th>Floor</th><th>Walls to paint</th></tr></thead>
      <tbody>{sum.rooms.map(r => <tr key={r.id}><td>{r.name}</td><td>{r.size || '—'}</td><td>{area(r.area_cm2)}</td><td>{area(r.wall_cm2)}</td></tr>)}</tbody></table>}
    {sum && sum.to_buy > 0 && <p>To buy: {sum.items.filter(i => i.buy).map(i => i.name).join(', ')} ({money(sum.budget, doc.plan.currency)}). Ceiling {len(sum.ceiling_cm)}.</p>}
  </div>
}
