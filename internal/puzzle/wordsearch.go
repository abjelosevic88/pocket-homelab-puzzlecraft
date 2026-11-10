package puzzle

import (
	"math/rand"
	"sort"
	"strings"
)

func dirsFor(level string) [][2]int {
	switch level {
	case "easy":
		return [][2]int{{0, 1}, {1, 0}}
	case "medium":
		return [][2]int{{0, 1}, {1, 0}, {1, 1}, {-1, 1}}
	}
	return [][2]int{{0, 1}, {1, 0}, {1, 1}, {-1, 1}, {0, -1}, {-1, 0}, {-1, -1}, {1, -1}}
}

// MakeWordSearch hides the words in a w × h grid and fills the rest with
// random letters — or with a hidden message, read in order from the
// squares no word uses.
func MakeWordSearch(list []WordClue, w, h int, level, message, lang string, seed int64) *WordSearch {
	if !oneOfS(level, Levels) {
		level = "medium"
	}
	if w < 5 {
		w = 5
	}
	if h < 5 {
		h = 5
	}
	w, h = min(w, maxGrid), min(h, maxGrid)
	rng := rand.New(rand.NewSource(seed))
	type item struct {
		WordClue
		l []string
	}
	items := []item{}
	seen := map[string]bool{}
	for _, wc := range list {
		l := Letters(wc.Word, lang)
		k := strings.Join(l, "")
		if len(l) < 2 || seen[k] {
			continue
		}
		seen[k] = true
		items = append(items, item{wc, l})
	}
	sort.SliceStable(items, func(a, b int) bool { return len(items[a].l) > len(items[b].l) })
	msg := Letters(message, lang)
	dirs := dirsFor(level)
	var best *WordSearch
	for try := 0; try < 40; try++ {
		grid := make([]string, w*h)
		ws := &WordSearch{W: w, H: h, Level: level, Message: strings.TrimSpace(message), Language: lang}
		for _, it := range items {
			placedOK := false
			n := len(it.l)
			// try spots in random order, preferring ones that share letters
			type spot struct{ r, c, dr, dc, share int }
			var spots []spot
			for _, d := range dirs {
				for r := 0; r < h; r++ {
					for c := 0; c < w; c++ {
						er, ec := r+d[0]*(n-1), c+d[1]*(n-1)
						if er < 0 || er >= h || ec < 0 || ec >= w {
							continue
						}
						share, ok := 0, true
						for k := 0; k < n; k++ {
							g := grid[(r+d[0]*k)*w+c+d[1]*k]
							if g != "" && g != it.l[k] {
								ok = false
								break
							}
							if g != "" {
								share++
							}
						}
						if ok && share < n {
							spots = append(spots, spot{r, c, d[0], d[1], share})
						}
					}
				}
			}
			if len(spots) > 0 {
				rng.Shuffle(len(spots), func(a, b int) { spots[a], spots[b] = spots[b], spots[a] })
				pick := spots[0]
				if level != "easy" && rng.Intn(3) == 0 {
					for _, s := range spots {
						if s.share > pick.share {
							pick = s
						}
					}
				}
				for k := 0; k < n; k++ {
					grid[(pick.r+pick.dr*k)*w+pick.c+pick.dc*k] = it.l[k]
				}
				ws.Words = append(ws.Words, WSWord{Word: strings.Join(it.l, ""), Clue: strings.TrimSpace(it.Clue), R: pick.r, C: pick.c, DR: pick.dr, DC: pick.dc, Len: n})
				placedOK = true
			}
			if !placedOK {
				ws.Words = append(ws.Words, WSWord{Word: strings.Join(it.l, ""), Clue: strings.TrimSpace(it.Clue), Off: true, Len: n})
			}
		}
		free := 0
		for _, g := range grid {
			if g == "" {
				free++
			}
		}
		if len(msg) > 0 && free < len(msg) {
			continue
		}
		mi := 0
		alpha := Alphabet(lang)
		// a message goes in reading order, spread over the free squares
		for i := range grid {
			if grid[i] != "" {
				continue
			}
			if mi < len(msg) {
				grid[i] = msg[mi]
				mi++
				continue
			}
			grid[i] = alpha[rng.Intn(len(alpha))]
		}
		ws.Grid = grid
		off := 0
		for _, x := range ws.Words {
			if x.Off {
				off++
			}
		}
		if best == nil || off < offCount(best) {
			best = ws
		}
		if off == 0 {
			break
		}
	}
	if best == nil {
		best = &WordSearch{W: w, H: h, Level: level, Language: lang, Grid: make([]string, w*h)}
		alpha := Alphabet(lang)
		for i := range best.Grid {
			best.Grid[i] = alpha[rng.Intn(len(alpha))]
		}
		best.Message = ""
	}
	if best.Words == nil {
		best.Words = []WSWord{}
	}
	return best
}

func offCount(ws *WordSearch) int {
	n := 0
	for _, x := range ws.Words {
		if x.Off {
			n++
		}
	}
	return n
}

