// The start page: today's three puzzles and your run of days, what you are
// in the middle of, escape rooms coming up, and who solved what you shared.
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { useOnChange } from '../lib/data'
import { addDays, ago, fmtDate, todayISO, weekday } from '../lib/format'
import { ADV_KIND, fmtMs, KIND, LEVEL_NAME } from '../lib/puzzlecraft'
import type { Adventure, Kind, PlayView, Puzzle } from '../lib/types'
import { Icon } from '../components/icons'
import { useCrumb } from '../components/Layout'
import { Player } from '../components/Player'
import { Empty, Loading } from '../components/ui'

interface TodayDoc {
  date: string; streak: number; best_streak: number
  daily: { kind: Kind; kind_name: string; title: string; level: string; size_text: string; started?: boolean; finished_at?: string | null; ms?: number; hints?: number }[]
  going: { id: number; title: string; kind: Kind; kind_name: string; last_at: string }[]
  recent: Puzzle[]; adventures: Adventure[]
  results: { what: string; ref_id: number; title: string; player: string; ms: number; hints: number; finished_at: string }[]
}

export default function Today() {
  const { user, settings } = useApp()
  const navigate = useNavigate()
  const [doc, setDoc] = useState<TodayDoc | null>(null)
  const load = useCallback(() => { api.get<TodayDoc>(`${V1}/today`).then(setDoc).catch(() => {}) }, [])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  if (!doc) return <Loading />
  const first = (user?.name || '').split(' ')[0]
  const h = new Date().getHours()
  const hello = h < 5 ? 'Good night' : h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening'
  const solvedToday = doc.daily.filter(d => d.finished_at).length
  return (
    <div className="page today">
      <div className="today-hero">
        <div className="grow">
          <h2>{hello}{first ? `, ${first}` : ''}</h2>
          <p className="muted">{weekday(doc.date, true)}, {fmtDate(doc.date)} · {doc.daily.length ? (solvedToday === doc.daily.length ? 'All of today’s puzzles solved — well done.' : `${doc.daily.length - solvedToday} of today’s puzzles to go.`) : 'Make a puzzle, or plan an escape room.'}</p>
        </div>
        {doc.streak > 0 && <div className="streak" title={`Your best run: ${doc.best_streak} days`}><Icon name="flame" size={22} /><b>{doc.streak}</b><span>{doc.streak === 1 ? 'day' : 'days in a row'}</span></div>}
      </div>

      {settings?.daily !== false && doc.daily.length > 0 && <section>
        <div className="sec-head"><h3>Puzzles of the day</h3><span className="muted small">New ones every morning; easier early in the week</span></div>
        <div className="daily-cards">
          {doc.daily.map(d => {
            const k = KIND[d.kind]
            return (
              <button key={d.kind} type="button" className={`daily-card ${d.finished_at ? 'done' : ''}`} onClick={() => navigate(`/today/${doc.date}/${d.kind}`)}>
                <span className="daily-ico" style={{ background: k.tint }}><Icon name={k.icon} size={22} /></span>
                <span className="daily-name">{d.kind_name}</span>
                <span className="muted small">{d.title !== d.kind_name ? `${d.title} · ` : ''}{LEVEL_NAME[d.level] || d.level}</span>
                <span className="daily-state">{d.finished_at ? <><Icon name="check-circle" size={15} />{fmtMs(d.ms || 0)}</> : d.started ? <><Icon name="play" size={14} />Carry on</> : <><Icon name="play" size={14} />Play</>}</span>
              </button>
            )
          })}
        </div>
        <p className="muted small"><Link to={`/today?date=${addDays(doc.date, -1)}`} onClick={e => { e.preventDefault(); api.get<TodayDoc>(`${V1}/today?date=${addDays(doc.date, -1)}`).then(setDoc) }}>Yesterday’s puzzles</Link>{doc.date !== todayISO() && <> · <a href="/" onClick={e => { e.preventDefault(); load() }}>Back to today</a></>}</p>
      </section>}

      <div className="home-cols">
        <section className="card">
          <div className="card-head"><h3>Your puzzles</h3><Link to="/puzzles" className="btn ghost sm">All</Link></div>
          {doc.going.length > 0 && <div className="rows">
            {doc.going.map(g => <Link key={g.id} to={`/puzzles/${g.id}?tab=play`} className="row"><KindIco kind={g.kind} /><span className="grow ellipsis">{g.title}</span><span className="tag">Carry on</span></Link>)}
          </div>}
          {doc.recent.length === 0 ? <Empty title="No puzzles yet" icon="puzzle"><Link to="/new" className="btn primary sm">Make your first puzzle</Link></Empty> :
            <div className="rows">{doc.recent.filter(p => !doc.going.some(g => g.id === p.id)).slice(0, 6).map(p => <Link key={p.id} to={`/puzzles/${p.id}`} className="row"><KindIco kind={p.kind} /><span className="grow ellipsis">{p.title}</span>{p.share_on && <Icon name="link" size={13} className="muted" />}<span className="muted small">{p.size_text}</span></Link>)}</div>}
        </section>
        <section className="card">
          <div className="card-head"><h3>Escape rooms &amp; hunts</h3><Link to="/adventures" className="btn ghost sm">All</Link></div>
          {doc.adventures.length === 0 ? <Empty title="Nothing planned" icon="key"><Link to="/adventures?new=1" className="btn sm">Plan an escape room</Link></Empty> :
            <div className="rows">{doc.adventures.map(a => <Link key={a.id} to={`/adventures/${a.id}`} className="row"><span className="list-ico"><Icon name={ADV_KIND[a.kind]?.icon || 'key'} size={15} /></span><span className="grow ellipsis">{a.title}</span>{a.live > 0 && <span className="tag live">{a.live} playing</span>}<span className="muted small">{a.date ? fmtDate(a.date, false) : 'no date'}</span></Link>)}</div>}
        </section>
      </div>

      {doc.results.length > 0 && <section className="card">
        <div className="card-head"><h3>Solved by others</h3><span className="muted small">The last week</span></div>
        <div className="rows">{doc.results.map((r, i) => <Link key={i} to={r.what === 'adventure' ? `/adventures/${r.ref_id}?tab=live` : `/puzzles/${r.ref_id}?tab=results`} className="row">
          <span className="list-ico"><Icon name={r.what === 'adventure' ? 'key' : 'trophy'} size={15} /></span>
          <span className="grow ellipsis"><b>{r.player}</b> {r.what === 'adventure' ? 'escaped' : 'solved'} {r.title}</span>
          <span className="num small">{fmtMs(r.ms)}</span><span className="muted small hide-sm">{ago(r.finished_at)}</span>
        </Link>)}</div>
      </section>}
    </div>
  )
}

