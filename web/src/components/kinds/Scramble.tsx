// Word scrambles: play (tap the letter tiles into the boxes, or type them;
// circled boxes collect into a final answer), make (words and hints, an
// optional final answer, mix them up), print.
import '../../styles/kind-scramble.css'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import { letters } from '../../lib/puzzlecraft'
import type { WordClue } from '../../lib/types'
import { Icon } from '../icons'
import { Spinner, Switch, useToast } from '../ui'
import { WordsInput } from './common'
import { useWidth } from './Crossword'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shared ----

interface SCPublicWord { scrambled: string; hint: string; circles: number[] | null; len: number }
interface SCPublic { words: SCPublicWord[]; final?: { clue: string; len: number } | null }
interface SCWord { word: string; hint?: string; scrambled: string; circles?: number[] }
interface SCData { words: SCWord[]; final?: { clue: string; answer: string } | null }

const runes = (s: string) => Array.from((s || '').normalize('NFC').toUpperCase())
const isLetterKey = (k: string) => k.length === 1 && /\p{L}|\p{N}/u.test(k)
const FINAL = -1

/** Which tiles the placed letters use up (each placed letter takes the first free tile with it). */
function usedTiles(tiles: string[], placed: string[]): boolean[] {
  const used = tiles.map(() => false)
  for (const l of placed) {
    if (!l) continue
    const k = tiles.findIndex((t, i) => !used[i] && t === l)
    if (k >= 0) used[k] = true
  }
  return used
}

function shuffleOnce<T>(a: T[]): T[] {
  const b = [...a]
  for (let i = b.length - 1; i > 0; i--) { const j = Math.floor(Math.random() * (i + 1)); [b[i], b[j]] = [b[j], b[i]] }
  return b
}

/** Mix a word's letters so it no longer reads as itself. */
function scrambleWord(word: string): string {
  const l = runes(word)
  if (l.length < 2) return l.join('')
  const orig = l.join('')
  for (let t = 0; t < 30; t++) { const s = shuffleOnce(l).join(''); if (s !== orig) return s }
  return orig
}

/** Circle letters in the words that spell the final answer, spread over the words (as the server does). */
function placeCircles(words: SCWord[], answer: string): SCWord[] {
  const need = letters(answer || '', 'en')
  let k = 0
  return words.map((w, i) => {
    const out: SCWord = { ...w, circles: [] }
    if (!need.length) return out
    const l = letters(w.word, 'en')
    let take = Math.floor((need.length - k + words.length - i - 1) / Math.max(1, words.length - i))
    for (let p = 0; p < l.length && take > 0 && k < need.length; p++) if (l[p] === need[k]) { out.circles!.push(p); k++; take-- }
    return out
  })
}

/** A row of answer boxes; circled ones are round. */
function Boxes({ n, filled, circles, size, onBox, state, cur, label }: { n: number; filled: string[]; circles?: Set<number>; size: number; onBox?: (k: number) => void; state?: string; cur?: boolean; label?: string }) {
  return (
    <div className={`scr-boxes ${state || ''} ${cur ? 'cur' : ''}`} style={{ ['--scr' as any]: `${size}px` }} aria-label={label}>
      {Array.from({ length: n }, (_, k) => (
        <span key={k} className={`scr-box ${circles?.has(k) ? 'circ' : ''} ${filled[k] ? 'filled' : ''}`} onClick={onBox ? () => onBox(k) : undefined} role={onBox && filled[k] ? 'button' : undefined}
          aria-label={onBox && filled[k] ? `Take ${filled[k]} back` : undefined}>{filled[k] || ''}</span>
      ))}
    </div>
  )
}

// ---- play ----

