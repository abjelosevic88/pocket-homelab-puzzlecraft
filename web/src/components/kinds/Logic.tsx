// Logic grids: play (tap ✗ / ✓ in the staircase grid, tick off the clues),
// make (categories, the answer, clues picked from lists, or made for you), print.
import '../../styles/kind-logic.css'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import { Icon } from '../icons'
import { Modal, Segmented, Spinner, Switch, useToast } from '../ui'
import { Select } from '../Select'
import { loadThemes } from './common'
import { useWidth } from './Crossword'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shapes (as the server has them) ----

interface Cat { name: string; items: string[]; ordered?: boolean }
interface Ref { cat: number; item: number }
type ClueType = 'same' | 'not' | 'either' | 'less' | 'more' | 'next'
interface Clue { type: ClueType; a: Ref; b: Ref; c?: Ref | null; by?: number; text: string }
interface LogicData { story: string; categories: Cat[]; clues: Clue[]; solution: number[][] }
interface LogicPublic { story: string; categories: Cat[]; clues: string[] }
type Marks = Record<string, number> // 1 = ✓, 2 = ✗

const range = (n: number, from = 0) => Array.from({ length: Math.max(0, n - from) }, (_, i) => i + from)
const keyOf = (ka: number, ia: number, kb: number, ib: number) => `${ka}:${ia}:${kb}:${ib}`
/** The square where item i of category a meets item j of category b (either order). */
const cellKey = (a: number, i: number, b: number, j: number) => a < b ? keyOf(a, i, b, j) : keyOf(b, j, a, i)
const parseKey = (k: string) => k.split(':').map(Number) as [number, number, number, number]

/** The classic staircase: category 0 and then the last ones down the left, 1.. across the top. */
function stairs(K: number) {
  const top = range(K, 1)
  const left = [0, ...range(K - 2).map(g => K - 1 - g)]
  return { top, left, shows: (L: number, T: number) => L === 0 || T < L }
}

/** How many ✓ each row and column of each block has (a ✓ crosses out the rest of them). */
function tally(marks: Marks) {
  const rows = new Map<string, number>(), cols = new Map<string, number>()
  for (const [k, v] of Object.entries(marks)) {
    if (v !== 1) continue
    const [ka, ia, kb, ib] = parseKey(k)
    const r = `${ka}:${ia}:${kb}`, c = `${ka}:${kb}:${ib}`
    rows.set(r, (rows.get(r) || 0) + 1)
    cols.set(c, (cols.get(c) || 0) + 1)
  }
  return { rows, cols }
}
type CellState = '' | 'yes' | 'no' | 'auto' | 'clash'
function stateOf(k: string, marks: Marks, t: ReturnType<typeof tally>): CellState {
  const [ka, ia, kb, ib] = parseKey(k)
  const rc = t.rows.get(`${ka}:${ia}:${kb}`) || 0, cc = t.cols.get(`${ka}:${kb}:${ib}`) || 0
  const m = marks[k]
  if (m === 1) return rc > 1 || cc > 1 ? 'clash' : 'yes'
  if (m === 2) return 'no'
  return rc || cc ? 'auto' : ''
}

/** What the ✓ marks between category 0 and the others say (−1 = not decided yet). */
function solutionOf(marks: Marks, K: number, n: number): number[][] {
  return range(K).map(k => k === 0 ? range(n) : range(n).map(i => {
    const js = range(n).filter(j => marks[keyOf(0, i, k, j)] === 1)
    return js.length === 1 ? js[0] : -1
  }))
}
function complete(sol: number[][], n: number) {
  for (let k = 1; k < sol.length; k++) {
    if (sol[k].some(v => v < 0)) return false
    if (new Set(sol[k]).size !== n) return false
  }
  return sol.length > 1
}
/** The ✓ squares a check says are wrong (k*100+i). */
function wrongKeys(list: number[] | undefined, marks: Marks, n: number) {
  const out = new Set<string>()
  for (const w of list || []) {
    const k = Math.floor(w / 100), i = w % 100
    for (let j = 0; j < n; j++) if (marks[keyOf(0, i, k, j)] === 1) out.add(keyOf(0, i, k, j))
  }
  return out
}
function cleanMarks(m: any): Marks {
  const out: Marks = {}
  if (m && typeof m === 'object') for (const [k, v] of Object.entries(m)) if ((v === 1 || v === 2) && /^\d+:\d+:\d+:\d+$/.test(k)) out[k] = v
  return out
}

