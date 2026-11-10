package puzzle

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Kinds of puzzle.
var Kinds = []string{"crossword", "wordsearch", "sudoku", "nonogram", "cryptogram", "scramble", "quiz", "maze", "logic", "ladder", "jigsaw"}

// KindName is how a kind is called on the page.
var KindName = map[string]string{
	"crossword": "Crossword", "wordsearch": "Word search", "sudoku": "Sudoku", "nonogram": "Picture puzzle",
	"cryptogram": "Secret code", "scramble": "Word scramble", "quiz": "Quiz", "maze": "Maze", "logic": "Logic grid",
	"ladder": "Word ladder", "jigsaw": "Jigsaw",
}

// Levels of difficulty.
var Levels = []string{"easy", "medium", "hard", "expert"}

// ErrBad is a definition that cannot be a puzzle.
var ErrBad = errors.New("not a puzzle")

func bad(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrBad, fmt.Sprintf(format, a...))
}

// Crossword: Cells row by row — "#" a black square, "" an empty white one,
// else its letter (or LJ/NJ/DŽ). Clues by key: "1A" = 1 across, "2D" = 2 down.
type Crossword struct {
	W        int               `json:"w"`
	H        int               `json:"h"`
	Cells    []string          `json:"cells"`
	Circles  []int             `json:"circles,omitempty"`
	Clues    map[string]string `json:"clues"`
	Style    string            `json:"style"`    // classic (black squares) | freeform (words criss-crossing, no black)
	Symmetry string            `json:"symmetry"` // rotational | mirror | none (what the editor keeps)
	Language string            `json:"language"`
}

// Entry is one word of a crossword.
type Entry struct {
	Key    string `json:"key"` // 1A
	Num    int    `json:"num"`
	Dir    string `json:"dir"` // A | D
	Cells  []int  `json:"cells"`
	Answer string `json:"answer,omitempty"`
	Clue   string `json:"clue"`
}

// WordSearch: a grid of letters with the words hidden in it.
type WordSearch struct {
	W        int      `json:"w"`
	H        int      `json:"h"`
	Grid     []string `json:"grid"` // row by row, one letter per square
	Words    []WSWord `json:"words"`
	Level    string   `json:"level"`   // easy: across and down · medium: + diagonals · hard: + backwards
	Message  string   `json:"message"` // spelled by the squares no word uses, in reading order
	Clues    bool     `json:"clues"`   // show the clues, not the words
	Language string   `json:"language"`
}

// WSWord is a hidden word: where it starts and which way it runs.
type WSWord struct {
	Word string `json:"word"`
	Clue string `json:"clue,omitempty"`
	R    int    `json:"r"`
	C    int    `json:"c"`
	DR   int    `json:"dr"`
	DC   int    `json:"dc"`
	Len  int    `json:"len"`
	Off  bool   `json:"off,omitempty"` // did not fit
}

// Sudoku: Givens and Solution row by row, 0 = empty.
type Sudoku struct {
	Size     int    `json:"size"` // 4, 6 or 9
	BoxW     int    `json:"box_w"`
	BoxH     int    `json:"box_h"`
	Givens   []int  `json:"givens"`
	Solution []int  `json:"solution"`
	Level    string `json:"level"`
	Diagonal bool   `json:"diagonal,omitempty"` // both long diagonals hold every number too
}

// Nonogram (picture puzzle): Cells row by row, 1 = filled.
type Nonogram struct {
	W     int     `json:"w"`
	H     int     `json:"h"`
	Cells []int   `json:"cells"`
	Rows  [][]int `json:"rows"`
	Cols  [][]int `json:"cols"`
	Color string  `json:"color,omitempty"`
}

// Cryptogram (secret code): every letter of Text stands for a symbol.
type Cryptogram struct {
	Text     string            `json:"text"`
	Author   string            `json:"author,omitempty"`
	Cipher   string            `json:"cipher"` // random | caesar | numbers | symbols
	Shift    int               `json:"shift,omitempty"`
	Key      map[string]string `json:"key"`   // plain letter → symbol
	Given    []string          `json:"given"` // plain letters shown from the start
	Language string            `json:"language"`
}

