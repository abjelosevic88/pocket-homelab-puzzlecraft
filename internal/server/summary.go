package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// streak counts the days in a row (to today, or to yesterday when today is
// not done yet) with a daily puzzle solved.
func streak(days []string, today string) (int, int) {
	set := map[string]bool{}
	for _, d := range days {
		set[d] = true
	}
	cur := 0
	d := today
	if !set[d] {
		d = store.AddDays(d, -1)
	}
	for set[d] {
		cur++
		d = store.AddDays(d, -1)
	}
	best, run := 0, 0
	sorted := append([]string(nil), days...)
	sort.Strings(sorted)
	prev := ""
	for _, x := range sorted {
		if prev != "" && store.AddDays(prev, 1) == x {
			run++
		} else {
			run = 1
		}
		best = max(best, run)
		prev = x
	}
	return cur, best
}

// handleToday is the start page: today's puzzles, what you are in the middle
// of, adventures coming up, what players did with your shared puzzles.
func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st := s.settingsOf(ctx, uid)
	day := r.URL.Query().Get("date")
	if !validDate(day) || day == "" || day > today() {
		day = today()
	}
	daily := []map[string]any{}
	if st.Daily {
		for _, k := range puzzle.DailyKinds(day) {
			m, err := puzzle.Daily(k, day, st.DailyLevel)
			if err != nil {
				continue
			}
			item := map[string]any{"kind": k, "kind_name": puzzle.KindName[k], "title": m.Title, "level": firstNonEmpty(st.DailyLevel, m.Level), "size_text": puzzle.Size(k, m.Data)}
			if p, err := s.store.MyPlay(ctx, uid, "daily", 0, day+"|"+k); err == nil {
				item["started"] = true
				item["finished_at"], item["ms"], item["hints"] = p.FinishedAt, p.Ms, p.Hints
			}
			daily = append(daily, item)
		}
	}
	days, _ := s.store.SolvedDays(ctx, uid)
	cur, best := streak(days, today())
	// puzzles you started and have not finished
	open := false
	mine, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{What: "puzzle", Via: "me", Finished: &open, Limit: 6})
	going := []map[string]any{}
	for _, p := range mine {
		if pz, err := s.store.GetPuzzle(ctx, uid, p.RefID); err == nil && !pz.Archived {
			going = append(going, map[string]any{"id": pz.ID, "title": pz.Title, "kind": pz.Kind, "kind_name": puzzle.KindName[pz.Kind], "last_at": p.LastAt})
		}
	}
	recent, _ := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{Limit: 8})
	advs, _ := s.store.ListAdventures(ctx, uid)
	coming := []store.Adventure{}
	for _, a := range advs {
		if a.Status != "done" && (a.Date == "" || a.Date >= today()) && len(coming) < 4 {
			coming = append(coming, a)
		}
	}
	// what others did with what you shared, the last week
	done := true
	from := store.AddDays(today(), -7)
	results := []map[string]any{}
	for _, what := range []string{"puzzle", "adventure"} {
		list, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{What: what, Finished: &done, From: from, Limit: 20})
		for _, p := range list {
			if p.Via == "me" {
				continue
			}
			title := ""
			if what == "puzzle" {
				if pz, err := s.store.GetPuzzle(ctx, uid, p.RefID); err == nil {
					title = pz.Title
				}
			} else if a, err := s.store.GetAdventure(ctx, uid, p.RefID); err == nil {
				title = a.Title
			}
			results = append(results, map[string]any{"what": what, "ref_id": p.RefID, "title": title, "player": firstNonEmpty(p.Player, "Someone"), "ms": p.Ms, "hints": p.Hints, "finished_at": p.FinishedAt})
		}
	}
	sort.Slice(results, func(a, b int) bool {
		return fmt.Sprint(results[a]["finished_at"]) > fmt.Sprint(results[b]["finished_at"])
	})
	if len(results) > 8 {
		results = results[:8]
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": day, "daily": daily, "streak": cur, "best_streak": best, "going": going, "recent": recent, "adventures": coming, "results": results})
}

