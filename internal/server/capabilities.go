package server

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// What Pocket Search, Pocket Day and Pocket Hub ask Puzzlecraft (the
// capabilities value in links.go): one route each, in the shape every Pocket
// app answers in. They read what /search, /day and /home read, as the
// request's person, and never a puzzle's answer; only the capture adds
// something, one word list.

// adventureWords are the kinds of adventure in plain words, as the pages name them.
var adventureWords = map[string][2]string{"escape": {"escape room", "escape rooms"}, "hunt": {"treasure hunt", "treasure hunts"},
	"mystery": {"mystery", "mysteries"}, "party": {"party game", "party games"}}

// puzzleWord is a puzzle's kind in plain words: "word search".
func puzzleWord(kind string) string {
	return firstNonEmpty(strings.ToLower(puzzle.KindName[kind]), "puzzle")
}

// adventureWord is an adventure's kind in plain words: "escape room".
func adventureWord(kind string) string {
	return firstNonEmpty(adventureWords[kind][0], "game")
}

// searchKinds lists every kind a match can be, for the manifest.
func searchKinds() []string {
	out := []string{}
	for _, k := range puzzle.Kinds {
		out = append(out, puzzleWord(k))
	}
	for _, k := range store.AdventureKinds {
		out = append(out, adventureWord(k))
	}
	return append(out, "puzzle book", "word list")
}

