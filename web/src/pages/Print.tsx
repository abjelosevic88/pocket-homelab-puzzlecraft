// Printing: one puzzle with its answer key, a whole book (answers at the
// back), and an escape room's stage cards, QR cards and host sheet.
import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { ADV_KIND, KIND, LEVEL_NAME, LOCKS } from '../lib/puzzlecraft'
import type { Adventure, Book, Kind, PublicPuzzle } from '../lib/types'
import { Icon } from '../components/icons'
import { KINDS_UI } from '../components/kinds'
import { Markdown } from '../components/Markdown'
import { QRCode } from '../components/QRCode'
import { Loading } from '../components/ui'

type PrintDoc = PublicPuzzle & { answer: any }

function Bar({ title, children }: { title: string; children?: React.ReactNode }) {
  return (
    <div className="print-bar no-print">
      <button type="button" className="btn ghost" onClick={() => window.close()}><Icon name="x" size={15} />Close</button>
      <b className="grow">{title}</b>
      {children}
      <button type="button" className="btn primary" onClick={() => window.print()}><Icon name="printer" size={15} />Print</button>
    </div>
  )
}

function PuzzleSheet({ p, n, large }: { p: PrintDoc; n?: number; large?: boolean }) {
  const K = KINDS_UI[p.kind as Kind]
  return (
    <section className="print-page">
      <header className="print-head">
        {n !== undefined && <span className="print-n">{n}</span>}
        <div className="grow"><h1>{p.title}</h1><p>{KIND[p.kind as Kind]?.name}{p.level ? ` · ${LEVEL_NAME[p.level]}` : ''}{p.size_text ? ` · ${p.size_text}` : ''}</p></div>
        <span className="print-time">Time: ______</span>
      </header>
      {p.instructions && <p className="print-instr">{p.instructions}</p>}
      <K.Print view={p} large={large} />
    </section>
  )
}

function AnswerBlock({ p, n }: { p: PrintDoc; n?: number }) {
  const K = KINDS_UI[p.kind as Kind]
  return (
    <div className="print-answer">
      <h3>{n !== undefined ? `${n}. ` : ''}{p.title}</h3>
      <K.Print view={p} answer={p.answer} />
    </div>
  )
}

export function PrintPuzzle() {
  const { id = '' } = useParams()
  const [doc, setDoc] = useState<{ puzzle: PrintDoc; paper: string; large: boolean; share_url: string; shared: boolean } | null>(null)
  const [answers, setAnswers] = useState(true)
  const [large, setLarge] = useState(false)
  useEffect(() => { api.get<NonNullable<typeof doc>>(`${V1}/puzzles/${id}/print`).then(d => { setDoc(d); setLarge(d.large) }).catch(() => {}) }, [id])
  useEffect(() => { if (doc) document.title = doc.puzzle.title }, [doc])
  if (!doc) return <Loading />
  return (
    <div className={`print-doc paper-${doc.paper} ${large ? 'large' : ''}`}>
      <Bar title={doc.puzzle.title}>
        <label className="check-row"><input type="checkbox" checked={answers} onChange={e => setAnswers(e.target.checked)} />Answers on a second page</label>
        <label className="check-row"><input type="checkbox" checked={large} onChange={e => setLarge(e.target.checked)} />Large print</label>
      </Bar>
      <PuzzleSheet p={doc.puzzle} large={large} />
      {doc.shared && <div className="print-qr"><QRCode value={doc.share_url} size={84} /><span>Play it on a phone</span></div>}
      {answers && <section className="print-page answers"><h2>Answers</h2><AnswerBlock p={doc.puzzle} /></section>}
    </div>
  )
}

export function PrintBook() {
  const { id = '' } = useParams()
  const [doc, setDoc] = useState<{ book: Book; puzzles: PrintDoc[] } | null>(null)
  useEffect(() => { api.get<NonNullable<typeof doc>>(`${V1}/books/${id}/print`).then(setDoc).catch(() => {}) }, [id])
  useEffect(() => { if (doc) document.title = doc.book.title }, [doc])
  if (!doc) return <Loading />
  const b = doc.book
  return (
    <div className={`print-doc ${b.large ? 'large' : ''}`}>
      <Bar title={b.title} />
      <section className="print-page print-cover">
        <h1>{b.title}</h1>
        {b.subtitle && <p className="print-sub">{b.subtitle}</p>}
        <ol className="print-toc">{doc.puzzles.map((p, i) => <li key={i}><span>{p.title}</span><span className="muted">{KIND[p.kind as Kind]?.name}</span></li>)}</ol>
        {b.share_on && b.share_url && <div className="print-qr"><QRCode value={b.share_url} size={96} /><span>Play every puzzle on a phone</span></div>}
      </section>
      {doc.puzzles.map((p, i) => <PuzzleSheet key={i} p={p} n={i + 1} large={b.large} />)}
      {b.answers === 'back' && <section className="print-page answers"><h2>Answers</h2><div className="print-answers-grid">{doc.puzzles.map((p, i) => <AnswerBlock key={i} p={p} n={i + 1} />)}</div></section>}
    </div>
  )
}

