// Left column: a new plan (and the other ways to start), the sections,
// then your plans and what is coming up on their checklists.
import { useCallback, useEffect, useState } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { useOnChange } from '../lib/data'
import { area } from '../lib/units'
import type { Plan, Todo } from '../lib/types'
import { Icon } from './icons'
import { createItems, type CreateKind } from './Layout'
import { type Anchor, Menu, rectOf } from './ui'

interface Side { plans: Plan[]; todos: Todo[]; pieces: number }

export function Sidebar({ onCreate, onNavigate }: { onCreate: (k: CreateKind) => void; onNavigate: () => void }) {
  const { status, settings } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [menu, setMenu] = useState<Anchor | null>(null)
  const [side, setSide] = useState<Side | null>(null)
  const load = useCallback(() => { api.get<Side>(`${V1}/home`).then(setSide).catch(() => {}) }, [])
  useEffect(() => { load() }, [load, loc.pathname])
  useOnChange(load)
  const title = settings?.title || status?.title || 'Pocket Rooms'
  const path = loc.pathname
  const nav: { to: string; label: string; icon: string; active: boolean; count?: number; hint?: string }[] = [
    { to: '/', label: 'Plans', icon: 'grid', active: path === '/' || path.startsWith('/plans'), count: side?.plans.length || undefined, hint: 'Plans' },
    { to: '/furniture', label: 'Furniture', icon: 'tag', active: path.startsWith('/furniture'), count: side?.pieces || undefined, hint: 'Measured pieces' },
    { to: '/fit', label: 'Will it fit?', icon: 'door', active: path.startsWith('/fit') },
  ]
  const todos = (side?.todos || []).slice(0, 4)
  return (
    <aside className="sidebar" aria-label="Pocket Rooms">
      <div className="sb-brand">
        <img src="/favicon.svg" alt="" className="sb-logo" />
        <span className="sb-title">{title !== 'Pocket Rooms' ? title : <>Pocket <b>Rooms</b></>}</span>
        {status?.version && <span className="sb-version" title={`Version ${status.version}`}>{/^v?\d+\.\d+/.test(status.version) ? 'v' + status.version.replace(/^v/, '').replace(/[-+].*$/, '') : status.version}</span>}
      </div>
      <div className="create-split">
        <button type="button" className="btn primary create-btn" onClick={() => onCreate('plan')} title="New plan (N)">
          <Icon name="plus" size={15} strokeWidth={2.2} />New plan
        </button>
        <button type="button" className="btn primary create-more" aria-label="More ways to start" aria-haspopup="menu" aria-expanded={!!menu} onClick={e => setMenu(menu ? null : rectOf(e.currentTarget))}>
          <Icon name="chevron-down" size={15} />
        </button>
      </div>
      {menu && <Menu anchor={menu} items={createItems(onCreate)} onClose={() => setMenu(null)} width={260} />}

      <nav className="sb-nav" aria-label="Sections">
        {nav.map(n => (
          <NavLink key={n.to} to={n.to} onClick={onNavigate} title={n.hint} className={() => `sb-nav-item ${n.active ? 'active' : ''}`}>
            <Icon name={n.icon} size={16} />{n.label}
            {!!n.count && <span className="sb-count" title={n.hint}>{n.count}</span>}
          </NavLink>
        ))}
      </nav>

      {(side?.plans.length || 0) > 0 && <section className="sb-sec">
        <h4 className="sb-h">Your plans</h4>
        {side!.plans.slice(0, 7).map(p => (
          <button key={p.id} type="button" className={`sb-row sb-list ${path === `/plans/${p.id}` ? 'active' : ''}`} onClick={() => { onNavigate(); navigate(`/plans/${p.id}`) }}>
            <span className="sb-place-ico" aria-hidden="true"><Icon name={p.kind === 'house' ? 'home' : p.kind === 'office' ? 'briefcase' : p.kind === 'garden' ? 'tree' : 'building'} size={14} /></span>
            <span className="grow ellipsis">{p.name}</span>
            <span className="muted small">{area(p.area_cm2)}</span>
          </button>
        ))}
      </section>}

      {todos.length > 0 && <section className="sb-sec">
        <h4 className="sb-h">To do</h4>
        {todos.map(t => (
          <button key={t.id} type="button" className="sb-row sb-list" onClick={() => { onNavigate(); navigate(`/plans/${t.plan_id}?tab=checklist`) }} title={t.plan_name}>
            <span className="sb-place-ico" aria-hidden="true"><Icon name="circle" size={13} /></span>
            <span className="grow ellipsis">{t.title}</span>
          </button>
        ))}
      </section>}

      <div className="sb-foot">
        <Link to="/settings" className="sb-foot-link" onClick={onNavigate}><Icon name="settings" size={15} />Settings</Link>
        <button type="button" className="sb-foot-link" onClick={() => { onNavigate(); window.dispatchEvent(new CustomEvent('prms:shortcuts')) }}><Icon name="keyboard" size={15} />Shortcuts</button>
      </div>
    </aside>
  )
}
