// Picture puzzle (nonogram): play (fill squares or mark them ✕, drag to paint
// a line, the numbers grey out when their line is right), make (paint it,
// take one from the library or turn a photo into one), print.
import '../../styles/kind-nonogram.css'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Icon } from '../icons'
import { PhotoPick } from '../PhotoPick'
import { Modal, Segmented, Spinner, Switch, useToast } from '../ui'
import { Select } from '../Select'
import { loadThemes } from './common'
import { useWidth } from './Crossword'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shared ----

interface NGPublic { w: number; h: number; rows: number[][]; cols: number[][]; color?: string }
interface NGFull extends NGPublic { cells: number[] }

const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v))

/** The lengths of the runs of filled squares in a line. */
function runs(line: number[]): number[] {
  const out: number[] = []
  let n = 0
  for (const v of line) { if (v === 1) { n++; continue } if (n) out.push(n); n = 0 }
  if (n) out.push(n)
  return out
}
const same = (a: number[], b: number[]) => a.length === b.length && a.every((v, i) => v === b[i])
const rowOf = (cells: number[], w: number, r: number) => cells.slice(r * w, (r + 1) * w)
const colOf = (cells: number[], w: number, h: number, c: number) => Array.from({ length: h }, (_, r) => cells[r * w + c])
function cluesOf(cells: number[], w: number, h: number) {
  return { rows: Array.from({ length: h }, (_, r) => runs(rowOf(cells, w, r))), cols: Array.from({ length: w }, (_, c) => runs(colOf(cells, w, h, c))) }
}

/** Square size and the room the numbers need, for the measured width. */
function layout(avail: number, w: number, h: number, rows: number[][], cols: number[][], max = 34) {
  const mr = Math.max(1, ...rows.map(x => x.length)), mc = Math.max(1, ...cols.map(x => x.length))
  let s = Math.floor((avail - 10) / (w + mr * 0.62))
  s = clamp(s, 13, max)
  const num = Math.max(s * 0.62, 13)
  return { s, left: Math.ceil(mr * num + 8), top: Math.ceil(mc * num + 6), font: clamp(Math.round(s * 0.5), 10, 17) }
}

interface Stroke { id: number; r0: number; c0: number; axis: '' | 'r' | 'c'; from: number; to: number; base: number[]; timer: number; touch: boolean }

/** Painting with a pointer: tap one square, or drag along a row or a column. */
function usePaint(opts: { w: number; h: number; s: number; get: () => number[]; locked?: (i: number) => boolean; target: (old: number, alt: boolean) => number; longPress?: (old: number) => number; onPaint: (cells: number[]) => void; onEnd: (cells: number[]) => void; disabled?: boolean }) {
  const st = useRef<Stroke | null>(null)
  const el = useRef<HTMLDivElement | null>(null)
  const o = useRef(opts)
  o.current = opts
  const at = (e: React.PointerEvent): [number, number] => {
    const rect = el.current!.getBoundingClientRect()
    const { s, w, h } = o.current
    return [clamp(Math.floor((e.clientY - rect.top) / s), 0, h - 1), clamp(Math.floor((e.clientX - rect.left) / s), 0, w - 1)]
  }
  const paintTo = (r: number, c: number) => {
    const g = st.current!
    const { w, locked } = o.current
    const next = [...g.base]
    const cellsOnLine: number[] = []
    if (g.axis === 'r') for (let k = Math.min(c, g.c0); k <= Math.max(c, g.c0); k++) cellsOnLine.push(g.r0 * w + k)
    else if (g.axis === 'c') for (let k = Math.min(r, g.r0); k <= Math.max(r, g.r0); k++) cellsOnLine.push(k * w + g.c0)
    else cellsOnLine.push(g.r0 * w + g.c0)
    for (const i of cellsOnLine) {
      if (locked?.(i)) continue
      const v = g.base[i]
      // erasing takes away only what the first square had; painting goes only on empty squares
      if (g.to === 0 ? v === g.from : v === 0 || i === g.r0 * w + g.c0) next[i] = g.to
    }
    o.current.onPaint(next)
    return next
  }
  const down = (e: React.PointerEvent<HTMLDivElement>) => {
    if (o.current.disabled || (e.button !== 0 && e.button !== 2)) return
    e.preventDefault()
    try { e.currentTarget.setPointerCapture(e.pointerId) } catch { /* old browsers */ }
    const [r, c] = at(e)
    const i = r * o.current.w + c
    if (o.current.locked?.(i)) return
    const base = o.current.get()
    const from = base[i]
    const g: Stroke = { id: e.pointerId, r0: r, c0: c, axis: '', from, to: o.current.target(from, e.button === 2), base, timer: 0, touch: e.pointerType === 'touch' }
    st.current = g
    paintTo(r, c)
    const lp = o.current.longPress
    if (g.touch && lp && e.button === 0) {
      g.timer = window.setTimeout(() => {
        const cur = st.current
        if (!cur || cur !== g || cur.axis) return
        cur.to = lp(cur.from)
        paintTo(cur.r0, cur.c0)
        try { navigator.vibrate?.(12) } catch { /* not on every phone */ }
      }, 450)
    }
  }
  const move = (e: React.PointerEvent<HTMLDivElement>) => {
    const g = st.current
    if (!g || g.id !== e.pointerId) return
    let [r, c] = at(e)
    if (!g.axis) {
      if (r === g.r0 && c === g.c0) return
      window.clearTimeout(g.timer)
      g.axis = Math.abs(c - g.c0) >= Math.abs(r - g.r0) ? 'r' : 'c'
    }
    if (g.axis === 'r') r = g.r0; else c = g.c0
    paintTo(r, c)
  }
  const up = (e: React.PointerEvent<HTMLDivElement>) => {
    const g = st.current
    if (!g || g.id !== e.pointerId) return
    window.clearTimeout(g.timer)
    st.current = null
    o.current.onEnd(o.current.get())
  }
  useEffect(() => () => { if (st.current) window.clearTimeout(st.current.timer) }, [])
  return { ref: (x: HTMLDivElement | null) => { el.current = x }, handlers: { onPointerDown: down, onPointerMove: move, onPointerUp: up, onPointerCancel: up, onContextMenu: (e: React.MouseEvent) => e.preventDefault() } }
}

