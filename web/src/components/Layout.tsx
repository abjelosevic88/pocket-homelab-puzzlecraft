// The app shell: sidebar, top bar, search palette, shortcuts — the Pocket Calendar layout.
import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { DataProvider } from '../lib/data'
import { Icon } from './icons'
import { Sidebar } from './Sidebar'
import { CommandPalette, type PaletteCommand } from './CommandPalette'
import { ShortcutsSheet } from './ShortcutsSheet'
import { Avatar, type MenuItem, MenuButton, modalOpen, useIsMobile, useMediaQuery } from './ui'
import { AskButton } from './AskButton'
import { KINDS } from '../lib/puzzlecraft'

export { announceChange, useOnChange } from '../lib/data'

const SIDEBAR_KEY = 'ppzc-sidebar'

/** The page's own name for the top bar breadcrumb. */
const CrumbCtx = createContext<(t: string, to?: string) => void>(() => {})
export function useCrumb(title: string | undefined, parent?: string) {
  const set = useContext(CrumbCtx)
  useEffect(() => { set(title || '', parent); return () => set('') }, [title, parent, set])
}

export type CreateKind = 'puzzle' | 'adventure' | 'book' | 'words' | 'import'

/** Start something from anywhere (a page's button). */
export const openCreate = (k: CreateKind) => window.dispatchEvent(new CustomEvent('ppzc:create', { detail: { kind: k } }))

/** The "new" menu: the sidebar's caret. */
export function createItems(onCreate: (k: CreateKind) => void): MenuItem[] {
  return [
    { label: 'New puzzle', icon: 'puzzle', hint: 'N', onSelect: () => onCreate('puzzle') },
    { label: 'Escape room or treasure hunt', icon: 'key', hint: '⇧N', onSelect: () => onCreate('adventure') },
    { label: 'Puzzle book', icon: 'book-open', onSelect: () => onCreate('book') },
    { label: 'Word list', icon: 'list', onSelect: () => onCreate('words') },
    'sep',
    { label: 'Open a crossword file or a sudoku', icon: 'upload', onSelect: () => onCreate('import') },
  ]
}

export default function Layout() {
  return <DataProvider><Shell /></DataProvider>
}

