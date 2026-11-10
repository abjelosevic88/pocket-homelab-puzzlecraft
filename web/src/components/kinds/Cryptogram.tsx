// Secret codes (cryptograms): play (type a letter once, every box with that
// symbol follows; an alphabet strip that is also a keyboard), make (text,
// author, kind of code, letters given away, quotes from the library or from
// Pocket Bookshelf), print.
import '../../styles/kind-cryptogram.css'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import { letters } from '../../lib/puzzlecraft'
import { Icon } from '../icons'
import { Modal, Segmented, Spinner, Switch, useToast } from '../ui'
import { loadThemes } from './common'
import { useWidth } from './Crossword'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shared ----

type Cipher = 'random' | 'caesar' | 'numbers' | 'symbols'
type Token = { s: string; l?: string } | { t: string }
interface CGPublic { cipher: Cipher; tokens: Token[]; author_code?: string; hint?: string; symbols: number; language: string }
interface CGData { text: string; author?: string; cipher: Cipher; shift?: number; key: Record<string, string>; given: string[]; language: string }

const EN = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')
const BCS = ['A', 'B', 'C', 'Č', 'Ć', 'D', 'DŽ', 'Đ', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'LJ', 'M', 'N', 'NJ', 'O', 'P', 'R', 'S', 'Š', 'T', 'U', 'V', 'Z', 'Ž']
const alphabetOf = (lang: string) => lang === 'bcs' ? BCS : EN
const isSym = (t: Token): t is { s: string; l?: string } => 's' in t
const isLetterKey = (k: string) => k.length === 1 && /\p{L}|\p{N}/u.test(k)
const isDigits = (s: string) => /^\p{N}+$/u.test(s)

/** The symbols of a coded text, once each, in the order they first appear (what check and reveal count by). */
function symbolOrder(tokens: Token[]): string[] {
  const seen = new Set<string>(), out: string[] = []
  for (const t of tokens) if (isSym(t) && !seen.has(t.s)) { seen.add(t.s); out.push(t.s) }
  return out
}

/** Words of the text: runs of tokens between spaces, so lines only break between words. */
function wordsOf(tokens: Token[]): { i: number; t: Token }[][] {
  const out: { i: number; t: Token }[][] = []
  let cur: { i: number; t: Token }[] = []
  tokens.forEach((t, i) => {
    if (!isSym(t) && /^\s+$/u.test(t.t)) { if (cur.length) out.push(cur); cur = []; return }
    cur.push({ i, t })
  })
  if (cur.length) out.push(cur)
  return out
}

/** The coded text as the server makes it (for the maker's preview). */
function encodeText(c: CGData): Token[] {
  const out: Token[] = []
  const rs = Array.from((c.text || '').normalize('NFC').toUpperCase())
  const given = new Set(c.given || [])
  for (let i = 0; i < rs.length; i++) {
    let l = rs[i]
    if (c.language === 'bcs' && i + 1 < rs.length) {
      const p = l + rs[i + 1]
      if (p === 'LJ' || p === 'NJ' || p === 'DŽ') { l = p; i++ }
    }
    if (letters(l, c.language).length === 1) {
      const s = c.key?.[l] || l
      out.push(given.has(l) ? { s, l } : { s })
    } else out.push({ t: l })
  }
  return out
}

/** Split a coded name into its symbols (letters, pairs like LJ, or symbols). */
function splitCode(code: string, known: Set<string>): string[] {
  const rs = Array.from(code)
  const out: string[] = []
  for (let i = 0; i < rs.length; i++) {
    const two = rs[i] + (rs[i + 1] || '')
    if (i + 1 < rs.length && (known.has(two) || two === 'LJ' || two === 'NJ' || two === 'DŽ')) { out.push(two); i++ } else out.push(rs[i])
  }
  return out
}