// Scramble: words with their letters mixed up, an optional final answer
// spelled by the circled letters.
type Scramble struct {
	Words []ScWord `json:"words"`
	Final *ScFinal `json:"final,omitempty"`
}

// ScWord is one scrambled word.
type ScWord struct {
	Word      string `json:"word"`
	Hint      string `json:"hint,omitempty"`
	Scrambled string `json:"scrambled"`
	Circles   []int  `json:"circles,omitempty"` // letter positions that go into the final answer
}

// ScFinal is the answer the circled letters make.
type ScFinal struct {
	Clue   string `json:"clue"`
	Answer string `json:"answer"`
}

// Quiz: questions, riddles or trivia.
type Quiz struct {
	Mode      string     `json:"mode"` // quiz | riddles | trivia
	Questions []Question `json:"questions"`
	PassMark  int        `json:"pass_mark,omitempty"` // percent
	Shuffle   bool       `json:"shuffle,omitempty"`
}

// Question is one quiz question.
type Question struct {
	Q       string   `json:"q"`
	Type    string   `json:"type"`    // text | choice | number | truefalse
	Answers []string `json:"answers"` // accepted answers (choice: the right choices)
	Choices []string `json:"choices,omitempty"`
	Hint    string   `json:"hint,omitempty"`
	Explain string   `json:"explain,omitempty"`
	PhotoID int64    `json:"photo_id,omitempty"`
	Points  int      `json:"points,omitempty"`
}