function Shell() {
  const navigate = useNavigate()
  const loc = useLocation()
  const mobile = useIsMobile()
  const narrow = useMediaQuery('(max-width: 1099px)')
  const drawer = mobile || narrow
  const [sidebarOpen, setSidebarOpen] = useState(() => !drawer && localStorage.getItem(SIDEBAR_KEY) !== '0')
  useEffect(() => { setSidebarOpen(!drawer && localStorage.getItem(SIDEBAR_KEY) !== '0') }, [drawer])
  const drawerOpenedAt = useRef(0)
  const toggleSidebar = () => setSidebarOpen(o => { drawerOpenedAt.current = performance.now(); const n = !o; if (!drawer) localStorage.setItem(SIDEBAR_KEY, n ? '1' : '0'); return n })
  const closeDrawer = () => { if (drawer) setSidebarOpen(false) }
  useEffect(() => { document.querySelector('.content')?.scrollTo({ top: 0 }) }, [loc.pathname])

  const [crumb, setCrumbRaw] = useState<{ t: string; to?: string }>({ t: '' })
  const setCrumb = useCallback((t: string, to?: string) => setCrumbRaw({ t, to }), [])

  const [palette, setPalette] = useState(false)
  const [shortcuts, setShortcuts] = useState(false)

  const create = (k: CreateKind) => {
    closeDrawer()
    switch (k) {
      case 'puzzle': navigate('/new'); break
      case 'adventure': navigate('/adventures?new=1'); break
      case 'book': navigate('/books?new=1'); break
      case 'words': navigate('/words?new=1'); break
      case 'import': navigate('/new?how=file'); break
    }
  }
  const createRef = useRef(create)
  createRef.current = create

  useEffect(() => {
    const open = () => setShortcuts(true)
    const make = (e: Event) => createRef.current((e as CustomEvent<{ kind: CreateKind }>).detail.kind)
    window.addEventListener('ppzc:shortcuts', open)
    window.addEventListener('ppzc:create', make)
    return () => { window.removeEventListener('ppzc:shortcuts', open); window.removeEventListener('ppzc:create', make) }
  }, [])
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); setPalette(p => !p); return }
      if (e.metaKey || e.ctrlKey || e.altKey) return
      if (t.closest('input, textarea, select, [contenteditable="true"]') || modalOpen() || document.querySelector('.menu, .popover, .sel-pop')) return
      const go = (to: string) => { e.preventDefault(); navigate(to) }
      const inEditor = /^\/(puzzles|adventures|books)\/\d+/.test(loc.pathname) || /^\/(today|play)\//.test(loc.pathname)
      if (inEditor && e.key.length === 1 && /\p{L}/u.test(e.key) && e.key !== 'N') return
      switch (e.key) {
        case '/': e.preventDefault(); setPalette(true); break
        case '?': e.preventDefault(); setShortcuts(true); break
        case 'n': e.preventDefault(); createRef.current('puzzle'); break
        case 'N': e.preventDefault(); createRef.current('adventure'); break
        case 't': go('/'); break
        case 'p': go('/puzzles'); break
        case 'e': go('/adventures'); break
        case 'b': go('/books'); break
        case 'w': go('/words'); break
        case 's': go('/stats'); break
        case ',': go('/settings'); break
        case '[': e.preventDefault(); toggleSidebar(); break
      }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }) // eslint-disable-line react-hooks/exhaustive-deps

  const commands: PaletteCommand[] = [
    { id: 'new', label: 'New puzzle', icon: 'puzzle', hint: 'N', keywords: 'make create crossword sudoku word search quiz', run: () => create('puzzle') },
    { id: 'adv', label: 'New escape room or treasure hunt', icon: 'key', hint: '⇧N', keywords: 'hunt mystery party locks', run: () => create('adventure') },
    { id: 'book', label: 'New puzzle book', icon: 'book-open', keywords: 'print booklet collection', run: () => create('book') },
    { id: 'words', label: 'New word list', icon: 'list', keywords: 'words clues vocabulary', run: () => create('words') },
    { id: 'import', label: 'Open a crossword file (.ipuz, .puz) or a sudoku', icon: 'upload', keywords: 'import across lite', run: () => create('import') },
    { id: 'today', label: 'Today', icon: 'sun', hint: 'T', keywords: 'daily puzzle of the day streak', run: () => navigate('/') },
    { id: 'puzzles', label: 'Puzzles', icon: 'puzzle', hint: 'P', run: () => navigate('/puzzles') },
    { id: 'advs', label: 'Escape rooms and hunts', icon: 'key', hint: 'E', run: () => navigate('/adventures') },
    { id: 'books', label: 'Puzzle books', icon: 'book-open', hint: 'B', run: () => navigate('/books') },
    { id: 'lists', label: 'Word lists', icon: 'list', hint: 'W', run: () => navigate('/words') },
    { id: 'stats', label: 'Your times', icon: 'chart', hint: 'S', keywords: 'stats statistics streak', run: () => navigate('/stats') },
    ...KINDS.map(k => ({ id: 'k-' + k.kind, label: `New ${k.name.toLowerCase()}`, icon: k.icon, keywords: k.line, run: () => navigate(`/new?kind=${k.kind}`) })),
    { id: 'settings', label: 'Settings', icon: 'settings', hint: ',', run: () => navigate('/settings') },
    { id: 'apps', label: 'Settings: connected apps', icon: 'link', keywords: 'calendar chat email photos files notes tasks learn quest blog family core', run: () => navigate('/settings/apps') },
    { id: 'ai-set', label: 'Settings: AI', icon: 'sparkles', keywords: 'pocket ai openai', run: () => navigate('/settings/ai') },
    { id: 'export', label: 'Settings: export and import', icon: 'download', keywords: 'backup json csv ipuz', run: () => navigate('/settings/data') },
    { id: 'shortcuts', label: 'Keyboard shortcuts', icon: 'keyboard', hint: '?', run: () => setShortcuts(true) },
  ]

  return (
    <CrumbCtx.Provider value={setCrumb}>
      <div className={`app ${sidebarOpen ? 'with-sidebar' : 'no-sidebar'} ${mobile ? 'is-mobile' : ''} ${drawer ? 'drawer-mode' : ''}`}>
        {drawer && sidebarOpen && <div className="drawer-backdrop" onClick={e => { if (e.timeStamp - drawerOpenedAt.current > 400) setSidebarOpen(false) }} />}
        <div className={`sidebar-wrap ${sidebarOpen ? 'open' : ''}`}>
          <Sidebar onCreate={create} onNavigate={closeDrawer} />
        </div>
        <div className="main">
          <TopBar onMenu={toggleSidebar} onPalette={() => setPalette(true)} mobile={mobile} compact={narrow} sidebarOpen={sidebarOpen} crumb={crumb} />
          <main className="content"><div className="content-inner"><Outlet /></div></main>
        </div>
        {mobile && /^\/(|puzzles|adventures|books|words)$/.test(loc.pathname) && <button type="button" className="fab" aria-label="New" title="New" onClick={() => create(loc.pathname.startsWith('/adventures') ? 'adventure' : loc.pathname.startsWith('/books') ? 'book' : loc.pathname.startsWith('/words') ? 'words' : 'puzzle')}><Icon name="plus" size={26} strokeWidth={2} /></button>}
        {palette && <CommandPalette onClose={() => setPalette(false)} commands={commands} />}
        {shortcuts && <ShortcutsSheet onClose={() => setShortcuts(false)} />}
      </div>
    </CrumbCtx.Provider>
  )
}

