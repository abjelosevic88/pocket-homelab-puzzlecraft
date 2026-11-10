// A new puzzle: pick a kind, then let it be made for you (from your words,
// a ready-made list, the AI) or start empty — or open a crossword file.
import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api, upload, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange } from '../lib/data'
import { KIND, KINDS, LANGS, LEVELS } from '../lib/puzzlecraft'
import type { Kind, Lang, Level, Puzzle, WordClue } from '../lib/types'
import { Icon } from '../components/icons'
import { loadThemes, ThemeSelect, WordsInput } from '../components/kinds/common'
import { useCrumb } from '../components/Layout'
import { PhotoPick } from '../components/PhotoPick'
import { Select } from '../components/Select'
import { Field, Segmented, Spinner, useToast } from '../components/ui'

export default function NewPuzzle() {
  const [sp, setSp] = useSearchParams()
  const kind = (sp.get('kind') || '') as Kind | ''
  const how = sp.get('how')
  useCrumb(kind ? KIND[kind]?.name : how === 'file' ? 'Open a file' : 'New puzzle', '/puzzles')
  if (how === 'file') return <FromFile />
  if (!kind || !KIND[kind]) return (
    <div className="page">
      <div className="page-head"><div className="grow"><h2 className="page-title">What shall we make?</h2><p className="muted">Pick a kind of puzzle. You can always change everything afterwards.</p></div>
        <button type="button" className="btn" onClick={() => setSp({ how: 'file' })}><Icon name="upload" size={14} />Open a file</button></div>
      <div className="kind-grid">
        {KINDS.map(k => (
          <button key={k.kind} type="button" className="kind-tile" onClick={() => setSp({ kind: k.kind })} style={{ ['--tint' as any]: k.tint }}>
            <span className="kind-tile-ico"><Icon name={k.icon} size={26} /></span>
            <span className="kind-tile-name">{k.name}</span>
            <span className="muted small">{k.line}</span>
          </button>
        ))}
      </div>
      <p className="muted small">Want a whole game with stages, locks and hints? Make an <a href="/adventures?new=1">escape room or treasure hunt</a>.</p>
    </div>
  )
  return <Maker kind={kind} />
}

