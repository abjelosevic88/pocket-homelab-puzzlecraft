package puzzle

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// A logic grid's possible answers: for each category k ≥ 1 a permutation
// matching its items to category 0's. With 4 categories of 5 items that
// is 120³ ≈ 1.7 million, small enough to try them all.

func permutations(n int) [][]int {
	var out [][]int
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	var rec func(k int)
	rec = func(k int) {
		if k == n {
			out = append(out, append([]int(nil), p...))
			return
		}
		for i := k; i < n; i++ {
			p[k], p[i] = p[i], p[k]
			rec(k + 1)
			p[k], p[i] = p[i], p[k]
		}
	}
	rec(0)
	return out
}

// row is which category-0 item an item belongs to, under a solution.
func rowOf(sol [][]int, r Ref) int {
	if r.Cat == 0 {
		return r.Item
	}
	for i, v := range sol[r.Cat] {
		if v == r.Item {
			return i
		}
	}
	return -1
}

func clueHolds(c Clue, sol [][]int) bool {
	a, b := rowOf(sol, c.A), rowOf(sol, c.B)
	switch c.Type {
	case "same":
		return a == b
	case "not":
		return a != b
	case "either":
		if c.C == nil {
			return false
		}
		cc := rowOf(sol, *c.C)
		return (a == b) != (a == cc)
	case "less", "more", "next":
		va, vb := sol[c.By][a], sol[c.By][b]
		switch c.Type {
		case "less":
			return va < vb
		case "more":
			return va > vb
		}
		return va-vb == 1 || vb-va == 1
	}
	return true
}

// logicCount counts the solutions the clues allow (up to limit), and keeps the first.
func logicCount(cats []Category, clues []Clue, limit int, deadline time.Time) (int, [][]int) {
	k := len(cats)
	if k < 2 {
		return 0, nil
	}
	n := len(cats[0].Items)
	perms := permutations(n)
	sol := make([][]int, k)
	sol[0] = perms[0]
	// clues sorted by the highest category they touch: check them as soon as possible
	byLevel := make([][]Clue, k)
	for _, c := range clues {
		lv := max(c.A.Cat, c.B.Cat)
		if c.C != nil {
			lv = max(lv, c.C.Cat)
		}
		if c.Type == "less" || c.Type == "more" || c.Type == "next" {
			lv = max(lv, c.By)
		}
		byLevel[lv] = append(byLevel[lv], c)
	}
	count := 0
	var first [][]int
	var rec func(cat int) bool
	rec = func(cat int) bool {
		if cat == k {
			count++
			if first == nil {
				first = make([][]int, k)
				for i := range sol {
					first[i] = append([]int(nil), sol[i]...)
				}
			}
			return count >= limit
		}
		for _, p := range perms {
			sol[cat] = p
			ok := true
			for _, c := range byLevel[cat] {
				if !clueHolds(c, sol) {
					ok = false
					break
				}
			}
			if ok && rec(cat+1) {
				return true
			}
			if time.Now().After(deadline) {
				return true
			}
		}
		return false
	}
	for _, c := range byLevel[0] {
		if !clueHolds(c, [][]int{perms[0]}) {
			return 0, nil
		}
	}
	rec(1)
	return count, first
}

func refName(cats []Category, r Ref) string {
	if r.Cat < 0 || r.Cat >= len(cats) || r.Item < 0 || r.Item >= len(cats[r.Cat].Items) {
		return "?"
	}
	return cats[r.Cat].Items[r.Item]
}

// ClueText writes a clue in plain words.
func ClueText(cats []Category, c Clue) string {
	a, b := refName(cats, c.A), refName(cats, c.B)
	by := ""
	if c.By >= 0 && c.By < len(cats) {
		by = strings.ToLower(cats[c.By].Name)
	}
	switch c.Type {
	case "same":
		return fmt.Sprintf("%s goes with %s.", a, b)
	case "not":
		return fmt.Sprintf("%s does not go with %s.", a, b)
	case "either":
		cc := "?"
		if c.C != nil {
			cc = refName(cats, *c.C)
		}
		return fmt.Sprintf("%s goes with either %s or %s.", a, b, cc)
	case "less":
		return fmt.Sprintf("%s comes before %s (by %s).", a, b, by)
	case "more":
		return fmt.Sprintf("%s comes after %s (by %s).", a, b, by)
	case "next":
		return fmt.Sprintf("%s and %s are next to each other (by %s).", a, b, by)
	}
	return ""
}

