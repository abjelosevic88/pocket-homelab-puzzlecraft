import type { Kind } from '../../lib/types'
import { Crossword } from './Crossword'
import { WordSearch } from './WordSearch'
import { Sudoku } from './Sudoku'
import { Nonogram } from './Nonogram'
import { Maze } from './Maze'
import { Cryptogram } from './Cryptogram'
import { Scramble } from './Scramble'
import { Quiz } from './Quiz'
import { Ladder } from './Ladder'
import { Logic } from './Logic'
import { Jigsaw } from './Jigsaw'
import type { KindModule } from './types'

export const KINDS_UI: Record<Kind, KindModule> = {
  crossword: Crossword, wordsearch: WordSearch, sudoku: Sudoku, nonogram: Nonogram, maze: Maze,
  cryptogram: Cryptogram, scramble: Scramble, quiz: Quiz, ladder: Ladder, logic: Logic, jigsaw: Jigsaw,
}
