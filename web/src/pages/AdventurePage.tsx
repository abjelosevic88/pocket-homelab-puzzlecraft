// One escape room / hunt / mystery: the story, the stages with their locks
// and hints, setting up, sharing, a test run, and the host's live view.
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, openAction, useAppState } from '../lib/data'
import { ago } from '../lib/format'
import { ADV_KIND, ADV_KINDS, fmtMs, KIND, LOCKS } from '../lib/puzzlecraft'
import type { Adventure, AdventureKind, Hint, Kind, Lock, PlayView, Stage } from '../lib/types'
import { AdventurePlayer } from '../components/AdventurePlayer'
import { Icon } from '../components/icons'
import { useCrumb } from '../components/Layout'
import { PhotoPick } from '../components/PhotoPick'
import { Select } from '../components/Select'
import { ShareBox, ShareOption } from '../components/ShareBox'
import { Confirm, Empty, Field, Loading, Modal, Segmented, Spinner, useToast } from '../components/ui'
import { AddPuzzles } from './Books'

type Tab = 'story' | 'stages' | 'prep' | 'share' | 'live'

export default function AdventurePage() {
  const { id = '' } = useParams()
  const [sp, setSp] = useSearchParams()
  const tab = (['story', 'stages', 'prep', 'share', 'live'].includes(sp.get('tab') || '') ? sp.get('tab') : 'stages') as Tab
  const navigate = useNavigate()
  const { toast } = useToast()
  const [a, setA] = useState<Adventure | null>(null)
  const [del, setDel] = useState(false)
  const [test, setTest] = useState<PlayView | null>(null)
  useCrumb(a?.title, '/adventures')
  const load = useCallback(() => { api.get<Adventure>(`${V1}/adventures/${id}`).then(setA).catch(() => {}) }, [id])
  useEffect(() => { load() }, [load])
  const save = async (body: Record<string, unknown>, msg?: string) => {
    try { setA(await api.put<Adventure>(`${V1}/adventures/${id}`, body)); announceChange(); if (msg) toast(msg) } catch (e) { toast((e as Error).message, 'err') }
  }
  const go = (t: Tab) => { const n = new URLSearchParams(sp); n.set('tab', t); setSp(n, { replace: true }) }
  if (!a) return <Loading />
  const k = ADV_KIND[a.kind]
  const startTest = async () => { try { setTest(await api.post<PlayView>(`${V1}/adventures/${a.id}/play`)) } catch (e) { toast((e as Error).message, 'err') } }
  return (
    <div className="page adv-page">
      <div className="pp-head">
        <span className="kind-ico"><Icon name={k?.icon || 'key'} size={20} /></span>
        <input className="pp-title-input" defaultValue={a.title} key={a.title} onBlur={e => e.target.value !== a.title && save({ title: e.target.value })} aria-label="Name" />
        <button type="button" className="btn" onClick={startTest} disabled={!a.stages?.length}><Icon name="play" size={15} /><span className="hide-sm">Test run</span></button>
        <a className="btn" href={`/adventures/${a.id}/print`} target="_blank" rel="noopener"><Icon name="printer" size={15} /><span className="hide-sm">Print</span></a>
        <button type="button" className="btn ghost icon-only" aria-label="Delete" title="Delete" onClick={() => setDel(true)}><Icon name="trash" size={16} /></button>
      </div>
      <div className="pp-tabs" role="tablist">
        {([['story', 'Story', 'book-open'], ['stages', 'Stages', 'list'], ['prep', 'Setting up', 'check-square'], ['share', 'Invite', 'link'], ['live', 'Live', 'activity']] as const).map(([t, l, ic]) => (
          <button key={t} type="button" role="tab" aria-selected={tab === t} className={`pp-tab ${tab === t ? 'on' : ''}`} onClick={() => go(t)}>
            <Icon name={ic} size={15} />{l}
            {t === 'stages' && <span className="count">{a.stages?.length || 0}</span>}
            {t === 'prep' && a.prep.some(p => !p.done) && <span className="count">{a.prep.filter(p => !p.done).length}</span>}
            {t === 'live' && a.live > 0 && <span className="dot-on" />}
          </button>
        ))}
      </div>
      {tab === 'story' && <StoryTab a={a} save={save} />}
      {tab === 'stages' && <StagesTab a={a} reload={load} />}
      {tab === 'prep' && <PrepTab a={a} save={save} reload={load} />}
      {tab === 'share' && <div className="page narrow-ish stack">
        <ShareBox what="adventure" id={a.id} on={a.share_on} url={a.share_url || ''} title={a.title} onToggle={on => save({ share_on: on })} onRenew={() => save({ renew: true })}
          options={<ShareOption title="Best times" help="Teams see the fastest at the end." checked={a.board} onChange={v => save({ board: v })} />} />
        <HuntMap a={a} />
      </div>}
      {tab === 'live' && <LiveTab a={a} />}
      {test && <Modal title={`Test run: ${a.title}`} onClose={() => { setTest(null); load() }} size="lg" className="test-run">
        <p className="muted small">Played as you — it shows on the Live tab as “Test run”. Answers are as players type them.</p>
        <AdventurePlayer initial={test} />
      </Modal>}
      {del && <Confirm title="Delete this game?" message={`“${a.title}”, its stages and how teams played it go for good.`} confirmLabel="Delete" onCancel={() => setDel(false)} onConfirm={async () => { await api.del(`${V1}/adventures/${a.id}`); announceChange(); navigate('/adventures') }} />}
    </div>
  )
}

