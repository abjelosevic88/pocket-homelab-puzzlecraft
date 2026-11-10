// Word ladders: play (one row of boxes per step, live "one letter changed"
// feedback against the word above), make (find the shortest way between two
// words, or pick one; edit the steps), print.
import '../../styles/kind-ladder.css'
import React, { useEffect, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import { Icon } from '../icons'
import { Segmented, Spinner, useToast } from '../ui'
import { Select } from '../Select'
import { useWidth } from './Crossword'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shared ----

interface LDPublic { from: string; to: string; steps: number; len: number; language: string }
interface LDData { from: string; to: string; steps: string[]; language: string }

const runes = (s: string) => Array.from((s || '').normalize('NFC').toUpperCase())
const isLetterKey = (k: string) => k.length === 1 && /\p{L}/u.test(k)

/** How many letters differ (null when one of them is not complete). */
function diff(a: string[], b: string[]): number | null {
  if (a.length !== b.length || a.some(x => !x) || b.some(x => !x)) return null
  return a.reduce((n, x, i) => n + (x === b[i] ? 0 : 1), 0)
}

function Feedback({ d, end }: { d: number | null; end?: boolean }) {
  if (d === null) return <span className="ld-fb" />
  if (d === 1) return <span className="ld-fb ok"><Icon name="check" size={13} />{end ? 'One letter from the end' : 'One letter changed'}</span>
  if (d === 0) return <span className={`ld-fb ${end ? 'ok' : 'warn'}`}>{end ? <><Icon name="check" size={13} />Same as the end word</> : 'Same as the word above'}</span>
  return <span className="ld-fb warn">{end ? `${d} letters away from the end` : `${d} letters changed — only one may`}</span>
}

/** A row of letter boxes; changed letters (against the row above) are marked. */
function Row({ word, above, size, cur, curCol, onBox, cls = '' }: { word: string[]; above?: string[]; size: number; cur?: boolean; curCol?: number; onBox?: (k: number) => void; cls?: string }) {
  return (
    <div className={`ld-row ${cls} ${cur ? 'cur' : ''}`} style={{ ['--ld' as any]: `${size}px` }}>
      {word.map((l, k) => {
        const changed = !!above && !!l && !!above[k] && above[k] !== l
        return <span key={k} className={`ld-box ${changed ? 'chg' : ''} ${cur && curCol === k ? 'at' : ''}`} onClick={onBox ? () => onBox(k) : undefined} role={onBox ? 'button' : undefined}
          aria-label={onBox ? `Letter ${k + 1}${l ? `: ${l}` : ', empty'}` : undefined}>{l}</span>
      })}
    </div>
  )
}

// ---- play ----

function Play({ view, progress, shown, done, showMistakes, onProgress, check, reveal, setActions }: KindPlayProps) {
  const d = view.data as LDPublic
  const n = Math.max(0, d.steps || 0)
  const len = d.len || runes(d.from).length
  const from = runes(d.from), to = runes(d.to)
  const [rows, setRows] = useState<string[][]>(() => Array.from({ length: n }, (_, r) => { const a = runes(progress?.answers?.[r] || ''); return Array.from({ length: len }, (_, k) => a[k] || '') }))
  const [given, setGiven] = useState<Set<number>>(() => new Set(shown.map(s => s - 1).filter(r => r >= 0 && r < n)))
  const [wrong, setWrong] = useState<Set<number>>(new Set())
  const [note, setNote] = useState('')
  const firstOpen = rows.findIndex((w, r) => !given.has(r) && w.some(x => !x))
  const [cur, setCur] = useState(firstOpen >= 0 ? firstOpen : 0)
  const [col, setCol] = useState(() => { const w = rows[cur]; const k = w ? w.findIndex(x => !x) : 0; return k >= 0 ? k : 0 })
  const input = useRef<HTMLInputElement>(null)
  const [box, bw] = useWidth(480)
  const size = Math.max(34, Math.min(54, Math.floor((Math.min(bw, 520) - 100) / Math.max(len, 3)) - 6))
  const rowsRef = useRef(rows)
  rowsRef.current = rows
  const locked = (r: number) => done || given.has(r)

  const update = (next: string[][], changed?: number) => {
    setRows(next)
    onProgress({ answers: next.map(w => w.join('')) })
    setNote('')
    if (showMistakes) setWrong(new Set())
    else if (changed !== undefined) setWrong(w => { if (!w.has(changed)) return w; const s = new Set(w); s.delete(changed); return s })
    if (next.every(w => w.every(Boolean))) {
      check({ answers: next.map(w => w.join('')) }, true).then(r => {
        if (!r || r.solved) return
        const ws = new Set(r.wrong || [])
        setWrong(ws)
        const lastD = diff(next[n - 1] || from, to)
        setNote(ws.size ? `${ws.size === 1 ? 'One word does' : `${ws.size} words do`} not work — ${ws.size === 1 ? 'it is' : 'they are'} marked in red (a real word, one letter changed).` : lastD !== null && lastD > 1 ? 'The last word has to be one letter away from the end word.' : 'Not quite — look at the words again.')
      })
    }
  }
  const go = (r: number, k?: number) => {
    if (r < 0 || r >= n) return
    setCur(r)
    const w = rowsRef.current[r]
    setCol(k ?? Math.max(0, w.findIndex(x => !x)))
    input.current?.focus({ preventScroll: true })
  }
  const type = (ch: string) => {
    if (locked(cur) || n === 0) return
    const next = rows.map(w => [...w])
    next[cur][col] = ch.toUpperCase()
    update(next, cur)
    if (col + 1 < len) setCol(col + 1)
    else if (next[cur].every(Boolean)) {
      const nx = next.findIndex((w, r) => r > cur && !given.has(r) && w.some(x => !x))
      if (nx >= 0) { setCur(nx); setCol(Math.max(0, next[nx].findIndex(x => !x))) }
    }
  }
  const back = () => {
    if (locked(cur)) return
    const next = rows.map(w => [...w])
    if (next[cur][col]) { next[cur][col] = ''; update(next, cur); return }
    if (col > 0) { next[cur][col - 1] = ''; setCol(col - 1); update(next, cur) }
  }
  const copyAbove = (r: number) => {
    if (locked(r)) return
    const above = r === 0 ? from : rows[r - 1]
    if (above.some(x => !x)) return
    const next = rows.map((w, q) => q === r ? [...above] : [...w])
    update(next, r)
    setCur(r); setCol(0)
    input.current?.focus({ preventScroll: true })
  }
  const onKey = (ev: React.KeyboardEvent) => {
    if (ev.metaKey || ev.ctrlKey || ev.altKey || done) return
    const k = ev.key
    if (k === 'ArrowLeft') { ev.preventDefault(); setCol(Math.max(0, col - 1)) }
    else if (k === 'ArrowRight') { ev.preventDefault(); setCol(Math.min(len - 1, col + 1)) }
    else if (k === 'ArrowUp') { ev.preventDefault(); if (cur > 0) { setCur(cur - 1) } }
    else if (k === 'ArrowDown' || k === 'Enter') { ev.preventDefault(); if (cur + 1 < n) { setCur(cur + 1); if (k === 'Enter') setCol(0) } }
    else if (k === 'Backspace' || k === 'Delete') { ev.preventDefault(); back() }
    else if (isLetterKey(k)) { ev.preventDefault(); type(k) }
  }

  useEffect(() => {
    setActions({
      check: async () => {
        const r = await check({ answers: rowsRef.current.map(w => w.join('')) })
        if (!r) return
        const ws = new Set(r.wrong || [])
        setWrong(ws)
        setNote(ws.size ? `${ws.size === 1 ? 'One word does' : `${ws.size} words do`} not work yet — marked in red.` : r.filled ? 'Every word so far works.' : 'Type a word in the first empty row.')
      },
      reveal: async () => {
        const open = (r: number) => !given.has(r)
        let r = open(cur) && (rowsRef.current[cur].some(x => !x) || wrong.has(cur)) ? cur : rowsRef.current.findIndex((w, q) => open(q) && (w.some(x => !x) || wrong.has(q)))
        if (r < 0) r = open(cur) ? cur : rowsRef.current.findIndex((_, q) => open(q))
        if (r < 0) return
        const v = await reveal(r + 1)
        if (v?.word) {
          const w = runes(String(v.word))
          setGiven(g => new Set([...g, r]))
          update(rowsRef.current.map((x, q) => q === r ? Array.from({ length: len }, (_, k) => w[k] || '') : x), r)
        }
      },
      revealLabel: 'Give a word',
      clear: () => {
        const next = rowsRef.current.map((w, r) => given.has(r) ? w : w.map(() => ''))
        setRows(next); setWrong(new Set()); setNote(''); onProgress({ answers: next.map(w => w.join('')) })
      },
    })
  }, [cur, given, wrong]) // eslint-disable-line react-hooks/exhaustive-deps

  const lastWord = n > 0 ? rows[n - 1] : from
  return (
    <div className="ld-play" ref={box}>
      <div className="ld-board" onKeyDown={onKey} tabIndex={-1}>
        <div className="ld-step end">
          <span className="ld-side"><span className="ld-tag">Start</span></span>
          <Row word={from} size={size} cls="fixed" />
          <span className="ld-fb" />
        </div>
        {rows.map((w, r) => {
          const above = r === 0 ? from : rows[r - 1]
          const st = wrong.has(r) ? 'wrong' : given.has(r) ? 'given' : ''
          return (
            <div key={r} className={`ld-step ${st}`}>
              <span className="ld-side">
                <button type="button" className="btn xs ghost" disabled={locked(r) || above.some(x => !x)} onClick={() => copyAbove(r)} title="Copy the word above, then change one letter" aria-label={`Copy the word above into row ${r + 1}`}><Icon name="arrow-down" size={14} /></button>
              </span>
              <Row word={w} above={above} size={size} cur={!done && cur === r} curCol={col} cls={st} onBox={locked(r) ? undefined : k => go(r, k)} />
              {given.has(r) ? <span className="ld-fb muted"><Icon name="lightbulb" size={13} />Given</span> : <Feedback d={diff(above, w)} />}
            </div>
          )
        })}
        <div className="ld-step end">
          <span className="ld-side"><span className="ld-tag">End</span></span>
          <Row word={to} above={lastWord.every(Boolean) ? lastWord : undefined} size={size} cls="fixed" />
          {n > 0 ? <Feedback d={diff(lastWord, to)} end /> : <span className="ld-fb" />}
        </div>
        <input ref={input} className="ld-input" aria-label="Type a letter" autoCapitalize="characters" autoComplete="off" autoCorrect="off" spellCheck={false} value=""
          onChange={e => { for (const ch of Array.from(e.target.value)) if (isLetterKey(ch)) type(ch) }} />
      </div>
      <div className="ld-status" aria-live="polite">{note ? <span className={wrong.size ? 'bad' : 'muted'}>{note}</span>
        : <span className="muted">Change one letter at a time, from {d.from} at the top to {d.to} at the bottom. Every step is a real word.</span>}</div>
    </div>
  )
}

// ---- make ----

function Edit({ data, onChange }: KindEditProps) {
  const { toast } = useToast()
  const l: LDData = { from: '', to: '', language: 'en', ...(data || {}), steps: data?.steps || [] }
  const [busy, setBusy] = useState('')
  const [w, setW] = useState<number>(() => Math.min(5, Math.max(3, runes(l.from).length || 4)))
  const [count, setCount] = useState('4')
  const mid = l.steps.length >= 2 ? l.steps.slice(1, -1) : []
  const emit = (from: string, to: string, middle: string[]) => onChange({ ...l, from, to, steps: [from, ...middle, to] })
  const clean = (v: string) => runes(v).filter(ch => /\p{L}/u.test(ch)).join('')
  const gen = async (body: Record<string, unknown>, what: string) => {
    setBusy(what)
    try { const r = await api.post<{ made: { data: LDData } }>(`${V1}/generate`, { kind: 'ladder', ...body }); onChange(r.made.data) } catch (e) { toast((e as Error).message, 'err') } finally { setBusy('') }
  }
  const len = runes(l.from).length
  const sameLen = len > 1 && len === runes(l.to).length
  const chain = [l.from, ...mid, l.to].map(runes)
  return (
    <div className="ld-edit stack">
      <div className="ld-ends">
        <label className="field"><span className="label">Start word</span><input value={l.from} maxLength={8} placeholder="COLD" onChange={e => emit(clean(e.target.value), l.to, mid)} /></label>
        <Icon name="arrow-down" size={18} className="ld-ends-arrow" />
        <label className="field"><span className="label">End word</span><input value={l.to} maxLength={8} placeholder="WARM" onChange={e => emit(l.from, clean(e.target.value), mid)} /></label>
        <button type="button" className="btn primary" onClick={() => gen({ from: l.from, to: l.to }, 'way')} disabled={!!busy || !sameLen}>{busy === 'way' ? <Spinner /> : <Icon name="route" size={14} />}Find the way</button>
      </div>
      {!sameLen && (l.from || l.to) && <p className="muted small">Both words need the same number of letters.</p>}
      <div className="ld-pick row wrap">
        <span className="small"><b>Or pick one for me:</b></span>
        <Segmented size="sm" value={w} onChange={setW} label="Letters" options={[3, 4, 5].map(x => ({ value: x, label: `${x} letters` }))} />
        <div className="ld-count"><Select ariaLabel="Steps" value={count} onChange={setCount} options={[3, 4, 5, 6, 7].map(x => ({ value: String(x), label: `${x} steps` }))} /></div>
        <button type="button" className="btn" onClick={() => gen({ w, steps: Number(count) }, 'pick')} disabled={!!busy}>{busy === 'pick' ? <Spinner /> : <Icon name="shuffle" size={14} />}Pick one</button>
      </div>
      <div className="ld-steps-edit">
        <div className="row"><b className="grow">The way</b><span className="muted small">{Math.max(0, chain.length - 1)} {chain.length - 1 === 1 ? 'step' : 'steps'}, {mid.length} to find</span></div>
        <ol className="ld-chain">
          <li className="ld-chain-row end"><span className="ld-chain-word">{l.from || '—'}</span><span className="ld-tag">Start</span>
            <button type="button" className="btn xs ghost" onClick={() => emit(l.from, l.to, [l.from, ...mid])} disabled={!len} title="Add a step after this" aria-label="Add a step after the start"><Icon name="plus" size={13} /></button></li>
          {mid.map((s, i) => (
            <li key={i} className="ld-chain-row">
              <input value={s} maxLength={len || 8} onChange={e => emit(l.from, l.to, mid.map((x, k) => k === i ? clean(e.target.value) : x))} aria-label={`Step ${i + 1}`} />
              <Feedback d={diff(chain[i], runes(s))} />
              <button type="button" className="btn xs ghost" onClick={() => emit(l.from, l.to, [...mid.slice(0, i + 1), s, ...mid.slice(i + 1)])} title="Add a step after this" aria-label={`Add a step after ${s}`}><Icon name="plus" size={13} /></button>
              <button type="button" className="btn xs ghost" onClick={() => emit(l.from, l.to, mid.filter((_, k) => k !== i))} title="Take this step out" aria-label={`Take ${s} out`}><Icon name="trash" size={13} /></button>
            </li>
          ))}
          <li className="ld-chain-row end"><span className="ld-chain-word">{l.to || '—'}</span><span className="ld-tag">End</span>
            {(l.from || l.to) && <Feedback d={diff(chain[chain.length - 2] || [], runes(l.to))} />}</li>
        </ol>
        <p className="muted small">Players fill in the words between. Each one changes one letter of the word above it.</p>
      </div>
    </div>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as LDPublic
  if (answer) {
    const a = answer as LDData
    return <p className="ld-print-key">{(a.steps?.length ? a.steps : [a.from, a.to]).join(' → ')}</p>
  }
  const len = d.len || runes(d.from).length
  const size = large ? 44 : 34
  const blank = Array.from({ length: len }, () => '')
  return (
    <div className={`ld-print ${large ? 'large' : ''}`}>
      <Row word={runes(d.from)} size={size} cls="fixed" />
      {Array.from({ length: Math.max(0, d.steps) }, (_, r) => <Row key={r} word={blank} size={size} />)}
      <Row word={runes(d.to)} size={size} cls="fixed" />
      <p className="ld-print-help">Change one letter at a time. Every step must be a real word.</p>
    </div>
  )
}

export const Ladder: KindModule = { Play, Edit, Print }