export function PrintAdventure() {
  const { id = '' } = useParams()
  const [a, setA] = useState<Adventure | null>(null)
  const [what, setWhat] = useState({ cards: true, qr: true, host: true })
  useEffect(() => { api.get<Adventure>(`${V1}/adventures/${id}`).then(setA).catch(() => {}) }, [id])
  useEffect(() => { if (a) document.title = a.title }, [a])
  if (!a) return <Loading />
  const stages = a.stages || []
  const qrStages = stages.filter(s => s.lock === 'qr' && s.code)
  const cardURL = (code: string) => `${location.origin}/a/${a.share_token}?card=${code}`
  return (
    <div className="print-doc">
      <Bar title={a.title}>
        <label className="check-row"><input type="checkbox" checked={what.cards} onChange={e => setWhat({ ...what, cards: e.target.checked })} />Stage cards</label>
        <label className="check-row"><input type="checkbox" checked={what.qr} onChange={e => setWhat({ ...what, qr: e.target.checked })} />QR cards</label>
        <label className="check-row"><input type="checkbox" checked={what.host} onChange={e => setWhat({ ...what, host: e.target.checked })} />Host sheet</label>
      </Bar>
      {!a.share_on && <p className="callout warn no-print"><Icon name="alert" size={15} />The share link is off: the QR codes will not open until you switch it on (Invite tab).</p>}
      {what.cards && <section className="print-page">
        <header className="print-head"><div className="grow"><h1>{a.title}</h1><p>{ADV_KIND[a.kind]?.name} · stage cards — cut along the lines</p></div></header>
        <div className="print-cards">
          <div className="print-card start"><h3>Start here</h3><Markdown text={a.intro} />{a.share_on && <div className="print-card-qr"><QRCode value={`${location.origin}/a/${a.share_token}`} size={90} /><span>Scan to play</span></div>}</div>
          {stages.map((s, i) => <div key={s.id} className="print-card"><span className="print-n">{i + 1}</span><h3>{s.title}</h3><Markdown text={s.story} />{s.lock_hint && <p className="print-lock"><Icon name="lock" size={12} />{s.lock_hint}</p>}</div>)}
        </div>
      </section>}
      {what.qr && qrStages.length > 0 && <section className="print-page">
        <header className="print-head"><div className="grow"><h1>QR cards</h1><p>Hide each where its stage says; players scan it with their phone camera.</p></div></header>
        <div className="print-cards qr">{qrStages.map(s => <div key={s.id} className="print-card qr-card"><QRCode value={cardURL(s.code)} size={150} /><b>{a.title}</b><span className="muted small">{s.code}</span></div>)}</div>
      </section>}
      {what.host && <section className="print-page host">
        <header className="print-head"><div className="grow"><h1>{a.title} — host sheet</h1><p>Keep this one to yourself.</p></div></header>
        <table className="print-table"><thead><tr><th>#</th><th>Stage</th><th>Opens with</th><th>Hidden</th><th>Hints</th></tr></thead>
          <tbody>{stages.map((s, i) => <tr key={s.id}><td>{i + 1}</td><td><b>{s.title}</b>{s.unlock && <div className="small">→ {s.unlock}</div>}</td>
            <td>{LOCKS.find(l => l.value === s.lock)?.label}{s.answers.length ? <>: <b>{s.answers.join(' / ')}</b></> : s.puzzle_title ? `: ${s.puzzle_title}` : ''}</td>
            <td>{s.hidden}{s.props && <div className="small">needs {s.props}</div>}{s.place_name && <div className="small">{s.place_name}</div>}</td>
            <td className="small">{s.hints.map((h, j) => <div key={j}>{h.after}′ {h.text}</div>)}</td></tr>)}</tbody></table>
        {a.prep.length > 0 && <><h3>Setting up</h3><ul className="print-check">{a.prep.map(p => <li key={p.id}>☐ {p.text}</li>)}</ul></>}
      </section>}
    </div>
  )
}
