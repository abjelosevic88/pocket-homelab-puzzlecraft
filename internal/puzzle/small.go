package puzzle

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ---- maze ----

const (
	wN = 1
	wE = 2
	wS = 4
	wW = 8
)

// MakeMaze carves a maze (a long winding way, one path between any two
// squares) from the top-left to the bottom-right.
func MakeMaze(w, h int, seed int64) *Maze {
	w, h = clampI(w, 4, 60), clampI(h, 4, 60)
	rng := rand.New(rand.NewSource(seed))
	walls := make([]int, w*h)
	for i := range walls {
		walls[i] = wN | wE | wS | wW
	}
	seen := make([]bool, w*h)
	stack := []int{0}
	seen[0] = true
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		r, c := cur/w, cur%w
		type nb struct{ i, wall, back int }
		var opts []nb
		if r > 0 && !seen[cur-w] {
			opts = append(opts, nb{cur - w, wN, wS})
		}
		if c < w-1 && !seen[cur+1] {
			opts = append(opts, nb{cur + 1, wE, wW})
		}
		if r < h-1 && !seen[cur+w] {
			opts = append(opts, nb{cur + w, wS, wN})
		}
		if c > 0 && !seen[cur-1] {
			opts = append(opts, nb{cur - 1, wW, wE})
		}
		if len(opts) == 0 {
			stack = stack[:len(stack)-1]
			continue
		}
		o := opts[rng.Intn(len(opts))]
		walls[cur] &^= o.wall
		walls[o.i] &^= o.back
		seen[o.i] = true
		stack = append(stack, o.i)
	}
	m := &Maze{W: w, H: h, Seed: seed, Walls: walls, Start: 0, End: w*h - 1}
	m.Path = solveMaze(m)
	return m
}

