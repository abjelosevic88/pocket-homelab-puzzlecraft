package puzzle

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

const maxGrid = 30

func cleanCrossword(c *Crossword) error {
	if c.W < 3 || c.H < 3 || c.W > maxGrid || c.H > maxGrid {
		return bad("a crossword is 3 to %d squares each way", maxGrid)
	}
	if len(c.Cells) != c.W*c.H {
		cells := make([]string, c.W*c.H)
		copy(cells, c.Cells)
		c.Cells = cells
	}
	for i, v := range c.Cells {
		v = upper(v)
		if v == "#" || v == "" {
			c.Cells[i] = v
			continue
		}
		if l := Letters(v, c.Language); len(l) > 0 {
			c.Cells[i] = l[0]
			if len(l) > 1 && c.Language != "bcs" {
				c.Cells[i] = strings.Join(l, "")[:1]
			}
		} else {
			c.Cells[i] = ""
		}
	}
	if c.Style != "freeform" {
		c.Style = "classic"
	}
	if c.Symmetry != "mirror" && c.Symmetry != "none" {
		c.Symmetry = "rotational"
	}
	if c.Language == "" {
		c.Language = "en"
	}
	if c.Clues == nil {
		c.Clues = map[string]string{}
	}
	keys := map[string]bool{}
	for _, e := range Entries(c) {
		keys[e.Key] = true
	}
	for k, v := range c.Clues {
		v = strings.TrimSpace(v)
		if !keys[k] || v == "" {
			delete(c.Clues, k)
			continue
		}
		if len(v) > 300 {
			v = v[:300]
		}
		c.Clues[k] = v
	}
	circ := []int{}
	for _, i := range c.Circles {
		if i >= 0 && i < len(c.Cells) && c.Cells[i] != "#" {
			circ = append(circ, i)
		}
	}
	c.Circles = circ
	return nil
}

// open: a square that takes a letter.
func (c *Crossword) open(i int) bool {
	if c.Style == "freeform" {
		return c.Cells[i] != "#" && c.Cells[i] != ""
	}
	return c.Cells[i] != "#"
}

// Entries lists the words, numbered the usual way (left to right, top to
// bottom; a square starting a word across or down gets the next number).
func Entries(c *Crossword) []Entry {
	out := []Entry{}
	if c.W*c.H != len(c.Cells) {
		return out
	}
	num := 0
	for r := 0; r < c.H; r++ {
		for col := 0; col < c.W; col++ {
			i := r*c.W + col
			if !c.open(i) {
				continue
			}
			across := (col == 0 || !c.open(i-1)) && col+1 < c.W && c.open(i+1)
			down := (r == 0 || !c.open(i-c.W)) && r+1 < c.H && c.open(i+c.W)
			if !across && !down {
				continue
			}
			num++
			if across {
				e := Entry{Num: num, Dir: "A", Key: fmt.Sprintf("%dA", num)}
				for k := col; k < c.W && c.open(r*c.W+k); k++ {
					e.Cells = append(e.Cells, r*c.W+k)
				}
				out = append(out, e)
			}
			if down {
				e := Entry{Num: num, Dir: "D", Key: fmt.Sprintf("%dD", num)}
				for k := r; k < c.H && c.open(k*c.W+col); k++ {
					e.Cells = append(e.Cells, k*c.W+col)
				}
				out = append(out, e)
			}
		}
	}
	for i := range out {
		var b strings.Builder
		full := true
		for _, ci := range out[i].Cells {
			if c.Cells[ci] == "" {
				full = false
				b.WriteString("?")
			} else {
				b.WriteString(c.Cells[ci])
			}
		}
		out[i].Answer = b.String()
		if !full {
			out[i].Answer = b.String()
		}
		out[i].Clue = c.Clues[out[i].Key]
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Dir != out[b].Dir {
			return out[a].Dir == "A"
		}
		return out[a].Num < out[b].Num
	})
	return out
}

func numbers(c *Crossword) []int {
	n := make([]int, len(c.Cells))
	for _, e := range Entries(c) {
		n[e.Cells[0]] = e.Num
	}
	return n
}

