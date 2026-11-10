package puzzle

import (
	"encoding/json"
	"math/rand"
	"strings"
	"time"
)

// Options are what "make it for me" takes; each kind uses what it needs.
type Options struct {
	Words      []WordClue `json:"words"`      // crossword, word search, scramble, quiz
	Theme      string     `json:"theme"`      // a built-in word list instead of words
	W          int        `json:"w"`          // grid size
	H          int        `json:"h"`          //
	Size       int        `json:"size"`       // sudoku 4/6/9
	Level      string     `json:"level"`      // easy | medium | hard | expert
	Style      string     `json:"style"`      // crossword: freeform | classic
	Message    string     `json:"message"`    // word search: hidden message
	Text       string     `json:"text"`       // secret code: the text
	Author     string     `json:"author"`     //
	Cipher     string     `json:"cipher"`     // random | caesar | numbers | symbols
	Given      int        `json:"given"`      // secret code: letters given away
	Picture    string     `json:"picture"`    // nonogram: a picture from the library
	From       string     `json:"from"`       // ladder
	To         string     `json:"to"`         //
	Steps      int        `json:"steps"`      //
	Set        string     `json:"set"`        // logic: a category bank
	Cats       int        `json:"cats"`       // logic: categories
	Items      int        `json:"items"`      // logic: items per category
	Final      *ScFinal   `json:"final"`      // scramble: the final answer
	Categories []Category `json:"categories"` // logic: your own categories (a random answer, clues made for it)
	Mode       string     `json:"mode"`       // quiz: quiz | riddles
	Count      int        `json:"count"`      // quiz: questions
	Diagonal   bool       `json:"diagonal"`   // sudoku: X-sudoku
	Language   string     `json:"language"`
	Seed       int64      `json:"seed"`
}

// Made is a generated puzzle and what to tell about it.
type Made struct {
	Data    json.RawMessage `json:"data"`
	Title   string          `json:"title"`
	Level   string          `json:"level"`
	Missing []string        `json:"missing,omitempty"` // words that did not fit
	Message string          `json:"message,omitempty"`
}

func (o *Options) words() ([]WordClue, string) {
	lang := o.Language
	list := []WordClue{}
	for _, w := range o.Words {
		if strings.TrimSpace(w.Word) != "" {
			list = append(list, w)
		}
	}
	name := ""
	if len(list) == 0 {
		if t, ok := ThemeByKey(o.Theme); ok {
			list, name = t.Words, t.Name
			if lang == "" {
				lang = t.Language
			}
		}
	}
	if lang == "" {
		lang = "en"
	}
	o.Language = lang
	return list, name
}