interface BoardProps {
  w: number; h: number; rows: number[][]; cols: number[][]; cells: number[]; s: number; left: number; top: number; font: number
  rowOk?: boolean[]; colOk?: boolean[]; wrongRows?: Set<number>; wrongCols?: Set<number>; hot?: number | null; cursor?: number | null
  lockedRows?: Set<number>; color?: string; className?: string; showMarks?: boolean
  cellsRef?: (el: HTMLDivElement | null) => void; handlers?: React.HTMLAttributes<HTMLDivElement>
}

/** The numbers on top and on the left, and the squares. */
function NGBoard({ w, h, rows, cols, cells, s, left, top, font, rowOk, colOk, wrongRows, wrongCols, hot, cursor, lockedRows, color, className = '', showMarks = true, cellsRef, handlers }: BoardProps) {
  const hr = hot != null ? Math.floor(hot / w) : -1, hc = hot != null ? hot % w : -1
  return (
    <div className={`ng-board ${className}`} style={{ gridTemplateColumns: `${left}px ${w * s}px`, gridTemplateRows: `${top}px ${h * s}px`, ['--ng' as any]: `${s}px`, ['--ng-font' as any]: `${font}px`, ['--ng-fill' as any]: color || undefined }}>
      <div className="ng-corner" />
      <div className="ng-top" style={{ gridTemplateColumns: `repeat(${w}, ${s}px)` }}>
        {cols.map((cl, c) => (
          <div key={c} className={`ng-clue col ${colOk?.[c] ? 'ok' : ''} ${wrongCols?.has(c) ? 'bad' : ''} ${c === hc ? 'hot' : ''} ${(c + 1) % 5 === 0 && c < w - 1 ? 'b5' : ''}`}>
            {(cl.length ? cl : [0]).map((v, k) => <span key={k}>{v}</span>)}
          </div>
        ))}
      </div>
      <div className="ng-left" style={{ gridTemplateRows: `repeat(${h}, ${s}px)` }}>
        {rows.map((rl, r) => (
          <div key={r} className={`ng-clue row ${rowOk?.[r] ? 'ok' : ''} ${wrongRows?.has(r) ? 'bad' : ''} ${r === hr ? 'hot' : ''} ${(r + 1) % 5 === 0 && r < h - 1 ? 'b5' : ''}`}>
            {(rl.length ? rl : [0]).map((v, k) => <span key={k}>{v}</span>)}
          </div>
        ))}
      </div>
      <div ref={cellsRef} className="ng-cells" style={{ gridTemplateColumns: `repeat(${w}, ${s}px)`, gridAutoRows: `${s}px` }} {...handlers}>
        {cells.map((v, i) => {
          const r = Math.floor(i / w), c = i % w
          const cls = ['ng-cell', v === 1 ? 'on' : v === 2 && showMarks ? 'x' : '', (c + 1) % 5 === 0 && c < w - 1 ? 'b5r' : '', (r + 1) % 5 === 0 && r < h - 1 ? 'b5b' : '',
            r === hr || c === hc ? 'hot' : '', i === cursor ? 'cur' : '', lockedRows?.has(r) ? 'lock' : ''].join(' ')
          return <div key={i} className={cls} />
        })}
      </div>
    </div>
  )
}

