// Pictures: shrunk in the browser before they leave the phone (a 1600 px
// JPEG and a 480 px thumbnail), and the main colours read from the middle
// of the picture (the background at the edges is left out).
import { ApiError, V1 } from './api'

export interface Shrunk { big: Blob; thumb: Blob; width: number; height: number; colors: string[]; url: string }

function load(file: Blob): Promise<HTMLImageElement> {
  return new Promise((res, rej) => {
    const url = URL.createObjectURL(file)
    const img = new Image()
    img.onload = () => res(img)
    img.onerror = () => rej(new Error('That file is not a picture this browser can read'))
    img.src = url
  })
}

function draw(img: HTMLImageElement, max: number): HTMLCanvasElement {
  const scale = Math.min(1, max / Math.max(img.naturalWidth, img.naturalHeight))
  const c = document.createElement('canvas')
  c.width = Math.max(1, Math.round(img.naturalWidth * scale))
  c.height = Math.max(1, Math.round(img.naturalHeight * scale))
  const g = c.getContext('2d')!
  g.fillStyle = '#ffffff'
  g.fillRect(0, 0, c.width, c.height)
  g.drawImage(img, 0, 0, c.width, c.height)
  return c
}

const blob = (c: HTMLCanvasElement, q: number) => new Promise<Blob>((res, rej) => c.toBlob(b => b ? res(b) : rej(new Error('could not shrink the picture')), 'image/jpeg', q))

const hex = (r: number, g: number, b: number) => '#' + [r, g, b].map(v => Math.round(v).toString(16).padStart(2, '0')).join('')

/** The main colours of the middle of a picture (k-means on a small copy). */
export function paletteOf(c: HTMLCanvasElement, k = 4): string[] {
  const s = document.createElement('canvas')
  s.width = 64; s.height = Math.max(1, Math.round(64 * c.height / c.width))
  const g = s.getContext('2d')!
  g.drawImage(c, 0, 0, s.width, s.height)
  const d = g.getImageData(0, 0, s.width, s.height).data
  const px: [number, number, number][] = []
  // the edges are usually the floor, the bed or the wall
  const corner = (x: number, y: number) => { const i = (y * s.width + x) * 4; return [d[i], d[i + 1], d[i + 2]] }
  const bg = [corner(0, 0), corner(s.width - 1, 0), corner(0, s.height - 1), corner(s.width - 1, s.height - 1)]
  const near = (a: number[], b: number[]) => Math.abs(a[0] - b[0]) + Math.abs(a[1] - b[1]) + Math.abs(a[2] - b[2]) < 40
  for (let y = Math.floor(s.height * 0.12); y < s.height * 0.88; y++) {
    for (let x = Math.floor(s.width * 0.15); x < s.width * 0.85; x++) {
      const i = (y * s.width + x) * 4
      const p: [number, number, number] = [d[i], d[i + 1], d[i + 2]]
      if (bg.some(b => near(p, b))) continue
      px.push(p)
    }
  }
  if (px.length < 20) return []
  let cent = Array.from({ length: k }, (_, i) => px[Math.floor((i + 0.5) * px.length / k)].slice() as number[])
  let groups: number[][][] = []
  for (let it = 0; it < 8; it++) {
    groups = cent.map(() => [])
    for (const p of px) {
      let bi = 0, bd = 1e9
      cent.forEach((c, i) => { const dd = (p[0] - c[0]) ** 2 + (p[1] - c[1]) ** 2 + (p[2] - c[2]) ** 2; if (dd < bd) { bd = dd; bi = i } })
      groups[bi].push(p)
    }
    cent = groups.map((gr, i) => gr.length ? [0, 1, 2].map(j => gr.reduce((a, p) => a + p[j], 0) / gr.length) : cent[i])
  }
  return cent.map((c, i) => ({ c, n: groups[i].length })).filter(x => x.n > px.length * 0.08).sort((a, b) => b.n - a.n).map(x => hex(x.c[0], x.c[1], x.c[2]))
}

/** Shrinks a picture for the upload and reads its main colours. */
export async function shrink(file: Blob, max = 1600): Promise<Shrunk> {
  const img = await load(file)
  const big = draw(img, max)
  const thumb = draw(img, 480)
  return { big: await blob(big, 0.85), thumb: await blob(thumb, 0.8), width: big.width, height: big.height, colors: paletteOf(big), url: img.src }
}

/** Uploads a shrunk picture; the answer has its id and addresses. */
export async function uploadPhoto(s: Shrunk, kind: string = 'portrait'): Promise<{ id: number; url: string; thumb_url: string; width: number; height: number }> {
  const fd = new FormData()
  fd.append('file', s.big, 'photo.jpg')
  fd.append('thumb', s.thumb, 'thumb.jpg')
  fd.append('kind', kind)
  fd.append('width', String(s.width))
  fd.append('height', String(s.height))
  const res = await fetch(`${V1}/photos`, { method: 'POST', body: fd, credentials: 'same-origin' })
  const j = await res.json().catch(() => ({}))
  if (!res.ok) throw new ApiError(res.status, j.error || res.statusText, j)
  return j
}

/** The colour under a point of a picture shown in an <img>. */
export function colorAt(img: HTMLImageElement, x: number, y: number): string {
  const c = document.createElement('canvas')
  c.width = 1; c.height = 1
  const g = c.getContext('2d')!
  const sx = x / img.clientWidth * img.naturalWidth, sy = y / img.clientHeight * img.naturalHeight
  g.drawImage(img, sx - 2, sy - 2, 5, 5, 0, 0, 1, 1)
  const d = g.getImageData(0, 0, 1, 1).data
  return hex(d[0], d[1], d[2])
}