// Maze: Walls per square, bits N=1 E=2 S=4 W=8 (set = a wall).
type Maze struct {
	W     int    `json:"w"`
	H     int    `json:"h"`
	Seed  int64  `json:"seed"`
	Walls []int  `json:"walls"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Path  []int  `json:"path,omitempty"` // the way through
	Theme string `json:"theme,omitempty"`
}

// Logic grid: who has which — every item of a category goes with exactly
// one item of each other category.
type Logic struct {
	Story      string     `json:"story"`
	Categories []Category `json:"categories"`
	Clues      []Clue     `json:"clues"`
	// Solution[k][i] = the item of category k that goes with item i of category 0.
	Solution [][]int `json:"solution"`
}

// Category is one column group of a logic grid.
type Category struct {
	Name    string   `json:"name"`
	Items   []string `json:"items"`
	Ordered bool     `json:"ordered,omitempty"` // items are in order (times, ages, places in a race)
}

// Ref is an item of a category.
type Ref struct {
	Cat  int `json:"cat"`
	Item int `json:"item"`
}

// Clue is one logic clue. Types: same (A goes with B), not (A does not go
// with B), either (A goes with B or C), less (A comes before B by category
// By), next (A is right before or after B by By), more (A comes after B).
type Clue struct {
	Type string `json:"type"`
	A    Ref    `json:"a"`
	B    Ref    `json:"b"`
	C    *Ref   `json:"c,omitempty"`
	By   int    `json:"by,omitempty"`
	Text string `json:"text"`
}

// Ladder: change one letter at a time from From to To.
type Ladder struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Steps    []string `json:"steps"` // the whole way, From … To
	Language string   `json:"language"`
}

// Jigsaw: a picture cut into pieces.
type Jigsaw struct {
	PhotoID int64  `json:"photo_id"`
	Cols    int    `json:"cols"`
	Rows    int    `json:"rows"`
	Shape   string `json:"shape"` // classic | square
	Seed    int64  `json:"seed"`
}

// Result is what a check says.
type Result struct {
	Solved  bool     `json:"solved"`
	Wrong   []int    `json:"wrong,omitempty"`   // squares, words, questions or symbols that are wrong
	Right   []int    `json:"right,omitempty"`   // the ones that are right
	Found   *int     `json:"found,omitempty"`   // word search: the word that line is
	Filled  int      `json:"filled"`            // how much is filled in
	Total   int      `json:"total"`             // out of
	Score   int      `json:"score,omitempty"`   // quiz points
	Max     int      `json:"max,omitempty"`     // quiz points possible
	Explain []string `json:"explain,omitempty"` // quiz: the explanations, once answered
	Answers []string `json:"answers,omitempty"` // quiz: the right answers, once checked for good
	Message string   `json:"message,omitempty"`
}

// Report is what testing a puzzle finds.
type Report struct {
	OK       bool           `json:"ok"`
	Problems []string       `json:"problems"` // must be fixed before it is fair
	Notes    []string       `json:"notes"`    // worth knowing
	Level    string         `json:"level,omitempty"`
	Stats    map[string]any `json:"stats,omitempty"`
}

func (r *Report) problem(f string, a ...any) { r.Problems = append(r.Problems, fmt.Sprintf(f, a...)) }
func (r *Report) note(f string, a ...any)    { r.Notes = append(r.Notes, fmt.Sprintf(f, a...)) }

func newReport() *Report {
	return &Report{Problems: []string{}, Notes: []string{}, Stats: map[string]any{}}
}

func (r *Report) done() Report {
	r.OK = len(r.Problems) == 0
	return *r
}

func decodeAs[T any](data json.RawMessage) (*T, error) {
	var v T
	if len(data) == 0 {
		return &v, nil
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, bad("%v", err)
	}
	return &v, nil
}

func enc(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// Clean checks and tidies a definition (upper-case letters, sizes, the
// parts worked out from the rest: crossword numbers, nonogram clues) and
// returns it ready to store.
func Clean(kind string, data json.RawMessage) (json.RawMessage, error) {
	switch kind {
	case "crossword":
		return cleanAs(data, cleanCrossword)
	case "wordsearch":
		return cleanAs(data, cleanWordSearch)
	case "sudoku":
		return cleanAs(data, cleanSudoku)
	case "nonogram":
		return cleanAs(data, cleanNonogram)
	case "cryptogram":
		return cleanAs(data, cleanCryptogram)
	case "scramble":
		return cleanAs(data, cleanScramble)
	case "quiz":
		return cleanAs(data, cleanQuiz)
	case "maze":
		return cleanAs(data, cleanMaze)
	case "logic":
		return cleanAs(data, cleanLogic)
	case "ladder":
		return cleanAs(data, cleanLadder)
	case "jigsaw":
		return cleanAs(data, cleanJigsaw)
	}
	return nil, bad("unknown kind %q", kind)
}

func cleanAs[T any](data json.RawMessage, f func(*T) error) (json.RawMessage, error) {
	v, err := decodeAs[T](data)
	if err != nil {
		return nil, err
	}
	if err := f(v); err != nil {
		return nil, err
	}
	return enc(v), nil
}

// Public is what a player gets: everything but the answer.
func Public(kind string, data json.RawMessage) (any, error) {
	switch kind {
	case "crossword":
		return publicAs(data, publicCrossword)
	case "wordsearch":
		return publicAs(data, publicWordSearch)
	case "sudoku":
		return publicAs(data, publicSudoku)
	case "nonogram":
		return publicAs(data, publicNonogram)
	case "cryptogram":
		return publicAs(data, publicCryptogram)
	case "scramble":
		return publicAs(data, publicScramble)
	case "quiz":
		return publicAs(data, publicQuiz)
	case "maze":
		return publicAs(data, publicMaze)
	case "logic":
		return publicAs(data, publicLogic)
	case "ladder":
		return publicAs(data, publicLadder)
	case "jigsaw":
		return publicAs(data, func(j *Jigsaw) any { return j })
	}
	return nil, bad("unknown kind %q", kind)
}

func publicAs[T any](data json.RawMessage, f func(*T) any) (any, error) {
	v, err := decodeAs[T](data)
	if err != nil {
		return nil, err
	}
	return f(v), nil
}

// Check marks a player's answer. final: they say they are done (quiz
// answers and explanations are shown then).
func Check(kind string, data, answer json.RawMessage, final bool) (Result, error) {
	switch kind {
	case "crossword":
		return checkAs(data, answer, final, checkCrossword)
	case "wordsearch":
		return checkAs(data, answer, final, checkWordSearch)
	case "sudoku":
		return checkAs(data, answer, final, checkSudoku)
	case "nonogram":
		return checkAs(data, answer, final, checkNonogram)
	case "cryptogram":
		return checkAs(data, answer, final, checkCryptogram)
	case "scramble":
		return checkAs(data, answer, final, checkScramble)
	case "quiz":
		return checkAs(data, answer, final, checkQuiz)
	case "maze":
		return checkAs(data, answer, final, checkMaze)
	case "logic":
		return checkAs(data, answer, final, checkLogic)
	case "ladder":
		return checkAs(data, answer, final, checkLadder)
	case "jigsaw":
		return checkAs(data, answer, final, checkJigsaw)
	}
	return Result{}, bad("unknown kind %q", kind)
}

func checkAs[T, A any](data, answer json.RawMessage, final bool, f func(*T, *A, bool) Result) (Result, error) {
	v, err := decodeAs[T](data)
	if err != nil {
		return Result{}, err
	}
	a, err := decodeAs[A](answer)
	if err != nil {
		return Result{}, err
	}
	return f(v, a, final), nil
}

// Reveal gives away one part: a square, a word, a letter, a question's
// answer, the next step. what is the index of that part.
func Reveal(kind string, data json.RawMessage, what int) (any, error) {
	switch kind {
	case "crossword":
		c, err := decodeAs[Crossword](data)
		if err != nil {
			return nil, err
		}
		if what < 0 || what >= len(c.Cells) || c.Cells[what] == "#" {
			return nil, bad("no such square")
		}
		return map[string]any{"cell": what, "letter": c.Cells[what]}, nil
	case "sudoku":
		s, err := decodeAs[Sudoku](data)
		if err != nil {
			return nil, err
		}
		if what < 0 || what >= len(s.Solution) {
			return nil, bad("no such square")
		}
		return map[string]any{"cell": what, "number": s.Solution[what]}, nil
	case "nonogram":
		n, err := decodeAs[Nonogram](data)
		if err != nil {
			return nil, err
		}
		if what < 0 || what >= n.H {
			return nil, bad("no such row")
		}
		return map[string]any{"row": what, "cells": n.Cells[what*n.W : (what+1)*n.W]}, nil
	case "wordsearch":
		ws, err := decodeAs[WordSearch](data)
		if err != nil {
			return nil, err
		}
		if what < 0 || what >= len(ws.Words) || ws.Words[what].Off {
			return nil, bad("no such word")
		}
		w := ws.Words[what]
		return map[string]any{"word": what, "line": []int{w.R, w.C, w.R + w.DR*(w.Len-1), w.C + w.DC*(w.Len-1)}}, nil
	case "cryptogram":
		c, err := decodeAs[Cryptogram](data)
		if err != nil {
			return nil, err
		}
		// what = the symbol's position in the text's list of symbols
		syms := cryptoSymbols(c)
		if what < 0 || what >= len(syms) {
			return nil, bad("no such letter")
		}
		return map[string]any{"symbol": syms[what][1], "letter": syms[what][0]}, nil
	case "scramble":
		s, err := decodeAs[Scramble](data)
		if err != nil {
			return nil, err
		}
		if what == -1 && s.Final != nil {
			return map[string]any{"final": s.Final.Answer}, nil
		}
		if what < 0 || what >= len(s.Words) {
			return nil, bad("no such word")
		}
		return map[string]any{"word": what, "answer": s.Words[what].Word}, nil
	case "quiz":
		q, err := decodeAs[Quiz](data)
		if err != nil {
			return nil, err
		}
		if what < 0 || what >= len(q.Questions) {
			return nil, bad("no such question")
		}
		qq := q.Questions[what]
		return map[string]any{"question": what, "answer": first(qq.Answers), "explain": qq.Explain}, nil
	case "maze":
		m, err := decodeAs[Maze](data)
		if err != nil {
			return nil, err
		}
		return map[string]any{"path": solveMaze(m)}, nil
	case "logic":
		l, err := decodeAs[Logic](data)
		if err != nil {
			return nil, err
		}
		// what = category k*100 + item i of category 0
		k, i := what/100, what%100
		if k <= 0 || k >= len(l.Solution) || i < 0 || i >= len(l.Solution[k]) {
			return nil, bad("no such square")
		}
		return map[string]any{"cat": k, "item": i, "match": l.Solution[k][i]}, nil
	case "ladder":
		l, err := decodeAs[Ladder](data)
		if err != nil {
			return nil, err
		}
		if what < 1 || what >= len(l.Steps)-1 {
			return nil, bad("no such step")
		}
		return map[string]any{"step": what, "word": l.Steps[what]}, nil
	case "jigsaw":
		return map[string]any{"piece": what}, nil
	}
	return nil, bad("nothing to reveal in a %s", kind)
}

// Test says whether a puzzle is ready: it has one answer, nothing missing.
func Test(kind string, data json.RawMessage) (Report, error) {
	switch kind {
	case "crossword":
		return testAs(data, testCrossword)
	case "wordsearch":
		return testAs(data, testWordSearch)
	case "sudoku":
		return testAs(data, testSudoku)
	case "nonogram":
		return testAs(data, testNonogram)
	case "cryptogram":
		return testAs(data, testCryptogram)
	case "scramble":
		return testAs(data, testScramble)
	case "quiz":
		return testAs(data, testQuiz)
	case "maze":
		return testAs(data, testMaze)
	case "logic":
		return testAs(data, testLogic)
	case "ladder":
		return testAs(data, testLadder)
	case "jigsaw":
		return testAs(data, func(j *Jigsaw) Report {
			r := newReport()
			if j.PhotoID == 0 {
				r.problem("Choose a picture")
			}
			r.Stats["pieces"] = j.Cols * j.Rows
			return r.done()
		})
	}
	return Report{}, bad("unknown kind %q", kind)
}

func testAs[T any](data json.RawMessage, f func(*T) Report) (Report, error) {
	v, err := decodeAs[T](data)
	if err != nil {
		return Report{}, err
	}
	return f(v), nil
}

// Size is a short line about a puzzle's size ("15 × 15, 78 words").
func Size(kind string, data json.RawMessage) string {
	switch kind {
	case "crossword":
		if c, err := decodeAs[Crossword](data); err == nil {
			return fmt.Sprintf("%d × %d, %d words", c.W, c.H, len(Entries(c)))
		}
	case "wordsearch":
		if w, err := decodeAs[WordSearch](data); err == nil {
			return fmt.Sprintf("%d × %d, %d words", w.W, w.H, len(w.Words))
		}
	case "sudoku":
		if s, err := decodeAs[Sudoku](data); err == nil {
			n := 0
			for _, g := range s.Givens {
				if g > 0 {
					n++
				}
			}
			return fmt.Sprintf("%d × %d, %d given", s.Size, s.Size, n)
		}
	case "nonogram":
		if n, err := decodeAs[Nonogram](data); err == nil {
			return fmt.Sprintf("%d × %d", n.W, n.H)
		}
	case "cryptogram":
		if c, err := decodeAs[Cryptogram](data); err == nil {
			return fmt.Sprintf("%d letters", len(Letters(c.Text, c.Language)))
		}
	case "scramble":
		if s, err := decodeAs[Scramble](data); err == nil {
			return fmt.Sprintf("%d words", len(s.Words))
		}
	case "quiz":
		if q, err := decodeAs[Quiz](data); err == nil {
			return fmt.Sprintf("%d questions", len(q.Questions))
		}
	case "maze":
		if m, err := decodeAs[Maze](data); err == nil {
			return fmt.Sprintf("%d × %d", m.W, m.H)
		}
	case "logic":
		if l, err := decodeAs[Logic](data); err == nil && len(l.Categories) > 0 {
			return fmt.Sprintf("%d × %d, %d clues", len(l.Categories), len(l.Categories[0].Items), len(l.Clues))
		}
	case "ladder":
		if l, err := decodeAs[Ladder](data); err == nil {
			return fmt.Sprintf("%s → %s, %d steps", l.From, l.To, max(0, len(l.Steps)-1))
		}
	case "jigsaw":
		if j, err := decodeAs[Jigsaw](data); err == nil {
			return fmt.Sprintf("%d pieces", j.Cols*j.Rows)
		}
	}
	return ""
}

func first(a []string) string {
	if len(a) == 0 {
		return ""
	}
	return a[0]
}
