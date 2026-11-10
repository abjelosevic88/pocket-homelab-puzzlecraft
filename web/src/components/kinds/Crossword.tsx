// Crosswords: play (type into the grid), make (black squares, letters,
// clues, word suggestions, fill the rest, criss-cross from your words), print.
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import { letters } from '../../lib/puzzlecraft'
import type { Entry } from '../../lib/types'
import { Icon } from '../icons'
import { Modal, Segmented, Spinner, useToast } from '../ui'
import { Select } from '../Select'
import { WordsInput } from './common'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shared ----

export interface CWData { w: number; h: number; cells: string[]; circles?: number[]; clues: Record<string, string>; style: 'classic' | 'freeform'; symmetry: string; language: string }

/** Words of a grid, numbered the usual way (same rules as the server). */
export function entriesOf(w: number, h: number, open: (i: number) => boolean, clues: Record<string, string> = {}, cells?: string[]): Entry[] {
  const out: Entry[] = []
  let num = 0
  for (let r = 0; r < h; r++) for (let c = 0; c < w; c++) {
    const i = r * w + c
    if (!open(i)) continue
    const across = (c === 0 || !open(i - 1)) && c + 1 < w && open(i + 1)
    const down = (r === 0 || !open(i - w)) && r + 1 < h && open(i + w)
    if (!across && !down) continue
    num++
    if (across) { const cs: number[] = []; for (let k = c; k < w && open(r * w + k); k++) cs.push(r * w + k); out.push({ key: `${num}A`, num, dir: 'A', cells: cs, clue: clues[`${num}A`] || '' }) }
    if (down) { const cs: number[] = []; for (let k = r; k < h && open(k * w + c); k++) cs.push(k * w + c); out.push({ key: `${num}D`, num, dir: 'D', cells: cs, clue: clues[`${num}D`] || '' }) }
  }
  if (cells) for (const e of out) e.answer = e.cells.map(i => cells[i] || '?').join('')
  return out.sort((a, b) => a.dir === b.dir ? a.num - b.num : a.dir === 'A' ? -1 : 1)
}

/** The width of a box, measured (so the grid fits a phone). */
export function useWidth(fallback: number) {
  const [w, setW] = useState(fallback)
  const ro = useRef<ResizeObserver | null>(null)
  const ref = useCallback((el: Element | null) => {
    ro.current?.disconnect(); ro.current = null
    if (!el) return
    const measure = () => { const cw = Math.round(el.getBoundingClientRect().width); if (cw > 0) setW(cw) }
    measure(); ro.current = new ResizeObserver(measure); ro.current.observe(el)
  }, [])
  return [ref, w] as const
}

const isLetter = (k: string) => k.length === 1 && /\p{L}|\p{N}/u.test(k)

interface GridProps {
  w: number; h: number; size: number; blocks: boolean[]; numbers: number[]; letters: string[]; circles?: number[]; freeform?: boolean
  cur?: number; word?: number[]; wrong?: Set<number>; given?: Set<number>; onCell?: (i: number) => void; className?: string
}

/** The grid itself (squares as boxes, so letters stay crisp at any size). */
export function CWGrid({ w, h, size, blocks, numbers, letters: ls, circles, freeform, cur, word, wrong, given, onCell, className = '' }: GridProps) {
  const inWord = useMemo(() => new Set(word || []), [word])
  const circ = useMemo(() => new Set(circles || []), [circles])
  return (
    <div className={`cw-grid ${freeform ? 'free' : ''} ${className}`} style={{ gridTemplateColumns: `repeat(${w}, ${size}px)`, gridAutoRows: `${size}px`, ['--cw' as any]: `${size}px` }}>
      {Array.from({ length: w * h }, (_, i) => {
        if (blocks[i]) return <div key={i} className={`cw-cell block ${freeform ? 'none' : ''}`} onClick={() => onCell?.(i)} />
        const cls = ['cw-cell', i === cur ? 'cur' : '', inWord.has(i) ? 'word' : '', wrong?.has(i) ? 'wrong' : '', given?.has(i) ? 'given' : '', circ.has(i) ? 'circ' : ''].join(' ')
        return (
          <div key={i} className={cls} onClick={() => onCell?.(i)} role={onCell ? 'button' : undefined} aria-label={onCell ? `row ${Math.floor(i / w) + 1}, column ${i % w + 1}` : undefined}>
            {numbers[i] > 0 && <span className="cw-num">{numbers[i]}</span>}
            <span className="cw-letter">{ls[i]}</span>
          </div>
        )
      })}
    </div>
  )
}

