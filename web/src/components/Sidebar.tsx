// Left column: new (and the other ways to start), the sections, recent
// puzzles and the escape rooms coming up.
import { useCallback, useEffect, useState } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { useOnChange } from '../lib/data'
import { KIND } from '../lib/puzzlecraft'
import type { Kind } from '../lib/types'
import { fmtDate } from '../lib/format'
import { Icon } from './icons'
import { createItems, type CreateKind } from './Layout'
import { type Anchor, Menu, rectOf } from './ui'

interface Side {
  puzzles: number; recent: { id: number; title: string; kind: Kind }[]; adventures: { id: number; title: string; kind: string; date: string; live: number }[]
  adventures_n: number; books: number; wordlists: number; streak: number; daily_left: number
}

export function Sidebar({ onCreate, onNavigate }: { onCreate: (k: CreateKind) => void; onNavigate: () => void }) {
  const { status, settings } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [menu, setMenu] = useState<Anchor | null>(null)
  const [side, setSide] = useState<Side | null>(null)
  const load = useCallback(() => { api.get<Side>(`${V1}/home`).then(setSide).catch(() => {}) }, [])
  useEffect(() => { load() }, [load, loc.pathname])
  useOnChange(load)
  const title = settings?.title || status?.title || 'Pocket Puzzlecraft'
  const path = loc.pathname
  const nav: { to: string; label: string; icon: string; active: boolean; count?: number; hint?: string }[] = [
    { to: '/', label: 'Today', icon: 'sun', active: path === '/' || path.startsWith('/today'), count: side?.daily_left || undefined, hint: 'Puzzles of the day still to solve' },
    { to: '/puzzles', label: 'Puzzles', icon: 'puzzle', active: path.startsWith('/puzzles') || path === '/new', count: side?.puzzles || undefined, hint: 'Your puzzles' },
    { to: '/adventures', label: 'Escape rooms & hunts', icon: 'key', active: path.startsWith('/adventures'), count: side?.adventures_n || undefined },
    { to: '/books', label: 'Puzzle books', icon: 'book-open', active: path.startsWith('/books'), count: side?.books || undefined },
    { to: '/words', label: 'Word lists', icon: 'list', active: path.startsWith('/words'), count: side?.wordlists || undefined },
    { to: '/stats', label: 'Your times', icon: 'chart', active: path.startsWith('/stats') },
  ]
  return (
    <aside className="sidebar" aria-label="Pocket Puzzlecraft">
      <div className="sb-brand">
        <img src="/favicon.svg" alt="" className="sb-logo" />
        <span className="sb-title">{title !== 'Pocket Puzzlecraft' ? title : <>Pocket <b>Puzzlecraft</b></>}</span>
        {status?.version && <span className="sb-version" title={`Version ${status.version}`}>{/^v?\d+\.\d+/.test(status.version) ? 'v' + status.version.replace(/^v/, '').replace(/[-+].*$/, '') : status.version}</span>}
      </div>
      <div className="create-split">
        <button type="button" className="btn primary create-btn" onClick={() => onCreate('puzzle')} title="New puzzle (N)">
          <Icon name="plus" size={15} strokeWidth={2.2} />New puzzle
        </button>
        <button type="button" className="btn primary create-more" aria-label="More ways to start" aria-haspopup="menu" aria-expanded={!!menu} onClick={e => setMenu(menu ? null : rectOf(e.currentTarget))}>
          <Icon name="chevron-down" size={15} />
        </button>
      </div>
      {menu && <Menu anchor={menu} items={createItems(onCreate)} onClose={() => setMenu(null)} width={270} />}

      <nav className="sb-nav" aria-label="Sections">
        {nav.map(n => (
          <NavLink key={n.to} to={n.to} onClick={onNavigate} title={n.hint} className={() => `sb-nav-item ${n.active ? 'active' : ''}`}>
            <Icon name={n.icon} size={16} />{n.label}
            {!!n.count && <span className="sb-count" title={n.hint}>{n.count}</span>}
          </NavLink>
        ))}
      </nav>

      {(side?.streak || 0) > 1 && <div className="sb-streak" title="Days in a row with a puzzle of the day solved"><Icon name="flame" size={15} />{side!.streak} days in a row</div>}

      {(side?.recent.length || 0) > 0 && <section className="sb-sec">
        <h4 className="sb-h">Recent</h4>
        {side!.recent.slice(0, 6).map(p => (
          <button key={p.id} type="button" className={`sb-row sb-list ${path === `/puzzles/${p.id}` ? 'active' : ''}`} onClick={() => { onNavigate(); navigate(`/puzzles/${p.id}`) }}>
            <span className="sb-place-ico" aria-hidden="true" style={{ color: KIND[p.kind]?.tint }}><Icon name={KIND[p.kind]?.icon || 'puzzle'} size={14} /></span>
            <span className="grow ellipsis">{p.title}</span>
          </button>
        ))}
      </section>}

      {(side?.adventures.length || 0) > 0 && <section className="sb-sec">
        <h4 className="sb-h">Coming up</h4>
        {side!.adventures.map(a => (
          <button key={a.id} type="button" className={`sb-row sb-list ${path.startsWith(`/adventures/${a.id}`) ? 'active' : ''}`} onClick={() => { onNavigate(); navigate(`/adventures/${a.id}`) }}>
            <span className="sb-place-ico" aria-hidden="true"><Icon name={a.kind === 'hunt' ? 'map' : 'key'} size={14} /></span>
            <span className="grow ellipsis">{a.title}</span>
            {a.live > 0 ? <span className="sb-live" title="Playing right now">live</span> : a.date ? <span className="muted small">{fmtDate(a.date, false)}</span> : null}
          </button>
        ))}
      </section>}

      <div className="sb-foot">
        <Link to="/settings" className="sb-foot-link" onClick={onNavigate}><Icon name="settings" size={15} />Settings</Link>
        <button type="button" className="sb-foot-link" onClick={() => { onNavigate(); window.dispatchEvent(new CustomEvent('ppzc:shortcuts')) }}><Icon name="keyboard" size={15} />Shortcuts</button>
      </div>
    </aside>
  )
}
