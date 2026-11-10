// Word search: play (drag a line over a word, or tap its first and last
// letter), make (your words hidden in a grid, maybe a secret message in the
// leftover letters), print.
import '../../styles/kind-wordsearch.css'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import { letters } from '../../lib/puzzlecraft'
import type { WordClue } from '../../lib/types'
import { Icon } from '../icons'
import { Empty, Segmented, Spinner, Switch, useToast } from '../ui'
import { Select } from '../Select'
import { WordsInput } from './common'
import { useWidth } from './Crossword'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shared ----

type Line = [number, number, number, number] // r1 c1 r2 c2

interface WSPublic {
  w: number; h: number; grid: string[]; words: { i: number; len: number; word?: string; clue?: string }[]
  level: string; clues: boolean; has_message: boolean; language: string
}
interface WSWordFull { word: string; clue?: string; r: number; c: number; dr: number; dc: number; len: number; off?: boolean }
interface WSFull { w: number; h: number; grid: string[]; words: WSWordFull[]; level: string; message: string; clues: boolean; language: string }
interface WSProgress { found: number[]; lines: { i: number; line: Line }[]; message: string }

const BANDS = 8
const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v))

/** The squares a straight line goes over. */
function lineCells(ln: Line, w: number): number[] {
  const [r1, c1, r2, c2] = ln
  const n = Math.max(Math.abs(r2 - r1), Math.abs(c2 - c1))
  const sr = Math.sign(r2 - r1), sc = Math.sign(c2 - c1)
  const out: number[] = []
  for (let k = 0; k <= n; k++) out.push((r1 + sr * k) * w + c1 + sc * k)
  return out
}

/** From a first square towards the pointer: the nearest of the 8 directions, kept inside the grid. */
function snapLine(r0: number, c0: number, r: number, c: number, w: number, h: number): Line {
  const dr = r - r0, dc = c - c0
  if (!dr && !dc) return [r0, c0, r0, c0]
  const oct = Math.round(Math.atan2(dr, dc) / (Math.PI / 4))
  const sr = Math.round(Math.sin(oct * Math.PI / 4)), sc = Math.round(Math.cos(oct * Math.PI / 4))
  let len = sr && sc ? Math.round((Math.abs(dr) + Math.abs(dc)) / 2) : Math.max(Math.abs(dr), Math.abs(dc))
  while (len > 0) {
    const er = r0 + sr * len, ec = c0 + sc * len
    if (er >= 0 && ec >= 0 && er < h && ec < w) break
    len--
  }
  return [r0, c0, r0 + sr * len, c0 + sc * len]
}

const straight = (r0: number, c0: number, r: number, c: number) => (r0 === r || c0 === c || Math.abs(r - r0) === Math.abs(c - c0)) && !(r0 === r && c0 === c)
const sameLine = (a: Line, b: Line) => (a[0] === b[0] && a[1] === b[1] && a[2] === b[2] && a[3] === b[3]) || (a[0] === b[2] && a[1] === b[3] && a[2] === b[0] && a[3] === b[1])
const lineOf = (x: WSWordFull): Line => [x.r, x.c, x.r + x.dr * (x.len - 1), x.c + x.dc * (x.len - 1)]

interface Band { line: Line; k: number; cls?: string }

interface GridProps {
  w: number; h: number; grid: string[]; size: number; bands: Band[]; draft?: Line | null; miss?: Line | null
  cursor?: number; emphasis?: Set<number>; className?: string
  boardRef?: React.Ref<HTMLDivElement>
  handlers?: React.HTMLAttributes<HTMLDivElement>
}