// MakeLogic makes a logic grid from categories: a random answer, then
// clues added until only that answer fits, then the clues it can do without
// taken out again.
func MakeLogic(cats []Category, story string, seed int64) (*Logic, error) {
	if len(cats) < 2 || len(cats) > 5 {
		return nil, bad("a logic grid has 2 to 5 categories")
	}
	n := len(cats[0].Items)
	if n < 3 || n > 6 {
		return nil, bad("each category has 3 to 6 items")
	}
	for _, c := range cats {
		if len(c.Items) != n {
			return nil, bad("every category needs the same number of items")
		}
	}
	if len(cats) == 5 && n > 5 {
		return nil, bad("five categories of six is too many to check — use fewer")
	}
	rng := rand.New(rand.NewSource(seed))
	sol := make([][]int, len(cats))
	sol[0] = make([]int, n)
	for i := range sol[0] {
		sol[0][i] = i
	}
	for k := 1; k < len(cats); k++ {
		sol[k] = rng.Perm(n)
	}
	ordered := []int{}
	for k, c := range cats {
		if c.Ordered {
			ordered = append(ordered, k)
		}
	}
	randRef := func() Ref {
		k := rng.Intn(len(cats))
		return Ref{k, rng.Intn(n)}
	}
	itemAt := func(cat, row int) Ref { return Ref{cat, sol[cat][row]} }
	newClue := func() (Clue, bool) {
		a := randRef()
		ra := rowOf(sol, a)
		bc := rng.Intn(len(cats))
		for bc == a.Cat {
			bc = rng.Intn(len(cats))
		}
		roll := rng.Intn(10)
		switch {
		case roll < 3:
			return Clue{Type: "same", A: a, B: itemAt(bc, ra)}, true
		case roll < 6:
			rb := rng.Intn(n)
			for rb == ra {
				rb = rng.Intn(n)
			}
			return Clue{Type: "not", A: a, B: itemAt(bc, rb)}, true
		case roll < 7:
			rb := rng.Intn(n)
			for rb == ra {
				rb = rng.Intn(n)
			}
			b, c := itemAt(bc, ra), itemAt(bc, rb)
			if rng.Intn(2) == 0 {
				b, c = c, b
			}
			return Clue{Type: "either", A: a, B: b, C: &c}, true
		default:
			if len(ordered) == 0 {
				return Clue{}, false
			}
			by := ordered[rng.Intn(len(ordered))]
			bcat := rng.Intn(len(cats))
			for bcat == by {
				bcat = rng.Intn(len(cats))
			}
			acat := a.Cat
			if acat == by {
				acat = bcat
				a = itemAt(acat, ra)
			}
			rb := rng.Intn(n)
			for rb == ra {
				rb = rng.Intn(n)
			}
			b := itemAt(bcat, rb)
			va, vb := sol[by][ra], sol[by][rb]
			switch {
			case va-vb == 1 || vb-va == 1:
				if rng.Intn(2) == 0 {
					return Clue{Type: "next", A: a, B: b, By: by}, true
				}
				fallthrough
			case va < vb:
				return Clue{Type: "less", A: a, B: b, By: by}, true
			default:
				return Clue{Type: "more", A: a, B: b, By: by}, true
			}
		}
	}
	deadline := time.Now().Add(6 * time.Second)
	clues := []Clue{}
	for guard := 0; guard < 400 && time.Now().Before(deadline); guard++ {
		c, ok := newClue()
		if !ok || !clueHolds(c, sol) {
			continue
		}
		clues = append(clues, c)
		if len(clues) >= 3 {
			if cnt, _ := logicCount(cats, clues, 2, deadline); cnt == 1 {
				break
			}
		}
	}
	if cnt, _ := logicCount(cats, clues, 2, deadline.Add(4*time.Second)); cnt != 1 {
		return nil, bad("could not make clues with one answer in time — try fewer categories or items")
	}
	// take out clues it can do without
	for i := len(clues) - 1; i >= 0 && time.Now().Before(deadline.Add(3*time.Second)); i-- {
		trial := append(append([]Clue(nil), clues[:i]...), clues[i+1:]...)
		if cnt, _ := logicCount(cats, trial, 2, time.Now().Add(2*time.Second)); cnt == 1 {
			clues = trial
		}
	}
	rng.Shuffle(len(clues), func(a, b int) { clues[a], clues[b] = clues[b], clues[a] })
	for i := range clues {
		clues[i].Text = ClueText(cats, clues[i])
	}
	return &Logic{Story: strings.TrimSpace(story), Categories: cats, Clues: clues, Solution: sol}, nil
}