interface TextProps {
  tokens: Token[]; letterOf: (sym: string) => string; size: number; cur?: number; curSym?: string
  locked?: (sym: string) => boolean; wrong?: Set<string>; right?: Set<string>; onPick?: (i: number) => void; numbers?: boolean; className?: string
}

/** The coded text as boxes: the symbol above, the letter below. */
function CodeText({ tokens, letterOf, size, cur, curSym, locked, wrong, right, onPick, numbers, className = '' }: TextProps) {
  const words = useMemo(() => wordsOf(tokens), [tokens])
  return (
    <div className={`cg-text ${numbers ? 'nums' : ''} ${className}`} style={{ ['--cg' as any]: `${size}px` }}>
      {words.map((w, k) => (
        <span key={k} className="cg-word">
          {w.map(({ i, t }) => {
            if (!isSym(t)) return <span key={i} className="cg-cell punct" aria-hidden="true"><span className="cg-sym"> </span><span className="cg-box">{t.t}</span></span>
            const l = letterOf(t.s)
            const cls = ['cg-cell', i === cur ? 'cur' : '', curSym === t.s && i !== cur ? 'same' : '', locked?.(t.s) ? 'given' : '', wrong?.has(t.s) ? 'wrong' : '', right?.has(t.s) ? 'right' : '', l ? 'filled' : ''].join(' ')
            return (
              <span key={i} className={cls} onClick={onPick ? () => onPick(i) : undefined} role={onPick ? 'button' : undefined}
                aria-label={onPick ? `Code ${t.s}${l ? `, letter ${l}` : ', empty'}` : undefined}>
                <span className="cg-sym">{t.s}</span>
                <span className="cg-box">{l}</span>
              </span>
            )
          })}
        </span>
      ))}
    </div>
  )
}

// ---- play ----