// handleHome is the sidebar: counts, recent puzzles, adventures coming up.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	kinds, _ := s.store.KindCounts(ctx, uid)
	total := 0
	for _, n := range kinds {
		total += n
	}
	recent, _ := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{Limit: 6})
	advs, _ := s.store.ListAdventures(ctx, uid)
	books, _ := s.store.ListBooks(ctx, uid)
	lists, _ := s.store.ListWordLists(ctx, uid)
	coming := []map[string]any{}
	for _, a := range advs {
		if a.Status != "done" && len(coming) < 4 {
			coming = append(coming, map[string]any{"id": a.ID, "title": a.Title, "kind": a.Kind, "date": a.Date, "live": a.Live})
		}
	}
	days, _ := s.store.SolvedDays(ctx, uid)
	cur, _ := streak(days, today())
	dailyLeft := 0
	if s.settingsOf(ctx, uid).Daily {
		for _, k := range puzzle.DailyKinds(today()) {
			if p, err := s.store.MyPlay(ctx, uid, "daily", 0, today()+"|"+k); err != nil || p.FinishedAt == nil {
				dailyLeft++
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"puzzles": total, "kinds": kinds, "recent": recent, "adventures": coming, "adventures_n": len(advs), "books": len(books),
		"wordlists": len(lists), "streak": cur, "daily_left": dailyLeft})
}

// handleStats is how you solve: times by kind, the last weeks, your best.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	done := true
	from := store.AddDays(today(), -83)
	mine, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{Via: "me", Finished: &done, Limit: 5000})
	type agg struct {
		n          int
		ms, hints  int64
		best       int64
		bestTitle  string
		lastFinish string
	}
	byKind := map[string]*agg{}
	perDay := map[string]int{}
	total := 0
	for _, p := range mine {
		if p.What == "adventure" {
			continue
		}
		total++
		a := byKind[p.Kind]
		if a == nil {
			a = &agg{}
			byKind[p.Kind] = a
		}
		a.n++
		a.ms += p.Ms
		a.hints += int64(p.Hints)
		if p.Ms > 0 && (a.best == 0 || p.Ms < a.best) {
			a.best = p.Ms
		}
		if p.FinishedAt != nil {
			d := (*p.FinishedAt)[:10]
			if t, err := time.Parse(time.RFC3339Nano, *p.FinishedAt); err == nil {
				d = t.Local().Format("2006-01-02")
			}
			if d >= from {
				perDay[d]++
			}
		}
	}
	kinds := []map[string]any{}
	for _, k := range puzzle.Kinds {
		a := byKind[k]
		if a == nil {
			continue
		}
		kinds = append(kinds, map[string]any{"kind": k, "kind_name": puzzle.KindName[k], "solved": a.n, "avg_ms": a.ms / int64(a.n), "best_ms": a.best,
			"hints_per": float64(a.hints) / float64(a.n)})
	}
	days := []map[string]any{}
	for i := 83; i >= 0; i-- {
		d := store.AddDays(today(), -i)
		days = append(days, map[string]any{"date": d, "n": perDay[d]})
	}
	sd, _ := s.store.SolvedDays(ctx, uid)
	cur, best := streak(sd, today())
	// others: plays of your shared puzzles
	others := 0
	all, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{Finished: &done, Limit: 5000})
	for _, p := range all {
		if p.Via != "me" {
			others++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"solved": total, "kinds": kinds, "days": days, "streak": cur, "best_streak": best, "daily_days": len(sd), "players_solved": others})
}

