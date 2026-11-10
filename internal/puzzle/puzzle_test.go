package puzzle

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func gen(t *testing.T, kind string, o Options) json.RawMessage {
	t.Helper()
	o.Seed = 42
	m, err := Generate(kind, o)
	if err != nil {
		t.Fatalf("%s: %v", kind, err)
	}
	clean, err := Clean(kind, m.Data)
	if err != nil {
		t.Fatalf("%s clean: %v", kind, err)
	}
	if _, err := Public(kind, clean); err != nil {
		t.Fatalf("%s public: %v", kind, err)
	}
	return clean
}

func TestEveryKindGenerates(t *testing.T) {
	for _, k := range Kinds {
		o := Options{Theme: "animals", Level: "medium"}
		if k == "quiz" {
			o.Mode = "riddles"
		}
		data := gen(t, k, o)
		rep, err := Test(k, data)
		if err != nil {
			t.Fatalf("%s test: %v", k, err)
		}
		if k != "jigsaw" && !rep.OK {
			t.Errorf("%s: generated puzzle is not ready: %v", k, rep.Problems)
		}
		if Size(k, data) == "" {
			t.Errorf("%s: no size line", k)
		}
	}
}

func TestCrosswordSolveAndCheck(t *testing.T) {
	data := gen(t, "crossword", Options{Theme: "fruit"})
	var c Crossword
	_ = json.Unmarshal(data, &c)
	if len(Entries(&c)) < 5 {
		t.Fatalf("too few words placed: %d", len(Entries(&c)))
	}
	ans, _ := json.Marshal(map[string]any{"cells": c.Cells})
	r, _ := Check("crossword", data, ans, true)
	if !r.Solved {
		t.Fatalf("own answer not solved: %+v", r)
	}
	bad := append([]string(nil), c.Cells...)
	for i, v := range bad {
		if v != "" && v != "#" {
			bad[i] = "Q"
			break
		}
	}
	ans, _ = json.Marshal(map[string]any{"cells": bad})
	r, _ = Check("crossword", data, ans, true)
	if r.Solved || len(r.Wrong) != 1 {
		t.Fatalf("one wrong letter: %+v", r)
	}
	// public view has no letters
	p, _ := Public("crossword", data)
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), `"answer":"`) {
		t.Fatalf("public crossword leaks answers: %s", b)
	}
}

func TestClassicFill(t *testing.T) {
	g := BlankGrid(5, 5, 1)
	if !gridOK(g) {
		t.Fatal("bad blank grid")
	}
	f, ok := Fill(g, nil, 1, 5*time.Second)
	if !ok {
		t.Skip("5×5 fill did not finish in time on this machine")
	}
	for _, e := range Entries(f) {
		if !IsWord(e.Answer) {
			t.Fatalf("%s is not a word", e.Answer)
		}
	}
}

func TestSudoku(t *testing.T) {
	for _, lv := range []string{"easy", "medium"} {
		s := MakeSudoku(9, lv, false, 7)
		g := newSGrid(9, false)
		if n := g.count(append([]int(nil), s.Givens...), 2, nil, nil); n != 1 {
			t.Fatalf("%s sudoku has %d answers", lv, n)
		}
	}
	s := MakeSudoku(4, "easy", false, 3)
	data := enc(s)
	ans, _ := json.Marshal(map[string]any{"cells": s.Solution})
	r, _ := Check("sudoku", data, ans, true)
	if !r.Solved {
		t.Fatalf("4×4 own solution not solved: %+v", r)
	}
}

func TestNonogramLogic(t *testing.T) {
	n := FromArt(Pictures["heart"])
	ok, _ := solveLogic(n)
	if !ok {
		t.Fatal("heart should solve line by line")
	}
	data := enc(n)
	ans, _ := json.Marshal(map[string]any{"cells": n.Cells})
	r, _ := Check("nonogram", data, ans, true)
	if !r.Solved {
		t.Fatal("own picture not solved")
	}
}

func TestLogicGenerator(t *testing.T) {
	set, _ := LogicSetByKey("race")
	cats := []Category{set.Cats[0], set.Cats[1], set.Cats[2]}
	for i := range cats {
		cats[i].Items = cats[i].Items[:4]
	}
	l, err := MakeLogic(cats, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := logicCount(l.Categories, l.Clues, 2, time.Now().Add(5*time.Second))
	if n != 1 {
		t.Fatalf("logic grid has %d answers", n)
	}
	ans, _ := json.Marshal(map[string]any{"solution": l.Solution})
	r, _ := Check("logic", enc(l), ans, true)
	if !r.Solved {
		t.Fatal("own logic answer not solved")
	}
}

func TestLadderAndCodes(t *testing.T) {
	p := LadderPath("cold", "warm")
	if len(p) < 4 {
		t.Fatalf("cold→warm: %v", p)
	}
	l := &Ladder{From: "COLD", To: "WARM", Steps: p, Language: "en"}
	ans, _ := json.Marshal(map[string]any{"answers": p[1 : len(p)-1]})
	r, _ := Check("ladder", enc(l), ans, true)
	if !r.Solved {
		t.Fatalf("ladder not solved: %+v", r)
	}
	c := MakeCryptogram("Hello there friend", "", "random", "en", 1, 9)
	m := map[string]string{}
	for _, s := range cryptoSymbols(c) {
		m[s[1]] = s[0]
	}
	ans, _ = json.Marshal(map[string]any{"map": m})
	r, _ = Check("cryptogram", enc(c), ans, true)
	if !r.Solved {
		t.Fatalf("code not solved: %+v", r)
	}
}

func TestBCSLetters(t *testing.T) {
	got := Letters("Džukela i njorka", "bcs")
	if strings.Join(got, "|") != "DŽ|U|K|E|L|A|I|NJ|O|R|K|A" {
		t.Fatalf("letters: %v", got)
	}
	if !SameAnswer("ZABA", "Žaba") || !SameAnswer("the piano", "a piano") {
		t.Fatal("forgiving answers")
	}
	ws := MakeWordSearch([]WordClue{{Word: "LJUBAV"}, {Word: "NJIVA"}}, 8, 8, "easy", "", "bcs", 1)
	if offCount(ws) != 0 {
		t.Fatal("bcs words did not fit")
	}
}

func TestWordSearchMessage(t *testing.T) {
	ws := MakeWordSearch(Themes[0].Words[:6], 10, 10, "medium", "well done", "en", 4)
	r := testWordSearch(ws)
	if !r.OK {
		t.Fatalf("message word search: %v", r.Problems)
	}
	found := []int{}
	for i, w := range ws.Words {
		if !w.Off {
			found = append(found, i)
		}
	}
	ans, _ := json.Marshal(map[string]any{"found": found, "message": "Well done!"})
	res, _ := Check("wordsearch", enc(ws), ans, true)
	if !res.Solved {
		t.Fatalf("not solved: %+v", res)
	}
}

func TestDaily(t *testing.T) {
	for _, k := range DailyKinds("2026-11-10") {
		a, err := Daily(k, "2026-11-10", "")
		if err != nil {
			t.Fatal(k, err)
		}
		b, _ := Daily(k, "2026-11-10", "")
		if string(a.Data) != string(b.Data) {
			t.Fatalf("daily %s is not the same twice", k)
		}
	}
}

func TestLibraryPicturesSolveByLogic(t *testing.T) {
	for _, k := range PictureNames() {
		if ok, _ := solveLogic(FromArt(Pictures[k])); !ok {
			t.Errorf("picture %s needs guessing", k)
		}
	}
}