func solveMaze(m *Maze) []int {
	if len(m.Walls) != m.W*m.H {
		return nil
	}
	prev := make([]int, m.W*m.H)
	for i := range prev {
		prev[i] = -2
	}
	prev[m.Start] = -1
	q := []int{m.Start}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if cur == m.End {
			break
		}
		for _, nb := range mazeMoves(m, cur) {
			if prev[nb] == -2 {
				prev[nb] = cur
				q = append(q, nb)
			}
		}
	}
	if prev[m.End] == -2 {
		return nil
	}
	path := []int{}
	for at := m.End; at != -1; at = prev[at] {
		path = append(path, at)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func mazeMoves(m *Maze, i int) []int {
	out := []int{}
	r, c := i/m.W, i%m.W
	if r > 0 && m.Walls[i]&wN == 0 {
		out = append(out, i-m.W)
	}
	if c < m.W-1 && m.Walls[i]&wE == 0 {
		out = append(out, i+1)
	}
	if r < m.H-1 && m.Walls[i]&wS == 0 {
		out = append(out, i+m.W)
	}
	if c > 0 && m.Walls[i]&wW == 0 {
		out = append(out, i-1)
	}
	return out
}

func cleanMaze(m *Maze) error {
	if m.W < 4 || m.H < 4 || m.W > 60 || m.H > 60 {
		return bad("a maze is 4 to 60 squares each way")
	}
	if len(m.Walls) != m.W*m.H {
		*m = *MakeMaze(m.W, m.H, m.Seed)
	}
	if m.Start < 0 || m.Start >= m.W*m.H {
		m.Start = 0
	}
	if m.End < 0 || m.End >= m.W*m.H || m.End == m.Start {
		m.End = m.W*m.H - 1
	}
	m.Path = solveMaze(m)
	return nil
}

func publicMaze(m *Maze) any {
	return map[string]any{"w": m.W, "h": m.H, "walls": m.Walls, "start": m.Start, "end": m.End, "theme": m.Theme}
}

type pathAnswer struct {
	Path []int `json:"path"`
}

func checkMaze(m *Maze, a *pathAnswer, final bool) Result {
	r := Result{Total: len(m.Path)}
	if len(a.Path) == 0 || a.Path[0] != m.Start {
		r.Message = "Start at the green square"
		return r
	}
	for k := 1; k < len(a.Path); k++ {
		ok := false
		for _, nb := range mazeMoves(m, a.Path[k-1]) {
			if nb == a.Path[k] {
				ok = true
			}
		}
		if !ok {
			r.Wrong = []int{k}
			r.Message = "That goes through a wall"
			return r
		}
	}
	r.Filled = len(a.Path)
	r.Solved = a.Path[len(a.Path)-1] == m.End
	return r
}

func testMaze(m *Maze) Report {
	r := newReport()
	if len(solveMaze(m)) == 0 {
		r.problem("There is no way from the start to the end")
	}
	r.Stats["path"] = len(m.Path)
	return r.done()
}

// ---- secret codes ----

var symbolSet = []string{"★", "☀", "☂", "☘", "♠", "♣", "♥", "♦", "♪", "⚑", "✈", "✉", "✂", "✿", "❄", "☾", "⚓", "⚡", "☯", "♞", "⌂", "☕", "✎", "⚙", "☎", "♛", "⚘", "☁", "♜", "✦"}

// MakeCryptogram writes a text in a secret code.
func MakeCryptogram(text, author, cipher, lang string, given int, seed int64) *Cryptogram {
	if !oneOfS(cipher, []string{"random", "caesar", "numbers", "symbols"}) {
		cipher = "random"
	}
	rng := rand.New(rand.NewSource(seed))
	c := &Cryptogram{Text: strings.TrimSpace(text), Author: strings.TrimSpace(author), Cipher: cipher, Language: lang, Key: map[string]string{}, Given: []string{}}
	alpha := Alphabet(lang)
	switch cipher {
	case "caesar":
		c.Shift = 1 + rng.Intn(len(alpha)-1)
		for i, l := range alpha {
			c.Key[l] = alpha[(i+c.Shift)%len(alpha)]
		}
	case "numbers":
		perm := rng.Perm(len(alpha))
		for i, l := range alpha {
			c.Key[l] = strconv.Itoa(perm[i] + 1)
		}
	case "symbols":
		perm := rng.Perm(max(len(alpha), len(symbolSet)))
		k := 0
		for _, l := range alpha {
			for perm[k] >= len(symbolSet) {
				k++
			}
			c.Key[l] = symbolSet[perm[k]]
			k++
			if k >= len(perm) {
				break
			}
		}
		// languages with more letters than symbols: numbers for the rest
		for i, l := range alpha {
			if c.Key[l] == "" {
				c.Key[l] = strconv.Itoa(i + 1)
			}
		}
	default:
		// a shuffle where no letter stands for itself
		for {
			perm := rng.Perm(len(alpha))
			okp := true
			for i := range perm {
				if perm[i] == i {
					okp = false
					break
				}
			}
			if okp {
				for i, l := range alpha {
					c.Key[l] = alpha[perm[i]]
				}
				break
			}
		}
	}
	// give away the commonest letters first, as many as asked
	if given > 0 {
		counts := map[string]int{}
		for _, l := range Letters(c.Text, lang) {
			if c.Key[l] != "" {
				counts[l]++
			}
		}
		type lc struct {
			l string
			n int
		}
		var list []lc
		for l, n := range counts {
			list = append(list, lc{l, n})
		}
		sort.Slice(list, func(a, b int) bool {
			if list[a].n != list[b].n {
				return list[a].n > list[b].n
			}
			return list[a].l < list[b].l
		})
		for i := 0; i < given && i < len(list); i++ {
			c.Given = append(c.Given, list[i].l)
		}
	}
	return c
}

func cleanCryptogram(c *Cryptogram) error {
	c.Text = strings.TrimSpace(c.Text)
	if c.Text == "" || len(c.Text) > 2000 {
		return bad("a secret code needs a text (up to 2000 characters)")
	}
	if c.Language == "" {
		c.Language = "en"
	}
	if len(c.Key) == 0 {
		*c = *MakeCryptogram(c.Text, c.Author, c.Cipher, c.Language, len(c.Given), int64(len(c.Text)))
	}
	if c.Given == nil {
		c.Given = []string{}
	}
	for l := range c.Key {
		if !isLetterTok(l) {
			delete(c.Key, l)
		}
	}
	for _, l := range Letters(c.Text, c.Language) {
		if c.Key[l] == "" && isLetterTok(l) {
			c.Key[l] = l
		}
	}
	return nil
}

// isLetterTok: a letter (LJ/NJ/DŽ too), not a digit.
func isLetterTok(l string) bool {
	for _, r := range l {
		return unicode.IsLetter(r)
	}
	return false
}

// cryptoSymbols lists the text's letters and their symbols, once each, in order of appearance.
func cryptoSymbols(c *Cryptogram) [][2]string {
	seen := map[string]bool{}
	out := [][2]string{}
	for _, l := range Letters(c.Text, c.Language) {
		if seen[l] || !isLetterTok(l) {
			continue
		}
		seen[l] = true
		out = append(out, [2]string{l, c.Key[l]})
	}
	return out
}

// tokens splits the text into what is shown: letters as symbols, the rest as it is.
func cryptoTokens(c *Cryptogram) []map[string]any {
	out := []map[string]any{}
	rs := []rune(strings.ToUpper(norm.NFC.String(c.Text)))
	given := map[string]bool{}
	for _, g := range c.Given {
		given[g] = true
	}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		l := string(r)
		if c.Language == "bcs" && i+1 < len(rs) {
			if p := string([]rune{r, rs[i+1]}); p == "LJ" || p == "NJ" || p == "DŽ" {
				l = p
				i++
			}
		}
		if sym, ok := c.Key[l]; ok && isLetterTok(l) && (len(Letters(l, c.Language)) == 1) {
			t := map[string]any{"s": sym}
			if given[l] {
				t["l"] = l
			}
			out = append(out, t)
			continue
		}
		out = append(out, map[string]any{"t": l})
	}
	return out
}

