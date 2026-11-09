// How each kind of piece looks from above, drawn in its own frame:
// x across (−w/2…w/2), y front to back (−d/2 back … +d/2 front).
import type { ReactNode } from 'react'

export interface SymProps { type: string; w: number; d: number; fill: string; line: string; detail?: boolean }

const ns = { vectorEffect: 'non-scaling-stroke' as const }

export function PieceSymbol({ type, w, d, fill, line, detail = true }: SymProps): ReactNode {
  const x0 = -w / 2, y0 = -d / 2
  const r = Math.min(w, d)
  const S = { stroke: line, strokeWidth: 1.1, ...ns }
  const thin = { stroke: line, strokeWidth: 0.8, ...ns }
  const line0 = { ...thin, fill: 'none' }
  const box = (rx = Math.min(4, r * 0.08)) => <rect x={x0} y={y0} width={w} height={d} rx={rx} fill={fill} {...S} />
  switch (type) {
    case 'sofa': case 'armchair': {
      const back = Math.min(d * 0.28, 24), arm = Math.min(w * 0.12, 18)
      const seats = type === 'armchair' ? 1 : Math.max(2, Math.round((w - 2 * arm) / 70))
      return <g>
        {box(Math.min(10, r * 0.15))}
        <rect x={x0 + 1} y={y0 + 1} width={w - 2} height={back} rx={Math.min(6, back / 2)} fill={line} fillOpacity={0.14} stroke="none" />
        <rect x={x0} y={y0} width={arm} height={d} rx={Math.min(8, arm / 2)} fill={line} fillOpacity={0.1} {...thin} />
        <rect x={-x0 - arm} y={y0} width={arm} height={d} rx={Math.min(8, arm / 2)} fill={line} fillOpacity={0.1} {...thin} />
        {detail && Array.from({ length: seats - 1 }, (_, i) => { const x = x0 + arm + (w - 2 * arm) * (i + 1) / seats; return <line key={i} x1={x} y1={y0 + back} x2={x} y2={-y0 - 4} {...line0} /> })}
      </g>
    }
    case 'sofa_l': {
      const dd = Math.min(95, d * 0.55), cw = Math.min(100, w * 0.38), back = 22
      const p = `M${x0},${y0} H${-x0} V${-y0} H${-x0 - cw} V${y0 + dd} H${x0} Z`
      return <g>
        <path d={p} fill={fill} strokeLinejoin="round" {...S} />
        <path d={`M${x0 + 1},${y0 + 1} H${-x0 - 1} V${-y0 - 1} H${-x0 - back} V${y0 + back} H${x0 + 1} Z`} fill={line} fillOpacity={0.14} stroke="none" />
        {detail && <><line x1={-x0 - cw} y1={y0 + back} x2={-x0 - cw} y2={y0 + dd} {...line0} />
          <line x1={(x0 + (-x0 - cw)) / 2} y1={y0 + back} x2={(x0 + (-x0 - cw)) / 2} y2={y0 + dd} {...line0} />
          <line x1={-x0 - cw} y1={(y0 + dd + -y0) / 2 + 10} x2={-x0 - back} y2={(y0 + dd + -y0) / 2 + 10} {...line0} /></>}
      </g>
    }
    case 'bed_double': case 'bed_single': case 'crib': {
      const n = type === 'bed_double' ? 2 : 1
      const pw = (w - 12 - (n - 1) * 6) / n, ph = Math.min(32, d * 0.16)
      return <g>
        {box(3)}
        <rect x={x0} y={y0} width={w} height={Math.min(8, d * 0.05)} fill={line} fillOpacity={0.25} stroke="none" />
        {type !== 'crib' && Array.from({ length: n }, (_, i) => <rect key={i} x={x0 + 6 + i * (pw + 6)} y={y0 + 10} width={pw} height={ph} rx={6} fill="#fff" fillOpacity={0.75} {...thin} />)}
        {type !== 'crib' && <path d={`M${x0 + 1},${y0 + d * 0.34} H${-x0 - 1}`} {...line0} />}
        {type !== 'crib' && detail && <path d={`M${-x0 - w * 0.32},${y0 + d * 0.34} L${-x0 - 1},${y0 + d * 0.34 + w * 0.2}`} {...line0} />}
        {type === 'crib' && detail && Array.from({ length: Math.floor(w / 9) }, (_, i) => <line key={i} x1={x0 + 4 + i * 9} y1={y0 + 2} x2={x0 + 4 + i * 9} y2={y0 + 6} {...line0} />)}
      </g>
    }
    case 'coffee_table': case 'dining_table': case 'desk': case 'bench': case 'garden_table': case 'island': case 'nightstand':
      return <g>{box(type === 'bench' ? 4 : 3)}{detail && w > 30 && d > 25 && <rect x={x0 + 4} y={y0 + 4} width={w - 8} height={d - 8} rx={2} {...line0} />}</g>
    case 'table_round':
      return <g><ellipse cx={0} cy={0} rx={w / 2} ry={d / 2} fill={fill} {...S} />{detail && <ellipse cx={0} cy={0} rx={w / 2 - 5} ry={d / 2 - 5} {...line0} />}</g>
    case 'chair':
      return <g><rect x={x0 + 2} y={y0 + 2} width={w - 4} height={d - 4} rx={5} fill={fill} {...S} /><rect x={x0 + 2} y={y0 + 2} width={w - 4} height={Math.min(9, d * 0.18)} rx={3} fill={line} fillOpacity={0.3} stroke="none" /></g>
    case 'office_chair':
      return <g><circle cx={0} cy={2} r={r * 0.4} fill={fill} {...S} /><path d={`M${-r * 0.36},${-r * 0.22} Q0,${-r * 0.58} ${r * 0.36},${-r * 0.22}`} fill="none" stroke={line} strokeWidth={2.2} {...ns} /></g>
    case 'wardrobe': case 'dresser': case 'shelf': case 'bookcase': case 'counter': case 'tv_unit': case 'fridge': case 'dishwasher':
      return <g>
        {box(1.5)}
        <line x1={x0 + 1} y1={-y0 - Math.min(5, d * 0.12)} x2={-x0 - 1} y2={-y0 - Math.min(5, d * 0.12)} {...line0} />
        {type === 'wardrobe' && detail && <line x1={0} y1={y0 + 2} x2={0} y2={-y0 - 2} {...line0} />}
        {type === 'wardrobe' && detail && <line x1={x0 + 4} y1={0} x2={-x0 - 4} y2={0} strokeDasharray="4 4" {...line0} />}
        {type === 'bookcase' && detail && <line x1={x0 + 3} y1={0} x2={-x0 - 3} y2={0} {...line0} />}
        {type === 'tv_unit' && <rect x={-w * 0.36} y={y0 + d * 0.3} width={w * 0.72} height={Math.max(3, d * 0.12)} rx={1} fill="#2b2f36" stroke="none" />}
        {type === 'fridge' && detail && <line x1={x0 + w * 0.18} y1={-y0 - 2} x2={x0 + w * 0.18} y2={-y0 - 8} stroke={line} strokeWidth={1.6} {...ns} />}
        {type === 'dishwasher' && detail && <rect x={-w * 0.2} y={-y0 - 9} width={w * 0.4} height={3} fill={line} fillOpacity={0.4} stroke="none" />}
      </g>
    case 'sink_unit':
      return <g>{box(1.5)}<rect x={-w * 0.32} y={y0 + d * 0.22} width={w * 0.64} height={d * 0.55} rx={6} fill="#fff" fillOpacity={0.6} {...thin} /><circle cx={0} cy={y0 + d * 0.12} r={2.4} fill={line} /></g>
    case 'stove': {
      const rr = Math.min(w, d) * 0.16
      return <g>{box(1.5)}{[[-1, -1], [1, -1], [-1, 1], [1, 1]].map(([a, b], i) => <circle key={i} cx={a * w * 0.22} cy={b * d * 0.2 - d * 0.04} r={rr * (i % 3 ? 0.85 : 1)} {...line0} />)}</g>
    }
    case 'washer':
      return <g>{box(2)}<circle cx={0} cy={d * 0.06} r={Math.min(w, d) * 0.32} {...line0} /><rect x={x0 + 4} y={y0 + 3} width={w * 0.4} height={4} rx={1} fill={line} fillOpacity={0.3} stroke="none" /></g>
    case 'toilet':
      return <g><rect x={x0 + 2} y={y0} width={w - 4} height={d * 0.26} rx={3} fill={fill} {...S} /><ellipse cx={0} cy={y0 + d * 0.62} rx={w * 0.44} ry={d * 0.36} fill={fill} {...S} /><ellipse cx={0} cy={y0 + d * 0.64} rx={w * 0.27} ry={d * 0.22} {...line0} /></g>
    case 'bathtub':
      return <g>{box(4)}<rect x={x0 + 6} y={y0 + 6} width={w - 12} height={d - 12} rx={Math.min(18, d * 0.3)} fill="#fff" fillOpacity={0.55} {...thin} /><circle cx={x0 + 16} cy={0} r={2.5} fill={line} /></g>
    case 'shower':
      return <g>{box(1)}<line x1={x0} y1={y0} x2={-x0} y2={-y0} {...line0} /><line x1={-x0} y1={y0} x2={x0} y2={-y0} {...line0} /><circle cx={0} cy={0} r={4} fill={fill} {...thin} /></g>
    case 'basin':
      return <g>{box(3)}<ellipse cx={0} cy={d * 0.06} rx={w * 0.34} ry={d * 0.3} fill="#fff" fillOpacity={0.6} {...thin} /><circle cx={0} cy={y0 + d * 0.14} r={2} fill={line} /></g>
    case 'rug':
      return <g><rect x={x0} y={y0} width={w} height={d} rx={3} fill={fill} fillOpacity={0.55} stroke={line} strokeOpacity={0.5} strokeWidth={0.8} {...ns} />{detail && <rect x={x0 + 8} y={y0 + 8} width={Math.max(0, w - 16)} height={Math.max(0, d - 16)} rx={2} strokeDasharray="5 4" {...line0} strokeOpacity={0.6} />}</g>
    case 'lamp':
      return <g><circle cx={0} cy={0} r={r / 2} fill={fill} fillOpacity={0.5} {...S} /><circle cx={0} cy={0} r={r * 0.15} fill={line} /></g>
    case 'plant': {
      const R = r / 2
      return <g>
        <circle cx={0} cy={0} r={R * 0.55} fill="#a0785a" {...thin} />
        {Array.from({ length: 7 }, (_, i) => { const a = i * 360 / 7; return <ellipse key={i} cx={0} cy={-R * 0.52} rx={R * 0.22} ry={R * 0.48} transform={`rotate(${a})`} fill={fill} fillOpacity={0.9} {...thin} /> })}
      </g>
    }
    case 'piano':
      return <g>{box(2)}<rect x={x0 + 3} y={-y0 - d * 0.3} width={w - 6} height={d * 0.26} fill="#fff" fillOpacity={0.85} {...thin} />{detail && Array.from({ length: Math.floor((w - 6) / 6) }, (_, i) => <line key={i} x1={x0 + 3 + i * 6} y1={-y0 - d * 0.3} x2={x0 + 3 + i * 6} y2={-y0 - d * 0.04} {...line0} strokeOpacity={0.5} />)}</g>
    case 'fireplace':
      return <g>{box(2)}<path d={`M${-w * 0.3},${-y0} V${0} A${w * 0.3},${d * 0.3} 0 0 1 ${w * 0.3},0 V${-y0}`} {...line0} /></g>
    case 'radiator':
      return <g>{box(1)}{detail && Array.from({ length: Math.max(2, Math.floor(w / 8)) }, (_, i) => <line key={i} x1={x0 + 4 + i * 8} y1={y0 + 1} x2={x0 + 4 + i * 8} y2={-y0 - 1} {...line0} />)}</g>
    case 'tv':
      return <rect x={x0} y={y0} width={w} height={d} rx={1} fill={fill} {...S} />
    case 'stairs': {
      const n = Math.max(3, Math.round(w / 26))
      return <g>{box(0)}{Array.from({ length: n - 1 }, (_, i) => <line key={i} x1={x0 + w * (i + 1) / n} y1={y0} x2={x0 + w * (i + 1) / n} y2={-y0} {...line0} />)}
        <path d={`M${x0 + 8},0 H${-x0 - 14} M${-x0 - 22},-6 L${-x0 - 14},0 L${-x0 - 22},6`} fill="none" stroke={line} strokeWidth={1.4} {...ns} /></g>
    }
    case 'car':
      return <g><rect x={x0} y={y0} width={w} height={d} rx={Math.min(30, w * 0.25)} fill={fill} {...S} /><rect x={x0 + w * 0.12} y={y0 + d * 0.22} width={w * 0.76} height={d * 0.18} rx={8} fill="#fff" fillOpacity={0.5} {...thin} /><rect x={x0 + w * 0.12} y={y0 + d * 0.62} width={w * 0.76} height={d * 0.12} rx={6} fill="#fff" fillOpacity={0.4} {...thin} /></g>
    case 'bike':
      return <g><ellipse cx={0} cy={y0 + d * 0.2} rx={w * 0.1} ry={d * 0.18} {...line0} /><ellipse cx={0} cy={-y0 - d * 0.2} rx={w * 0.1} ry={d * 0.18} {...line0} /><line x1={0} y1={y0 + d * 0.2} x2={0} y2={-y0 - d * 0.2} stroke={line} strokeWidth={2} {...ns} /><line x1={-w * 0.4} y1={y0 + d * 0.3} x2={w * 0.4} y2={y0 + d * 0.3} stroke={line} strokeWidth={1.6} {...ns} /></g>
  }
  return <g>{box(2)}{detail && <path d={`M${x0 + 3},${y0 + 3} L${-x0 - 3},${-y0 - 3} M${-x0 - 3},${y0 + 3} L${x0 + 3},${-y0 - 3}`} {...line0} strokeOpacity={0.5} />}</g>
}
