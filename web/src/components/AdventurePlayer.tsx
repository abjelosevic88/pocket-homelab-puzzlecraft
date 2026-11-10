// Playing an escape room, a treasure hunt or a mystery: one stage at a time,
// its story and its lock — a word, a number lock, arrows, colours, a QR card
// to find, or a puzzle — hints that come with time, and the end.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, V1 } from '../lib/api'
import { fmtMs } from '../lib/puzzlecraft'
import type { AdventureView, PlayView } from '../lib/types'
import { Icon } from './icons'
import { Markdown } from './Markdown'
import { Player } from './Player'
import { Spinner, useToast } from './ui'

const playURL = (key: string) => `${V1}/public/play/${key}`

const COLOR_HEX: Record<string, string> = { red: '#e03131', orange: '#f76707', yellow: '#fab005', green: '#2f9e44', blue: '#1c7ed6', purple: '#7048e8', pink: '#e64980', white: '#f8f9fa', black: '#212529', brown: '#8d5524' }
const ARROW: Record<string, string> = { U: '↑', D: '↓', L: '←', R: '→' }

function sinceMs(iso: string, nowISO: string, skew: number) {
  return Math.max(0, Date.now() - skew - new Date(iso).getTime())
}

export function AdventurePlayer({ initial, scanCode, onCard }: { initial: PlayView; scanCode?: string; onCard?: () => void }) {
  const { toast } = useToast()
  const [view, setView] = useState<PlayView>(initial)
  const [answer, setAnswer] = useState('')
  const [seq, setSeq] = useState<string[]>([])
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null)
  const [unlock, setUnlock] = useState('')
  const [busy, setBusy] = useState(false)
  const [code, setCode] = useState('')
  const [, tick] = useState(0)
  const skew = useMemo(() => Date.now() - new Date(initial.server_now).getTime(), [initial.server_now])
  useEffect(() => { const t = window.setInterval(() => tick(x => x + 1), 1000); return () => window.clearInterval(t) }, [])
  const a = view.adventure as AdventureView
  const st = a.stage
  const ms = view.finished_at ? view.ms : sinceMs(view.started_at, view.server_now, skew)
  const left = a.minutes > 0 ? a.minutes * 60000 - ms : 0

  const apply = useCallback((res: any) => {
    if (res.opened && res.play) {
      setView(res.play)
      setUnlock(res.unlock || '')
      setAnswer(''); setSeq([]); setMsg(null); setCode('')
      window.scrollTo({ top: 0, behavior: 'smooth' })
    } else if (res.message) setMsg({ ok: false, text: res.message })
  }, [])
  const tryAnswer = async (value: string) => {
    if (busy) return
    setBusy(true)
    try { apply(await api.post(`${playURL(view.key)}/answer`, { answer: value, ms })) } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  const scan = useCallback(async (c: string) => {
    if (!c.trim()) return
    try { const r = await api.post<any>(`${playURL(view.key)}/scan`, { code: c.trim(), ms }); apply(r); if (!r.opened) setMsg({ ok: false, text: r.message }) } catch (e) { toast((e as Error).message, 'err') }
  }, [view.key, ms, apply, toast])
  useEffect(() => { if (scanCode) { scan(scanCode); onCard?.() } }, [scanCode]) // eslint-disable-line react-hooks/exhaustive-deps
  const hint = async () => {
    try { setView(await api.post<PlayView>(`${playURL(view.key)}/hint`)) } catch (e) { toast((e as Error).message, 'err') }
  }

  if (view.finished_at) return (
    <div className="adv-player done">
      <div className="adv-end">
        <div className="player-end-badge big"><Icon name="trophy" size={34} /></div>
        <h2>{a.kind === 'escape' ? 'You escaped!' : a.kind === 'hunt' ? 'Treasure found!' : 'Solved!'}</h2>
        <p className="muted">{fmtMs(view.ms)}{view.hints ? ` (with ${view.hints} ${view.hints === 1 ? 'hint' : 'hints'}${a.hint_penalty ? `, +${Math.round(a.hint_penalty / 60)} min each` : ''})` : ''}{a.over ? ' — past the time limit, but you made it' : ''}</p>
        {unlock && <div className="adv-unlock"><Icon name="key" size={16} /><Markdown text={unlock} /></div>}
        {a.finale && <div className="adv-story"><Markdown text={a.finale} /></div>}
        {a.best && a.best.length > 0 && <ol className="board">{a.best.slice(0, 8).map((b, i) => <li key={i}><span className="board-n">{i + 1}</span><span className="grow ellipsis">{b.player}</span><span className="num">{b.time}</span></li>)}</ol>}
      </div>
    </div>
  )
  if (!st) return <Spinner />
  const hintWait = st.hints_left > 0 ? Math.max(0, st.next_hint_in * 60000 - sinceMs(view.server_now, view.server_now, skew)) : 0
  const lockBox = () => {
    switch (st.lock) {
      case 'none': return <button type="button" className="btn primary lg" onClick={() => tryAnswer('')} disabled={busy}>Carry on<Icon name="chevron-right" size={16} /></button>
      case 'qr': return (
        <div className="adv-qr">
          <Icon name="qr" size={40} />
          <p><b>Find the card</b> and scan its code with your phone’s camera.</p>
          <div className="row"><input value={code} onChange={e => setCode(e.target.value)} placeholder="Or type the code under it" aria-label="Card code" /><button type="button" className="btn" onClick={() => scan(code)} disabled={!code.trim()}>Use</button></div>
        </div>
      )
      case 'code': {
        const n = Math.max(st.length, 3)
        const digits = (answer.padEnd(n, '0')).slice(0, n).split('')
        return (
          <div className="lock-code" role="group" aria-label={st.lock_hint || 'Number lock'}>
            {digits.map((d, i) => (
              <div key={i} className="wheel">
                <button type="button" aria-label="Up" onClick={() => { const x = [...digits]; x[i] = String((Number(d) + 1) % 10); setAnswer(x.join('')) }}><Icon name="chevron-up" size={18} /></button>
                <span className="wheel-v">{d}</span>
                <button type="button" aria-label="Down" onClick={() => { const x = [...digits]; x[i] = String((Number(d) + 9) % 10); setAnswer(x.join('')) }}><Icon name="chevron-down" size={18} /></button>
              </div>
            ))}
            <button type="button" className="btn primary lock-try" onClick={() => tryAnswer(digits.join(''))} disabled={busy}><Icon name="key" size={16} />Open</button>
          </div>
        )
      }
      case 'directions': return (
        <div className="lock-dir">
          <div className="dir-seq" aria-live="polite">{seq.length ? seq.map((s, i) => <span key={i}>{ARROW[s]}</span>) : <span className="muted">Push the lock…</span>}</div>
          <div className="dir-pad">
            <span /><button type="button" onClick={() => setSeq([...seq, 'U'])} aria-label="Up">↑</button><span />
            <button type="button" onClick={() => setSeq([...seq, 'L'])} aria-label="Left">←</button><button type="button" className="dir-clear" onClick={() => setSeq([])} aria-label="Start again"><Icon name="rotate-ccw" size={16} /></button><button type="button" onClick={() => setSeq([...seq, 'R'])} aria-label="Right">→</button>
            <span /><button type="button" onClick={() => setSeq([...seq, 'D'])} aria-label="Down">↓</button><span />
          </div>
          <button type="button" className="btn primary" onClick={() => tryAnswer(seq.join(''))} disabled={busy || !seq.length}><Icon name="key" size={16} />Open</button>
        </div>
      )
      case 'colors': return (
        <div className="lock-colors">
          <div className="col-seq">{seq.length ? seq.map((c, i) => <i key={i} style={{ background: COLOR_HEX[c] || c }} title={c} />) : <span className="muted">Press the colours in order…</span>}</div>
          <div className="col-pad">{(st.colors || []).map(c => <button key={c} type="button" style={{ background: COLOR_HEX[c] || c }} onClick={() => setSeq([...seq, c])} aria-label={c} title={c} />)}</div>
          <div className="row"><button type="button" className="btn" onClick={() => setSeq([])} disabled={!seq.length}>Start again</button><button type="button" className="btn primary" onClick={() => tryAnswer(seq.join(','))} disabled={busy || !seq.length}><Icon name="key" size={16} />Open</button></div>
        </div>
      )
      case 'puzzle': return st.puzzle ? <div className="adv-puzzle"><Player initial={{ ...view, progress: null, finished_at: null, puzzle: st.puzzle }} pick={v => v.puzzle} onRaw={apply} bare /></div> : <p className="muted">This stage’s puzzle is missing — ask the host.</p>
      default: return (
        <form className="lock-answer" onSubmit={e => { e.preventDefault(); tryAnswer(answer) }}>
          <input value={answer} onChange={e => setAnswer(e.target.value)} placeholder={st.lock === 'word' && st.length ? `${st.length} letters` : 'Your answer'} autoCapitalize="off" autoComplete="off" aria-label="Answer" />
          <button type="submit" className="btn primary" disabled={busy || !answer.trim()}><Icon name="key" size={16} />Try it</button>
        </form>
      )
    }
  }
  return (
    <div className="adv-player">
      <div className="adv-bar">
        <span className="player-clock"><Icon name="timer" size={15} />{a.minutes > 0 ? (left >= 0 ? fmtMs(left) : `+${fmtMs(-left)}`) : fmtMs(ms)}</span>
        <span className="adv-dots" aria-label={`Stage ${st.n} of ${a.stages}`}>{Array.from({ length: a.stages }, (_, i) => <i key={i} className={i < a.done ? 'done' : i === a.done ? 'cur' : ''} />)}</span>
        {view.hints > 0 && <span className="muted small"><Icon name="lightbulb" size={13} />{view.hints}</span>}
      </div>
      {unlock && <div className="adv-unlock"><Icon name="key" size={16} /><div><b>Open!</b> <Markdown text={unlock} /></div></div>}
      <article className="adv-stage">
        <div className="muted small">Stage {st.n} of {a.stages}{st.place_name ? ` · ${st.place_name}` : ''}</div>
        <h2>{st.title}</h2>
        {st.photo && st.photo_id && <img className="adv-photo" src={`${playURL(view.key)}/photo/${st.photo_id}`} alt="" />}
        {st.story && <div className="adv-story"><Markdown text={st.story} /></div>}
        {st.lat != null && st.lng != null && <a className="btn sm" href={`https://www.openstreetmap.org/?mlat=${st.lat}&mlon=${st.lng}#map=18/${st.lat}/${st.lng}`} target="_blank" rel="noopener"><Icon name="map" size={14} />Show the place on a map</a>}
        {st.lock_hint && <p className="lock-hint"><Icon name="lock" size={14} />{st.lock_hint}</p>}
        <div className="adv-lock">{lockBox()}</div>
        {msg && <p className={`adv-msg ${msg.ok ? 'ok' : 'bad'}`} role="status">{msg.text}</p>}
      </article>
      {(st.hints.length > 0 || st.hints_left > 0) && <section className="adv-hints">
        {st.hints.map((h, i) => <div key={i} className="adv-hint"><Icon name="lightbulb" size={15} /><div>{h}</div></div>)}
        {st.hints_left > 0 && <button type="button" className="btn sm" onClick={hint} disabled={hintWait > 1000}>
          <Icon name="lightbulb" size={14} />{hintWait > 1000 ? `A hint in ${Math.ceil(hintWait / 60000)} min` : `A hint${a.hint_penalty ? ` (+${Math.round(a.hint_penalty / 60)} min)` : ''}`}
        </button>}
      </section>}
      {a.past.length > 0 && <details className="adv-past"><summary className="muted small">What you found so far ({a.past.length})</summary>
        <ol>{a.past.map((x, i) => <li key={i}><b>{x.title}</b>{x.unlock && <> — {x.unlock}</>}</li>)}</ol></details>}
    </div>
  )
}
