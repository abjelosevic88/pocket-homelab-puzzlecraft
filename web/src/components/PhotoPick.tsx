// Choose a picture: from this device, or from Pocket Photos (greyed out,
// with what to do, when Photos is not connected).
import { useEffect, useRef, useState } from 'react'
import { api, V1 } from '../lib/api'
import { useAppState } from '../lib/data'
import { shrink, uploadPhoto } from '../lib/photo'
import { Icon } from './icons'
import { Modal, Spinner, useToast } from './ui'

export interface Picked { id: number; url: string; thumb_url: string; width?: number; height?: number }

export function PhotoPick({ kind = 'puzzle', label = 'Choose a picture', onPicked, small }: { kind?: string; label?: string; onPicked: (p: Picked) => void; small?: boolean }) {
  const { toast } = useToast()
  const file = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [browse, setBrowse] = useState(false)
  const state = useAppState()
  const photos = state('photos', 'photos:read')
  const up = async (f: File) => {
    setBusy(true)
    try { const s = await shrink(f, 2000); onPicked(await uploadPhoto(s, kind)) } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  return (
    <div className="row wrap photo-pick">
      <button type="button" className={`btn ${small ? 'sm' : ''}`} onClick={() => file.current?.click()} disabled={busy}>{busy ? <Spinner /> : <Icon name="upload" size={14} />}{label}</button>
      <button type="button" className={`btn ${small ? 'sm' : ''}`} onClick={() => setBrowse(true)} disabled={!photos.ok} title={photos.ok ? 'Pick one of your photos' : (photos.app?.fix || 'Connect Pocket Photos in Settings → Connected apps')}><Icon name="image" size={14} />From Pocket Photos</button>
      <input ref={file} type="file" accept="image/*" hidden onChange={e => { const f = e.target.files?.[0]; if (f) up(f); e.target.value = '' }} />
      {browse && <PhotosBrowser kind={kind} onClose={() => setBrowse(false)} onPicked={p => { setBrowse(false); onPicked(p) }} />}
    </div>
  )
}

function PhotosBrowser({ kind, onClose, onPicked }: { kind: string; onClose: () => void; onPicked: (p: Picked) => void }) {
  const { toast } = useToast()
  const [q, setQ] = useState('')
  const [list, setList] = useState<{ id: string; name: string; thumb_url: string }[] | null>(null)
  const [busy, setBusy] = useState('')
  const load = (query: string) => { setList(null); api.get<{ photos: { id: string; name: string; thumb_url: string }[] }>(`${V1}/apps/photos${query ? `?q=${encodeURIComponent(query)}` : ''}`).then(r => setList(r.photos)).catch(e => { toast((e as Error).message, 'err'); setList([]) }) }
  useEffect(() => { load('') }, []) // eslint-disable-line react-hooks/exhaustive-deps
  const take = async (id: string) => {
    setBusy(id)
    try { onPicked(await api.post<Picked>(`${V1}/apps/photos/${id}/copy?kind=${kind}`)) } catch (e) { toast((e as Error).message, 'err') } finally { setBusy('') }
  }
  return (
    <Modal title="Pocket Photos" onClose={onClose} size="lg">
      <div className="row"><input className="grow" value={q} onChange={e => setQ(e.target.value)} placeholder="Search your photos (beach, Ana, birthday…)" onKeyDown={e => { if (e.key === 'Enter') load(q) }} /><button type="button" className="btn" onClick={() => load(q)}><Icon name="search" size={14} />Search</button></div>
      {list === null ? <Spinner /> : list.length === 0 ? <p className="muted">No photos found.</p> :
        <div className="photo-grid">{list.map(p => <button key={p.id} type="button" className="photo-tile" onClick={() => take(p.id)} disabled={!!busy} title={p.name}><img src={p.thumb_url} alt={p.name} loading="lazy" />{busy === p.id && <span className="photo-busy"><Spinner /></span>}</button>)}</div>}
    </Modal>
  )
}