function Play({ view, progress, shown, done, showMistakes, onProgress, check, reveal, setActions }: KindPlayProps) {
  const d = view.data as CGPublic
  const tokens = d.tokens || []
  const order = useMemo(() => symbolOrder(tokens), [tokens])
  const alphabet = alphabetOf(d.language)
  // letters shown from the start: given away by the maker, and plain digits in a letter code
  const fixed = useMemo(() => {
    const m: Record<string, string> = {}
    for (const t of tokens) if (isSym(t) && t.l) m[t.s] = t.l
    if (d.cipher !== 'numbers') for (const s of order) if (isDigits(s) && !m[s]) m[s] = s
    return m
  }, [tokens, order, d.cipher])
  const [map, setMap] = useState<Record<string, string>>(() => ({ ...(progress?.map || {}) }))
  const [revealed, setRevealed] = useState<Set<string>>(() => new Set(shown.map(i => order[i]).filter(Boolean)))
  const [wrong, setWrong] = useState<Set<string>>(new Set())
  const [note, setNote] = useState('')
  const firstSym = tokens.findIndex(t => isSym(t) && !fixed[t.s])
  const [cur, setCur] = useState(firstSym >= 0 ? firstSym : 0)
  const lastTyped = useRef<{ sym: string; ch: string } | null>(null)
  const board = useRef<HTMLDivElement>(null)
  const [box, bw] = useWidth(600)
  const size = Math.max(30, Math.min(42, Math.floor(Math.min(bw, 760) / 11)))

  const locked = (s: string) => !!fixed[s] || revealed.has(s)
  const letterOf = (s: string) => fixed[s] || map[s] || ''
  const curTok = tokens[cur]
  const curSym = !done && curTok && isSym(curTok) ? curTok.s : undefined
  const full = (m: Record<string, string>) => ({ ...m, ...fixed })
  const usedBy = useMemo(() => {
    const u: Record<string, string> = {}
    for (const s of order) { const l = fixed[s] || map[s]; if (l) u[l] = s }
    return u
  }, [order, fixed, map])

  const mapRef = useRef(map)
  mapRef.current = map
  const update = (next: Record<string, string>, changed?: string) => {
    setMap(next)
    onProgress({ map: next })
    if (changed || showMistakes) setWrong(w => { if (showMistakes) return new Set(); const n = new Set(w); if (changed) n.delete(changed); return n })
    setNote('')
    if (order.every(s => fixed[s] || next[s])) {
      check({ map: full(next) }, true).then(r => {
        if (r && !r.solved) { const ws = new Set((r.wrong || []).map(i => order[i])); setWrong(ws); setNote(`Every box is filled, but ${ws.size === 1 ? 'one letter is' : `${ws.size} letters are`} not right yet — marked in red.`) }
      })
    }
  }
  const nextOpen = (from: number, m: Record<string, string>) => {
    for (let k = 1; k <= tokens.length; k++) {
      const j = (from + k) % tokens.length
      const t = tokens[j]
      if (isSym(t) && !fixed[t.s] && !revealed.has(t.s) && !m[t.s]) return j
    }
    return -1
  }
  const stepSym = (dir: 1 | -1) => {
    for (let k = 1; k < tokens.length; k++) {
      const j = cur + dir * k
      if (j < 0 || j >= tokens.length) return
      const t = tokens[j]
      if (isSym(t)) { setCur(j); return }
    }
  }
  const put = (sym: string, letter: string, m: Record<string, string>) => {
    const next = { ...m }
    for (const s of Object.keys(next)) if (next[s] === letter && s !== sym) delete next[s]
    next[sym] = letter
    return next
  }
  const type = (ch: string) => {
    if (done || !curSym) return
    const up = ch.toUpperCase()
    const lt = lastTyped.current
    // LJ, NJ, DŽ: the second letter joins the one just typed
    if (d.language === 'bcs' && lt && ((lt.ch === 'L' || lt.ch === 'N') && up === 'J' || lt.ch === 'D' && up === 'Ž') && map[lt.sym] === lt.ch) {
      lastTyped.current = null
      update(put(lt.sym, lt.ch + up, map), lt.sym)
      return
    }
    if (locked(curSym)) { const j = nextOpen(cur, map); if (j >= 0) setCur(j); return }
    const owner = usedBy[up]
    if (owner && owner !== curSym && locked(owner)) { setNote(`${up} is already given: it stands for ${owner}.`); return }
    const next = put(curSym, up, map)
    lastTyped.current = { sym: curSym, ch: up }
    update(next, curSym)
    const j = nextOpen(cur, next)
    if (j >= 0) setCur(j)
  }
  const erase = () => {
    if (done || !curSym) return
    lastTyped.current = null
    if (locked(curSym)) { stepSym(-1); return }
    if (map[curSym]) { const next = { ...map }; delete next[curSym]; update(next, curSym) } else stepSym(-1)
  }
  const onKey = (ev: React.KeyboardEvent) => {
    if (ev.metaKey || ev.ctrlKey || ev.altKey) return
    const k = ev.key
    if (k === 'ArrowRight' || k === 'ArrowDown') { ev.preventDefault(); stepSym(1) }
    else if (k === 'ArrowLeft' || k === 'ArrowUp') { ev.preventDefault(); stepSym(-1) }
    else if (k === 'Tab' || k === 'Enter') { ev.preventDefault(); const j = nextOpen(cur, map); if (j >= 0) setCur(j) }
    else if (k === 'Backspace' || k === 'Delete' || k === ' ') { ev.preventDefault(); erase() }
    else if (isLetterKey(k)) { ev.preventDefault(); type(k) }
  }
  const pick = (i: number) => { if (done) return; setCur(i); lastTyped.current = null; board.current?.focus({ preventScroll: true }) }

  useEffect(() => {
    setActions({
      check: async () => {
        const r = await check({ map: full(mapRef.current) })
        if (!r) return
        const ws = new Set((r.wrong || []).map(i => order[i]))
        setWrong(ws)
        setNote(ws.size ? `${ws.size === 1 ? 'One letter is' : `${ws.size} letters are`} not right yet — marked in red.` : r.filled ? 'Everything you have so far is right.' : 'Type a letter in a box to start.')
      },
      reveal: async () => {
        const m = mapRef.current
        const open = (s: string) => !locked(s) && (!m[s] || wrong.has(s))
        let s = curSym && open(curSym) ? curSym : order.find(open)
        if (!s) s = curSym && !locked(curSym) ? curSym : order.find(x => !locked(x))
        if (!s) return
        const v = await reveal(order.indexOf(s))
        if (v?.letter) {
          setRevealed(r => new Set([...r, v.symbol || s!]))
          update(put(v.symbol || s!, String(v.letter), m), v.symbol || s)
        }
      },
      revealLabel: 'Give a letter',
      clear: () => {
        const next: Record<string, string> = {}
        for (const s of revealed) if (mapRef.current[s]) next[s] = mapRef.current[s]
        setWrong(new Set()); setNote(''); setMap(next); onProgress({ map: next })
      },
    })
  }, [cur, revealed, wrong]) // eslint-disable-line react-hooks/exhaustive-deps

  const known = useMemo(() => new Set(order), [order])
  const authorSyms = useMemo(() => d.author_code && d.cipher !== 'numbers' ? splitCode(d.author_code, known) : [], [d.author_code, d.cipher, known])
  const filled = order.filter(s => letterOf(s)).length
  return (
    <div className="cg-play" ref={box}>
      {d.hint && <div className="callout info cg-hint"><Icon name="lightbulb" size={15} /><div>{d.hint}</div></div>}
      <div className="cg-board" ref={board} tabIndex={0} onKeyDown={onKey} aria-label="The secret message. Pick a box and type a letter.">
        <CodeText tokens={tokens} letterOf={letterOf} size={size} cur={done ? undefined : cur} curSym={curSym} locked={locked} wrong={wrong} onPick={done ? undefined : pick} numbers={d.cipher === 'numbers'} />
        {d.author_code && (
          <div className="cg-author">
            <span className="muted">—</span>
            {authorSyms.length > 0
              ? <span className="cg-word">{authorSyms.map((s, k) => { const l = letterOf(s); return <span key={k} className={`cg-cell small ${curSym === s ? 'same' : ''}`} onClick={!done && known.has(s) ? () => pick(tokens.findIndex(t => isSym(t) && t.s === s)) : undefined}><span className="cg-sym">{s}</span><span className="cg-box">{l}</span></span> })}</span>
              : <span className="cg-author-code">{d.author_code}</span>}
          </div>
        )}
      </div>
      <div className="cg-status" aria-live="polite">
        {note ? <span className={wrong.size ? 'bad' : ''}>{note}</span> : done ? <span className="ok"><Icon name="check-circle" size={14} />Cracked!</span>
          : curSym ? <span className="muted">Box <b className="cg-status-sym">{curSym}</b> — type a letter or tap one below. {filled} of {order.length} found.</span> : <span className="muted">Pick a box.</span>}
      </div>
      <div className={`cg-keys ${d.language === 'bcs' ? 'many' : ''}`} role="group" aria-label="Letters">
        {alphabet.map(l => {
          const owner = usedBy[l]
          return (
            <button key={l} type="button" className={`cg-key ${owner ? 'used' : ''} ${owner && locked(owner) ? 'fixed' : ''} ${owner && owner === curSym ? 'mine' : ''}`} disabled={done}
              onPointerDown={e => e.preventDefault()} onClick={() => { type(l); board.current?.focus({ preventScroll: true }) }}
              aria-label={owner ? `${l}, used for ${owner}` : l}>
              <span className="cg-key-l">{l}</span>
              <span className="cg-key-s">{owner || ''}</span>
            </button>
          )
        })}
        <button type="button" className="cg-key erase" disabled={done} onPointerDown={e => e.preventDefault()} onClick={() => { erase(); board.current?.focus({ preventScroll: true }) }} aria-label="Rub out">
          <Icon name="x" size={16} /><span className="cg-key-s">Rub out</span>
        </button>
      </div>
      <p className="muted small cg-help">A letter you type goes into every box with the same code. Letters already used are marked in the row above; using one again moves it.</p>
    </div>
  )
}