function ClueList({ entries, cur, onPick, editable, onClue, aiBusy }: { entries: Entry[]; cur?: string; onPick: (e: Entry) => void; editable?: boolean; onClue?: (key: string, v: string) => void; aiBusy?: boolean }) {
  const refs = useRef<Record<string, HTMLElement | null>>({})
  useEffect(() => { if (cur) refs.current[cur]?.scrollIntoView({ block: 'nearest' }) }, [cur])
  return (
    <div className="cw-clues">
      {(['A', 'D'] as const).map(dir => (
        <section key={dir}>
          <h4>{dir === 'A' ? 'Across' : 'Down'}</h4>
          <ol>
            {entries.filter(e => e.dir === dir).map(e => (
              <li key={e.key} ref={el => { refs.current[e.key] = el }} className={`cw-clue ${e.key === cur ? 'on' : ''}`} onClick={() => onPick(e)}>
                <b>{e.num}</b>
                {editable ? <input value={e.clue} placeholder={aiBusy ? 'Writing…' : `Clue for ${e.answer || ''}`} onFocus={() => onPick(e)} onChange={ev => onClue?.(e.key, ev.target.value)} aria-label={`Clue ${e.num} ${dir === 'A' ? 'across' : 'down'}`} />
                  : <span>{e.clue || <span className="muted">—</span>} <span className="muted small">({e.cells.length})</span></span>}
              </li>
            ))}
          </ol>
        </section>
      ))}
    </div>
  )
}

// ---- play ----

