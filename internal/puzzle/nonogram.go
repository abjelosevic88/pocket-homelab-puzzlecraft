package puzzle

import "strings"

func runs(line []int) []int {
	out := []int{}
	n := 0
	for _, v := range line {
		if v == 1 {
			n++
			continue
		}
		if n > 0 {
			out = append(out, n)
		}
		n = 0
	}
	if n > 0 {
		out = append(out, n)
	}
	return out
}

// NonogramClues works out the row and column numbers from the picture.
func NonogramClues(n *Nonogram) {
	n.Rows = make([][]int, n.H)
	n.Cols = make([][]int, n.W)
	for r := 0; r < n.H; r++ {
		n.Rows[r] = runs(n.Cells[r*n.W : (r+1)*n.W])
	}
	for c := 0; c < n.W; c++ {
		col := make([]int, n.H)
		for r := 0; r < n.H; r++ {
			col[r] = n.Cells[r*n.W+c]
		}
		n.Cols[c] = runs(col)
	}
}

func cleanNonogram(n *Nonogram) error {
	if n.W < 3 || n.H < 3 || n.W > 40 || n.H > 40 {
		return bad("a picture puzzle is 3 to 40 squares each way")
	}
	if len(n.Cells) != n.W*n.H {
		c := make([]int, n.W*n.H)
		copy(c, n.Cells)
		n.Cells = c
	}
	for i, v := range n.Cells {
		if v != 0 {
			n.Cells[i] = 1
		}
	}
	NonogramClues(n)
	return nil
}

// FromArt makes a nonogram from rows of text ("#" or "X" filled).
func FromArt(art []string) *Nonogram {
	h := len(art)
	w := 0
	for _, l := range art {
		w = max(w, len(l))
	}
	n := &Nonogram{W: w, H: h, Cells: make([]int, w*h)}
	for r, l := range art {
		for c, ch := range l {
			if ch == '#' || ch == 'X' {
				n.Cells[r*w+c] = 1
			}
		}
	}
	NonogramClues(n)
	return n
}

func publicNonogram(n *Nonogram) any {
	return map[string]any{"w": n.W, "h": n.H, "rows": n.Rows, "cols": n.Cols, "color": n.Color}
}