/** Label sizes from the longest names (so the grid never jumps). */
function metrics(cats: Cat[], print = false) {
  const K = cats.length
  const { top, left } = stairs(K)
  const fs = print ? 11.5 : 13
  const len = (ks: number[]) => Math.max(3, ...ks.flatMap(k => (cats[k]?.items || []).map(s => s.length)))
  const labelW = Math.round(Math.min(150, Math.max(56, len(left) * fs * 0.58 + 16)))
  const headH = Math.round(Math.min(130, Math.max(44, len(top) * fs * 0.58 + 14)))
  return { labelW, headH, catW: 26, catH: 26 }
}

// ---- the grid ----

interface GridProps {
  cats: Cat[]; size: number; marks: Marks; given?: Set<string>; wrong?: Set<string>
  onCell?: (k: string, to?: number) => void; onFocusKey?: (k: string) => void; print?: boolean
}

function LGGrid({ cats, size, marks, given, wrong, onCell, onFocusKey, print }: GridProps) {
  const K = cats.length, n = cats[0]?.items.length || 0
  const { top, left, shows } = stairs(K)
  const t = useMemo(() => tally(marks), [marks])
  const [foc, setFoc] = useState({ r: 0, c: 0 })
  const [hov, setHov] = useState<{ r: number; c: number } | null>(null)
  const table = useRef<HTMLTableElement>(null)
  const m = metrics(cats, print)
  const hl = hov || (onCell ? foc : null)
  const style = { ['--lg-s' as any]: `${size}px`, ['--lg-lw' as any]: `${m.labelW}px`, ['--lg-hh' as any]: `${m.headH}px`, ['--lg-cw' as any]: `${m.catW}px`, ['--lg-ch' as any]: `${m.catH}px` }
  const iconSize = Math.max(12, Math.round(size * 0.5))
  const go = (r: number, c: number) => {
    const el = table.current?.querySelector<HTMLElement>(`[data-r="${r}"][data-c="${c}"]`)
    if (el) { el.focus(); setFoc({ r, c }) }
  }
  const onKey = (e: React.KeyboardEvent) => {
    const el = e.target as HTMLElement
    const k = el.dataset.k
    if (!k || e.metaKey || e.ctrlKey || e.altKey) return
    const r = Number(el.dataset.r), c = Number(el.dataset.c)
    const key = e.key.toLowerCase()
    if (key === 'arrowright') { e.preventDefault(); go(r, c + 1) }
    else if (key === 'arrowleft') { e.preventDefault(); go(r, c - 1) }
    else if (key === 'arrowdown') { e.preventDefault(); go(r + 1, c) }
    else if (key === 'arrowup') { e.preventDefault(); go(r - 1, c) }
    else if (key === 'x' || key === 'n') { e.preventDefault(); onCell?.(k, 2) }
    else if (key === 'v' || key === 'y') { e.preventDefault(); onCell?.(k, 1) }
    else if (key === 'backspace' || key === 'delete') { e.preventDefault(); onCell?.(k, 0) }
  }
  const cell = (L: number, i: number, T: number, j: number, r: number, c: number) => {
    const k = cellKey(L, i, T, j)
    const st = stateOf(k, marks, t)
    const glyph = st === 'yes' || st === 'clash' ? <Icon name="check" size={iconSize} /> : st === 'no' || st === 'auto' ? <Icon name="x" size={iconSize} /> : null
    const cls = `lg-cell ${st} ${given?.has(k) ? 'given' : ''} ${wrong?.has(k) ? 'wrong' : ''}`
    const word = st === 'yes' ? 'yes' : st === 'clash' ? 'yes (clashes with another yes)' : st === 'no' ? 'no' : st === 'auto' ? 'no (worked out for you)' : 'empty'
    return (
      <td key={c} className={j === n - 1 ? 'je' : ''} onPointerEnter={e => { if (e.pointerType === 'mouse') setHov({ r, c }) }}>
        {print ? <span className={cls}>{glyph}</span>
          : <button type="button" className={cls} data-k={k} data-r={r} data-c={c} tabIndex={foc.r === r && foc.c === c ? 0 : -1} disabled={!onCell}
            aria-label={`${cats[L].items[i]} and ${cats[T].items[j]}: ${word}`} onClick={() => onCell?.(k)}
            onFocus={() => { setFoc({ r, c }); onFocusKey?.(k) }}>{glyph}</button>}
      </td>
    )
  }
  return (
    <table ref={table} className={`lg-grid ${print ? 'print' : ''}`} style={style} onKeyDown={onKey} onPointerLeave={() => setHov(null)}>
      <thead>
        <tr className="lg-h1">
          <th className="lg-corner" colSpan={2} rowSpan={2} />
          {top.map(T => <th key={T} colSpan={n} className="lg-cat-top"><span>{cats[T].name}</span></th>)}
        </tr>
        <tr className="lg-h2">
          {top.flatMap((T, ti) => cats[T].items.map((it, j) => (
            <th key={`${T}-${j}`} className={`lg-item-top ${j === n - 1 ? 'je' : ''} ${hl && hl.c === ti * n + j ? 'hl' : ''}`} title={it}><span>{it}</span></th>
          )))}
        </tr>
      </thead>
      <tbody>
        {left.flatMap((L, g) => cats[L].items.map((it, i) => {
          const r = g * n + i
          return (
            <tr key={`${L}-${i}`} className={i === n - 1 ? 'ie' : ''}>
              {i === 0 && <th rowSpan={n} className="lg-cat-left"><span>{cats[L].name}</span></th>}
              <th className={`lg-item-left ${hl && hl.r === r ? 'hl' : ''}`} title={it}><span>{it}</span></th>
              {top.flatMap((T, ti) => shows(L, T) ? range(n).map(j => cell(L, i, T, j, r, ti * n + j)) : [])}
            </tr>
          )
        }))}
      </tbody>
    </table>
  )
}