func publicCrossword(c *Crossword) any {
	blocks := make([]bool, len(c.Cells))
	for i := range c.Cells {
		blocks[i] = !c.open(i)
	}
	es := Entries(c)
	for i := range es {
		es[i].Answer = ""
	}
	return map[string]any{"w": c.W, "h": c.H, "blocks": blocks, "numbers": numbers(c), "entries": es, "circles": c.Circles, "style": c.Style, "language": c.Language}
}

type cellsAnswer struct {
	Cells []string `json:"cells"`
}

func checkCrossword(c *Crossword, a *cellsAnswer, final bool) Result {
	r := Result{Wrong: []int{}}
	for i := range c.Cells {
		if !c.open(i) {
			continue
		}
		r.Total++
		got := ""
		if i < len(a.Cells) {
			got = upper(a.Cells[i])
		}
		if got == "" {
			continue
		}
		r.Filled++
		if got != c.Cells[i] && Fold(got) != Fold(c.Cells[i]) {
			r.Wrong = append(r.Wrong, i)
		}
	}
	r.Solved = r.Total > 0 && r.Filled == r.Total && len(r.Wrong) == 0
	return r
}

func testCrossword(c *Crossword) Report {
	r := newReport()
	es := Entries(c)
	empty, short, unknown, noClue := 0, 0, []string{}, 0
	for i := range c.Cells {
		if c.open(i) && c.Cells[i] == "" {
			empty++
		}
	}
	seen := map[string]int{}
	for _, e := range es {
		if len(e.Cells) < 3 && c.Style == "classic" {
			short++
		}
		if e.Clue == "" {
			noClue++
		}
		if !strings.Contains(e.Answer, "?") {
			seen[e.Answer]++
			if c.Language == "en" && !IsWord(e.Answer) && len(unknown) < 12 {
				unknown = append(unknown, e.Answer)
			}
		}
	}
	if len(es) == 0 {
		r.problem("There are no words yet")
	}
	if empty > 0 {
		r.problem("%d %s still need a letter", empty, pl(empty, "square", "squares"))
	}
	if noClue > 0 {
		r.problem("%d %s no clue", noClue, pl(noClue, "word has", "words have"))
	}
	if short > 0 {
		r.note("%d %s only two letters long — most crosswords keep words to three letters or more", short, pl(short, "word is", "words are"))
	}
	for w, n := range seen {
		if n > 1 {
			r.note("%s is in the grid %d times", w, n)
		}
	}
	if len(unknown) > 0 {
		r.note("Not in the dictionary (fine for names and themes): %s", strings.Join(unknown, ", "))
	}
	// every white square in a classic grid should belong to two words
	if c.Style == "classic" {
		count := make([]int, len(c.Cells))
		for _, e := range es {
			for _, ci := range e.Cells {
				count[ci]++
			}
		}
		lone := 0
		for i := range c.Cells {
			if c.open(i) && count[i] == 0 {
				lone++
			}
		}
		if lone > 0 {
			r.problem("%d white %s in no word", lone, pl(lone, "square is", "squares are"))
		}
		if !connected(c) {
			r.note("The grid falls apart in pieces that do not touch")
		}
	}
	r.Stats["words"] = len(es)
	return r.done()
}

func connected(c *Crossword) bool {
	start := -1
	n := 0
	for i := range c.Cells {
		if c.open(i) {
			n++
			if start < 0 {
				start = i
			}
		}
	}
	if start < 0 {
		return true
	}
	seen := map[int]bool{start: true}
	q := []int{start}
	for len(q) > 0 {
		i := q[0]
		q = q[1:]
		r, col := i/c.W, i%c.W
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			rr, cc := r+d[0], col+d[1]
			if rr < 0 || cc < 0 || rr >= c.H || cc >= c.W {
				continue
			}
			j := rr*c.W + cc
			if c.open(j) && !seen[j] {
				seen[j] = true
				q = append(q, j)
			}
		}
	}
	return len(seen) == n
}

