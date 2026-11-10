package puzzle

import (
	"math/rand"
	"time"
)

func boxOf(size int) (int, int) {
	switch size {
	case 4:
		return 2, 2
	case 6:
		return 3, 2
	}
	return 3, 3
}

type sgrid struct {
	n, bw, bh int
	diag      bool
	units     [][]int // rows, columns, boxes (and diagonals)
	peers     [][]int
}

func newSGrid(n int, diag bool) *sgrid {
	bw, bh := boxOf(n)
	g := &sgrid{n: n, bw: bw, bh: bh, diag: diag}
	for r := 0; r < n; r++ {
		u := []int{}
		for c := 0; c < n; c++ {
			u = append(u, r*n+c)
		}
		g.units = append(g.units, u)
	}
	for c := 0; c < n; c++ {
		u := []int{}
		for r := 0; r < n; r++ {
			u = append(u, r*n+c)
		}
		g.units = append(g.units, u)
	}
	for br := 0; br < n; br += bh {
		for bc := 0; bc < n; bc += bw {
			u := []int{}
			for r := br; r < br+bh; r++ {
				for c := bc; c < bc+bw; c++ {
					u = append(u, r*n+c)
				}
			}
			g.units = append(g.units, u)
		}
	}
	if diag {
		a, b := []int{}, []int{}
		for i := 0; i < n; i++ {
			a = append(a, i*n+i)
			b = append(b, i*n+n-1-i)
		}
		g.units = append(g.units, a, b)
	}
	g.peers = make([][]int, n*n)
	for i := range g.peers {
		set := map[int]bool{}
		for _, u := range g.units {
			in := false
			for _, x := range u {
				if x == i {
					in = true
				}
			}
			if in {
				for _, x := range u {
					if x != i {
						set[x] = true
					}
				}
			}
		}
		for x := range set {
			g.peers[i] = append(g.peers[i], x)
		}
	}
	return g
}

func (g *sgrid) can(b []int, i, v int) bool {
	for _, p := range g.peers[i] {
		if b[p] == v {
			return false
		}
	}
	return true
}

// count solutions up to limit (backtracking, most constrained square first).
func (g *sgrid) count(b []int, limit int, rng *rand.Rand, out []int) int {
	best, bestN := -1, g.n+1
	for i, v := range b {
		if v != 0 {
			continue
		}
		k := 0
		for v := 1; v <= g.n; v++ {
			if g.can(b, i, v) {
				k++
			}
		}
		if k < bestN {
			best, bestN = i, k
			if k == 0 {
				return 0
			}
		}
	}
	if best < 0 {
		if out != nil {
			copy(out, b)
		}
		return 1
	}
	vals := make([]int, 0, g.n)
	for v := 1; v <= g.n; v++ {
		vals = append(vals, v)
	}
	if rng != nil {
		rng.Shuffle(len(vals), func(a, c int) { vals[a], vals[c] = vals[c], vals[a] })
	}
	total := 0
	for _, v := range vals {
		if !g.can(b, best, v) {
			continue
		}
		b[best] = v
		total += g.count(b, limit-total, rng, out)
		b[best] = 0
		if total >= limit {
			return total
		}
	}
	return total
}

