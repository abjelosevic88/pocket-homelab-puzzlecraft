// One puzzle: make it (the kind's editor, tested as you go), play it,
// share it, see how others did, print it.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, openAction, useAppState } from '../lib/data'
import { ago, fmtDateTime } from '../lib/format'
import { fmtMs, KIND, LANGS, LEVELS } from '../lib/puzzlecraft'
import type { Lang, Level, Play, PlayView, Puzzle, Report } from '../lib/types'
import { Icon } from '../components/icons'
import { KINDS_UI } from '../components/kinds'
import { useCrumb } from '../components/Layout'
import { Player } from '../components/Player'
import { Select } from '../components/Select'
import { ShareBox, ShareOption } from '../components/ShareBox'
import { Confirm, download, Empty, Field, Loading, MenuButton, Modal, Segmented, Spinner, useToast } from '../components/ui'
import { KindIco } from './Today'

type Tab = 'make' | 'play' | 'share' | 'results'

export default function PuzzlePage() {
  const { id = '' } = useParams()
  const [sp, setSp] = useSearchParams()
  const tab = (['make', 'play', 'share', 'results'].includes(sp.get('tab') || '') ? sp.get('tab') : 'make') as Tab
  const navigate = useNavigate()
  const { toast } = useToast()
  const { settings } = useApp()
  const [p, setP] = useState<Puzzle | null>(null)
  const [data, setData] = useState<any>(null)
  const [report, setReport] = useState<Report | null>(null)
  const [save, setSave] = useState<'' | 'saving' | 'saved' | 'error'>('')
  const [err, setErr] = useState('')
  const [del, setDel] = useState(false)
  const [translate, setTranslate] = useState(false)
  const [playN, setPlayN] = useState(0)
  const state = useAppState()
  useCrumb(p?.title, '/puzzles')

  const load = useCallback(async () => {
    try {
      const doc = await api.get<Puzzle>(`${V1}/puzzles/${id}`)
      setP(doc); setData(doc.data)
      api.post<Report>(`${V1}/puzzles/${id}/test`).then(setReport).catch(() => {})
    } catch (e) { setErr((e as Error).message) }
  }, [id])
  useEffect(() => { load() }, [load])

  // save the definition a moment after the last change, then test it
  const timer = useRef(0)
  const pendingData = useRef<any>(null)
  const flush = useCallback(async () => {
    if (pendingData.current === null) return
    const d = pendingData.current
    pendingData.current = null
    setSave('saving')
    try {
      const doc = await api.put<Puzzle>(`${V1}/puzzles/${id}`, { data: d })
      setP(doc)
      setSave('saved')
      api.post<Report>(`${V1}/puzzles/${id}/test`).then(setReport).catch(() => {})
      announceChange()
    } catch (e) { setSave('error'); toast((e as Error).message, 'err') }
  }, [id, toast])
  useEffect(() => () => { window.clearTimeout(timer.current); flush() }, [flush])
  const onData = (d: any) => {
    setData(d)
    pendingData.current = d
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(flush, 900)
  }
  const patch = async (body: Record<string, unknown>, msg?: string) => {
    try { const doc = await api.put<Puzzle>(`${V1}/puzzles/${id}`, body); setP(doc); if (body.data === undefined) setData(doc.data); if (msg) toast(msg); announceChange() } catch (e) { toast((e as Error).message, 'err') }
  }
  const share = async (body: Record<string, unknown>) => {
    try { const doc = await api.post<Puzzle>(`${V1}/puzzles/${id}/share`, body); setP(doc); announceChange() } catch (e) { toast((e as Error).message, 'err') }
  }

  if (err) return <div className="page"><Empty title="Not found" icon="puzzle">{err}</Empty></div>
  if (!p) return <Loading />
  const K = KINDS_UI[p.kind]
  const k = KIND[p.kind]
  const send = async (url: string, body: Record<string, unknown>) => {
    try { const r = await api.post<{ text: string; url?: string }>(url, body); toast(r.text, 'ok', openAction(r.url)) } catch (e) { toast((e as Error).message, 'err') }
  }
  const files = state('files', 'files:write'), notes = state('notes', 'notes:write'), learn = state('learn', 'learn:write'), tr = state('translate', 'translate:run')
  const wordy = ['crossword', 'wordsearch', 'scramble', 'quiz'].includes(p.kind)
  const go = (t: Tab) => { const n = new URLSearchParams(sp); n.set('tab', t); setSp(n, { replace: true }) }
  return (
    <div className="page pz-page">
      <div className="pp-head">
        <KindIco kind={p.kind} size={20} />
        <input className="pp-title-input" value={p.title} onChange={e => setP({ ...p, title: e.target.value })} onBlur={e => patch({ title: e.target.value })} aria-label="Name" />
        <span className={`save-state ${save === 'error' ? 'error' : ''}`} aria-live="polite">{save === 'saving' ? 'Saving…' : save === 'saved' ? 'Saved' : save === 'error' ? 'Not saved' : ''}</span>
        <MenuButton label="More" align="right" width={260} items={() => [
          { label: 'Print', icon: 'printer', onSelect: () => window.open(`/puzzles/${p.id}/print`, '_blank') },
          { label: 'Make a copy', icon: 'copy', onSelect: async () => { const c = await api.post<Puzzle>(`${V1}/puzzles/${p.id}/duplicate`); announceChange(); navigate(`/puzzles/${c.id}`) } },
          ...(p.kind === 'crossword' || p.kind === 'sudoku' ? [{ label: 'Download .ipuz', icon: 'download', onSelect: () => download(`${V1}/puzzles/${p.id}/export.ipuz`) }] : []),
          ...(p.kind === 'crossword' ? [{ label: 'Download .puz (Across Lite)', icon: 'download', onSelect: () => download(`${V1}/puzzles/${p.id}/export.puz`) }] : []),
          'sep' as const,
          { label: 'Answers to Pocket Notes', icon: 'note', disabled: !notes.ok, hint: notes.ok ? undefined : 'not connected', onSelect: () => send(`${V1}/apps/notes`, { what: 'puzzle', id: p.id }) },
          ...(p.kind === 'crossword' || p.kind === 'sudoku' ? [{ label: 'Save .ipuz in Pocket Files', icon: 'file', disabled: !files.ok, hint: files.ok ? undefined : 'not connected', onSelect: () => send(`${V1}/apps/files?type=ipuz&puzzle=${p.id}`, {}) }] : []),
          ...(wordy ? [{ label: 'Words to Pocket Learn', icon: 'brain', disabled: !learn.ok, hint: learn.ok ? undefined : 'not connected', onSelect: () => send(`${V1}/apps/learn`, { puzzle_id: p.id }) }] : []),
          ...(wordy ? [{ label: 'A translated copy…', icon: 'globe', disabled: !tr.ok, hint: tr.ok ? undefined : 'not connected', onSelect: () => setTranslate(true) }] : []),
          'sep' as const,
          { label: p.pinned ? 'Unpin' : 'Pin to the top', icon: 'pin', onSelect: () => patch({ pinned: !p.pinned }) },
          { label: p.archived ? 'Bring back' : 'Archive', icon: 'archive', onSelect: () => patch({ archived: !p.archived }, p.archived ? 'Back with your puzzles' : 'Archived') },
          { label: 'Delete', icon: 'trash', danger: true, onSelect: () => setDel(true) },
        ]}><Icon name="more" size={18} /></MenuButton>
      </div>
      <div className="pp-tabs" role="tablist">
        {([['make', 'Make', 'edit'], ['play', 'Play', 'play'], ['share', 'Share', 'link'], ['results', 'Results', 'trophy']] as const).map(([t, l, ic]) => (
          <button key={t} type="button" role="tab" aria-selected={tab === t} className={`pp-tab ${tab === t ? 'on' : ''}`} onClick={() => go(t)}>
            <Icon name={ic} size={15} />{l}{t === 'results' && p.solves > 0 && <span className="count">{p.solves}</span>}{t === 'share' && p.share_on && <span className="dot-on" />}
          </button>
        ))}
      </div>

      {tab === 'make' && <div className="make-cols">
        <div className="make-main card pad-card">
          {data && <K.Edit puzzle={p} data={data} onChange={onData} settings={settings} language={p.language} />}
        </div>
        <aside className="make-side stack">
          <TestCard report={report} ready={p.ready} />
          <section className="card pad-card stack">
            <Field label="Difficulty"><Segmented size="sm" value={p.level} onChange={(v: Level) => patch({ level: v })} options={LEVELS.map(l => ({ value: l.value, label: l.label }))} /></Field>
            {wordy || p.kind === 'cryptogram' ? <Field label="Language"><Select ariaLabel="Language" value={p.language} onChange={v => patch({ language: v as Lang })} options={LANGS.map(l => ({ value: l.value, label: l.label }))} /></Field> : null}
            <Field label="For players" help="Shown above the puzzle."><textarea rows={2} defaultValue={p.instructions} onBlur={e => e.target.value !== p.instructions && patch({ instructions: e.target.value })} placeholder={`${k.line}.`} /></Field>
            <Field label="Tags"><input defaultValue={p.tags.join(', ')} onBlur={e => patch({ tags: e.target.value.split(',').map(t => t.trim()).filter(Boolean) })} placeholder="kids, birthday" /></Field>
            <Field label="Your notes" help="Only you see these."><textarea rows={2} defaultValue={p.notes} onBlur={e => e.target.value !== p.notes && patch({ notes: e.target.value })} /></Field>
            {p.uses && p.uses.length > 0 && <p className="muted small"><Icon name="layers" size={12} className="inline-ico" /> Used in {p.uses.join(', ')}</p>}
            <p className="muted small">Made {fmtDateTime(p.created_at)}{p.source ? ` by Pocket ${p.source[0].toUpperCase()}${p.source.slice(1)}` : ''} · changed {ago(p.updated_at)}</p>
          </section>
        </aside>
      </div>}

      {tab === 'play' && <div className="play-page">
        {!p.ready && <div className="callout warn"><Icon name="alert" size={15} /><div>The test still finds something to fix (see Make) — you can play it, but others might get stuck.</div></div>}
        <Player key={playN} start={() => api.post<PlayView>(`${V1}/puzzles/${p.id}/play`, { restart: playN > 0 })} showMistakes={settings?.show_mistake} bare
          onSolved={() => { load(); announceChange() }} restart={() => setPlayN(n => n + 1)} />
      </div>}

      {tab === 'share' && <div className="page narrow-ish">
        <ShareBox what="puzzle" id={p.id} on={p.share_on} url={p.share_url || ''} title={p.title}
          onToggle={on => share({ on })} onRenew={() => share({ renew: true })}
          options={<>
            <ShareOption title="Best times" help="Players see the fastest finishers." checked={p.board} onChange={v => share({ board: v })} />
            <ShareOption title="Hints" help={p.kind === 'crossword' ? 'Players may ask for a letter (it counts).' : 'Players may ask for help (it counts).'} checked={p.reveal} onChange={v => share({ reveal: v })} />
            <ShareOption title="Clock" help="Show the time while they play." checked={p.timer} onChange={v => share({ timer: v })} />
          </>} />
      </div>}

      {tab === 'results' && <Results puzzle={p} />}

      {translate && <TranslateCopy puzzle={p} data={data} onClose={() => setTranslate(false)} onDone={id2 => navigate(`/puzzles/${id2}`)} />}
      {del && <Confirm title="Delete this puzzle?" message={`“${p.title}” and how it was played go for good. Books and escape rooms using it lose it.`} confirmLabel="Delete"
        onCancel={() => setDel(false)} onConfirm={async () => { await api.del(`${V1}/puzzles/${p.id}`); announceChange(); navigate('/puzzles') }} />}
    </div>
  )
}