// ---- play ----

function Play({ view, progress, done, onProgress, check, reveal, setActions }: KindPlayProps) {
  const d = view.data as LogicPublic
  const cats = d.categories || []
  const K = cats.length, n = cats[0]?.items.length || 0
  const [marks, setMarks] = useState<Marks>(() => cleanMarks(progress?.marks))
  const [given, setGiven] = useState<Set<string>>(() => new Set(Array.isArray(progress?.given) ? progress.given.filter((k: unknown) => typeof k === 'string') : []))
  const [ticked, setTicked] = useState<Set<number>>(() => new Set(Array.isArray(progress?.ticked) ? progress.ticked.filter((v: unknown) => typeof v === 'number') : []))
  const [wrong, setWrong] = useState<Set<string>>(new Set())
  const [note, setNote] = useState('')
  const [box, bw] = useWidth(700)
  const marksRef = useRef(marks), givenRef = useRef(given), tickedRef = useRef(ticked), focRef = useRef('')
  const checkRef = useRef(check), revealRef = useRef(reveal), progressRef = useRef(onProgress)
  checkRef.current = check
  progressRef.current = onProgress
  revealRef.current = reveal

  const wide = bw >= 900
  const m = metrics(cats)
  const avail = (wide ? bw - 320 : bw) - m.catW - m.labelW - 6
  const size = Math.max(30, Math.min(40, Math.floor(avail / Math.max(1, n * (K - 1)))))

  const save = () => progressRef.current({ marks: marksRef.current, given: [...givenRef.current], ticked: [...tickedRef.current] })
  const update = (next: Marks) => {
    marksRef.current = next
    setMarks(next)
    setNote('')
    save()
    const sol = solutionOf(next, K, n)
    if (complete(sol, n)) checkRef.current({ solution: sol }, true).then(r => {
      if (r && !r.solved) { const w = wrongKeys(r.wrong, marksRef.current, n); setWrong(w); setNote(w.size ? 'Every row is filled, but not all of it is right yet — the red ones are wrong.' : '') }
    })
  }
  const onCell = (k: string, to?: number) => {
    if (done || givenRef.current.has(k)) return
    const cur = marksRef.current[k] || 0
    const auto = !cur && stateOf(k, marksRef.current, tally(marksRef.current)) === 'auto'
    const nx = to ?? (cur === 0 ? (auto ? 1 : 2) : cur === 2 ? 1 : 0)
    if (nx === cur) return
    const next = { ...marksRef.current }
    if (nx) next[k] = nx; else delete next[k]
    setWrong(w => { if (!w.has(k)) return w; const s = new Set(w); s.delete(k); return s })
    update(next)
  }
  const tick = (i: number) => {
    if (done) return
    const s = new Set(tickedRef.current)
    if (s.has(i)) s.delete(i); else s.add(i)
    tickedRef.current = s
    setTicked(s)
    save()
  }

  useEffect(() => {
    setActions({
      check: async () => {
        const sol = solutionOf(marksRef.current, K, n)
        const r = await checkRef.current({ solution: sol }, true)
        if (!r || r.solved) return
        const w = wrongKeys(r.wrong, marksRef.current, n)
        setWrong(w)
        const decided = sol.slice(1).reduce((a, row) => a + row.filter(v => v >= 0).length, 0)
        setNote(w.size ? `${w.size} ${w.size === 1 ? 'match is' : 'matches are'} wrong — marked in red.` : decided ? 'Nothing is wrong so far. Keep going!' : 'Put a ✓ in the top row of blocks first — those are the ones that get checked.')
      },
      reveal: async () => {
        const cur = marksRef.current
        const sol = solutionOf(cur, K, n)
        const givenRows = new Set([...givenRef.current].map(k => { const [, ia, kb] = parseKey(k); return kb * 100 + ia }))
        let what = -1
        if (focRef.current) { const [ka, ia, kb] = parseKey(focRef.current); if (ka === 0 && !givenRows.has(kb * 100 + ia)) what = kb * 100 + ia }
        for (let k = 1; k < K && what < 0; k++) for (let i = 0; i < n && what < 0; i++) if (sol[k][i] < 0 && !givenRows.has(k * 100 + i)) what = k * 100 + i
        for (let k = 1; k < K && what < 0; k++) for (let i = 0; i < n && what < 0; i++) if (!givenRows.has(k * 100 + i)) what = k * 100 + i
        if (what < 0) return
        const v = await revealRef.current(what)
        if (!v || typeof v.match !== 'number') return
        const k = Number(v.cat), i = Number(v.item), j = Number(v.match)
        const next = { ...marksRef.current }
        for (let x = 0; x < n; x++) {
          if (next[keyOf(0, i, k, x)] === 1) delete next[keyOf(0, i, k, x)]
          if (next[keyOf(0, x, k, j)] === 1) delete next[keyOf(0, x, k, j)]
        }
        const gk = keyOf(0, i, k, j)
        next[gk] = 1
        const g = new Set(givenRef.current).add(gk)
        givenRef.current = g
        setGiven(g)
        setWrong(new Set())
        update(next)
      },
      revealLabel: 'Show a match',
      clear: () => {
        const next: Marks = {}
        for (const k of givenRef.current) next[k] = 1
        marksRef.current = next
        tickedRef.current = new Set()
        setMarks(next)
        setTicked(new Set())
        setWrong(new Set())
        setNote('')
        save()
      },
    })
  }, [K, n]) // eslint-disable-line react-hooks/exhaustive-deps

  if (K < 2 || n < 1) return <div className="callout info"><Icon name="info" size={15} /><div>This logic grid has no categories yet.</div></div>
  return (
    <div className="lg-play" ref={box}>
      {d.story && <p className="lg-story">{d.story}</p>}
      <div className={`lg-layout ${wide ? 'wide' : ''}`}>
        <div className="lg-board">
          <div className="lg-scroll">
            <LGGrid cats={cats} size={size} marks={marks} given={given} wrong={wrong} onCell={done ? undefined : onCell} onFocusKey={k => { focRef.current = k }} />
          </div>
          <p className="lg-note small" aria-live="polite">{note || (done ? 'Solved — well done.' : 'Tap a square: once for ✗ (no), again for ✓ (yes), again to clear. A ✓ crosses out the rest of its row and column for you, in grey.')}</p>
        </div>
        <section className="lg-clues">
          <h4>Clues <span className="muted small">{ticked.size} of {d.clues.length} ticked off</span></h4>
          <ol>
            {d.clues.map((c, i) => (
              <li key={i}>
                <button type="button" className={`lg-clue ${ticked.has(i) ? 'on' : ''}`} onClick={() => tick(i)} disabled={done} aria-pressed={ticked.has(i)} title={ticked.has(i) ? 'Untick this clue' : 'Tick off this clue when you have used it'}>
                  <span className="lg-tick"><Icon name="check" size={13} /></span>
                  <span className="lg-num">{i + 1}.</span>
                  <span className="lg-text">{c}</span>
                </button>
              </li>
            ))}
          </ol>
        </section>
      </div>
    </div>
  )
}