// ---- play ----

function Play({ view, progress, shown, done, onProgress, check, reveal, canReveal, setActions }: KindPlayProps) {
  const d = view.data as NGPublic
  const n = d.w * d.h
  const [cells, setCells] = useState<number[]>(() => Array.isArray(progress?.cells) && progress.cells.length === n ? progress.cells : Array(n).fill(0))
  const [mode, setMode] = useState<'fill' | 'mark'>('fill')
  const [wrongRows, setWrongRows] = useState<Set<number>>(new Set())
  const [wrongCols, setWrongCols] = useState<Set<number>>(new Set())
  const [hot, setHot] = useState<number | null>(null)
  const [cursor, setCursor] = useState<number | null>(null)
  const [locked, setLocked] = useState<Set<number>>(() => new Set(shown.filter(r => r >= 0 && r < d.h)))
  const ref = useRef(cells)
  ref.current = cells
  const sent = useRef(false)
  const [box, bw] = useWidth(600)
  const L = layout(Math.min(bw, 760), d.w, d.h, d.rows, d.cols, 36)

  const rowOk = useMemo(() => d.rows.map((cl, r) => same(runs(rowOf(cells, d.w, r)), cl)), [cells, d.rows, d.w])
  const colOk = useMemo(() => d.cols.map((cl, c) => same(runs(colOf(cells, d.w, d.h, c)), cl)), [cells, d.cols, d.w, d.h])
  const left = useMemo(() => {
    let need = 0
    for (const cl of d.rows) for (const v of cl) need += v
    return need - cells.filter(v => v === 1).length
  }, [cells, d.rows])

  const finish = (next: number[]) => {
    const ok = d.rows.every((cl, r) => same(runs(rowOf(next, d.w, r)), cl)) && d.cols.every((cl, c) => same(runs(colOf(next, d.w, d.h, c)), cl))
    if (!ok) { sent.current = false; return }
    if (sent.current) return
    sent.current = true
    check({ cells: next.map(v => v === 1 ? 1 : 0) }, true).then(r => { if (!r?.solved) sent.current = false })
  }
  const commit = (next: number[]) => {
    ref.current = next
    setCells(next)
    onProgress({ cells: next })
    finish(next)
  }
  const paint = usePaint({
    w: d.w, h: d.h, s: L.s, disabled: done, get: () => ref.current,
    locked: i => locked.has(Math.floor(i / d.w)),
    target: (old, alt) => (mode === 'mark') !== alt ? (old === 2 ? 0 : 2) : (old === 1 ? 0 : 1),
    longPress: old => old === 2 ? 0 : 2,
    onPaint: next => { ref.current = next; setCells(next); if (wrongRows.size || wrongCols.size) { setWrongRows(new Set()); setWrongCols(new Set()) } },
    onEnd: next => commit(next),
  })

  const setOne = (i: number, to: (old: number) => number) => {
    if (done || locked.has(Math.floor(i / d.w))) return
    const next = [...ref.current]
    next[i] = to(next[i])
    setWrongRows(new Set()); setWrongCols(new Set())
    commit(next)
  }
  const onKey = (ev: React.KeyboardEvent) => {
    if (done || ev.metaKey || ev.ctrlKey || ev.altKey) return
    const k = ev.key
    const cur = cursor ?? 0
    const mv = (dr: number, dc: number) => { ev.preventDefault(); if (cursor === null) { setCursor(0); return } setCursor(clamp(Math.floor(cur / d.w) + dr, 0, d.h - 1) * d.w + clamp(cur % d.w + dc, 0, d.w - 1)) }
    if (k === 'ArrowUp') mv(-1, 0)
    else if (k === 'ArrowDown') mv(1, 0)
    else if (k === 'ArrowLeft') mv(0, -1)
    else if (k === 'ArrowRight') mv(0, 1)
    else if (cursor === null) return
    else if (k === ' ' || k === 'Enter') { ev.preventDefault(); const mark = (mode === 'mark') !== ev.shiftKey; setOne(cur, v => mark ? (v === 2 ? 0 : 2) : (v === 1 ? 0 : 1)) }
    else if (k === 'x' || k === 'X') { ev.preventDefault(); setOne(cur, v => v === 2 ? 0 : 2) }
    else if (k === 'Backspace' || k === 'Delete') { ev.preventDefault(); setOne(cur, () => 0) }
  }

  useEffect(() => {
    const row = rowOk.findIndex((ok, r) => !ok && !locked.has(r))
    const target = row >= 0 ? row : Array.from({ length: d.h }, (_, r) => r).find(r => !locked.has(r)) ?? -1
    setActions({
      check: async () => {
        const r = await check({ cells: ref.current.map(v => v === 1 ? 1 : 0) }, true)
        if (r && !r.solved) {
          const w = r.wrong || []
          setWrongRows(new Set(w.filter(x => x < 1000)))
          setWrongCols(new Set(w.filter(x => x >= 1000).map(x => x - 1000)))
        }
      },
      reveal: canReveal && target >= 0 ? async () => {
        const v = await reveal(target)
        if (v && Array.isArray(v.cells)) {
          const row = typeof v.row === 'number' ? v.row : target
          const next = [...ref.current]
          ;(v.cells as number[]).forEach((x, c) => { next[row * d.w + c] = x === 1 ? 1 : 2 })
          setLocked(s => new Set([...s, row]))
          setWrongRows(new Set()); setWrongCols(new Set())
          commit(next)
        }
      } : undefined,
      revealLabel: 'Show a row',
      clear: () => {
        const next = ref.current.map((v, i) => locked.has(Math.floor(i / d.w)) ? v : 0)
        setWrongRows(new Set()); setWrongCols(new Set())
        ref.current = next; setCells(next); onProgress({ cells: next })
      },
    })
  }, [rowOk, locked, canReveal]) // eslint-disable-line react-hooks/exhaustive-deps

  const onHover = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.pointerType !== 'mouse') return
    const rect = e.currentTarget.getBoundingClientRect()
    const c = Math.floor((e.clientX - rect.left) / L.s), r = Math.floor((e.clientY - rect.top) / L.s)
    const i = r >= 0 && c >= 0 && r < d.h && c < d.w ? r * d.w + c : null
    if (i !== hot) setHot(i)
  }
  return (
    <div className="ng-play" ref={box}>
      <div className="ng-bar">
        <Segmented value={mode} onChange={setMode} label="What a tap does" options={[
          { value: 'fill', label: <span className="ng-mode"><span className="ng-sw fill" />Fill</span> },
          { value: 'mark', label: <span className="ng-mode"><span className="ng-sw mark">✕</span>Mark empty</span> }]} />
        <span className="muted small ng-left-count" aria-live="polite">{done ? 'Finished' : left > 0 ? `${left} to fill` : left === 0 ? 'All filled — check the numbers' : `${-left} too many`}</span>
      </div>
      <div className="ng-scroll">
        <div className="ng-focus" tabIndex={done ? -1 : 0} onKeyDown={onKey} onBlur={() => setCursor(null)} aria-label="Picture grid. Arrows move, space fills, X marks empty.">
          <NGBoard w={d.w} h={d.h} rows={d.rows} cols={d.cols} cells={cells} s={L.s} left={L.left} top={L.top} font={L.font} rowOk={rowOk} colOk={colOk}
            wrongRows={wrongRows} wrongCols={wrongCols} hot={done ? null : hot} cursor={done ? null : cursor} lockedRows={locked} color={d.color} showMarks={!done}
            className={done ? 'is-done' : ''} cellsRef={paint.ref}
            handlers={{ ...paint.handlers, onPointerDown: e => { setCursor(null); paint.handlers.onPointerDown(e) }, onPointerMove: e => { onHover(e); paint.handlers.onPointerMove(e) }, onPointerLeave: () => setHot(null) }} />
        </div>
      </div>
      <p className="ng-tip muted small">The numbers say how many filled squares come together in that row or column, in order. {done ? '' : 'Long-press (or right-click) marks a square as empty.'}</p>
    </div>
  )
}