/** Letters in squares, with the found words as soft bands behind them. */
function WSGrid({ w, h, grid, size, bands, draft, miss, cursor, emphasis, className = '', boardRef, handlers }: GridProps) {
  const W = w * size, H = h * size
  const stroke = Math.max(10, size * 0.74)
  const seg = (ln: Line, key: string | number, cls: string) => {
    const x1 = ln[1] * size + size / 2, y1 = ln[0] * size + size / 2, x2 = ln[3] * size + size / 2, y2 = ln[2] * size + size / 2
    return <line key={key} className={cls} x1={x1} y1={y1} x2={x2} y2={y2} strokeWidth={stroke} strokeLinecap="round" />
  }
  return (
    <div ref={boardRef} className={`ws-grid ${className}`} style={{ width: W, height: H, ['--ws' as any]: `${size}px` }} {...handlers}>
      <svg className="ws-bands" width={W} height={H} viewBox={`0 0 ${W} ${H}`} aria-hidden="true">
        {bands.map((b, j) => seg(b.line, `b${j}`, `ws-band c${b.k % BANDS} ${b.cls || ''}`))}
        {miss && seg(miss, 'miss', 'ws-band miss')}
        {draft && seg(draft, 'draft', 'ws-band draft')}
      </svg>
      <div className="ws-letters" style={{ gridTemplateColumns: `repeat(${w}, ${size}px)`, gridAutoRows: `${size}px` }}>
        {grid.map((ch, i) => <span key={i} className={`ws-l ${i === cursor ? 'cur' : ''} ${emphasis?.has(i) ? 'left' : ''}`}>{ch}</span>)}
      </div>
    </div>
  )
}

// ---- play ----

