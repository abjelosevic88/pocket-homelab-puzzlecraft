// Playing a puzzle of any kind: the clock, the bar (check, give a hint, start
// again), saving progress, and the end. The puzzle itself comes from its kind.
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../lib/api'
import { fmtMs, KIND, LEVEL_NAME } from '../lib/puzzlecraft'
import type { BoardRow, CheckResult, PlayView, PublicPuzzle } from '../lib/types'
import { Icon } from './icons'
import { KINDS_UI } from './kinds'
import type { PlayActions } from './kinds/types'
import { Loading, useToast } from './ui'

const playURL = (key: string) => `${V1}/public/play/${key}`

/** A ticking clock that stops while the page is hidden. */
function useClock(running: boolean, start: number) {
  const [ms, setMs] = useState(start)
  const base = useRef(start)
  const since = useRef(performance.now())
  useEffect(() => { base.current = start; since.current = performance.now(); setMs(start) }, [start])
  useEffect(() => {
    if (!running) return
    since.current = performance.now()
    const tick = () => { if (document.visibilityState === 'visible') setMs(base.current + performance.now() - since.current) }
    const vis = () => {
      if (document.visibilityState === 'hidden') base.current += performance.now() - since.current
      since.current = performance.now()
    }
    const t = window.setInterval(tick, 1000)
    document.addEventListener('visibilitychange', vis)
    return () => { window.clearInterval(t); document.removeEventListener('visibilitychange', vis); base.current += performance.now() - since.current }
  }, [running])
  const now = useCallback(() => base.current + (running ? performance.now() - since.current : 0), [running])
  return [ms, now] as const
}

export interface PlayerProps {
  start?: () => Promise<PlayView>   // starts or carries on with a go
  initial?: PlayView                // or a go already started
  pick?: (v: PlayView) => PublicPuzzle | undefined // which puzzle of the go (an escape-room stage's)
  onRaw?: (res: any) => void        // every check's full answer (an escape room moves on with it)
  onSolved?: (v: PlayView) => void
  board?: () => Promise<BoardRow[] | undefined>
  showMistakes?: boolean
  restart?: () => void              // "Play again"
  footer?: React.ReactNode          // under the end panel (share buttons)
  bare?: boolean                    // inside another page (no title)
}