function StoryTab({ a, save }: { a: Adventure; save: (b: Record<string, unknown>) => Promise<void> }) {
  const blur = (k: keyof Adventure) => (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) => { if (e.target.value !== String(a[k] ?? '')) save({ [k]: e.target.value }) }
  return (
    <div className="make-cols">
      <div className="make-main card pad-card stack">
        <Field label="What kind"><Segmented size="sm" value={a.kind} onChange={(v: AdventureKind) => save({ kind: v })} options={ADV_KINDS.map(k => ({ value: k.kind, label: k.name }))} /></Field>
        <Field label="The story" help="What players read before they start. **Bold** and lists work."><textarea rows={6} defaultValue={a.intro} onBlur={blur('intro')} placeholder="Professor Vale has vanished…" /></Field>
        <Field label="The end" help="What they read when they finish."><textarea rows={4} defaultValue={a.finale} onBlur={blur('finale')} placeholder="You escaped!" /></Field>
        <Field label="Your notes" help="Only you see these (and they go on the host sheet)."><textarea rows={3} defaultValue={a.notes} onBlur={blur('notes')} /></Field>
      </div>
      <aside className="make-side card pad-card stack">
        <div className="two-col tight">
          <Field label="Day"><input type="date" defaultValue={a.date} onBlur={blur('date')} /></Field>
          <Field label="Time"><input type="time" defaultValue={a.time} onBlur={blur('time')} /></Field>
        </div>
        <Field label="Where"><input defaultValue={a.place} onBlur={blur('place')} placeholder="Our flat" /></Field>
        <Field label="Who is coming"><input defaultValue={a.players} onBlur={blur('players')} placeholder="Ana, Marko, Lejla, Ivan" /></Field>
        <div className="two-col tight">
          <Field label="Time limit (minutes)" help="0 = none"><input type="number" min={0} max={1440} defaultValue={a.minutes} onBlur={e => save({ minutes: Number(e.target.value) })} /></Field>
          <Field label="A hint costs (minutes)"><input type="number" min={0} max={60} defaultValue={Math.round(a.hint_penalty / 60)} onBlur={e => save({ hint_penalty: Number(e.target.value) * 60 })} /></Field>
        </div>
        <Field label="State"><Segmented size="sm" value={a.status} onChange={v => save({ status: v })} options={[{ value: 'draft', label: 'Being made' }, { value: 'ready', label: 'Ready' }, { value: 'done', label: 'Played' }]} /></Field>
        <Field label="Cover picture">{a.photo_id ? <div className="row"><img src={`${V1}/photos/${a.photo_id}?thumb=1`} alt="" className="thumb-lg" /><button type="button" className="btn sm" onClick={() => save({ photo_id: null })}>Remove</button></div> : <PhotoPick kind="cover" small onPicked={p => save({ photo_id: p.id })} />}</Field>
      </aside>
    </div>
  )
}