const SECTION: Record<string, [string, string]> = {
  '': ['Today', '/'], today: ['Today', '/'], puzzles: ['Puzzles', '/puzzles'], new: ['Puzzles', '/puzzles'], adventures: ['Escape rooms & hunts', '/adventures'],
  books: ['Puzzle books', '/books'], words: ['Word lists', '/words'], stats: ['Your times', '/stats'],
}

function TopBar({ onMenu, onPalette, mobile, compact, sidebarOpen, crumb }: { onMenu: () => void; onPalette: () => void; mobile: boolean; compact: boolean; sidebarOpen: boolean; crumb: { t: string; to?: string } }) {
  const { user, themeMode, setThemeMode, status, setUser } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [section, sectionTo] = SECTION[loc.pathname.split('/')[1] || ''] || ['', '/']
  const signOut = async () => { await api.post(`${V1}/auth/logout`).catch(() => {}); setUser(null); navigate('/login') }
  const avatarMenu = (): MenuItem[] => [
    { heading: user?.name || user?.email || '' },
    { label: 'Profile', icon: 'user', onSelect: () => navigate('/settings/profile') },
    { label: 'Settings', icon: 'settings', onSelect: () => navigate('/settings'), hint: ',' },
    { label: 'Keyboard shortcuts', icon: 'keyboard', onSelect: () => window.dispatchEvent(new CustomEvent('ppzc:shortcuts')), hint: '?' },
    'sep',
    { heading: 'Appearance' },
    { label: 'System', checked: themeMode === 'system', onSelect: () => setThemeMode('system') },
    { label: 'Light', checked: themeMode === 'light', onSelect: () => setThemeMode('light') },
    { label: 'Dark', checked: themeMode === 'dark', onSelect: () => setThemeMode('dark') },
    ...(status?.auth_mode === 'local' ? ['sep' as const, { label: 'Sign out', icon: 'log-out', onSelect: signOut }] : []),
  ]
  return (
    <header className="topbar">
      <button type="button" className="btn ghost icon-only" onClick={onMenu} aria-label={sidebarOpen ? 'Hide the sidebar' : 'Show the sidebar'} title="Menu ([)"><Icon name={mobile || compact ? 'menu' : 'sidebar'} size={18} /></button>
      <h1 className="range-title">
        {crumb.t ? <><button type="button" className="crumb" onClick={() => navigate(crumb.to || sectionTo)}>{section}</button><Icon name="chevron-right" size={16} className="crumb-sep" /><span className="ellipsis">{crumb.t}</span></> : section}
      </h1>
      <span className="spacer" />
      <button type="button" className="btn ghost icon-only" onClick={onPalette} aria-label="Search" title="Search (⌘K or /)"><Icon name="search" size={18} /></button>
      <AskButton />
      <MenuButton label="Account" className="avatar-btn" align="right" width={220} items={avatarMenu}>
        <Avatar name={user?.name} email={user?.email} size={30} />
      </MenuButton>
    </header>
  )
}