function Play({ view, progress, shown, done, showMistakes, onProgress, check, reveal, setActions }: KindPlayProps) {
  const d = view.data as { w: number; h: number; blocks: boolean[]; numbers: number[]; entries: Entry[]; circles: number[]; style: string; language: string }
  const n = d.w * d.h
  const [cells, setCells] = useState<string[]>(() => progress?.cells?.length === n ? progress.cells : Array(n).fill(''))
  const firstOpen = d.blocks.findIndex(b => !b)
  const [cur, setCur] = useState(Math.max(0, firstOpen))
  const [dir, setDir] = useState<'A' | 'D'>('A')
  const [wrong, setWrong] = useState<Set<number>>(new Set())
  const [given, setGiven] = useState<Set<number>>(() => new Set(shown))
  const lastTyped = useRef<{ cell: number; ch: string } | null>(null)
  const input = useRef<HTMLInputElement>(null)
  const [box, bw] = useWidth(600)
  const byCell = useMemo(() => {
    const m: Record<string, Entry> = {}
    for (const e of d.entries) for (const c of e.cells) m[`${c}${e.dir}`] = e
    return m
  }, [d.entries])
  const entry = byCell[`${cur}${dir}`] || byCell[`${cur}${dir === 'A' ? 'D' : 'A'}`]
  const size = Math.max(22, Math.min(44, Math.floor((Math.min(bw, 640) - 2) / d.w)))

  const cellsRef = useRef(cells)
  cellsRef.current = cells
  const update = (next: string[]) => {
    setCells(next)
    onProgress({ cells: next })
    if (showMistakes) setWrong(new Set())
    if (next.every((v, i) => d.blocks[i] || v)) check({ cells: next }, true).then(r => { if (r && !r.solved) setWrong(new Set(r.wrong || [])) })
  }
  useEffect(() => {
    setActions({
      check: async () => { const r = await check({ cells: cellsRef.current }); if (r) setWrong(new Set(r.wrong || [])) },
      reveal: async () => {
        const v = await reveal(cur)
        if (v?.letter) { const next = [...cellsRef.current]; next[cur] = v.letter; setGiven(g => new Set([...g, cur])); update(next) }
      },
      revealLabel: 'Give a letter',
      clear: () => { const next = cellsRef.current.map((v, i) => given.has(i) ? v : ''); setWrong(new Set()); setCells(next); onProgress({ cells: next }) },
    })
  }, [cur, given]) // eslint-disable-line react-hooks/exhaustive-deps

  const move = (step: number, d0 = dir) => {
    const e = byCell[`${cur}${d0}`]
    if (!e) return
    const k = e.cells.indexOf(cur) + step
    if (k >= 0 && k < e.cells.length) setCur(e.cells[k])
  }
  const nextWord = (back = false) => {
    const list = d.entries
    const at = entry ? list.findIndex(x => x.key === entry.key) : -1
    const nx = list[(at + (back ? -1 : 1) + list.length) % list.length]
    if (nx) { setDir(nx.dir); setCur(nx.cells.find(c => !cells[c]) ?? nx.cells[0]) }
  }
  const type = (ch: string) => {
    if (done) return
    const up = ch.toUpperCase()
    const next = [...cells]
    const lt = lastTyped.current
    // LJ, NJ, DŽ: the second letter joins the square just typed
    if (d.language === 'bcs' && lt && ((lt.ch === 'L' || lt.ch === 'N') && up === 'J' || lt.ch === 'D' && up === 'Ž')) {
      next[lt.cell] = lt.ch + up
      lastTyped.current = null
      update(next)
      return
    }
    if (given.has(cur)) { move(1); return }
    next[cur] = up
    lastTyped.current = { cell: cur, ch: up }
    update(next)
    const e = byCell[`${cur}${dir}`]
    if (e) {
      const k = e.cells.indexOf(cur)
      const rest = e.cells.slice(k + 1).find(c => !next[c])
      if (rest !== undefined) setCur(rest)
      else if (k + 1 < e.cells.length) setCur(e.cells[k + 1])
    }
  }
  const onKey = (ev: React.KeyboardEvent) => {
    if (ev.metaKey || ev.ctrlKey || ev.altKey) return
    const k = ev.key
    const go = (dr: number, dc: number, nd: 'A' | 'D') => {
      ev.preventDefault()
      if (dir !== nd && byCell[`${cur}${nd}`]) { setDir(nd); return }
      let r = Math.floor(cur / d.w) + dr, c = cur % d.w + dc
      while (r >= 0 && c >= 0 && r < d.h && c < d.w) { const i = r * d.w + c; if (!d.blocks[i]) { setCur(i); return } r += dr; c += dc }
    }
    if (k === 'ArrowRight') go(0, 1, 'A')
    else if (k === 'ArrowLeft') go(0, -1, 'A')
    else if (k === 'ArrowDown') go(1, 0, 'D')
    else if (k === 'ArrowUp') go(-1, 0, 'D')
    else if (k === 'Tab' || k === 'Enter') { ev.preventDefault(); nextWord(ev.shiftKey) }
    else if (k === ' ') { ev.preventDefault(); if (byCell[`${cur}${dir === 'A' ? 'D' : 'A'}`]) setDir(dir === 'A' ? 'D' : 'A') }
    else if (k === 'Backspace' || k === 'Delete') {
      ev.preventDefault()
      if (done) return
      lastTyped.current = null
      const next = [...cells]
      if (next[cur] && !given.has(cur)) { next[cur] = ''; update(next); return }
      move(-1)
      const e = byCell[`${cur}${dir}`]
      if (e) { const k2 = e.cells.indexOf(cur) - 1; if (k2 >= 0 && !given.has(e.cells[k2])) { next[e.cells[k2]] = ''; update(next) } }
    } else if (isLetter(k)) { ev.preventDefault(); type(k) }
  }
  const pick = (i: number) => {
    if (d.blocks[i]) return
    if (i === cur) { if (byCell[`${i}${dir === 'A' ? 'D' : 'A'}`]) setDir(dir === 'A' ? 'D' : 'A') }
    else { setCur(i); if (!byCell[`${i}${dir}`]) setDir(dir === 'A' ? 'D' : 'A') }
    input.current?.focus({ preventScroll: true })
  }
  return (
    <div className="cw-play" ref={box}>
      <div className="cw-current" aria-live="polite">{entry ? <><b>{entry.num} {entry.dir === 'A' ? 'across' : 'down'}</b> {entry.clue}</> : ' '}</div>
      <div className="cw-layout">
        <div className="cw-board" tabIndex={0} onKeyDown={onKey} onClick={() => input.current?.focus({ preventScroll: true })}>
          <CWGrid w={d.w} h={d.h} size={size} blocks={d.blocks} numbers={d.numbers} letters={cells} circles={d.circles} freeform={d.style === 'freeform'}
            cur={done ? undefined : cur} word={done ? undefined : entry?.cells} wrong={wrong} given={given} onCell={pick} />
          <input ref={input} className="cw-input" aria-label="Type a letter" autoCapitalize="characters" autoComplete="off" autoCorrect="off" spellCheck={false} value=""
            onKeyDown={e => { e.stopPropagation(); onKey(e) }} onChange={e => { const v = e.target.value; for (const ch of Array.from(v)) if (isLetter(ch)) type(ch) }} />
        </div>
        <ClueList entries={d.entries} cur={entry?.key} onPick={e => { setDir(e.dir); setCur(e.cells.find(c => !cells[c]) ?? e.cells[0]); input.current?.focus({ preventScroll: true }) }} />
      </div>
    </div>
  )
}