function StagesTab({ a, reload }: { a: Adventure; reload: () => void }) {
  const { toast } = useToast()
  const stages = a.stages || []
  const [sel, setSel] = useState<number | null>(stages[0]?.id ?? null)
  const cur = stages.find(s => s.id === sel) || null
  const add = async () => { try { const s = await api.post<Stage>(`${V1}/adventures/${a.id}/stages`, {}); reload(); setSel(s.id) } catch (e) { toast((e as Error).message, 'err') } }
  const move = async (i: number, d: number) => { const ids = stages.map(s => s.id); const j = i + d; if (j < 0 || j >= ids.length) return; [ids[i], ids[j]] = [ids[j], ids[i]]; await api.put(`${V1}/adventures/${a.id}/order`, { ids }); reload() }
  if (!stages.length) return <Empty title="No stages yet" icon="list"><button type="button" className="btn primary sm" onClick={add}><Icon name="plus" size={14} />Add the first stage</button></Empty>
  return (
    <div className="stage-cols">
      <div className="stage-list card">
        {stages.map((s, i) => {
          const lock = LOCKS.find(l => l.value === s.lock)
          return (
            <button key={s.id} type="button" className={`stage-row ${s.id === sel ? 'on' : ''}`} onClick={() => setSel(s.id)}>
              <span className="stage-n">{i + 1}</span>
              <span className="grow"><span className="ellipsis stage-t">{s.title || `Stage ${i + 1}`}</span><span className="muted small">{lock?.label}{s.answers.length ? ` · ${s.answers[0]}` : s.puzzle_title ? ` · ${s.puzzle_title}` : ''}</span></span>
              <span className="row-actions">
                <span role="button" tabIndex={0} className="btn ghost xs" aria-label="Up" onClick={e => { e.stopPropagation(); move(i, -1) }}><Icon name="arrow-up" size={12} /></span>
                <span role="button" tabIndex={0} className="btn ghost xs" aria-label="Down" onClick={e => { e.stopPropagation(); move(i, 1) }}><Icon name="arrow-down" size={12} /></span>
              </span>
            </button>
          )
        })}
        <button type="button" className="stage-row add" onClick={add}><Icon name="plus" size={15} />Add a stage</button>
      </div>
      {cur && <StageEditor key={cur.id} s={cur} n={stages.findIndex(x => x.id === cur.id) + 1} reload={reload} onDeleted={() => setSel(stages.find(x => x.id !== cur.id)?.id ?? null)} />}
    </div>
  )
}