func pl(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ---- making a crossword from a list of words (criss-cross) ----

// WordClue is a word and its clue.
type WordClue struct {
	Word string `json:"word"`
	Clue string `json:"clue"`
}

type placed struct {
	letters []string
	clue    string
	r, c    int
	across  bool
}

// CrissCross lays words out so they cross each other, as compact as it
// can, trying many orders. It returns a freeform crossword and the words
// that did not fit.
func CrissCross(list []WordClue, lang string, maxSize int, seed int64) (*Crossword, []string) {
	if maxSize < 5 || maxSize > maxGrid {
		maxSize = 21
	}
	type wl struct {
		l    []string
		clue string
		raw  string
	}
	ws := []wl{}
	seen := map[string]bool{}
	for _, w := range list {
		l := Letters(w.Word, lang)
		k := strings.Join(l, "")
		if len(l) < 2 || len(l) > maxSize || seen[k] {
			continue
		}
		seen[k] = true
		ws = append(ws, wl{l, strings.TrimSpace(w.Clue), w.Word})
	}
	if len(ws) == 0 {
		return nil, nil
	}
	rng := rand.New(rand.NewSource(seed))
	var best []placed
	bestScore := -1.0
	deadline := time.Now().Add(4 * time.Second) // a safety net; normally the tries run out first
	for try := 0; try < 160 && time.Now().Before(deadline); try++ {
		order := make([]wl, len(ws))
		copy(order, ws)
		if try == 0 {
			sort.SliceStable(order, func(a, b int) bool { return len(order[a].l) > len(order[b].l) })
		} else {
			rng.Shuffle(len(order), func(a, b int) { order[a], order[b] = order[b], order[a] })
			// longest first most of the time
			if try%3 != 0 {
				sort.SliceStable(order, func(a, b int) bool { return len(order[a].l) > len(order[b].l) })
			}
		}
		g := map[[2]int]string{}
		ps := []placed{}
		put := func(p placed) {
			for k, l := range p.letters {
				r, c := p.r, p.c+k
				if !p.across {
					r, c = p.r+k, p.c
				}
				g[[2]int{r, c}] = l
			}
			ps = append(ps, p)
		}
		put(placed{order[0].l, order[0].clue, 0, 0, rng.Intn(2) == 0})
		left := order[1:]
		for pass := 0; pass < 2; pass++ {
			var rest []wl
			for _, w := range left {
				p, ok := bestSpot(g, ps, w.l, maxSize, rng)
				if !ok {
					rest = append(rest, w)
					continue
				}
				p.clue = w.clue
				put(p)
			}
			left = rest
		}
		minR, minC, maxR, maxC := bbox(g)
		area := float64((maxR - minR + 1) * (maxC - minC + 1))
		score := float64(len(ps))*1000 - area - float64(abs((maxR-minR)-(maxC-minC)))*2
		if score > bestScore {
			bestScore = score
			best = append([]placed(nil), ps...)
		}
		if len(ps) == len(ws) && try > 60 {
			break
		}
	}
	g := map[[2]int]string{}
	for _, p := range best {
		for k, l := range p.letters {
			r, c := p.r, p.c+k
			if !p.across {
				r, c = p.r+k, p.c
			}
			g[[2]int{r, c}] = l
		}
	}
	minR, minC, maxR, maxC := bbox(g)
	cw := &Crossword{W: maxC - minC + 1, H: maxR - minR + 1, Style: "freeform", Symmetry: "none", Language: lang, Clues: map[string]string{}}
	if cw.W < 3 {
		cw.W = 3
	}
	if cw.H < 3 {
		cw.H = 3
	}
	cw.Cells = make([]string, cw.W*cw.H)
	for k, l := range g {
		cw.Cells[(k[0]-minR)*cw.W+(k[1]-minC)] = l
	}
	// clues by where each word starts
	for _, e := range Entries(cw) {
		r0, c0 := e.Cells[0]/cw.W+minR, e.Cells[0]%cw.W+minC
		for _, p := range best {
			if p.r == r0 && p.c == c0 && p.across == (e.Dir == "A") && len(p.letters) == len(e.Cells) {
				cw.Clues[e.Key] = p.clue
			}
		}
	}
	placedSet := map[string]bool{}
	for _, p := range best {
		placedSet[strings.Join(p.letters, "")] = true
	}
	missing := []string{}
	for _, w := range ws {
		if !placedSet[strings.Join(w.l, "")] {
			missing = append(missing, w.raw)
		}
	}
	return cw, missing
}

func bbox(g map[[2]int]string) (int, int, int, int) {
	first := true
	var a, b, c, d int
	for k := range g {
		if first {
			a, b, c, d = k[0], k[1], k[0], k[1]
			first = false
			continue
		}
		a, b, c, d = min(a, k[0]), min(b, k[1]), max(c, k[0]), max(d, k[1])
	}
	return a, b, c, d
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// bestSpot finds where a word crosses the grid best (most crossings,
// smallest growth).
func bestSpot(g map[[2]int]string, ps []placed, w []string, maxSize int, rng *rand.Rand) (placed, bool) {
	minR, minC, maxR, maxC := bbox(g)
	type cand struct {
		p     placed
		score float64
	}
	var cands []cand
	keys := make([][2]int, 0, len(g))
	for k := range g {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a][0] != keys[b][0] {
			return keys[a][0] < keys[b][0]
		}
		return keys[a][1] < keys[b][1]
	})
	for _, k := range keys {
		l := g[k]
		for i, wl := range w {
			if wl != l {
				continue
			}
			for _, across := range []bool{true, false} {
				p := placed{letters: w, across: across}
				if across {
					p.r, p.c = k[0], k[1]-i
				} else {
					p.r, p.c = k[0]-i, k[1]
				}
				cross, ok := fitsAt(g, p)
				if !ok || cross == 0 {
					continue
				}
				r2, c2 := p.r, p.c+len(w)-1
				if !across {
					r2, c2 = p.r+len(w)-1, p.c
				}
				nr0, nc0, nr1, nc1 := min(minR, p.r), min(minC, p.c), max(maxR, r2), max(maxC, c2)
				if nr1-nr0+1 > maxSize || nc1-nc0+1 > maxSize {
					continue
				}
				growth := float64((nr1-nr0+1)*(nc1-nc0+1) - (maxR-minR+1)*(maxC-minC+1))
				cands = append(cands, cand{p, float64(cross)*10 - growth*0.5 + rng.Float64()})
			}
		}
	}
	if len(cands) == 0 {
		return placed{}, false
	}
	sort.Slice(cands, func(a, b int) bool { return cands[a].score > cands[b].score })
	return cands[0].p, true
}

