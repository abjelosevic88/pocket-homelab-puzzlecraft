export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = '' | 'core' | 'direct'
export type AppSlug = 'calendar' | 'contacts' | 'chat' | 'email' | 'photos' | 'files' | 'notes' | 'tasks' | 'inventory' | 'places' | 'map'
  | 'learn' | 'bookshelf' | 'habits' | 'quest' | 'blog' | 'family' | 'translate' | 'alerts'

type ConnFields = { [K in AppSlug as `${K}_mode`]: ConnMode } & { [K in AppSlug as `${K}_via`]: ConnVia } & { [K in AppSlug as `${K}_detail` | `${K}_url` | `${K}_token` | `${K}_public_url` | `${K}_open_url`]: string } & { [K in AppSlug as `${K}_configured`]: boolean }

export type Lang = 'en' | 'bcs' | 'other'
export type Level = 'easy' | 'medium' | 'hard' | 'expert'

export type Settings = ConnFields & {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number
  language: Lang; level: Level; paper_size: 'a4' | 'letter'; large_print: boolean; show_mistake: boolean; daily: boolean; daily_level: '' | Level; share_board: boolean
  notes_folder: string; files_folder: string; chat_to: string; habit_id: number; learn_deck: string; map_list: string; alert_plays: boolean; ai_help: boolean; ai_photos: boolean
  notify_plays: boolean; notify_daily: boolean; notify_prep: boolean; notify_hour: number
  notify_url: string; notify_format: string; notify_configured: boolean
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string; demo: boolean
}

export type AppStateName = 'on' | 'partly' | 'off' | 'link_off' | 'missing' | 'old' | 'no_core' | 'not_set_up' | 'error'
export interface AppState { slug: AppSlug; name: string; reason: string; mode: ConnMode; via: ConnVia; state: AppStateName; detail: string; fix: string; on: string[]; open_url: string }

// ---- puzzles ----

export type Kind = 'crossword' | 'wordsearch' | 'sudoku' | 'nonogram' | 'cryptogram' | 'scramble' | 'quiz' | 'maze' | 'logic' | 'ladder' | 'jigsaw'

export interface WordClue { word: string; clue: string }

export interface Puzzle {
  id: number; kind: Kind; kind_name?: string; title: string; instructions: string; notes: string; level: Level; language: Lang; tags: string[]
  data?: any; size_text: string; ready: boolean; problems: number; tested_at: string | null; photo_id: number | null
  share_token: string; share_on: boolean; board: boolean; reveal: boolean; timer: boolean; source: string; source_ref: string; pinned: boolean; archived: boolean
  created_at: string; updated_at: string; plays: number; solves: number; best_ms: number
  share_url?: string; url?: string; uses?: string[]; mine?: { finished_at: string | null; ms: number; hints: number; started_at: string }
  entries?: Entry[]; message?: string; missing?: string[]
}

export interface Entry { key: string; num: number; dir: 'A' | 'D'; cells: number[]; answer?: string; clue: string }

export interface Report { ok: boolean; problems: string[]; notes: string[]; level?: string; stats?: Record<string, any> }

/** The result of checking what a player filled in. */
export interface CheckResult {
  solved: boolean; wrong?: number[]; right?: number[]; found?: number; filled: number; total: number; score?: number; max?: number
  explain?: string[]; answers?: string[]; message?: string
}

/** A puzzle as a player gets it (no answer). */
export interface PublicPuzzle {
  id?: number; kind: Kind; kind_name: string; title: string; instructions?: string; level: string; language?: Lang; size_text: string
  timer: boolean; reveal: boolean; board?: boolean; data: any; photo?: boolean; daily?: string
}

export interface BoardRow { player: string; ms: number; hints: number; mistakes: number; score: number; max_score: number; finished_at: string; time: string }

/** One go at something, as the player's browser has it. */
export interface PlayView {
  key: string; what: 'puzzle' | 'adventure' | 'daily'; kind: Kind | ''; player: string; started_at: string; finished_at: string | null; ms: number
  hints: number; mistakes: number; score: number; max_score: number; progress: any; shown: number[] | null; title: string; server_now: string; via: string
  puzzle?: PublicPuzzle; adventure?: AdventureView
}

export interface AdventureView {
  id: number; kind: AdventureKind; title: string; intro: string; theme: string; minutes: number; hint_penalty: number; stages: number; done: number
  finished: boolean; over: boolean; photo: boolean; finale?: string; best?: BoardRow[]; past: { title: string; unlock: string }[]
  stage?: {
    id: number; n: number; title: string; story: string; lock: Lock; lock_hint: string; photo: boolean; photo_id: number | null; place_name: string
    lat: number | null; lng: number | null; hints: string[]; hints_left: number; next_hint_in: number; started_at: string; length: number
    puzzle?: PublicPuzzle; colors?: string[]
  }
}

export interface Play {
  id: number; what: string; ref_id: number; kind: string; day: string; via: string; player: string; started_at: string; finished_at: string | null
  ms: number; hints: number; mistakes: number; score: number; max_score: number; stage: number; last_at: string
}

export interface Book {
  id: number; title: string; subtitle: string; notes: string; answers: 'back' | 'none'; large: boolean; share_token: string; share_on: boolean
  created_at: string; updated_at: string; items: { id: number; puzzle_id: number; sort: number; title: string; kind: Kind; level: string; size_text: string }[]; share_url?: string
}

export interface WordList { id: number; name: string; language: Lang; words: WordClue[]; source: string; created_at: string; updated_at: string; count: number }

export type AdventureKind = 'escape' | 'hunt' | 'mystery' | 'party'
export type Lock = 'answer' | 'code' | 'word' | 'directions' | 'colors' | 'qr' | 'puzzle' | 'none'
export interface PrepItem { id: string; text: string; done: boolean; task_url?: string; task_id?: string }
export interface Hint { text: string; after: number }
export interface Stage {
  id: number; adventure_id: number; sort: number; title: string; story: string; photo_id: number | null; lock: Lock; puzzle_id: number | null
  answers: string[]; lock_hint: string; hints: Hint[]; hidden: string; place_name: string; lat: number | null; lng: number | null; place_ref: string
  unlock: string; props: string; code: string; created_at: string; puzzle_title?: string; puzzle_kind?: string
}
export interface Adventure {
  id: number; kind: AdventureKind; title: string; intro: string; finale: string; theme: string; date: string; time: string; place: string; players: string
  minutes: number; hint_penalty: number; status: 'draft' | 'ready' | 'done'; notes: string; prep: PrepItem[]; share_token: string; share_on: boolean; board: boolean
  calendar_ref: string; calendar_url: string; photo_id: number | null; source: string; created_at: string; updated_at: string
  stages?: Stage[]; stages_n: number; plays: number; live: number; share_url?: string; cards?: { stage: number; url: string }[]
}