// ---- make ----

const SIZES = Array.from({ length: 21 }, (_, k) => k + 5).map(v => ({ value: String(v), label: String(v) }))
const COLORS = ['', '#1f2937', '#0f766e', '#2563eb', '#7c3aed', '#be123c', '#c2410c', '#15803d']

function Edit({ data, onChange }: KindEditProps) {
  const d = data as NGFull
  const w = d.w || 10, h = d.h || 10
  const cells = d.cells?.length === w * h ? d.cells : Array(w * h).fill(0)
  const [live, setLive] = useState<number[] | null>(null)
  const shownCells = live || cells
  const clues = useMemo(() => cluesOf(shownCells, w, h), [shownCells, w, h])
  const [lib, setLib] = useState(false)
  const [fromPhoto, setFromPhoto] = useState(false)
  const [box, bw] = useWidth(600)
  const L = layout(Math.min(bw, 680), w, h, clues.rows, clues.cols, 30)
  const ref = useRef(cells)
  ref.current = live || cells

  const save = (next: number[], nw = w, nh = h) => { setLive(null); onChange({ ...d, w: nw, h: nh, cells: next, ...cluesOf(next, nw, nh) }) }
  const paint = usePaint({
    w, h, s: L.s, get: () => ref.current,
    target: (old, alt) => alt ? 0 : old === 1 ? 0 : 1,
    onPaint: next => { ref.current = next; setLive(next) },
    onEnd: next => save(next),
  })
  const resize = (nw: number, nh: number) => {
    const next = Array(nw * nh).fill(0)
    for (let r = 0; r < Math.min(nh, h); r++) for (let c = 0; c < Math.min(nw, w); c++) next[r * nw + c] = cells[r * w + c]
    save(next, nw, nh)
  }
  const filled = shownCells.filter(v => v === 1).length
  return (
    <div className="ng-edit" ref={box}>
      <div className="ed-toolbar">
        <span className="muted small">Across</span>
        <Select ariaLabel="Squares across" value={String(w)} onChange={v => resize(Number(v), h)} options={SIZES} />
        <span className="muted small">Down</span>
        <Select ariaLabel="Squares down" value={String(h)} onChange={v => resize(w, Number(v))} options={SIZES} />
        <span className="spacer" />
        <button type="button" className="btn sm" onClick={() => setLib(true)}><Icon name="book-open" size={14} />From the library</button>
        <button type="button" className="btn sm" onClick={() => setFromPhoto(true)}><Icon name="image" size={14} />From a photo</button>
      </div>
      <div className="ed-toolbar">
        <button type="button" className="btn sm ghost" onClick={() => save(cells.map(v => v === 1 ? 0 : 1))}><Icon name="refresh" size={14} />Swap filled and empty</button>
        <button type="button" className="btn sm ghost" onClick={() => save(Array(w * h).fill(0))} disabled={!filled}><Icon name="trash" size={14} />Clear</button>
        <span className="spacer" />
        <span className="muted small">Colour</span>
        <div className="ng-colors" role="radiogroup" aria-label="Colour of the picture">
          {COLORS.map(c => <button key={c || 'plain'} type="button" role="radio" aria-checked={(d.color || '') === c} className={`ng-color ${(d.color || '') === c ? 'on' : ''}`} title={c ? c : 'Plain'}
            style={{ ['--sw' as any]: c || 'var(--text)' }} onClick={() => onChange({ ...d, color: c })} />)}
        </div>
      </div>
      <p className="muted small ed-help">Tap squares to fill them, or drag along a row or column. Right-click (or start on a filled square) rubs out. The numbers update as you paint.</p>
      <div className="ng-scroll">
        <NGBoard w={w} h={h} rows={clues.rows} cols={clues.cols} cells={shownCells} s={L.s} left={L.left} top={L.top} font={L.font} color={d.color} className="edit" cellsRef={paint.ref} handlers={paint.handlers} />
      </div>
      <p className="muted small ng-count">{filled} of {w * h} squares filled{filled === 0 ? ' — paint something, or take one from the library' : ''}</p>
      {lib && <Library onClose={() => setLib(false)} onPick={art => {
        const nh = art.length, nw = Math.max(...art.map(l => l.length))
        const next = Array(nw * nh).fill(0)
        art.forEach((l, r) => Array.from(l).forEach((ch, c) => { if (ch === '#' || ch === 'X') next[r * nw + c] = 1 }))
        save(next, nw, nh)
        setLib(false)
      }} />}
      {fromPhoto && <FromPhoto w={w} h={h} onClose={() => setFromPhoto(false)} onUse={(next, nw, nh) => { save(next, nw, nh); setFromPhoto(false) }} />}
    </div>
  )
}

