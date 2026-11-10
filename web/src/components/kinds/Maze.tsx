// Maze: play (draw the way from the green square to the red one with a
// finger, the mouse or the arrow keys; go back over it to undo), make (a new
// maze of a size), print.
import '../../styles/kind-maze.css'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import { Icon } from '../icons'
import { Segmented, Spinner, Switch, useToast } from '../ui'
import { Select } from '../Select'
import { useWidth } from './Crossword'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shared ----

interface MZPublic { w: number; h: number; walls: number[]; start: number; end: number; theme?: string }
interface MZFull extends MZPublic { seed: number; path?: number[] }

const N = 1, E = 2, S = 4, W = 8
const clamp = (v: number, a: number, b: number) => Math.max(a, Math.min(b, v))

/** Can you step from square a to its neighbour b? */
function open(m: MZPublic, a: number, b: number): boolean {
  const { w, h, walls } = m
  if (b < 0 || b >= w * h) return false
  const ar = Math.floor(a / w), ac = a % w, br = Math.floor(b / w), bc = b % w
  if (ar === br && bc === ac + 1) return !(walls[a] & E) && !(walls[b] & W)
  if (ar === br && bc === ac - 1) return !(walls[a] & W) && !(walls[b] & E)
  if (ac === bc && br === ar + 1) return !(walls[a] & S) && !(walls[b] & N)
  if (ac === bc && br === ar - 1) return !(walls[a] & N) && !(walls[b] & S)
  return false
}

/** The way through (shortest), worked out here for the editor's preview. */
function solve(m: MZPublic): number[] {
  const n = m.w * m.h
  if (m.walls.length !== n) return []
  const prev = new Int32Array(n).fill(-2)
  prev[m.start] = -1
  const q = [m.start]
  for (let k = 0; k < q.length; k++) {
    const cur = q[k]
    if (cur === m.end) break
    for (const nb of [cur - m.w, cur + 1, cur + m.w, cur - 1]) {
      if (nb < 0 || nb >= n || prev[nb] !== -2 || !open(m, cur, nb)) continue
      prev[nb] = cur
      q.push(nb)
    }
  }
  if (prev[m.end] === -2) return []
  const out: number[] = []
  for (let at = m.end; at !== -1; at = prev[at]) out.push(at)
  return out.reverse()
}

/** From the end of the path to a square: a neighbour, or straight along an open corridor, or round one corner. */
function route(m: MZPublic, a: number, b: number): number[] | null {
  if (a === b) return []
  const w = m.w
  const ar = Math.floor(a / w), ac = a % w, br = Math.floor(b / w), bc = b % w
  if (ar === br || ac === bc) {
    const step = ar === br ? Math.sign(bc - ac) : Math.sign(br - ar) * w
    const out: number[] = []
    for (let cur = a; cur !== b; cur += step) {
      if (!open(m, cur, cur + step)) return null
      out.push(cur + step)
    }
    return out
  }
  if (Math.abs(br - ar) === 1 && Math.abs(bc - ac) === 1) {
    for (const mid of [ar * w + bc, br * w + ac]) if (open(m, a, mid) && open(m, mid, b)) return [mid, b]
  }
  return null
}

interface SVGProps {
  m: MZPublic; s: number; path?: number[]; way?: number[]; className?: string; head?: boolean; print?: boolean
  svgRef?: (el: SVGSVGElement | null) => void; handlers?: React.SVGAttributes<SVGSVGElement>
}