function StageEditor({ s, n, reload, onDeleted }: { s: Stage; n: number; reload: () => void; onDeleted: () => void }) {
  const { toast } = useToast()
  const { settings } = useApp()
  const state = useAppState()
  const [st, setSt] = useState(s)
  const [pickPuzzle, setPickPuzzle] = useState(false)
  const [places, setPlaces] = useState(false)
  const [busy, setBusy] = useState('')
  const [del, setDel] = useState(false)
  const save = async (body: Record<string, unknown>) => {
    try { const x = await api.put<Stage>(`${V1}/stages/${s.id}`, body); setSt(x); reload() } catch (e) { toast((e as Error).message, 'err') }
  }
  const blur = (k: keyof Stage) => (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) => { if (e.target.value !== String(st[k] ?? '')) save({ [k]: e.target.value }) }
  const setHints = (h: Hint[]) => { setSt({ ...st, hints: h }); save({ hints: h }) }
  const aiHints = async () => {
    setBusy('hints')
    try { const r = await api.post<{ hints: string[] }>(`${V1}/ai/hints`, { question: `${st.title}\n${st.story}\n${st.lock_hint}`, answer: st.answers.join(' / ') || st.puzzle_title || '' }); setHints(r.hints.map((t, i) => ({ text: t, after: 3 + i * 4 }))) } catch (e) { toast((e as Error).message, 'err') } finally { setBusy('') }
  }
  const lockHelp: Record<Lock, string> = {
    answer: 'Players type it; capitals, spaces and accents do not matter. Add other spellings you accept.',
    code: 'Digits only, like a padlock (1874). Players turn number wheels.', word: 'A word; players see one box per letter.',
    directions: 'U D L R (or ↑ ↓ ← →): the order players push the direction lock.', colors: 'Colours in order, separated by commas: red, blue, red, green.',
    qr: 'Print the stage’s QR card (Print → cards) and hide it; finding and scanning it opens the stage.', puzzle: 'Solving one of your puzzles opens the stage.', none: 'A story stage: players read it and carry on.',
  }
  return (
    <div className="stage-ed card pad-card stack">
      <div className="row"><h3 className="grow">Stage {n}</h3><button type="button" className="btn ghost sm danger-text" onClick={() => setDel(true)}><Icon name="trash" size={14} />Remove</button></div>
      <Field label="Name"><input defaultValue={st.title} onBlur={blur('title')} /></Field>
      <Field label="What players read" help="The story so far and the puzzle to solve. **Bold** works."><textarea rows={5} defaultValue={st.story} onBlur={blur('story')} /></Field>
      <Field label="How it opens"><Select ariaLabel="Lock" value={st.lock} onChange={v => save({ lock: v })} options={LOCKS.map(l => ({ value: l.value, label: l.label, hint: l.line }))} /></Field>
      <p className="muted small">{lockHelp[st.lock]}</p>
      {['answer', 'code', 'word', 'directions', 'colors'].includes(st.lock) && <>
        <Field label={st.lock === 'answer' ? 'Answers that open it' : 'The combination'}><input defaultValue={st.answers.join(' | ')} key={st.answers.join('|')} onBlur={e => save({ answers: e.target.value.split('|').map(x => x.trim()).filter(Boolean) })} placeholder={st.lock === 'code' ? '1874' : st.lock === 'directions' ? 'UUDLR' : st.lock === 'colors' ? 'red, blue, red, green' : 'tomorrow | the future'} /></Field>
        <Field label="On the lock (optional)" help="What players are told: “A 4-digit lock on the drawer”."><input defaultValue={st.lock_hint} onBlur={blur('lock_hint')} /></Field>
      </>}
      {st.lock === 'puzzle' && <Field label="The puzzle">{st.puzzle_id ? <div className="row"><Icon name={KIND[(st.puzzle_kind || 'crossword') as Kind]?.icon || 'puzzle'} size={15} /><Link to={`/puzzles/${st.puzzle_id}`} className="grow">{st.puzzle_title}</Link><button type="button" className="btn sm" onClick={() => setPickPuzzle(true)}>Another</button></div>
        : <button type="button" className="btn sm" onClick={() => setPickPuzzle(true)}><Icon name="puzzle" size={14} />Choose a puzzle</button>}</Field>}
      {st.lock === 'qr' && st.code && <p className="muted small"><Icon name="qr" size={13} className="inline-ico" /> Card code <code>{st.code}</code> · <button type="button" className="linkish" onClick={() => save({ new_code: true })}>new code</button></p>}
      <Field label="When they open it" help="What players see right after: the next clue’s hiding place, a piece of the story."><textarea rows={2} defaultValue={st.unlock} onBlur={blur('unlock')} placeholder="Inside: an old photo with something written on the back." /></Field>
      <div className="hint-ed">
        <div className="row"><h4 className="grow">Hints</h4>{settings?.ai_help !== false && <button type="button" className="btn xs" onClick={aiHints} disabled={!!busy}>{busy === 'hints' ? <Spinner /> : <Icon name="sparkles" size={13} />}Write hints</button>}</div>
        {st.hints.map((h, i) => <div key={i} className="hint-row">
          <input value={h.text} onChange={e => setSt({ ...st, hints: st.hints.map((x, j) => j === i ? { ...x, text: e.target.value } : x) })} onBlur={() => save({ hints: st.hints })} aria-label={`Hint ${i + 1}`} />
          <label className="hint-after"><input type="number" min={0} max={600} value={h.after} onChange={e => setSt({ ...st, hints: st.hints.map((x, j) => j === i ? { ...x, after: Number(e.target.value) } : x) })} onBlur={() => save({ hints: st.hints })} /><span className="muted small">min</span></label>
          <button type="button" className="btn ghost xs" aria-label="Remove the hint" onClick={() => setHints(st.hints.filter((_, j) => j !== i))}><Icon name="x" size={13} /></button>
        </div>)}
        <button type="button" className="btn xs" onClick={() => setHints([...st.hints, { text: '', after: st.hints.length ? st.hints[st.hints.length - 1].after + 4 : 3 }])}><Icon name="plus" size={13} />A hint</button>
        <p className="muted small">Each hint can be had once players have been at this stage for that many minutes.</p>
      </div>
      <details className="host-only" open={!!(st.hidden || st.props)}>
        <summary><Icon name="eye-off" size={14} />For you, the host</summary>
        <Field label="Where it is hidden"><input defaultValue={st.hidden} onBlur={blur('hidden')} placeholder="Under the blue vase in the hall" /></Field>
        <Field label="What you need"><input defaultValue={st.props} onBlur={blur('props')} placeholder="A 4-digit padlock, a shoebox" /></Field>
      </details>
      <Field label="Place (treasure hunts)">
        <div className="row wrap">
          <input className="grow" defaultValue={st.place_name} onBlur={blur('place_name')} placeholder="The fountain in the square" />
          <button type="button" className="btn sm" disabled={!state('places', 'places:read').ok} title={state('places', 'places:read').ok ? 'Pick one of your saved places' : 'Connect Pocket Places in Settings → Connected apps'} onClick={() => setPlaces(true)}><Icon name="pin" size={14} />From Pocket Places</button>
        </div>
        {st.lat != null && <p className="muted small">At {st.lat.toFixed(5)}, {st.lng?.toFixed(5)} · <button type="button" className="linkish" onClick={() => save({ lat: null, lng: null, place_ref: '' })}>clear</button></p>}
      </Field>
      <Field label="Picture">{st.photo_id ? <div className="row"><img src={`${V1}/photos/${st.photo_id}?thumb=1`} alt="" className="thumb-lg" /><button type="button" className="btn sm" onClick={() => save({ photo_id: null })}>Remove</button></div> : <PhotoPick kind="stage" small onPicked={p => save({ photo_id: p.id })} />}</Field>
      {pickPuzzle && <AddPuzzles single have={[]} onClose={() => setPickPuzzle(false)} onAdd={([pid]) => { save({ puzzle_id: pid, lock: 'puzzle' }); setPickPuzzle(false) }} />}
      {places && <PlacesPick onClose={() => setPlaces(false)} onPick={p => { save({ place_name: p.name, lat: p.lat, lng: p.lng, place_ref: p.id }); setPlaces(false) }} />}
      {del && <Confirm title="Remove this stage?" message="The stage and its hints go." confirmLabel="Remove" onCancel={() => setDel(false)} onConfirm={async () => { await api.del(`${V1}/stages/${s.id}`); onDeleted(); reload() }} />}
    </div>
  )
}

