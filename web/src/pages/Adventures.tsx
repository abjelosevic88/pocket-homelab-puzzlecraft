// Escape rooms, treasure hunts, mysteries and party games: the list, and
// starting one (empty, from a ready-made story, or written by the AI).
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useOnChange } from '../lib/data'
import { fmtDate } from '../lib/format'
import { ADV_KIND, ADV_KINDS } from '../lib/puzzlecraft'
import type { Adventure, AdventureKind } from '../lib/types'
import { Icon } from '../components/icons'
import { Empty, Field, Loading, Modal, Segmented, Spinner, useToast } from '../components/ui'

export default function Adventures() {
  const [sp, setSp] = useSearchParams()
  const [list, setList] = useState<Adventure[] | null>(null)
  const load = useCallback(() => { api.get<{ adventures: Adventure[] }>(`${V1}/adventures`).then(r => setList(r.adventures)).catch(() => {}) }, [])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  return (
    <div className="page">
      <div className="page-head">
        <div className="grow"><h2 className="page-title">Escape rooms &amp; hunts</h2><p className="muted">Stages played one after another: number, word, arrow and colour locks, QR cards to find, puzzles to solve, hints that come with time.</p></div>
        <button type="button" className="btn primary" onClick={() => setSp({ new: '1' })}><Icon name="plus" size={15} />New</button>
      </div>
      {!list ? <Loading /> : list.length === 0 ? <Empty title="Nothing planned yet" icon="key">Start from a ready-made story — The Professor’s Study, a birthday mystery, a pirate party hunt — and change it to fit your home.</Empty> :
        <div className="pz-cards">{list.map(a => {
          const k = ADV_KIND[a.kind]
          return (
            <Link key={a.id} to={`/adventures/${a.id}`} className="pz-card adv">
              <div className="pz-card-top"><span className="kind-ico"><Icon name={k?.icon || 'key'} size={18} /></span><span className="muted small grow">{k?.name}</span>
                {a.live > 0 && <span className="tag live">{a.live} playing</span>}{a.share_on && <span className="tag"><Icon name="link" size={11} />shared</span>}</div>
              <div className="pz-card-title">{a.title}</div>
              <div className="muted small">{a.stages_n} {a.stages_n === 1 ? 'stage' : 'stages'}{a.minutes ? ` · ${a.minutes} min` : ''}{a.place ? ` · ${a.place}` : ''}</div>
              <div className="pz-card-foot">
                <span className={`tag st-${a.status}`}>{a.status === 'draft' ? 'Being made' : a.status === 'ready' ? 'Ready' : 'Played'}</span>
                <span className="spacer" />
                <span className="muted small">{a.date ? fmtDate(a.date, false) + (a.time ? ` ${a.time}` : '') : 'no date'}</span>
              </div>
            </Link>
          )
        })}</div>}
      {sp.get('new') === '1' && <NewAdventure onClose={() => setSp({})} />}
    </div>
  )
}

function NewAdventure({ onClose }: { onClose: () => void }) {
  const { settings } = useApp()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [how, setHow] = useState<'template' | 'ai' | 'empty'>('template')
  const [tpls, setTpls] = useState<{ key: string; kind: AdventureKind; title: string; intro: string; stages: number; minutes: number }[]>([])
  const [tpl, setTpl] = useState('study')
  const [kind, setKind] = useState<AdventureKind>('escape')
  const [title, setTitle] = useState('')
  const [theme, setTheme] = useState('')
  const [players, setPlayers] = useState('')
  const [stages, setStages] = useState(5)
  const [busy, setBusy] = useState(false)
  useEffect(() => { api.get<{ adventures: typeof tpls }>(`${V1}/templates`).then(r => setTpls(r.adventures)).catch(() => {}) }, [])
  const make = async () => {
    setBusy(true)
    try {
      let a: Adventure
      if (how === 'template') a = await api.post<Adventure>(`${V1}/adventures`, { template: tpl })
      else if (how === 'empty') a = await api.post<Adventure>(`${V1}/adventures`, { kind, title: title.trim() || ADV_KIND[kind].name })
      else {
        const s = await api.post<{ title: string; intro: string; finale: string; stages: Record<string, unknown>[]; prep: string[] }>(`${V1}/ai/story`, { kind, theme, stages, players })
        a = await api.post<Adventure>(`${V1}/adventures`, { kind, title: title.trim() || s.title, intro: s.intro, finale: s.finale, theme, players, prep: s.prep.map(t => ({ text: t })) })
        for (const st of s.stages) await api.post(`${V1}/adventures/${a.id}/stages`, st)
      }
      announceChange()
      navigate(`/adventures/${a.id}`)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="A new game" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={busy || (how === 'ai' && !theme.trim())} onClick={make}>{busy && <Spinner />}{how === 'ai' ? (busy ? 'Writing the story…' : 'Write it') : 'Make it'}</button></>}>
      <Segmented value={how} onChange={setHow} options={[{ value: 'template', label: 'A ready-made story' }, { value: 'ai', label: 'Written by the AI', disabled: settings?.ai_help === false }, { value: 'empty', label: 'From scratch' }]} />
      {how === 'template' && <div className="tpl-grid">{tpls.map(t => (
        <button key={t.key} type="button" className={`tpl ${tpl === t.key ? 'on' : ''}`} onClick={() => setTpl(t.key)}>
          <span className="row"><Icon name={ADV_KIND[t.kind]?.icon || 'key'} size={16} /><b className="grow">{t.title}</b></span>
          <span className="muted small">{ADV_KIND[t.kind]?.name} · {t.stages} stages · {t.minutes} min</span>
          <span className="small">{t.intro}</span>
        </button>
      ))}</div>}
      {how !== 'template' && <>
        <Field label="What kind"><div className="tpl-grid four">{ADV_KINDS.map(k => (
          <button key={k.kind} type="button" className={`tpl ${kind === k.kind ? 'on' : ''}`} onClick={() => setKind(k.kind)}><span className="row"><Icon name={k.icon} size={16} /><b>{k.name}</b></span><span className="muted small">{k.line}</span></button>
        ))}</div></Field>
        <Field label="Name"><input value={title} onChange={e => setTitle(e.target.value)} placeholder={how === 'ai' ? 'Leave empty for the AI’s' : 'The locked library'} /></Field>
      </>}
      {how === 'ai' && <>
        <Field label="Theme" help="The story it is about, and anything it should use (rooms, objects, names)."><textarea rows={2} value={theme} onChange={e => setTheme(e.target.value)} placeholder="A pirate birthday for Mia (8) in our garden; the treasure is the cake" /></Field>
        <div className="two-col tight">
          <Field label="Who plays"><input value={players} onChange={e => setPlayers(e.target.value)} placeholder="6 children aged 7–9" /></Field>
          <Field label="Stages"><Segmented size="sm" value={stages} onChange={setStages} options={[3, 4, 5, 6, 8].map(n => ({ value: n, label: String(n) }))} /></Field>
        </div>
        <p className="muted small">The AI writes the story, every stage’s lock, answer and hints, where to hide things, and a setting-up list. Read it through and change what you like.</p>
      </>}
    </Modal>
  )
}