function Play({ view, progress, shown, done, onProgress, check, reveal, setActions }: KindPlayProps) {
  const d = view.data as SCPublic
  const words = d.words || []
  const fin = d.final && d.final.len > 0 ? d.final : null
  const [answers, setAnswers] = useState<string[][]>(() => words.map((w, i) => { const a = runes(progress?.answers?.[i] || '').slice(0, w.len); return Array.from({ length: w.len }, (_, k) => a[k] || '') }))
  const [final, setFinal] = useState<string[]>(() => { const a = runes(progress?.final || ''); return Array.from({ length: fin?.len || 0 }, (_, k) => a[k] || '') })
  const [order, setOrder] = useState<number[][]>(() => words.map(w => runes(w.scrambled).map((_, i) => i)))
  const [right, setRight] = useState<Set<number>>(() => new Set([...(progress?.right || []), ...shown]))
  const [wrong, setWrong] = useState<Set<number>>(new Set())
  const [given, setGiven] = useState<Set<number>>(() => new Set(shown))
  const [sel, setSel] = useState(() => { const i = words.findIndex((_, i) => !right.has(i)); return i >= 0 ? i : fin ? FINAL : 0 })
  const [note, setNote] = useState('')
  const [box, bw] = useWidth(600)
  const longest = Math.max(fin?.len || 0, ...words.map(w => w.len), 3)
  const size = Math.max(30, Math.min(46, Math.floor((Math.min(bw, 680) - 28) / longest) - 5))

  const tilesOf = (i: number) => i === FINAL ? collected().filter(Boolean) : runes(words[i]?.scrambled || '')
  const collected = (ans = answers) => words.flatMap((w, i) => (w.circles || []).map(c => ans[i]?.[c] || ''))
  const locked = (i: number) => done || right.has(i) || given.has(i)
  const placed = (i: number) => i === FINAL ? final : answers[i]
  const full = (i: number, a = answers, f = final) => (i === FINAL ? f : a[i]).every(Boolean)

  const st = useRef({ answers, final, right })
  st.current = { answers, final, right }
  const save = (a: string[][], f: string[], r: Set<number>) => onProgress({ answers: a.map(x => x.join('')), final: f.join(''), right: [...r] })
  const commit = (a: string[][], f: string[], changed: number) => {
    setAnswers(a); setFinal(f); setNote('')
    setWrong(w => { if (!w.has(changed)) return w; const n = new Set(w); n.delete(changed); return n })
    save(a, f, right)
    const allFull = words.every((_, i) => full(i, a, f)) && (!fin || full(FINAL, a, f))
    if (allFull) {
      check({ answers: a.map(x => x.join('')), final: f.join('') }, true).then(r => {
        if (!r || r.solved) return
        const rs = new Set(r.right || []), ws = new Set(r.wrong || [])
        setRight(rs); setWrong(ws); save(a, f, rs)
        setNote(ws.size ? `Every box is filled, but ${ws.size === 1 ? 'one answer is' : `${ws.size} answers are`} not right yet.` : '')
      })
    }
  }
  const place = (i: number, letter: string) => {
    if (locked(i)) return false
    const tiles = tilesOf(i)
    const cur = placed(i)
    const used = usedTiles(tiles, cur)
    if (!tiles.some((t, k) => t === letter && !used[k])) { setNote(`There is no ${letter} left to use in ${i === FINAL ? 'the final answer' : 'this word'}.`); return false }
    const at = cur.findIndex(x => !x)
    if (at < 0) return false
    const nextCur = [...cur]; nextCur[at] = letter
    if (i === FINAL) commit(answers, nextCur, i)
    else { const a = answers.map((x, k) => k === i ? nextCur : x); commit(a, trimFinal(a), i) }
    if (at === cur.length - 1 && i !== FINAL) {
      // word full: move on to the next one still to do
      const nx = words.findIndex((_, k) => k > i && !locked(k) && !answers[k].every(Boolean))
      if (nx >= 0) setSel(nx)
    }
    return true
  }
  // circled letters taken out of a word leave the final answer too
  const trimFinal = (a: string[][]) => {
    if (!fin) return final
    const pool = collected(a).filter(Boolean)
    const out = final.map(() => '')
    const used = pool.map(() => false)
    final.forEach((l, k) => { if (!l) return; const j = pool.findIndex((p, q) => !used[q] && p === l); if (j >= 0) { used[j] = true; out[k] = l } })
    return out
  }
  const takeBack = (i: number, k: number) => {
    if (locked(i)) return
    setSel(i)
    const cur = [...placed(i)]
    if (!cur[k]) return
    cur[k] = ''
    if (i === FINAL) commit(answers, cur, i)
    else { const a = answers.map((x, q) => q === i ? cur : x); commit(a, trimFinal(a), i) }
  }
  const backspace = (i: number) => {
    const cur = placed(i)
    let k = cur.length - 1
    while (k >= 0 && !cur[k]) k--
    if (k >= 0) takeBack(i, k)
  }
  const clearWord = (i: number) => {
    if (locked(i)) return
    const empty = placed(i).map(() => '')
    if (i === FINAL) commit(answers, empty, i)
    else { const a = answers.map((x, q) => q === i ? empty : x); commit(a, trimFinal(a), i) }
  }
  const checkOne = async (i: number) => {
    // only this answer is sent, so only it can count as a mistake
    const body = i === FINAL ? { answers: words.map(() => ''), final: final.join('') } : { answers: words.map((_, k) => k === i ? answers[i].join('') : ''), final: '' }
    const r = await check(body)
    if (!r) return
    if ((r.right || []).includes(i)) {
      const rs = new Set([...right, i]); setRight(rs); save(answers, final, rs); setNote('')
      const nx = words.findIndex((_, k) => !rs.has(k) && !given.has(k))
      setSel(nx >= 0 ? nx : fin && !rs.has(FINAL) ? FINAL : i)
    } else { setWrong(w => new Set([...w, i])); setNote('Not that one yet — take letters back by tapping them and try another way.') }
  }
  const allIdx = [...words.map((_, i) => i), ...(fin ? [FINAL] : [])]
  const onKey = (ev: React.KeyboardEvent) => {
    if (ev.metaKey || ev.ctrlKey || ev.altKey || done) return
    if ((ev.target as HTMLElement).tagName === 'BUTTON' && (ev.key === 'Enter' || ev.key === ' ')) return
    const k = ev.key
    const at = allIdx.indexOf(sel)
    if (k === 'ArrowDown' || (k === 'Tab' && !ev.shiftKey)) { if (at + 1 < allIdx.length) { ev.preventDefault(); setSel(allIdx[at + 1]) } }
    else if (k === 'ArrowUp' || (k === 'Tab' && ev.shiftKey)) { if (at > 0) { ev.preventDefault(); setSel(allIdx[at - 1]) } }
    else if (k === 'Backspace' || k === 'Delete') { ev.preventDefault(); backspace(sel) }
    else if (k === 'Enter') { ev.preventDefault(); if (full(sel) && !locked(sel)) checkOne(sel) }
    else if (isLetterKey(k)) { ev.preventDefault(); place(sel, k.toUpperCase()) }
  }

  useEffect(() => {
    setActions({
      check: async () => {
        const { answers: a, final: f } = st.current
        const r = await check({ answers: a.map(x => x.join('')), final: f.join('') })
        if (!r) return
        const rs = new Set([...st.current.right, ...(r.right || [])]); setRight(rs); setWrong(new Set(r.wrong || [])); save(a, f, rs)
        setNote((r.wrong || []).length ? `${r.wrong!.length === 1 ? 'One answer is' : `${r.wrong!.length} answers are`} not right yet — marked in red.` : r.filled ? 'Everything you have filled in is right.' : 'Tap the letters to put them in the boxes.')
      },
      reveal: async () => {
        const target = !locked(sel) && sel !== FINAL ? sel : words.findIndex((_, i) => !locked(i))
        const what = target >= 0 ? target : fin && !locked(FINAL) ? FINAL : -2
        if (what === -2) return
        const v = await reveal(what)
        if (!v) return
        setGiven(g => new Set([...g, what]))
        if (what === FINAL && v.final) {
          const f = letters(String(v.final), 'en').slice(0, fin!.len)
          commit(st.current.answers, Array.from({ length: fin!.len }, (_, k) => f[k] || ''), FINAL)
        } else if (v.answer) {
          const w = runes(String(v.answer))
          const a = st.current.answers.map((x, k) => k === what ? Array.from({ length: words[what].len }, (_, q) => w[q] || '') : x)
          commit(a, trimFinal(a), what)
        }
      },
      revealLabel: 'Show a word',
      clear: () => {
        const a = answers.map((x, i) => given.has(i) || right.has(i) ? x : x.map(() => ''))
        const f = given.has(FINAL) || right.has(FINAL) ? final : final.map(() => '')
        setAnswers(a); setFinal(f); setWrong(new Set()); setNote(''); save(a, f, right)
      },
    })
  }, [sel, answers, final, right, given]) // eslint-disable-line react-hooks/exhaustive-deps

  const card = (i: number) => {
    const isFin = i === FINAL
    const tiles = tilesOf(i)
    const ord = isFin ? tiles.map((_, k) => k) : (order[i] || tiles.map((_, k) => k))
    const cur = placed(i)
    const used = usedTiles(tiles, cur)
    const circ = new Set(isFin ? [] : words[i].circles || [])
    const state = right.has(i) ? 'right' : wrong.has(i) ? 'wrong' : ''
    const isLocked = locked(i)
    const n = isFin ? fin!.len : words[i].len
    return (
      <section key={i} className={`scr-word ${sel === i && !done ? 'sel' : ''} ${state} ${isFin ? 'final' : ''}`} onClick={() => setSel(i)} aria-label={isFin ? 'The final answer' : `Word ${i + 1}`}>
        <div className="scr-word-head">
          <span className="scr-num">{isFin ? <Icon name="star" size={14} /> : i + 1}</span>
          <span className="grow scr-hint">{isFin ? (fin!.clue || 'The final answer, from the circled letters') : (words[i].hint || <span className="muted">Unscramble the letters</span>)}</span>
          <span className="scr-mark" aria-hidden={!state}>{state === 'right' ? <Icon name="check-circle" size={18} /> : state === 'wrong' ? <Icon name="x-circle" size={18} /> : null}</span>
        </div>
        <div className="scr-tiles" style={{ ['--scr' as any]: `${size}px` }} role="group" aria-label={isFin ? 'Circled letters' : 'Letters to use'}>
          {isFin && tiles.length === 0 && <span className="muted small scr-empty">The circled letters from the words above land here.</span>}
          {ord.map(k => tiles[k] === undefined ? null : (
            <button key={k} type="button" className={`scr-tile ${used[k] ? 'used' : ''}`} disabled={isLocked || used[k]} onClick={e => { e.stopPropagation(); setSel(i); place(i, tiles[k]) }}
              aria-label={used[k] ? `${tiles[k]}, used` : `Put ${tiles[k]} in`}>{tiles[k]}</button>
          ))}
        </div>
        <Boxes n={n} filled={cur} circles={circ} size={size} state={state} cur={sel === i && !done} onBox={isLocked ? undefined : k => takeBack(i, k)} label="Your answer" />
        <div className="scr-word-tools">
          {!isFin && <button type="button" className="btn sm ghost" disabled={isLocked} onClick={e => { e.stopPropagation(); setOrder(o => o.map((x, k) => k === i ? shuffleOnce(x) : x)) }}><Icon name="shuffle" size={14} />Mix</button>}
          <button type="button" className="btn sm ghost" disabled={isLocked || !cur.some(Boolean)} onClick={e => { e.stopPropagation(); clearWord(i) }}><Icon name="rotate-ccw" size={14} />Take back</button>
          <span className="spacer" />
          <button type="button" className="btn sm" disabled={isLocked || !full(i)} onClick={e => { e.stopPropagation(); checkOne(i) }}>
            {state === 'right' ? <><Icon name="check" size={14} />Right</> : given.has(i) ? <><Icon name="lightbulb" size={14} />Shown</> : <><Icon name="check" size={14} />Is it right?</>}
          </button>
        </div>
      </section>
    )
  }
  return (
    <div className="scr-play" ref={box} tabIndex={0} onKeyDown={onKey} aria-label="Word scramble. Tap the letters, or type them.">
      {words.map((_, i) => card(i))}
      {fin && card(FINAL)}
      <div className="scr-status" aria-live="polite">{note ? <span className={wrong.size ? 'bad' : 'muted'}>{note}</span> : <span className="muted">Tap a letter to put it in the next box, tap a box to take it back. On a keyboard, just type.</span>}</div>
    </div>
  )
}