// fitsAt checks a word can go there: letters agree where it crosses, and
// it does not touch other words side by side or end to end.
func fitsAt(g map[[2]int]string, p placed) (int, bool) {
	cross := 0
	n := len(p.letters)
	at := func(k int) (int, int) {
		if p.across {
			return p.r, p.c + k
		}
		return p.r + k, p.c
	}
	// the squares just before and after must be empty
	br, bc := at(-1)
	ar, ac := at(n)
	if _, ok := g[[2]int{br, bc}]; ok {
		return 0, false
	}
	if _, ok := g[[2]int{ar, ac}]; ok {
		return 0, false
	}
	for k := 0; k < n; k++ {
		r, c := at(k)
		if l, ok := g[[2]int{r, c}]; ok {
			if l != p.letters[k] {
				return 0, false
			}
			cross++
			continue
		}
		// a new square: nothing beside it across the word's direction
		var s1, s2 [2]int
		if p.across {
			s1, s2 = [2]int{r - 1, c}, [2]int{r + 1, c}
		} else {
			s1, s2 = [2]int{r, c - 1}, [2]int{r, c + 1}
		}
		if _, ok := g[s1]; ok {
			return 0, false
		}
		if _, ok := g[s2]; ok {
			return 0, false
		}
	}
	if cross == n {
		return 0, false
	}
	return cross, true
}

// ---- classic grids: black squares and filling the rest ----

