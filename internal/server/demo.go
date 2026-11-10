package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"time"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// seedDemo fills the owner's account with months of made-up puzzles and
// plays (DEMO_DATA=true): for trying it out, CI and the screenshots.
func (s *Server) seedDemo(ctx context.Context, uid int64) error {
	// with AUTH_MODE=none this runs inside the first request; finish even if that client leaves
	ctx = context.WithoutCancel(ctx)
	if n, _ := s.store.KindCounts(ctx, uid); len(n) > 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(42))
	now := time.Now()
	add := func(kind, title, level string, data json.RawMessage, shared bool, tags ...string) *store.Puzzle {
		p := &store.Puzzle{UserID: uid, Kind: kind, Title: title, Level: level, Language: "en", Data: data, ShareToken: newToken(), ShareOn: shared, Board: true, Reveal: true, Timer: true, Tags: tags}
		if err := prepare(p); err != nil {
			s.log.Warn("demo puzzle", "title", title, "err", err)
			return nil
		}
		if err := s.store.CreatePuzzle(ctx, p); err != nil {
			s.log.Warn("demo puzzle", "title", title, "err", err)
			return nil
		}
		return p
	}
	gen := func(kind string, o puzzle.Options) json.RawMessage {
		o.Seed = rng.Int63()
		m, err := puzzle.Generate(kind, o)
		if err != nil {
			s.log.Warn("demo generate", "kind", kind, "err", err)
			return nil
		}
		return m.Data
	}
	// a finished go some days ago
	played := func(what string, ref int64, kind, via, player string, daysAgo int, ms int64, hints int, day string) {
		start := now.AddDate(0, 0, -daysAgo).Add(-time.Duration(rng.Intn(10)) * time.Hour)
		p := &store.Play{UserID: uid, What: what, RefID: ref, Kind: kind, Via: via, Player: player, Key: newKey(), Day: day, StartedAt: start.UTC().Format(time.RFC3339Nano)}
		if err := s.store.CreatePlay(ctx, p); err != nil {
			return
		}
		fin := start.Add(time.Duration(ms) * time.Millisecond).UTC().Format(time.RFC3339Nano)
		p.FinishedAt, p.Ms, p.Hints, p.Mistakes = &fin, ms, hints, rng.Intn(4)
		_ = s.store.UpdatePlay(ctx, p)
		_, _ = s.store.DB.ExecContext(ctx, `UPDATE plays SET last_at = ? WHERE id = ?`, fin, p.ID)
	}
	friends := []string{"Ana", "Marko", "Lejla", "Ivan", "Sara", "Emir", "Nina", "Tom"}

	// word lists
	for _, l := range []store.WordList{
		{Name: "Spanish food words", Language: "other", Words: []store.WordClue{{Word: "manzana", Clue: "apple"}, {Word: "pan", Clue: "bread"}, {Word: "queso", Clue: "cheese"},
			{Word: "leche", Clue: "milk"}, {Word: "huevo", Clue: "egg"}, {Word: "pescado", Clue: "fish"}, {Word: "arroz", Clue: "rice"}, {Word: "naranja", Clue: "orange"}, {Word: "pollo", Clue: "chicken"}}},
		{Name: "Ana's birthday", Language: "en", Words: []store.WordClue{{Word: "ANA", Clue: "The birthday girl"}, {Word: "MOSTAR", Clue: "Where she was born"}, {Word: "VIOLIN", Clue: "What she plays"},
			{Word: "PANCAKES", Clue: "Her favourite breakfast"}, {Word: "BISCUIT", Clue: "Her dog"}, {Word: "THIRTY", Clue: "Her age this year"}, {Word: "PARIS", Clue: "Her dream trip"}}},
	} {
		l.UserID = uid
		_ = s.store.SaveWordList(ctx, &l)
	}

	// a hand-made mini crossword
	mini := &puzzle.Crossword{W: 5, H: 5, Style: "classic", Symmetry: "rotational", Language: "en", Cells: []string{
		"V", "I", "A", "L", "S", "R", "#", "L", "#", "L", "O", "L", "I", "V", "E", "O", "#", "B", "#", "E", "M", "O", "I", "S", "T"},
		Clues: map[string]string{"1A": "Small glass bottles for medicine", "1D": "Sound of a racing car", "2D": "Proof you were somewhere else", "3D": "Icy rain",
			"4A": "Small fruit pressed for oil", "5A": "Slightly wet, like a good cake"}}
	pMini := add("crossword", "Monday mini", "easy", mustJSON(mini), true, "mini")
	pCross := add("crossword", "Animal criss-cross", "easy", gen("crossword", puzzle.Options{Theme: "animals"}), true, "kids", "animals")
	pBirthday := add("crossword", "Ana's birthday crossword", "medium", gen("crossword", puzzle.Options{Words: []puzzle.WordClue{{Word: "ANA", Clue: "The birthday girl"},
		{Word: "MOSTAR", Clue: "Where she was born"}, {Word: "VIOLIN", Clue: "What she plays"}, {Word: "PANCAKES", Clue: "Her favourite breakfast"}, {Word: "BISCUIT", Clue: "Her dog"},
		{Word: "THIRTY", Clue: "Her age this year"}, {Word: "PARIS", Clue: "Her dream trip"}}}), true, "birthday")
	pSea := add("wordsearch", "Under the sea", "medium", gen("wordsearch", puzzle.Options{Theme: "ocean", Level: "medium", Message: "dive in", W: 13, H: 13}), true, "kids")
	pSpace := add("wordsearch", "Space words", "hard", gen("wordsearch", puzzle.Options{Theme: "space", Level: "hard", W: 14, H: 14}), false)
	pSudoku := add("sudoku", "Sunday sudoku", "hard", gen("sudoku", puzzle.Options{Size: 9, Level: "hard"}), true)
	pSudokuE := add("sudoku", "Gentle sudoku", "easy", gen("sudoku", puzzle.Options{Size: 9, Level: "easy"}), false)
	pLittle := add("sudoku", "Little sudoku for Mia", "easy", gen("sudoku", puzzle.Options{Size: 4, Level: "easy"}), true, "kids")
	pCat := add("nonogram", "Mystery picture", "medium", mustJSON(puzzle.FromArt(puzzle.Pictures["cat"])), true)
	pRocket := add("nonogram", "Starry night", "easy", mustJSON(puzzle.FromArt(puzzle.Pictures["star"])), false, "kids")
	pCode := add("cryptogram", "A message from Tolkien", "medium", mustJSON(puzzle.MakeCryptogram("Not all those who wander are lost.", "J. R. R. Tolkien", "symbols", "en", 2, 7)), true)
	pCaesar := add("cryptogram", "Spy school: Caesar", "easy", mustJSON(puzzle.MakeCryptogram("Meet me at the old bridge at noon.", "", "caesar", "en", 0, 11)), false, "spy")
	pJumble := add("scramble", "Kitchen jumble", "easy", mustJSON(puzzle.MakeScramble([]puzzle.WordClue{{Word: "kettle", Clue: "Boils water"}, {Word: "spoon", Clue: "For soup"},
		{Word: "plate", Clue: "You eat from it"}, {Word: "whisk", Clue: "Beats eggs"}, {Word: "apron", Clue: "Keeps you clean"}}, &puzzle.ScFinal{Clue: "What you drink at five", Answer: "tea"}, "en", 3)), true)
	quiz := &puzzle.Quiz{Mode: "quiz", PassMark: 50, Questions: []puzzle.Question{
		{Q: "Which is the longest river in the world?", Type: "choice", Choices: []string{"Amazon", "Nile", "Danube", "Yangtze"}, Answers: []string{"Nile"}, Explain: "The Nile is about 6,650 km long, just ahead of the Amazon.", Points: 1},
		{Q: "How many sides does a hexagon have?", Type: "number", Answers: []string{"6"}, Points: 1},
		{Q: "What is the capital of Australia?", Type: "text", Answers: []string{"Canberra"}, Hint: "Not Sydney!", Points: 1},
		{Q: "The Great Wall of China can be seen from the Moon with the naked eye.", Type: "truefalse", Answers: []string{"False"}, Explain: "It is far too narrow.", Points: 1},
		{Q: "Which planet is known as the red planet?", Type: "choice", Choices: []string{"Venus", "Mars", "Jupiter", "Mercury"}, Answers: []string{"Mars"}, Points: 1},
		{Q: "In which city is the Old Bridge (Stari most)?", Type: "text", Answers: []string{"Mostar"}, Points: 2},
		{Q: "Which of these are mammals?", Type: "choice", Choices: []string{"Dolphin", "Shark", "Bat", "Penguin"}, Answers: []string{"Dolphin", "Bat"}, Explain: "Dolphins and bats feed their young milk.", Points: 2},
	}}
	pQuiz := add("quiz", "Pub quiz: around the world", "medium", mustJSON(quiz), true, "party")
	pRiddles := add("quiz", "Ten old riddles", "easy", gen("quiz", puzzle.Options{Mode: "riddles", Count: 10}), true)
	pMaze := add("maze", "Pirate maze", "medium", gen("maze", puzzle.Options{Level: "medium"}), true, "kids")
	pLogic := add("logic", "The fun run", "medium", gen("logic", puzzle.Options{Set: "race", Cats: 3, Items: 4}), true)
	pLadder := add("ladder", "Cold to warm", "easy", mustJSON(&puzzle.Ladder{From: "COLD", To: "WARM", Steps: puzzle.LadderPath("COLD", "WARM"), Language: "en"}), true)
	var pJig *store.Puzzle
	if ph, err := s.savePhoto(ctx, uid, "puzzle", demoPicture()); err == nil {
		pJig = add("jigsaw", "Sunset over the hills", "medium", mustJSON(&puzzle.Jigsaw{PhotoID: ph.ID, Cols: 5, Rows: 4, Shape: "classic", Seed: 9}), true)
	}
	_ = pSpace
	_ = pSudokuE
	_ = pRocket
	_ = pCaesar

	// friends playing what was shared, and your own goes
	for _, p := range []*store.Puzzle{pMini, pCross, pBirthday, pSea, pSudoku, pLittle, pCat, pCode, pJumble, pQuiz, pRiddles, pMaze, pLogic, pLadder, pJig} {
		if p == nil {
			continue
		}
		base := map[string]int64{"crossword": 240, "wordsearch": 420, "sudoku": 900, "nonogram": 600, "cryptogram": 480, "scramble": 200, "quiz": 300, "maze": 150, "logic": 700, "ladder": 120, "jigsaw": 540}[p.Kind]
		if p.Kind == "sudoku" && p.Level == "easy" {
			base = 120
		}
		n := 2 + rng.Intn(5)
		for i := 0; i < n; i++ {
			ms := (base + int64(rng.Intn(int(base)))) * 1000
			played("puzzle", p.ID, p.Kind, "link", friends[rng.Intn(len(friends))], rng.Intn(40), ms, rng.Intn(3), "")
		}
		if rng.Intn(3) > 0 {
			played("puzzle", p.ID, p.Kind, "me", "Aleksandar", rng.Intn(30), (base+int64(rng.Intn(int(base)/2)))*1000, rng.Intn(2), "")
		}
	}
	// a go in the middle (letters filled in so far)
	if pSudoku != nil {
		p := &store.Play{UserID: uid, What: "puzzle", RefID: pSudoku.ID, Kind: "sudoku", Via: "me", Player: "Aleksandar", Key: newKey(), StartedAt: now.Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)}
		_ = s.store.CreatePlay(ctx, p)
	}

	// puzzles of the day: a long run up to yesterday
	for d := 1; d <= 46; d++ {
		if d == 19 || d == 33 {
			continue // missed days break the streak
		}
		day := store.AddDays(today(), -d)
		for _, k := range puzzle.DailyKinds(day) {
			if rng.Intn(5) == 0 {
				continue
			}
			base := map[string]int64{"sudoku": 600, "ladder": 90, "maze": 120, "wordsearch": 300, "crossword": 260, "cryptogram": 400, "logic": 500, "nonogram": 420, "scramble": 160}[k]
			played("daily", 0, k, "me", "Aleksandar", d, (base+int64(rng.Intn(int(base))))*1000, rng.Intn(2), day+"|"+k)
		}
	}

	// a puzzle book
	ids := []int64{}
	for _, p := range []*store.Puzzle{pMini, pSea, pLittle, pCat, pJumble, pMaze, pLadder} {
		if p != nil {
			ids = append(ids, p.ID)
		}
	}
	book := &store.Book{UserID: uid, Title: "Rainy day puzzle book", Subtitle: "For the long drive to the seaside", Answers: "back", ShareToken: newToken(), ShareOn: true}
	_ = s.store.SaveBook(ctx, book, ids)
	book2 := &store.Book{UserID: uid, Title: "Office quiz night", Subtitle: "Friday, round two", Answers: "back", ShareToken: newToken()}
	ids2 := []int64{}
	for _, p := range []*store.Puzzle{pQuiz, pRiddles, pLogic, pCode} {
		if p != nil {
			ids2 = append(ids2, p.ID)
		}
	}
	_ = s.store.SaveBook(ctx, book2, ids2)

	// adventures: an escape room next Saturday (two teams have played it), a party hunt
	sat := now
	for sat.Weekday() != time.Saturday || !sat.After(now) {
		sat = sat.AddDate(0, 0, 1)
	}
	mk := func(tpl string, date, tm, place, players string, shared bool, doneN int) *store.Adventure {
		t := adventureTemplates[tpl]
		a := &store.Adventure{UserID: uid, Kind: t.kind, Title: t.title, Intro: t.intro, Finale: t.finale, Theme: t.theme, Minutes: t.minutes, HintPenalty: 60, Status: "ready",
			Date: date, Time: tm, Place: place, Players: players, ShareToken: newToken(), ShareOn: shared, Board: true}
		for i, p := range t.prep {
			a.Prep = append(a.Prep, store.PrepItem{ID: newToken()[:10], Text: p, Done: i < doneN})
		}
		if err := s.store.SaveAdventure(ctx, a); err != nil {
			return nil
		}
		for _, st := range t.stages {
			x := st
			x.UserID, x.AdventureID = uid, a.ID
			if x.Lock == "qr" {
				x.Code = newToken()[:12]
			}
			_ = s.store.SaveStage(ctx, &x)
		}
		return a
	}
	study := mk("study", sat.Format("2006-01-02"), "19:30", "Our flat", "Ana, Marko, Lejla, Ivan", true, 3)
	if study != nil {
		played("adventure", study.ID, "escape", "link", "The Owls", 9, 47*60000, 2, "")
		played("adventure", study.ID, "escape", "link", "Team Vienna", 9, 54*60000, 4, "")
		// a team playing right now, at stage 3
		p := &store.Play{UserID: uid, What: "adventure", RefID: study.ID, Kind: "escape", Via: "link", Player: "The Night Shift", Key: newKey(), StartedAt: now.Add(-22 * time.Minute).UTC().Format(time.RFC3339Nano)}
		if s.store.CreatePlay(ctx, p) == nil {
			at := now.Add(-6 * time.Minute).UTC().Format(time.RFC3339Nano)
			p.Stage, p.StageAt, p.Hints = 2, &at, 1
			_ = s.store.UpdatePlay(ctx, p)
		}
	}
	mk("kids", now.AddDate(0, 0, 16).Format("2006-01-02"), "15:00", "Grandma's garden", "Mia and 8 friends", false, 0)
	mk("hunt", "", "", "Old town", "", false, 0)
	s.log.Info("demo data ready")
	return nil
}

