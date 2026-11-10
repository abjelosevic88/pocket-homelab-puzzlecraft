// Quizzes, riddles and trivia: play (one question at a time like a pub quiz,
// or all of them in a list; points; a score page with the answers and why),
// make (questions with types, choices, pictures, riddles, from words, the AI),
// print (a numbered answer sheet; the key lists the answers).
import '../../styles/kind-quiz.css'
import React, { useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import type { CheckResult, WordClue } from '../../lib/types'
import { Icon } from '../icons'
import { Modal, Segmented, Spinner, Switch, useIsMobile, useToast } from '../ui'
import { Select } from '../Select'
import { PhotoPick } from '../PhotoPick'
import { WordsInput } from './common'
import type { KindEditProps, KindModule, KindPlayProps, KindPrintProps } from './types'

// ---- shared ----

type QType = 'text' | 'choice' | 'number' | 'truefalse'
type Mode = 'quiz' | 'riddles' | 'trivia'
interface QZPublicQ { q: string; type: QType; choices: string[] | null; hint?: string; points: number; photo_id?: number; many?: boolean }
interface QZPublic { mode: Mode; questions: QZPublicQ[]; pass_mark?: number }
interface QZQuestion { q: string; type: QType; answers: string[]; choices?: string[]; hint?: string; explain?: string; photo_id?: number; points?: number }
interface QZData { mode: Mode; questions: QZQuestion[]; pass_mark?: number; shuffle?: boolean }
interface Final { score: number; max: number; right: number[]; wrong: number[]; explain: string[]; answers: string[]; solved: boolean }

const LETTERS = 'ABCDEFGH'
const norm = (s: string) => (s || '').trim().toLowerCase().replace(/\s+/g, ' ')
const word = (mode: Mode) => mode === 'riddles' ? 'Riddle' : 'Question'
const pts = (n: number) => `${n} ${n === 1 ? 'point' : 'points'}`
const splitMany = (s: string) => (s || '').split('|').filter(Boolean)
const thumbURL = (id: number) => `${V1}/photos/${id}?thumb=1`

function cheer(pct: number, pass: number): string {
  if (pass > 0) return pct >= pass ? (pct === 100 ? 'A perfect score — you passed!' : 'You passed!') : `The pass mark is ${pass}% — have another go!`
  if (pct === 100) return 'A perfect score!'
  if (pct >= 70) return 'Well done!'
  if (pct >= 40) return 'Not bad at all!'
  return 'Good try — have another go!'
}

// ---- play ----

function Play({ view, progress, shown, done, onProgress, check, reveal, setActions, photoURL }: KindPlayProps) {
  const d = view.data as QZPublic
  const qs = d.questions || []
  const n = qs.length
  const mobile = useIsMobile()
  const [answers, setAnswers] = useState<string[]>(() => qs.map((_, i) => String(progress?.answers?.[i] ?? '')))
  const [checked, setChecked] = useState<Set<number>>(() => new Set([...(progress?.checked || []), ...shown]))
  const [right, setRight] = useState<Set<number>>(() => new Set(progress?.right || []))
  const [revealed, setRevealed] = useState<Record<number, { answer: string; explain: string }>>(() => ({ ...(progress?.revealed || {}) }))
  const [final, setFinal] = useState<Final | null>(() => progress?.final || null)
  const [stage, setStage] = useState<'questions' | 'score'>(() => progress?.final ? 'score' : 'questions')
  const [layout, setLayout] = useState<'one' | 'all'>(() => mobile || n <= 1 ? 'one' : 'all')
  const [idx, setIdx] = useState(() => { const k = qs.findIndex((_, i) => !(progress?.checked || []).includes(i) && !shown.includes(i)); return k >= 0 ? k : 0 })
  const [hints, setHints] = useState<Set<number>>(new Set())
  const [busy, setBusy] = useState(-2)
  const [confirmEnd, setConfirmEnd] = useState(false)
  const st = useRef({ answers, checked, right, revealed, final })
  st.current = { answers, checked, right, revealed, final }
  const finished = done || !!final
  const locked = (i: number) => finished || checked.has(i)

  const save = (patch: Partial<typeof st.current>) => {
    const s = { ...st.current, ...patch }
    onProgress({ answers: s.answers, checked: [...s.checked], right: [...s.right], revealed: s.revealed, final: s.final })
  }
  const setAnswer = (i: number, v: string) => {
    if (locked(i)) return
    const a = answers.map((x, k) => k === i ? v : x)
    setAnswers(a); setConfirmEnd(false); save({ answers: a })
  }
  const checkOne = async (i: number) => {
    if (locked(i) || !answers[i].trim()) return
    setBusy(i)
    try {
      const r = await check({ answers, index: i })
      if (!r) return
      const c = new Set([...checked, i]), rt = new Set(right)
      if ((r.right || []).includes(i)) rt.add(i)
      setChecked(c); setRight(rt); save({ checked: c, right: rt })
    } finally { setBusy(-2) }
  }
  const finish = async () => {
    setBusy(-1)
    try {
      const r = await check({ answers: st.current.answers }, true) as CheckResult | null
      if (!r) return
      const f: Final = { score: r.score || 0, max: r.max || qs.reduce((s, q) => s + (q.points || 1), 0), right: r.right || [], wrong: r.wrong || [], explain: r.explain || [], answers: r.answers || [], solved: r.solved }
      setFinal(f); setRight(new Set(f.right)); setStage('score'); setConfirmEnd(false)
      save({ final: f, right: new Set(f.right) })
      window.scrollTo?.({ top: 0, behavior: 'smooth' })
    } finally { setBusy(-2) }
  }
  const askFinish = () => {
    const empty = qs.filter((_, i) => !answers[i].trim() && !checked.has(i)).length
    if (empty && !confirmEnd) { setConfirmEnd(true); return }
    finish()
  }
  const showAnswer = async (i: number) => {
    if (locked(i)) return
    const v = await reveal(i)
    if (!v) return
    const rv = { ...st.current.revealed, [i]: { answer: String(v.answer ?? ''), explain: String(v.explain ?? '') } }
    const c = new Set([...st.current.checked, i])
    setRevealed(rv); setChecked(c); save({ revealed: rv, checked: c })
  }

  useEffect(() => {
    setActions({
      check: async () => {
        if (finished) return
        if (layout === 'one' || stage === 'score') { await checkOne(idx); return }
        // all at once: send only the answers not checked yet, so nothing counts twice
        const todo = qs.map((_, i) => i).filter(i => !checked.has(i) && answers[i].trim())
        if (!todo.length) return
        const only = answers.map((a, i) => todo.includes(i) ? a : '')
        const r = await check({ answers: only })
        if (!r) return
        const c = new Set([...checked, ...todo]), rt = new Set([...right, ...(r.right || []).filter(i => todo.includes(i))])
        setChecked(c); setRight(rt); save({ checked: c, right: rt })
      },
      reveal: finished ? undefined : async () => {
        const target = layout === 'one' && !locked(idx) ? idx : qs.findIndex((_, i) => !locked(i))
        if (target >= 0) await showAnswer(target)
      },
      revealLabel: 'Show the answer',
      clear: () => {
        const a = answers.map((x, i) => checked.has(i) ? x : '')
        setAnswers(a); setConfirmEnd(false); save({ answers: a })
      },
    })
  }, [idx, layout, answers, checked, right, finished, stage]) // eslint-disable-line react-hooks/exhaustive-deps

  const status = (i: number) => final ? (final.right.includes(i) ? 'right' : 'wrong') : revealed[i] ? 'shown' : checked.has(i) ? (right.has(i) ? 'right' : 'wrong') : answers[i].trim() ? 'answered' : ''
  const allDone = qs.every((_, i) => checked.has(i))
  const card = (i: number) => (
    <QuestionCard key={i} i={i} n={n} q={qs[i]} mode={d.mode} value={answers[i]} onValue={v => setAnswer(i, v)} locked={locked(i)} status={status(i)}
      revealed={revealed[i]} finalAnswer={final?.answers[i]} explain={final?.explain[i]} photoURL={photoURL} hintOpen={hints.has(i)}
      onHint={() => setHints(h => new Set([...h, i]))} onCheck={() => checkOne(i)} busy={busy === i} finished={finished} />
  )

  if (stage === 'score' && final) return <ScorePage d={d} final={final} answers={answers} onBack={() => setStage('questions')} />
  return (
    <div className={`qz-play ${layout}`}>
      <div className="qz-top">
        {layout === 'one'
          ? <nav className="qz-strip" aria-label={`${word(d.mode)}s`}>
            {qs.map((_, i) => <button key={i} type="button" className={`qz-dot ${status(i)} ${i === idx ? 'cur' : ''}`} onClick={() => setIdx(i)} aria-label={`${word(d.mode)} ${i + 1}`} aria-current={i === idx ? 'step' : undefined}>{i + 1}</button>)}
          </nav>
          : <span className="muted small grow">{n} {n === 1 ? word(d.mode).toLowerCase() : `${word(d.mode).toLowerCase()}s`} · {pts(qs.reduce((s, q) => s + (q.points || 1), 0))}</span>}
        {n > 1 && <Segmented size="sm" value={layout} onChange={setLayout} label="How to show the questions" options={[{ value: 'one', label: 'One at a time' }, { value: 'all', label: 'All' }]} />}
      </div>
      {layout === 'one' ? <>
        {qs[idx] && card(idx)}
        <div className="qz-nav">
          <button type="button" className="btn" disabled={idx === 0} onClick={() => setIdx(idx - 1)}><Icon name="chevron-left" size={15} />Back</button>
          <span className="spacer" />
          {idx + 1 < n
            ? <button type="button" className={`btn ${locked(idx) ? 'primary' : ''}`} onClick={() => setIdx(idx + 1)}>Next<Icon name="chevron-right" size={15} /></button>
            : finished ? <button type="button" className="btn primary" disabled={!final} onClick={() => setStage('score')}><Icon name="trophy" size={15} />My score</button>
              : <button type="button" className={`btn ${allDone ? 'primary' : ''}`} onClick={askFinish} disabled={busy === -1}>{busy === -1 ? <Spinner /> : <Icon name="flag" size={15} />}See my score</button>}
        </div>
      </> : <>
        <div className="qz-list">{qs.map((_, i) => card(i))}</div>
        <div className="qz-nav">
          <span className="spacer" />
          {finished ? <button type="button" className="btn primary" disabled={!final} onClick={() => setStage('score')}><Icon name="trophy" size={15} />My score</button>
            : <button type="button" className="btn primary" onClick={askFinish} disabled={busy === -1}>{busy === -1 ? <Spinner /> : <Icon name="flag" size={15} />}I'm done — see my score</button>}
        </div>
      </>}
      <div className="qz-end-note" aria-live="polite">
        {confirmEnd && !finished && <span>{qs.filter((_, i) => !answers[i].trim() && !checked.has(i)).length} still without an answer. <button type="button" className="link-btn" onClick={finish}>Finish anyway</button></span>}
      </div>
    </div>
  )
}

interface CardProps {
  i: number; n: number; q: QZPublicQ; mode: Mode; value: string; onValue: (v: string) => void; locked: boolean; status: string
  revealed?: { answer: string; explain: string }; finalAnswer?: string; explain?: string; photoURL: (id: number, thumb?: boolean) => string
  hintOpen: boolean; onHint: () => void; onCheck: () => void; busy: boolean; finished: boolean
}

function QuestionCard({ i, n, q, mode, value, onValue, locked, status, revealed, finalAnswer, explain, photoURL, hintOpen, onHint, onCheck, busy, finished }: CardProps) {
  const choices = q.type === 'truefalse' ? (q.choices?.length ? q.choices : ['True', 'False']) : (q.choices || [])
  const picked = q.many ? splitMany(value) : [value]
  const rightOnes = useMemo(() => new Set((finalAnswer || revealed?.answer || '').split(' / ').map(norm).filter(Boolean)), [finalAnswer, revealed])
  const toggle = (c: string) => {
    if (locked) return
    if (!q.many) { onValue(c); return }
    const s = new Set(picked)
    if (s.has(c)) s.delete(c); else s.add(c)
    onValue(choices.filter(x => s.has(x)).join('|'))
  }
  const onEnter = (e: React.KeyboardEvent) => { if (e.key === 'Enter') { e.preventDefault(); onCheck() } }
  return (
    <article className={`qz-card ${status}`} aria-label={`${word(mode)} ${i + 1}`}>
      <header className="qz-card-head">
        <span className="qz-count">{word(mode)} {i + 1}<span className="muted"> of {n}</span></span>
        <span className="spacer" />
        <span className="qz-points">{pts(q.points || 1)}</span>
      </header>
      {q.photo_id ? <div className="qz-photo"><img src={photoURL(q.photo_id)} alt="" loading="lazy" /></div> : null}
      <h3 className="qz-q">{q.q}</h3>
      {(q.type === 'choice' || q.type === 'truefalse') && <>
        {q.many && <p className="muted small qz-many">More than one is right — pick them all.</p>}
        <div className={`qz-choices ${q.type === 'truefalse' ? 'tf' : ''}`} role={q.many ? 'group' : 'radiogroup'}>
          {choices.map((c, k) => {
            const on = picked.includes(c)
            const isRight = rightOnes.has(norm(c))
            const cls = ['qz-choice', on ? 'on' : '', on && status === 'right' ? 'good' : '', on && status === 'wrong' ? 'bad' : '', isRight ? 'is-right' : ''].join(' ')
            return (
              <button key={k} type="button" className={cls} role={q.many ? 'checkbox' : 'radio'} aria-checked={on} disabled={locked && !on && !isRight} onClick={() => toggle(c)} aria-readonly={locked || undefined}>
                <span className="qz-choice-key">{q.type === 'truefalse' ? (k === 0 ? <Icon name="check" size={15} /> : <Icon name="x" size={15} />) : q.many ? (on ? <Icon name="check" size={14} /> : '') : LETTERS[k]}</span>
                <span className="qz-choice-t">{q.type === 'truefalse' ? (k === 0 ? 'True' : 'False') : c}</span>
              </button>
            )
          })}
        </div>
      </>}
      {(q.type === 'text' || q.type === 'number') && (
        <input className={`qz-input ${status}`} value={value} disabled={locked} onChange={e => onValue(e.target.value)} onKeyDown={onEnter}
          inputMode={q.type === 'number' ? 'decimal' : 'text'} placeholder={q.type === 'number' ? 'A number' : 'Your answer'} aria-label="Your answer" autoComplete="off" />
      )}
      <div className="qz-card-foot">
        <button type="button" className="btn sm ghost" onClick={onHint} disabled={!q.hint || hintOpen || finished}><Icon name="lightbulb" size={14} />{q.hint ? 'A hint' : 'No hint'}</button>
        <span className="spacer" />
        <button type="button" className="btn sm primary" onClick={onCheck} disabled={locked || !value.trim() || busy}>{busy ? <Spinner /> : <Icon name="check" size={14} />}Check</button>
      </div>
      <div className="qz-feedback" aria-live="polite">
        {hintOpen && q.hint && <p className="qz-hint"><Icon name="lightbulb" size={14} />{q.hint}</p>}
        {revealed ? <p className="qz-fb shown"><Icon name="eye" size={15} /><span>The answer: <b>{revealed.answer}</b>{revealed.explain ? <> — {revealed.explain}</> : null}</span></p>
          : status === 'right' ? <p className="qz-fb good"><Icon name="check-circle" size={15} /><span>Right! +{pts(q.points || 1)}{explain ? <> — {explain}</> : null}</span></p>
            : status === 'wrong' ? <p className="qz-fb bad"><Icon name="x-circle" size={15} /><span>{finalAnswer ? <>The answer: <b>{finalAnswer}</b>{explain ? <> — {explain}</> : null}</> : 'Not quite.'}</span></p>
              : null}
      </div>
    </article>
  )
}

function ScorePage({ d, final, answers, onBack }: { d: QZPublic; final: Final; answers: string[]; onBack: () => void }) {
  const pct = final.max ? Math.round(final.score * 100 / final.max) : 0
  const pass = d.pass_mark || 0
  return (
    <div className="qz-score">
      <div className="qz-score-head">
        <div className="qz-score-ring" style={{ ['--pct' as any]: `${pct}` }}><span><b>{final.score}</b><span className="muted"> / {final.max}</span></span></div>
        <div className="grow">
          <h3>{cheer(pct, pass)}</h3>
          <p className="muted">{final.score} of {pts(final.max)} · {pct}% · {final.right.length} of {d.questions.length} right</p>
        </div>
      </div>
      <ol className="qz-review">
        {d.questions.map((q, i) => {
          const ok = final.right.includes(i)
          const mine = q.many ? splitMany(answers[i]).join(', ') : answers[i]
          return (
            <li key={i} className={ok ? 'right' : 'wrong'}>
              <span className="qz-review-mark">{ok ? <Icon name="check-circle" size={17} /> : <Icon name="x-circle" size={17} />}</span>
              <div className="grow">
                <div className="qz-review-q">{q.q}</div>
                <div className="small">{mine ? <>You said <b>{mine}</b></> : <span className="muted">No answer</span>}{!ok && final.answers[i] ? <> · the answer is <b>{final.answers[i]}</b></> : null}</div>
                {final.explain[i] && <div className="small muted qz-review-why">{final.explain[i]}</div>}
              </div>
              <span className="qz-review-pts small">{ok ? `+${q.points || 1}` : '0'}</span>
            </li>
          )
        })}
      </ol>
      <div className="row"><button type="button" className="btn" onClick={onBack}><Icon name="chevron-left" size={15} />Look at the questions</button></div>
    </div>
  )
}

// ---- make ----

const TYPES: { value: QType; label: string; icon: string }[] = [
  { value: 'choice', label: 'Choices', icon: 'list' },
  { value: 'text', label: 'Typed answer', icon: 'text' },
  { value: 'number', label: 'Number', icon: 'hash' },
  { value: 'truefalse', label: 'True or false', icon: 'check-circle' },
]
const blankQ = (): QZQuestion => ({ q: '', type: 'choice', answers: [], choices: ['', '', '', ''], points: 1, hint: '', explain: '' })
const isBlank = (q: QZQuestion) => !q.q.trim() && !q.answers.length && !q.photo_id

function problemsOf(q: QZQuestion): string[] {
  const out: string[] = []
  if (!q.q.trim() && !q.photo_id) out.push('No question yet')
  if (!q.answers.filter(a => a.trim()).length) out.push(q.type === 'choice' ? 'Tick the right choice' : 'No answer yet')
  if (q.type === 'choice' && (q.choices || []).filter(c => c.trim()).length < 2) out.push('Needs two choices or more')
  if (q.type === 'number' && q.answers.some(a => a.trim() && isNaN(Number(a.replace(',', '.'))))) out.push('The answer is not a number')
  return out
}

function answerText(q: QZQuestion) {
  const a = q.answers.filter(x => x.trim())
  return a.length ? a.join(' / ') : ''
}

function Edit({ data, onChange, puzzle, settings, language }: KindEditProps) {
  const { toast } = useToast()
  const z: QZData = { mode: 'quiz', questions: [], pass_mark: 0, ...(data || {}) }
  const qs = (z.questions || []).map(q => ({ ...q, q: q.q || '', answers: q.answers || [], choices: q.choices || [] }))
  const [open, setOpen] = useState<number>(qs.length ? 0 : -1)
  const [dlg, setDlg] = useState<'' | 'riddles' | 'words' | 'ai'>('')
  const set = (patch: Partial<QZData>) => onChange({ ...z, ...patch })
  const setQ = (i: number, patch: Partial<QZQuestion>) => set({ questions: qs.map((q, k) => k === i ? { ...q, ...patch } : q) })
  const add = () => { set({ questions: [...qs, blankQ()] }); setOpen(qs.length) }
  const remove = (i: number) => { set({ questions: qs.filter((_, k) => k !== i) }); setOpen(o => o === i ? -1 : o > i ? o - 1 : o) }
  const move = (i: number, by: number) => {
    const j = i + by
    if (j < 0 || j >= qs.length) return
    const next = [...qs];[next[i], next[j]] = [next[j], next[i]]
    set({ questions: next }); setOpen(o => o === i ? j : o === j ? i : o)
  }
  const copy = (i: number) => { const next = [...qs]; next.splice(i + 1, 0, JSON.parse(JSON.stringify(qs[i]))); set({ questions: next }); setOpen(i + 1) }
  const append = (more: QZQuestion[], mode?: Mode) => {
    const keep = qs.filter(q => !isBlank(q))
    const fixed = more.map(q => ({ ...q, answers: q.answers || [], choices: q.choices || [], points: q.points || 1 }))
    onChange({ ...z, mode: mode && !keep.length ? mode : z.mode, questions: [...keep, ...fixed] })
    setOpen(-1)
    toast(`${fixed.length} ${fixed.length === 1 ? 'question' : 'questions'} added — read them through`)
  }
  const total = qs.reduce((s, q) => s + (q.points || 1), 0)
  const aiOn = settings?.ai_help !== false
  return (
    <div className="qz-edit">
      <div className="ed-toolbar qz-toolbar">
        <Segmented size="sm" value={z.mode} onChange={v => set({ mode: v })} label="Kind of quiz" options={[{ value: 'quiz', label: 'Quiz' }, { value: 'riddles', label: 'Riddles' }, { value: 'trivia', label: 'Trivia' }]} />
        <div className="qz-pass"><Select ariaLabel="Pass mark" value={String(z.pass_mark || 0)} onChange={v => set({ pass_mark: Number(v) })}
          options={[{ value: '0', label: 'No pass mark' }, ...[50, 60, 70, 80, 90, 100].map(p => ({ value: String(p), label: `Pass at ${p}%` }))]} /></div>
        <span className="spacer" />
        <button type="button" className="btn sm" onClick={() => setDlg('riddles')}><Icon name="help" size={14} />Riddles</button>
        <button type="button" className="btn sm" onClick={() => setDlg('words')}><Icon name="list" size={14} />From words</button>
        <button type="button" className="btn sm" onClick={() => setDlg('ai')} disabled={!aiOn} title={aiOn ? 'Questions about anything, written by the AI' : 'AI help is switched off in Settings'}><Icon name="sparkles" size={14} />Ask the AI</button>
      </div>
      <p className="muted small ed-help">{qs.length} {qs.length === 1 ? 'question' : 'questions'} · {pts(total)} in all.{z.pass_mark ? ` Players pass with ${z.pass_mark}% of the points.` : ''} Players see one question at a time on a phone, with points and a score at the end.</p>
      <ol className="qz-qlist">
        {qs.map((q, i) => {
          const probs = problemsOf(q)
          const isOpen = open === i
          return (
            <li key={i} className={`qz-qitem ${isOpen ? 'open' : ''}`}>
              <div className="qz-qrow">
                <button type="button" className="qz-qsum" onClick={() => setOpen(isOpen ? -1 : i)} aria-expanded={isOpen}>
                  <span className="qz-qnum">{i + 1}</span>
                  <Icon name={TYPES.find(t => t.value === q.type)?.icon || 'help'} size={15} className="muted" />
                  <span className="grow qz-qsum-t">
                    <span className="ellipsis">{q.q.trim() || <span className="muted">A new question</span>}</span>
                    <span className="small muted ellipsis">{answerText(q) ? <>Answer: {answerText(q)}</> : null}{probs.length ? <span className="qz-prob"> {answerText(q) ? '· ' : ''}{probs[0]}</span> : null}</span>
                  </span>
                  {q.photo_id ? <Icon name="image" size={14} className="muted" /> : null}
                  <span className="qz-qpts small">{q.points || 1}</span>
                  <Icon name={isOpen ? 'chevron-up' : 'chevron-down'} size={15} className="muted" />
                </button>
                <div className="qz-qmove">
                  <button type="button" className="btn xs ghost" disabled={i === 0} onClick={() => move(i, -1)} aria-label="Move up" title="Move up"><Icon name="arrow-up" size={13} /></button>
                  <button type="button" className="btn xs ghost" disabled={i === qs.length - 1} onClick={() => move(i, 1)} aria-label="Move down" title="Move down"><Icon name="arrow-down" size={13} /></button>
                </div>
              </div>
              {isOpen && <QuestionEditor q={q} i={i} onChange={p => setQ(i, p)} onRemove={() => remove(i)} onCopy={() => copy(i)} problems={probs} />}
            </li>
          )
        })}
      </ol>
      <button type="button" className="btn qz-add" onClick={add}><Icon name="plus" size={15} />Add a question</button>
      {dlg === 'riddles' && <RiddlesDialog onClose={() => setDlg('')} onMade={q => append(q, 'riddles')} />}
      {dlg === 'words' && <FromWordsDialog lang={language || puzzle.language} onClose={() => setDlg('')} onMade={q => append(q)} />}
      {dlg === 'ai' && <AIDialog mode={z.mode} level={puzzle.level} lang={language || puzzle.language} onClose={() => setDlg('')} onMade={q => append(q)} />}
    </div>
  )
}

function QuestionEditor({ q, i, onChange, onRemove, onCopy, problems }: { q: QZQuestion; i: number; onChange: (p: Partial<QZQuestion>) => void; onRemove: () => void; onCopy: () => void; problems: string[] }) {
  const { toast } = useToast()
  const choices = q.choices || []
  const ticked = (c: string) => !!c.trim() && q.answers.some(a => norm(a) === norm(c))
  const setType = (t: QType) => {
    if (t === q.type) return
    if (t === 'truefalse') { const a = q.answers.find(x => /^(true|false)$/i.test(x.trim())); onChange({ type: t, choices: ['True', 'False'], answers: [a ? (/^t/i.test(a) ? 'True' : 'False') : 'True'] }); return }
    if (t === 'choice') {
      const base = q.type === 'truefalse' ? [] : q.answers.filter(a => a.trim()).slice(0, 1)
      const cs = [...base, ...choices.filter(c => c.trim() && !base.some(b => norm(b) === norm(c)) && q.type !== 'truefalse')]
      while (cs.length < 4) cs.push('')
      onChange({ type: t, choices: cs, answers: base })
      return
    }
    onChange({ type: t, choices: [], answers: q.type === 'truefalse' || q.type === 'choice' ? q.answers.slice(0, 1) : q.answers })
  }
  const tick = (k: number) => {
    const c = choices[k]
    if (!c.trim()) { toast('Write the choice first'); return }
    onChange({ answers: ticked(c) ? q.answers.filter(a => norm(a) !== norm(c)) : [...q.answers, c.trim()] })
  }
  const setChoice = (k: number, v: string) => {
    const old = choices[k]
    const cs = choices.map((x, j) => j === k ? v : x)
    onChange(ticked(old) ? { choices: cs, answers: q.answers.map(a => norm(a) === norm(old) ? v.trim() : a).filter(a => a) } : { choices: cs })
  }
  const dropChoice = (k: number) => {
    const old = choices[k]
    onChange({ choices: choices.filter((_, j) => j !== k), answers: q.answers.filter(a => norm(a) !== norm(old) || !old.trim()) })
  }
  const accepted = q.answers.length ? q.answers : ['']
  const setAccepted = (k: number, v: string) => onChange({ answers: accepted.map((a, j) => j === k ? v : a) })
  return (
    <div className="qz-qedit">
      <label className="field"><span className="label">{`Question ${i + 1}`}</span>
        <textarea rows={2} value={q.q} onChange={e => onChange({ q: e.target.value })} placeholder="What is the tallest animal in the world?" /></label>
      <div className="field"><span className="label">How to answer</span>
        <div className="scroll-x"><Segmented size="sm" value={q.type} onChange={setType} label="How to answer" options={TYPES.map(t => ({ value: t.value, label: t.label }))} /></div></div>

      {q.type === 'choice' && <div className="field">
        <span className="label">Choices — tick the right one</span>
        <div className="qz-ch-list">
          {choices.map((c, k) => (
            <div key={k} className={`qz-ch-row ${ticked(c) ? 'on' : ''}`}>
              <button type="button" className="qz-tick" role="checkbox" aria-checked={ticked(c)} onClick={() => tick(k)} aria-label={`Choice ${LETTERS[k] || k + 1} is right`} title="This one is right">{ticked(c) ? <Icon name="check" size={14} /> : LETTERS[k]}</button>
              <input value={c} onChange={e => setChoice(k, e.target.value)} placeholder={`Choice ${LETTERS[k] || k + 1}`} aria-label={`Choice ${LETTERS[k] || k + 1}`} />
              <button type="button" className="btn xs ghost" onClick={() => dropChoice(k)} disabled={choices.length <= 2} aria-label="Remove this choice" title="Remove this choice"><Icon name="x" size={13} /></button>
            </div>
          ))}
        </div>
        <div className="row wrap">
          <button type="button" className="btn sm ghost" onClick={() => onChange({ choices: [...choices, ''] })} disabled={choices.length >= 8}><Icon name="plus" size={14} />Another choice</button>
          <span className="help grow">{q.answers.length > 1 ? 'Several are ticked: players must pick all of them.' : 'Tick more than one if several are right.'}</span>
        </div>
      </div>}

      {q.type === 'truefalse' && <div className="field">
        <span className="label">The answer</span>
        <Segmented value={/^f/i.test(q.answers[0] || '') ? 'False' : 'True'} onChange={v => onChange({ answers: [v], choices: ['True', 'False'] })} label="The answer" options={[{ value: 'True', label: 'True' }, { value: 'False', label: 'False' }]} />
      </div>}

      {q.type === 'text' && <div className="field">
        <span className="label">The answer</span>
        <div className="qz-ch-list">
          {accepted.map((a, k) => (
            <div key={k} className="qz-ch-row">
              <input value={a} onChange={e => setAccepted(k, e.target.value)} placeholder={k === 0 ? 'Giraffe' : 'Another way to write it'} aria-label={k === 0 ? 'The answer' : 'Also accept'} />
              <button type="button" className="btn xs ghost" onClick={() => onChange({ answers: accepted.filter((_, j) => j !== k) })} disabled={accepted.length <= 1} aria-label="Remove" title="Remove"><Icon name="x" size={13} /></button>
            </div>
          ))}
        </div>
        <div className="row wrap">
          <button type="button" className="btn sm ghost" onClick={() => onChange({ answers: [...accepted, ''] })}><Icon name="plus" size={14} />Also accept…</button>
          <span className="help grow">Capitals, accents and “the” or “a” in front do not matter.</span>
        </div>
      </div>}

      {q.type === 'number' && <label className="field"><span className="label">The answer (a number)</span>
        <input className="qz-num-in" value={q.answers[0] || ''} inputMode="decimal" placeholder="42" onChange={e => onChange({ answers: [e.target.value] })} /></label>}

      <div className="two-col">
        <label className="field"><span className="label">A hint (optional)</span><input value={q.hint || ''} onChange={e => onChange({ hint: e.target.value })} placeholder="It has a very long neck" /></label>
        <div className="field"><span className="label">Points</span>
          <div className="qz-stepper">
            <button type="button" className="btn sm" onClick={() => onChange({ points: Math.max(1, (q.points || 1) - 1) })} disabled={(q.points || 1) <= 1} aria-label="One point less"><Icon name="minus" size={14} /></button>
            <span className="qz-stepper-v">{pts(q.points || 1)}</span>
            <button type="button" className="btn sm" onClick={() => onChange({ points: Math.min(20, (q.points || 1) + 1) })} disabled={(q.points || 1) >= 20} aria-label="One point more"><Icon name="plus" size={14} /></button>
          </div>
        </div>
      </div>
      <label className="field"><span className="label">Why (shown after the quiz)</span>
        <textarea rows={2} value={q.explain || ''} onChange={e => onChange({ explain: e.target.value })} placeholder="Giraffes grow up to 5.5 metres tall." /></label>
      <div className="field"><span className="label">A picture (optional)</span>
        <div className="qz-pic">
          {q.photo_id ? <img src={thumbURL(q.photo_id)} alt="" /> : <span className="qz-pic-empty"><Icon name="image" size={20} /></span>}
          <div className="col">
            <PhotoPick small label={q.photo_id ? 'Another picture' : 'Add a picture'} onPicked={p => onChange({ photo_id: p.id })} />
            <button type="button" className="btn sm ghost" disabled={!q.photo_id} onClick={() => onChange({ photo_id: 0 })}><Icon name="trash" size={14} />Remove the picture</button>
          </div>
        </div>
      </div>
      <div className="qz-qedit-foot">
        {problems.length > 0 ? <span className="small qz-prob"><Icon name="alert" size={14} />{problems.join(' · ')}</span> : <span className="small ok-text"><Icon name="check" size={14} />Ready</span>}
        <span className="spacer" />
        <button type="button" className="btn sm ghost" onClick={onCopy}><Icon name="copy" size={14} />Copy</button>
        <button type="button" className="btn sm ghost danger-ghost" onClick={onRemove}><Icon name="trash" size={14} />Remove</button>
      </div>
    </div>
  )
}

function RiddlesDialog({ onClose, onMade }: { onClose: () => void; onMade: (q: QZQuestion[]) => void }) {
  const { toast } = useToast()
  const [count, setCount] = useState(10)
  const [busy, setBusy] = useState(false)
  const make = async () => {
    setBusy(true)
    try { const r = await api.post<{ made: { data: QZData } }>(`${V1}/generate`, { kind: 'quiz', mode: 'riddles', count }); onMade(r.made.data.questions || []); onClose() } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="Riddles" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={make} disabled={busy}>{busy ? <Spinner /> : <Icon name="plus" size={14} />}Add riddles</button></>}>
      <p className="muted small">Ready-made riddles with their answers (“What has keys but can't open locks?”), added at the end.</p>
      <Segmented value={count} onChange={setCount} label="How many" options={[5, 10, 15, 20].map(n => ({ value: n, label: String(n) }))} />
    </Modal>
  )
}

function FromWordsDialog({ lang, onClose, onMade }: { lang: string; onClose: () => void; onMade: (q: QZQuestion[]) => void }) {
  const { toast } = useToast()
  const [words, setWords] = useState<WordClue[]>([])
  const [busy, setBusy] = useState(false)
  const withClue = words.filter(w => w.clue.trim()).length
  const make = async () => {
    setBusy(true)
    try { const r = await api.post<{ made: { data: QZData } }>(`${V1}/generate`, { kind: 'quiz', words, count: Math.min(50, words.length), language: lang }); onMade(r.made.data.questions || []); onClose() } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="Questions from words" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={make} disabled={busy || withClue < 2}>{busy ? <Spinner /> : null}Make questions</button></>}>
      <p className="muted small">Each clue becomes a question; its word is the right answer and three other words are the wrong choices.</p>
      <WordsInput value={words} onChange={setWords} language={lang} max={50} />
    </Modal>
  )
}

function AIDialog({ mode, level, lang, onClose, onMade }: { mode: Mode; level: string; lang: string; onClose: () => void; onMade: (q: QZQuestion[]) => void }) {
  const { toast } = useToast()
  const [topic, setTopic] = useState('')
  const [count, setCount] = useState('10')
  const [choices, setChoices] = useState(true)
  const [busy, setBusy] = useState(false)
  const make = async () => {
    if (!topic.trim()) return
    setBusy(true)
    try {
      const r = await api.post<{ data: QZData }>(`${V1}/ai/quiz`, { topic, count: Number(count), mode, choices, level, language: lang })
      onMade(r.data.questions || []); onClose()
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="Ask the AI for questions" onClose={onClose} footer={<><button type="button" className="btn" onClick={onClose} disabled={busy}>Cancel</button><button type="button" className="btn primary" onClick={make} disabled={busy || !topic.trim()}>{busy ? <Spinner /> : <Icon name="sparkles" size={14} />}{busy ? 'Writing…' : 'Write them'}</button></>}>
      <div className="stack tight">
        <label className="field"><span className="label">What about?</span>
          <input value={topic} onChange={e => setTopic(e.target.value)} placeholder="Space, the 1990s, our family holiday in Split…" onKeyDown={e => { if (e.key === 'Enter') make() }} autoFocus /></label>
        <div className="row wrap">
          <div className="qz-pass"><Select ariaLabel="How many" value={count} onChange={setCount} options={[5, 10, 15, 20, 30].map(n => ({ value: String(n), label: `${n} ${mode === 'riddles' ? 'riddles' : 'questions'}` }))} /></div>
          {mode !== 'riddles' && <><Switch checked={choices} onChange={setChoices} label="Multiple choice only" size="sm" /><span className="small">Multiple choice only</span></>}
        </div>
        <p className="muted small">They are added at the end. Read them through: the AI can get a fact wrong.</p>
      </div>
    </Modal>
  )
}

// ---- print ----

function Print({ view, answer, large }: KindPrintProps) {
  const d = view.data as QZPublic
  const qs = d.questions || []
  if (answer) {
    const a = answer as QZData
    return (
      <ol className="qz-print-key">
        {(a.questions || []).map((q, i) => <li key={i}><b>{answerText(q) || '—'}</b>{q.explain ? <span> — {q.explain}</span> : null}</li>)}
      </ol>
    )
  }
  const max = qs.reduce((s, q) => s + (q.points || 1), 0)
  return (
    <div className={`qz-print ${large ? 'large' : ''}`}>
      <div className="qz-print-head"><span>Name: <i className="qz-line w" /></span><span>Score: <i className="qz-line s" /> / {max}</span></div>
      <ol className="qz-print-list">
        {qs.map((q, i) => {
          const choices = q.type === 'truefalse' ? ['True', 'False'] : (q.choices || [])
          return (
            <li key={i}>
              <div className="qz-print-q">{q.q} <span className="qz-print-pts">({pts(q.points || 1)})</span></div>
              {q.photo_id ? <img className="qz-print-img" src={thumbURL(q.photo_id)} alt="" /> : null}
              {(q.type === 'choice' || q.type === 'truefalse')
                ? <>{q.many && <div className="qz-print-note">Tick all that are right.</div>}
                  <div className={`qz-print-choices ${q.type === 'truefalse' ? 'tf' : ''}`}>{choices.map((c, k) => <span key={k}><i className={q.many ? 'sq' : 'ci'} />{q.type === 'truefalse' ? c : <><b>{LETTERS[k]}</b> {c}</>}</span>)}</div></>
                : <div className="qz-print-ans"><i className="qz-line a" /></div>}
            </li>
          )
        })}
      </ol>
    </div>
  )
}

export const Quiz: KindModule = { Play, Edit, Print }