export function Player({ start, initial, pick, onRaw, onSolved, board, showMistakes = false, restart, footer, bare }: PlayerProps) {
  const { toast } = useToast()
  const [view, setView] = useState<PlayView | null>(initial || null)
  const [err, setErr] = useState('')
  const [actions, setActions] = useState<PlayActions>({})
  const [result, setResult] = useState<CheckResult | null>(null)
  const [best, setBest] = useState<BoardRow[] | null>(null)
  const [busy, setBusy] = useState(false)
  const startRef = useRef(start)
  startRef.current = start
  useEffect(() => { if (initial) setView(initial) }, [initial])
  useEffect(() => {
    if (initial || !startRef.current) return
    let live = true
    startRef.current().then(v => { if (live) setView(v) }).catch(e => { if (live) setErr((e as Error).message) })
    return () => { live = false }
  }, [initial])
  const pz = view ? (pick ? pick(view) : view.puzzle) : undefined
  const saved = view?.progress && typeof view.progress === 'object' && 'p' in view.progress ? view.progress : null
  const startMs = useMemo(() => {
    if (!view) return 0
    if (view.finished_at) return view.ms
    if (saved && typeof saved.ms === 'number') return saved.ms
    return 0
  }, [view?.key]) // eslint-disable-line react-hooks/exhaustive-deps
  const done = !!view?.finished_at
  const [ms, now] = useClock(!!view && !done, startMs)
  const shownMs = done ? view!.ms : ms

  // save what they filled in, a moment after they stop typing
  const pending = useRef<any>(undefined)
  const timer = useRef<number>(0)
  const key = view?.key
  const flush = useCallback(() => {
    if (!key || pending.current === undefined) return
    const body = { progress: { p: pending.current, ms: Math.round(now()) } }
    pending.current = undefined
    api.post(`${playURL(key)}/save`, body).catch(() => {})
  }, [key, now])
  const onProgress = useCallback((p: any) => {
    pending.current = p
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(flush, 900)
  }, [flush])
  useEffect(() => {
    const h = () => flush()
    window.addEventListener('pagehide', h)
    return () => { window.removeEventListener('pagehide', h); flush() }
  }, [flush])

  const check = useCallback(async (answer: any, final = false): Promise<CheckResult | null> => {
    if (!view) return null
    try {
      const res = await api.post<any>(`${playURL(view.key)}/check`, { answer, final, ms: Math.round(now()), progress: pending.current !== undefined ? { p: pending.current, ms: Math.round(now()) } : undefined })
      pending.current = undefined
      onRaw?.(res)
      const r = res.result as CheckResult
      setResult(r)
      if (r.solved && res.finished_at) {
        const nv = { ...view, finished_at: res.finished_at, ms: res.ms, hints: res.hints, mistakes: res.mistakes, score: r.score || view.score, max_score: r.max || view.max_score }
        setView(nv)
        onSolved?.(nv)
        if (board) board().then(b => setBest(b || null)).catch(() => {})
      }
      return r
    } catch (e) { toast((e as Error).message, 'err'); return null }
  }, [view, now, onRaw, onSolved, board, toast])

  const reveal = useCallback(async (what: number) => {
    if (!view) return null
    try {
      const r = await api.post<{ reveal: any; hints: number }>(`${playURL(view.key)}/reveal`, { what })
      setView(v => v ? { ...v, hints: r.hints, shown: [...(v.shown || []), what] } : v)
      return r.reveal
    } catch (e) { toast((e as Error).message, 'err'); return null }
  }, [view, toast])

  const photoURL = useCallback((id: number, thumb = false) => view ? `${playURL(view.key)}/photo/${id}${thumb ? '?thumb=1' : ''}` : '', [view])

  if (err) return <div className="callout warn"><Icon name="alert" size={15} /><div>{err}</div></div>
  if (!view || !pz) return <Loading />
  const K = KINDS_UI[pz.kind]
  const meta = KIND[pz.kind]
  const run = async (f?: () => void) => { if (!f || busy) return; setBusy(true); try { await f() } finally { setBusy(false) } }
  return (
    <div className={`player kind-${pz.kind} ${done ? 'is-done' : ''}`}>
      {!bare && <div className="player-head">
        <span className="player-ico" style={{ background: meta?.tint }}><Icon name={meta?.icon || 'puzzle'} size={18} /></span>
        <div className="grow">
          <h2 className="player-title">{pz.title}</h2>
          <div className="muted small">{pz.kind_name}{pz.level ? ` · ${LEVEL_NAME[pz.level] || pz.level}` : ''}{pz.size_text ? ` · ${pz.size_text}` : ''}</div>
        </div>
      </div>}
      {pz.instructions && !bare && <p className="player-instr">{pz.instructions}</p>}
      <div className="player-bar" role="toolbar" aria-label="Puzzle">
        {pz.timer !== false && <span className="player-clock" aria-label="Time"><Icon name="timer" size={15} />{fmtMs(shownMs)}</span>}
        {view.hints > 0 && <span className="player-hints muted small"><Icon name="lightbulb" size={14} />{view.hints}</span>}
        <span className="spacer" />
        <button type="button" className="btn sm" disabled={done || !actions.check || busy} onClick={() => run(actions.check)}><Icon name="check" size={14} />Check</button>
        {pz.reveal !== false && <button type="button" className="btn sm" disabled={done || !actions.reveal || busy} onClick={() => run(actions.reveal)} title="Counts as a hint"><Icon name="lightbulb" size={14} />{actions.revealLabel || 'Hint'}</button>}
        {actions.clear && <button type="button" className="btn sm ghost" disabled={done} onClick={() => actions.clear?.()} title="Clear what you filled in"><Icon name="rotate-ccw" size={14} /><span className="hide-sm">Clear</span></button>}
      </div>
      {result && !result.solved && result.message && <div className="callout info player-msg"><Icon name="info" size={15} /><div>{result.message}</div></div>}
      <div className="player-body">
        <K.Play key={view.key + (pz.id || '') + (pz.daily || '')} view={pz} progress={saved ? saved.p : null} shown={view.shown || []} done={done} showMistakes={showMistakes}
          onProgress={onProgress} check={check} reveal={reveal} canReveal={pz.reveal !== false} photoURL={photoURL} setActions={setActions} />
      </div>
      {done && <div className="player-end" role="status">
        <div className="player-end-badge"><Icon name="trophy" size={26} /></div>
        <div className="grow">
          <h3>Solved!</h3>
          <p className="muted">In {fmtMs(view.ms)}{view.hints ? `, with ${view.hints} ${view.hints === 1 ? 'hint' : 'hints'}` : ', no hints'}{view.max_score ? ` · ${view.score} of ${view.max_score} points` : ''}.</p>
          {best && best.length > 0 && <ol className="board">{best.slice(0, 5).map((b, i) => <li key={i}><span className="board-n">{i + 1}</span><span className="grow ellipsis">{b.player}</span><span className="num">{b.max_score ? `${b.score}/${b.max_score} · ` : ''}{b.time}</span></li>)}</ol>}
          {result?.explain && result.explain.some(Boolean) && null}
          <div className="row wrap">{restart && <button type="button" className="btn sm" onClick={restart}><Icon name="refresh" size={14} />Play again</button>}{footer}</div>
        </div>
      </div>}
    </div>
  )
}
