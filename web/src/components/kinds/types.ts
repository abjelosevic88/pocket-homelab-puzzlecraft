// What every kind of puzzle provides: a way to play it, to make it, and to print it.
import type React from 'react'
import type { CheckResult, Lang, Puzzle, PublicPuzzle, Settings } from '../../lib/types'

/** The buttons a puzzle puts in the player's bar (all optional). */
export interface PlayActions {
  check?: () => void          // "Check": mark what is wrong
  reveal?: () => void         // "Give me a letter" etc.
  revealLabel?: string
  clear?: () => void          // start again (clears the grid)
}

export interface KindPlayProps {
  view: PublicPuzzle                       // what the player sees (no answer): view.data is the kind's public shape
  progress: any                            // what they filled in last time (the kind's own shape), or null
  shown: number[]                          // parts given away so far
  done: boolean                            // solved: show it, read-only
  showMistakes: boolean                    // mark wrong squares as soon as they are typed (needs a check call)
  onProgress: (p: any) => void             // keep what they filled in (saved for them, debounced)
  check: (answer: any, final?: boolean) => Promise<CheckResult | null> // ask the server; solved → the player shows the end
  reveal: (what: number) => Promise<any | null>                         // give one part away (counts as a hint)
  canReveal: boolean
  photoURL: (id: number, thumb?: boolean) => string
  setActions: (a: PlayActions) => void     // the bar's buttons for this puzzle
}

export interface KindEditProps {
  puzzle: Puzzle            // the saved puzzle (title, language, level…)
  data: any                 // the definition being edited (with the answer)
  onChange: (data: any) => void
  settings: Settings | null
  language: Lang
}

export interface KindPrintProps {
  view: PublicPuzzle        // the puzzle as players see it
  answer?: any              // the full definition: print the answer key from it
  large?: boolean
}

export interface KindModule {
  Play: React.FC<KindPlayProps>
  Edit: React.FC<KindEditProps>
  Print: React.FC<KindPrintProps>
}