function PlacesPick({ onClose, onPick }: { onClose: () => void; onPick: (p: { id: string; name: string; lat: number; lng: number }) => void }) {
  const { toast } = useToast()
  const [q, setQ] = useState('')
  const [list, setList] = useState<{ id: string; name: string; address: string; lat: number | null; lng: number | null }[] | null>(null)
  const load = (query: string) => { setList(null); api.get<{ places: NonNullable<typeof list> }>(`${V1}/apps/places${query ? `?q=${encodeURIComponent(query)}` : ''}`).then(r => setList(r.places)).catch(e => { toast((e as Error).message, 'err'); setList([]) }) }
  useEffect(() => { load('') }, []) // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <Modal title="Pocket Places" onClose={onClose}>
      <div className="add-search"><Icon name="search" size={15} /><input autoFocus value={q} onChange={e => setQ(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') load(q) }} placeholder="Find a place" /></div>
      {list === null ? <Spinner /> : list.length === 0 ? <p className="muted">No places found.</p> :
        <div className="pick-list tall">{list.map(p => <button key={p.id} type="button" className="pick-row" disabled={p.lat == null} onClick={() => p.lat != null && p.lng != null && onPick({ id: p.id, name: p.name, lat: p.lat, lng: p.lng })}><Icon name="pin" size={14} /><span className="grow"><span className="ellipsis">{p.name}</span><span className="muted small ellipsis">{p.address}</span></span></button>)}</div>}
    </Modal>
  )
}

function PrepTab({ a, save, reload }: { a: Adventure; save: (b: Record<string, unknown>, msg?: string) => Promise<void>; reload: () => void }) {
  const { toast } = useToast()
  const state = useAppState()
  const [text, setText] = useState('')
  const [inv, setInv] = useState(false)
  const items = a.prep
  const set = (prep: typeof items) => save({ prep })
  const send = async (url: string, body?: Record<string, unknown>) => { try { const r = await api.post<{ text: string; url?: string }>(url, body); toast(r.text, 'ok', openAction(r.url)); reload() } catch (e) { toast((e as Error).message, 'err') } }
  const props = (a.stages || []).filter(s => s.props || s.hidden)
  return (
    <div className="make-cols">
      <div className="make-main stack">
        <section className="card">
          <div className="card-head"><h3>Setting up</h3><span className="muted small">{items.filter(i => i.done).length} of {items.length} done</span></div>
          <div className="rows">
            {items.map((it, i) => (
              <div key={it.id} className={`row prep-row ${it.done ? 'done' : ''}`}>
                <button type="button" className={`check-box ${it.done ? 'on' : ''}`} aria-label={it.done ? 'Not done' : 'Done'} onClick={() => set(items.map((x, j) => j === i ? { ...x, done: !x.done } : x))}>{it.done && <Icon name="check" size={12} />}</button>
                <span className="grow">{it.text}</span>
                {it.task_url && <a className="muted small" href={it.task_url} target="_blank" rel="noopener" title="In Pocket Tasks"><Icon name="check-square" size={13} /></a>}
                <button type="button" className="btn ghost xs" aria-label="Remove" onClick={() => set(items.filter((_, j) => j !== i))}><Icon name="x" size={13} /></button>
              </div>
            ))}
            <form className="row" onSubmit={e => { e.preventDefault(); if (text.trim()) { set([...items, { id: '', text: text.trim(), done: false }]); setText('') } }}>
              <Icon name="plus" size={14} className="muted" /><input className="grow bare" value={text} onChange={e => setText(e.target.value)} placeholder="Something to do before the game" />
            </form>
          </div>
        </section>
        {props.length > 0 && <section className="card"><div className="card-head"><h3>Where things go</h3></div>
          <div className="rows">{props.map(s => <div key={s.id} className="row"><span className="stage-n sm">{(a.stages || []).indexOf(s) + 1}</span><span className="grow"><b>{s.title}</b>{s.hidden && <> — {s.hidden}</>}{s.props && <span className="muted small"> · needs {s.props}</span>}</span></div>)}</div>
        </section>}
      </div>
      <aside className="make-side stack">
        <section className="card pad-card stack">
          <h3>Help from your other apps</h3>
          <AppButton ok={state('tasks', 'tasks:write').ok} fix={state('tasks', 'tasks:write').app?.fix} icon="check-square" label="Send the list to Pocket Tasks" onClick={() => send(`${V1}/adventures/${a.id}/tasks`)} />
          <AppButton ok={state('notes', 'notes:write').ok} fix={state('notes', 'notes:write').app?.fix} icon="note" label="Host sheet to Pocket Notes" onClick={() => send(`${V1}/apps/notes`, { what: 'adventure', id: a.id })} />
          <AppButton ok={state('inventory', 'items:read').ok} fix={state('inventory', 'items:read').app?.fix} icon="package" label="Find props in Pocket Inventory" onClick={() => setInv(true)} />
          <AppButton ok={state('calendar', 'calendar:write').ok} fix={state('calendar', 'calendar:write').app?.fix} icon="calendar" label={a.calendar_url ? 'In Pocket Calendar ✓ (add again)' : 'Put it in Pocket Calendar'} onClick={() => a.date ? send(`${V1}/apps/calendar`, { what: 'adventure', id: a.id }) : toast('Give it a day first (Story tab)', 'err')} />
          <a className="btn" href={`/adventures/${a.id}/print`} target="_blank" rel="noopener"><Icon name="printer" size={15} />Print the cards and the host sheet</a>
        </section>
      </aside>
      {inv && <InventoryPick onClose={() => setInv(false)} onPick={x => { set([...items, { id: '', text: `Get the ${x.name}${x.location ? ` (${x.location})` : ''}`, done: false }]); setInv(false) }} />}
    </div>
  )
}

function AppButton({ ok, fix, icon, label, onClick }: { ok: boolean; fix?: string; icon: string; label: string; onClick: () => void }) {
  return <button type="button" className="btn" disabled={!ok} title={ok ? label : (fix || 'Connect it in Settings → Connected apps')} onClick={onClick}><Icon name={icon} size={15} />{label}</button>
}

function InventoryPick({ onClose, onPick }: { onClose: () => void; onPick: (x: { name: string; location: string }) => void }) {
  const { toast } = useToast()
  const [q, setQ] = useState('padlock')
  const [list, setList] = useState<{ id: number; name: string; location: string; quantity: number }[] | null>(null)
  const load = (query: string) => { setList(null); api.get<{ items: NonNullable<typeof list> }>(`${V1}/apps/inventory?q=${encodeURIComponent(query)}`).then(r => setList(r.items)).catch(e => { toast((e as Error).message, 'err'); setList([]) }) }
  useEffect(() => { load(q) }, []) // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <Modal title="Props from Pocket Inventory" onClose={onClose}>
      <div className="add-search"><Icon name="search" size={15} /><input autoFocus value={q} onChange={e => setQ(e.target.value)} onKeyDown={e => { if (e.key === 'Enter') load(q) }} placeholder="padlock, torch, box…" /></div>
      {list === null ? <Spinner /> : list.length === 0 ? <p className="muted">Nothing like that in your inventory.</p> :
        <div className="pick-list tall">{list.map(x => <button key={x.id} type="button" className="pick-row" onClick={() => onPick(x)}><Icon name="package" size={14} /><span className="grow">{x.name}</span><span className="muted small">{x.location}</span></button>)}</div>}
    </Modal>
  )
}