// Generate makes a puzzle of a kind.
func Generate(kind string, o Options) (*Made, error) {
	if o.Seed == 0 {
		o.Seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(o.Seed))
	switch kind {
	case "crossword":
		list, name := o.words()
		if o.Style == "classic" {
			w, h := clampI(o.W, 3, 21), clampI(o.H, 3, 21)
			if o.W == 0 {
				w, h = 9, 9
			}
			if o.H == 0 {
				h = w
			}
			var best *Crossword
			for try := 0; try < 4; try++ {
				g := BlankGrid(w, h, o.Seed+int64(try))
				extra := []string{}
				for _, x := range list {
					extra = append(extra, strings.Join(Letters(x.Word, "en"), ""))
				}
				if f, ok := Fill(g, extra, o.Seed+int64(try), 4*time.Second); ok {
					best = f
					break
				}
				best = g
			}
			msg := ""
			if blankCount(best) > 0 {
				msg = "The grid is ready; filling every word did not work out in time — fill it yourself or try again."
			}
			return &Made{Data: enc(best), Title: firstNonEmpty(name, "Crossword"), Level: o.Level, Message: msg}, nil
		}
		if len(list) < 2 {
			return nil, bad("give at least two words (or pick a word list)")
		}
		c, missing := CrissCross(list, o.Language, clampI(firstPos(o.W, 21), 5, maxGrid), o.Seed)
		if c == nil {
			return nil, bad("no words to use")
		}
		return &Made{Data: enc(c), Title: firstNonEmpty(name, "Crossword"), Missing: missing}, nil
	case "wordsearch":
		list, name := o.words()
		if len(list) == 0 {
			return nil, bad("give some words (or pick a word list)")
		}
		longest := 0
		for _, w := range list {
			longest = max(longest, len(Letters(w.Word, o.Language)))
		}
		w := firstPos(o.W, clampI(longest+3, 10, 18))
		h := firstPos(o.H, w)
		ws := MakeWordSearch(list, w, h, o.Level, o.Message, o.Language, o.Seed)
		missing := []string{}
		for _, x := range ws.Words {
			if x.Off {
				missing = append(missing, x.Word)
			}
		}
		return &Made{Data: enc(ws), Title: firstNonEmpty(name, "Word search"), Level: ws.Level, Missing: missing}, nil
	case "sudoku":
		s := MakeSudoku(o.Size, o.Level, o.Diagonal, o.Seed)
		return &Made{Data: enc(s), Title: "Sudoku", Level: s.Level}, nil
	case "nonogram":
		art, ok := Pictures[o.Picture]
		if !ok {
			names := PictureNames()
			o.Picture = names[rng.Intn(len(names))]
			art = Pictures[o.Picture]
		}
		n := FromArt(art)
		return &Made{Data: enc(n), Title: "Picture puzzle", Level: "medium"}, nil
	case "cryptogram":
		text, author := strings.TrimSpace(o.Text), strings.TrimSpace(o.Author)
		lang := firstNonEmpty(o.Language, "en")
		if text == "" {
			pool := []Quote{}
			for _, q := range Quotes {
				bcs := strings.Contains(q.Author, "Narodna")
				if (lang == "bcs") == bcs {
					pool = append(pool, q)
				}
			}
			if len(pool) == 0 {
				pool = Quotes
			}
			q := pool[rng.Intn(len(pool))]
			text, author = q.Text, q.Author
		}
		given := o.Given
		if given == 0 {
			given = map[string]int{"easy": 4, "medium": 2, "hard": 1, "expert": 0}[o.Level]
		}
		c := MakeCryptogram(text, author, o.Cipher, lang, given, o.Seed)
		return &Made{Data: enc(c), Title: "Secret code", Level: o.Level}, nil
	case "scramble":
		list, name := o.words()
		if len(list) == 0 {
			return nil, bad("give some words (or pick a word list)")
		}
		if o.Count > 0 && len(list) > o.Count {
			rng.Shuffle(len(list), func(a, b int) { list[a], list[b] = list[b], list[a] })
			list = list[:o.Count]
		}
		s := MakeScramble(list, o.Final, o.Language, o.Seed)
		return &Made{Data: enc(s), Title: firstNonEmpty(name, "Word scramble")}, nil
	case "quiz":
		q := &Quiz{Mode: firstNonEmpty(o.Mode, "quiz")}
		count := clampI(firstPos(o.Count, 10), 1, 50)
		if q.Mode == "riddles" {
			perm := rng.Perm(len(Riddles))
			for _, i := range perm[:min(count, len(perm))] {
				q.Questions = append(q.Questions, Question{Q: Riddles[i].Q, Type: "text", Answers: []string{Riddles[i].A}, Points: 1})
			}
			return &Made{Data: enc(q), Title: "Riddles"}, nil
		}
		list, name := o.words()
		if len(list) == 0 {
			return nil, bad("give some words with clues (or pick a word list)")
		}
		perm := rng.Perm(len(list))
		for _, i := range perm[:min(count, len(perm))] {
			w := list[i]
			if strings.TrimSpace(w.Clue) == "" {
				continue
			}
			// multiple choice from the other words
			choices := []string{strings.TrimSpace(w.Word)}
			for _, j := range rng.Perm(len(list)) {
				if j != i && len(choices) < 4 {
					choices = append(choices, strings.TrimSpace(list[j].Word))
				}
			}
			rng.Shuffle(len(choices), func(a, b int) { choices[a], choices[b] = choices[b], choices[a] })
			tc := []string{}
			for _, c := range choices {
				tc = append(tc, titleWord(c))
			}
			q.Questions = append(q.Questions, Question{Q: w.Clue, Type: "choice", Choices: tc, Answers: []string{titleWord(w.Word)}, Points: 1})
		}
		if len(q.Questions) == 0 {
			return nil, bad("the words need clues to become questions")
		}
		return &Made{Data: enc(q), Title: firstNonEmpty(name, "Quiz")}, nil
	case "maze":
		size := map[string]int{"easy": 10, "medium": 16, "hard": 24, "expert": 34}[o.Level]
		w := firstPos(o.W, firstPos(size, 16))
		h := firstPos(o.H, w)
		m := MakeMaze(w, h, o.Seed)
		return &Made{Data: enc(m), Title: "Maze", Level: o.Level}, nil
	case "logic":
		if len(o.Categories) >= 2 {
			l, err := MakeLogic(o.Categories, "", o.Seed)
			if err != nil {
				return nil, err
			}
			return &Made{Data: enc(l), Title: "Logic grid", Level: o.Level}, nil
		}
		set, ok := LogicSetByKey(o.Set)
		if !ok {
			set = LogicSets[rng.Intn(len(LogicSets))]
		}
		cats := clampI(firstPos(o.Cats, map[string]int{"easy": 3, "medium": 3, "hard": 4, "expert": 4}[o.Level]), 2, len(set.Cats))
		if cats < 2 {
			cats = 3
		}
		items := clampI(firstPos(o.Items, map[string]int{"easy": 3, "medium": 4, "hard": 4, "expert": 5}[o.Level]), 3, 6)
		if cats >= 4 && items > 5 {
			items = 5
		}
		// keep an ordered category in when the bank has one
		order := rng.Perm(len(set.Cats))
		pick := []int{}
		for _, k := range order {
			if set.Cats[k].Ordered {
				pick = append(pick, k)
				break
			}
		}
		for _, k := range order {
			if len(pick) >= cats {
				break
			}
			if len(pick) > 0 && k == pick[0] {
				continue
			}
			pick = append(pick, k)
		}
		chosen := []Category{}
		// the first category is the people (or what the story is about)
		sortInts(pick)
		for _, k := range pick {
			c := set.Cats[k]
			idx := rng.Perm(len(c.Items))[:items]
			if c.Ordered {
				sortInts(idx)
			}
			it := []string{}
			for _, i := range idx {
				it = append(it, c.Items[i])
			}
			chosen = append(chosen, Category{Name: c.Name, Items: it, Ordered: c.Ordered})
		}
		l, err := MakeLogic(chosen, set.Story, o.Seed)
		if err != nil {
			return nil, err
		}
		return &Made{Data: enc(l), Title: set.Name, Level: o.Level}, nil
	case "ladder":
		if o.From != "" && o.To != "" {
			p := LadderPath(o.From, o.To)
			if p == nil {
				return nil, bad("no way from %s to %s one letter at a time", upper(o.From), upper(o.To))
			}
			return &Made{Data: enc(&Ladder{From: p[0], To: p[len(p)-1], Steps: p, Language: "en"}), Title: "Word ladder"}, nil
		}
		steps := firstPos(o.Steps, map[string]int{"easy": 3, "medium": 4, "hard": 5, "expert": 6}[o.Level])
		l := MakeLadder(firstPos(o.W, 4), firstPos(steps, 4), o.Seed)
		return &Made{Data: enc(l), Title: "Word ladder", Level: o.Level}, nil
	case "jigsaw":
		n := map[string]int{"easy": 3, "medium": 5, "hard": 7, "expert": 10}[o.Level]
		n = firstPos(n, 5)
		return &Made{Data: enc(&Jigsaw{Cols: firstPos(o.W, n), Rows: firstPos(o.H, n), Shape: "classic", Seed: o.Seed}), Title: "Jigsaw", Level: o.Level}, nil
	}
	return nil, bad("unknown kind %q", kind)
}