func publicCryptogram(c *Cryptogram) any {
	m := map[string]any{"cipher": c.Cipher, "tokens": cryptoTokens(c), "author": "", "language": c.Language, "symbols": len(cryptoSymbols(c))}
	if c.Author != "" {
		m["author_code"] = encode(c, c.Author)
	}
	if c.Cipher == "caesar" {
		m["hint"] = "Every letter is moved the same number of places along the alphabet."
	}
	return m
}

func encode(c *Cryptogram, s string) string {
	var b strings.Builder
	for _, l := range Letters(s, c.Language) {
		if v, ok := c.Key[l]; ok {
			b.WriteString(v)
		}
	}
	return b.String()
}

type cryptoAnswer struct {
	Map    map[string]string `json:"map"` // symbol → the letter the player thinks
	Author string            `json:"author"`
}

func checkCryptogram(c *Cryptogram, a *cryptoAnswer, final bool) Result {
	r := Result{Wrong: []int{}}
	syms := cryptoSymbols(c)
	given := map[string]bool{}
	for _, g := range c.Given {
		given[g] = true
	}
	for i, s := range syms {
		if given[s[0]] {
			continue
		}
		r.Total++
		got := upper(a.Map[s[1]])
		if got == "" {
			continue
		}
		r.Filled++
		if got != s[0] && Fold(got) != Fold(s[0]) {
			r.Wrong = append(r.Wrong, i)
		} else {
			r.Right = append(r.Right, i)
		}
	}
	r.Solved = r.Filled == r.Total && len(r.Wrong) == 0
	if r.Solved {
		r.Message = c.Text
		if c.Author != "" {
			r.Message += " — " + c.Author
		}
	}
	return r
}

func testCryptogram(c *Cryptogram) Report {
	r := newReport()
	syms := cryptoSymbols(c)
	n := len(Letters(c.Text, c.Language))
	if n < 12 {
		r.note("Short texts are hard to crack: with %d letters there is little to go on", n)
	}
	same := 0
	for _, s := range syms {
		if s[0] == s[1] && c.Cipher == "random" {
			same++
		}
	}
	if same > 0 {
		r.note("%d %s for itself", same, pl(same, "letter stands", "letters stand"))
	}
	r.Stats["letters"] = n
	r.Stats["different"] = len(syms)
	return r.done()
}

// ---- word scramble ----