// BlankGrid makes an empty classic grid with a symmetric pattern of black
// squares (no word shorter than three, every square in two words).
func BlankGrid(w, h int, seed int64) *Crossword {
	if w < 3 {
		w = 3
	}
	if h < 3 {
		h = 3
	}
	rng := rand.New(rand.NewSource(seed))
	best := (*Crossword)(nil)
	target := w * h / 6
	if w <= 5 {
		target = w * h / 8
	}
	for try := 0; try < 200; try++ {
		c := &Crossword{W: w, H: h, Cells: make([]string, w*h), Style: "classic", Symmetry: "rotational", Language: "en", Clues: map[string]string{}}
		placedN := 0
		for guard := 0; guard < w*h*4 && placedN < target; guard++ {
			i := rng.Intn(w * h)
			j := w*h - 1 - i
			if c.Cells[i] == "#" {
				continue
			}
			c.Cells[i], c.Cells[j] = "#", "#"
			if !gridOK(c) {
				c.Cells[i], c.Cells[j] = "", ""
				continue
			}
			placedN += 2
			if i == j {
				placedN--
			}
		}
		for guard := 0; guard < w*h*4 && !gridRuns(c, maxRun(w)); guard++ {
			i := rng.Intn(w * h)
			j := w*h - 1 - i
			if c.Cells[i] == "#" {
				continue
			}
			c.Cells[i], c.Cells[j] = "#", "#"
			if !gridOK(c) {
				c.Cells[i], c.Cells[j] = "", ""
			}
		}
		if gridRuns(c, maxRun(w)) && connected(c) {
			if best == nil || abs(blackCount(c)-target) < abs(blackCount(best)-target) {
				best = c
			}
			if abs(blackCount(c)-target) <= 2 {
				break
			}
		}
	}
	if best == nil {
		best = &Crossword{W: w, H: h, Cells: make([]string, w*h), Style: "classic", Symmetry: "rotational", Language: "en", Clues: map[string]string{}}
	}
	return best
}

func blackCount(c *Crossword) int {
	n := 0
	for _, v := range c.Cells {
		if v == "#" {
			n++
		}
	}
	return n
}

// gridOK: every run of white squares across and down is 0 or ≥ 3 long.
func gridOK(c *Crossword) bool { return gridRuns(c, 0) }

// maxRun is the longest word a blank grid of that width gets (long words
// make a grid much harder to fill).
func maxRun(w int) int {
	switch {
	case w <= 7:
		return w
	case w <= 11:
		return 7
	}
	return 8
}

func gridRuns(c *Crossword, longest int) bool {
	for r := 0; r < c.H; r++ {
		run := 0
		for col := 0; col <= c.W; col++ {
			if col < c.W && c.Cells[r*c.W+col] != "#" {
				run++
				continue
			}
			if run > 0 && run < 3 || longest > 0 && run > longest {
				return false
			}
			run = 0
		}
	}
	for col := 0; col < c.W; col++ {
		run := 0
		for r := 0; r <= c.H; r++ {
			if r < c.H && c.Cells[r*c.W+col] != "#" {
				run++
				continue
			}
			if run > 0 && run < 3 || longest > 0 && run > longest {
				return false
			}
			run = 0
		}
	}
	return true
}

// fillIndex finds words of one length by the letters they have where.
type fillIndex struct {
	words []string
	at    []map[rune][]int32 // position → letter → word numbers (ascending)
}

func newFillIndex(ws []string, n int) *fillIndex {
	x := &fillIndex{at: make([]map[rune][]int32, n)}
	for i := range x.at {
		x.at[i] = map[rune][]int32{}
	}
	for _, w := range ws {
		rs := []rune(w)
		if len(rs) != n {
			continue
		}
		k := int32(len(x.words))
		x.words = append(x.words, w)
		for i, r := range rs {
			x.at[i][r] = append(x.at[i][r], k)
		}
	}
	return x
}

func intersect(a, b []int32) []int32 {
	out := make([]int32, 0, min(len(a), len(b)))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, a[i])
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return out
}

// match lists the word numbers fitting a pattern ('?' = open), at most cap (0 = all).
func (x *fillIndex) match(p []rune, used map[string]bool, cap int) []int32 {
	var cur []int32
	have := false
	for i, r := range p {
		if r == '?' {
			continue
		}
		l := x.at[i][r]
		if !have {
			cur, have = l, true
		} else {
			cur = intersect(cur, l)
		}
		if len(cur) == 0 {
			return nil
		}
	}
	out := []int32{}
	if !have {
		for k := range x.words {
			if !used[x.words[k]] {
				out = append(out, int32(k))
				if cap > 0 && len(out) >= cap {
					break
				}
			}
		}
		return out
	}
	for _, k := range cur {
		if !used[x.words[k]] {
			out = append(out, k)
			if cap > 0 && len(out) >= cap {
				break
			}
		}
	}
	return out
}