export function KindIco({ kind, size = 15 }: { kind: Kind; size?: number }) {
  const k = KIND[kind]
  return <span className="kind-ico" style={{ color: k?.tint, background: `color-mix(in srgb, ${k?.tint} 13%, transparent)` }}><Icon name={k?.icon || 'puzzle'} size={size} /></span>
}

/** One of the day's puzzles. */
export function DailyPlay() {
  const { date = '', kind = '' } = useParams()
  const { settings } = useApp()
  const navigate = useNavigate()
  const [n, setN] = useState(0)
  useCrumb(`Puzzle of the day · ${KIND[kind as Kind]?.name || ''}`, '/')
  const start = useCallback(() => api.post<PlayView>(`${V1}/daily/${date}/${kind}/play`), [date, kind])
  const [others, setOthers] = useState<{ kind: Kind; finished_at?: string | null }[]>([])
  useEffect(() => { api.get<TodayDoc>(`${V1}/today?date=${date}`).then(d => setOthers(d.daily)).catch(() => {}) }, [date, n])
  const next = others.find(o => o.kind !== kind && !o.finished_at)
  return (
    <div className="page play-page">
      <Player key={date + kind} start={start} showMistakes={settings?.show_mistake} onSolved={() => setN(x => x + 1)}
        footer={next ? <button type="button" className="btn sm primary" onClick={() => navigate(`/today/${date}/${next.kind}`)}><Icon name="chevron-right" size={14} />Next: {KIND[next.kind].name}</button> : <Link to="/" className="btn sm">Back to today</Link>} />
    </div>
  )
}
