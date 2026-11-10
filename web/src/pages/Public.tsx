// What someone sees through a share link — no account: a puzzle, a book
// of puzzles, or an escape room / hunt (with ?card= when they scanned a QR card).
import { useCallback, useEffect, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { fmtDate } from '../lib/format'
import { ADV_KIND, KIND, LEVEL_NAME } from '../lib/puzzlecraft'
import type { BoardRow, Kind, PlayView } from '../lib/types'
import { AdventurePlayer } from '../components/AdventurePlayer'
import { Icon } from '../components/icons'
import { Markdown } from '../components/Markdown'
import { Player } from '../components/Player'
import { Loading, Spinner, useToast } from '../components/ui'

const NAME_KEY = 'ppzc-player'
const keyFor = (k: string) => `ppzc-play-${k}`
const savedKey = (k: string) => { try { return localStorage.getItem(keyFor(k)) || '' } catch { return '' } }
const keep = (k: string, v: string) => { try { localStorage.setItem(keyFor(k), v) } catch { /* private mode */ } }
const forget = (k: string) => { try { localStorage.removeItem(keyFor(k)) } catch { /* private mode */ } }
const savedName = () => { try { return localStorage.getItem(NAME_KEY) || '' } catch { return '' } }

function Shell({ children, from }: { children: React.ReactNode; from?: string }) {
  return (
    <div className="public">
      <header className="public-head"><img src="/favicon.svg" alt="" width={26} height={26} /><span>Pocket <b>Puzzlecraft</b></span>{from && <span className="muted small grow right">from {from}</span>}</header>
      <main className="public-main">{children}</main>
      <footer className="public-foot muted small">Made with Pocket Puzzlecraft — one Go binary, one SQLite file, one volume.</footer>
    </div>
  )
}

function NameBox({ name, setName, label = 'Your name (for the best times)' }: { name: string; setName: (v: string) => void; label?: string }) {
  return <label className="name-box"><span className="muted small">{label}</span><input value={name} onChange={e => setName(e.target.value)} maxLength={40} placeholder="Optional" /></label>
}

function Board({ best }: { best?: BoardRow[] }) {
  if (!best?.length) return null
  return <section className="card pad-card"><h3>Best times</h3><ol className="board">{best.map((b, i) => <li key={i}><span className="board-n">{i + 1}</span><span className="grow ellipsis">{b.player}</span><span className="num">{b.max_score ? `${b.score}/${b.max_score} · ` : ''}{b.time}</span></li>)}</ol></section>
}

export function PublicPuzzle() {
  const { token = '' } = useParams()
  const [info, setInfo] = useState<any>(null)
  const [err, setErr] = useState('')
  const [name, setName] = useState(savedName())
  const [go, setGo] = useState(() => !!savedKey('p-' + token))
  const [n, setN] = useState(0)
  useEffect(() => { api.get(`${V1}/public/p/${token}`).then(setInfo).catch(e => setErr((e as Error).message)) }, [token])
  const start = useCallback(async () => {
    try { localStorage.setItem(NAME_KEY, name) } catch { /* */ }
    const v = await api.post<PlayView>(`${V1}/public/p/${token}/play`, { player: name, key: savedKey('p-' + token) })
    keep('p-' + token, v.key)
    return v
  }, [token, name])
  const board = useCallback(async () => (await api.get<any>(`${V1}/public/p/${token}`)).best as BoardRow[] | undefined, [token])
  if (err) return <Shell><div className="public-err"><Icon name="lock" size={28} /><h2>Not here</h2><p className="muted">{err}</p></div></Shell>
  if (!info) return <Shell><Loading /></Shell>
  const k = KIND[info.kind as Kind]
  if (!go) return (
    <Shell from={info.from}>
      <div className="public-intro card pad-card">
        <span className="kind-ico lg" style={{ color: k?.tint, background: `color-mix(in srgb, ${k?.tint} 13%, transparent)` }}><Icon name={k?.icon || 'puzzle'} size={28} /></span>
        <h1>{info.title}</h1>
        <p className="muted">{info.kind_name}{info.level ? ` · ${LEVEL_NAME[info.level]}` : ''}{info.size_text ? ` · ${info.size_text}` : ''}</p>
        {info.instructions && <p>{info.instructions}</p>}
        <NameBox name={name} setName={setName} />
        <button type="button" className="btn primary lg" onClick={() => setGo(true)}><Icon name="play" size={16} />Play</button>
      </div>
      <Board best={info.best} />
    </Shell>
  )
  return (
    <Shell from={info.from}>
      <Player key={n} start={start} board={info.board ? board : undefined} restart={() => { forget('p-' + token); setN(x => x + 1) }} />
    </Shell>
  )
}

export function PublicBook() {
  const { token = '' } = useParams()
  const [sp, setSp] = useSearchParams()
  const [info, setInfo] = useState<any>(null)
  const [err, setErr] = useState('')
  const [name, setName] = useState(savedName())
  const pid = Number(sp.get('puzzle') || 0)
  useEffect(() => { api.get(`${V1}/public/b/${token}`).then(setInfo).catch(e => setErr((e as Error).message)) }, [token])
  const start = useCallback(async () => {
    try { localStorage.setItem(NAME_KEY, name) } catch { /* */ }
    const k = `b-${token}-${pid}`
    const v = await api.post<PlayView>(`${V1}/public/b/${token}/play`, { puzzle_id: pid, player: name, key: savedKey(k) })
    keep(k, v.key)
    return v
  }, [token, pid, name])
  if (err) return <Shell><div className="public-err"><Icon name="lock" size={28} /><h2>Not here</h2><p className="muted">{err}</p></div></Shell>
  if (!info) return <Shell><Loading /></Shell>
  const items = info.items as { puzzle_id: number; title: string; kind: Kind; kind_name: string; level: string; size_text: string }[]
  const at = items.findIndex(i => i.puzzle_id === pid)
  if (pid && at >= 0) return (
    <Shell from={info.from}>
      <button type="button" className="btn ghost sm" onClick={() => setSp({})}><Icon name="chevron-left" size={14} />{info.title}</button>
      <Player key={pid} start={start} footer={at + 1 < items.length ? <button type="button" className="btn sm primary" onClick={() => setSp({ puzzle: String(items[at + 1].puzzle_id) })}>Next puzzle<Icon name="chevron-right" size={14} /></button> : undefined} />
    </Shell>
  )
  return (
    <Shell from={info.from}>
      <div className="public-intro card pad-card">
        <span className="kind-ico lg"><Icon name="book-open" size={28} /></span>
        <h1>{info.title}</h1>
        {info.subtitle && <p className="muted">{info.subtitle}</p>}
        <NameBox name={name} setName={setName} />
      </div>
      <div className="rows card">{items.map((it, i) => (
        <button key={it.puzzle_id} type="button" className="row" onClick={() => setSp({ puzzle: String(it.puzzle_id) })}>
          <span className="muted num">{i + 1}</span><span className="kind-ico" style={{ color: KIND[it.kind]?.tint }}><Icon name={KIND[it.kind]?.icon || 'puzzle'} size={15} /></span>
          <span className="grow ellipsis">{it.title}</span><span className="muted small">{it.kind_name}</span><Icon name="chevron-right" size={14} className="muted" />
        </button>
      ))}</div>
    </Shell>
  )
}

export function PublicAdventure() {
  const { token = '' } = useParams()
  const [sp, setSp] = useSearchParams()
  const { toast } = useToast()
  const card = sp.get('card') || ''
  const [info, setInfo] = useState<any>(null)
  const [err, setErr] = useState('')
  const [team, setTeam] = useState(savedName())
  const [view, setView] = useState<PlayView | null>(null)
  const [busy, setBusy] = useState(false)
  useEffect(() => { api.get(`${V1}/public/a/${token}`).then(setInfo).catch(e => setErr((e as Error).message)) }, [token])
  const begin = useCallback(async (resumeOnly = false) => {
    const key = savedKey('a-' + token)
    if (resumeOnly && !key) return
    setBusy(true)
    try {
      try { localStorage.setItem(NAME_KEY, team) } catch { /* */ }
      const v = await api.post<PlayView>(`${V1}/public/a/${token}/play`, { player: team, key })
      keep('a-' + token, v.key)
      setView(v)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }, [token, team, toast])
  // carry on where this phone left off (a QR card scanned with the camera opens this page again)
  useEffect(() => { if (savedKey('a-' + token)) begin(true) }, [token]) // eslint-disable-line react-hooks/exhaustive-deps
  if (err) return <Shell><div className="public-err"><Icon name="lock" size={28} /><h2>Not here</h2><p className="muted">{err}</p></div></Shell>
  if (!info) return <Shell><Loading /></Shell>
  if (view) return <Shell from={info.from}><AdventurePlayer initial={view} scanCode={card} onCard={() => setSp({}, { replace: true })} /></Shell>
  const kind = ADV_KIND[info.kind as keyof typeof ADV_KIND]
  return (
    <Shell from={info.from}>
      <div className="public-intro card pad-card adv-intro">
        {info.photo && info.photo_id ? <img className="adv-cover" src={`${V1}/public/a/${token}/photo/${info.photo_id}`} alt="" /> : <span className="kind-ico lg"><Icon name={kind?.icon || 'key'} size={28} /></span>}
        <h1>{info.title}</h1>
        <p className="muted">{kind?.name} · {info.stages} {info.stages === 1 ? 'stage' : 'stages'}{info.minutes ? ` · ${info.minutes} minutes` : ''}{info.date ? ` · ${fmtDate(info.date)}${info.time ? ' ' + info.time : ''}` : ''}{info.place ? ` · ${info.place}` : ''}</p>
        {info.intro && <div className="adv-story"><Markdown text={info.intro} /></div>}
        {card && <p className="callout info"><Icon name="qr" size={15} /><span>You scanned a card. Start the game (or open it on the phone you started on) and it counts.</span></p>}
        <NameBox name={team} setName={setTeam} label="Your team’s name" />
        <button type="button" className="btn primary lg" onClick={() => begin()} disabled={busy}>{busy ? <Spinner /> : <Icon name="play" size={16} />}Start</button>
        {info.hint_penalty > 0 && <p className="muted small">Hints are there when you need them; each adds {Math.round(info.hint_penalty / 60)} {Math.round(info.hint_penalty / 60) === 1 ? 'minute' : 'minutes'} to your time.</p>}
      </div>
      <Board best={info.best} />
    </Shell>
  )
}