// ---- make ----

const CIPHERS: { value: Cipher; label: string; line: string }[] = [
  { value: 'random', label: 'Letters', line: 'Every letter is swapped for another letter.' },
  { value: 'caesar', label: 'Caesar shift', line: 'Every letter moves the same number of places along the alphabet — a good first code for children.' },
  { value: 'numbers', label: 'Numbers', line: 'Every letter becomes a number.' },
  { value: 'symbols', label: 'Symbols', line: 'Every letter becomes a little picture (★ ☀ ♥ …).' },
]

function commonest(text: string, lang: string, key: Record<string, string>, n: number): string[] {
  const counts: Record<string, number> = {}
  for (const l of letters(text, lang)) if (!key || Object.keys(key).length === 0 || key[l]) counts[l] = (counts[l] || 0) + 1
  return Object.entries(counts).sort((a, b) => b[1] - a[1] || (a[0] < b[0] ? -1 : 1)).slice(0, n).map(e => e[0])
}

function Edit({ data, onChange, language }: KindEditProps) {
  const { toast } = useToast()
  const c: CGData = { text: '', author: '', cipher: 'random', language: language || 'en', ...(data || {}), key: data?.key || {}, given: data?.given || [] }
  const lang = c.language || language || 'en'
  const [busy, setBusy] = useState(false)
  const [pickQuote, setPickQuote] = useState<'' | 'library' | 'books'>('')
  const [showAnswer, setShowAnswer] = useState(false)
  const [box, bw] = useWidth(600)
  const set = (patch: Partial<CGData>) => onChange({ ...c, ...patch })
  const hasKey = Object.keys(c.key || {}).length > 0
  const make = async (patch: Partial<CGData> = {}) => {
    const n = { ...c, ...patch }
    if (!n.text.trim()) { set(patch); toast('Write the text first'); return }
    setBusy(true)
    try {
      const r = await api.post<{ made: { data: CGData } }>(`${V1}/generate`, { kind: 'cryptogram', text: n.text, author: n.author || '', cipher: n.cipher, given: n.given.length, language: lang })
      onChange(r.made.data)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  const tokens = useMemo(() => encodeText({ ...c, language: lang }), [c.text, c.key, c.given, lang]) // eslint-disable-line react-hooks/exhaustive-deps
  const plain = useMemo(() => { const m: Record<string, string> = {}; for (const [l, s] of Object.entries(c.key || {})) m[s] = l; return m }, [c.key])
  const givenSet = new Set(c.given)
  const size = Math.max(28, Math.min(38, Math.floor(Math.min(bw, 700) / 12)))
  const different = new Set(letters(c.text, lang)).size
  return (
    <div className="cg-edit stack" ref={box}>
      <div className="field">
        <span className="label">The secret message</span>
        <textarea rows={4} value={c.text} maxLength={2000} placeholder="Meet me by the old oak tree at four o'clock." onChange={e => set({ text: e.target.value })} aria-label="The secret message" />
        <div className="row wrap">
          <span className="muted small grow">{letters(c.text, lang).length} letters, {different} different. Longer texts are easier to crack.</span>
          <button type="button" className="btn sm" onClick={() => setPickQuote('library')} disabled={busy}><Icon name="book-open" size={14} />A famous quote</button>
          <button type="button" className="btn sm" onClick={() => setPickQuote('books')} disabled={busy}><Icon name="book-open" size={14} />From Pocket Bookshelf</button>
        </div>
      </div>
      <div className="field">
        <span className="label">Who said it (optional)</span>
        <input value={c.author || ''} placeholder="Grandma" onChange={e => set({ author: e.target.value })} aria-label="Who said it" />
        <span className="help">The name is put in code too, under the message.</span>
      </div>
      <div className="field">
        <span className="label">Kind of code</span>
        <div className="scroll-x"><Segmented value={c.cipher} onChange={v => make({ cipher: v })} options={CIPHERS.map(x => ({ value: x.value, label: x.label, disabled: busy }))} /></div>
        <span className="help">{CIPHERS.find(x => x.value === c.cipher)?.line}</span>
      </div>
      <div className="field">
        <span className="label">Letters given away at the start</span>
        <Segmented value={c.given.length} onChange={n => set({ given: commonest(c.text, lang, c.key, n) })} options={[0, 1, 2, 3, 4, 5, 6].map(n => ({ value: n, label: String(n) }))} />
        <span className="help">{c.given.length ? `The commonest letters are shown: ${c.given.join(', ')}.` : 'Nothing is given away — the hardest.'} More letters make it easier.</span>
      </div>
      <div className="row wrap">
        <button type="button" className="btn primary" onClick={() => make()} disabled={busy || !c.text.trim()}>{busy ? <Spinner /> : <Icon name="key" size={14} />}{hasKey ? 'Make a new code' : 'Make the code'}</button>
        <span className="spacer" />
        <Switch checked={showAnswer} onChange={setShowAnswer} label="Show the letters" size="sm" /><span className="small muted">Show the letters</span>
      </div>
      <div className="cg-preview">
        <div className="muted small cg-preview-h">How players see it</div>
        {c.text.trim()
          ? <CodeText tokens={tokens} size={size} numbers={c.cipher === 'numbers'} locked={s => givenSet.has(plain[s])}
            letterOf={s => (showAnswer || givenSet.has(plain[s] || s)) ? (plain[s] || (hasKey ? '' : s)) : ''} />
          : <p className="muted small">Write a message above, then press “Make the code”.</p>}
        {!hasKey && c.text.trim() && <p className="muted small">No code yet — press “Make the code”.</p>}
      </div>
      {pickQuote === 'library' && <LibraryQuotes onClose={() => setPickQuote('')} onPick={q => { setPickQuote(''); make({ text: q.text, author: q.author }) }} />}
      {pickQuote === 'books' && <BookQuotes onClose={() => setPickQuote('')} onPick={q => { setPickQuote(''); make({ text: q.text, author: q.author }) }} />}
    </div>
  )
}

interface QuoteRow { text: string; author: string; book?: string }

function QuoteList({ quotes, onPick }: { quotes: QuoteRow[]; onPick: (q: QuoteRow) => void }) {
  return (
    <div className="cg-quotes">
      {quotes.map((q, i) => (
        <button key={i} type="button" className="cg-quote" onClick={() => onPick(q)}>
          <span className="cg-quote-t">“{q.text}”</span>
          {(q.author || q.book) && <span className="muted small">{[q.author, q.book].filter(Boolean).join(' · ')}</span>}
        </button>
      ))}
    </div>
  )
}

function LibraryQuotes({ onClose, onPick }: { onClose: () => void; onPick: (q: QuoteRow) => void }) {
  const [quotes, setQuotes] = useState<QuoteRow[] | null>(null)
  const [q, setQ] = useState('')
  useEffect(() => { loadThemes().then(r => setQuotes(r.quotes || [])).catch(() => setQuotes([])) }, [])
  const list = (quotes || []).filter(x => !q.trim() || (x.text + ' ' + x.author).toLowerCase().includes(q.trim().toLowerCase()))
  return (
    <Modal title="A famous quote" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={!quotes?.length} onClick={() => quotes && onPick(quotes[Math.floor(Math.random() * quotes.length)])}><Icon name="shuffle" size={14} />Surprise me</button></>}>
      <input value={q} onChange={e => setQ(e.target.value)} placeholder="Look for a word or a name" aria-label="Look for a quote" />
      {quotes === null ? <Spinner /> : list.length === 0 ? <p className="muted small">No quotes found.</p> : <QuoteList quotes={list} onPick={onPick} />}
    </Modal>
  )
}

function BookQuotes({ onClose, onPick }: { onClose: () => void; onPick: (q: QuoteRow) => void }) {
  const [books, setBooks] = useState<{ id: number | string; title: string; author: string; quotes: number }[] | null>(null)
  const [book, setBook] = useState<{ id: number | string; title: string } | null>(null)
  const [quotes, setQuotes] = useState<QuoteRow[] | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => { api.get<{ books: { id: number; title: string; author: string; quotes: number }[] }>(`${V1}/apps/books`).then(r => setBooks(r.books || [])).catch(e => { setErr((e as Error).message); setBooks([]) }) }, [])
  useEffect(() => {
    if (!book) return
    setQuotes(null)
    api.get<{ quotes: QuoteRow[] }>(`${V1}/apps/books/${book.id}/quotes`).then(r => setQuotes(r.quotes || [])).catch(e => { setErr((e as Error).message); setQuotes([]) })
  }, [book])
  return (
    <Modal title={book ? book.title : 'A quote from Pocket Bookshelf'} onClose={onClose} size="lg"
      footer={<>{book && <button type="button" className="btn" onClick={() => { setBook(null); setErr('') }}><Icon name="chevron-left" size={14} />All books</button>}<span className="spacer" /><button type="button" className="btn" onClick={onClose}>Cancel</button></>}>
      {err && <div className="callout warn"><Icon name="alert" size={15} /><div>{err}</div></div>}
      {!book && (books === null ? <Spinner /> : books.length === 0 ? (!err && <p className="muted small">None of your books has quotes yet. Add quotes to a book in Pocket Bookshelf.</p>)
        : <div className="cg-books">{books.map(b => <button key={b.id} type="button" className="cg-book" onClick={() => setBook(b)}><Icon name="book-open" size={15} /><span className="grow">{b.title}{b.author && <span className="muted"> · {b.author}</span>}</span><span className="muted small">{b.quotes}</span></button>)}</div>)}
      {book && (quotes === null ? <Spinner /> : quotes.length === 0 ? (!err && <p className="muted small">This book has no quotes short enough.</p>) : <QuoteList quotes={quotes} onPick={onPick} />)}
    </Modal>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as CGPublic
  if (answer) {
    const a = answer as CGData
    const used = Array.from(new Set(letters(a.text, a.language))).sort((x, y) => x.localeCompare(y))
    return (
      <div className="cg-print-key">
        <p className="cg-print-plain">{a.text}{a.author ? ` — ${a.author}` : ''}</p>
        <div className="cg-print-table">{used.map(l => <span key={l}><b>{l}</b> = {a.key?.[l] || l}</span>)}</div>
      </div>
    )
  }
  const fixed: Record<string, string> = {}
  for (const t of d.tokens || []) if (isSym(t) && t.l) fixed[t.s] = t.l
  const known = new Set(symbolOrder(d.tokens || []))
  const author = d.author_code && d.cipher !== 'numbers' ? splitCode(d.author_code, known) : []
  return (
    <div className={`cg-print ${large ? 'large' : ''}`}>
      {d.hint && <p className="cg-print-hint">{d.hint}</p>}
      <CodeText tokens={d.tokens || []} letterOf={s => fixed[s] || ''} size={large ? 40 : 30} numbers={d.cipher === 'numbers'} className="print" />
      {d.author_code && <div className="cg-author print"><span>—</span>{author.length ? <span className="cg-word">{author.map((s, k) => <span key={k} className="cg-cell small"><span className="cg-sym">{s}</span><span className="cg-box">{fixed[s] || ''}</span></span>)}</span> : <span>{d.author_code}</span>}</div>}
    </div>
  )
}

export const Cryptogram: KindModule = { Play, Edit, Print }