// ---- make ----

const SIZES = [5, 7, 9, 11, 13, 15, 17, 19, 21]

function Edit({ data, onChange, puzzle, settings }: KindEditProps) {
  const { toast } = useToast()
  const c = data as CWData
  const n = c.w * c.h
  const [cur, setCur] = useState(0)
  const [dir, setDir] = useState<'A' | 'D'>('A')
  const [mode, setMode] = useState<'letters' | 'blocks'>('letters')
  const [busy, setBusy] = useState('')
  const [sugg, setSugg] = useState<{ common: string[]; more: string[]; mine: { word: string; clue: string; list: string }[] } | null>(null)
  const [fromWords, setFromWords] = useState(false)
  const [box, bw] = useWidth(600)
  const free = c.style === 'freeform'
  const open = (i: number) => free ? !!c.cells[i] && c.cells[i] !== '#' : c.cells[i] !== '#'
  const entries = useMemo(() => entriesOf(c.w, c.h, open, c.clues, c.cells), [c]) // eslint-disable-line react-hooks/exhaustive-deps
  const numbers = useMemo(() => { const m = Array(n).fill(0); for (const e of entries) m[e.cells[0]] = e.num; return m }, [entries, n])
  const byCell = useMemo(() => { const m: Record<string, Entry> = {}; for (const e of entries) for (const x of e.cells) m[`${x}${e.dir}`] = e; return m }, [entries])
  const entry = byCell[`${cur}${dir}`] || byCell[`${cur}${dir === 'A' ? 'D' : 'A'}`]
  const size = Math.max(20, Math.min(40, Math.floor((Math.min(bw, 620) - 2) / c.w)))
  const set = (patch: Partial<CWData>) => onChange({ ...c, ...patch })
  const mirror = (i: number) => c.symmetry === 'rotational' ? n - 1 - i : c.symmetry === 'mirror' ? Math.floor(i / c.w) * c.w + (c.w - 1 - i % c.w) : -1
  const toggleBlock = (i: number) => {
    const cells = [...c.cells]
    const to = cells[i] === '#' ? '' : '#'
    cells[i] = to
    const j = mirror(i)
    if (j >= 0 && j !== i && !free) cells[j] = to
    set({ cells })
  }
  const putLetter = (ch: string) => {
    const cells = [...c.cells]
    cells[cur] = letters(ch, c.language)[0] || ''
    set({ cells })
    const e = byCell[`${cur}${dir}`]
    if (e) { const k = e.cells.indexOf(cur); if (k + 1 < e.cells.length) setCur(e.cells[k + 1]) }
    else if (free) setCur(dir === 'A' ? Math.min(n - 1, cur + 1) : Math.min(n - 1, cur + c.w))
  }
  const onKey = (ev: React.KeyboardEvent) => {
    if (ev.metaKey || ev.ctrlKey || ev.altKey || (ev.target as HTMLElement).tagName === 'INPUT') return
    const k = ev.key
    const go = (dr: number, dc: number, nd: 'A' | 'D') => { ev.preventDefault(); setDir(nd); const r = Math.floor(cur / c.w) + dr, col = cur % c.w + dc; if (r >= 0 && col >= 0 && r < c.h && col < c.w) setCur(r * c.w + col) }
    if (k === 'ArrowRight') go(0, 1, 'A')
    else if (k === 'ArrowLeft') go(0, -1, 'A')
    else if (k === 'ArrowDown') go(1, 0, 'D')
    else if (k === 'ArrowUp') go(-1, 0, 'D')
    else if (k === '.' || k === '#') { ev.preventDefault(); toggleBlock(cur) }
    else if (k === ' ') { ev.preventDefault(); setDir(dir === 'A' ? 'D' : 'A') }
    else if (k === 'Backspace' || k === 'Delete') {
      ev.preventDefault()
      const cells = [...c.cells]
      if (cells[cur] && cells[cur] !== '#') { cells[cur] = ''; set({ cells }) }
      else { const e = byCell[`${cur}${dir}`]; if (e) { const kk = e.cells.indexOf(cur); if (kk > 0) { setCur(e.cells[kk - 1]); cells[e.cells[kk - 1]] = ''; set({ cells }) } } }
    } else if (isLetter(k) && mode === 'letters') { ev.preventDefault(); putLetter(k) }
  }
  const pick = (i: number) => {
    if (mode === 'blocks' && !free) { toggleBlock(i); setCur(i); return }
    if (i === cur) setDir(dir === 'A' ? 'D' : 'A')
    else setCur(i)
  }
  // word suggestions for the word being edited
  const pattern = entry ? entry.cells.map(i => c.cells[i] && c.cells[i] !== '#' ? c.cells[i] : '?').join('') : ''
  useEffect(() => { setSugg(null) }, [pattern])
  const suggest = async () => {
    if (!pattern) return
    setBusy('words')
    try { setSugg(await api.get(`${V1}/words?pattern=${encodeURIComponent(pattern)}&limit=40`)) } catch (e) { toast((e as Error).message, 'err') } finally { setBusy('') }
  }
  const useWord = (w: string, clue?: string) => {
    if (!entry) return
    const ls = letters(w, c.language)
    if (ls.length !== entry.cells.length) return
    const cells = [...c.cells]
    entry.cells.forEach((ci, k) => { cells[ci] = ls[k] })
    set(clue ? { cells, clues: { ...c.clues, [entry.key]: clue } } : { cells })
  }
  const fill = async () => {
    setBusy('fill')
    try {
      const r = await api.post<{ ok: boolean; data: CWData; message: string }>(`${V1}/crossword/fill`, { data: c })
      if (r.ok) set({ cells: r.data.cells })
      toast(r.message, r.ok ? 'ok' : 'err')
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy('') }
  }
  const newGrid = async (w: number) => {
    setBusy('grid')
    try { const r = await api.post<{ data: CWData }>(`${V1}/crossword/grid`, { w, h: w }); onChange({ ...r.data, clues: {}, language: c.language }); setCur(0) } catch (e) { toast((e as Error).message, 'err') } finally { setBusy('') }
  }
  const resize = (w: number) => {
    const cells = Array(w * w).fill('')
    for (let r = 0; r < Math.min(w, c.h); r++) for (let col = 0; col < Math.min(w, c.w); col++) cells[r * w + col] = c.cells[r * c.w + col]
    onChange({ ...c, w, h: w, cells, circles: [] })
    setCur(0)
  }
  const aiClues = async () => {
    const missing = entries.filter(e => !e.clue && e.answer && !e.answer.includes('?'))
    if (!missing.length) { toast('Every word has a clue'); return }
    setBusy('ai')
    try {
      const r = await api.post<{ clues: { word: string; clue: string }[] }>(`${V1}/ai/clues`, { words: missing.map(e => e.answer), language: c.language, level: puzzle.level })
      const clues = { ...c.clues }
      missing.forEach((e, i) => { if (r.clues[i]?.clue) clues[e.key] = r.clues[i].clue })
      set({ clues })
      toast(`${missing.length} clues written — read them through`)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy('') }
  }
  const toggleCircle = () => {
    const s = new Set(c.circles || [])
    if (s.has(cur)) s.delete(cur); else s.add(cur)
    set({ circles: [...s] })
  }
  const aiOn = settings?.ai_help !== false
  return (
    <div className="cw-edit" ref={box}>
      <div className="ed-toolbar">
        {!free && <Segmented size="sm" value={mode} onChange={setMode} options={[{ value: 'letters', label: 'Letters' }, { value: 'blocks', label: 'Black squares' }]} />}
        {!free && <Select ariaLabel="Size" value={String(c.w)} onChange={v => resize(Number(v))} options={SIZES.map(s => ({ value: String(s), label: `${s} × ${s}` }))} />}
        {free && <span className="muted small">{c.w} × {c.h}</span>}
        {!free && <Select ariaLabel="Symmetry" value={c.symmetry} onChange={v => set({ symmetry: v })} options={[{ value: 'rotational', label: 'Symmetric (turned)' }, { value: 'mirror', label: 'Symmetric (mirrored)' }, { value: 'none', label: 'Free' }]} />}
        <span className="spacer" />
        {!free && <button type="button" className="btn sm" onClick={() => newGrid(c.w)} disabled={!!busy} title="A new pattern of black squares (letters go)">{busy === 'grid' ? <Spinner /> : <Icon name="grid" size={14} />}New pattern</button>}
        {!free && <button type="button" className="btn sm" onClick={fill} disabled={!!busy} title="Fill every empty square with words">{busy === 'fill' ? <Spinner /> : <Icon name="sparkles" size={14} />}Fill the rest</button>}
        <button type="button" className="btn sm" onClick={() => setFromWords(true)} disabled={!!busy}><Icon name="shuffle" size={14} />From my words</button>
      </div>
      <p className="muted small ed-help">{free ? 'Words cross each other with no black squares. Click a square and type; space turns the direction.'
        : 'Click a square and type. A full stop (.) makes a black square — its partner on the other side follows. Space turns the direction; arrows move.'}</p>
      <div className="cw-layout">
        <div className="cw-board" tabIndex={0} onKeyDown={onKey}>
          <CWGrid w={c.w} h={c.h} size={size} blocks={c.cells.map((v, i) => free ? false : v === '#')} numbers={numbers} letters={c.cells.map(v => v === '#' ? '' : v)}
            circles={c.circles} cur={cur} word={entry?.cells} onCell={pick} className={free ? 'edit-free' : ''} />
          <div className="cw-word-tools">
            {entry && <span className="cw-pattern">{entry.num}{entry.dir === 'A' ? ' across' : ' down'} · <code>{pattern}</code></span>}
            <button type="button" className="btn xs" onClick={suggest} disabled={!entry || !!busy}>{busy === 'words' ? <Spinner /> : <Icon name="search" size={13} />}Words that fit</button>
            <button type="button" className="btn xs ghost" onClick={toggleCircle} title="Circle this square (for a hidden word)"><Icon name="circle" size={13} />Circle</button>
          </div>
          {sugg && <div className="cw-sugg">
            {sugg.mine.length > 0 && <div className="chips">{sugg.mine.map(m => <button key={m.word + m.list} type="button" className="chip on" onClick={() => useWord(m.word, m.clue)} title={`From ${m.list}`}>{m.word}</button>)}</div>}
            <div className="chips">{sugg.common.map(w => <button key={w} type="button" className="chip" onClick={() => useWord(w)}>{w}</button>)}</div>
            {sugg.more.length > 0 && <details><summary className="muted small">Less common ({sugg.more.length})</summary><div className="chips">{sugg.more.map(w => <button key={w} type="button" className="chip" onClick={() => useWord(w)}>{w}</button>)}</div></details>}
            {!sugg.common.length && !sugg.more.length && !sugg.mine.length && <p className="muted small">No English word fits {pattern}.</p>}
          </div>}
        </div>
        <div className="cw-side">
          <div className="row">
            <h4 className="grow">Clues</h4>
            {aiOn && <button type="button" className="btn xs" onClick={aiClues} disabled={!!busy}>{busy === 'ai' ? <Spinner /> : <Icon name="sparkles" size={13} />}Write missing clues</button>}
          </div>
          <ClueList entries={entries} cur={entry?.key} editable aiBusy={busy === 'ai'} onPick={e => { setDir(e.dir); setCur(e.cells[0]) }} onClue={(k, v) => set({ clues: { ...c.clues, [k]: v } })} />
        </div>
      </div>
      {fromWords && <FromWords lang={c.language} onClose={() => setFromWords(false)} onMade={d => { onChange(d); setCur(0) }} />}
    </div>
  )
}