// ScrambleWord mixes a word's letters so it no longer reads as itself.
func ScrambleWord(word, lang string, rng *rand.Rand) string {
	l := Letters(word, lang)
	if len(l) < 2 {
		return strings.Join(l, "")
	}
	orig := strings.Join(l, "")
	for try := 0; try < 30; try++ {
		rng.Shuffle(len(l), func(a, b int) { l[a], l[b] = l[b], l[a] })
		if s := strings.Join(l, ""); s != orig {
			return s
		}
	}
	return orig
}

// MakeScramble mixes up each word; with a final answer, it circles letters
// in the words that spell it.
func MakeScramble(list []WordClue, final *ScFinal, lang string, seed int64) *Scramble {
	rng := rand.New(rand.NewSource(seed))
	s := &Scramble{}
	for _, w := range list {
		word := strings.Join(Letters(w.Word, lang), "")
		if word == "" {
			continue
		}
		s.Words = append(s.Words, ScWord{Word: word, Hint: strings.TrimSpace(w.Clue), Scrambled: ScrambleWord(w.Word, lang, rng)})
	}
	if final != nil && strings.TrimSpace(final.Answer) != "" {
		s.Final = &ScFinal{Clue: strings.TrimSpace(final.Clue), Answer: strings.TrimSpace(final.Answer)}
		need := Letters(final.Answer, lang)
		k := 0
		for i := range s.Words {
			l := Letters(s.Words[i].Word, lang)
			take := (len(need) - k + len(s.Words) - i - 1) / max(1, len(s.Words)-i)
			for p := 0; p < len(l) && take > 0 && k < len(need); p++ {
				if l[p] == need[k] {
					s.Words[i].Circles = append(s.Words[i].Circles, p)
					k++
					take--
				}
			}
		}
	}
	if s.Words == nil {
		s.Words = []ScWord{}
	}
	return s
}

func cleanScramble(s *Scramble) error {
	if len(s.Words) == 0 || len(s.Words) > 60 {
		return bad("a scramble has 1 to 60 words")
	}
	rng := rand.New(rand.NewSource(int64(len(s.Words))))
	for i := range s.Words {
		w := &s.Words[i]
		w.Word = strings.Join(Letters(w.Word, "en"), "")
		if w.Scrambled == "" || strings.EqualFold(w.Scrambled, w.Word) {
			w.Scrambled = ScrambleWord(w.Word, "en", rng)
		}
		w.Scrambled = upper(w.Scrambled)
	}
	return nil
}

func publicScramble(s *Scramble) any {
	ws := []map[string]any{}
	for _, w := range s.Words {
		ws = append(ws, map[string]any{"scrambled": w.Scrambled, "hint": w.Hint, "circles": w.Circles, "len": len([]rune(w.Word))})
	}
	m := map[string]any{"words": ws}
	if s.Final != nil {
		m["final"] = map[string]any{"clue": s.Final.Clue, "len": len(Letters(s.Final.Answer, "en"))}
	}
	return m
}

type textsAnswer struct {
	Answers []string `json:"answers"`
	Final   string   `json:"final"`
	Index   *int     `json:"index"` // check one only
}

func checkScramble(s *Scramble, a *textsAnswer, final bool) Result {
	r := Result{Total: len(s.Words), Wrong: []int{}}
	for i, w := range s.Words {
		got := ""
		if i < len(a.Answers) {
			got = a.Answers[i]
		}
		if strings.TrimSpace(got) == "" {
			continue
		}
		r.Filled++
		if SameAnswer(got, w.Word) || (IsWord(got) && sameLetters(got, w.Word)) {
			r.Right = append(r.Right, i)
		} else {
			r.Wrong = append(r.Wrong, i)
		}
	}
	r.Solved = len(r.Right) == len(s.Words)
	if s.Final != nil {
		r.Total++
		if strings.TrimSpace(a.Final) != "" {
			r.Filled++
			if SameAnswer(a.Final, s.Final.Answer) {
				r.Right = append(r.Right, -1)
			} else {
				r.Wrong = append(r.Wrong, -1)
				r.Solved = false
			}
		} else {
			r.Solved = false
		}
	}
	return r
}

func sameLetters(a, b string) bool {
	x, y := Letters(a, "en"), Letters(b, "en")
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, "") == strings.Join(y, "")
}

