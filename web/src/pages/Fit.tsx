// Will it fit? A piece (yours, or any size) through a way in: doors,
// hallways, corners, stairs, a lift — step by step, with how to carry it.
import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { FitResult, FitStep, Piece, Plan, Route, RouteStep, StepKind } from '../lib/types'
import { useApp } from '../lib/app-context'
import { len, size } from '../lib/units'
import { Icon } from '../components/icons'
import { Card, Field, Loading, PageHeader, Segmented } from '../components/ui'
import { Select } from '../components/Select'
import { LenInput } from '../components/LenInput'
import { useSize } from '../components/Preview'

export default function Fit() {
  const [sp] = useSearchParams()
  const { settings } = useApp()
  const [pieces, setPieces] = useState<Piece[]>([])
  const [plans, setPlans] = useState<Plan[]>([])
  const [routes, setRoutes] = useState<Route[]>([])
  const [pieceID, setPieceID] = useState(sp.get('piece') || '')
  const [dims, setDims] = useState({ w: Number(sp.get('w')) || 220, d: Number(sp.get('d')) || 95, h: Number(sp.get('h')) || 85 })
  const [routeID, setRouteID] = useState(sp.get('route') || '')
  const [steps, setSteps] = useState<RouteStep[]>([{ kind: 'door', name: 'Front door', width: 86, height: 203 }])
  const [res, setRes] = useState<FitResult | null>(null)
  const [hands, setHands] = useState<number | null>(null)
  useEffect(() => { api.get<Piece[]>(`${V1}/pieces`).then(setPieces).catch(() => {}) }, [])
  useEffect(() => {
    api.get<Plan[]>(`${V1}/plans?lean=1`).then(async ps => {
      setPlans(ps)
      const all = (await Promise.all(ps.map(p => api.get<Route[]>(`${V1}/plans/${p.id}/routes`).catch(() => [] as Route[])))).flat()
      setRoutes(all)
      if (!routeID && all.length) {
        const want = sp.get('plan')
        setRouteID(String((want && all.find(r => String(r.plan_id) === want)?.id) || all[0].id))
      }
    }).catch(() => {})
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
  const piece = pieces.find(p => String(p.id) === pieceID)
  const route = routes.find(r => String(r.id) === routeID)
  const W = piece?.width ?? dims.w, D = piece?.depth ?? dims.d, H = piece?.height ?? dims.h
  const usedSteps = route ? route.steps : steps
  const key = JSON.stringify([W, D, H, usedSteps, hands])
  useEffect(() => {
    const t = window.setTimeout(() => {
      api.post<{ result: FitResult }>(`${V1}/fit`, { width: W, depth: D, height: H, steps: usedSteps.filter(s => s.width > 0), ...(hands != null ? { hands } : {}) }).then(r => setRes(r.result)).catch(() => setRes(null))
    }, 200)
    return () => window.clearTimeout(t)
  }, [key]) // eslint-disable-line react-hooks/exhaustive-deps
  const planName = (id: number) => plans.find(p => p.id === id)?.name || ''
  return <div className="page">
    <PageHeader title="Will it fit?" sub="Through every door, hallway, corner and staircase on the way in — before you buy it" />
    <div className="fit-grid">
      <Card title="The piece">
        <Field label="Which"><Select value={pieceID} onChange={setPieceID} placeholder="Any size (type it below)" clearLabel="Any size" options={pieces.map(p => ({ value: String(p.id), label: p.name, hint: size(p.width, p.depth, p.height) }))} /></Field>
        <div className="grid three">
          <Field label="Width"><LenInput value={W} min={1} disabled={!!piece} onChange={v => setDims({ ...dims, w: v })} /></Field>
          <Field label="Depth"><LenInput value={D} min={1} disabled={!!piece} onChange={v => setDims({ ...dims, d: v })} /></Field>
          <Field label="Height"><LenInput value={H} min={1} disabled={!!piece} onChange={v => setDims({ ...dims, h: v })} /></Field>
        </div>
        <Field label="Room for hands and blankets" help="Taken off every width and height"><Segmented value={String(hands ?? settings?.hands_cm ?? 2)} onChange={v => setHands(Number(v))} options={[{ value: '0', label: 'None' }, { value: '2', label: '2 cm' }, { value: '5', label: '5 cm' }]} /></Field>
      </Card>
      <Card title="The way in">
        <Field label="Measured way in"><Select value={routeID} onChange={setRouteID} placeholder="Measure it here" clearLabel="Measure it here instead" options={routes.map(r => ({ value: String(r.id), label: r.name, hint: planName(r.plan_id), group: planName(r.plan_id) }))} /></Field>
        {!route && <Steps steps={steps} onChange={setSteps} />}
        {route && <ol className="steps compact">{route.steps.map((s, i) => <li key={i}><span className="step-n">{i + 1}</span>{s.name || s.kind} · {len(s.width)}{s.height ? ` × ${len(s.height)}` : ''}{s.turn ? `, turning from ${len(s.turn)}` : ''}</li>)}</ol>}
      </Card>
    </div>
    {!res ? <Loading /> : <div className={`fit-result ${res.ok ? 'ok' : 'bad'}`}>
      <div className="fit-head"><Icon name={res.ok ? 'check-circle' : 'x-circle'} size={26} /><div><h3>{res.ok ? 'It fits' : 'It does not fit'}</h3><p>{res.summary}</p></div></div>
      <div className="fit-steps">{res.steps.map((s, i) => <FitStepCard key={i} n={i + 1} s={s} piece={[W, D, H]} />)}</div>
      <p className="muted small">The sums: a piece goes straight through when two of its sides fit the opening. To turn a corner it swings round the inside corner; the longest piece that can is worked out for its width (the “ladder round a corner” problem). Real life has door handles, skirting boards and banisters: leave a little extra.</p>
    </div>}
  </div>
}

function Steps({ steps, onChange }: { steps: RouteStep[]; onChange: (s: RouteStep[]) => void }) {
  const set = (i: number, p: Partial<RouteStep>) => onChange(steps.map((s, j) => j === i ? { ...s, ...p } : s))
  return <div>
    <ol className="steps">{steps.map((s, i) => <li key={i} className="step">
      <div className="step-head"><span className="step-n">{i + 1}</span>
        <Select value={s.kind} onChange={v => set(i, { kind: v as StepKind })} options={[{ value: 'door', label: 'Door' }, { value: 'hall', label: 'Hallway' }, { value: 'corner', label: 'Corner' }, { value: 'stairs', label: 'Stairs' }, { value: 'lift', label: 'Lift' }]} />
        <button type="button" className="btn ghost icon-only sm" onClick={() => onChange(steps.filter((_, j) => j !== i))} aria-label="Remove"><Icon name="x" size={14} /></button>
      </div>
      <div className="step-fields">
        <label className="insp-f"><span>Width</span><LenInput value={s.width} min={20} onChange={v => set(i, { width: v })} /></label>
        {s.kind !== 'hall' && s.kind !== 'corner' && <label className="insp-f"><span>Height</span><LenInput value={s.height || 200} min={50} onChange={v => set(i, { height: v })} /></label>}
        {s.kind === 'lift' && <label className="insp-f"><span>Depth</span><LenInput value={s.depth || 130} min={40} onChange={v => set(i, { depth: v })} /></label>}
        {s.kind !== 'lift' && s.kind !== 'hall' && <label className="insp-f"><span>{s.kind === 'corner' ? 'Coming from' : 'Turn from a hall'}</span><LenInput value={s.turn || 0} min={0} onChange={v => set(i, { turn: v || undefined })} /></label>}
      </div>
    </li>)}</ol>
    <button type="button" className="btn sm" onClick={() => onChange([...steps, { kind: 'corner', name: '', width: 100, turn: 110 }])}><Icon name="plus" size={14} />Add a step</button>
  </div>
}

/** One step: the opening and the piece's end as it is carried, drawn to the same scale. */
function FitStepCard({ n, s, piece }: { n: number; s: FitStep; piece: [number, number, number] }) {
  const [ref, Wpx] = useSize<HTMLDivElement>([220, 150])
  const H = 150
  const ow = s.step.width, oh = s.step.height || (s.step.kind === 'door' || s.step.kind === 'stairs' ? 200 : s.step.kind === 'lift' ? 210 : 250)
  const carry = s.carry && s.carry[0] > 0 ? s.carry : [Math.min(...piece), [...piece].sort((a, b) => a - b)[1], Math.max(...piece)]
  const pw = carry[1], ph = carry[0]
  const scale = Math.min((Wpx - 20) / Math.max(ow, pw), (H - 20) / Math.max(oh, ph))
  const ox = (Wpx - ow * scale) / 2, oy = H - 10 - oh * scale
  const px = (Wpx - pw * scale) / 2, py = H - 10 - ph * scale
  const name = s.step.name || ({ door: 'Door', hall: 'Hallway', corner: 'Corner', stairs: 'Stairs', lift: 'Lift' } as Record<string, string>)[s.step.kind]
  const drawing = useMemo(() => s.step.kind !== 'corner', [s.step.kind])
  return <div className={`fit-step ${s.ok ? 'ok' : 'bad'}`}>
    <div className="fit-step-head"><span className="step-n">{n}</span><b className="grow">{name}</b><Icon name={s.ok ? 'check' : 'x'} size={16} /></div>
    <div ref={ref} className="fit-pic">{drawing ? <svg viewBox={`0 0 ${Wpx} ${H}`} width={Wpx} height={H} role="img" aria-label={`${name}: the opening and the piece's end`}>
      <rect x={ox} y={oy} width={ow * scale} height={oh * scale} className="fit-open" />
      <rect x={px} y={py} width={pw * scale} height={ph * scale} className="fit-piece" />
      <text x={ox + ow * scale / 2} y={oy - 3} textAnchor="middle" className="fit-lab">{len(ow)} × {len(oh)}</text>
    </svg> : <CornerPic s={s} w={Wpx} h={H} />}</div>
    <p className="small">{s.how}</p>
    {s.ok && <p className="muted small">{len(Math.max(0, s.spare))} to spare</p>}
  </div>
}

function CornerPic({ s, w, h }: { s: FitStep; w: number; h: number }) {
  const a = s.step.turn || s.step.width, b = s.step.width
  const L = Math.max(a, b) * 2.6
  const k = Math.min((w - 16) / L, (h - 16) / L)
  // an L: coming from below (width a), turning right into width b
  const x0 = 8, y0 = h - 8
  const pts = [[0, 0], [a, 0], [a, -(L - b)], [L, -(L - b)], [L, -L], [0, -L]].map(([x, y]) => `${x0 + x * k},${y0 + y * k}`).join(' ')
  return <svg viewBox={`0 0 ${w} ${h}`} width={w} height={h} role="img" aria-label="The corner">
    <polygon points={pts} className="fit-open" />
    {s.max_len ? <text x={x0 + a * k + 6} y={y0 - 6} className="fit-lab">turns up to {len(s.max_len)}</text> : null}
  </svg>
}