/** The maze as lines, sized in real pixels from the measured width. */
function MazeSVG({ m, s, path, way, className = '', head = true, print, svgRef, handlers }: SVGProps) {
  const pad = Math.max(3, Math.ceil(s * 0.15))
  const W2 = m.w * s + pad * 2, H2 = m.h * s + pad * 2
  const wallD = useMemo(() => {
    const { w, h, walls } = m
    let d = ''
    for (let i = 0; i < w * h; i++) {
      const r = Math.floor(i / w), c = i % w
      const x = pad + c * s, y = pad + r * s
      if (walls[i] & N || (r > 0 && walls[i - w] & S)) d += `M${x} ${y}h${s}`
      if (walls[i] & W || (c > 0 && walls[i - 1] & E)) d += `M${x} ${y}v${s}`
      if (c === w - 1 && walls[i] & E) d += `M${x + s} ${y}v${s}`
      if (r === h - 1 && walls[i] & S) d += `M${x} ${y + s}h${s}`
    }
    return d
  }, [m, s, pad])
  const pt = (i: number) => `${pad + (i % m.w) * s + s / 2},${pad + Math.floor(i / m.w) * s + s / 2}`
  const cx = (i: number) => pad + (i % m.w) * s + s / 2
  const cy = (i: number) => pad + Math.floor(i / m.w) * s + s / 2
  const box = (i: number, cls: string) => <rect className={cls} x={pad + (i % m.w) * s + 1} y={pad + Math.floor(i / m.w) * s + 1} width={s - 2} height={s - 2} rx={Math.min(6, s * 0.18)} />
  const wall = Math.max(print ? 1.4 : 1.6, s * 0.11)
  const last = path && path.length ? path[path.length - 1] : -1
  return (
    <svg ref={svgRef} className={`mz-svg ${className}`} width={W2} height={H2} viewBox={`0 0 ${W2} ${H2}`} {...handlers}>
      {box(m.start, 'mz-start')}
      {box(m.end, 'mz-end')}
      {way && way.length > 1 && <polyline className="mz-way" points={way.map(pt).join(' ')} strokeWidth={Math.max(2, s * 0.22)} strokeDasharray={`${Math.max(2, s * 0.2)} ${Math.max(3, s * 0.3)}`} />}
      {path && path.length > 1 && <polyline className="mz-path" points={path.map(pt).join(' ')} strokeWidth={Math.max(3, s * 0.36)} />}
      {print && <>
        <text className="mz-mark" x={cx(m.start)} y={cy(m.start)} fontSize={Math.max(8, s * 0.6)}>●</text>
        <text className="mz-mark" x={cx(m.end)} y={cy(m.end)} fontSize={Math.max(8, s * 0.66)}>★</text>
      </>}
      <path className="mz-walls" d={wallD} strokeWidth={wall} />
      {head && last >= 0 && <circle className="mz-head" cx={cx(last)} cy={cy(last)} r={Math.max(3, s * 0.27)} />}
    </svg>
  )
}

const sizeFor = (avail: number, w: number, max = 40) => clamp(Math.floor((Math.min(avail, 760) - 10) / w), 7, max)

// ---- play ----