export function TestCard({ report, ready }: { report: Report | null; ready: boolean }) {
  return (
    <section className={`card pad-card test-card ${report ? (report.ok ? 'ok' : 'bad') : ''}`}>
      <div className="row"><Icon name={report?.ok ?? ready ? 'check-circle' : 'alert'} size={18} /><h3 className="grow">{!report ? 'Testing…' : report.ok ? 'Ready to play' : 'Not ready yet'}</h3>{!report && <Spinner />}</div>
      {report && report.problems.length > 0 && <ul className="test-list bad">{report.problems.map((x, i) => <li key={i}>{x}</li>)}</ul>}
      {report && report.notes.length > 0 && <ul className="test-list">{report.notes.map((x, i) => <li key={i}>{x}</li>)}</ul>}
      {report?.ok && !report.notes.length && <p className="muted small">It has one fair answer and nothing is missing.</p>}
    </section>
  )
}

function Results({ puzzle }: { puzzle: Puzzle }) {
  const { toast } = useToast()
  const [doc, setDoc] = useState<{ plays: Play[]; finished: number; started: number; avg_ms: number } | null>(null)
  const load = useCallback(() => { api.get<typeof doc>(`${V1}/puzzles/${puzzle.id}/plays`).then(setDoc).catch(() => {}) }, [puzzle.id])
  useEffect(() => { load() }, [load])
  if (!doc) return <Loading />
  if (!doc.plays.length) return <Empty title="Nobody has played it yet" icon="trophy">{puzzle.share_on ? 'Send the link to someone.' : 'Share it (the Share tab) and send the link to someone.'}</Empty>
  const done = doc.plays.filter(x => x.finished_at).sort((a, b) => a.ms - b.ms)
  const going = doc.plays.filter(x => !x.finished_at)
  return (
    <div className="stack">
      <div className="stats-row">
        <div className="stat"><div className="stat-l">Played</div><div className="stat-v">{doc.started}</div></div>
        <div className="stat"><div className="stat-l">Solved</div><div className="stat-v">{doc.finished}</div></div>
        <div className="stat"><div className="stat-l">Best time</div><div className="stat-v">{done[0] ? fmtMs(done[0].ms) : '—'}</div></div>
        <div className="stat"><div className="stat-l">On average</div><div className="stat-v">{doc.avg_ms ? fmtMs(doc.avg_ms) : '—'}</div></div>
      </div>
      <section className="card">
        <div className="card-head"><h3>Best times</h3></div>
        <div className="table-wrap"><table className="tbl">
          <thead><tr><th>#</th><th>Who</th><th className="num">Time</th><th className="num">Hints</th><th className="num">Mistakes</th>{puzzle.kind === 'quiz' && <th className="num">Points</th>}<th>When</th><th /></tr></thead>
          <tbody>{done.map((x, i) => <tr key={x.id}>
            <td>{i + 1}</td><td>{x.player || 'Someone'}{x.via === 'me' && <span className="badge">you</span>}{x.via === 'book' && <span className="badge">book</span>}</td>
            <td className="num">{fmtMs(x.ms)}</td><td className="num">{x.hints}</td><td className="num">{x.mistakes}</td>{puzzle.kind === 'quiz' && <td className="num">{x.score}/{x.max_score}</td>}
            <td className="muted small">{x.finished_at ? ago(x.finished_at) : ''}</td>
            <td><button type="button" className="btn ghost xs" title="Take it off the list" onClick={async () => { try { await api.del(`${V1}/plays/${x.id}`); load() } catch (e) { toast((e as Error).message, 'err') } }}><Icon name="x" size={13} /></button></td>
          </tr>)}</tbody>
        </table></div>
      </section>
      {going.length > 0 && <section className="card"><div className="card-head"><h3>Still going</h3></div>
        <div className="rows">{going.map(x => <div key={x.id} className="row"><Icon name="clock" size={14} className="muted" /><span className="grow">{x.player || 'Someone'}</span><span className="muted small">started {ago(x.started_at)}</span></div>)}</div></section>}
    </div>
  )
}