// handleSummary is a short overview for Pocket AI and other apps.
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	kinds, _ := s.store.KindCounts(ctx, uid)
	total := 0
	for _, n := range kinds {
		total += n
	}
	shared, _ := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{Shared: true})
	advs, _ := s.store.ListAdventures(ctx, uid)
	coming := []map[string]any{}
	for _, a := range advs {
		if a.Status != "done" && a.Date >= today() {
			coming = append(coming, map[string]any{"id": a.ID, "title": a.Title, "kind": a.Kind, "date": a.Date, "time": a.Time, "stages": a.StagesN,
				"prep_left": prepLeft(a.Prep), "url": s.link(fmt.Sprintf("/adventures/%d", a.ID))})
		}
	}
	days, _ := s.store.SolvedDays(ctx, uid)
	cur, best := streak(days, today())
	daily := []map[string]any{}
	for _, k := range puzzle.DailyKinds(today()) {
		item := map[string]any{"kind": k, "kind_name": puzzle.KindName[k], "solved": false}
		if p, err := s.store.MyPlay(ctx, uid, "daily", 0, today()+"|"+k); err == nil && p.FinishedAt != nil {
			item["solved"], item["time"] = true, duration(p.Ms)
		}
		daily = append(daily, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"puzzles": total, "by_kind": kinds, "shared": len(shared), "adventures_coming": coming, "streak": cur, "best_streak": best,
		"today": daily, "url": s.link("/")})
}

func prepLeft(p []store.PrepItem) int {
	n := 0
	for _, it := range p {
		if !it.Done {
			n++
		}
	}
	return n
}

// dayPlay is a go finished on a day, with what it was at.
type dayPlay struct {
	store.Play
	at    time.Time // when it was finished, on that day's clock
	title string
	path  string // its page here
}

// playsOn lists the goes finished on a day as loc's clock has it: the
// person's own, and those of the players of what they shared. A go at
// something that is gone is left out.
func (s *Server) playsOn(ctx context.Context, uid int64, day string, loc *time.Location) ([]dayPlay, error) {
	done := true
	list, err := s.store.ListPlays(ctx, uid, store.PlayFilter{Finished: &done, From: store.AddDays(day, -1), To: store.AddDays(day, 1), Limit: 200})
	out := []dayPlay{}
	for _, p := range list {
		if p.FinishedAt == nil {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, *p.FinishedAt)
		if err != nil || t.In(loc).Format("2006-01-02") != day {
			continue
		}
		d := dayPlay{Play: p, at: t.In(loc)}
		switch p.What {
		case "daily":
			d.title, d.path = "Puzzle of the day: "+puzzle.KindName[p.Kind], "/today/"+dayOf(&p)+"/"+p.Kind
		case "puzzle":
			if pz, err := s.store.GetPuzzle(ctx, uid, p.RefID); err == nil {
				d.title, d.path = pz.Title, fmt.Sprintf("/puzzles/%d", pz.ID)
			}
		case "adventure":
			if a, err := s.store.GetAdventure(ctx, uid, p.RefID); err == nil {
				d.title, d.path = a.Title, fmt.Sprintf("/adventures/%d", a.ID)
			}
		}
		if d.title == "" {
			continue
		}
		out = append(out, d)
	}
	return out, err
}