// demoPicture paints a sunset over hills for the demo jigsaw.
func demoPicture() []byte {
	w, h := 1000, 750
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h)
		sky := color.RGBA{uint8(255 - 60*t), uint8(140 + 60*(1-t)), uint8(90 + 120*t), 255}
		for x := 0; x < w; x++ {
			img.Set(x, y, sky)
		}
	}
	// the sun
	cx, cy, r := 640.0, 360.0, 110.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := math.Hypot(float64(x)-cx, float64(y)-cy)
			if d < r {
				img.Set(x, y, color.RGBA{255, 214, 102, 255})
			} else if d < r+40 {
				c := img.RGBAAt(x, y)
				k := (d - r) / 40
				img.Set(x, y, color.RGBA{uint8(float64(c.R)*k + 255*(1-k)), uint8(float64(c.G)*k + 200*(1-k)), uint8(float64(c.B)*k + 120*(1-k)), 255})
			}
		}
	}
	// three rows of hills
	hills := []struct {
		base, amp, freq, phase float64
		c                      color.RGBA
	}{{430, 60, 0.006, 0, color.RGBA{122, 92, 140, 255}}, {520, 50, 0.009, 2, color.RGBA{72, 110, 98, 255}}, {610, 40, 0.012, 4, color.RGBA{40, 78, 64, 255}}}
	for _, hl := range hills {
		for x := 0; x < w; x++ {
			top := int(hl.base - hl.amp*math.Sin(float64(x)*hl.freq+hl.phase) - hl.amp*0.4*math.Sin(float64(x)*hl.freq*2.7+hl.phase))
			for y := max(0, top); y < h; y++ {
				img.Set(x, y, hl.c)
			}
		}
	}
	// a little house with a lit window
	for y := 560; y < 610; y++ {
		for x := 200; x < 260; x++ {
			img.Set(x, y, color.RGBA{230, 220, 200, 255})
		}
	}
	for y := 530; y < 560; y++ {
		for x := 200 + (560 - y); x < 260-(560-y); x++ {
			img.Set(x, y, color.RGBA{170, 60, 50, 255})
		}
	}
	for y := 575; y < 592; y++ {
		for x := 215; x < 232; x++ {
			img.Set(x, y, color.RGBA{255, 200, 80, 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

var _ = fmt.Sprintf
