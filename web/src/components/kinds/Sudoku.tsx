// Sudoku: play (tap a square, then a number; pencil notes; the row, column,
// box and same numbers lit up), make (made for you, typed in, or read from a
// photo), print.
import '../../styles/kind-sudoku.css'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import { LEVELS } from '../../lib/puzzlecraft'
import { Icon } from '../icons'
import { PhotoPick } from '../PhotoPick'
import { Segmented, Spinner, Switch, useToast } from '../ui'
import { useWidth } from './Crossword'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shared ----

interface SDPublic { size: number; box_w: number; box_h: number; givens: number[]; level: string; diagonal?: boolean }
interface SDFull extends SDPublic { solution: number[] }
type Notes = Record<string, number[]>

const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v))
const boxOf = (size: number): [number, number] => size === 4 ? [2, 2] : size === 6 ? [3, 2] : [3, 3]

/** Squares that share a row, column, box (or long diagonal) with i. */
function peersOf(i: number, size: number, bw: number, bh: number, diag?: boolean): Set<number> {
  const r = Math.floor(i / size), c = i % size
  const out = new Set<number>()
  for (let k = 0; k < size; k++) { out.add(r * size + k); out.add(k * size + c) }
  const br = Math.floor(r / bh) * bh, bc = Math.floor(c / bw) * bw
  for (let y = br; y < br + bh; y++) for (let x = bc; x < bc + bw; x++) out.add(y * size + x)
  if (diag) {
    if (r === c) for (let k = 0; k < size; k++) out.add(k * size + k)
    if (r + c === size - 1) for (let k = 0; k < size; k++) out.add(k * size + size - 1 - k)
  }
  out.delete(i)
  return out
}

/** Squares whose number is already in their row, column or box. */
function clashes(cells: number[], size: number, bw: number, bh: number, diag?: boolean): Set<number> {
  const out = new Set<number>()
  cells.forEach((v, i) => {
    if (!v) return
    for (const p of peersOf(i, size, bw, bh, diag)) if (cells[p] === v) { out.add(i); break }
  })
  return out
}

interface GridProps {
  size: number; bw: number; bh: number; px: number; cells: number[]; locked: (i: number) => boolean
  notes?: Notes; sel?: number | null; wrong?: Set<number>; clash?: Set<number>; diagonal?: boolean; className?: string
  onPick?: (i: number) => void
}

function SDGrid({ size, bw, bh, px, cells, locked, notes, sel, wrong, clash, diagonal, className = '', onPick }: GridProps) {
  const selV = sel != null ? cells[sel] : 0
  const peers = useMemo(() => sel != null ? peersOf(sel, size, bw, bh, false) : new Set<number>(), [sel, size, bw, bh])
  return (
    <div className={`sd-grid ${className}`} role={onPick ? 'grid' : undefined} style={{ gridTemplateColumns: `repeat(${size}, ${px}px)`, gridAutoRows: `${px}px`, ['--sd' as any]: `${px}px` }}>
      {cells.map((v, i) => {
        const r = Math.floor(i / size), c = i % size
        const cls = ['sd-cell',
          (c + 1) % bw === 0 && c < size - 1 ? 'bx-r' : '', (r + 1) % bh === 0 && r < size - 1 ? 'bx-b' : '', c === size - 1 ? 'lc' : '', r === size - 1 ? 'lr' : '',
          locked(i) ? 'given' : 'mine', i === sel ? 'sel' : '', peers.has(i) ? 'peer' : '', selV && v === selV && i !== sel ? 'same' : '',
          wrong?.has(i) ? 'wrong' : '', clash?.has(i) ? 'clash' : '', diagonal && (r === c || r + c === size - 1) ? 'diag' : ''].join(' ')
        const ns = !v && notes?.[i]
        return (
          <div key={i} className={cls} onPointerDown={onPick ? e => { e.preventDefault(); onPick(i) } : undefined}
            role={onPick ? 'gridcell' : undefined} aria-label={onPick ? `Row ${r + 1}, column ${c + 1}${v ? `: ${v}` : ', empty'}` : undefined} aria-selected={onPick ? i === sel : undefined}>
            {v ? <span className="sd-n">{v}</span> : ns && ns.length ? (
              <span className="sd-notes" style={{ gridTemplateColumns: `repeat(${bw}, 1fr)` }}>
                {Array.from({ length: size }, (_, k) => <span key={k} className={selV === k + 1 ? 'hl' : ''}>{ns.includes(k + 1) ? k + 1 : ''}</span>)}
              </span>
            ) : null}
          </div>
        )
      })}
    </div>
  )
}