/** A little picture of rows of "#" and ".". */
function ArtThumb({ art }: { art: string[] }) {
  const h = art.length, w = Math.max(...art.map(l => l.length))
  const s = Math.max(3, Math.floor(84 / Math.max(w, h)))
  return (
    <svg width={w * s} height={h * s} viewBox={`0 0 ${w * s} ${h * s}`} aria-hidden="true">
      {art.flatMap((l, r) => Array.from(l).map((ch, c) => ch === '#' || ch === 'X' ? <rect key={`${r}-${c}`} x={c * s} y={r * s} width={s} height={s} /> : null))}
    </svg>
  )
}

function Library({ onClose, onPick }: { onClose: () => void; onPick: (art: string[]) => void }) {
  const [pics, setPics] = useState<Record<string, string[]> | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => { loadThemes().then(r => setPics(r.pictures || {})).catch(e => setErr((e as Error).message)) }, [])
  const names = pics ? Object.keys(pics).sort() : []
  return (
    <Modal title="Pictures from the library" onClose={onClose} size="lg">
      <p className="muted small">Picking one replaces what you painted.</p>
      {err ? <div className="callout warn"><Icon name="alert" size={15} /><div>{err}</div></div> : !pics ? <Spinner /> :
        <div className="ng-lib">{names.map(k => {
          const art = pics[k]
          const w = Math.max(...art.map(l => l.length))
          return (
            <button key={k} type="button" className="ng-lib-tile" onClick={() => onPick(art)}>
              <ArtThumb art={art} />
              <span className="ng-lib-name">{k.replace(/[-_]/g, ' ')}</span>
              <span className="muted small">{w} × {art.length}</span>
            </button>
          )
        })}</div>}
    </Modal>
  )
}