// ---- make ----

function Edit({ data, onChange, language }: KindEditProps) {
  const { toast } = useToast()
  const s: SCData = { final: null, ...(data || {}), words: data?.words || [] }
  const lang = language || 'en'
  const [busy, setBusy] = useState(false)
  const [finalOn, setFinalOn] = useState(!!s.final)
  const list: WordClue[] = useMemo(() => (s.words || []).map(w => ({ word: w.word, clue: w.hint || '' })), [s.words])
  const finalAns = s.final?.answer || ''
  const emit = (words: SCWord[], final: SCData['final']) => onChange({ ...s, words: final?.answer ? placeCircles(words, final.answer) : words.map(w => ({ ...w, circles: [] })), final: final || null })
  const onWords = (ws: WordClue[]) => {
    const old = new Map((s.words || []).map(w => [w.word, w]))
    const words = ws.map(w => {
      const word = letters(w.word, 'en').join('')
      const prev = old.get(word)
      return { word, hint: w.clue.trim(), scrambled: prev?.scrambled || scrambleWord(word), circles: prev?.circles || [] }
    }).filter(w => w.word)
    emit(words, s.final)
  }
  const setFinal = (patch: Partial<{ clue: string; answer: string }>) => emit(s.words, { clue: s.final?.clue || '', answer: s.final?.answer || '', ...patch })
  const mix = async () => {
    setBusy(true)
    try {
      const r = await api.post<{ made: { data: SCData } }>(`${V1}/generate`, { kind: 'scramble', words: list, final: finalOn && finalAns.trim() ? s.final : null, language: lang })
      onChange(r.made.data)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  const remix = (i: number) => onChange({ ...s, words: s.words.map((w, k) => k === i ? { ...w, scrambled: scrambleWord(w.word) } : w) })
  const need = letters(finalAns, 'en').length
  const got = (s.words || []).reduce((n, w) => n + (w.circles?.length || 0), 0)
  return (
    <div className="scr-edit stack">
      <div className="field">
        <span className="label">Words and their hints</span>
        <WordsInput value={list} onChange={onWords} language={lang} max={60} />
      </div>
      <div className="scr-final-edit">
        <div className="row">
          <Switch checked={finalOn} onChange={v => { setFinalOn(v); emit(s.words, v ? { clue: s.final?.clue || '', answer: s.final?.answer || '' } : null) }} label="A final answer" />
          <div className="grow"><b>A final answer</b><div className="muted small">Some boxes get a circle; their letters spell one more word to find at the end.</div></div>
        </div>
        {finalOn && <div className="two-col mt-s">
          <label className="field"><span className="label">The final answer</span><input value={finalAns} placeholder="PICNIC" onChange={e => setFinal({ answer: e.target.value })} /></label>
          <label className="field"><span className="label">Its clue</span><input value={s.final?.clue || ''} placeholder="What we are doing on Saturday" onChange={e => setFinal({ clue: e.target.value })} /></label>
        </div>}
        {finalOn && need > 0 && got < need && <p className="small bad scr-warn"><Icon name="alert" size={14} />The words only have {got} of the {need} letters in the right order. Add words with those letters, or put them in another order.</p>}
      </div>
      <div className="row wrap">
        <button type="button" className="btn primary" onClick={mix} disabled={busy || list.length === 0}>{busy ? <Spinner /> : <Icon name="shuffle" size={14} />}Mix them up</button>
        <span className="muted small">New mixes for every word{finalOn ? ' and new circles' : ''}.</span>
      </div>
      {s.words.length > 0 && <div className="scr-preview">
        <div className="muted small">How players see it</div>
        <ol className="scr-preview-list">
          {s.words.map((w, i) => {
            const c = new Set(w.circles || [])
            return (
              <li key={i}>
                <span className="scr-pv-mixed">{runes(w.scrambled).join(' ')}</span>
                <span className="scr-pv-word">{letters(w.word, 'en').map((l, k) => <span key={k} className={c.has(k) ? 'circ' : ''}>{l}</span>)}</span>
                <span className="muted small grow ellipsis">{w.hint}</span>
                <button type="button" className="btn xs ghost" onClick={() => remix(i)} title="Mix this word again" aria-label={`Mix ${w.word} again`}><Icon name="refresh" size={13} /></button>
              </li>
            )
          })}
        </ol>
      </div>}
    </div>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as SCPublic
  if (answer) {
    const a = answer as SCData
    return (
      <ol className="scr-print-key">
        {(a.words || []).map((w, i) => <li key={i}>{w.word}</li>)}
        {a.final?.answer && <li className="fin">Final: {a.final.answer}</li>}
      </ol>
    )
  }
  const size = large ? 36 : 28
  return (
    <div className={`scr-print ${large ? 'large' : ''}`}>
      <ol>
        {(d.words || []).map((w, i) => (
          <li key={i}>
            <div className="scr-print-mixed">{runes(w.scrambled).join(' ')}</div>
            {w.hint && <div className="scr-print-hint">{w.hint}</div>}
            <Boxes n={w.len} filled={[]} circles={new Set(w.circles || [])} size={size} />
          </li>
        ))}
      </ol>
      {d.final && d.final.len > 0 && <div className="scr-print-final">
        <div><b>Final answer</b>{d.final.clue ? ` — ${d.final.clue}` : ''}</div>
        <p className="scr-print-hint">Use the letters in the circles.</p>
        <Boxes n={d.final.len} filled={[]} size={size} />
      </div>}
    </div>
  )
}

export const Scramble: KindModule = { Play, Edit, Print }
