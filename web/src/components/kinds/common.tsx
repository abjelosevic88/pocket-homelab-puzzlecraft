// Bits several puzzle kinds share: typing in words with clues (or taking
// them from a word list, a ready-made theme or the AI), a level picker.
import { useEffect, useMemo, useState } from 'react'
import { api, V1 } from '../../lib/api'
import type { WordClue, WordList } from '../../lib/types'
import { Icon } from '../icons'
import { Segmented, Spinner, useToast } from '../ui'
import { Select } from '../Select'

/** "word - clue" per line ↔ the list. */
export function wordsToText(ws: WordClue[]): string { return ws.map(w => w.clue ? `${w.word} - ${w.clue}` : w.word).join('\n') }
export function textToWords(text: string): WordClue[] {
  const out: WordClue[] = []
  for (let line of text.split('\n')) {
    line = line.trim().replace(/^[-*•\d.)\s]+/, '')
    if (!line) continue
    let word = line, clue = ''
    for (const sep of ['\t', ' - ', ' – ', ' — ', ': ', ';', ',']) {
      const i = line.indexOf(sep)
      if (i > 0) { word = line.slice(0, i); clue = line.slice(i + sep.length); break }
    }
    word = word.trim().replace(/^["']|["']$/g, '')
    if (word) out.push({ word, clue: clue.trim() })
  }
  return out
}

interface Theme { key: string; name: string; language: string; count: number; words: WordClue[] }
let themesCache: Promise<{ themes: Theme[]; logic_sets: { key: string; name: string; categories: string[] }[]; pictures: Record<string, string[]>; quotes: { text: string; author: string }[] }> | null = null
export function loadThemes() {
  if (!themesCache) themesCache = api.get(`${V1}/themes`)
  return themesCache
}

/** Words with clues from anywhere: typed, a word list, a ready-made theme, or the AI. */
export function WordsInput({ value, onChange, language, needClues = true, max = 80 }: { value: WordClue[]; onChange: (w: WordClue[]) => void; language: string; needClues?: boolean; max?: number }) {
  const { toast } = useToast()
  const [how, setHow] = useState<'type' | 'list' | 'theme' | 'ai'>('type')
  const [text, setText] = useState(() => wordsToText(value))
  const [lists, setLists] = useState<WordList[] | null>(null)
  const [themes, setThemes] = useState<Theme[] | null>(null)
  const [topic, setTopic] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => { if (how === 'list' && !lists) api.get<{ wordlists: WordList[] }>(`${V1}/wordlists`).then(r => setLists(r.wordlists)).catch(() => setLists([])) }, [how, lists])
  useEffect(() => { if (how === 'theme' && !themes) loadThemes().then(r => setThemes(r.themes)).catch(() => setThemes([])) }, [how, themes])
  const take = (ws: WordClue[]) => { const w = ws.slice(0, max); onChange(w); setText(wordsToText(w)); setHow('type') }
  const ai = async () => {
    if (!topic.trim()) return
    setBusy(true)
    try { const r = await api.post<{ words: WordClue[] }>(`${V1}/ai/words`, { topic, count: 15, language }); take(r.words) } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  const noClue = useMemo(() => value.filter(w => !w.clue).length, [value])
  return (
    <div className="words-input">
      <Segmented size="sm" value={how} onChange={setHow} options={[{ value: 'type', label: 'Type them' }, { value: 'list', label: 'My word lists' }, { value: 'theme', label: 'Ready-made' }, { value: 'ai', label: 'Ask the AI' }]} />
      {how === 'type' && <>
        <textarea rows={8} value={text} placeholder={needClues ? 'apple - fruit that keeps the doctor away\nbanana - long yellow fruit' : 'apple\nbanana\ncherry'}
          onChange={e => { setText(e.target.value); onChange(textToWords(e.target.value).slice(0, max)) }} aria-label="Words" />
        <p className="muted small">{value.length} {value.length === 1 ? 'word' : 'words'}{needClues && noClue > 0 ? ` · ${noClue} without a clue` : ''}. One per line{needClues ? '; the clue after a dash or a colon' : ''}.</p>
      </>}
      {how === 'list' && (lists === null ? <Spinner /> : lists.length === 0 ? <p className="muted small">No word lists yet — make one under Word lists.</p>
        : <div className="pick-list">{lists.map(l => <button key={l.id} type="button" className="pick-row" onClick={() => take(l.words)}><Icon name="list" size={15} /><span className="grow">{l.name}</span><span className="muted small">{l.count}</span></button>)}</div>)}
      {how === 'theme' && (themes === null ? <Spinner /> : <div className="pick-list">{themes.filter(t => t.language === language || language === 'other' || t.language === 'en').map(t => <button key={t.key} type="button" className="pick-row" onClick={() => take(t.words)}><Icon name="sparkles" size={15} /><span className="grow">{t.name}</span><span className="muted small">{t.count}</span></button>)}</div>)}
      {how === 'ai' && <div className="row">
        <input className="grow" value={topic} onChange={e => setTopic(e.target.value)} placeholder="What about? (the Olympics, our trip to Rome…)" onKeyDown={e => { if (e.key === 'Enter') ai() }} />
        <button type="button" className="btn" onClick={ai} disabled={busy || !topic.trim()}>{busy ? <Spinner /> : <Icon name="sparkles" size={14} />}Make words</button>
      </div>}
    </div>
  )
}

/** Pick a ready-made list in a select (for quick “make one for me”). */
export function ThemeSelect({ value, onChange, language }: { value: string; onChange: (k: string) => void; language: string }) {
  const [themes, setThemes] = useState<Theme[]>([])
  useEffect(() => { loadThemes().then(r => setThemes(r.themes)).catch(() => {}) }, [])
  return <Select ariaLabel="Words about" value={value} onChange={onChange} options={themes.filter(t => language === 'other' || t.language === language || (language === 'en' && t.language === 'en')).map(t => ({ value: t.key, label: t.name }))} />
}