func checkNonogram(n *Nonogram, a *numsAnswer, final bool) Result {
	r := Result{Wrong: []int{}}
	// solved when every row and column matches its numbers (another picture
	// that fits the numbers counts too)
	got := make([]int, n.W*n.H)
	for i := range got {
		if i < len(a.Cells) && a.Cells[i] == 1 {
			got[i] = 1
			r.Filled++
		}
		if n.Cells[i] == 1 {
			r.Total++
		}
	}
	ok := true
	for row := 0; row < n.H; row++ {
		if !sameInts(runs(got[row*n.W:(row+1)*n.W]), n.Rows[row]) {
			ok = false
			r.Wrong = append(r.Wrong, row)
		}
	}
	for c := 0; c < n.W; c++ {
		col := make([]int, n.H)
		for row := 0; row < n.H; row++ {
			col[row] = got[row*n.W+c]
		}
		if !sameInts(runs(col), n.Cols[c]) {
			ok = false
			r.Wrong = append(r.Wrong, 1000+c)
		}
	}
	r.Solved = ok
	if !final {
		r.Wrong = []int{} // only on "check": which rows (0…) and columns (1000+) are off
	}
	return r
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// lineSolve narrows one line: cells -1 unknown, 0 empty, 1 filled.
// It returns false when the line cannot fit its numbers. Dynamic
// programming over (run, position): O(runs × n²).
func lineSolve(line []int, clue []int) ([]int, bool) {
	n, K := len(line), len(clue)
	// ok[k][p]: runs k… fit in line[p:] (p may be n)
	ok := make([][]bool, K+1)
	for k := range ok {
		ok[k] = make([]bool, n+2)
	}
	for p := n; p >= 0; p-- {
		allEmpty := true
		for i := p; i < n; i++ {
			if line[i] == 1 {
				allEmpty = false
				break
			}
		}
		ok[K][p] = allEmpty
	}
	runFits := func(start, length int) bool {
		if start+length > n {
			return false
		}
		for i := start; i < start+length; i++ {
			if line[i] == 0 {
				return false
			}
		}
		return start+length == n || line[start+length] != 1
	}
	for k := K - 1; k >= 0; k-- {
		for p := n; p >= 0; p-- {
			for s := p; s < n; s++ {
				if s > p && line[s-1] == 1 {
					break
				}
				if runFits(s, clue[k]) {
					next := s + clue[k]
					if next < n {
						next++
					}
					if ok[k+1][next] {
						ok[k][p] = true
						break
					}
				}
			}
		}
	}
	if !ok[0][0] {
		return line, false
	}
	canFill := make([]bool, n)
	canEmpty := make([]bool, n)
	pre := make([][]bool, K+1)
	for k := range pre {
		pre[k] = make([]bool, n+2)
	}
	pre[0][0] = true
	for k := 0; k < K; k++ {
		for p := 0; p <= n; p++ {
			if !pre[k][p] || !ok[k][p] {
				continue
			}
			for s := p; s < n; s++ {
				if s > p && line[s-1] == 1 {
					break
				}
				if !runFits(s, clue[k]) {
					continue
				}
				end := s + clue[k]
				next := end
				if end < n {
					next = end + 1
				}
				if !ok[k+1][next] {
					continue
				}
				for i := p; i < s; i++ {
					canEmpty[i] = true
				}
				for i := s; i < end; i++ {
					canFill[i] = true
				}
				if end < n {
					canEmpty[end] = true
				}
				pre[k+1][next] = true
			}
		}
	}
	for p := 0; p <= n; p++ {
		if pre[K][p] && ok[K][p] {
			for i := p; i < n; i++ {
				canEmpty[i] = true
			}
		}
	}
	out := append([]int(nil), line...)
	for i := range out {
		if out[i] != -1 {
			continue
		}
		if canFill[i] && !canEmpty[i] {
			out[i] = 1
		} else if canEmpty[i] && !canFill[i] {
			out[i] = 0
		}
	}
	return out, true
}

// solveLogic solves a nonogram one line at a time; it returns whether that
// alone finishes it (a fair puzzle needs no guessing).
func solveLogic(n *Nonogram) (bool, []int) {
	g := make([]int, n.W*n.H)
	for i := range g {
		g[i] = -1
	}
	for pass := 0; pass < 200; pass++ {
		changed := false
		for r := 0; r < n.H; r++ {
			line := g[r*n.W : (r+1)*n.W]
			out, ok := lineSolve(line, n.Rows[r])
			if !ok {
				return false, g
			}
			for i := range out {
				if line[i] != out[i] {
					line[i] = out[i]
					changed = true
				}
			}
		}
		for c := 0; c < n.W; c++ {
			line := make([]int, n.H)
			for r := 0; r < n.H; r++ {
				line[r] = g[r*n.W+c]
			}
			out, ok := lineSolve(line, n.Cols[c])
			if !ok {
				return false, g
			}
			for r := range out {
				if g[r*n.W+c] != out[r] {
					g[r*n.W+c] = out[r]
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	for _, v := range g {
		if v == -1 {
			return false, g
		}
	}
	return true, g
}

func testNonogram(n *Nonogram) Report {
	r := newReport()
	filled := 0
	for _, v := range n.Cells {
		filled += v
	}
	if filled == 0 {
		r.problem("The picture is empty")
		return r.done()
	}
	ok, g := solveLogic(n)
	if ok {
		r.note("It can be solved line by line, with no guessing")
	} else {
		unknown := 0
		for _, v := range g {
			if v == -1 {
				unknown++
			}
		}
		r.problem("%d %s cannot be worked out from the numbers alone — the picture may have more than one answer. Fill or clear a square in the grey area", unknown, pl(unknown, "square", "squares"))
		r.Stats["unknown"] = g
	}
	r.Stats["filled"] = filled
	return r.done()
}

// Pictures: a small library for daily puzzles and to start from.
var Pictures = map[string][]string{
	"boat": {
		"....#.....",
		"....##....",
		"....###...",
		"....####..",
		"....#####.",
		"....#.....",
		"##########",
		".########.",
		"..######..",
		"##.####.##",
	},
	"cat": {
		"#.......#.",
		"##.....##.",
		"#########.",
		"#.##.##.#.",
		"#########.",
		"###.#.###.",
		".#######..",
		"..#####...",
		".#######.#",
		"#########.",
	},
	"fish": {
		"..........",
		"....###...",
		"#..#####..",
		"##########",
		"####.#####",
		"##########",
		"#..#####..",
		"....###...",
		"..........",
		"..........",
	},
	"heart": {
		"..........",
		".##....##.",
		"####..####",
		"##########",
		"##########",
		".########.",
		"..######..",
		"...####...",
		"....##....",
		"..........",
	},
	"house": {
		"....##....",
		"...####...",
		"..######..",
		".########.",
		"##########",
		".#......#.",
		".#.##.#.#.",
		".#.##...#.",
		".#.##...#.",
		".########.",
	},
	"tree": {
		"....##....",
		"...####...",
		"..######..",
		"...####...",
		"..######..",
		".########.",
		"..######..",
		".########.",
		"##########",
		"....##....",
	},
	"umbrella": {
		"....#.....",
		"..#####...",
		".#######..",
		"#########.",
		"#.#.#.#.#.",
		"....#.....",
		"....#.....",
		"....#.....",
		".#..#.....",
		"..##......",
	},
	"apple": {
		"....#.....",
		"...#......",
		".###.###..",
		"#########.",
		"#########.",
		"#########.",
		"#########.",
		".#######..",
		"..#####...",
		"..........",
	},
	"mushroom": {
		"...####...",
		".########.",
		"##.####.##",
		"##########",
		"#.##..##.#",
		"...#..#...",
		"...#..#...",
		"...####...",
		"..........",
		"..........",
	},
	"note": {
		"....######",
		"....#....#",
		"....#....#",
		"....#....#",
		"....#....#",
		"..###..###",
		".####.####",
		".####.####",
		"..##...##.",
		"..........",
	},
	"car": {
		"..........",
		"...####...",
		"..#.#..#..",
		".########.",
		"##########",
		"##########",
		".##....##.",
		".##....##.",
		"..........",
		"..........",
	},
	"star": {
		"....##....",
		"....##....",
		"...####...",
		"##########",
		".########.",
		"..######..",
		"..######..",
		".###..###.",
		".##....##.",
		"##......##",
	},
	"smile": {
		"..######..",
		".########.",
		"##.####.##",
		"##.####.##",
		"##########",
		"#.######.#",
		"##.####.##",
		"###....###",
		".########.",
		"..######..",
	},
	"bird": {
		"..........",
		"...##.....",
		"..####....",
		".##.####..",
		"########..",
		"..#######.",
		"...#######",
		"....#####.",
		".....#.#..",
		"....##.##.",
	},
	"key": {
		".####.....",
		"##..##....",
		"##..######",
		"##..##.#.#",
		".####..#.#",
		"..........",
		"..........",
		"..........",
		"..........",
		"..........",
	},
	"duck": {
		"...###....",
		"..#####...",
		"..##.###..",
		"..######..",
		"...###....",
		".#######..",
		"##########",
		"##########",
		".########.",
		"..######..",
	},
}

// PictureNames lists the library's pictures.
func PictureNames() []string {
	out := []string{}
	for k := range Pictures {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && strings.Compare(a[j-1], a[j]) > 0; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}