// upperFirst starts a line with a capital letter.
func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 || unicode.IsUpper(r) {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// dots joins the parts of a line that are there: "9 × 9, 30 given · hard".
func dots(parts ...string) string {
	out := parts[:0:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

// howMany is a number with its word: "1 puzzle", "3 puzzles".
func howMany(n int, one, many string) string {
	return strconv.Itoa(n) + " " + plural(n, one, many)
}

// hasWord says whether there is a letter or a digit to look for.
func hasWord(q string) bool {
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// ---------- search ----------

// pocketSearch answers Pocket Search: the puzzles, escape rooms and hunts,
// puzzle books and word lists /search finds, as one list with a match in the
// title or name first, and the kinds taking turns so a common word does not
// fill the list with puzzles alone. Only the person's own: what someone else
// shared by link is never here.
func (s *Server) pocketSearch(r *http.Request, q string, limit int) ([]core.SearchItem, error) {
	if !hasWord(q) {
		return nil, nil
	}
	ctx := r.Context()
	f := s.find(ctx, actorID(ctx), q)
	lq := strings.ToLower(q)
	score := func(title string) float64 {
		if strings.Contains(strings.ToLower(title), lq) {
			return 3
		}
		return 1
	}
	out := []core.SearchItem{}
	table := []int{} // which of the four each match is
	for _, p := range f.puzzles {
		table = append(table, 0)
		shared := ""
		if p.ShareOn {
			shared = "shared"
		}
		out = append(out, core.SearchItem{ID: "puzzle:" + itoa(p.ID), Title: p.Title, Kind: puzzleWord(p.Kind), Date: p.UpdatedAt, URL: "/puzzles/" + itoa(p.ID),
			Snippet: dots(p.SizeText, p.Level, shared), Score: score(p.Title)})
	}
	for _, a := range f.advs {
		table = append(table, 1)
		out = append(out, core.SearchItem{ID: "adventure:" + itoa(a.ID), Title: a.Title, Kind: adventureWord(a.Kind), Date: firstNonEmpty(a.Date, a.UpdatedAt),
			URL: "/adventures/" + itoa(a.ID), Snippet: dots(a.Place, clip(firstLine(a.Intro), 160)), Score: score(a.Title)})
	}
	for _, b := range f.books {
		table = append(table, 2)
		out = append(out, core.SearchItem{ID: "book:" + itoa(b.ID), Title: b.Title, Kind: "puzzle book", Date: b.UpdatedAt, URL: "/books/" + itoa(b.ID),
			Snippet: dots(b.Subtitle, howMany(len(b.Items), "puzzle", "puzzles")), Score: score(b.Title)})
	}
	for _, l := range f.lists {
		table = append(table, 3)
		it := core.SearchItem{ID: "wordlist:" + itoa(l.ID), Title: l.Name, Kind: "word list", Date: l.UpdatedAt, URL: "/words?list=" + itoa(l.ID),
			Snippet: howMany(len(l.Words), "word", "words"), Score: score(l.Name)}
		if it.Score < 3 { // found by one of its words: say which
			for _, w := range l.Words {
				if strings.Contains(strings.ToLower(w.Word), lq) {
					it.Snippet = dots(it.Snippet, w.Word)
					break
				}
			}
		}
		out = append(out, it)
	}
	// a match's turn is its place among the equally good ones of its kind
	type lane struct {
		table int
		score float64
	}
	seen := map[lane]int{}
	turn := make([]int, len(out))
	order := make([]int, len(out))
	for i := range out {
		l := lane{table[i], out[i].Score}
		turn[i], order[i] = seen[l], i
		seen[l]++
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if out[a].Score != out[b].Score {
			return out[a].Score > out[b].Score
		}
		return turn[a] < turn[b]
	})
	sorted := make([]core.SearchItem, 0, min(len(out), limit))
	for _, i := range order[:min(len(order), limit)] {
		sorted = append(sorted, out[i])
	}
	return sorted, nil
}

// ---------- day ----------

var clockRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// minutesOf is a solving time in minutes, to one decimal.
func minutesOf(ms int64) float64 {
	return math.Round(float64(ms)/6000) / 10
}

// pocketDay answers Pocket Day: the puzzles made that day, the goes finished
// (the person's own with their minutes, and those of the players of what
// they shared, as /day has them) and the escape rooms and hunts set for that
// day. The day is the caller's time zone's when it says one, else the box's.
func (s *Server) pocketDay(r *http.Request, date string, tz *time.Location) (*core.Day, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	loc := time.Local
	if tz != nil {
		loc = tz
	}
	day := &core.Day{Date: date}

	// stamps are UTC: a day of any zone lies within the days around it
	made, err := s.store.PuzzlesMade(ctx, uid, store.AddDays(date, -1), store.AddDays(date, 1), 200)
	if err != nil {
		return nil, err
	}
	nMade := 0
	for _, p := range made {
		t, err := time.Parse(time.RFC3339Nano, p.CreatedAt)
		if err != nil || t.In(loc).Format("2006-01-02") != date {
			continue
		}
		nMade++
		day.Items = append(day.Items, core.DayItem{ID: "puzzle:" + itoa(p.ID), Time: t.In(loc).Format("15:04"), Title: firstNonEmpty(p.Title, puzzle.KindName[p.Kind]),
			Sub: dots(puzzle.KindName[p.Kind], p.SizeText), Kind: "made", URL: "/puzzles/" + itoa(p.ID)})
	}

	plays, err := s.playsOn(ctx, uid, date, loc)
	if err != nil {
		return nil, err
	}
	var solved, tested, others int
	var ms int64
	for _, p := range plays {
		it := core.DayItem{ID: "play:" + itoa(p.ID), Time: p.at.Format("15:04"), Title: p.title, Done: true, URL: p.path}
		took := "" // a time under a second says nothing
		if p.Ms >= 1000 {
			took = " in " + duration(p.Ms)
		}
		who := firstNonEmpty(p.Player, "Someone")
		switch mine := p.Via == "me"; {
		case mine && p.What == "adventure":
			it.Kind, it.Sub, it.Minutes = "tested", "Test run finished"+took, minutesOf(p.Ms)
			tested++
		case mine:
			it.Kind, it.Sub, it.Minutes = "solved", "Solved"+took, minutesOf(p.Ms)
			solved++
			ms += p.Ms
		case p.What == "adventure":
			it.Kind, it.Sub, it.URL = "solved by others", who+" finished it"+took, p.path+"?tab=live"
			others++
		default:
			it.Kind, it.Sub, it.URL = "solved by others", who+" solved it"+took, p.path+"?tab=results"
			others++
		}
		if p.Hints > 0 {
			it.Sub += ", " + howMany(p.Hints, "hint", "hints")
		}
		day.Items = append(day.Items, it)
	}

	advs, err := s.store.ListAdventures(ctx, uid)
	if err != nil {
		return nil, err
	}
	var planned []store.Adventure
	for _, a := range advs {
		if a.Date != date {
			continue
		}
		planned = append(planned, a)
		it := core.DayItem{ID: "adventure:" + itoa(a.ID), Title: a.Title, Sub: dots(upperFirst(adventureWord(a.Kind)), a.Place), Kind: "planned",
			Done: a.Status == "done", URL: "/adventures/" + itoa(a.ID)}
		if clockRe.MatchString(a.Time) {
			it.Time = a.Time
		}
		day.Items = append(day.Items, it)
	}
	if len(day.Items) == 0 {
		return nil, nil
	}

	var parts []string
	if solved > 0 {
		part := howMany(solved, "puzzle solved", "puzzles solved")
		if mins := int(math.Round(float64(ms) / 60000)); mins > 0 {
			part += " in " + strconv.Itoa(mins) + " min"
			day.Metrics = append(day.Metrics, core.Metric{Label: "Time", Value: strconv.Itoa(mins) + " min"})
		}
		parts = append(parts, part)
		day.Metrics = append([]core.Metric{{Label: "Solved", Value: strconv.Itoa(solved)}}, day.Metrics...)
	}
	if nMade > 0 {
		parts = append(parts, howMany(nMade, "puzzle made", "puzzles made"))
		day.Metrics = append(day.Metrics, core.Metric{Label: "Made", Value: strconv.Itoa(nMade)})
	}
	if tested > 0 {
		parts = append(parts, howMany(tested, "test run", "test runs"))
	}
	if others > 0 {
		parts = append(parts, strconv.Itoa(others)+" solved by others")
		day.Metrics = append(day.Metrics, core.Metric{Label: "By others", Value: strconv.Itoa(others)})
	}
	if n := len(planned); n > 0 {
		// "1 escape room planned", "2 treasure hunts planned", "2 games planned" when they differ
		words, same := [2]string{"game", "games"}, true
		for _, a := range planned {
			same = same && a.Kind == planned[0].Kind
		}
		if w, ok := adventureWords[planned[0].Kind]; ok && same {
			words = w
		}
		parts = append(parts, howMany(n, words[0], words[1])+" planned")
	}
	day.Summary = upperFirst(strings.Join(parts, ", "))
	return day, nil
}

// ---------- widget ----------

// pocketWidget answers Pocket Hub: the puzzles of the day and whether each
// is solved, the run of days, then the puzzles made or changed last, as the
// start page has them. Nothing is made here: which puzzles are today's
// follows from the date alone, and a puzzle of the day is only worked out
// when someone opens it. Someone with the puzzles of the day switched off
// and no puzzles gets nothing to show.
func (s *Server) pocketWidget(r *http.Request) (*core.WidgetData, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	day := today()
	w := &core.WidgetData{URL: "/today"}
	total, solved := 0, 0
	if s.settingsOf(ctx, uid).Daily {
		for _, k := range puzzle.DailyKinds(day) {
			it := core.WidgetItem{ID: "daily:" + day + ":" + k, Title: "Puzzle of the day: " + puzzle.KindName[k], Detail: "Not solved yet", URL: "/today/" + day + "/" + k}
			p, err := s.store.MyPlay(ctx, uid, "daily", 0, day+"|"+k)
			switch {
			case err == nil && p.FinishedAt != nil:
				it.Done, it.Detail = true, "Solved"
				if p.Ms >= 1000 {
					it.Detail = "Solved in " + duration(p.Ms)
				}
				solved++
			case err == nil:
				it.Detail = "Started"
			case !errors.Is(err, store.ErrNotFound):
				return nil, err
			}
			total++
			w.Items = append(w.Items, it)
		}
	}
	kinds, err := s.store.KindCounts(ctx, uid)
	if err != nil {
		return nil, err
	}
	puzzles := 0
	for _, n := range kinds {
		puzzles += n
	}
	if total == 0 && puzzles == 0 {
		return nil, nil
	}
	if total > 0 {
		days, err := s.store.SolvedDays(ctx, uid)
		if err != nil {
			return nil, err
		}
		run, best := streak(days, day)
		switch {
		case solved == 0 && total == 1:
			w.Line = "Today's puzzle is waiting"
		case solved == 0:
			w.Line = fmt.Sprintf("Today's %d puzzles are waiting", total)
		case total == 1:
			w.Line = "Solved today"
		case solved == total:
			w.Line = fmt.Sprintf("All %d solved today", total)
		default:
			w.Line = fmt.Sprintf("%d of %d solved today", solved, total)
		}
		switch {
		case run > 1 && solved > 0:
			w.Line += fmt.Sprintf(", %d days in a row", run)
		case run > 1:
			w.Line += fmt.Sprintf(", %d days in a row so far", run)
		}
		w.Metrics = append(w.Metrics, core.Metric{Label: "Solved today", Value: fmt.Sprintf("%d of %d", solved, total)},
			core.Metric{Label: "Days in a row", Value: strconv.Itoa(run)}, core.Metric{Label: "Longest run", Value: strconv.Itoa(best)})
	} else {
		w.Line = howMany(puzzles, "puzzle", "puzzles")
	}
	w.Metrics = append(w.Metrics, core.Metric{Label: "Puzzles", Value: strconv.Itoa(puzzles)})
	if room := 8 - len(w.Items); puzzles > 0 && room > 0 {
		recent, err := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{Limit: room})
		if err != nil {
			return nil, err
		}
		for _, p := range recent {
			w.Items = append(w.Items, core.WidgetItem{ID: "puzzle:" + itoa(p.ID), Title: p.Title, Detail: dots(puzzle.KindName[p.Kind], p.SizeText), URL: "/puzzles/" + itoa(p.ID)})
		}
	}
	return w, nil
}

// ---------- capture ----------

// captureWords reads a quick line as words: one per line, or several with
// commas between them. Each is read as the word-list box reads a line (a
// dash or a number in front is dropped, "word - clue" keeps its clue), and
// only what a puzzle can use stays: two letters or more, each word once.
func captureWords(text, lang string) []store.WordClue {
	words := []store.WordClue{}
	for _, piece := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' }) {
		for _, w := range parseWordLines(piece) {
			if len(puzzle.Letters(w.Word, lang)) >= 2 {
				words = append(words, w)
			}
		}
	}
	return cleanWords(words)
}

// captureName names a list after its first words: "Apple, pear, plum…".
func captureName(words []store.WordClue) string {
	first := []string{}
	for _, w := range words[:min(3, len(words))] {
		first = append(first, w.Word)
	}
	name := upperFirst(strings.Join(first, ", "))
	if len(words) > len(first) {
		name += "…"
	}
	return name
}

// pocketCapture keeps a quick line from Pocket Hub as a new word list, as
// POST /wordlists does: in the person's language, marked with the app that
// sent it. The title is the list's name; without one the list is named
// after its first words. The same request_id again answers with the list
// made the first time and adds nothing: the receipt is written in the
// transaction that adds the list.
func (s *Server) pocketCapture(r *http.Request, in core.CaptureInput) (*core.CaptureResult, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	op, err := core.NewOperation(fmt.Sprintf("puzzlecraft:user:%d:%s", uid, core.CapturePath), in.RequestID, in)
	if err != nil {
		return nil, err
	}
	var res core.CaptureResult
	replay := func() (*core.CaptureResult, error) {
		found, err := op.Replay(ctx, s.store.DB, &res)
		switch {
		case errors.Is(err, core.ErrOperationConflict):
			return nil, fmt.Errorf("%w: this was already kept with other words", core.ErrCapture)
		case err != nil || !found:
			return nil, err
		}
		return &res, nil
	}
	if kept, err := replay(); kept != nil || err != nil {
		return kept, err
	}

	l := &store.WordList{UserID: uid, Language: s.settingsOf(ctx, uid).Language, Source: sourceOf(ctx), Name: clip(strings.Join(strings.Fields(in.Title), " "), maxName)}
	text := in.Text
	if text == "" { // only a line: it is the words, not the name
		text, l.Name = in.Title, ""
	}
	l.Words = captureWords(text, l.Language)
	if len(l.Words) < 2 {
		return nil, fmt.Errorf("%w: a word list needs at least two words, with commas between them", core.ErrCapture)
	}
	if l.Name == "" {
		l.Name = clip(captureName(l.Words), maxName)
	}
	err = s.store.AddWordList(ctx, l, func(tx *sql.Tx) error {
		res = core.CaptureResult{ID: "wordlist:" + itoa(l.ID), Title: l.Name, URL: "/words?list=" + itoa(l.ID)}
		return op.Save(ctx, tx, res)
	})
	if err != nil {
		// the same line sent twice at once: the other one kept it
		if kept, rerr := replay(); kept != nil || rerr != nil {
			return kept, rerr
		}
		return nil, err
	}
	return &res, nil
}