// ---- clue words and truth (the same rules as the server) ----

function refName(cats: Cat[], r?: Ref | null) {
  if (!r || r.cat < 0 || r.cat >= cats.length || r.item < 0 || r.item >= cats[r.cat].items.length) return '?'
  return cats[r.cat].items[r.item]
}
function clueText(cats: Cat[], c: Clue): string {
  const a = refName(cats, c.a), b = refName(cats, c.b)
  const byK = c.by ?? 0
  const by = byK >= 0 && byK < cats.length ? cats[byK].name.toLowerCase() : ''
  switch (c.type) {
    case 'same': return `${a} goes with ${b}.`
    case 'not': return `${a} does not go with ${b}.`
    case 'either': return `${a} goes with either ${b} or ${c.c ? refName(cats, c.c) : '?'}.`
    case 'less': return `${a} comes before ${b} (by ${by}).`
    case 'more': return `${a} comes after ${b} (by ${by}).`
    case 'next': return `${a} and ${b} are next to each other (by ${by}).`
  }
  return ''
}
function rowOf(sol: number[][], r: Ref) { return r.cat === 0 ? r.item : (sol[r.cat] || []).indexOf(r.item) }
function clueHolds(c: Clue, sol: number[][]): boolean {
  const a = rowOf(sol, c.a), b = rowOf(sol, c.b)
  switch (c.type) {
    case 'same': return a === b
    case 'not': return a !== b
    case 'either': { if (!c.c) return false; const cc = rowOf(sol, c.c); return (a === b) !== (a === cc) }
    case 'less': case 'more': case 'next': {
      const row = sol[c.by ?? 0] || []
      const va = row[a], vb = row[b]
      if (va === undefined || vb === undefined) return false
      if (c.type === 'less') return va < vb
      if (c.type === 'more') return va > vb
      return Math.abs(va - vb) === 1
    }
  }
  return true
}

