// Jigsaws: play (drag the pieces onto the board, they snap in place), make
// (a picture, how many pieces, the shape of the cut), print (the picture with the cut lines).
import '../../styles/kind-jigsaw.css'
import React, { memo, useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import { V1 } from '../../lib/api'
import { Icon } from '../icons'
import { PhotoPick } from '../PhotoPick'
import { Segmented, Spinner } from '../ui'
import { useWidth } from './Crossword'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

interface JData { photo_id: number; cols: number; rows: number; shape: 'classic' | 'square'; seed: number }
interface Pos { x: number; y: number; ok: boolean }

// ---- the cut ----

/** A small seeded random (the same seed always cuts the same way). */
function rand(seed: number) {
  let a = (seed >>> 0) || 0x9e3779b9
  return () => {
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

interface Edge { s: number; j: [number, number, number, number, number] }
interface Cut { h: (Edge | null)[][]; v: (Edge | null)[][] }

/** Which way each tab points, and a little wobble so no two are alike. */
function makeCut(cols: number, rows: number, seed: number, shape: string): Cut {
  const r = rand(seed)
  const jit = () => (r() * 2 - 1) * 0.04
  const edge = (): Edge | null => shape === 'square' ? null : { s: r() < 0.5 ? 1 : -1, j: [jit(), jit(), jit(), jit(), jit()] }
  // h[r][c]: the line above row r (borders are straight); v[r][c]: the line left of column c
  const h = Array.from({ length: rows + 1 }, (_, y) => Array.from({ length: cols }, () => y === 0 || y === rows ? null : edge()))
  const v = Array.from({ length: rows }, () => Array.from({ length: cols + 1 }, (_, x) => x === 0 || x === cols ? null : edge()))
  return { h, v }
}

type Pt = [number, number]
/** The ten points of a tab, along a line of length 1 (u along, w out). */
function tabUW(e: Edge): Pt[] {
  const t = 0.1
  const [a, b, c, d, f] = e.j
  return [[0, 0], [0.2, a], [0.5 + b + d, -t + c], [0.5 - t + b, t + c], [0.5 - 2 * t + b - d, 3 * t + c],
    [0.5 + 2 * t + b - d, 3 * t + c], [0.5 + t + b, t + c], [0.5 + b + d, -t + c], [0.8, f], [1, 0]]
}
/** The line above row y, column x, left to right (null = straight). */
function hPts(cut: Cut, y: number, x: number, pw: number, ph: number): Pt[] | null {
  const e = cut.h[y][x]
  if (!e) return null
  return tabUW(e).map(([u, w]) => [(x + u) * pw, y * ph + w * e.s * pw])
}
/** The line left of column x, row y, top to bottom. */
function vPts(cut: Cut, y: number, x: number, pw: number, ph: number): Pt[] | null {
  const e = cut.v[y][x]
  if (!e) return null
  return tabUW(e).map(([u, w]) => [x * pw + w * e.s * ph, (y + u) * ph])
}
const f1 = (v: number) => Math.round(v * 10) / 10

/** One piece's outline, in its own box (its square starts at pad, pad). */
function piecePath(cut: Cut, y: number, x: number, pw: number, ph: number, pad: number) {
  const ox = x * pw - pad, oy = y * ph - pad
  const P = (p: Pt) => `${f1(p[0] - ox)},${f1(p[1] - oy)}`
  let d = `M${P([x * pw, y * ph])}`
  const run = (pts: Pt[] | null, end: Pt, back = false) => {
    if (!pts) { d += `L${P(end)}`; return }
    const q = back ? [...pts].reverse() : pts
    d += `C${P(q[1])} ${P(q[2])} ${P(q[3])}C${P(q[4])} ${P(q[5])} ${P(q[6])}C${P(q[7])} ${P(q[8])} ${P(q[9])}`
  }
  run(hPts(cut, y, x, pw, ph), [(x + 1) * pw, y * ph])
  run(vPts(cut, y, x + 1, pw, ph), [(x + 1) * pw, (y + 1) * ph])
  run(hPts(cut, y + 1, x, pw, ph), [x * pw, (y + 1) * ph], true)
  run(vPts(cut, y, x, pw, ph), [x * pw, y * ph], true)
  return d + 'Z'
}

/** Every cut line of the picture, as one path (for the preview and print). */
function cutLines(cut: Cut, cols: number, rows: number, pw: number, ph: number) {
  const P = (p: Pt) => `${f1(p[0])},${f1(p[1])}`
  let d = ''
  for (let y = 1; y < rows; y++) for (let x = 0; x < cols; x++) {
    const p = hPts(cut, y, x, pw, ph)
    d += p ? `M${P(p[0])}C${P(p[1])} ${P(p[2])} ${P(p[3])}C${P(p[4])} ${P(p[5])} ${P(p[6])}C${P(p[7])} ${P(p[8])} ${P(p[9])}` : `M${P([x * pw, y * ph])}L${P([(x + 1) * pw, y * ph])}`
  }
  for (let y = 0; y < rows; y++) for (let x = 1; x < cols; x++) {
    const p = vPts(cut, y, x, pw, ph)
    d += p ? `M${P(p[0])}C${P(p[1])} ${P(p[2])} ${P(p[3])}C${P(p[4])} ${P(p[5])} ${P(p[6])}C${P(p[7])} ${P(p[8])} ${P(p[9])}` : `M${P([x * pw, y * ph])}L${P([x * pw, (y + 1) * ph])}`
  }
  return d
}

/** The picture's width ÷ height once it has loaded (−1 = it could not be loaded). */
function useRatio(url: string) {
  const [ratio, setRatio] = useState<number | null>(null)
  useEffect(() => {
    setRatio(null)
    if (!url) return
    const im = new Image()
    im.onload = () => setRatio(im.naturalWidth && im.naturalHeight ? im.naturalWidth / im.naturalHeight : 1)
    im.onerror = () => setRatio(-1)
    im.src = url
    return () => { im.onload = null; im.onerror = null }
  }, [url])
  return ratio
}

function useViewportHeight() {
  const [h, setH] = useState(() => typeof window === 'undefined' ? 800 : window.innerHeight)
  useEffect(() => {
    const on = () => setH(window.innerHeight)
    window.addEventListener('resize', on)
    return () => window.removeEventListener('resize', on)
  }, [])
  return h
}

// ---- play ----

interface Lay { cw: number; ah: number; bx: number; by: number; bw: number; bh: number; pw: number; ph: number; pad: number; zones: [number, number, number, number][]; wide: boolean }

/** Where the board goes and where loose pieces lie: below it on a phone, around it on a big screen. */
function layoutFor(cw: number, vh: number, ratio: number, cols: number, rows: number): Lay {
  const wide = cw >= 720
  const gap = 14
  if (wide) {
    const bw = Math.max(200, Math.min(cw * 0.58, vh * 0.62 * ratio))
    const bh = bw / ratio, pw = bw / cols, ph = bh / rows, pad = Math.max(pw, ph) * 0.36
    const bx = (cw - bw) / 2, by = Math.max(8, pad * 0.6)
    const trayH = bh * 0.5 + ph
    const ah = by + bh + gap + trayH
    const side = bx - gap
    const zones: [number, number, number, number][] = [[0, 0, side, ah], [bx + bw + gap, 0, side, ah], [bx, by + bh + gap, bw, trayH]]
    return { cw, ah, bx, by, bw, bh, pw, ph, pad, zones, wide }
  }
  const bw = Math.max(160, Math.min(cw, vh * 0.5 * ratio))
  const bh = bw / ratio, pw = bw / cols, ph = bh / rows, pad = Math.max(pw, ph) * 0.36
  const bx = (cw - bw) / 2, by = 0
  const trayH = Math.max(bh * 1.25, ph * 3) + ph * 0.5
  return { cw, ah: bh + gap + trayH, bx, by, bw, bh, pw, ph, pad, zones: [[0, bh + gap, cw, trayH]], wide }
}

/** Spread the pieces out over the loose areas, in a loose grid with a wobble. */
function scatter(n: number, keep: Pos[] | null, L: Lay, seed: number): Pos[] {
  const r = rand(seed)
  const loose = Array.from({ length: n }, (_, i) => i).filter(i => !keep?.[i]?.ok)
  for (let i = loose.length - 1; i > 0; i--) { const j = Math.floor(r() * (i + 1)); [loose[i], loose[j]] = [loose[j], loose[i]] }
  const areas = L.zones.map(z => Math.max(0, z[2] - L.pw * 0.2) * Math.max(0, z[3] - L.ph * 0.2))
  const total = areas.reduce((a, b) => a + b, 0) || 1
  const out: Pos[] = Array.from({ length: n }, (_, i) => keep?.[i]?.ok ? keep[i] : { x: 0, y: 0, ok: false })
  let at = 0
  L.zones.forEach((z, zi) => {
    const count = zi === L.zones.length - 1 ? loose.length - at : Math.round(loose.length * areas[zi] / total)
    if (count <= 0) return
    const [zx, zy, zw, zh] = z
    const cols = Math.max(1, Math.round(Math.sqrt(count * zw / Math.max(1, zh))))
    const rowsN = Math.ceil(count / cols)
    const sw = zw / cols, sh = zh / rowsN
    for (let k = 0; k < count && at < loose.length; k++, at++) {
      const cx = k % cols, cy = Math.floor(k / cols)
      const fx = zx + cx * sw + Math.max(0, sw - L.pw) * r()
      const fy = zy + cy * sh + Math.max(0, sh - L.ph) * r()
      const px = Math.min(Math.max(fx, zx), zx + Math.max(0, zw - L.pw))
      const py = Math.min(Math.max(fy, zy), zy + Math.max(0, zh - L.ph))
      out[loose[at]] = { x: (px - L.bx) / L.bw, y: (py - L.by) / L.bh, ok: false }
    }
  })
  return out
}

interface PieceProps {
  i: number; d: string; col: number; row: number; L: Lay; href: string; clip: string
  x: number; y: number; z: number; ok: boolean; locked: boolean
  onDown: (e: React.PointerEvent<HTMLDivElement>) => void; onKey: (e: React.KeyboardEvent<HTMLDivElement>) => void
  setRef: (i: number, el: HTMLDivElement | null) => void
}

const Piece = memo(function Piece({ i, d, col, row, L, href, clip, x, y, z, ok, locked, onDown, onKey, setRef }: PieceProps) {
  const w = L.pw + 2 * L.pad, h = L.ph + 2 * L.pad
  const id = `${clip}-${i}`
  return (
    <div ref={el => setRef(i, el)} className={`jg-piece ${ok ? 'ok' : ''}`} data-i={i}
      style={{ width: w, height: h, transform: `translate3d(${x - L.pad}px, ${y - L.pad}px, 0)`, zIndex: z }}
      tabIndex={locked || ok ? -1 : 0} role={locked || ok ? undefined : 'button'} aria-label={ok ? undefined : `Piece ${i + 1}. Drag it to its place, or use the arrow keys.`}
      onPointerDown={onDown} onKeyDown={onKey}>
      <svg width={w} height={h} aria-hidden="true">
        <defs><clipPath id={id}><path d={d} /></clipPath></defs>
        <image href={href} x={L.pad - col * L.pw} y={L.pad - row * L.ph} width={L.bw} height={L.bh} clipPath={`url(#${id})`} />
        <path d={d} className="jg-edge" />
      </svg>
    </div>
  )
})

function Play({ view, progress, done, onProgress, check, reveal, photoURL, setActions }: KindPlayProps) {
  const d = view.data as JData
  const cols = Math.max(1, d.cols || 1), rows = Math.max(1, d.rows || 1), n = cols * rows
  const href = d.photo_id ? photoURL(d.photo_id) : ''
  const ratio = useRatio(href)
  const [box, cw] = useWidth(360)
  const vh = useViewportHeight()
  const L = useMemo(() => ratio && ratio > 0 ? layoutFor(cw, vh, ratio, cols, rows) : null, [cw, vh, ratio, cols, rows])
  const cut = useMemo(() => makeCut(cols, rows, Number(d.seed) || 1, d.shape), [cols, rows, d.seed, d.shape])
  const paths = useMemo(() => L ? Array.from({ length: n }, (_, i) => piecePath(cut, Math.floor(i / cols), i % cols, L.pw, L.ph, L.pad)) : [], [cut, L?.pw, L?.ph, L?.pad, n, cols]) // eslint-disable-line react-hooks/exhaustive-deps
  const [pos, setPos] = useState<Pos[] | null>(() => Array.isArray(progress?.pos) && progress.pos.length === n
    ? progress.pos.map((p: any) => ({ x: Number(p?.x) || 0, y: Number(p?.y) || 0, ok: !!p?.ok })) : null)
  const [z, setZ] = useState<number[]>(() => Array.from({ length: n }, (_, i) => i))
  const [ghost, setGhost] = useState(false)
  const clip = 'jg' + useId().replace(/[^a-zA-Z0-9_-]/g, '')
  const area = useRef<HTMLDivElement>(null)
  const els = useRef<(HTMLDivElement | null)[]>([])
  const posRef = useRef(pos), LRef = useRef(L), zTop = useRef(n), doneRef = useRef(done)
  const checkRef = useRef(check), revealRef = useRef(reveal), progressRef = useRef(onProgress)
  posRef.current = pos; LRef.current = L; doneRef.current = done; checkRef.current = check; revealRef.current = reveal; progressRef.current = onProgress
  const drag = useRef<{ i: number; offX: number; offY: number; x: number; y: number; id: number } | null>(null)

  // first time (or the layout switched between phone and big screen): spread the loose pieces out
  const lastWide = useRef<boolean | null>(null)
  useEffect(() => {
    if (!L) return
    if (!posRef.current) { setPos(scatter(n, null, L, (Number(d.seed) || 1) + 7)); lastWide.current = L.wide; return }
    if (lastWide.current !== null && lastWide.current !== L.wide) setPos(scatter(n, posRef.current, L, (Number(d.seed) || 1) + 11))
    lastWide.current = L.wide
  }, [L, n, d.seed])

  const setRef = useCallback((i: number, el: HTMLDivElement | null) => { els.current[i] = el }, [])

  /** Where a piece sits, in pixels of the play area (loose ones kept inside it). */
  const pxOf = (p: Pos, i: number, Lx: Lay) => {
    if (p.ok) return { x: Lx.bx + (i % cols) * Lx.pw, y: Lx.by + Math.floor(i / cols) * Lx.ph }
    const x = Lx.bx + p.x * Lx.bw, y = Lx.by + p.y * Lx.bh
    return { x: Math.min(Math.max(x, -Lx.pad * 0.5), Lx.cw - Lx.pw + Lx.pad * 0.5), y: Math.min(Math.max(y, 0), Lx.ah - Lx.ph) }
  }

  const finish = (next: Pos[]) => {
    posRef.current = next
    setPos(next)
    progressRef.current({ pos: next })
    if (next.every(p => p.ok)) checkRef.current({ done: true }, true)
  }
  /** Drop piece i at (x, y) pixels: snap it in when it is close to its place. */
  const drop = (i: number, x: number, y: number) => {
    const Lx = LRef.current, cur = posRef.current
    if (!Lx || !cur) return
    const tx = Lx.bx + (i % cols) * Lx.pw, ty = Lx.by + Math.floor(i / cols) * Lx.ph
    const snap = Math.max(10, Math.min(Lx.pw, Lx.ph) * 0.3)
    const ok = Math.hypot(x - tx, y - ty) <= snap
    const next = [...cur]
    next[i] = ok ? { x: (tx - Lx.bx) / Lx.bw, y: (ty - Lx.by) / Lx.bh, ok: true } : { x: (x - Lx.bx) / Lx.bw, y: (y - Lx.by) / Lx.bh, ok: false }
    finish(next)
  }
  const raise = (i: number) => {
    zTop.current += 1
    const top = zTop.current
    setZ(zs => { const nz = [...zs]; nz[i] = top; return nz })
  }

  const onMove = useCallback((e: PointerEvent) => {
    const g = drag.current, a = area.current, Lx = LRef.current
    if (!g || !a || !Lx || e.pointerId !== g.id) return
    e.preventDefault()
    const r = a.getBoundingClientRect()
    g.x = Math.min(Math.max(e.clientX - r.left - g.offX, -Lx.pad * 0.5), Lx.cw - Lx.pw + Lx.pad * 0.5)
    g.y = Math.min(Math.max(e.clientY - r.top - g.offY, 0), Lx.ah - Lx.ph)
    const el = els.current[g.i]
    if (el) el.style.transform = `translate3d(${g.x - Lx.pad}px, ${g.y - Lx.pad}px, 0)`
  }, [])
  const onUp = useCallback((e: PointerEvent) => {
    const g = drag.current
    if (!g || e.pointerId !== g.id) return
    drag.current = null
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    window.removeEventListener('pointercancel', onUp)
    els.current[g.i]?.classList.remove('drag')
    drop(g.i, g.x, g.y)
  }, [onMove]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => () => {
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    window.removeEventListener('pointercancel', onUp)
  }, [onMove, onUp])

  const onDown = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    if (doneRef.current || drag.current || (e.pointerType === 'mouse' && e.button !== 0)) return
    const i = Number(e.currentTarget.dataset.i)
    const p = posRef.current?.[i], a = area.current, Lx = LRef.current
    if (!p || p.ok || !a || !Lx) return
    e.preventDefault()
    const r = a.getBoundingClientRect()
    const at = pxOf(p, i, Lx)
    drag.current = { i, offX: e.clientX - r.left - at.x, offY: e.clientY - r.top - at.y, x: at.x, y: at.y, id: e.pointerId }
    const el = els.current[i]
    if (el) { el.classList.add('drag'); el.style.zIndex = '100000' }
    raise(i)
    window.addEventListener('pointermove', onMove, { passive: false })
    window.addEventListener('pointerup', onUp)
    window.addEventListener('pointercancel', onUp)
  }, [onMove, onUp]) // eslint-disable-line react-hooks/exhaustive-deps

  /** Keyboard: arrows move the piece (a quarter of a piece, or a whole one with Shift). */
  const onKey = useCallback((e: React.KeyboardEvent<HTMLDivElement>) => {
    const i = Number(e.currentTarget.dataset.i)
    const p = posRef.current?.[i], Lx = LRef.current
    if (doneRef.current || !p || p.ok || !Lx) return
    const step = e.shiftKey ? 1 : 0.25
    const dx = e.key === 'ArrowLeft' ? -step : e.key === 'ArrowRight' ? step : 0
    const dy = e.key === 'ArrowUp' ? -step : e.key === 'ArrowDown' ? step : 0
    if (!dx && !dy) return
    e.preventDefault()
    const at = pxOf(p, i, Lx)
    drop(i, at.x + dx * Lx.pw, at.y + dy * Lx.ph)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    setActions({
      reveal: async () => {
        const cur = posRef.current
        if (!cur || doneRef.current) return
        const i = cur.findIndex(p => !p.ok)
        if (i < 0) return
        const next = [...cur]
        const Lx = LRef.current
        next[i] = Lx ? { x: (i % cols) * Lx.pw / Lx.bw, y: Math.floor(i / cols) * Lx.ph / Lx.bh, ok: true } : { ...cur[i], ok: true }
        finish(next)
        revealRef.current(i).catch(() => null) // only counts the hint
      },
      revealLabel: 'Place a piece',
      clear: () => {
        const Lx = LRef.current
        if (!Lx || doneRef.current) return
        finish(scatter(n, null, Lx, Math.floor(Math.random() * 1e9) + 1))
      },
    })
  }, [n, cols]) // eslint-disable-line react-hooks/exhaustive-deps

  if (!d.photo_id) return <div className="callout info"><Icon name="image" size={15} /><div>This jigsaw has no picture yet.</div></div>
  if (ratio === -1) return <div className="callout warn"><Icon name="alert" size={15} /><div>The picture could not be loaded. Check your connection and open the puzzle again.</div></div>
  const placed = pos ? pos.filter(p => p.ok).length : 0
  return (
    <div className="jg-play" ref={box}>
      <div className="jg-bar">
        <button type="button" className={`btn sm ${ghost ? 'on' : ''}`} aria-pressed={ghost} onClick={() => setGhost(g => !g)}>
          <Icon name={ghost ? 'eye-off' : 'eye'} size={14} />{ghost ? 'Hide the picture' : 'Show the picture'}
        </button>
        <span className="spacer" />
        <span className="muted small tnum">{done ? n : placed} of {n} in place</span>
      </div>
      {!L || !pos ? <div className="jg-loading"><Spinner /> Cutting the pieces…</div> : (
        <div className={`jg-area ${done ? 'done' : ''}`} ref={area} style={{ height: L.ah }}>
          <div className={`jg-board ${ghost ? 'ghost' : ''}`} style={{ left: L.bx, top: L.by, width: L.bw, height: L.bh }}>
            <img src={href} alt="" draggable={false} />
          </div>
          {paths.map((p, i) => {
            const at = pxOf(done ? { x: 0, y: 0, ok: true } : pos[i], i, L)
            const ok = done || pos[i].ok
            return <Piece key={i} i={i} d={p} col={i % cols} row={Math.floor(i / cols)} L={L} href={href} clip={clip}
              x={at.x} y={at.y} z={ok ? 1 : 10 + z[i]} ok={ok} locked={done} onDown={onDown} onKey={onKey} setRef={setRef} />
          })}
        </div>
      )}
      {!done && <p className="muted small jg-help">Drag a piece onto the board — it clicks into place when it is close.</p>}
    </div>
  )
}

// ---- the picture with its cut lines (editor preview and print) ----

function CutPicture({ url, width, ratio, data, lines = true, print }: { url: string; width: number; ratio: number; data: JData; lines?: boolean; print?: boolean }) {
  const cols = Math.max(1, data.cols), rows = Math.max(1, data.rows)
  const w = Math.round(width), h = Math.round(width / ratio)
  const d = useMemo(() => cutLines(makeCut(cols, rows, Number(data.seed) || 1, data.shape), cols, rows, w / cols, h / rows), [cols, rows, data.seed, data.shape, w, h])
  return (
    <div className={`jg-cut ${print ? 'print' : ''}`} style={{ width: w, height: h }}>
      <img src={url} alt="" width={w} height={h} draggable={false} />
      {lines && <svg width={w} height={h} aria-hidden="true">
        <path d={d} className="jg-cut-shadow" />
        <path d={d} className="jg-cut-line" />
        <rect x={0.5} y={0.5} width={w - 1} height={h - 1} className="jg-cut-line" />
      </svg>}
    </div>
  )
}

// ---- make ----

const COUNTS = [12, 20, 35, 63, 100]
/** Columns × rows near a number of pieces, with pieces close to square for this picture. */
function gridFor(count: number, ratio: number): [number, number] {
  let best: [number, number] = [4, 3], score = Infinity
  for (let c = 2; c <= 30; c++) for (let r = 2; r <= 30; r++) {
    const s = Math.abs(c * r - count) / count * 3 + Math.abs(Math.log((c / r) / ratio))
    if (s < score) { score = s; best = [c, r] }
  }
  return best
}
const nearest = (n: number) => COUNTS.reduce((a, b) => Math.abs(b - n) < Math.abs(a - n) ? b : a, COUNTS[0])

function Edit({ data, onChange }: KindEditProps) {
  const j: JData = { photo_id: Number(data?.photo_id) || 0, cols: Number(data?.cols) || 4, rows: Number(data?.rows) || 3, shape: data?.shape === 'square' ? 'square' : 'classic', seed: Number(data?.seed) || 1 }
  const url = j.photo_id ? `${V1}/photos/${j.photo_id}` : ''
  const ratio = useRatio(url)
  const [box, bw] = useWidth(560)
  const set = (patch: Partial<JData>) => onChange({ ...j, ...patch })
  const count = nearest(j.cols * j.rows)
  const setCount = (c: number) => { const [cols, rows] = gridFor(c, ratio && ratio > 0 ? ratio : 4 / 3); set({ cols, rows }) }
  // a new picture of another shape: keep about the same number of pieces
  const lastRatio = useRef<number | null>(null)
  useEffect(() => {
    if (!ratio || ratio < 0) return
    if (lastRatio.current !== null && Math.abs(lastRatio.current - ratio) > 0.01) {
      const [cols, rows] = gridFor(count, ratio)
      if (cols !== j.cols || rows !== j.rows) onChange({ ...j, cols, rows })
    }
    lastRatio.current = ratio
  }, [ratio]) // eslint-disable-line react-hooks/exhaustive-deps
  const width = Math.min(bw, 560, ratio && ratio > 0 && ratio < 1 ? 420 * ratio * 1.4 : 560)
  return (
    <div className="jg-edit" ref={box}>
      <section className="jg-ed-sec">
        <h4>The picture</h4>
        <PhotoPick kind="puzzle" label={j.photo_id ? 'Choose another picture' : 'Choose a picture'} onPicked={p => {
          const patch: Partial<JData> = { photo_id: p.id }
          if (p.width && p.height) { const [cols, rows] = gridFor(count, p.width / p.height); patch.cols = cols; patch.rows = rows }
          set(patch)
        }} />
        {!j.photo_id && <p className="muted small">Any photo works — one with lots of different colours and shapes is the most fun.</p>}
      </section>

      <section className="jg-ed-sec">
        <div className="row wrap jg-ed-head">
          <h4 className="grow">Pieces</h4>
          <span className="muted small tnum">{j.cols} × {j.rows} = {j.cols * j.rows} pieces</span>
        </div>
        <Segmented size="sm" label="How many pieces" value={count} onChange={setCount} options={COUNTS.map(c => ({ value: c, label: String(c) }))} />
        <div className="row wrap jg-ed-head">
          <Segmented size="sm" label="Shape of the pieces" value={j.shape} onChange={v => set({ shape: v })} options={[{ value: 'classic', label: 'Jigsaw shapes' }, { value: 'square', label: 'Squares' }]} />
          <button type="button" className="btn sm" onClick={() => set({ seed: Math.floor(Math.random() * 2147483646) + 1 })} disabled={j.shape === 'square'} title="Tabs and holes in different places">
            <Icon name="shuffle" size={14} />Cut it differently
          </button>
        </div>
      </section>

      {j.photo_id > 0 && <section className="jg-ed-sec">
        <h4>How it is cut</h4>
        <div className="jg-preview">
          {ratio === null ? <div className="jg-loading"><Spinner /> Loading the picture…</div>
            : ratio < 0 ? <div className="callout warn"><Icon name="alert" size={15} /><div>The picture could not be loaded.</div></div>
              : <CutPicture url={url} width={width} ratio={ratio} data={j} />}
        </div>
      </section>}
    </div>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as JData
  const url = d.photo_id ? `${V1}/photos/${d.photo_id}` : ''
  const ratio = useRatio(url)
  if (!url || !ratio || ratio < 0) return null
  const max = answer ? 260 : large ? 700 : 640
  const width = ratio >= 1 ? max : Math.round(max * Math.max(0.55, ratio))
  return <CutPicture url={url} width={width} ratio={ratio} data={d} lines={!answer} print />
}

export const Jigsaw: KindModule = { Play, Edit, Print }