function Play({ view, progress, done, onProgress, check, reveal, canReveal, setActions }: KindPlayProps) {
  const d = view.data as MZPublic
  const okPath = (p: unknown): p is number[] => Array.isArray(p) && p.length > 0 && p[0] === d.start
  const [path, setPath] = useState<number[]>(() => okPath(progress?.path) ? progress.path : [d.start])
  const [way, setWay] = useState<number[]>(() => Array.isArray(progress?.way) ? progress.way : [])
  const [bump, setBump] = useState(false)
  const ref = useRef({ path, way })
  ref.current = { path, way }
  const drag = useRef<number | null>(null)
  const lastCell = useRef(-1)
  const svgEl = useRef<SVGSVGElement | null>(null)
  const sent = useRef(false)
  const bumpTimer = useRef(0)
  const [box, bw] = useWidth(600)
  const s = sizeFor(bw, d.w)
  const pad = Math.max(3, Math.ceil(s * 0.15))

  const save = (next: number[], nextWay = ref.current.way) => {
    ref.current = { path: next, way: nextWay }
    setPath(next)
    onProgress({ path: next, way: nextWay })
    const at = next[next.length - 1]
    if (at === d.end && !sent.current) {
      sent.current = true
      check({ path: next }, true).then(r => { if (!r?.solved) sent.current = false })
    } else if (at !== d.end) sent.current = false
  }
  const goTo = (i: number) => {
    const p = ref.current.path
    const head = p[p.length - 1]
    if (i === head) return true
    const k = p.indexOf(i)
    if (k >= 0) { save(p.slice(0, k + 1)); return true }
    if (head === d.end) return false
    const steps = route(d, head, i)
    if (!steps) return false
    // going along the way already drawn, backwards, shortens it
    const next = [...p]
    for (const st of steps) {
      const j = next.indexOf(st)
      if (j >= 0) next.length = j + 1; else next.push(st)
    }
    save(next)
    return true
  }
  const cellOf = (e: React.PointerEvent): number | null => {
    const rect = svgEl.current!.getBoundingClientRect()
    const x = e.clientX - rect.left - pad, y = e.clientY - rect.top - pad
    const c = Math.floor(x / s), r = Math.floor(y / s)
    if (r < 0 || c < 0 || r >= d.h || c >= d.w) return null
    return r * d.w + c
  }
  const onDown = (e: React.PointerEvent<SVGSVGElement>) => {
    if (done || e.button > 0) return
    const i = cellOf(e)
    if (i === null) return
    e.preventDefault()
    try { e.currentTarget.setPointerCapture(e.pointerId) } catch { /* old browsers */ }
    drag.current = e.pointerId
    lastCell.current = i
    goTo(i)
  }
  const onMove = (e: React.PointerEvent<SVGSVGElement>) => {
    if (drag.current !== e.pointerId) return
    // fast drags: also walk the squares between the last point and this one
    const i = cellOf(e)
    if (i === null || i === lastCell.current) return
    const from = lastCell.current
    lastCell.current = i
    if (from < 0) { goTo(i); return }
    const r0 = Math.floor(from / d.w), c0 = from % d.w, r1 = Math.floor(i / d.w), c1 = i % d.w
    const n = Math.max(Math.abs(r1 - r0), Math.abs(c1 - c0))
    let prev = from
    for (let k = 1; k <= n; k++) {
      const r = Math.round(r0 + (r1 - r0) * k / n), c = Math.round(c0 + (c1 - c0) * k / n)
      const at = r * d.w + c
      if (at === prev) continue
      // a diagonal jump: go through the square beside it first
      if (Math.floor(prev / d.w) !== r && prev % d.w !== c) goTo(Math.floor(prev / d.w) * d.w + c)
      goTo(at)
      prev = at
    }
  }
  const onUp = (e: React.PointerEvent<SVGSVGElement>) => { if (drag.current === e.pointerId) { drag.current = null; lastCell.current = -1 } }

  const onKey = (ev: React.KeyboardEvent) => {
    if (done || ev.metaKey || ev.ctrlKey || ev.altKey) return
    const p = ref.current.path
    const head = p[p.length - 1]
    const r = Math.floor(head / d.w), c = head % d.w
    const dirs: Record<string, [number, number]> = { ArrowUp: [-1, 0], ArrowDown: [1, 0], ArrowLeft: [0, -1], ArrowRight: [0, 1], w: [-1, 0], s: [1, 0], a: [0, -1], d: [0, 1] }
    const dd = dirs[ev.key]
    if (dd) {
      ev.preventDefault()
      const nr = r + dd[0], nc = c + dd[1]
      const nb = nr * d.w + nc
      if (nr < 0 || nc < 0 || nr >= d.h || nc >= d.w || !open(d, head, nb) || (head === d.end && !p.includes(nb))) {
        window.clearTimeout(bumpTimer.current)
        setBump(true)
        bumpTimer.current = window.setTimeout(() => setBump(false), 180)
        return
      }
      goTo(nb)
    } else if (ev.key === 'Backspace') {
      ev.preventDefault()
      if (p.length > 1) save(p.slice(0, -1))
    }
  }
  useEffect(() => () => window.clearTimeout(bumpTimer.current), [])

  useEffect(() => {
    setActions({
      check: async () => { await check({ path: ref.current.path }) },
      reveal: canReveal && !way.length ? async () => {
        const v = await reveal(0)
        if (v && Array.isArray(v.path)) { setWay(v.path); ref.current = { ...ref.current, way: v.path }; onProgress({ path: ref.current.path, way: v.path }) }
      } : undefined,
      revealLabel: 'Show the way',
      clear: () => save([d.start]),
    })
  }, [way.length, canReveal]) // eslint-disable-line react-hooks/exhaustive-deps

  const steps = path.length - 1
  return (
    <div className="mz-play" ref={box}>
      <div className={`mz-board ${bump ? 'bump' : ''}`} tabIndex={done ? -1 : 0} onKeyDown={onKey} aria-label="Maze. Draw from the green square to the red one; the arrow keys move, Backspace goes back.">
        <MazeSVG m={d} s={s} path={path} way={done ? undefined : way} className={done ? 'is-done' : ''} head={!done}
          svgRef={el => { svgEl.current = el }} handlers={{ onPointerDown: onDown, onPointerMove: onMove, onPointerUp: onUp, onPointerCancel: onUp }} />
      </div>
      <p className="mz-tip muted small" aria-live="polite">
        {done ? `Out in ${steps} steps.` : steps === 0 ? 'Start on the green square and draw the way to the red one.' : `${steps} ${steps === 1 ? 'step' : 'steps'} — go back over your line to undo.`}
      </p>
    </div>
  )
}

// ---- make ----

const PRESETS = [{ v: 10, label: 'Small' }, { v: 16, label: 'Medium' }, { v: 24, label: 'Large' }, { v: 34, label: 'Huge' }]
const DIMS = Array.from({ length: 57 }, (_, k) => k + 4).map(v => ({ value: String(v), label: String(v) }))
type Ends = 'corners' | 'sides' | 'top'
function endsFor(kind: Ends, w: number, h: number): [number, number] {
  if (kind === 'sides') { const r = Math.floor(h / 2); return [r * w, r * w + w - 1] }
  if (kind === 'top') { const c = Math.floor(w / 2); return [c, (h - 1) * w + c] }
  return [0, w * h - 1]
}