function Play({ view, progress, shown, done, onProgress, check, reveal, canReveal, setActions }: KindPlayProps) {
  const { toast } = useToast()
  const d = view.data as WSPublic
  const n = d.w * d.h
  const [st, setSt] = useState<WSProgress>(() => ({
    found: Array.isArray(progress?.found) ? progress.found : [],
    lines: Array.isArray(progress?.lines) ? progress.lines : [],
    message: typeof progress?.message === 'string' ? progress.message : '',
  }))
  const stRef = useRef(st)
  stRef.current = st
  const [draft, setDraft] = useState<Line | null>(null)
  const [miss, setMiss] = useState<Line | null>(null)
  const [anchor, setAnchor] = useState<[number, number] | null>(null)
  const [cursor, setCursor] = useState<number | null>(null)
  const [msgWrong, setMsgWrong] = useState(false)
  const [busy, setBusy] = useState(false)
  const drag = useRef<{ r: number; c: number; moved: boolean; id: number } | null>(null)
  const boardEl = useRef<HTMLDivElement | null>(null)
  const msgInput = useRef<HTMLInputElement>(null)
  const missTimer = useRef(0)
  const [box, bw] = useWidth(600)
  const wide = bw >= 760
  const area = wide ? bw - 260 : bw
  const size = clamp(Math.floor((Math.min(area, 640) - 16) / d.w), 12, 46)
  const shownKey = shown.join(',')
  const shownSet = useMemo(() => new Set(shown), [shownKey]) // eslint-disable-line react-hooks/exhaustive-deps

  const foundSet = useMemo(() => new Set(st.found), [st.found])
  const allFound = d.words.length > 0 && d.words.every(x => foundSet.has(x.i))
  const used = useMemo(() => { const s = new Set<number>(); for (const l of st.lines) for (const i of lineCells(l.line, d.w)) s.add(i); return s }, [st.lines, d.w])
  const leftover = useMemo(() => allFound && d.has_message ? new Set(Array.from({ length: n }, (_, i) => i).filter(i => !used.has(i))) : undefined, [allFound, d.has_message, used, n])

  const save = (next: WSProgress) => { stRef.current = next; setSt(next); onProgress(next) }
  const addFound = (i: number, line: Line) => {
    const cur = stRef.current
    if (cur.found.includes(i)) return cur
    const next = { ...cur, found: [...cur.found, i], lines: [...cur.lines, { i, line }] }
    save(next)
    return next
  }
  const finishIfDone = (next: WSProgress) => {
    const all = d.words.every(x => next.found.includes(x.i))
    if (!all) return
    if (d.has_message) { window.setTimeout(() => msgInput.current?.focus({ preventScroll: false }), 50); return }
    check({ found: next.found, message: '' }, true)
  }

  const submit = async (ln: Line) => {
    if (done) return
    if (stRef.current.lines.some(l => sameLine(l.line, ln))) return
    setDraft(ln)
    const r = await check({ line: ln, found: stRef.current.found, message: stRef.current.message }, false)
    setDraft(null)
    if (!r) return
    const f = typeof r.found === 'number' ? r.found : -1
    if (f >= 0) {
      const next = addFound(f, ln)
      if (!r.solved) finishIfDone(next)
    } else {
      window.clearTimeout(missTimer.current)
      setMiss(ln)
      missTimer.current = window.setTimeout(() => setMiss(null), 650)
    }
  }
  useEffect(() => () => window.clearTimeout(missTimer.current), [])

  const cellOf = (e: React.PointerEvent): [number, number] => {
    const rect = boardEl.current!.getBoundingClientRect()
    return [clamp(Math.floor((e.clientY - rect.top) / size), 0, d.h - 1), clamp(Math.floor((e.clientX - rect.left) / size), 0, d.w - 1)]
  }
  const onDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (done || e.button > 0) return
    e.preventDefault()
    setCursor(null)
    const [r, c] = cellOf(e)
    try { e.currentTarget.setPointerCapture(e.pointerId) } catch { /* old browsers */ }
    // a first letter tapped before: this tap ends the line there
    if (anchor && straight(anchor[0], anchor[1], r, c)) {
      drag.current = { r: anchor[0], c: anchor[1], moved: true, id: e.pointerId }
      setDraft(snapLine(anchor[0], anchor[1], r, c, d.w, d.h))
      return
    }
    drag.current = { r, c, moved: false, id: e.pointerId }
    setDraft([r, c, r, c])
  }
  const onMove = (e: React.PointerEvent<HTMLDivElement>) => {
    const g = drag.current
    if (!g || g.id !== e.pointerId) return
    const [r, c] = cellOf(e)
    if (r !== g.r || c !== g.c) g.moved = true
    setDraft(snapLine(g.r, g.c, r, c, d.w, d.h))
  }
  const onUp = (e: React.PointerEvent<HTMLDivElement>) => {
    const g = drag.current
    if (!g || g.id !== e.pointerId) return
    drag.current = null
    const [r, c] = cellOf(e)
    const ln = snapLine(g.r, g.c, r, c, d.w, d.h)
    if (ln[0] === ln[2] && ln[1] === ln[3]) {
      // a tap: remember it as the first letter (a second tap on it forgets it)
      setDraft(null)
      setAnchor(a => a && a[0] === r && a[1] === c ? null : [r, c])
      return
    }
    setAnchor(null)
    submit(ln)
  }
  const onCancel = () => { drag.current = null; setDraft(null) }

  // keyboard: arrows move, Enter or space on the first letter and again on the last
  const onKey = (ev: React.KeyboardEvent) => {
    if (done || ev.metaKey || ev.ctrlKey || ev.altKey) return
    const cur = cursor ?? 0
    let r = Math.floor(cur / d.w), c = cur % d.w
    const k = ev.key
    if (k.startsWith('Arrow')) {
      ev.preventDefault()
      if (cursor === null) { setCursor(0); return }
      if (k === 'ArrowUp') r--; else if (k === 'ArrowDown') r++; else if (k === 'ArrowLeft') c--; else c++
      r = clamp(r, 0, d.h - 1); c = clamp(c, 0, d.w - 1)
      setCursor(r * d.w + c)
      if (anchor) setDraft(snapLine(anchor[0], anchor[1], r, c, d.w, d.h))
    } else if (k === 'Enter' || k === ' ') {
      ev.preventDefault()
      if (cursor === null) { setCursor(0); return }
      if (!anchor) { setAnchor([r, c]); setDraft([r, c, r, c]); return }
      const ln = snapLine(anchor[0], anchor[1], r, c, d.w, d.h)
      setAnchor(null)
      if (ln[0] === ln[2] && ln[1] === ln[3]) { setDraft(null); return }
      submit(ln)
    } else if (k === 'Escape') { setAnchor(null); setDraft(null) }
  }

  const sendMessage = async () => {
    const m = stRef.current.message.trim()
    if (!m || busy) return
    setBusy(true)
    try {
      const r = await check({ found: stRef.current.found, message: m }, true)
      if (r && !r.solved) setMsgWrong(true)
    } finally { setBusy(false) }
  }

  useEffect(() => {
    const nextWord = d.words.find(x => !stRef.current.found.includes(x.i))
    setActions({
      check: async () => {
        const cur = stRef.current
        const r = await check({ found: cur.found, message: cur.message }, false)
        if (r && !r.message) toast(`${r.filled} of ${r.total} words found`)
      },
      reveal: canReveal && nextWord ? async () => {
        const target = d.words.find(x => !stRef.current.found.includes(x.i))
        if (!target) return
        const v = await reveal(target.i)
        if (v && Array.isArray(v.line) && v.line.length === 4) {
          const next = addFound(target.i, v.line as Line)
          finishIfDone(next)
        }
      } : undefined,
      revealLabel: 'Show a word',
      clear: () => {
        const cur = stRef.current
        const keep = cur.lines.filter(l => shownSet.has(l.i))
        save({ found: keep.map(l => l.i), lines: keep, message: '' })
        setAnchor(null); setDraft(null); setMsgWrong(false)
      },
    })
  }, [st.found.length, canReveal, shownKey]) // eslint-disable-line react-hooks/exhaustive-deps

  const colourOf = useMemo(() => { const m = new Map<number, number>(); st.lines.forEach((l, k) => m.set(l.i, k)); return m }, [st.lines])
  const bands: Band[] = st.lines.map((l, k) => ({ line: l.line, k, cls: shownSet.has(l.i) ? 'given' : '' }))
  const anchorDraft: Line | null = draft ?? (anchor ? [anchor[0], anchor[1], anchor[0], anchor[1]] : null)
  const wordText = (i: number) => { const l = st.lines.find(x => x.i === i); return l ? lineCells(l.line, d.w).map(c => d.grid[c]).join('') : '' }
  const toFind = d.words.length - d.words.filter(x => foundSet.has(x.i)).length

  return (
    <div className={`ws-play ${wide ? 'wide' : ''}`} ref={box}>
      <div className="ws-board-wrap">
        <div className="ws-board" tabIndex={done ? -1 : 0} onKeyDown={onKey} onBlur={() => setCursor(null)} aria-label="Letter grid. Drag over a word, or tap its first and last letter.">
          <WSGrid w={d.w} h={d.h} grid={d.grid} size={size} bands={bands} draft={anchorDraft} miss={miss} cursor={cursor ?? undefined} emphasis={leftover}
            className={done ? 'is-done' : ''} boardRef={el => { boardEl.current = el }}
            handlers={{ onPointerDown: onDown, onPointerMove: onMove, onPointerUp: onUp, onPointerCancel: onCancel, onContextMenu: e => e.preventDefault() }} />
        </div>
        <p className="ws-tip muted small" aria-live="polite">
          {done ? 'All found.' : allFound ? (d.has_message ? 'Every word is found — now the secret message.' : 'Every word is found!')
            : anchor ? 'Now tap the last letter of the word.' : `${toFind} to find. Drag across a word, or tap its first and last letter.`}
        </p>
      </div>
      <div className="ws-side">
        {allFound && d.has_message && !done && <div className="ws-msg">
          <p><Icon name="sparkles" size={15} /> Read the leftover letters in order — what do they say?</p>
          <div className="row">
            <input ref={msgInput} className={`grow ${msgWrong ? 'wrong' : ''}`} value={st.message} placeholder="The secret message" aria-label="The secret message" autoComplete="off" spellCheck={false}
              onChange={e => { setMsgWrong(false); save({ ...stRef.current, message: e.target.value }) }} onKeyDown={e => { if (e.key === 'Enter') sendMessage() }} />
            <button type="button" className="btn primary" onClick={sendMessage} disabled={busy || !st.message.trim()}>{busy ? <Spinner /> : <Icon name="check" size={14} />}That's it</button>
          </div>
          <p className="muted small ws-msg-note">{msgWrong ? 'Not quite — read the letters again, row by row.' : ' '}</p>
        </div>}
        <ul className={`ws-words ${d.clues ? 'clues' : ''}`} aria-label="Words to find">
          {d.words.map(x => {
            const f = foundSet.has(x.i)
            const k = colourOf.get(x.i)
            return (
              <li key={x.i} className={`ws-word ${f ? `found c${(k ?? 0) % BANDS}` : ''}`}>
                <span className="ws-dot" aria-hidden="true" />
                {x.clue ? <span className="ws-w"><span className="ws-clue">{x.clue}</span> <span className="muted small">({x.len})</span>{f && <b className="ws-ans">{wordText(x.i)}</b>}</span>
                  : <span className="ws-w">{x.word}</span>}
                {f && <span className="sr-only"> (found)</span>}
              </li>
            )
          })}
        </ul>
      </div>
    </div>
  )
}