function HuntMap({ a }: { a: Adventure }) {
  const { toast } = useToast()
  const state = useAppState()
  const map = state('map', 'map:write')
  const stops = (a.stages || []).filter(s => s.lat != null)
  if (!stops.length) return null
  return (
    <section className="card pad-card row wrap">
      <Icon name="map" size={18} />
      <div className="grow"><h3>The stops on a map</h3><p className="muted small">{stops.length} {stops.length === 1 ? 'stage has' : 'stages have'} a place.</p></div>
      <button type="button" className="btn" disabled={!map.ok} title={map.ok ? '' : (map.app?.fix || 'Connect Pocket Map')} onClick={async () => { try { const r = await api.post<{ text: string; url?: string }>(`${V1}/adventures/${a.id}/map`); toast(r.text, 'ok', openAction(r.url)) } catch (e) { toast((e as Error).message, 'err') } }}><Icon name="map" size={15} />Put them on Pocket Map</button>
    </section>
  )
}

interface LivePlay { id: number; player: string; via: string; done: number; stages: number; stage: string; started_at: string; finished_at: string | null; ms: number; hints: number; mistakes: number; last_at: string; over: boolean; at_stage_min: number }

function LiveTab({ a }: { a: Adventure }) {
  const [doc, setDoc] = useState<{ plays: LivePlay[]; minutes: number } | null>(null)
  const load = useCallback(() => { api.get<typeof doc>(`${V1}/adventures/${a.id}/live`).then(setDoc).catch(() => {}) }, [a.id])
  useEffect(() => {
    load()
    const t = window.setInterval(() => { if (document.visibilityState === 'visible') load() }, 5000)
    return () => window.clearInterval(t)
  }, [load])
  if (!doc) return <Loading />
  if (!doc.plays.length) return <Empty title="No team has started yet" icon="activity">Invite them with the share link (Invite tab). Teams show up here as they play — where they are, their time, the hints they took.</Empty>
  const playing = doc.plays.filter(p => !p.finished_at)
  const done = doc.plays.filter(p => p.finished_at).sort((x, y) => x.ms - y.ms)
  return (
    <div className="stack">
      {playing.length > 0 && <section className="card">
        <div className="card-head"><h3>Playing now</h3><span className="muted small">updates by itself</span></div>
        <div className="rows">{playing.map(p => (
          <div key={p.id} className="row live-row">
            <span className="grow"><b>{p.player}</b>{p.via === 'me' && <span className="badge">test</span>}<span className="muted small"> · at {p.stage} for {Math.floor(p.at_stage_min)} min</span></span>
            <span className="live-bar" title={`${p.done} of ${p.stages} stages`}><i style={{ width: `${(p.done / Math.max(1, p.stages)) * 100}%` }} /></span>
            <span className="num small live-time">{fmtMs(p.ms)}</span>
            <span className="muted small hide-sm"><Icon name="lightbulb" size={12} />{p.hints}</span>
            <span className="muted small hide-sm">{ago(p.last_at)}</span>
          </div>
        ))}</div>
      </section>}
      {done.length > 0 && <section className="card">
        <div className="card-head"><h3>Finished</h3></div>
        <div className="rows">{done.map((p, i) => <div key={p.id} className="row"><span className="muted num">{i + 1}</span><span className="grow"><b>{p.player}</b>{p.over && <span className="tag">over time</span>}</span><span className="num">{fmtMs(p.ms)}</span><span className="muted small"><Icon name="lightbulb" size={12} />{p.hints}</span></div>)}</div>
      </section>}
    </div>
  )
}