// logic tries to solve like a person: squares with one possible number
// (naked singles), numbers with one possible square in a row, column or
// box (hidden singles), then pointing pairs. It returns how far it got and
// the hardest step it needed (1 naked, 2 hidden, 3 pointing).
func (g *sgrid) logic(givens []int) (bool, int) {
	b := append([]int(nil), givens...)
	n := g.n
	cand := make([]uint32, n*n)
	full := uint32(1<<(n+1)) - 2
	for i := range cand {
		if b[i] != 0 {
			cand[i] = 1 << b[i]
			continue
		}
		c := full
		for _, p := range g.peers[i] {
			if b[p] != 0 {
				c &^= 1 << b[p]
			}
		}
		cand[i] = c
	}
	set := func(i, v int) {
		b[i] = v
		cand[i] = 1 << v
		for _, p := range g.peers[i] {
			if b[p] == 0 {
				cand[p] &^= 1 << v
			}
		}
	}
	hardest := 0
	for {
		progress := false
		for i := range b {
			if b[i] == 0 && popcount(cand[i]) == 1 {
				set(i, lowbit(cand[i]))
				progress = true
				hardest = max(hardest, 1)
			}
		}
		if progress {
			continue
		}
		for _, u := range g.units {
			for v := 1; v <= n; v++ {
				where, k := -1, 0
				done := false
				for _, i := range u {
					if b[i] == v {
						done = true
						break
					}
					if b[i] == 0 && cand[i]&(1<<v) != 0 {
						where = i
						k++
					}
				}
				if !done && k == 1 {
					set(where, v)
					progress = true
					hardest = max(hardest, 2)
				}
			}
		}
		if progress {
			continue
		}
		// pointing: a number in a box only on one row/column clears it from the rest of that line
		boxes := g.units[2*n : 3*n]
		for _, box := range boxes {
			for v := 1; v <= n; v++ {
				cells := []int{}
				for _, i := range box {
					if b[i] == 0 && cand[i]&(1<<v) != 0 {
						cells = append(cells, i)
					}
				}
				if len(cells) < 2 {
					continue
				}
				sameR, sameC := true, true
				for _, i := range cells {
					if i/n != cells[0]/n {
						sameR = false
					}
					if i%n != cells[0]%n {
						sameC = false
					}
				}
				inBox := map[int]bool{}
				for _, i := range box {
					inBox[i] = true
				}
				for k := 0; k < n; k++ {
					var i int
					switch {
					case sameR:
						i = (cells[0]/n)*n + k
					case sameC:
						i = k*n + cells[0]%n
					default:
						continue
					}
					if !inBox[i] && b[i] == 0 && cand[i]&(1<<v) != 0 {
						cand[i] &^= 1 << v
						progress = true
						hardest = max(hardest, 3)
					}
				}
			}
		}
		if !progress {
			break
		}
	}
	for _, v := range b {
		if v == 0 {
			return false, hardest
		}
	}
	return true, hardest
}

func popcount(x uint32) int {
	n := 0
	for x != 0 {
		x &= x - 1
		n++
	}
	return n
}

func lowbit(x uint32) int {
	for v := 1; v < 32; v++ {
		if x&(1<<v) != 0 {
			return v
		}
	}
	return 0
}

// MakeSudoku makes a sudoku with exactly one answer at a level:
// easy and medium can be solved with singles, hard needs more than
// singles, expert needs guessing (or long chains).
func MakeSudoku(size int, level string, diag bool, seed int64) *Sudoku {
	if size != 4 && size != 6 {
		size = 9
	}
	if !oneOfS(level, Levels) {
		level = "medium"
	}
	if size < 9 && (level == "hard" || level == "expert") {
		level = "medium"
	}
	rng := rand.New(rand.NewSource(seed))
	g := newSGrid(size, diag)
	var best *Sudoku
	deadline := time.Now().Add(2500 * time.Millisecond)
	for try := 0; try < 30; try++ {
		sol := make([]int, size*size)
		g.count(make([]int, size*size), 1, rng, sol)
		givens := append([]int(nil), sol...)
		order := rng.Perm(size * size)
		minGivens := map[string]int{"easy": size * size * 45 / 81, "medium": size * size * 32 / 81, "hard": 26, "expert": 22}[level]
		if size == 4 {
			minGivens = map[string]int{"easy": 8, "medium": 5}[level]
		}
		left := size * size
		for _, i := range order {
			if left <= minGivens {
				break
			}
			j := size*size - 1 - i // keep it symmetric when we can
			bi, bj := givens[i], givens[j]
			if bi == 0 {
				continue
			}
			givens[i], givens[j] = 0, 0
			b := append([]int(nil), givens...)
			if g.count(b, 2, nil, nil) != 1 {
				givens[i], givens[j] = bi, bj
				continue
			}
			if level == "easy" || level == "medium" {
				if ok, h := g.logic(givens); !ok || (level == "easy" && h > 1) {
					givens[i], givens[j] = bi, bj
					continue
				}
			}
			left -= 2
			if i == j {
				left++
			}
		}
		s := &Sudoku{Size: size, Givens: givens, Solution: sol, Level: level, Diagonal: diag}
		s.BoxW, s.BoxH = boxOf(size)
		got := rateSudoku(g, givens)
		if got == level || best == nil {
			best = s
			best.Level = got
		}
		if got == level || time.Now().After(deadline) {
			break
		}
	}
	return best
}