func testScramble(s *Scramble) Report {
	r := newReport()
	for _, w := range s.Words {
		if len([]rune(w.Word)) < 3 {
			r.note("%s is very short to unscramble", w.Word)
		}
	}
	if s.Final != nil {
		need := Letters(s.Final.Answer, "en")
		got := 0
		for _, w := range s.Words {
			got += len(w.Circles)
		}
		if got != len(need) {
			r.problem("The circled letters (%d) do not make the final answer (%d letters) — make the scramble again or change the words", got, len(need))
		}
	}
	r.Stats["words"] = len(s.Words)
	return r.done()
}

// ---- quiz ----

func cleanQuiz(q *Quiz) error {
	if len(q.Questions) == 0 || len(q.Questions) > 200 {
		return bad("a quiz has 1 to 200 questions")
	}
	if !oneOfS(q.Mode, []string{"quiz", "riddles", "trivia"}) {
		q.Mode = "quiz"
	}
	q.PassMark = clampI(q.PassMark, 0, 100)
	for i := range q.Questions {
		x := &q.Questions[i]
		x.Q = strings.TrimSpace(x.Q)
		if !oneOfS(x.Type, []string{"text", "choice", "number", "truefalse"}) {
			x.Type = "text"
		}
		if x.Type == "truefalse" {
			x.Choices = []string{"True", "False"}
		}
		ans := []string{}
		for _, a := range x.Answers {
			if a = strings.TrimSpace(a); a != "" {
				ans = append(ans, a)
			}
		}
		x.Answers = ans
		ch := []string{}
		for _, c := range x.Choices {
			if c = strings.TrimSpace(c); c != "" {
				ch = append(ch, c)
			}
		}
		x.Choices = ch
		if x.Points <= 0 {
			x.Points = 1
		}
	}
	return nil
}

func publicQuiz(q *Quiz) any {
	qs := []map[string]any{}
	for _, x := range q.Questions {
		m := map[string]any{"q": x.Q, "type": x.Type, "choices": x.Choices, "hint": x.Hint, "points": x.Points, "photo_id": x.PhotoID}
		if x.Type == "choice" && len(x.Answers) > 1 {
			m["many"] = true
		}
		qs = append(qs, m)
	}
	return map[string]any{"mode": q.Mode, "questions": qs, "pass_mark": q.PassMark}
}

func quizRight(x Question, got string) bool {
	if strings.TrimSpace(got) == "" {
		return false
	}
	switch x.Type {
	case "number":
		gv, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(got), ",", "."), 64)
		if err != nil {
			return false
		}
		for _, a := range x.Answers {
			if av, err := strconv.ParseFloat(strings.ReplaceAll(a, ",", "."), 64); err == nil && av == gv {
				return true
			}
		}
		return false
	case "choice":
		if len(x.Answers) > 1 {
			// several right choices, sent joined by "|"
			gotSet := map[string]bool{}
			for _, g := range strings.Split(got, "|") {
				if g = Answer(g); g != "" {
					gotSet[g] = true
				}
			}
			if len(gotSet) != len(x.Answers) {
				return false
			}
			for _, a := range x.Answers {
				if !gotSet[Answer(a)] {
					return false
				}
			}
			return true
		}
	}
	for _, a := range x.Answers {
		if SameAnswer(got, a) {
			return true
		}
	}
	return false
}

func checkQuiz(q *Quiz, a *textsAnswer, final bool) Result {
	r := Result{Total: len(q.Questions), Wrong: []int{}}
	for i, x := range q.Questions {
		r.Max += x.Points
		if a.Index != nil && *a.Index != i {
			continue
		}
		got := ""
		if i < len(a.Answers) {
			got = a.Answers[i]
		}
		if strings.TrimSpace(got) != "" {
			r.Filled++
		}
		if quizRight(x, got) {
			r.Right = append(r.Right, i)
			r.Score += x.Points
		} else if strings.TrimSpace(got) != "" || final {
			r.Wrong = append(r.Wrong, i)
		}
	}
	if final {
		for _, x := range q.Questions {
			r.Explain = append(r.Explain, x.Explain)
			r.Answers = append(r.Answers, strings.Join(x.Answers, " / "))
		}
		pct := 0
		if r.Max > 0 {
			pct = r.Score * 100 / r.Max
		}
		// a final check ends the quiz; with a pass mark only when it is reached
		r.Solved = q.PassMark == 0 || pct >= q.PassMark
		if !r.Solved {
			r.Message = fmt.Sprintf("%d%% — %d%% passes. Try again?", pct, q.PassMark)
		}
	} else {
		r.Solved = len(r.Right) == len(q.Questions)
	}
	return r
}