/** The number buttons under the grid. */
function Pad({ size, counts, pencil, onNum, onErase, onPencil, disabled, width }: { size: number; counts?: number[]; pencil?: boolean; onNum: (n: number) => void; onErase: () => void; onPencil?: (v: boolean) => void; disabled?: boolean; width: number }) {
  const per = size === 9 ? 9 : size
  const bwid = clamp(Math.floor((width - (per - 1) * 6) / per), 30, 56)
  return (
    <div className="sd-pad" style={{ width: Math.max(width, 260) }}>
      <div className="sd-nums" style={{ gridTemplateColumns: `repeat(${per}, ${bwid}px)` }}>
        {Array.from({ length: size }, (_, k) => {
          const left = counts ? size - (counts[k + 1] || 0) : 1
          return (
            <button key={k} type="button" className={`sd-key ${pencil ? 'pencil' : ''} ${left <= 0 ? 'full' : ''}`} style={{ height: Math.round(bwid * 1.15) }} disabled={disabled}
              onPointerDown={e => e.preventDefault()} onClick={() => onNum(k + 1)} aria-label={pencil ? `Note ${k + 1}` : `Put ${k + 1}`}>
              <span className="sd-key-n">{k + 1}</span>
              {counts && <span className="sd-key-left">{left > 0 ? left : <Icon name="check" size={10} />}</span>}
            </button>
          )
        })}
      </div>
      <div className="sd-tools">
        {onPencil && <button type="button" className={`btn sd-tool ${pencil ? 'on' : ''}`} aria-pressed={!!pencil} disabled={disabled} onPointerDown={e => e.preventDefault()} onClick={() => onPencil(!pencil)} title="Small notes of the numbers that could go here (N)">
          <Icon name="pen" size={15} />Notes {pencil ? 'on' : 'off'}
        </button>}
        <button type="button" className="btn sd-tool" disabled={disabled} onPointerDown={e => e.preventDefault()} onClick={onErase} title="Rub out (Backspace)"><Icon name="x" size={15} />Rub out</button>
      </div>
    </div>
  )
}

const digitOf = (ev: React.KeyboardEvent): number => {
  const m = /^(?:Digit|Numpad)(\d)$/.exec(ev.code)
  if (m) return Number(m[1])
  return /^\d$/.test(ev.key) ? Number(ev.key) : -1
}

// ---- play ----