func rateSudoku(g *sgrid, givens []int) string {
	ok, h := g.logic(givens)
	switch {
	case !ok:
		return "expert"
	case h >= 3:
		return "hard"
	case h == 2:
		return "medium"
	}
	return "easy"
}

func cleanSudoku(s *Sudoku) error {
	if s.Size != 4 && s.Size != 6 && s.Size != 9 {
		return bad("a sudoku is 4, 6 or 9 squares wide")
	}
	s.BoxW, s.BoxH = boxOf(s.Size)
	n := s.Size * s.Size
	if len(s.Givens) != n {
		gv := make([]int, n)
		copy(gv, s.Givens)
		s.Givens = gv
	}
	for i, v := range s.Givens {
		if v < 0 || v > s.Size {
			s.Givens[i] = 0
		}
	}
	g := newSGrid(s.Size, s.Diagonal)
	// work the solution out from the givens when it is missing or does not agree
	agree := len(s.Solution) == n
	if agree {
		for i, v := range s.Givens {
			if v != 0 && s.Solution[i] != v {
				agree = false
			}
		}
	}
	if !agree {
		out := make([]int, n)
		if g.count(append([]int(nil), s.Givens...), 1, nil, out) == 1 {
			s.Solution = out
		} else {
			s.Solution = make([]int, n)
		}
	}
	if !oneOfS(s.Level, Levels) {
		s.Level = rateSudoku(g, s.Givens)
	}
	return nil
}

func publicSudoku(s *Sudoku) any {
	return map[string]any{"size": s.Size, "box_w": s.BoxW, "box_h": s.BoxH, "givens": s.Givens, "level": s.Level, "diagonal": s.Diagonal}
}

type numsAnswer struct {
	Cells []int `json:"cells"`
}

func checkSudoku(s *Sudoku, a *numsAnswer, final bool) Result {
	r := Result{Wrong: []int{}}
	for i, want := range s.Solution {
		if s.Givens[i] != 0 {
			continue
		}
		r.Total++
		got := 0
		if i < len(a.Cells) {
			got = a.Cells[i]
		}
		if got == 0 {
			continue
		}
		r.Filled++
		if got != want {
			r.Wrong = append(r.Wrong, i)
		}
	}
	r.Solved = r.Filled == r.Total && len(r.Wrong) == 0
	return r
}

func testSudoku(s *Sudoku) Report {
	r := newReport()
	g := newSGrid(s.Size, s.Diagonal)
	b := append([]int(nil), s.Givens...)
	for i, v := range b {
		if v != 0 {
			b[i] = 0
			if !g.can(b, i, v) {
				r.problem("Two of the same number clash (row %d, column %d)", i/s.Size+1, i%s.Size+1)
				b[i] = v
				return r.done()
			}
			b[i] = v
		}
	}
	switch n := g.count(append([]int(nil), s.Givens...), 2, nil, nil); n {
	case 0:
		r.problem("It cannot be solved: no way to fill it")
	case 1:
		r.Level = rateSudoku(g, s.Givens)
		r.note("One answer. Difficulty: %s", r.Level)
	default:
		r.problem("It has more than one answer — add a few more numbers")
	}
	given := 0
	for _, v := range s.Givens {
		if v != 0 {
			given++
		}
	}
	r.Stats["given"] = given
	return r.done()
}