/** Make a criss-cross from words and clues (yours, a word list, or a built-in one). */
function FromWords({ lang, onClose, onMade }: { lang: string; onClose: () => void; onMade: (d: CWData) => void }) {
  const { toast } = useToast()
  const [words, setWords] = useState<{ word: string; clue: string }[]>([])
  const [busy, setBusy] = useState(false)
  const make = async () => {
    setBusy(true)
    try {
      const r = await api.post<{ made: { data: CWData; missing?: string[] } }>(`${V1}/generate`, { kind: 'crossword', words, language: lang })
      onMade(r.made.data)
      if (r.made.missing?.length) toast(`Did not fit: ${r.made.missing.join(', ')}`, 'err')
      onClose()
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="A crossword from your words" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={make} disabled={busy || words.length < 2}>{busy ? <Spinner /> : null}Make it</button></>}>
      <p className="muted small">The words are laid out to cross each other as tightly as they can. This replaces the grid you have.</p>
      <WordsInput value={words} onChange={setWords} language={lang} />
    </Modal>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as { w: number; h: number; blocks: boolean[]; numbers: number[]; entries: Entry[]; circles: number[]; style: string }
  const size = Math.min(large ? 40 : 32, Math.floor(640 / d.w))
  if (answer) {
    const a = answer as CWData
    return <CWGrid w={a.w} h={a.h} size={Math.min(18, Math.floor(300 / a.w))} blocks={d.blocks} numbers={Array(a.w * a.h).fill(0)} letters={a.cells.map(v => v === '#' ? '' : v)} circles={a.circles} freeform={a.style === 'freeform'} className="print-grid small" />
  }
  return (
    <div className="cw-print">
      <CWGrid w={d.w} h={d.h} size={size} blocks={d.blocks} numbers={d.numbers} letters={Array(d.w * d.h).fill('')} circles={d.circles} freeform={d.style === 'freeform'} className="print-grid" />
      <ClueList entries={d.entries} onPick={() => {}} />
    </div>
  )
}

export const Crossword: KindModule = { Play, Edit, Print }
