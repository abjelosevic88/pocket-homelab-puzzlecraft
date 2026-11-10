// Puzzle books: puzzles put together to print (answers at the back) or to
// share as one link.
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useOnChange } from '../lib/data'
import { ago } from '../lib/format'
import { KIND, LEVEL_NAME } from '../lib/puzzlecraft'
import type { Book, Puzzle } from '../lib/types'
import { Icon } from '../components/icons'
import { useCrumb } from '../components/Layout'
import { ShareBox } from '../components/ShareBox'
import { Confirm, Empty, Field, Loading, Modal, Segmented, SettingRow, Spinner, Switch, useToast } from '../components/ui'
import { KindIco } from './Today'

export default function Books() {
  const [sp, setSp] = useSearchParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [books, setBooks] = useState<Book[] | null>(null)
  const [title, setTitle] = useState('')
  const load = useCallback(() => { api.get<{ books: Book[] }>(`${V1}/books`).then(r => setBooks(r.books)).catch(() => {}) }, [])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  const creating = sp.get('new') === '1'
  const create = async () => {
    try { const b = await api.post<Book>(`${V1}/books`, { title: title.trim() || 'Puzzle book' }); announceChange(); navigate(`/books/${b.id}`) } catch (e) { toast((e as Error).message, 'err') }
  }
  return (
    <div className="page">
      <div className="page-head">
        <div className="grow"><h2 className="page-title">Puzzle books</h2><p className="muted">Puzzles together: print them as a booklet with the answers at the back, or share one link to all of them.</p></div>
        <button type="button" className="btn primary" onClick={() => setSp({ new: '1' })}><Icon name="plus" size={15} />New book</button>
      </div>
      {!books ? <Loading /> : books.length === 0 ? <Empty title="No books yet" icon="book-open">Make one for a long journey, a rainy day or a quiz night.</Empty> :
        <div className="pz-cards">{books.map(b => (
          <Link key={b.id} to={`/books/${b.id}`} className="pz-card book">
            <div className="pz-card-top"><span className="kind-ico"><Icon name="book-open" size={18} /></span><span className="muted small grow">{b.items.length} puzzles</span>{b.share_on && <span className="tag"><Icon name="link" size={11} />shared</span>}</div>
            <div className="pz-card-title">{b.title}</div>
            <div className="muted small ellipsis">{b.subtitle || b.items.map(i => i.title).join(', ')}</div>
            <div className="pz-card-foot"><span className="book-kinds">{b.items.slice(0, 8).map(i => <span key={i.id} style={{ color: KIND[i.kind]?.tint }}><Icon name={KIND[i.kind]?.icon || 'puzzle'} size={13} /></span>)}</span><span className="spacer" /><span className="muted small">{ago(b.updated_at)}</span></div>
          </Link>
        ))}</div>}
      {creating && <Modal title="New puzzle book" onClose={() => setSp({})} footer={<><button type="button" className="btn" onClick={() => setSp({})}>Cancel</button><button type="button" className="btn primary" onClick={create}>Make it</button></>}>
        <Field label="Name"><input autoFocus value={title} onChange={e => setTitle(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') create() }} placeholder="Rainy day puzzle book" /></Field>
      </Modal>}
    </div>
  )
}

export function BookPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [b, setB] = useState<Book | null>(null)
  const [adding, setAdding] = useState(false)
  const [del, setDel] = useState(false)
  useCrumb(b?.title, '/books')
  const load = useCallback(() => { api.get<Book>(`${V1}/books/${id}`).then(setB).catch(() => {}) }, [id])
  useEffect(() => { load() }, [load])
  const save = async (body: Record<string, unknown>) => {
    try { setB(await api.put<Book>(`${V1}/books/${id}`, body)); announceChange() } catch (e) { toast((e as Error).message, 'err') }
  }
  if (!b) return <Loading />
  const ids = b.items.map(i => i.puzzle_id)
  const move = (i: number, d: number) => { const n = [...ids]; const j = i + d; if (j < 0 || j >= n.length) return; [n[i], n[j]] = [n[j], n[i]]; save({ puzzle_ids: n }) }
  return (
    <div className="page">
      <div className="pp-head">
        <span className="kind-ico"><Icon name="book-open" size={20} /></span>
        <input className="pp-title-input" defaultValue={b.title} onBlur={e => e.target.value !== b.title && save({ title: e.target.value })} aria-label="Name" />
        <a className="btn" href={`/books/${b.id}/print`} target="_blank" rel="noopener"><Icon name="printer" size={15} />Print</a>
        <button type="button" className="btn ghost icon-only" aria-label="Delete the book" title="Delete the book" onClick={() => setDel(true)}><Icon name="trash" size={16} /></button>
      </div>
      <div className="make-cols">
        <div className="make-main stack">
          <section className="card">
            <div className="card-head"><h3>Puzzles</h3><button type="button" className="btn sm" onClick={() => setAdding(true)}><Icon name="plus" size={14} />Add puzzles</button></div>
            {b.items.length === 0 ? <Empty title="An empty book" icon="puzzle">Add some of your puzzles.</Empty> :
              <div className="rows">{b.items.map((it, i) => (
                <div key={it.id} className="row">
                  <span className="muted num">{i + 1}</span><KindIco kind={it.kind} />
                  <Link to={`/puzzles/${it.puzzle_id}`} className="grow ellipsis">{it.title}</Link>
                  <span className="muted small hide-sm">{KIND[it.kind]?.name} · {LEVEL_NAME[it.level] || it.level}</span>
                  <span className="row-actions">
                    <button type="button" className="btn ghost xs" aria-label="Up" disabled={i === 0} onClick={() => move(i, -1)}><Icon name="arrow-up" size={13} /></button>
                    <button type="button" className="btn ghost xs" aria-label="Down" disabled={i === b.items.length - 1} onClick={() => move(i, 1)}><Icon name="arrow-down" size={13} /></button>
                    <button type="button" className="btn ghost xs" aria-label="Take out" onClick={() => save({ puzzle_ids: ids.filter(x => x !== it.puzzle_id) })}><Icon name="x" size={13} /></button>
                  </span>
                </div>
              ))}</div>}
          </section>
          <section className="card pad-card stack">
            <Field label="Under the title"><input defaultValue={b.subtitle} onBlur={e => e.target.value !== b.subtitle && save({ subtitle: e.target.value })} placeholder="For the long drive to the seaside" /></Field>
            <SettingRow title="Answers" help="Printed at the back of the book, or not at all."><Segmented size="sm" value={b.answers} onChange={v => save({ answers: v })} options={[{ value: 'back', label: 'At the back' }, { value: 'none', label: 'None' }]} /></SettingRow>
            <SettingRow title="Large print" help="Bigger squares and letters."><Switch checked={b.large} onChange={v => save({ large: v })} label="Large print" /></SettingRow>
          </section>
        </div>
        <aside className="make-side">
          <ShareBox what="book" id={b.id} on={b.share_on} url={b.share_url || ''} title={b.title} onToggle={on => save({ share_on: on })} onRenew={() => save({ renew: true })} />
        </aside>
      </div>
      {adding && <AddPuzzles have={ids} onClose={() => setAdding(false)} onAdd={add => { save({ puzzle_ids: [...ids, ...add] }); setAdding(false) }} />}
      {del && <Confirm title="Delete this book?" message="The puzzles stay; only the book goes." confirmLabel="Delete" onCancel={() => setDel(false)} onConfirm={async () => { await api.del(`${V1}/books/${b.id}`); announceChange(); navigate('/books') }} />}
    </div>
  )
}

/** Pick puzzles (for a book or a stage). */
export function AddPuzzles({ have, onClose, onAdd, single }: { have: number[]; onClose: () => void; onAdd: (ids: number[]) => void; single?: boolean }) {
  const [list, setList] = useState<Puzzle[] | null>(null)
  const [pick, setPick] = useState<number[]>([])
  const [q, setQ] = useState('')
  useEffect(() => { api.get<{ puzzles: Puzzle[] }>(`${V1}/puzzles`).then(r => setList(r.puzzles)).catch(() => setList([])) }, [])
  const shown = (list || []).filter(p => !have.includes(p.id) && (!q || p.title.toLowerCase().includes(q.toLowerCase())))
  return (
    <Modal title={single ? 'Choose a puzzle' : 'Add puzzles'} onClose={onClose} size="lg" footer={single ? undefined : <><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={!pick.length} onClick={() => onAdd(pick)}>Add {pick.length || ''}</button></>}>
      <div className="add-search"><Icon name="search" size={15} /><input autoFocus value={q} onChange={e => setQ(e.target.value)} placeholder="Find a puzzle" /></div>
      {list === null ? <Spinner /> : shown.length === 0 ? <p className="muted">No more puzzles to add. <Link to="/new">Make one</Link>.</p> :
        <div className="pick-list tall">{shown.map(p => (
          <button key={p.id} type="button" className={`pick-row ${pick.includes(p.id) ? 'on' : ''}`} onClick={() => single ? onAdd([p.id]) : setPick(pick.includes(p.id) ? pick.filter(x => x !== p.id) : [...pick, p.id])}>
            {!single && <span className={`check-box ${pick.includes(p.id) ? 'on' : ''}`}>{pick.includes(p.id) && <Icon name="check" size={12} />}</span>}
            <KindIco kind={p.kind} /><span className="grow ellipsis">{p.title}</span><span className="muted small">{p.size_text}</span>
          </button>
        ))}</div>}
    </Modal>
  )
}