// Fill fills a classic grid's empty squares with English words (everyday
// ones first, your own words before those), keeping the letters already
// there. It looks ahead: a word goes in only when every word crossing it
// can still be finished. It gives up after the time allowed.
func Fill(c *Crossword, extra []string, seed int64, limit time.Duration) (*Crossword, bool) {
	out := *c
	out.Cells = append([]string(nil), c.Cells...)
	es := Entries(&out)
	if len(es) == 0 {
		return &out, false
	}
	for _, v := range out.Cells {
		if len([]rune(v)) > 1 {
			return &out, false // LJ/NJ/DŽ squares: the English list cannot fill those
		}
	}
	rng := rand.New(rand.NewSource(seed))
	lens := map[int]bool{}
	for _, e := range es {
		lens[len(e.Cells)] = true
	}
	idx := map[int]*fillIndex{}
	for n := range lens {
		list := []string{}
		seen := map[string]bool{}
		for _, w := range extra {
			w = upper(w)
			if len([]rune(w)) == n && !seen[w] {
				seen[w] = true
				list = append(list, w)
			}
		}
		common, rest := []string{}, []string{}
		for _, w := range words().byLen[n] {
			if seen[w] {
				continue
			}
			if words().common[w] {
				common = append(common, w)
			} else {
				rest = append(rest, w)
			}
		}
		rng.Shuffle(len(common), func(a, b int) { common[a], common[b] = common[b], common[a] })
		rng.Shuffle(len(rest), func(a, b int) { rest[a], rest[b] = rest[b], rest[a] })
		list = append(append(list, common...), rest...)
		idx[n] = newFillIndex(list, n)
	}
	deadline := time.Now().Add(limit)
	used := map[string]bool{}
	pattern := func(e Entry) []rune {
		p := make([]rune, len(e.Cells))
		for k, ci := range e.Cells {
			if out.Cells[ci] == "" {
				p[k] = '?'
			} else {
				p[k] = []rune(out.Cells[ci])[0]
			}
		}
		return p
	}
	complete := func(p []rune) bool {
		for _, r := range p {
			if r == '?' {
				return false
			}
		}
		return true
	}
	for _, e := range es {
		if p := pattern(e); complete(p) {
			used[string(p)] = true
		}
	}
	cellEntries := map[int][]int{}
	for i, e := range es {
		for _, ci := range e.Cells {
			cellEntries[ci] = append(cellEntries[ci], i)
		}
	}
	var solve func() bool
	solve = func() bool {
		if time.Now().After(deadline) {
			return false
		}
		bestI, bestN := -1, 1<<30
		var bestC []int32
		for i, e := range es {
			p := pattern(e)
			if complete(p) {
				continue
			}
			cs := idx[len(p)].match(p, used, 0)
			if len(cs) < bestN {
				bestI, bestN, bestC = i, len(cs), cs
				if bestN == 0 {
					return false
				}
			}
		}
		if bestI < 0 {
			return true
		}
		e := es[bestI]
		x := idx[len(e.Cells)]
		if len(bestC) > 200 {
			bestC = bestC[:200]
		}
		for _, k := range bestC {
			w := []rune(x.words[k])
			saved := make([]string, len(e.Cells))
			for j, ci := range e.Cells {
				saved[j] = out.Cells[ci]
				out.Cells[ci] = string(w[j])
			}
			used[x.words[k]] = true
			ok := true
			for _, ci := range e.Cells {
				for _, j := range cellEntries[ci] {
					if j == bestI {
						continue
					}
					p := pattern(es[j])
					if complete(p) {
						if !idx[len(p)].has(string(p)) && !used[string(p)] {
							ok = false
						}
					} else if len(idx[len(p)].match(p, used, 1)) == 0 {
						ok = false
					}
					if !ok {
						break
					}
				}
				if !ok {
					break
				}
			}
			if ok && solve() {
				return true
			}
			delete(used, x.words[k])
			for j, ci := range e.Cells {
				out.Cells[ci] = saved[j]
			}
			if time.Now().After(deadline) {
				return false
			}
		}
		return false
	}
	if solve() {
		return &out, true
	}
	return &Crossword{W: c.W, H: c.H, Cells: append([]string(nil), c.Cells...), Circles: c.Circles, Clues: c.Clues, Style: c.Style, Symmetry: c.Symmetry, Language: c.Language}, false
}

func (x *fillIndex) has(w string) bool {
	p := []rune(w)
	return len(x.match(p, nil, 1)) > 0
}