func oneOfS(v string, list []string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func cleanWordSearch(ws *WordSearch) error {
	if ws.W < 4 || ws.H < 4 || ws.W > maxGrid || ws.H > maxGrid || len(ws.Grid) != ws.W*ws.H {
		return bad("a word search is 4 to %d squares each way", maxGrid)
	}
	if ws.Language == "" {
		ws.Language = "en"
	}
	if !oneOfS(ws.Level, Levels) {
		ws.Level = "medium"
	}
	for i := range ws.Grid {
		ws.Grid[i] = upper(ws.Grid[i])
	}
	for i := range ws.Words {
		x := &ws.Words[i]
		x.Word = strings.Join(Letters(x.Word, ws.Language), "")
		x.Len = len(Letters(x.Word, ws.Language))
		if !x.Off && !wsAt(ws, *x) {
			x.Off = true
		}
	}
	return nil
}

// wsAt: the word really is there.
func wsAt(ws *WordSearch, x WSWord) bool {
	l := Letters(x.Word, ws.Language)
	for k := range l {
		r, c := x.R+x.DR*k, x.C+x.DC*k
		if r < 0 || c < 0 || r >= ws.H || c >= ws.W || ws.Grid[r*ws.W+c] != l[k] {
			return false
		}
	}
	return len(l) > 0
}

func publicWordSearch(ws *WordSearch) any {
	list := []map[string]any{}
	for i, x := range ws.Words {
		if x.Off {
			continue
		}
		m := map[string]any{"i": i, "len": x.Len}
		if ws.Clues && x.Clue != "" {
			m["clue"] = x.Clue
		} else {
			m["word"] = x.Word
		}
		list = append(list, m)
	}
	return map[string]any{"w": ws.W, "h": ws.H, "grid": ws.Grid, "words": list, "level": ws.Level, "clues": ws.Clues, "has_message": ws.Message != "", "language": ws.Language}
}

type wsAnswer struct {
	Line  []int  `json:"line"`  // r1 c1 r2 c2: a line the player drew
	Found []int  `json:"found"` // the words they found so far
	Msg   string `json:"message"`
}

func checkWordSearch(ws *WordSearch, a *wsAnswer, final bool) Result {
	r := Result{}
	total := 0
	for _, x := range ws.Words {
		if !x.Off {
			total++
		}
	}
	r.Total = total
	if len(a.Line) == 4 {
		r1, c1, r2, c2 := a.Line[0], a.Line[1], a.Line[2], a.Line[3]
		f := -1
		for i, x := range ws.Words {
			if x.Off {
				continue
			}
			er, ec := x.R+x.DR*(x.Len-1), x.C+x.DC*(x.Len-1)
			if (x.R == r1 && x.C == c1 && er == r2 && ec == c2) || (x.R == r2 && x.C == c2 && er == r1 && ec == c1) {
				f = i
				break
			}
		}
		r.Found = &f
	}
	got := map[int]bool{}
	for _, i := range a.Found {
		if i >= 0 && i < len(ws.Words) && !ws.Words[i].Off {
			got[i] = true
		}
	}
	if r.Found != nil && *r.Found >= 0 {
		got[*r.Found] = true
	}
	for i := range got {
		r.Right = append(r.Right, i)
	}
	sort.Ints(r.Right)
	r.Filled = len(got)
	r.Solved = total > 0 && len(got) == total
	if r.Solved && ws.Message != "" {
		if a.Msg != "" && !SameAnswer(a.Msg, ws.Message) {
			r.Solved = false
			r.Message = "All the words are found — the hidden message is not quite right yet"
		} else if a.Msg == "" {
			r.Solved = false
			r.Message = "All the words are found. Now read the letters left over: what do they say?"
		}
	}
	return r
}

func testWordSearch(ws *WordSearch) Report {
	r := newReport()
	off := []string{}
	for _, x := range ws.Words {
		if x.Off {
			off = append(off, x.Word)
		}
	}
	if len(ws.Words) == 0 {
		r.problem("There are no words to find")
	}
	if len(off) > 0 {
		r.problem("Did not fit: %s — make the grid bigger or the words fewer", strings.Join(off, ", "))
	}
	// the same word twice in the grid makes "found it" ambiguous
	for _, x := range ws.Words {
		if x.Off {
			continue
		}
		if n := occurrences(ws, Letters(x.Word, ws.Language)); n > 1 {
			r.note("%s can be found %d times", x.Word, n)
		}
	}
	if ws.Message != "" {
		used := make([]bool, len(ws.Grid))
		for _, x := range ws.Words {
			if x.Off {
				continue
			}
			for k := 0; k < x.Len; k++ {
				used[(x.R+x.DR*k)*ws.W+x.C+x.DC*k] = true
			}
		}
		left := []string{}
		for i, u := range used {
			if !u {
				left = append(left, ws.Grid[i])
			}
		}
		msg := strings.Join(Letters(ws.Message, ws.Language), "")
		if !strings.HasPrefix(strings.Join(left, ""), msg) {
			r.problem("The leftover letters no longer spell the hidden message — make the grid again")
		} else if len(left) > len(Letters(ws.Message, ws.Language)) {
			r.note("After the message there are %d more leftover letters", len(left)-len(Letters(ws.Message, ws.Language)))
		}
	}
	r.Stats["words"] = len(ws.Words) - len(off)
	return r.done()
}

func occurrences(ws *WordSearch, l []string) int {
	n := 0
	for r := 0; r < ws.H; r++ {
		for c := 0; c < ws.W; c++ {
			for _, d := range dirsFor("hard") {
				ok := true
				for k := range l {
					rr, cc := r+d[0]*k, c+d[1]*k
					if rr < 0 || cc < 0 || rr >= ws.H || cc >= ws.W || ws.Grid[rr*ws.W+cc] != l[k] {
						ok = false
						break
					}
				}
				if ok {
					n++
				}
			}
		}
	}
	if len(l) == 1 {
		return n
	}
	// a palindrome is found once each way
	rev := true
	for i := range l {
		if l[i] != l[len(l)-1-i] {
			rev = false
		}
	}
	if rev {
		n /= 2
	}
	return n
}