function Maker({ kind }: { kind: Kind }) {
  const { settings } = useApp()
  const navigate = useNavigate()
  const { toast } = useToast()
  const k = KIND[kind]
  const [title, setTitle] = useState('')
  const [lang, setLang] = useState<Lang>(settings?.language || 'en')
  const [level, setLevel] = useState<Level>(settings?.level || 'medium')
  const [mode, setMode] = useState<string>(() => ({ crossword: 'words', quiz: 'riddles', nonogram: 'library', cryptogram: 'quote', ladder: 'random', logic: 'bank' } as Record<string, string>)[kind] || 'make')
  const [words, setWords] = useState<WordClue[]>([])
  const [theme, setTheme] = useState('animals')
  const [size, setSize] = useState(kind === 'sudoku' ? 9 : kind === 'crossword' ? 9 : 0)
  const [message, setMessage] = useState('')
  const [text, setText] = useState('')
  const [author, setAuthor] = useState('')
  const [cipher, setCipher] = useState('random')
  const [picture, setPicture] = useState('')
  const [pics, setPics] = useState<Record<string, string[]>>({})
  const [sets, setSets] = useState<{ key: string; name: string }[]>([])
  const [set, setSet] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [topic, setTopic] = useState('')
  const [photo, setPhoto] = useState<{ id: number; thumb_url: string } | null>(null)
  const [busy, setBusy] = useState(false)
  useEffect(() => { loadThemes().then(t => { setPics(t.pictures); setSets(t.logic_sets); setPicture(Object.keys(t.pictures)[0] || ''); setSet(t.logic_sets[0]?.key || '') }).catch(() => {}) }, [])

  const make = async (empty = false) => {
    setBusy(true)
    try {
      let generate: Record<string, unknown> | undefined = { level, language: lang }
      let data: unknown
      switch (kind) {
        case 'crossword':
          if (mode === 'words') generate = { ...generate, words, theme: words.length ? '' : theme }
          else if (mode === 'classic') generate = { ...generate, style: 'classic', w: size, h: size }
          else generate = undefined
          break
        case 'wordsearch': generate = { ...generate, words, theme: words.length ? '' : theme, message, w: size || 0, h: size || 0 }; break
        case 'sudoku': generate = { ...generate, size }; break
        case 'nonogram': generate = mode === 'library' ? { picture } : undefined; break
        case 'cryptogram': generate = { ...generate, text: mode === 'own' ? text : '', author: mode === 'own' ? author : '', cipher }; break
        case 'scramble': generate = { ...generate, words, theme: words.length ? '' : theme }; break
        case 'quiz':
          if (mode === 'riddles') generate = { mode: 'riddles', count: 10 }
          else if (mode === 'words') generate = { ...generate, words, theme: words.length ? '' : theme }
          else if (mode === 'ai') {
            const r = await api.post<{ data: unknown }>(`${V1}/ai/quiz`, { topic, count: 10, level, language: lang })
            data = r.data; generate = undefined
          } else generate = undefined
          break
        case 'maze': generate = { ...generate }; break
        case 'logic': generate = { ...generate, set }; break
        case 'ladder': generate = mode === 'own' ? { from, to } : { ...generate }; break
        case 'jigsaw':
          if (!photo) { toast('Choose a picture first', 'err'); setBusy(false); return }
          data = { photo_id: photo.id, cols: { easy: 4, medium: 5, hard: 7, expert: 10 }[level], rows: { easy: 3, medium: 4, hard: 5, expert: 7 }[level], shape: 'classic' }
          generate = undefined
          break
      }
      if (empty) { generate = undefined; data = undefined }
      const p = await api.post<Puzzle>(`${V1}/puzzles`, { kind, title: title.trim() || undefined, language: lang, level, generate, data })
      announceChange()
      if (p.missing?.length) toast(`Did not fit: ${p.missing.join(', ')}`, 'err')
      else if (p.message) toast(p.message)
      navigate(`/puzzles/${p.id}`)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }

  const needsWords = (kind === 'crossword' && mode === 'words') || kind === 'wordsearch' || kind === 'scramble' || (kind === 'quiz' && mode === 'words')
  return (
    <div className="page narrow">
      <div className="page-head">
        <span className="kind-ico lg" style={{ color: k.tint, background: `color-mix(in srgb, ${k.tint} 13%, transparent)` }}><Icon name={k.icon} size={24} /></span>
        <div className="grow"><h2 className="page-title">New {k.name.toLowerCase()}</h2><p className="muted">{k.line}</p></div>
      </div>
      <div className="card pad-card stack">
        <div className="two-col tight">
          <Field label="Name"><input value={title} onChange={e => setTitle(e.target.value)} placeholder={k.name} /></Field>
          <Field label="Difficulty"><Segmented size="sm" value={level} onChange={setLevel} options={LEVELS.map(l => ({ value: l.value, label: l.label }))} /></Field>
        </div>
        {['crossword', 'wordsearch', 'scramble', 'cryptogram', 'quiz'].includes(kind) && <Field label="Language" help="Bosnian, Croatian and Serbian puzzles keep LJ, NJ and DŽ in one square."><Select ariaLabel="Language" value={lang} onChange={v => setLang(v as Lang)} options={LANGS.map(l => ({ value: l.value, label: l.label }))} /></Field>}

        {kind === 'crossword' && <Field label="How"><Segmented size="sm" value={mode} onChange={setMode} options={[{ value: 'words', label: 'From words and clues' }, { value: 'classic', label: 'A classic grid, filled for me' }, { value: 'empty', label: 'An empty grid' }]} /></Field>}
        {kind === 'quiz' && <Field label="How"><Segmented size="sm" value={mode} onChange={setMode} options={[{ value: 'riddles', label: 'Ten old riddles' }, { value: 'words', label: 'From words and clues' }, { value: 'ai', label: 'Ask the AI', disabled: settings?.ai_help === false }, { value: 'empty', label: 'I’ll write them' }]} /></Field>}
        {kind === 'nonogram' && <Field label="How"><Segmented size="sm" value={mode} onChange={setMode} options={[{ value: 'library', label: 'A ready-made picture' }, { value: 'empty', label: 'I’ll draw one' }]} /></Field>}
        {kind === 'cryptogram' && <Field label="Text"><Segmented size="sm" value={mode} onChange={setMode} options={[{ value: 'quote', label: 'A famous quote' }, { value: 'own', label: 'My own message' }]} /></Field>}
        {kind === 'ladder' && <Field label="How"><Segmented size="sm" value={mode} onChange={setMode} options={[{ value: 'random', label: 'Pick two words for me' }, { value: 'own', label: 'From my word to another' }]} /></Field>}

        {needsWords && <>
          <Field label="Words" help={words.length ? '' : 'Or leave it empty and use a ready-made list:'}><WordsInput value={words} onChange={setWords} language={lang} needClues={kind !== 'wordsearch'} /></Field>
          {!words.length && <Field label="Ready-made list"><ThemeSelect value={theme} onChange={setTheme} language={lang} /></Field>}
        </>}
        {kind === 'crossword' && mode === 'classic' && <Field label="Size" help="Bigger grids take longer to fill. You write the clues after (or let the AI)."><Segmented size="sm" value={size} onChange={setSize} options={[5, 7, 9, 11, 13].map(n => ({ value: n, label: `${n} × ${n}` }))} /></Field>}
        {kind === 'wordsearch' && <>
          <Field label="Hidden message (optional)" help="Spelled by the letters no word uses — the last thing to find."><input value={message} onChange={e => setMessage(e.target.value)} placeholder="Happy birthday" /></Field>
          <Field label="Size"><Segmented size="sm" value={size} onChange={setSize} options={[{ value: 0, label: 'Fits the words' }, { value: 10, label: '10 × 10' }, { value: 13, label: '13 × 13' }, { value: 16, label: '16 × 16' }, { value: 20, label: '20 × 20' }]} /></Field>
        </>}
        {kind === 'sudoku' && <Field label="Size" help="4 × 4 and 6 × 6 are lovely for children."><Segmented size="sm" value={size} onChange={setSize} options={[{ value: 4, label: '4 × 4' }, { value: 6, label: '6 × 6' }, { value: 9, label: '9 × 9' }]} /></Field>}
        {kind === 'nonogram' && mode === 'library' && <div className="pic-pick">{Object.entries(pics).map(([name, rows]) => (
          <button key={name} type="button" className={`pic-tile ${picture === name ? 'on' : ''}`} onClick={() => setPicture(name)} title={name}>
            <span className="pic-mini" style={{ gridTemplateColumns: `repeat(${rows[0].length}, 1fr)` }}>{rows.join('').split('').map((c, i) => <i key={i} className={c === '#' ? 'on' : ''} />)}</span>
          </button>))}</div>}
        {kind === 'cryptogram' && <>
          {mode === 'own' && <><Field label="Your message"><textarea rows={3} value={text} onChange={e => setText(e.target.value)} placeholder="Meet me at the old oak tree at noon." /></Field>
            <Field label="Signed (optional)"><input value={author} onChange={e => setAuthor(e.target.value)} /></Field></>}
          <Field label="Code"><Segmented size="sm" value={cipher} onChange={setCipher} options={[{ value: 'random', label: 'Mixed-up letters' }, { value: 'caesar', label: 'Letters moved along' }, { value: 'numbers', label: 'Numbers' }, { value: 'symbols', label: 'Symbols' }]} /></Field>
        </>}
        {kind === 'quiz' && mode === 'ai' && <Field label="What about?"><input value={topic} onChange={e => setTopic(e.target.value)} placeholder="The 1990s, the solar system, our family…" /></Field>}
        {kind === 'logic' && <Field label="Story"><Select ariaLabel="Story" value={set} onChange={setSet} options={sets.map(s => ({ value: s.key, label: s.name }))} /></Field>}
        {kind === 'ladder' && mode === 'own' && <div className="two-col tight"><Field label="From"><input value={from} onChange={e => setFrom(e.target.value)} placeholder="COLD" /></Field><Field label="To"><input value={to} onChange={e => setTo(e.target.value)} placeholder="WARM" /></Field></div>}
        {kind === 'jigsaw' && <Field label="Picture">{photo ? <div className="row"><img src={photo.thumb_url} alt="" className="thumb-lg" /><button type="button" className="btn sm" onClick={() => setPhoto(null)}>Another</button></div> : <PhotoPick onPicked={p => setPhoto(p)} />}</Field>}

        <div className="row wrap">
          <button type="button" className="btn primary" onClick={() => make(false)} disabled={busy || (kind === 'quiz' && mode === 'ai' && !topic.trim())}>{busy ? <Spinner /> : <Icon name="sparkles" size={15} />}{['crossword', 'quiz', 'nonogram'].includes(kind) && mode === 'empty' ? 'Start' : 'Make it'}</button>
          {kind !== 'jigsaw' && !(['crossword', 'quiz', 'nonogram'].includes(kind) && mode === 'empty') && <button type="button" className="btn" onClick={() => make(true)} disabled={busy}>Start empty instead</button>}
          <span className="spacer" />
          <button type="button" className="btn ghost" onClick={() => navigate('/new')}>Another kind</button>
        </div>
      </div>
    </div>
  )
}

function FromFile() {
  const navigate = useNavigate()
  const { toast } = useToast()
  const [busy, setBusy] = useState(false)
  const [text, setText] = useState('')
  const send = async (f: File) => {
    setBusy(true)
    try {
      const r = await upload<{ ok: boolean; puzzle_id?: number; text: string }>(`${V1}/import`, 'file', f)
      toast(r.text)
      announceChange()
      navigate(r.puzzle_id ? `/puzzles/${r.puzzle_id}` : '/puzzles')
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <div className="page narrow">
      <div className="page-head"><div className="grow"><h2 className="page-title">Open a puzzle file</h2><p className="muted">A crossword from Across Lite (.puz) or the open .ipuz format, a sudoku (.ipuz, or 81 digits from a sudoku site), or a Pocket Puzzlecraft export.</p></div></div>
      <div className="card pad-card stack">
        <label className="drop-zone">
          <Icon name="upload" size={24} />
          <span>{busy ? 'Opening…' : 'Choose a file'}</span>
          <input type="file" accept=".puz,.ipuz,.json,.txt" hidden onChange={e => { const f = e.target.files?.[0]; if (f) send(f) }} />
        </label>
        <Field label="Or paste a sudoku" help="81 digits, 0 or . for an empty square."><textarea rows={3} value={text} onChange={e => setText(e.target.value)} placeholder="530070000600195000…" /></Field>
        <div className="row"><button type="button" className="btn" disabled={busy || text.replace(/[^0-9.]/g, '').length !== 81} onClick={() => send(new File([text], 'sudoku.txt', { type: 'text/plain' }))}>Add the sudoku</button></div>
      </div>
    </div>
  )
}