func testQuiz(q *Quiz) Report {
	r := newReport()
	for i, x := range q.Questions {
		n := i + 1
		if x.Q == "" && x.PhotoID == 0 {
			r.problem("Question %d has no question", n)
		}
		if len(x.Answers) == 0 {
			r.problem("Question %d has no answer", n)
		}
		if x.Type == "choice" {
			if len(x.Choices) < 2 {
				r.problem("Question %d needs at least two choices", n)
			}
			for _, a := range x.Answers {
				found := false
				for _, c := range x.Choices {
					if SameAnswer(a, c) {
						found = true
					}
				}
				if !found {
					r.problem("Question %d: the answer “%s” is not one of the choices", n, a)
				}
			}
		}
		if x.Type == "number" {
			for _, a := range x.Answers {
				if _, err := strconv.ParseFloat(strings.ReplaceAll(a, ",", "."), 64); err != nil {
					r.problem("Question %d: “%s” is not a number", n, a)
				}
			}
		}
	}
	r.Stats["questions"] = len(q.Questions)
	return r.done()
}

// ---- word ladder ----

func oneApart(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	d := 0
	for i := range a {
		if a[i] != b[i] {
			d++
		}
	}
	return d == 1
}

// LadderPath finds the shortest way from one word to another, one letter
// at a time through everyday English words (any word when everyday ones
// do not reach).
func LadderPath(from, to string) []string {
	from, to = upper(from), upper(to)
	if len(from) != len(to) || len(from) < 2 || len(from) > 8 {
		return nil
	}
	for _, commonOnly := range []bool{true, false} {
		pool := []string{}
		for _, w := range words().byLen[len(from)] {
			if commonOnly && !words().common[w] && w != from && w != to {
				continue
			}
			pool = append(pool, w)
		}
		if p := bfsLadder(from, to, pool); p != nil {
			return p
		}
	}
	return nil
}

func bfsLadder(from, to string, pool []string) []string {
	in := map[string]bool{}
	for _, w := range pool {
		in[w] = true
	}
	in[from], in[to] = true, true
	prev := map[string]string{from: ""}
	q := []string{from}
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if cur == to {
			break
		}
		rs := []rune(cur)
		for i := range rs {
			orig := rs[i]
			for ch := 'A'; ch <= 'Z'; ch++ {
				if ch == orig {
					continue
				}
				rs[i] = ch
				nw := string(rs)
				if in[nw] {
					if _, seen := prev[nw]; !seen {
						prev[nw] = cur
						q = append(q, nw)
					}
				}
			}
			rs[i] = orig
		}
	}
	if _, ok := prev[to]; !ok {
		return nil
	}
	path := []string{}
	for at := to; at != ""; at = prev[at] {
		path = append(path, at)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// MakeLadder picks a start and an end some steps apart.
func MakeLadder(length, steps int, seed int64) *Ladder {
	length = clampI(length, 3, 5)
	steps = clampI(steps, 3, 7)
	rng := rand.New(rand.NewSource(seed))
	pool := []string{}
	for _, w := range words().byLen[length] {
		if words().common[w] {
			pool = append(pool, w)
		}
	}
	var best []string
	for try := 0; try < 60 && len(pool) > 0; try++ {
		from := pool[rng.Intn(len(pool))]
		// walk outwards: the words at exactly `steps` from it
		dist := map[string]int{from: 0}
		prev := map[string]string{from: ""}
		q := []string{from}
		frontier := []string{}
		for len(q) > 0 {
			cur := q[0]
			q = q[1:]
			if dist[cur] == steps {
				frontier = append(frontier, cur)
				continue
			}
			rs := []rune(cur)
			for i := range rs {
				o := rs[i]
				for ch := 'A'; ch <= 'Z'; ch++ {
					rs[i] = ch
					nw := string(rs)
					if _, seen := dist[nw]; !seen && words().common[nw] {
						dist[nw] = dist[cur] + 1
						prev[nw] = cur
						q = append(q, nw)
					}
				}
				rs[i] = o
			}
		}
		if len(frontier) == 0 {
			continue
		}
		to := frontier[rng.Intn(len(frontier))]
		path := []string{}
		for at := to; at != ""; at = prev[at] {
			path = append(path, at)
		}
		for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
			path[i], path[j] = path[j], path[i]
		}
		best = path
		break
	}
	if best == nil {
		best = []string{"COLD", "CORD", "CARD", "WARD", "WARM"}
	}
	return &Ladder{From: best[0], To: best[len(best)-1], Steps: best, Language: "en"}
}

