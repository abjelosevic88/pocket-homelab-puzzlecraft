// A small, safe Markdown reader for recaps, notes and the rules text:
// headings, paragraphs, lists, bold and italic, tables. No HTML gets in.
import React from 'react'

function inline(text: string, key = ''): React.ReactNode[] {
  const out: React.ReactNode[] = []
  const re = /(\*\*\*[^*]+\*\*\*|\*\*[^*]+\*\*|\*[^*\s][^*]*\*|_[^_\s][^_]*_)/g
  let last = 0
  let m: RegExpExecArray | null
  let i = 0
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index))
    const t = m[0]
    if (t.startsWith('***')) out.push(<b key={key + i++}><i>{t.slice(3, -3)}</i></b>)
    else if (t.startsWith('**')) out.push(<b key={key + i++}>{t.slice(2, -2)}</b>)
    else out.push(<i key={key + i++}>{t.slice(1, -1)}</i>)
    last = m.index + t.length
  }
  if (last < text.length) out.push(text.slice(last))
  return out
}

export function Markdown({ text, className = '', skipFirstHeading = false }: { text: string; className?: string; skipFirstHeading?: boolean }) {
  const lines = (text || '').replace(/\r/g, '').split('\n')
  const blocks: React.ReactNode[] = []
  let i = 0
  let k = 0
  let headingSeen = false
  while (i < lines.length) {
    const l = lines[i]
    if (!l.trim()) { i++; continue }
    const h = /^(#{1,6})\s+(.*)$/.exec(l)
    if (h) {
      i++
      if (skipFirstHeading && !headingSeen) { headingSeen = true; continue }
      headingSeen = true
      const lvl = Math.min(6, h[1].length + 1)
      const Tag = `h${lvl}` as 'h2'
      blocks.push(<Tag key={k++} className="md-h">{inline(h[2])}</Tag>)
      continue
    }
    if (/^\|.*\|\s*$/.test(l)) {
      const rows: string[][] = []
      while (i < lines.length && /^\|.*\|\s*$/.test(lines[i])) {
        const cells = lines[i].trim().slice(1, -1).split('|').map(c => c.trim())
        if (!cells.every(c => /^:?-{2,}:?$/.test(c))) rows.push(cells)
        i++
      }
      const [head, ...body] = rows
      blocks.push(<div key={k++} className="md-table-wrap"><table className="md-table"><thead><tr>{head.map((c, j) => <th key={j}>{inline(c)}</th>)}</tr></thead>
        <tbody>{body.map((r, j) => <tr key={j}>{r.map((c, n) => <td key={n}>{inline(c)}</td>)}</tr>)}</tbody></table></div>)
      continue
    }
    if (/^\s*([-*•]|\d+\.)\s+/.test(l)) {
      const ordered = /^\s*\d+\./.test(l)
      const items: string[] = []
      while (i < lines.length && /^\s*([-*•]|\d+\.)\s+/.test(lines[i])) { items.push(lines[i].replace(/^\s*([-*•]|\d+\.)\s+/, '')); i++ }
      const L = ordered ? 'ol' : 'ul'
      blocks.push(<L key={k++} className="md-list">{items.map((t, j) => <li key={j}>{inline(t, `${k}-${j}-`)}</li>)}</L>)
      continue
    }
    const para: string[] = []
    while (i < lines.length && lines[i].trim() && !/^(#{1,6}\s|\||\s*([-*•]|\d+\.)\s)/.test(lines[i])) { para.push(lines[i].trim()); i++ }
    blocks.push(<p key={k++}>{inline(para.join(' '), `${k}-`)}</p>)
  }
  return <div className={`md ${className}`}>{blocks}</div>
}
