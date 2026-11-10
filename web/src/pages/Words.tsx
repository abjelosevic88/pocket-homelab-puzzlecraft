// Word lists: words with clues to make puzzles from — typed, pasted, made by
// the AI, or a ready-made one to start from; and on to Pocket Learn.
import { useCallback, useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, openAction, useAppState, useOnChange } from '../lib/data'
import { LANGS } from '../lib/puzzlecraft'
import type { Lang, Puzzle, WordList } from '../lib/types'
import { Icon } from '../components/icons'
import { WordsInput } from '../components/kinds/common'
import { Select } from '../components/Select'
import { Confirm, Empty, Field, Loading, MenuButton, Modal, useToast } from '../components/ui'

export default function Words() {
  const [sp, setSp] = useSearchParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const state = useAppState()
  const [lists, setLists] = useState<WordList[] | null>(null)
  const [del, setDel] = useState<WordList | null>(null)
  const sel = Number(sp.get('list') || 0)
  const load = useCallback(() => { api.get<{ wordlists: WordList[] }>(`${V1}/wordlists`).then(r => setLists(r.wordlists)).catch(() => {}) }, [])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  const cur = lists?.find(l => l.id === sel) || null
  const make = async (kind: string, l: WordList) => {
    try {
      const p = await api.post<Puzzle>(`${V1}/puzzles`, { kind, title: l.name, language: l.language, generate: { words: l.words, language: l.language } })
      announceChange(); navigate(`/puzzles/${p.id}`)
    } catch (e) { toast((e as Error).message, 'err') }
  }
  const learn = state('learn', 'learn:write')
  return (
    <div className="page">
      <div className="page-head">
        <div className="grow"><h2 className="page-title">Word lists</h2><p className="muted">Words with their clues — for crosswords, word searches, scrambles and quizzes. The crossword helper suggests them first.</p></div>
        <button type="button" className="btn primary" onClick={() => setSp({ new: '1' })}><Icon name="plus" size={15} />New list</button>
      </div>
      {!lists ? <Loading /> : lists.length === 0 ? <Empty title="No word lists yet" icon="list">Paste words and clues, let the AI make some on any topic, or start from a ready-made list.</Empty> :
        <div className="words-cols">
          <div className="card rows words-index">{lists.map(l => (
            <button key={l.id} type="button" className={`row ${l.id === sel ? 'on' : ''}`} onClick={() => setSp({ list: String(l.id) })}>
              <Icon name="list" size={15} className="muted" /><span className="grow ellipsis">{l.name}</span><span className="muted small">{l.count}</span>
            </button>
          ))}</div>
          {cur ? <div className="card pad-card stack" key={cur.id}>
            <div className="row">
              <input className="pp-title-input grow" defaultValue={cur.name} onBlur={async e => { if (e.target.value !== cur.name) { await api.put(`${V1}/wordlists/${cur.id}`, { name: e.target.value }); load() } }} aria-label="Name" />
              <MenuButton label="Make a puzzle" align="right" width={240} className="btn" items={() => [
                { heading: 'Make from these words' },
                { label: 'Crossword', icon: 'crossword', onSelect: () => make('crossword', cur) },
                { label: 'Word search', icon: 'search', onSelect: () => make('wordsearch', cur) },
                { label: 'Word scramble', icon: 'shuffle', onSelect: () => make('scramble', cur) },
                { label: 'Quiz (multiple choice)', icon: 'help', onSelect: () => make('quiz', cur) },
                'sep',
                { label: 'Flashcards in Pocket Learn', icon: 'brain', disabled: !learn.ok, hint: learn.ok ? undefined : 'not connected', onSelect: async () => { try { const r = await api.post<{ text: string; url?: string }>(`${V1}/apps/learn`, { wordlist_id: cur.id }); toast(r.text, 'ok', openAction(r.url)) } catch (e) { toast((e as Error).message, 'err') } } },
                { label: 'Delete the list', icon: 'trash', danger: true, onSelect: () => setDel(cur) },
              ]}><Icon name="sparkles" size={15} />Make…</MenuButton>
            </div>
            <Field label="Language"><Select ariaLabel="Language" value={cur.language} onChange={async v => { await api.put(`${V1}/wordlists/${cur.id}`, { language: v }); load() }} options={LANGS.map(l => ({ value: l.value, label: l.label }))} /></Field>
            <ListEditor list={cur} onSaved={load} />
          </div> : <Empty title="Pick a list" icon="list" />}
        </div>}
      {sp.get('new') === '1' && <NewList onClose={() => setSp({})} onMade={id => { load(); setSp({ list: String(id) }) }} />}
      {del && <Confirm title="Delete this list?" message={`“${del.name}” and its ${del.count} words go. Puzzles made from it stay.`} confirmLabel="Delete" onCancel={() => setDel(null)} onConfirm={async () => { await api.del(`${V1}/wordlists/${del.id}`); setDel(null); setSp({}); load(); announceChange() }} />}
    </div>
  )
}

function ListEditor({ list, onSaved }: { list: WordList; onSaved: () => void }) {
  const { toast } = useToast()
  const [words, setWords] = useState(list.words)
  const [dirty, setDirty] = useState(false)
  const save = async () => { try { await api.put(`${V1}/wordlists/${list.id}`, { words }); setDirty(false); onSaved(); toast('Saved') } catch (e) { toast((e as Error).message, 'err') } }
  return <>
    <WordsInput value={words} onChange={w => { setWords(w); setDirty(true) }} language={list.language} max={1000} />
    <div className="row"><button type="button" className="btn primary" disabled={!dirty} onClick={save}>Save the words</button></div>
  </>
}

function NewList({ onClose, onMade }: { onClose: () => void; onMade: (id: number) => void }) {
  const { toast } = useToast()
  const [name, setName] = useState('')
  const [lang, setLang] = useState<Lang>('en')
  const [words, setWords] = useState<{ word: string; clue: string }[]>([])
  const make = async () => {
    try { const l = await api.post<WordList>(`${V1}/wordlists`, { name: name.trim() || 'Words', language: lang, words }); announceChange(); onMade(l.id); onClose() } catch (e) { toast((e as Error).message, 'err') }
  }
  return (
    <Modal title="New word list" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={make}>Make it</button></>}>
      <div className="two-col tight"><Field label="Name"><input autoFocus value={name} onChange={e => setName(e.target.value)} placeholder="Spanish food words" /></Field>
        <Field label="Language"><Select ariaLabel="Language" value={lang} onChange={v => setLang(v as Lang)} options={LANGS.map(l => ({ value: l.value, label: l.label }))} /></Field></div>
      <WordsInput value={words} onChange={setWords} language={lang} max={1000} />
    </Modal>
  )
}