// ---- make ----

const TYPES: { value: ClueType; label: string }[] = [
  { value: 'same', label: 'goes with' }, { value: 'not', label: 'does not go with' }, { value: 'either', label: 'goes with one of' },
  { value: 'less', label: 'comes before' }, { value: 'more', label: 'comes after' }, { value: 'next', label: 'is next to' },
]
const ORDER_TYPES = new Set<ClueType>(['less', 'more', 'next'])
const isPerm = (p: unknown, n: number): p is number[] => Array.isArray(p) && p.length === n && new Set(p).size === n && p.every(v => Number.isInteger(v) && v >= 0 && v < n)
const STARTER: Cat[] = [{ name: 'Person', items: ['', '', '', ''] }, { name: 'Pet', items: ['', '', '', ''] }, { name: 'Colour', items: ['', '', '', ''] }]

function normLogic(d: any): LogicData {
  let cats: Cat[] = Array.isArray(d?.categories) && d.categories.length >= 2
    ? d.categories.map((c: any) => ({ name: String(c?.name ?? ''), items: Array.isArray(c?.items) ? c.items.map((s: unknown) => String(s ?? '')) : [], ordered: !!c?.ordered }))
    : STARTER.map(c => ({ ...c, items: [...c.items] }))
  const n = Math.max(2, cats[0].items.length)
  cats = cats.map(c => ({ ...c, items: range(n).map(i => c.items[i] ?? '') }))
  const K = cats.length
  const solution = range(K).map(k => isPerm(d?.solution?.[k], n) ? [...d.solution[k]] : range(n))
  const ok = (r: any): r is Ref => !!r && Number.isInteger(r.cat) && Number.isInteger(r.item) && r.cat >= 0 && r.cat < K && r.item >= 0 && r.item < n
  const clues: Clue[] = (Array.isArray(d?.clues) ? d.clues : []).filter((c: any) => c && ok(c.a) && ok(c.b))
    .map((c: any) => ({ type: TYPES.some(t => t.value === c.type) ? c.type : 'same', a: c.a, b: c.b, c: ok(c.c) ? c.c : undefined, by: Number.isInteger(c.by) && c.by >= 0 && c.by < K ? c.by : 0, text: String(c.text ?? '') }))
  return { story: String(d?.story ?? ''), categories: cats, clues, solution }
}

const itemLabel = (cats: Cat[], k: number, j: number) => cats[k].items[j] || `${cats[k].name || `Category ${k + 1}`} ${j + 1}`