function Play({ view, progress, shown, done, showMistakes, onProgress, check, reveal, canReveal, setActions }: KindPlayProps) {
  const d = view.data as SDPublic
  const { size } = d
  const bw = d.box_w || boxOf(size)[0], bh = d.box_h || boxOf(size)[1]
  const n = size * size
  const [cells, setCells] = useState<number[]>(() => {
    const p: number[] | undefined = progress?.cells
    return d.givens.map((g, i) => g || (Array.isArray(p) && p.length === n ? p[i] || 0 : 0))
  })
  const [notes, setNotes] = useState<Notes>(() => progress?.notes && typeof progress.notes === 'object' ? progress.notes : {})
  const [sel, setSel] = useState<number | null>(() => { const k = d.givens.findIndex(g => !g); return k >= 0 ? k : 0 })
  const [pencil, setPencil] = useState(false)
  const [wrong, setWrong] = useState<Set<number>>(new Set())
  const [given, setGiven] = useState<Set<number>>(() => new Set(shown))
  const [box, bwid] = useWidth(480)
  const px = clamp(Math.floor((Math.min(bwid, 520) - 4) / size), 30, size === 9 ? 56 : 76)
  const gridW = px * size + 4
  const ref = useRef({ cells, notes })
  ref.current = { cells, notes }
  const board = useRef<HTMLDivElement>(null)

  const locked = (i: number) => d.givens[i] !== 0 || (given.has(i) && cells[i] !== 0)
  const clash = useMemo(() => clashes(cells, size, bw, bh, d.diagonal), [cells, size, bw, bh, d.diagonal])
  const counts = useMemo(() => { const m = Array(size + 1).fill(0); for (const v of cells) if (v) m[v]++; return m }, [cells, size])

  const commit = (next: number[], nextNotes: Notes, changed?: number) => {
    ref.current = { cells: next, notes: nextNotes }
    setCells(next); setNotes(nextNotes)
    onProgress({ cells: next, notes: nextNotes })
    if (changed !== undefined) setWrong(w => { if (!w.has(changed)) return w; const s = new Set(w); s.delete(changed); return s })
    if (next.every(v => v)) {
      check({ cells: next }, true).then(r => { if (r && !r.solved) setWrong(new Set(r.wrong || [])) })
    } else if (showMistakes && changed !== undefined && next[changed]) {
      // only the square just filled, so each mistake counts once
      const one = Array(n).fill(0)
      one[changed] = next[changed]
      check({ cells: one }).then(r => { if (r?.wrong?.includes(changed)) setWrong(w => new Set([...w, changed])) })
    }
  }

  const put = (num: number, asNote = pencil) => {
    if (done || sel == null || locked(sel)) return
    const cur = ref.current
    if (asNote) {
      if (cur.cells[sel]) return
      const have = cur.notes[sel] || []
      const nn: Notes = { ...cur.notes, [sel]: have.includes(num) ? have.filter(x => x !== num) : [...have, num].sort((a, b) => a - b) }
      if (!nn[sel].length) delete nn[sel]
      commit(cur.cells, nn)
      return
    }
    const next = [...cur.cells]
    next[sel] = cur.cells[sel] === num ? 0 : num
    const nn = { ...cur.notes }
    delete nn[sel]
    // the number is no longer a possibility for the squares around it
    if (next[sel]) for (const p of peersOf(sel, size, bw, bh, d.diagonal)) if (nn[p]?.includes(num)) { nn[p] = nn[p].filter(x => x !== num); if (!nn[p].length) delete nn[p] }
    commit(next, nn, sel)
  }
  const erase = () => {
    if (done || sel == null || locked(sel)) return
    const cur = ref.current
    if (!cur.cells[sel] && !cur.notes[sel]) return
    const next = [...cur.cells]
    next[sel] = 0
    const nn = { ...cur.notes }
    if (!cur.cells[sel]) delete nn[sel]
    commit(next, nn, sel)
  }
  const onKey = (ev: React.KeyboardEvent) => {
    if (done || ev.metaKey || ev.ctrlKey || ev.altKey) return
    const k = ev.key
    const cur = sel ?? 0
    const mv = (dr: number, dc: number) => { ev.preventDefault(); const r = clamp(Math.floor(cur / size) + dr, 0, size - 1), c = clamp(cur % size + dc, 0, size - 1); setSel(r * size + c) }
    if (k === 'ArrowUp') mv(-1, 0)
    else if (k === 'ArrowDown') mv(1, 0)
    else if (k === 'ArrowLeft') mv(0, -1)
    else if (k === 'ArrowRight') mv(0, 1)
    else if (k === 'Backspace' || k === 'Delete' || k === '0') { ev.preventDefault(); erase() }
    else if (k === 'n' || k === 'N' || k === 'p' || k === 'P') { ev.preventDefault(); setPencil(p => !p) }
    else {
      const dg = digitOf(ev)
      if (dg >= 1 && dg <= size) {
        ev.preventDefault()
        put(dg, pencil || ev.shiftKey)
      }
    }
  }

  useEffect(() => {
    const target = sel != null && !locked(sel) && !cells[sel] ? sel : cells.findIndex((v, i) => !v && !locked(i))
    setActions({
      check: async () => { const r = await check({ cells: ref.current.cells }); if (r) setWrong(new Set(r.wrong || [])) },
      reveal: canReveal && target >= 0 ? async () => {
        const v = await reveal(target)
        if (v && typeof v.number === 'number' && v.number > 0) {
          const cur = ref.current
          const next = [...cur.cells]
          next[v.cell ?? target] = v.number
          const nn = { ...cur.notes }
          delete nn[v.cell ?? target]
          setGiven(g => new Set([...g, v.cell ?? target]))
          setSel(v.cell ?? target)
          commit(next, nn, v.cell ?? target)
        }
      } : undefined,
      revealLabel: 'Give a number',
      clear: () => {
        const next = d.givens.map((g, i) => g || (given.has(i) ? ref.current.cells[i] : 0))
        setWrong(new Set())
        ref.current = { cells: next, notes: {} }
        setCells(next); setNotes({})
        onProgress({ cells: next, notes: {} })
      },
    })
  }, [sel, given, cells, canReveal]) // eslint-disable-line react-hooks/exhaustive-deps

  const pick = (i: number) => { if (done) return; setSel(i); board.current?.focus({ preventScroll: true }) }
  return (
    <div className="sd-play" ref={box}>
      <div ref={board} className="sd-board" tabIndex={done ? -1 : 0} onKeyDown={onKey} aria-label="Sudoku grid. Arrows move, numbers fill in, N switches notes.">
        <SDGrid size={size} bw={bw} bh={bh} px={px} cells={cells} locked={locked} notes={notes} sel={done ? null : sel} wrong={wrong} clash={clash} diagonal={d.diagonal}
          className={done ? 'is-done' : ''} onPick={done ? undefined : pick} />
      </div>
      <Pad size={size} counts={counts} pencil={pencil} onNum={k => put(k)} onErase={erase} onPencil={setPencil} disabled={done} width={gridW} />
      <p className="sd-tip muted small">{d.diagonal ? 'Every row, column, box and both long diagonals hold each number once.' : `Every row, column and box holds each number from 1 to ${size} once.`}</p>
    </div>
  )
}