/** A copy of the puzzle with its clues and questions in another language. */
function TranslateCopy({ puzzle, data, onClose, onDone }: { puzzle: Puzzle; data: any; onClose: () => void; onDone: (id: number) => void }) {
  const { toast } = useToast()
  const [to, setTo] = useState('de')
  const [busy, setBusy] = useState(false)
  const run = async () => {
    setBusy(true)
    try {
      const d = JSON.parse(JSON.stringify(data))
      const texts: string[] = []
      const put: ((v: string) => void)[] = []
      const add = (v: string | undefined, f: (v: string) => void) => { if (v && v.trim()) { texts.push(v); put.push(f) } }
      if (puzzle.kind === 'crossword') for (const k of Object.keys(d.clues || {})) add(d.clues[k], v => { d.clues[k] = v })
      if (puzzle.kind === 'quiz') for (const q of d.questions || []) { add(q.q, v => { q.q = v }); add(q.hint, v => { q.hint = v }); add(q.explain, v => { q.explain = v }) }
      if (puzzle.kind === 'scramble') for (const w of d.words || []) add(w.hint, v => { w.hint = v })
      if (puzzle.kind === 'wordsearch') for (const w of d.words || []) add(w.clue, v => { w.clue = v })
      if (!texts.length) { toast('Nothing to translate', 'err'); setBusy(false); return }
      const r = await api.post<{ texts: string[] }>(`${V1}/apps/translate`, { texts, to })
      r.texts.forEach((v, i) => put[i](v))
      const c = await api.post<Puzzle>(`${V1}/puzzles/${puzzle.id}/duplicate`)
      await api.put(`${V1}/puzzles/${c.id}`, { data: d, title: `${puzzle.title} (${to})`, language: 'other' })
      announceChange()
      toast('Translated copy made — the answers stay as they were; check them for the new language')
      onDone(c.id)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="A translated copy" onClose={onClose} footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={busy} onClick={run}>{busy && <Spinner />}Translate</button></>}>
      <p className="muted small">Pocket Translate translates the clues and questions into a copy. The answers stay as they are.</p>
      <Field label="Into"><Select ariaLabel="Language" value={to} onChange={setTo} options={[['en', 'English'], ['bs', 'Bosnian'], ['hr', 'Croatian'], ['sr', 'Serbian'], ['de', 'German'], ['fr', 'French'], ['es', 'Spanish'], ['it', 'Italian'], ['tr', 'Turkish'], ['nl', 'Dutch']].map(([v, l]) => ({ value: v, label: l }))} /></Field>
    </Modal>
  )
}