function Edit({ data, onChange }: KindEditProps) {
  const { toast } = useToast()
  const m = data as MZFull
  const has = m.w > 0 && m.h > 0 && m.walls?.length === m.w * m.h
  const preset = PRESETS.find(p => p.v === m.w && p.v === m.h)
  const [own, setOwn] = useState(has && !preset)
  const [busy, setBusy] = useState(false)
  const [showWay, setShowWay] = useState(false)
  const [box, bw] = useWidth(600)
  const ends: Ends = has && m.start === endsFor('sides', m.w, m.h)[0] && m.end === endsFor('sides', m.w, m.h)[1] ? 'sides'
    : has && m.start === endsFor('top', m.w, m.h)[0] && m.end === endsFor('top', m.w, m.h)[1] ? 'top' : 'corners'
  const way = useMemo(() => has && showWay ? solve(m) : [], [m, has, showWay])

  const make = async (w: number, h: number, keepEnds: Ends = ends) => {
    setBusy(true)
    try {
      const r = await api.post<{ made: { data: MZFull } }>(`${V1}/generate`, { kind: 'maze', w, h, seed: Math.floor(Math.random() * 2 ** 31) + 1 })
      const md = r.made.data
      const [start, end] = endsFor(keepEnds, md.w, md.h)
      onChange({ ...md, theme: m.theme, start, end, path: undefined })
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  useEffect(() => { if (!has) make(16, 16) }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const setEnds = (k: Ends) => { if (!has) return; const [start, end] = endsFor(k, m.w, m.h); onChange({ ...m, start, end, path: undefined }) }
  const s = has ? sizeFor(bw, m.w, 30) : 0
  return (
    <div className="mz-edit" ref={box}>
      <div className="mz-field"><span className="muted small">Size</span>
        <Segmented size="sm" label="Size" value={own ? 0 : preset?.v ?? 0} onChange={v => { if (v === 0) { setOwn(true); return } setOwn(false); make(v, v) }}
          options={[...PRESETS.map(p => ({ value: p.v, label: `${p.label} ${p.v}` })), { value: 0, label: 'Your own' }]} />
      </div>
      {own && <div className="ed-toolbar">
        <span className="muted small">Across</span>
        <Select ariaLabel="Squares across" value={String(m.w || 16)} onChange={v => make(Number(v), m.h || 16)} options={DIMS} />
        <span className="muted small">Down</span>
        <Select ariaLabel="Squares down" value={String(m.h || 16)} onChange={v => make(m.w || 16, Number(v))} options={DIMS} />
      </div>}
      <div className="mz-field"><span className="muted small">Start and end</span>
        <Segmented size="sm" label="Start and end" value={ends} onChange={setEnds} options={[{ value: 'corners', label: 'Opposite corners' }, { value: 'sides', label: 'Left to right' }, { value: 'top', label: 'Top to bottom' }]} />
      </div>
      <div className="row wrap">
        <button type="button" className="btn primary" onClick={() => make(m.w || 16, m.h || 16)} disabled={busy}>{busy ? <Spinner /> : <Icon name="shuffle" size={14} />}New maze</button>
        <label className="row mz-switch"><Switch size="sm" checked={showWay} onChange={setShowWay} label="Show the way through" /><span className="small">Show the way through</span></label>
      </div>
      <div className="mz-preview">
        {has ? <MazeSVG m={m} s={s} path={showWay ? way : undefined} head={false} className="preview" /> : <div className="mz-wait"><Spinner /> Making a maze…</div>}
      </div>
      {has && <p className="muted small mz-count">{m.w} × {m.h} squares{showWay && way.length ? ` · the way through is ${way.length - 1} steps` : ''}</p>}
    </div>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as MZPublic
  const [box, bw] = useWidth(640)
  if (answer) {
    const a = answer as MZFull
    const s = clamp(Math.floor(Math.min(bw, 320) / a.w), 4, 14)
    const p = a.path?.length ? a.path : solve(a)
    return <div className="mz-print answer" ref={box}><MazeSVG m={a} s={s} path={p} head={false} print className="print" /></div>
  }
  const s = clamp(Math.floor((Math.min(bw, 700) - 10) / d.w), 8, large ? 48 : 36)
  return (
    <div className={`mz-print ${large ? 'large' : ''}`} ref={box}>
      <MazeSVG m={d} s={s} head={false} print className="print" />
      <p className="mz-print-rule">Find the way from ● to ★.</p>
    </div>
  )
}

export const Maze: KindModule = { Play, Edit, Print }