function Edit({ data, onChange }: KindEditProps) {
  const L = useMemo(() => normLogic(data), [data])
  const cats = L.categories
  const K = cats.length, n = cats[0].items.length
  const [making, setMaking] = useState(false)
  const set = (patch: Partial<LogicData>) => onChange({ ...L, ...patch })

  /** New categories: clue words follow, unless someone wrote their own. */
  const setCats = (next: Cat[], patch: Partial<LogicData> = {}) => {
    const clues = (patch.clues || L.clues).map(c => !c.text || c.text === clueText(cats, c) ? { ...c, text: clueText(next, c) } : c)
    set({ ...patch, categories: next, clues })
  }
  const setCount = (K2: number, n2: number) => {
    const next: Cat[] = range(K2).map(k => {
      const c = cats[k] || { name: '', items: [], ordered: false }
      return { ...c, items: range(n2).map(i => c.items[i] ?? '') }
    })
    const solution = range(K2).map(k => n2 === n && L.solution[k] ? [...L.solution[k]] : range(n2))
    const ok = (r?: Ref | null) => !!r && r.cat < K2 && r.item < n2
    const clues = L.clues.filter(c => ok(c.a) && ok(c.b) && (c.type !== 'either' || ok(c.c)) && (c.by ?? 0) < K2)
    setCats(next, { solution, clues })
  }
  const setCat = (k: number, patch: Partial<Cat>) => setCats(cats.map((c, i) => i === k ? { ...c, ...patch } : c))
  const setItem = (k: number, j: number, v: string) => setCat(k, { items: cats[k].items.map((s, i) => i === j ? v : s) })
  const pick = (k: number, i: number, j: number) => {
    const solution = L.solution.map(r => [...r])
    const other = solution[k].indexOf(j)
    if (other >= 0) solution[k][other] = solution[k][i]
    solution[k][i] = j
    set({ solution })
  }
  const setClue = (i: number, next: Clue) => set({ clues: L.clues.map((c, x) => x === i ? next : c) })
  const addClue = () => {
    const b: Ref = { cat: 1, item: L.solution[1][(n > 1 ? 1 : 0)] }
    const c: Clue = { type: 'not', a: { cat: 0, item: 0 }, b, by: 0, text: '' }
    set({ clues: [...L.clues, { ...c, text: clueText(cats, c) }] })
  }
  const moveClue = (i: number, by: number) => {
    const j = i + by
    if (j < 0 || j >= L.clues.length) return
    const clues = [...L.clues]
    ;[clues[i], clues[j]] = [clues[j], clues[i]]
    set({ clues })
  }
  const hasOrder = cats.some(c => c.ordered)
  return (
    <div className="lg-edit">
      <div className="ed-toolbar">
        <span className="muted small grow">Build your own below, or let it be made for you.</span>
        <button type="button" className="btn sm primary" onClick={() => setMaking(true)}><Icon name="sparkles" size={14} />Make it for me</button>
      </div>

      <section className="lg-ed-sec">
        <h4>The story</h4>
        <textarea value={L.story} rows={2} placeholder="A few words that set the scene, e.g. “Four friends each have a pet with a different name. Work out who has which.”" onChange={e => set({ story: e.target.value })} aria-label="The story" />
      </section>

      <section className="lg-ed-sec">
        <div className="row wrap lg-ed-head">
          <h4 className="grow">Categories</h4>
          <Segmented size="sm" label="How many categories" value={K} onChange={v => setCount(v, n)} options={[2, 3, 4].map(v => ({ value: v, label: `${v} groups` }))} />
          <Segmented size="sm" label="Items in each" value={n} onChange={v => setCount(K, v)} options={[3, 4, 5].map(v => ({ value: v, label: `${v} each` }))} />
        </div>
        <p className="muted small ed-help">The first group is what the puzzle is about (usually people). Switch on “In order” for things like times, ages or places in a race — then clues can say “before”, “after” and “next to”.</p>
        <div className="lg-cats">
          {cats.map((c, k) => (
            <div key={k} className="lg-cat-card">
              <div className="row">
                <input className="grow" value={c.name} placeholder={k === 0 ? 'e.g. Person' : `Group ${k + 1}, e.g. Pet`} aria-label={`Name of group ${k + 1}`} onChange={e => setCat(k, { name: e.target.value })} />
              </div>
              <label className="row lg-order small"><Switch size="sm" checked={!!c.ordered} onChange={v => setCat(k, { ordered: v })} label="In order" /><span>In order</span></label>
              <div className="lg-items">
                {c.items.map((it, j) => <input key={j} value={it} placeholder={`Item ${j + 1}`} aria-label={`${c.name || `Group ${k + 1}`}, item ${j + 1}`} onChange={e => setItem(k, j, e.target.value)} />)}
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="lg-ed-sec">
        <h4>The answer</h4>
        <p className="muted small ed-help">For each {cats[0].name ? cats[0].name.toLowerCase() : 'item of the first group'}, pick what goes with it. Picking one that is already taken swaps the two.</p>
        <div className="lg-answer-wrap">
          <table className="lg-answer">
            <thead><tr><th>{cats[0].name || 'Group 1'}</th>{range(K, 1).map(k => <th key={k}>{cats[k].name || `Group ${k + 1}`}</th>)}</tr></thead>
            <tbody>
              {range(n).map(i => (
                <tr key={i}>
                  <th>{itemLabel(cats, 0, i)}</th>
                  {range(K, 1).map(k => (
                    <td key={k}>
                      <Select ariaLabel={`${cats[k].name || `Group ${k + 1}`} for ${itemLabel(cats, 0, i)}`} value={String(L.solution[k][i])} onChange={v => pick(k, i, Number(v))}
                        options={range(n).map(j => ({ value: String(j), label: itemLabel(cats, k, j) }))} />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="lg-ed-sec">
        <div className="row lg-ed-head">
          <h4 className="grow">Clues <span className="muted small">{L.clues.length}</span></h4>
          <button type="button" className="btn sm" onClick={addClue}><Icon name="plus" size={14} />Add a clue</button>
        </div>
        <p className="muted small ed-help">Build each clue from the lists; the words are written for you and you can change them. The test above says whether your clues lead to just one answer.</p>
        {L.clues.length === 0 && <p className="muted small lg-empty">No clues yet. Add one, or press “Make it for me”.</p>}
        <ol className="lg-ed-clues">
          {L.clues.map((c, i) => (
            <ClueRow key={i} i={i} count={L.clues.length} clue={c} cats={cats} sol={L.solution} hasOrder={hasOrder}
              onChange={next => setClue(i, next)} onMove={by => moveClue(i, by)} onRemove={() => set({ clues: L.clues.filter((_, x) => x !== i) })} />
          ))}
        </ol>
      </section>

      {making && <MakeLogic cats={cats} story={L.story} onClose={() => setMaking(false)} onMade={d => onChange(d)} />}
    </div>
  )
}

function ClueRow({ i, count, clue, cats, sol, hasOrder, onChange, onMove, onRemove }: {
  i: number; count: number; clue: Clue; cats: Cat[]; sol: number[][]; hasOrder: boolean
  onChange: (c: Clue) => void; onMove: (by: number) => void; onRemove: () => void
}) {
  const n = cats[0].items.length
  const refVal = (r?: Ref | null) => r ? `${r.cat}:${r.item}` : ''
  const parse = (v: string): Ref => { const [cat, item] = v.split(':').map(Number); return { cat, item } }
  const opts = (avoid = -1) => cats.flatMap((c, k) => c.items.map((_, j) => ({
    value: `${k}:${j}`, label: itemLabel(cats, k, j), group: c.name || `Group ${k + 1}`, disabled: k === avoid,
  })))
  const ordered = range(cats.length).filter(k => cats[k].ordered)
  /** A change to what the clue says: its words are written again. */
  const put = (patch: Partial<Clue>) => {
    const next: Clue = { ...clue, ...patch }
    if (ORDER_TYPES.has(next.type) && !cats[next.by ?? 0]?.ordered && ordered.length) next.by = ordered[0]
    if (next.type === 'either' && (!next.c || next.c.cat !== next.b.cat || next.c.item === next.b.item)) next.c = { cat: next.b.cat, item: (next.b.item + 1) % n }
    if (next.type !== 'either') delete next.c
    onChange({ ...next, text: clueText(cats, next) })
  }
  const warn = !clueHolds(clue, sol) ? 'Not true for your answer — change the clue or the answer.'
    : ORDER_TYPES.has(clue.type) && !cats[clue.by ?? 0]?.ordered ? 'Pick a group that is in order.' : ''
  return (
    <li className="lg-ed-clue">
      <div className="lg-ed-clue-top">
        <span className="lg-num">{i + 1}.</span>
        <div className="lg-ed-parts">
          <Select ariaLabel="First item" value={refVal(clue.a)} onChange={v => put({ a: parse(v) })} options={opts()} />
          <Select ariaLabel="How they go together" value={clue.type} onChange={v => put({ type: v as ClueType })}
            options={TYPES.map(t => ({ value: t.value, label: t.label, disabled: ORDER_TYPES.has(t.value) && !hasOrder, hint: ORDER_TYPES.has(t.value) && !hasOrder ? 'needs a group in order' : undefined }))} />
          <Select ariaLabel="Second item" value={refVal(clue.b)} onChange={v => put({ b: parse(v) })} options={opts(clue.type === 'same' || clue.type === 'not' || clue.type === 'either' ? clue.a.cat : -1)} />
          {clue.type === 'either' && <>
            <span className="muted small">or</span>
            <Select ariaLabel="Third item" value={refVal(clue.c)} onChange={v => put({ c: parse(v) })} options={opts(clue.a.cat)} />
          </>}
          {ORDER_TYPES.has(clue.type) && <Select ariaLabel="In the order of" value={String(clue.by ?? 0)} onChange={v => put({ by: Number(v) })}
            options={range(cats.length).map(k => ({ value: String(k), label: `by ${(cats[k].name || `group ${k + 1}`).toLowerCase()}`, disabled: !cats[k].ordered }))} />}
        </div>
        <div className="lg-ed-tools">
          <button type="button" className="btn ghost icon-only sm" onClick={() => onMove(-1)} disabled={i === 0} aria-label="Move up" title="Move up"><Icon name="arrow-up" size={14} /></button>
          <button type="button" className="btn ghost icon-only sm" onClick={() => onMove(1)} disabled={i === count - 1} aria-label="Move down" title="Move down"><Icon name="arrow-down" size={14} /></button>
          <button type="button" className="btn ghost icon-only sm" onClick={onRemove} aria-label="Remove this clue" title="Remove"><Icon name="trash" size={14} /></button>
        </div>
      </div>
      <input className="lg-ed-text" value={clue.text} onChange={e => onChange({ ...clue, text: e.target.value })} aria-label={`Words of clue ${i + 1}`} placeholder="What the player reads" />
      <div className={`lg-ed-warn small ${warn ? 'on' : ''}`}>{warn ? <><Icon name="alert" size={13} />{warn}</> : null}</div>
    </li>
  )
}

/** "Make it for me": from a ready-made set, or clues for the categories you typed. */
function MakeLogic({ cats, story, onClose, onMade }: { cats: Cat[]; story: string; onClose: () => void; onMade: (d: LogicData) => void }) {
  const { toast } = useToast()
  const [from, setFrom] = useState<'set' | 'mine'>('set')
  const [sets, setSets] = useState<{ key: string; name: string; categories: string[] }[]>([])
  const [setKey, setSetKey] = useState('')
  const [nCats, setNCats] = useState(3)
  const [nItems, setNItems] = useState(4)
  const [busy, setBusy] = useState(false)
  useEffect(() => { loadThemes().then(t => setSets(t.logic_sets || [])).catch(() => {}) }, [])
  const mineReady = cats.every(c => c.name.trim() && c.items.every(s => s.trim()))
  const maxCats = setKey ? Math.max(2, sets.find(s => s.key === setKey)?.categories.length || 4) : 4
  const make = async () => {
    setBusy(true)
    try {
      const body = from === 'mine'
        ? { kind: 'logic', categories: cats.map(c => ({ name: c.name.trim(), items: c.items.map(s => s.trim()), ordered: !!c.ordered })) }
        : { kind: 'logic', set: setKey, cats: Math.min(nCats, maxCats), items: nItems }
      const r = await api.post<{ made: { data: LogicData; message?: string } }>(`${V1}/generate`, body)
      const made = normLogic(r.made.data)
      onMade(from === 'mine' ? { ...made, story: story || made.story } : made)
      if (r.made.message) toast(r.made.message)
      onClose()
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="Make a logic grid for me" onClose={onClose} footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={make} disabled={busy || (from === 'mine' && !mineReady)}>{busy ? <Spinner /> : <Icon name="sparkles" size={14} />}Make it</button>
    </>}>
      <div className="stack">
        <Segmented value={from} onChange={setFrom} options={[{ value: 'set', label: 'A ready-made one' }, { value: 'mine', label: 'Clues for my groups' }]} />
        {from === 'set' ? <>
          <div className="field">
            <span className="label">Topic</span>
            <Select ariaLabel="Topic" value={setKey} onChange={setSetKey} options={[{ value: '', label: 'Surprise me' }, ...sets.map(s => ({ value: s.key, label: s.name, hint: s.categories.join(', ') }))]} />
          </div>
          <div className="row wrap">
            <div className="field"><span className="label">Groups</span><Segmented size="sm" value={Math.min(nCats, maxCats)} onChange={setNCats} options={[2, 3, 4].map(v => ({ value: v, label: String(v), disabled: v > maxCats }))} /></div>
            <div className="field"><span className="label">Items in each</span><Segmented size="sm" value={nItems} onChange={setNItems} options={[3, 4, 5].map(v => ({ value: v, label: String(v) }))} /></div>
          </div>
          <p className="muted small">More groups and items make it harder. This replaces what you have here.</p>
        </> : <p className="muted small">{mineReady
          ? 'Keeps your groups and items, picks a random answer and writes clues that lead to just that answer. Your answer and clues are replaced.'
          : 'First give every group and every item a name.'}</p>}
        {busy && <p className="muted small">Finding clues with one answer — this can take a few seconds…</p>}
      </div>
    </Modal>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as LogicPublic
  const cats = d.categories || []
  const K = cats.length, n = cats[0]?.items.length || 0
  if (K < 2 || !n) return null
  if (answer) {
    const a = normLogic(answer)
    return (
      <table className="lg-answer print">
        <thead><tr><th>{cats[0].name}</th>{range(K, 1).map(k => <th key={k}>{cats[k].name}</th>)}</tr></thead>
        <tbody>{range(n).map(i => <tr key={i}><th>{cats[0].items[i]}</th>{range(K, 1).map(k => <td key={k}>{cats[k].items[a.solution[k]?.[i] ?? -1] ?? '?'}</td>)}</tr>)}</tbody>
      </table>
    )
  }
  const m = metrics(cats, true)
  const size = Math.max(18, Math.min(large ? 34 : 26, Math.floor((660 - m.catW - m.labelW) / (n * (K - 1)))))
  return (
    <div className={`lg-print ${large ? 'large' : ''}`}>
      {d.story && <p className="lg-story">{d.story}</p>}
      <LGGrid cats={cats} size={size} marks={{}} print />
      <ol className="lg-print-clues">{d.clues.map((c, i) => <li key={i}>{c}</li>)}</ol>
    </div>
  )
}

export const Logic: KindModule = { Play, Edit, Print }
