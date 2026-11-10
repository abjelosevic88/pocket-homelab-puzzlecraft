// Your puzzles: every kind, searchable, by tag, shared or archived.
import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useOnChange } from '../lib/data'
import { ago } from '../lib/format'
import { fmtMs, KIND, KINDS, LEVEL_NAME } from '../lib/puzzlecraft'
import type { Kind, Puzzle } from '../lib/types'
import { Icon } from '../components/icons'
import { Empty, Loading, useDebounce } from '../components/ui'
import { KindIco } from './Today'

interface List { puzzles: Puzzle[]; kinds: Record<string, number>; tags: Record<string, number> }

export default function Puzzles() {
  const [sp, setSp] = useSearchParams()
  const kind = sp.get('kind') || ''
  const tag = sp.get('tag') || ''
  const view = sp.get('view') || ''
  const [q, setQ] = useState(sp.get('q') || '')
  const dq = useDebounce(q, 250)
  const [list, setList] = useState<List | null>(null)
  const load = useCallback(() => {
    const p = new URLSearchParams()
    if (kind) p.set('kind', kind)
    if (tag) p.set('tag', tag)
    if (dq) p.set('q', dq)
    if (view === 'shared') p.set('shared', '1')
    if (view === 'archived') p.set('archived', '1')
    api.get<List>(`${V1}/puzzles?${p}`).then(setList).catch(() => {})
  }, [kind, tag, dq, view])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  const set = (k: string, v: string) => { const n = new URLSearchParams(sp); if (v) n.set(k, v); else n.delete(k); setSp(n, { replace: true }) }
  const total = list ? Object.values(list.kinds).reduce((a, b) => a + b, 0) : 0
  return (
    <div className="page">
      <div className="page-head">
        <div className="grow"><h2 className="page-title">Puzzles</h2><p className="muted">{total ? `${total} ${total === 1 ? 'puzzle' : 'puzzles'}` : 'Crosswords, word searches, sudoku, codes, quizzes and more'}</p></div>
        <Link to="/new" className="btn primary"><Icon name="plus" size={15} />New puzzle</Link>
      </div>
      <div className="list-tools">
        <div className="add-search grow"><Icon name="search" size={15} /><input value={q} onChange={e => { setQ(e.target.value); set('q', e.target.value) }} placeholder="Find a puzzle" aria-label="Find a puzzle" /></div>
        <div className="chips">
          {[['', 'All'], ['shared', 'Shared'], ['archived', 'Archived']].map(([v, l]) => <button key={v} type="button" className={`chip ${view === v ? 'on' : ''}`} onClick={() => set('view', v)}>{l}</button>)}
        </div>
      </div>
      <div className="chips kind-chips scroll-x">
        <button type="button" className={`chip ${!kind ? 'on' : ''}`} onClick={() => set('kind', '')}>Every kind</button>
        {KINDS.filter(k => list?.kinds[k.kind]).map(k => <button key={k.kind} type="button" className={`chip ${kind === k.kind ? 'on' : ''}`} onClick={() => set('kind', kind === k.kind ? '' : k.kind)}><Icon name={k.icon} size={13} />{k.name}<span className="muted">{list!.kinds[k.kind]}</span></button>)}
      </div>
      {list && Object.keys(list.tags).length > 0 && <div className="chips scroll-x">
        {Object.entries(list.tags).sort((a, b) => b[1] - a[1]).map(([t, n]) => <button key={t} type="button" className={`chip ${tag === t ? 'on' : ''}`} onClick={() => set('tag', tag === t ? '' : t)}>#{t}<span className="muted">{n}</span></button>)}
      </div>}
      {!list ? <Loading /> : list.puzzles.length === 0 ? (
        <Empty title={q || kind || tag || view ? 'Nothing found' : 'No puzzles yet'} icon="puzzle">
          {!(q || kind || tag || view) && <Link to="/new" className="btn primary sm">Make your first puzzle</Link>}
        </Empty>
      ) : (
        <div className="pz-cards">
          {list.puzzles.map(p => <PuzzleCard key={p.id} p={p} />)}
        </div>
      )}
    </div>
  )
}

export function PuzzleCard({ p }: { p: Puzzle }) {
  const k = KIND[p.kind as Kind]
  return (
    <Link to={`/puzzles/${p.id}`} className="pz-card" style={{ ['--tint' as any]: k?.tint }}>
      <div className="pz-card-top">
        <KindIco kind={p.kind} size={18} />
        <span className="muted small grow">{k?.name}</span>
        {p.pinned && <Icon name="pin" size={13} className="muted" />}
        {p.share_on && <span className="tag" title="Anyone with the link can play"><Icon name="link" size={11} />shared</span>}
      </div>
      <div className="pz-card-title">{p.title}</div>
      <div className="muted small">{p.size_text}{p.level ? ` · ${LEVEL_NAME[p.level] || p.level}` : ''}</div>
      <div className="pz-card-foot">
        {p.ready ? <span className="ok small"><Icon name="check-circle" size={13} />Ready</span> : <span className="warn-text small"><Icon name="alert" size={13} />{p.problems} to fix</span>}
        <span className="spacer" />
        {p.solves > 0 && <span className="muted small" title="Solved, and the best time">{p.solves}× · {fmtMs(p.best_ms)}</span>}
        {!p.solves && <span className="muted small">{ago(p.updated_at)}</span>}
      </div>
    </Link>
  )
}