// ---- make ----

function Edit({ data, onChange, puzzle, settings }: KindEditProps) {
  const { toast } = useToast()
  const d = data as SDFull
  const size = d.size || 9
  const [bw, bh] = d.box_w && d.box_h ? [d.box_w, d.box_h] : boxOf(size)
  const givens = d.givens?.length === size * size ? d.givens : Array(size * size).fill(0)
  const [how, setHow] = useState<'make' | 'type' | 'photo'>('make')
  const [mkSize, setMkSize] = useState<number>(size)
  const [lvl, setLvl] = useState<string>(puzzle.level || d.level || 'medium')
  const [diag, setDiag] = useState(!!d.diagonal)
  const [busy, setBusy] = useState('')
  const [sel, setSel] = useState<number | null>(null)
  const [photo, setPhoto] = useState('')
  const [box, bwid] = useWidth(480)
  const px = clamp(Math.floor((Math.min(bwid, 480) - 4) / size), 28, size === 9 ? 50 : 66)
  const clash = useMemo(() => clashes(givens, size, bw, bh, d.diagonal), [givens, size, bw, bh, d.diagonal])
  const filled = givens.filter(Boolean).length

  const setGivens = (next: number[]) => onChange({ ...d, size, box_w: bw, box_h: bh, givens: next, solution: [] })
  const make = async () => {
    setBusy('make')
    try {
      const r = await api.post<{ made: { data: SDFull } }>(`${V1}/generate`, { kind: 'sudoku', size: mkSize, level: lvl, diagonal: diag })
      onChange(r.made.data)
      setSel(null)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy('') }
  }
  const newSize = (s: number) => {
    const [w2, h2] = boxOf(s)
    onChange({ ...d, size: s, box_w: w2, box_h: h2, givens: Array(s * s).fill(0), solution: [], diagonal: diag })
    setSel(null)
  }
  const read = async (id: number, url: string) => {
    setPhoto(url)
    setBusy('photo')
    try {
      const r = await api.post<{ data: SDFull; report?: { problems?: string[] } }>(`${V1}/ai/read-puzzle`, { photo_id: id })
      onChange(r.data)
      const got = (r.data.givens || []).filter(Boolean).length
      toast(`Read ${got} numbers — compare them with the photo and fix any that are wrong`)
      setHow('type')
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy('') }
  }
  const put = (num: number, advance = false) => {
    if (sel == null) return
    const next = [...givens]
    next[sel] = next[sel] === num && !advance ? 0 : num
    setGivens(next)
    if (advance && sel + 1 < size * size) setSel(sel + 1)
  }
  const erase = () => { if (sel == null) return; const next = [...givens]; next[sel] = 0; setGivens(next) }
  const onKey = (ev: React.KeyboardEvent) => {
    if (ev.metaKey || ev.ctrlKey || ev.altKey) return
    const k = ev.key
    const cur = sel ?? 0
    const mv = (dr: number, dc: number) => { ev.preventDefault(); setSel(clamp(Math.floor(cur / size) + dr, 0, size - 1) * size + clamp(cur % size + dc, 0, size - 1)) }
    if (k === 'ArrowUp') mv(-1, 0)
    else if (k === 'ArrowDown') mv(1, 0)
    else if (k === 'ArrowLeft') mv(0, -1)
    else if (k === 'ArrowRight') mv(0, 1)
    else if (k === 'Backspace' || k === 'Delete' || k === '0' || k === ' ') { ev.preventDefault(); erase(); if (k === ' ' && sel != null && sel + 1 < size * size) setSel(sel + 1) }
    else { const dg = digitOf(ev); if (dg >= 1 && dg <= size) { ev.preventDefault(); put(dg, true) } }
  }
  const aiOn = settings?.ai_help !== false
  return (
    <div className="sd-edit" ref={box}>
      <Segmented value={how} onChange={setHow} label="How to make it" options={[
        { value: 'make', label: 'Make one for me' }, { value: 'type', label: 'Type your own' },
        { value: 'photo', label: 'Read a photo', disabled: !aiOn, title: aiOn ? 'The AI reads a printed sudoku' : 'AI help is switched off in Settings' }]} />
      {how === 'make' && <div className="sd-edit-sec">
        <div className="sd-field"><span className="muted small">Size</span>
          <Segmented size="sm" label="Size" value={mkSize} onChange={setMkSize} options={[{ value: 4, label: '4 × 4 (children)' }, { value: 6, label: '6 × 6' }, { value: 9, label: '9 × 9' }]} />
        </div>
        <div className="sd-field"><span className="muted small">How hard</span>
          <Segmented size="sm" label="How hard" value={lvl} onChange={setLvl} options={LEVELS.map(l => ({ value: l.value as string, label: l.label }))} />
        </div>
        <label className="row sd-switch"><Switch checked={diag} onChange={setDiag} label="Diagonals too" /><span>Diagonals too <span className="muted small">— both long diagonals also hold every number</span></span></label>
        <div><button type="button" className="btn primary" onClick={make} disabled={!!busy}>{busy === 'make' ? <Spinner /> : <Icon name="sparkles" size={14} />}Make a sudoku</button></div>
      </div>}
      {how === 'type' && <div className="sd-edit-sec">
        <div className="sd-field"><span className="muted small">Size</span>
          <Segmented size="sm" label="Size" value={size} onChange={newSize} options={[{ value: 4, label: '4 × 4' }, { value: 6, label: '6 × 6' }, { value: 9, label: '9 × 9' }]} />
        </div>
        <label className="row sd-switch"><Switch checked={!!d.diagonal} onChange={v => onChange({ ...d, diagonal: v, solution: [] })} label="Diagonals too" /><span>Diagonals too</span></label>
        <p className="muted small ed-help">Tap a square and type the numbers printed in it (or use the buttons). The test below the editor says whether it has exactly one answer.</p>
      </div>}
      {how === 'photo' && <div className="sd-edit-sec">
        <p className="muted small ed-help">A straight, sharp photo of a printed 9 × 9 sudoku. The AI reads the printed numbers (not pencil marks); you can fix any it got wrong.</p>
        <PhotoPick kind="puzzle" label="Choose a photo" onPicked={p => read(p.id, p.thumb_url || p.url)} />
        {busy === 'photo' && <p className="row muted small"><Spinner /> Reading the numbers…</p>}
      </div>}

      <div className="sd-edit-main">
        <div className="sd-edit-grid">
          <div className="sd-board" tabIndex={0} onKeyDown={onKey} aria-label="The numbers given at the start">
            <SDGrid size={size} bw={bw} bh={bh} px={px} cells={givens} locked={i => !!givens[i]} sel={sel} clash={clash} diagonal={d.diagonal} className="edit" onPick={i => setSel(i)} />
          </div>
          <Pad size={size} onNum={k => put(k)} onErase={erase} disabled={sel == null} width={px * size + 4} />
          <p className="muted small sd-count">{filled} {filled === 1 ? 'number' : 'numbers'} given{clash.size ? ' · the red ones are twice in a row, column or box' : ''}</p>
        </div>
        {photo && how !== 'make' && <figure className="sd-photo"><img src={photo} alt="The photo that was read" /><figcaption className="muted small">The photo</figcaption></figure>}
      </div>
    </div>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as SDPublic
  const [box, bwid] = useWidth(640)
  const [bw, bh] = d.box_w && d.box_h ? [d.box_w, d.box_h] : boxOf(d.size)
  if (answer) {
    const a = answer as SDFull
    const sol = a.solution?.length === a.size * a.size ? a.solution : a.givens
    const px = clamp(Math.floor(Math.min(bwid, 260) / a.size), 14, 26)
    return <div className="sd-print answer" ref={box}><SDGrid size={a.size} bw={bw} bh={bh} px={px} cells={sol} locked={i => !!a.givens[i]} diagonal={a.diagonal} className="print" /></div>
  }
  const px = clamp(Math.floor(Math.min(bwid, large ? 640 : 520) / d.size), 30, large ? 72 : 58)
  return (
    <div className={`sd-print ${large ? 'large' : ''}`} ref={box}>
      <SDGrid size={d.size} bw={bw} bh={bh} px={px} cells={d.givens} locked={i => !!d.givens[i]} diagonal={d.diagonal} className="print" />
      <p className="sd-print-rule">{d.diagonal ? `Fill in the numbers 1 to ${d.size}: once in every row, column, box and both long diagonals.` : `Fill in the numbers 1 to ${d.size}: once in every row, column and box.`}</p>
    </div>
  )
}

export const Sudoku: KindModule = { Play, Edit, Print }
