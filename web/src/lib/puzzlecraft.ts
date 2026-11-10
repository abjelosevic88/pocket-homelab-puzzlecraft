// Names and icons: kinds of puzzle, the Pocket apps Puzzlecraft reaches, app states in words.
import type { AdventureKind, AppSlug, AppStateName, Kind, Level, Lock } from './types'

export const APP_INFO: Record<AppSlug, { name: string; icon: string }> = {
  calendar: { name: 'Pocket Calendar', icon: 'calendar' },
  contacts: { name: 'Pocket Contacts', icon: 'users' },
  chat: { name: 'Pocket Chat', icon: 'message' },
  email: { name: 'Pocket Email', icon: 'mail' },
  photos: { name: 'Pocket Photos', icon: 'image' },
  files: { name: 'Pocket Files', icon: 'file' },
  notes: { name: 'Pocket Notes', icon: 'note' },
  tasks: { name: 'Pocket Tasks', icon: 'check-square' },
  inventory: { name: 'Pocket Inventory', icon: 'package' },
  places: { name: 'Pocket Places', icon: 'pin' },
  map: { name: 'Pocket Map', icon: 'map' },
  learn: { name: 'Pocket Learn', icon: 'brain' },
  bookshelf: { name: 'Pocket Bookshelf', icon: 'book-open' },
  habits: { name: 'Pocket Habits', icon: 'habit' },
  quest: { name: 'Pocket Quest', icon: 'd20' },
  blog: { name: 'Pocket Blog', icon: 'pen' },
  family: { name: 'Pocket Family', icon: 'home' },
  translate: { name: 'Pocket Translate', icon: 'globe' },
  alerts: { name: 'Pocket Alerts', icon: 'bell' },
}

export const STATE_WORDS: Record<AppStateName, string> = {
  on: 'connected', partly: 'partly connected', off: 'switched off', link_off: 'switched off in Pocket Core', missing: 'not on this box',
  old: 'needs an update', no_core: 'no Pocket Core', not_set_up: 'not set up', error: 'not reachable',
}

export const KINDS: { kind: Kind; name: string; icon: string; line: string; tint: string }[] = [
  { kind: 'crossword', name: 'Crossword', icon: 'crossword', line: 'Your own grid, or your words criss-crossed for you', tint: '#1f7a8c' },
  { kind: 'wordsearch', name: 'Word search', icon: 'search', line: 'Hidden words, maybe a secret message in the leftover letters', tint: '#c2410c' },
  { kind: 'sudoku', name: 'Sudoku', icon: 'hash', line: 'Four levels, 4 × 4 for children up to expert 9 × 9', tint: '#4f46e5' },
  { kind: 'nonogram', name: 'Picture puzzle', icon: 'image', line: 'Numbers that reveal a picture (nonogram)', tint: '#0f766e' },
  { kind: 'cryptogram', name: 'Secret code', icon: 'lock', line: 'A message in code: letters, numbers or symbols', tint: '#7c3aed' },
  { kind: 'scramble', name: 'Word scramble', icon: 'shuffle', line: 'Mixed-up words, with a final answer from circled letters', tint: '#b45309' },
  { kind: 'quiz', name: 'Quiz', icon: 'help', line: 'Questions, riddles or a pub quiz with points', tint: '#be123c' },
  { kind: 'maze', name: 'Maze', icon: 'maze', line: 'Find the way from the green square to the red one', tint: '#15803d' },
  { kind: 'logic', name: 'Logic grid', icon: 'brain', line: 'Who had which? Work it out from the clues', tint: '#9333ea' },
  { kind: 'ladder', name: 'Word ladder', icon: 'stairs', line: 'COLD to WARM, one letter at a time', tint: '#0369a1' },
  { kind: 'jigsaw', name: 'Jigsaw', icon: 'puzzle', line: 'A photo of yours cut into pieces', tint: '#a16207' },
]
export const KIND: Record<Kind, (typeof KINDS)[number]> = Object.fromEntries(KINDS.map(k => [k.kind, k])) as any

export const LEVELS: { value: Level; label: string }[] = [
  { value: 'easy', label: 'Easy' }, { value: 'medium', label: 'Medium' }, { value: 'hard', label: 'Hard' }, { value: 'expert', label: 'Expert' },
]
export const LEVEL_NAME: Record<string, string> = { easy: 'Easy', medium: 'Medium', hard: 'Hard', expert: 'Expert' }

export const LANGS = [{ value: 'en', label: 'English' }, { value: 'bcs', label: 'Bosnian / Croatian / Serbian' }, { value: 'other', label: 'Another language' }] as const

export const ADV_KINDS: { kind: AdventureKind; name: string; icon: string; line: string }[] = [
  { kind: 'escape', name: 'Escape room', icon: 'key', line: 'Locks to open one after the other, against the clock' },
  { kind: 'hunt', name: 'Treasure hunt', icon: 'map', line: 'Stops to find around town or the garden' },
  { kind: 'mystery', name: 'Mystery', icon: 'search', line: 'Clues around the house that lead to the answer' },
  { kind: 'party', name: 'Party game', icon: 'gift', line: 'A hunt for children at a birthday' },
]
export const ADV_KIND: Record<AdventureKind, (typeof ADV_KINDS)[number]> = Object.fromEntries(ADV_KINDS.map(k => [k.kind, k])) as any

export const LOCKS: { value: Lock; label: string; line: string }[] = [
  { value: 'answer', label: 'Answer', line: 'Type a word or a phrase' },
  { value: 'code', label: 'Number lock', line: 'Digits, like a padlock' },
  { value: 'word', label: 'Word lock', line: 'Letter wheels' },
  { value: 'directions', label: 'Direction lock', line: 'Up, down, left, right' },
  { value: 'colors', label: 'Colour lock', line: 'Colours in the right order' },
  { value: 'qr', label: 'Find the card', line: 'Scan the QR card hidden there' },
  { value: 'puzzle', label: 'Solve a puzzle', line: 'One of your puzzles' },
  { value: 'none', label: 'Just read', line: 'A story stage, nothing to open' },
]

/** 4:05 or 1:02:10 */
export function fmtMs(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000))
  const h = Math.floor(s / 3600), m = Math.floor(s / 60) % 60, x = s % 60
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${String(x).padStart(2, '0')}` : `${m}:${String(x).padStart(2, '0')}`
}

/** Letters of a word as squares (LJ, NJ, DŽ are one in Bosnian/Croatian/Serbian). */
export function letters(word: string, lang = 'en'): string[] {
  const rs = Array.from(word.normalize('NFC').toUpperCase())
  const out: string[] = []
  for (let i = 0; i < rs.length; i++) {
    const r = rs[i]
    if (!/[\p{L}\p{N}]/u.test(r)) continue
    if (lang === 'bcs' && i + 1 < rs.length) {
      const p = r + rs[i + 1]
      if (p === 'LJ' || p === 'NJ' || p === 'DŽ') { out.push(p); i++; continue }
    }
    out.push(r)
  }
  return out
}

export const shareURL = (path: string) => `${location.origin}${path}`