// ---- make ----

const DIRS = [
  { value: 'easy', label: 'Across and down' },
  { value: 'medium', label: '+ diagonal' },
  { value: 'hard', label: '+ backwards' },
] as const
const SIZE_OPTS = Array.from({ length: 26 }, (_, k) => k + 5).map(v => ({ value: String(v), label: String(v) }))
const wordKey = (w: string, lang: string) => letters(w, lang).join('')

function Edit({ data, onChange, language }: KindEditProps) {
  const { toast } = useToast()
  const d = data as WSFull
  const lang = d.language || language
  const hasGrid = !!d.grid?.length && d.w > 0 && d.h > 0
  const [list, setList] = useState<WordClue[]>(() => (d.words || []).map(x => ({ word: x.word, clue: x.clue || '' })))
  const [w, setW] = useState(hasGrid ? d.w : 12)
  const [h, setH] = useState(hasGrid ? d.h : 12)
  const [lvl, setLvl] = useState<string>(d.level === 'expert' ? 'hard' : d.level || 'medium')
  const [msg, setMsg] = useState(d.message || '')
  const [busy, setBusy] = useState(false)
  const [showWords, setShowWords] = useState(true)
  const [box, bw] = useWidth(600)

  const gridWords = useMemo(() => (d.words || []).map(x => wordKey(x.word, lang)).sort().join('|'), [d.words, lang])
  const listWords = useMemo(() => list.map(x => wordKey(x.word, lang)).filter(Boolean).sort().join('|'), [list, lang])
  const stale = hasGrid && (gridWords !== listWords || w !== d.w || h !== d.h || (lvl !== (d.level === 'expert' ? 'hard' : d.level)) || msg.trim() !== (d.message || ''))
  const off = (d.words || []).filter(x => x.off).map(x => x.word)

  // only the clues changed: keep the grid, update the clues
  const takeList = (next: WordClue[]) => {
    setList(next)
    if (!hasGrid) return
    const nk = next.map(x => wordKey(x.word, lang)).filter(Boolean).sort().join('|')
    if (nk !== gridWords) return
    const clues = new Map(next.map(x => [wordKey(x.word, lang), x.clue]))
    const words = d.words.map(x => ({ ...x, clue: clues.get(wordKey(x.word, lang)) ?? x.clue }))
    if (words.some((x, i) => (x.clue || '') !== (d.words[i].clue || ''))) onChange({ ...d, words })
  }

  const make = async () => {
    if (!list.length) { toast('Type some words first', 'err'); return }
    setBusy(true)
    try {
      const r = await api.post<{ made: { data: WSFull; missing?: string[]; message?: string } }>(`${V1}/generate`, { kind: 'wordsearch', words: list, w, h, level: lvl, message: msg.trim(), language: lang })
      const m = r.made.data
      onChange({ ...m, clues: d.clues })
      setW(m.w); setH(m.h)
      if (r.made.missing?.length) toast(`Did not fit: ${r.made.missing.join(', ')}`, 'err')
      else if (r.made.message) toast(r.made.message)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }

  const size = hasGrid ? clamp(Math.floor((Math.min(bw, 560) - 2) / d.w), 14, 36) : 24
  const bands: Band[] = showWords && hasGrid ? d.words.filter(x => !x.off).map((x, k) => ({ line: lineOf(x), k })) : []
  return (
    <div className="ws-edit" ref={box}>
      <section className="ws-edit-sec">
        <h4>Words to hide</h4>
        <WordsInput value={list} onChange={takeList} language={lang} needClues={d.clues} max={60} />
        <label className="row ws-switch">
          <Switch checked={!!d.clues} onChange={v => onChange({ ...d, clues: v })} label="Show clues instead of words" />
          <span className="grow">Show clues instead of words <span className="muted small">— players find the answer to each clue</span></span>
        </label>
      </section>
      <section className="ws-edit-sec">
        <h4>The grid</h4>
        <div className="ed-toolbar">
          <span className="muted small">Across</span>
          <Select ariaLabel="Squares across" value={String(w)} onChange={v => setW(Number(v))} options={SIZE_OPTS} />
          <span className="muted small">Down</span>
          <Select ariaLabel="Squares down" value={String(h)} onChange={v => setH(Number(v))} options={SIZE_OPTS} />
        </div>
        <div className="ws-field">
          <span className="muted small">Words run</span>
          <Segmented size="sm" label="Which ways the words run" value={lvl} onChange={v => setLvl(v)} options={DIRS.map(x => ({ value: x.value as string, label: x.label }))} />
        </div>
        <label className="ws-field">
          <span className="muted small">Secret message <span className="muted">(optional — spelled by the letters no word uses)</span></span>
          <input value={msg} onChange={e => setMsg(e.target.value)} placeholder="Happy birthday Ana" maxLength={120} />
        </label>
        <div className="row wrap">
          <button type="button" className="btn primary" onClick={make} disabled={busy || !list.length}>{busy ? <Spinner /> : <Icon name="grid" size={14} />}{hasGrid ? 'Make the grid again' : 'Make the grid'}</button>
          {hasGrid && <label className="row ws-show"><Switch size="sm" checked={showWords} onChange={setShowWords} label="Show where the words are" /><span className="small">Show where the words are</span></label>}
        </div>
        {stale && <div className="callout info"><Icon name="info" size={15} /><div>You changed something — press <b>Make the grid again</b> to see it.</div></div>}
        {off.length > 0 && <div className="callout warn"><Icon name="alert" size={15} /><div>Did not fit: <b>{off.join(', ')}</b>. Make the grid bigger, or use fewer or shorter words.</div></div>}
      </section>
      <div className="ws-preview">
        {hasGrid ? <WSGrid w={d.w} h={d.h} grid={d.grid} size={size} bands={bands} className="preview" />
          : <Empty title="No grid yet" icon="search">Type your words, then press Make the grid.</Empty>}
      </div>
    </div>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as WSPublic
  const [box, bw] = useWidth(640)
  if (answer) {
    const a = answer as WSFull
    const size = clamp(Math.floor(Math.min(bw, 320) / a.w), 10, 20)
    return (
      <div className="ws-print answer" ref={box}>
        <WSGrid w={a.w} h={a.h} grid={a.grid} size={size} bands={a.words.filter(x => !x.off).map((x, k) => ({ line: lineOf(x), k }))} className="print" />
        {a.message && <p className="ws-print-msg">Secret message: <b>{a.message}</b></p>}
      </div>
    )
  }
  const size = clamp(Math.floor(Math.min(bw, 680) / d.w), 16, large ? 46 : 36)
  const words = d.clues ? d.words : [...d.words].sort((x, y) => (x.word || '').localeCompare(y.word || ''))
  return (
    <div className={`ws-print ${large ? 'large' : ''}`} ref={box}>
      <WSGrid w={d.w} h={d.h} grid={d.grid} size={size} bands={[]} className="print" />
      <ul className={`ws-print-words ${d.clues ? 'clues' : ''}`}>
        {words.map(x => <li key={x.i}><span className="ws-box" aria-hidden="true" />{x.clue ? <>{x.clue} <span className="ws-len">({x.len})</span></> : x.word}</li>)}
      </ul>
      {d.has_message && <p className="ws-print-msg">When every word is found, the leftover letters, read in order, spell a secret message: <span className="ws-line" /></p>}
    </div>
  )
}

export const WordSearch: KindModule = { Play, Edit, Print }