func cleanLogic(l *Logic) error {
	if len(l.Categories) < 2 || len(l.Categories) > 5 {
		return bad("a logic grid has 2 to 5 categories")
	}
	n := len(l.Categories[0].Items)
	if n < 2 || n > 6 {
		return bad("each category has 2 to 6 items")
	}
	for i := range l.Categories {
		c := &l.Categories[i]
		c.Name = strings.TrimSpace(c.Name)
		if len(c.Items) != n {
			return bad("every category needs the same number of items")
		}
		for j := range c.Items {
			c.Items[j] = strings.TrimSpace(c.Items[j])
		}
	}
	if len(l.Solution) != len(l.Categories) {
		l.Solution = make([][]int, len(l.Categories))
	}
	for k := range l.Solution {
		if !isPerm(l.Solution[k], n) {
			l.Solution[k] = make([]int, n)
			for i := range l.Solution[k] {
				l.Solution[k][i] = i
			}
		}
	}
	for i := range l.Clues {
		c := &l.Clues[i]
		if c.Text == "" {
			c.Text = ClueText(l.Categories, *c)
		}
	}
	return nil
}

func isPerm(p []int, n int) bool {
	if len(p) != n {
		return false
	}
	seen := make([]bool, n)
	for _, v := range p {
		if v < 0 || v >= n || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}

func publicLogic(l *Logic) any {
	cl := []string{}
	for _, c := range l.Clues {
		cl = append(cl, c.Text)
	}
	return map[string]any{"story": l.Story, "categories": l.Categories, "clues": cl}
}

type gridAnswer struct {
	Solution [][]int `json:"solution"`
}

func checkLogic(l *Logic, a *gridAnswer, final bool) Result {
	r := Result{Wrong: []int{}}
	n := len(l.Categories[0].Items)
	for k := 1; k < len(l.Solution); k++ {
		for i := 0; i < n; i++ {
			r.Total++
			if k < len(a.Solution) && i < len(a.Solution[k]) && a.Solution[k][i] >= 0 {
				r.Filled++
				if a.Solution[k][i] != l.Solution[k][i] {
					r.Wrong = append(r.Wrong, k*100+i)
				}
			}
		}
	}
	r.Solved = r.Filled == r.Total && len(r.Wrong) == 0
	if !final {
		r.Wrong = []int{}
	}
	return r
}

func testLogic(l *Logic) Report {
	r := newReport()
	if len(l.Clues) == 0 {
		r.problem("There are no clues yet")
		return r.done()
	}
	for i, c := range l.Clues {
		if !clueHolds(c, l.Solution) {
			r.problem("Clue %d is not true for your answer: “%s”", i+1, c.Text)
		}
	}
	if len(r.Problems) > 0 {
		return r.done()
	}
	cnt, _ := logicCount(l.Categories, l.Clues, 2, time.Now().Add(8*time.Second))
	switch cnt {
	case 0:
		r.problem("No answer fits all the clues")
	case 1:
		r.note("One answer fits — it can be solved")
	default:
		r.problem("More than one answer fits the clues — add a clue")
	}
	r.Stats["clues"] = len(l.Clues)
	return r.done()
}
