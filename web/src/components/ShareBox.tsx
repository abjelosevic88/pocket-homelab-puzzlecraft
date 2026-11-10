// Sharing: the link anyone can play with (on/off, copy, QR code) and
// sending it to people through the other Pocket apps.
import React, { useEffect, useState } from 'react'
import { api, V1 } from '../lib/api'
import { openAction, useAppState } from '../lib/data'
import type { AppSlug } from '../lib/types'
import { Icon } from './icons'
import { QRCode } from './QRCode'
import { Select } from './Select'
import { copyText, Field, Modal, SettingRow, Spinner, Switch, useToast } from './ui'

export type ShareWhat = 'puzzle' | 'adventure' | 'book'

interface Props {
  what: ShareWhat; id: number; on: boolean; url: string; title: string
  onToggle: (on: boolean) => Promise<void>; onRenew?: () => Promise<void>
  options?: React.ReactNode // board / hints switches
}

export function ShareBox({ what, id, on, url, title, onToggle, onRenew, options }: Props) {
  const { toast } = useToast()
  const [busy, setBusy] = useState(false)
  const [qr, setQr] = useState(false)
  const [send, setSend] = useState<null | 'chat' | 'email' | 'family' | 'blog' | 'quest' | 'calendar'>(null)
  const state = useAppState()
  const flip = async (v: boolean) => { setBusy(true); try { await onToggle(v) } finally { setBusy(false) } }
  const word = what === 'adventure' ? 'game' : what
  const apps: { key: NonNullable<typeof send>; slug: AppSlug; scope: string; label: string; icon: string; only?: ShareWhat[] }[] = [
    { key: 'chat', slug: 'chat', scope: 'chat:write', label: 'Send to a chat', icon: 'message' },
    { key: 'email', slug: 'email', scope: 'mail:send', label: 'Email it', icon: 'mail' },
    { key: 'family', slug: 'family', scope: 'family:write', label: 'Pin to the family board', icon: 'home' },
    { key: 'calendar', slug: 'calendar', scope: 'calendar:write', label: 'Put in the calendar', icon: 'calendar' },
    { key: 'blog', slug: 'blog', scope: 'blog:write', label: 'Start a blog post', icon: 'pen' },
    { key: 'quest', slug: 'quest', scope: 'quest:write', label: 'Hand out in Pocket Quest', icon: 'd20', only: ['puzzle'] },
  ]
  return (
    <div className="stack">
      <section className="card share-card">
        <div className="row">
          <span className="share-ico"><Icon name="link" size={18} /></span>
          <div className="grow">
            <h3>Share link</h3>
            <p className="muted small">{on ? `Anyone with the link can play — no account needed.` : `Switch it on and anyone you give the link to can play this ${word}.`}</p>
          </div>
          <Switch checked={on} onChange={flip} disabled={busy} label="Share link" />
        </div>
        <div className={`share-url ${on ? '' : 'off'}`}>
          <code className="ellipsis">{url}</code>
          <button type="button" className="btn sm" disabled={!on} onClick={async () => { if (await copyText(url)) toast('Link copied') }}><Icon name="copy" size={14} />Copy</button>
          <a className={`btn sm ${on ? '' : 'disabled'}`} href={on ? url : undefined} target="_blank" rel="noopener" aria-disabled={!on}><Icon name="external" size={14} />Open</a>
          <button type="button" className="btn sm" disabled={!on} onClick={() => setQr(true)}><Icon name="qr" size={14} />QR code</button>
        </div>
        {options && <div className="share-opts">{options}</div>}
        {onRenew && <p className="muted small">Shared with someone you should not have? <button type="button" className="linkish" onClick={async () => { await onRenew(); toast('A new link; the old one no longer works') }}>Make a new link</button> — the old one stops working.</p>}
      </section>
      <section className="card share-card">
        <h3>Send it</h3>
        <p className="muted small">Through your other Pocket apps. {on ? '' : 'Switch the share link on first.'}</p>
        <div className="share-apps">
          {apps.filter(a => !a.only || a.only.includes(what)).map(a => {
            const st = state(a.slug, a.scope)
            const needsLink = a.key !== 'calendar' && a.key !== 'quest'
            const off = !st.ok || (needsLink && !on)
            return (
              <button key={a.key} type="button" className="share-app" disabled={off} onClick={() => setSend(a.key)} title={!st.ok ? (st.app?.fix || `Connect ${st.app?.name || a.slug} in Settings → Connected apps`) : needsLink && !on ? 'Switch the share link on first' : a.label}>
                <Icon name={a.icon} size={18} /><span>{a.label}</span>
              </button>
            )
          })}
        </div>
      </section>
      {qr && <Modal title="Scan to play" onClose={() => setQr(false)}>
        <div className="qr-big"><QRCode value={url} size={260} label={`QR code for ${title}`} /></div>
        <p className="muted small center">{title}</p>
        <div className="row center"><button type="button" className="btn" onClick={() => window.print()}><Icon name="printer" size={14} />Print</button></div>
      </Modal>}
      {send && <SendModal how={send} what={what} id={id} title={title} onClose={() => setSend(null)} />}
    </div>
  )
}

