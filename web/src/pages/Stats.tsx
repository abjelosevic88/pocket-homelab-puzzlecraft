// Your times: puzzles solved, by kind, your run of days, the last twelve weeks.
import { useEffect, useState } from 'react'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { fmtMs, KIND } from '../lib/puzzlecraft'
import type { Kind } from '../lib/types'
import { Heatmap, HBars } from '../components/Chart'
import { Icon } from '../components/icons'
import { Empty, Loading } from '../components/ui'
import { KindIco } from './Today'

interface StatsDoc {
  solved: number; streak: number; best_streak: number; daily_days: number; players_solved: number
  kinds: { kind: Kind; kind_name: string; solved: number; avg_ms: number; best_ms: number; hints_per: number }[]
  days: { date: string; n: number }[]
}

export default function Stats() {
  const { settings } = useApp()
  const [s, setS] = useState<StatsDoc | null>(null)
  useEffect(() => { api.get<StatsDoc>(`${V1}/stats`).then(setS).catch(() => {}) }, [])
  if (!s) return <Loading />
  return (
    <div className="page">
      <div className="page-head"><div className="grow"><h2 className="page-title">Your times</h2><p className="muted">What you solved yourself — the puzzles of the day and your own.</p></div></div>
      <div className="stats-row">
        <div className="stat"><div className="stat-l">Solved</div><div className="stat-v">{s.solved}</div></div>
        <div className="stat"><div className="stat-l">Days in a row</div><div className="stat-v"><Icon name="flame" size={18} />{s.streak}</div><div className="stat-s muted small">best {s.best_streak}</div></div>
        <div className="stat"><div className="stat-l">Days with a puzzle of the day</div><div className="stat-v">{s.daily_days}</div></div>
        <div className="stat"><div className="stat-l">Your puzzles solved by others</div><div className="stat-v">{s.players_solved}</div></div>
      </div>
      {s.solved === 0 ? <Empty title="Nothing solved yet" icon="trophy">Today’s puzzles are waiting on the Today page.</Empty> : <>
        <section className="card pad-card">
          <h3>The last twelve weeks</h3>
          <Heatmap days={s.days.map(d => ({ date: d.date, minutes: d.n }))} weekStart={settings?.week_start || 1} fmt={n => `${n} solved`} />
        </section>
        <div className="home-cols">
          <section className="card">
            <div className="card-head"><h3>By kind</h3></div>
            <div className="table-wrap"><table className="tbl">
              <thead><tr><th>Kind</th><th className="num">Solved</th><th className="num">Usually</th><th className="num">Best</th><th className="num">Hints</th></tr></thead>
              <tbody>{s.kinds.map(k => <tr key={k.kind}><td><span className="row"><KindIco kind={k.kind} />{k.kind_name}</span></td><td className="num">{k.solved}</td><td className="num">{fmtMs(k.avg_ms)}</td><td className="num">{k.best_ms ? fmtMs(k.best_ms) : '—'}</td><td className="num">{k.hints_per.toFixed(1)}</td></tr>)}</tbody>
            </table></div>
          </section>
          <section className="card pad-card">
            <h3>What you play most</h3>
            <HBars items={s.kinds.map(k => ({ label: k.kind_name, value: k.solved, color: KIND[k.kind]?.tint }))} format={n => String(n)} />
          </section>
        </div>
      </>}
    </div>
  )
}