// handleDay is one day for Pocket Day: puzzles solved, adventures played or coming.
func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	day := chiParam(r, "date")
	if !validDate(day) || day == "" {
		writeErr(w, http.StatusBadRequest, "the date is YYYY-MM-DD")
		return
	}
	list, _ := s.playsOn(ctx, uid, day, time.Local)
	items := []map[string]any{}
	minutes := int64(0)
	for _, p := range list {
		url := s.link(p.path)
		if p.What == "daily" {
			url = s.link("/today?date=" + dayOf(&p.Play))
		}
		who := "You"
		if p.Via != "me" {
			who = firstNonEmpty(p.Player, "Someone")
		} else {
			minutes += p.Ms / 60000
		}
		sub := fmt.Sprintf("%s solved it in %s", who, duration(p.Ms))
		if p.Hints > 0 {
			sub += fmt.Sprintf(", %d %s", p.Hints, plural(p.Hints, "hint", "hints"))
		}
		items = append(items, map[string]any{"time": p.at.Format("15:04"), "title": p.title, "sub": sub, "url": url, "kind": p.Kind, "mine": p.Via == "me"})
	}
	advs, _ := s.store.ListAdventures(ctx, uid)
	planned := []map[string]any{}
	for _, a := range advs {
		if a.Date == day {
			planned = append(planned, map[string]any{"title": a.Title, "time": a.Time, "kind": a.Kind, "place": a.Place, "url": s.link(fmt.Sprintf("/adventures/%d", a.ID))})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": day, "count": len(items), "minutes": minutes, "items": items, "planned": planned})
}

// found is what a search finds, each kind in the order its own list has.
type found struct {
	puzzles []store.Puzzle
	advs    []store.Adventure
	books   []store.Book
	lists   []store.WordList
}

// find looks for the words in a person's puzzles (title, instructions, notes,
// tags), adventures, books and word lists (the name, or one of the words).
func (s *Server) find(ctx context.Context, uid int64, q string) found {
	var f found
	lq := strings.ToLower(q)
	f.puzzles, _ = s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{Q: q, Limit: 30})
	if advs, err := s.store.ListAdventures(ctx, uid); err == nil {
		for _, a := range advs {
			if strings.Contains(strings.ToLower(a.Title+" "+a.Intro+" "+a.Theme+" "+a.Place), lq) {
				f.advs = append(f.advs, a)
			}
		}
	}
	if books, err := s.store.ListBooks(ctx, uid); err == nil {
		for _, b := range books {
			if strings.Contains(strings.ToLower(b.Title+" "+b.Subtitle), lq) {
				f.books = append(f.books, b)
			}
		}
	}
	if lists, err := s.store.ListWordLists(ctx, uid); err == nil {
		for _, l := range lists {
			hit := strings.Contains(strings.ToLower(l.Name), lq)
			for _, w := range l.Words {
				if strings.Contains(strings.ToLower(w.Word), lq) {
					hit = true
					break
				}
			}
			if hit {
				f.lists = append(f.lists, l)
			}
		}
	}
	return f
}

// handleSearch finds puzzles, adventures, books and word lists (for Pocket Search and the palette).
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	out := []map[string]any{}
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"results": out})
		return
	}
	f := s.find(ctx, uid, q)
	for _, p := range f.puzzles {
		out = append(out, map[string]any{"type": "puzzle", "id": p.ID, "title": p.Title, "snippet": puzzle.KindName[p.Kind] + " · " + p.SizeText, "kind": p.Kind,
			"date": p.UpdatedAt, "url": s.link(fmt.Sprintf("/puzzles/%d", p.ID)), "path": fmt.Sprintf("/puzzles/%d", p.ID)})
	}
	for _, a := range f.advs {
		out = append(out, map[string]any{"type": "adventure", "id": a.ID, "title": a.Title, "snippet": clip(firstLine(a.Intro), 160), "kind": a.Kind, "date": a.Date,
			"url": s.link(fmt.Sprintf("/adventures/%d", a.ID)), "path": fmt.Sprintf("/adventures/%d", a.ID)})
	}
	for _, b := range f.books {
		out = append(out, map[string]any{"type": "book", "id": b.ID, "title": b.Title, "snippet": fmt.Sprintf("Puzzle book, %d puzzles", len(b.Items)), "date": b.UpdatedAt,
			"url": s.link(fmt.Sprintf("/books/%d", b.ID)), "path": fmt.Sprintf("/books/%d", b.ID)})
	}
	for _, l := range f.lists {
		out = append(out, map[string]any{"type": "wordlist", "id": l.ID, "title": l.Name, "snippet": fmt.Sprintf("Word list, %d words", len(l.Words)), "date": l.UpdatedAt,
			"url": s.link(fmt.Sprintf("/words?list=%d", l.ID)), "path": fmt.Sprintf("/words?list=%d", l.ID)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"adventures": adventureTemplateList()})
}

var _ = context.Background