function SendModal({ how, what, id, title, onClose }: { how: 'chat' | 'email' | 'family' | 'blog' | 'quest' | 'calendar'; what: ShareWhat; id: number; title: string; onClose: () => void }) {
  const { toast } = useToast()
  const [text, setText] = useState('')
  const [to, setTo] = useState('')
  const [busy, setBusy] = useState(false)
  const [convs, setConvs] = useState<{ id: string; name: string }[] | null>(null)
  const [camps, setCamps] = useState<{ id: number; name: string }[] | null>(null)
  const [people, setPeople] = useState<{ name: string; addr: string }[]>([])
  const [found, setFound] = useState<{ id: string; name: string; emails: string[] }[]>([])
  const [addr, setAddr] = useState('')
  const [date, setDate] = useState('')
  const [time, setTime] = useState('')
  useEffect(() => {
    if (how === 'chat') api.get<{ conversations: { id: string; name: string }[] }>(`${V1}/apps/chats`).then(r => { setConvs(r.conversations); if (r.conversations[0]) setTo(r.conversations[0].id) }).catch(e => { toast((e as Error).message, 'err'); setConvs([]) })
    if (how === 'quest') api.get<{ campaigns: { id: number; name: string }[] }>(`${V1}/apps/quest/campaigns`).then(r => { setCamps(r.campaigns); if (r.campaigns[0]) setTo(String(r.campaigns[0].id)) }).catch(e => { toast((e as Error).message, 'err'); setCamps([]) })
  }, [how]) // eslint-disable-line react-hooks/exhaustive-deps
  const search = async (q: string) => {
    setAddr(q)
    if (q.length < 2 || q.includes('@')) { setFound([]); return }
    try { const r = await api.get<{ contacts: { id: string; name: string; emails: string[] }[] }>(`${V1}/apps/contacts?q=${encodeURIComponent(q)}`); setFound(r.contacts.filter(c => c.emails.length)) } catch { setFound([]) }
  }
  const go = async () => {
    setBusy(true)
    try {
      let r: { text: string; url?: string }
      switch (how) {
        case 'chat': r = await api.post(`${V1}/apps/chat`, { what, id, text, to }); break
        case 'email': {
          const list = [...people]
          if (addr.includes('@')) list.push({ name: '', addr })
          r = await api.post(`${V1}/apps/email`, { what, id, text, to: list }); break
        }
        case 'family': r = await api.post(`${V1}/apps/family`, { what, id, text }); break
        case 'blog': r = await api.post(`${V1}/apps/blog`, { what, id, text }); break
        case 'quest': r = await api.post(`${V1}/apps/quest`, { what, id, campaign_id: Number(to) }); break
        case 'calendar': r = await api.post(`${V1}/apps/calendar`, { what, id, date, time }); break
      }
      toast(r.text, 'ok', openAction(r.url))
      onClose()
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  const heading = { chat: 'Send to a chat', email: 'Email it', family: 'Pin to the family board', blog: 'Start a blog post', quest: 'Hand it out in Pocket Quest', calendar: 'Put it in the calendar' }[how]
  const ready = how === 'email' ? people.length > 0 || addr.includes('@') : how === 'chat' ? !!convs : how === 'quest' ? !!to : how === 'calendar' ? !!date : true
  return (
    <Modal title={heading} onClose={onClose} footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={busy || !ready} onClick={go}>{busy && <Spinner />}{how === 'calendar' ? 'Add' : how === 'blog' ? 'Make the draft' : 'Send'}</button></>}>
      <p className="muted small">“{title}”</p>
      {how === 'chat' && (convs === null ? <Spinner /> : <Field label="Chat"><Select ariaLabel="Chat" value={to} onChange={setTo} options={[{ value: 'me', label: 'A note to myself' }, ...convs.map(c => ({ value: c.id, label: c.name || 'Chat' }))]} /></Field>)}
      {how === 'quest' && (camps === null ? <Spinner /> : camps.length === 0 ? <p className="muted">No campaigns in Pocket Quest yet.</p> : <Field label="Campaign"><Select ariaLabel="Campaign" value={to} onChange={setTo} options={camps.map(c => ({ value: String(c.id), label: c.name }))} /></Field>)}
      {how === 'email' && <Field label="To" help="Type a name to find them in Pocket Contacts, or an email address.">
        <div className="chips">{people.map(p => <span key={p.addr} className="chip on">{p.name || p.addr}<button type="button" className="chip-x" aria-label="Remove" onClick={() => setPeople(people.filter(x => x !== p))}><Icon name="x" size={12} /></button></span>)}</div>
        <input value={addr} onChange={e => search(e.target.value)} onKeyDown={e => { if (e.key === 'Enter' && addr.includes('@')) { setPeople([...people, { name: '', addr }]); setAddr('') } }} placeholder="Ana or ana@example.com" />
        {found.length > 0 && <div className="pick-list">{found.slice(0, 6).map(c => c.emails.map(em => <button key={c.id + em} type="button" className="pick-row" onClick={() => { setPeople([...people, { name: c.name, addr: em }]); setAddr(''); setFound([]) }}><Icon name="user" size={14} /><span className="grow">{c.name}</span><span className="muted small">{em}</span></button>))}</div>}
      </Field>}
      {how === 'calendar' && <div className="two-col tight"><Field label="Day"><input type="date" value={date} onChange={e => setDate(e.target.value)} /></Field><Field label="Time (optional)"><input type="time" value={time} onChange={e => setTime(e.target.value)} /></Field></div>}
      {(how === 'chat' || how === 'email' || how === 'family' || how === 'blog') && <Field label={how === 'blog' ? 'A line before the puzzle' : 'Message'}><textarea rows={3} value={text} onChange={e => setText(e.target.value)} placeholder={what === 'adventure' ? 'You are invited! Can your team escape in time?' : 'Can you solve this one?'} /></Field>}
    </Modal>
  )
}

/** A row with a switch (for the share options). */
export function ShareOption({ title, help, checked, onChange }: { title: string; help?: string; checked: boolean; onChange: (v: boolean) => void }) {
  return <SettingRow title={title} help={help}><Switch checked={checked} onChange={onChange} label={title} /></SettingRow>
}