/** A photo drawn into the grid in the browser: dark squares are filled. */
function FromPhoto({ w, h, onClose, onUse }: { w: number; h: number; onClose: () => void; onUse: (cells: number[], w: number, h: number) => void }) {
  const { toast } = useToast()
  const file = useRef<HTMLInputElement>(null)
  const [img, setImg] = useState<HTMLImageElement | null>(null)
  const [busy, setBusy] = useState(false)
  const [gw, setGw] = useState(w)
  const [fit, setFit] = useState(true)
  const [thr, setThr] = useState(50)
  const [inv, setInv] = useState(false)
  const gh = img && fit ? clamp(Math.round(gw * img.naturalHeight / img.naturalWidth), 5, 25) : h
  const lum = useMemo(() => {
    if (!img) return null
    const k = 8
    const cv = document.createElement('canvas')
    cv.width = gw * k; cv.height = gh * k
    const g = cv.getContext('2d')
    if (!g) return null
    g.fillStyle = '#fff'
    g.fillRect(0, 0, cv.width, cv.height)
    // the whole picture, centred, keeping its shape
    const sc = Math.min(cv.width / img.naturalWidth, cv.height / img.naturalHeight)
    const dw = img.naturalWidth * sc, dh = img.naturalHeight * sc
    g.imageSmoothingQuality = 'high'
    g.drawImage(img, (cv.width - dw) / 2, (cv.height - dh) / 2, dw, dh)
    let px: Uint8ClampedArray
    try { px = g.getImageData(0, 0, cv.width, cv.height).data } catch { return null }
    const out: number[] = []
    for (let r = 0; r < gh; r++) for (let c = 0; c < gw; c++) {
      let sum = 0
      for (let y = 0; y < k; y++) for (let x = 0; x < k; x++) {
        const i = ((r * k + y) * cv.width + c * k + x) * 4
        sum += (0.299 * px[i] + 0.587 * px[i + 1] + 0.114 * px[i + 2]) / 255
      }
      out.push(sum / (k * k))
    }
    return out
  }, [img, gw, gh])
  // start the slider at the picture's own middle brightness
  useEffect(() => { if (lum?.length) { const m = lum.reduce((a, b) => a + b, 0) / lum.length; setThr(clamp(Math.round(m * 100), 10, 90)) } }, [img]) // eslint-disable-line react-hooks/exhaustive-deps
  const cells = useMemo(() => lum ? lum.map(l => (inv ? l * 100 >= thr : l * 100 < thr) ? 1 : 0) : null, [lum, thr, inv])
  const load = (src: string, revoke = false) => {
    setBusy(true)
    const im = new Image()
    im.onload = () => { setImg(im); setBusy(false); if (revoke) URL.revokeObjectURL(src) }
    im.onerror = () => { setBusy(false); toast('That picture could not be read', 'err') }
    im.src = src
  }
  const s = cells ? clamp(Math.floor(300 / Math.max(gw, gh)), 8, 20) : 0
  return (
    <Modal title="A picture puzzle from a photo" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={!cells} onClick={() => cells && onUse(cells, gw, gh)}>Use it</button></>}>
      <p className="muted small">A simple picture with a plain background works best — a silhouette, a logo, a drawing. A picture from this device is only read here (not uploaded); only the squares are kept.</p>
      <div className="row wrap">
        <button type="button" className="btn" onClick={() => file.current?.click()} disabled={busy}>{busy ? <Spinner /> : <Icon name="upload" size={14} />}A picture from this device</button>
        <input ref={file} type="file" accept="image/*" hidden onChange={e => { const f = e.target.files?.[0]; if (f) load(URL.createObjectURL(f), true); e.target.value = '' }} />
      </div>
      <PhotoPick kind="puzzle" label="Upload a picture" onPicked={p => load(p.url)} />
      {img && <div className="ng-photo">
        <div className="ng-photo-ctl">
          <label className="ng-field"><span className="muted small">Squares across</span>
            <Select ariaLabel="Squares across" value={String(gw)} onChange={v => setGw(Number(v))} options={SIZES} />
          </label>
          <label className="row ng-switch"><Switch size="sm" checked={fit} onChange={setFit} label="Match the photo's shape" /><span className="small">Match the photo's shape ({gw} × {gh})</span></label>
          <label className="ng-field"><span className="muted small">Light or dark: {thr}%</span>
            <input type="range" min={5} max={95} value={thr} onChange={e => setThr(Number(e.target.value))} aria-label="Which squares count as dark" />
          </label>
          <label className="row ng-switch"><Switch size="sm" checked={inv} onChange={setInv} label="Fill the light parts instead" /><span className="small">Fill the light parts instead</span></label>
        </div>
        {cells && <div className="ng-photo-prev" style={{ gridTemplateColumns: `repeat(${gw}, ${s}px)`, gridAutoRows: `${s}px` }}>
          {cells.map((v, i) => <span key={i} className={v ? 'on' : ''} />)}
        </div>}
      </div>}
    </Modal>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as NGPublic
  const [box, bw] = useWidth(640)
  if (answer) {
    const a = answer as NGFull
    const s = clamp(Math.floor(Math.min(bw, 240) / a.w), 6, 14)
    return (
      <div className="ng-print answer" ref={box}>
        <div className="ng-key" style={{ gridTemplateColumns: `repeat(${a.w}, ${s}px)`, gridAutoRows: `${s}px` }}>
          {a.cells.map((v, i) => <span key={i} className={v === 1 ? 'on' : ''} />)}
        </div>
      </div>
    )
  }
  const L = layout(Math.min(bw, 700), d.w, d.h, d.rows, d.cols, large ? 40 : 30)
  return (
    <div className={`ng-print ${large ? 'large' : ''}`} ref={box}>
      <NGBoard w={d.w} h={d.h} rows={d.rows} cols={d.cols} cells={Array(d.w * d.h).fill(0)} s={L.s} left={L.left} top={L.top} font={L.font} className="print" />
      <p className="ng-print-rule">Shade the squares. The numbers say how many shaded squares come together in each row and column, in that order, with at least one empty square between groups.</p>
    </div>
  )
}

export const Nonogram: KindModule = { Play, Edit, Print }