func blankCount(c *Crossword) int {
	n := 0
	for _, v := range c.Cells {
		if v == "" {
			n++
		}
	}
	return n
}

func firstPos(v ...int) int {
	for _, x := range v {
		if x > 0 {
			return x
		}
	}
	return 0
}

func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

func titleWord(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

// Daily is the puzzle of the day of a kind: the same for everyone on that
// date, different each day.
func Daily(kind, date string, level string) (*Made, error) {
	var seed int64 = 17
	for _, r := range date + kind {
		seed = seed*31 + int64(r)
	}
	t, _ := time.Parse("2006-01-02", date)
	wd := int(t.Weekday())
	if level == "" {
		// gentle at the start of the week, harder towards the weekend
		level = []string{"hard", "easy", "easy", "medium", "medium", "hard", "hard"}[wd]
	}
	o := Options{Level: level, Seed: seed}
	switch kind {
	case "wordsearch", "crossword", "scramble":
		rng := rand.New(rand.NewSource(seed))
		en := []Theme{}
		for _, th := range Themes {
			if th.Language == "en" {
				en = append(en, th)
			}
		}
		th := en[rng.Intn(len(en))]
		list := append([]WordClue(nil), th.Words...)
		rng.Shuffle(len(list), func(a, b int) { list[a], list[b] = list[b], list[a] })
		n := map[string]int{"easy": 8, "medium": 10, "hard": 12, "expert": 14}[level]
		o.Words = list[:min(n, len(list))]
		if kind == "scramble" {
			o.Words = list[:min(6, len(list))]
		}
		m, err := Generate(kind, o)
		if err == nil {
			m.Title, m.Level = th.Name, level
		}
		return m, err
	case "nonogram":
		names := PictureNames()
		o.Picture = names[int(seed%int64(len(names))+int64(len(names)))%len(names)]
	case "logic":
		o.Level = map[string]string{"easy": "easy", "medium": "easy", "hard": "medium", "expert": "hard"}[level]
	}
	m, err := Generate(kind, o)
	if err == nil && m.Level == "" {
		m.Level = level
	}
	return m, err
}

// DailyKinds are today's three: a sudoku, a word ladder and one that
// changes with the day of the week.
func DailyKinds(date string) []string {
	t, _ := time.Parse("2006-01-02", date)
	rot := []string{"maze", "wordsearch", "crossword", "cryptogram", "logic", "nonogram", "scramble"}[int(t.Weekday())]
	return []string{"sudoku", "ladder", rot}
}