func cleanLadder(l *Ladder) error {
	l.From, l.To = upper(l.From), upper(l.To)
	if len([]rune(l.From)) != len([]rune(l.To)) || len([]rune(l.From)) < 2 {
		return bad("both words need the same number of letters")
	}
	if l.Language == "" {
		l.Language = "en"
	}
	steps := []string{}
	for _, s := range l.Steps {
		if s = upper(s); s != "" {
			steps = append(steps, s)
		}
	}
	if len(steps) < 2 || steps[0] != l.From || steps[len(steps)-1] != l.To {
		if p := LadderPath(l.From, l.To); p != nil {
			steps = p
		} else {
			steps = []string{l.From, l.To}
		}
	}
	l.Steps = steps
	return nil
}

func publicLadder(l *Ladder) any {
	return map[string]any{"from": l.From, "to": l.To, "steps": len(l.Steps) - 2, "len": len([]rune(l.From)), "language": l.Language}
}

func checkLadder(l *Ladder, a *textsAnswer, final bool) Result {
	r := Result{Total: len(l.Steps) - 2, Wrong: []int{}}
	chain := []string{l.From}
	known := map[string]bool{}
	for _, s := range l.Steps {
		known[s] = true
	}
	for i, w := range a.Answers {
		w = upper(w)
		if w == "" {
			continue
		}
		r.Filled++
		prev := []rune(chain[len(chain)-1])
		if !oneApart(prev, []rune(w)) || (l.Language == "en" && !IsWord(w) && !known[w]) {
			r.Wrong = append(r.Wrong, i)
			continue
		}
		r.Right = append(r.Right, i)
		chain = append(chain, w)
	}
	last := []rune(chain[len(chain)-1])
	r.Solved = len(r.Wrong) == 0 && (string(last) == l.To || oneApart(last, []rune(l.To))) && len(chain) > 1
	if len(l.Steps) == 2 {
		r.Solved = true
	}
	return r
}

func testLadder(l *Ladder) Report {
	r := newReport()
	for k := 1; k < len(l.Steps); k++ {
		if !oneApart([]rune(l.Steps[k-1]), []rune(l.Steps[k])) {
			r.problem("%s → %s changes more than one letter", l.Steps[k-1], l.Steps[k])
		}
	}
	if l.Language == "en" {
		for _, s := range l.Steps {
			if !IsWord(s) {
				r.note("%s is not in the dictionary", s)
			}
		}
		if p := LadderPath(l.From, l.To); p != nil && len(p) < len(l.Steps) {
			r.note("There is a shorter way: %s", strings.Join(p, " → "))
		}
	}
	r.Stats["steps"] = len(l.Steps) - 1
	return r.done()
}

// ---- jigsaw ----

func cleanJigsaw(j *Jigsaw) error {
	j.Cols, j.Rows = clampI(j.Cols, 2, 30), clampI(j.Rows, 2, 30)
	if j.Shape != "square" {
		j.Shape = "classic"
	}
	if j.Seed == 0 {
		j.Seed = int64(j.Cols*31 + j.Rows*7 + int(j.PhotoID))
	}
	return nil
}

type doneAnswer struct {
	Done bool `json:"done"`
}

func checkJigsaw(j *Jigsaw, a *doneAnswer, final bool) Result {
	return Result{Solved: a.Done, Total: j.Cols * j.Rows, Filled: j.Cols * j.Rows}
}

func clampI(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
