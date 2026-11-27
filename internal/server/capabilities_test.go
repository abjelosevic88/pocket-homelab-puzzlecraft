package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/config"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// fakeCapCore is a Pocket Core that vouches for link tokens
// pcx_<alice|bob>_<search|day|widget|capture|read|write> from Pocket Hub
// and remembers the manifest Puzzlecraft registers with.
type fakeCapCore struct {
	*httptest.Server
	mu       sync.Mutex
	manifest map[string]any
}

func newCapApp(t *testing.T) (*app, *fakeCapCore) {
	t.Helper()
	fc := &fakeCapCore{}
	scopes := map[string]string{"search": core.ScopeSearch, "day": core.ScopeDay, "widget": core.ScopeWidget, "capture": core.ScopeCapture,
		"read": "puzzlecraft:read", "write": "puzzlecraft:write"}
	fc.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/apps/register":
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			fc.mu.Lock()
			fc.manifest = m
			fc.mu.Unlock()
			io.WriteString(w, `{}`)
		case "/api/v1/auth/introspect":
			var in struct{ Token string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			parts := strings.Split(in.Token, "_")
			if len(parts) != 3 || parts[0] != "pcx" || scopes[parts[2]] == "" {
				io.WriteString(w, `{"active":false}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "kind": "delegated", "audience": "puzzlecraft", "scope": scopes[parts[2]],
				"user": map[string]any{"id": 1, "sub": parts[1] + "@example.com", "email": parts[1] + "@example.com", "name": parts[1], "role": "member"},
				"app":  map[string]any{"id": 9, "slug": "hub", "name": "Pocket Hub"}})
		default:
			io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(fc.Close)
	return newApp(t, func(c *config.Config) { c.CoreURL, c.CoreRegistrationKey = fc.URL, "key" }), fc
}

// items are the rows of a capability answer.
func items(t *testing.T, m map[string]any) []map[string]any {
	t.Helper()
	l, ok := m["items"].([]any)
	if !ok {
		t.Fatalf("no items list in %v", m)
	}
	out := []map[string]any{}
	for _, v := range l {
		out = append(out, v.(map[string]any))
	}
	return out
}

// item finds the row with that title and kind ("" = any kind).
func item(t *testing.T, m map[string]any, title, kind string) map[string]any {
	t.Helper()
	for _, it := range items(t, m) {
		if it["title"] == title && (kind == "" || it["kind"] == kind) {
			return it
		}
	}
	t.Fatalf("no %q (%s) in %v", title, kind, m["items"])
	return nil
}

// metric is the value of a labelled number ("" when it is not there).
func metric(m map[string]any, label string) string {
	l, _ := m["metrics"].([]any)
	for _, v := range l {
		if x := v.(map[string]any); x["label"] == label {
			return x["value"].(string)
		}
	}
	return ""
}

// solveDaily solves today's word ladder for tok and returns the go's key.
func solveDaily(a *app, tok string) string {
	a.t.Helper()
	day := today()
	p := a.ok(tok, "POST", "/api/v1/daily/"+day+"/ladder/play", nil)
	m, _ := puzzle.Daily("ladder", day, "")
	var l puzzle.Ladder
	_ = json.Unmarshal(m.Data, &l)
	res := a.ok("", "POST", "/api/v1/public/play/"+p["key"].(string)+"/check", map[string]any{"answer": map[string]any{"answers": l.Steps[1 : len(l.Steps)-1]}, "final": true})
	if res["result"].(map[string]any)["solved"] != true {
		a.t.Fatalf("ladder: %v", res)
	}
	return p["key"].(string)
}

// solveShared has a player without an account solve a shared crossword
// through its link, with one hint.
func solveShared(a *app, owner, puzzleID, player string) string {
	a.t.Helper()
	a.ok(owner, "POST", "/api/v1/puzzles/"+puzzleID+"/share", map[string]any{"on": true})
	pz := a.ok(owner, "GET", "/api/v1/puzzles/"+puzzleID, nil)
	var c puzzle.Crossword
	_ = json.Unmarshal(mustJSON(pz["data"]), &c)
	play := a.ok("", "POST", "/api/v1/public/p/"+pz["share_token"].(string)+"/play", map[string]any{"player": player})
	key := play["key"].(string)
	a.ok("", "POST", "/api/v1/public/play/"+key+"/reveal", map[string]any{"what": 0})
	res := a.ok("", "POST", "/api/v1/public/play/"+key+"/check", map[string]any{"answer": map[string]any{"cells": c.Cells}, "final": true})
	if res["result"].(map[string]any)["solved"] != true {
		a.t.Fatalf("shared crossword: %v", res)
	}
	return key
}

// took sets how long a go took (the server trusts its own clock, not a test's).
func took(a *app, key string, ms int64) {
	a.t.Helper()
	if _, err := a.st.DB.Exec(`UPDATE plays SET ms = ? WHERE play_key = ?`, ms, key); err != nil {
		a.t.Fatal(err)
	}
}

func TestPocketSearch(t *testing.T) {
	a := newApp(t, nil)
	ids := things(a, a.alice)
	hunt := a.expect(201, a.alice, "POST", "/api/v1/adventures", map[string]any{"title": "Garden hunt", "kind": "hunt", "intro": "Find the fruit basket\nbefore dark", "place": "Orchard", "date": "2026-12-24"})
	res := a.ok(a.alice, "GET", "/api/v1/pocket/search?q=fruit", nil)
	all := items(t, res)
	if len(all) != 2 || all[0]["title"] != "Fruit" || all[0]["kind"] != "crossword" || all[0]["id"] != "puzzle:"+ids["puzzle"] || all[0]["url"] != "/puzzles/"+ids["puzzle"] {
		t.Fatalf("the title that has the word leads: %v", all)
	}
	if sn, _ := all[0]["snippet"].(string); !strings.Contains(sn, "words · ") || strings.Contains(strings.ToLower(sn), "private") || all[0]["date"] == "" {
		t.Fatalf("puzzle: %v", all[0])
	}
	if h := all[1]; h["title"] != "Garden hunt" || h["kind"] != "treasure hunt" || h["id"] != "adventure:"+id(hunt) || h["url"] != "/adventures/"+id(hunt) ||
		h["snippet"] != "Orchard · Find the fruit basket" || h["date"] != "2026-12-24" {
		t.Fatalf("hunt: %v", h)
	}
	// private notes find a puzzle but are never shown
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=private", nil)); len(got) != 1 || got[0]["title"] != "Fruit" || strings.Contains(strings.ToLower(got[0]["snippet"].(string)), "private") {
		t.Fatalf("by notes: %v", got)
	}
	esc := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=escape", nil), "Escape", "")
	if esc["kind"] != "escape room" || esc["id"] != "adventure:"+ids["adventure"] || esc["snippet"] != "Get out!" {
		t.Fatalf("escape room: %v", esc)
	}
	book := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=book", nil), "Book", "")
	if book["kind"] != "puzzle book" || book["id"] != "book:"+ids["book"] || book["url"] != "/books/"+ids["book"] || book["snippet"] != "1 puzzle" {
		t.Fatalf("book: %v", book)
	}
	// a word list by its name, and by one of its words
	list := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=words", nil), "Words", "word list")
	if list["id"] != "wordlist:"+ids["wordlist"] || list["url"] != "/words?list="+ids["wordlist"] || list["snippet"] != "2 words" {
		t.Fatalf("word list: %v", list)
	}
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=pea", nil)); len(got) != 1 || got[0]["kind"] != "word list" || got[0]["snippet"] != "2 words · pear" {
		t.Fatalf("by a word: %v", got)
	}
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=fruit&limit=1", nil)); len(got) != 1 || got[0]["title"] != "Fruit" {
		t.Fatalf("limit 1: %v", got)
	}
	// a word many things have: the kinds take turns, so the puzzles do not fill the list
	for _, title := range []string{"Sea one", "Sea two", "Sea three"} {
		a.expect(201, a.alice, "POST", "/api/v1/puzzles", map[string]any{"kind": "sudoku", "title": title, "generate": map[string]any{"level": "easy"}})
	}
	a.expect(201, a.alice, "POST", "/api/v1/adventures", map[string]any{"title": "Sea monsters", "kind": "mystery"})
	a.expect(201, a.alice, "POST", "/api/v1/wordlists", map[string]any{"name": "Sea words", "text": "wave\ntide"})
	a.expect(201, a.alice, "POST", "/api/v1/wordlists", map[string]any{"name": "Beach", "text": "seashell\nsand"})
	got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=sea&limit=4", nil))
	kinds := []string{}
	for _, it := range got {
		kinds = append(kinds, it["kind"].(string))
	}
	if strings.Join(kinds, ", ") != "sudoku, mystery, word list, sudoku" || got[2]["title"] != "Sea words" {
		t.Fatalf("taking turns: %v", got)
	}
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=sea&limit=50", nil)); len(got) != 6 || got[5]["title"] != "Beach" || got[5]["snippet"] != "2 words · seashell" {
		t.Fatalf("a name before a word in the list: %v", got)
	}
	// nothing to look for finds nothing, also where the app's own list would give everything
	for _, q := range []string{"", "%3F%3F", "*", "%25", "_", "+"} {
		if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q="+q, nil)); len(got) != 0 {
			t.Fatalf("q=%q: %v", q, got)
		}
	}
	// another person's things never appear, shared by link and played or not
	nothing := func(when string) {
		t.Helper()
		for _, q := range []string{"fruit", "escape", "book", "words", "apple", "garden", "sea", "e"} {
			if got := items(t, a.ok(a.bob, "GET", "/api/v1/pocket/search?q="+q, nil)); len(got) != 0 {
				t.Fatalf("%s bob finds alice's %q: %v", when, q, got)
			}
		}
	}
	nothing("before sharing")
	solveShared(a, a.alice, ids["puzzle"], "Bob")
	a.ok(a.alice, "PUT", "/api/v1/adventures/"+ids["adventure"], map[string]any{"share_on": true})
	nothing("after sharing")
	a.expect(401, "", "GET", "/api/v1/pocket/search?q=fruit", nil)
}

func TestPocketDay(t *testing.T) {
	a := newApp(t, nil)
	day := today()
	ids := things(a, a.alice)
	a.ok(a.alice, "PUT", "/api/v1/adventures/"+ids["adventure"], map[string]any{"date": day, "time": "18:30", "place": "Garden"})
	took(a, solveDaily(a, a.alice), 125000)
	took(a, solveShared(a, a.alice, ids["puzzle"], "Zoe"), 61000)

	d := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+day, nil)
	if d["date"] != day || d["summary"] != "1 puzzle solved in 2 min, 1 puzzle made, 1 solved by others, 1 escape room planned" {
		t.Fatalf("day: %v", d)
	}
	if len(items(t, d)) != 4 || metric(d, "Solved") != "1" || metric(d, "Time") != "2 min" || metric(d, "Made") != "1" || metric(d, "By others") != "1" {
		t.Fatalf("day: %v", d)
	}
	made := item(t, d, "Fruit", "made")
	if made["id"] != "puzzle:"+ids["puzzle"] || made["url"] != "/puzzles/"+ids["puzzle"] || !strings.HasPrefix(made["sub"].(string), "Crossword · ") || made["done"] != nil || !clockRe.MatchString(made["time"].(string)) {
		t.Fatalf("made: %v", made)
	}
	mine := item(t, d, "Puzzle of the day: Word ladder", "solved")
	if mine["sub"] != "Solved in 2:05" || mine["done"] != true || num(mine, "minutes") != 2.1 || mine["url"] != "/today/"+day+"/ladder" || !clockRe.MatchString(mine["time"].(string)) {
		t.Fatalf("solved: %v", mine)
	}
	theirs := item(t, d, "Fruit", "solved by others")
	if theirs["sub"] != "Zoe solved it in 1:01, 1 hint" || theirs["minutes"] != nil || theirs["url"] != "/puzzles/"+ids["puzzle"]+"?tab=results" {
		t.Fatalf("solved by others: %v", theirs)
	}
	plan := item(t, d, "Escape", "planned")
	if plan["id"] != "adventure:"+ids["adventure"] || plan["time"] != "18:30" || plan["sub"] != "Escape room · Garden" || plan["url"] != "/adventures/"+ids["adventure"] || plan["done"] != nil {
		t.Fatalf("planned: %v", plan)
	}
	for _, it := range items(t, d) {
		b, _ := json.Marshal(it)
		if strings.Contains(strings.ToLower(string(b)), "private") {
			t.Fatalf("the day shows private notes: %s", b)
		}
	}

	// the day is the asker's: 23:30 UTC is half past midnight the next day in Sarajevo
	if _, err := a.st.DB.Exec(`UPDATE plays SET finished_at = '2026-03-10T23:30:00Z' WHERE via = 'me'`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.st.DB.Exec(`UPDATE puzzles SET created_at = '2026-03-10T23:40:00.000Z'`); err != nil {
		t.Fatal(err)
	}
	late := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-03-11?tz=Europe/Sarajevo", nil)
	if len(items(t, late)) != 2 || item(t, late, "Puzzle of the day: Word ladder", "solved")["time"] != "00:30" || item(t, late, "Fruit", "made")["time"] != "00:40" ||
		late["summary"] != "1 puzzle solved in 2 min, 1 puzzle made" {
		t.Fatalf("the 11th in Sarajevo: %v", late)
	}
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-03-10?tz=Europe/Sarajevo", nil); len(items(t, d)) != 0 || d["summary"] != nil {
		t.Fatalf("the 10th in Sarajevo: %v", d)
	}
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-03-10?tz=UTC", nil); len(items(t, d)) != 2 || item(t, d, "Fruit", "made")["time"] != "23:40" {
		t.Fatalf("the 10th in UTC: %v", d)
	}
	// a puzzle put away is not in the day
	a.ok(a.alice, "PUT", "/api/v1/puzzles/"+ids["puzzle"], map[string]any{"archived": true})
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-03-10?tz=UTC", nil); len(items(t, d)) != 1 || d["summary"] != "1 puzzle solved in 2 min" {
		t.Fatalf("after putting the puzzle away: %v", d)
	}

	// what Zoe did with Alice's puzzle is Alice's day, nobody else's
	for _, date := range []string{day, "2026-03-10", "2026-03-11"} {
		if d := a.ok(a.bob, "GET", "/api/v1/pocket/day/"+date, nil); len(items(t, d)) != 0 || d["summary"] != nil || d["metrics"] != nil {
			t.Fatalf("bob's %s: %v", date, d)
		}
	}
	a.expect(400, a.alice, "GET", "/api/v1/pocket/day/today", nil)
	a.expect(401, "", "GET", "/api/v1/pocket/day/"+day, nil)
}

// Two hunts on a day are named as what they are; different games are "games".
func TestPocketDayPlans(t *testing.T) {
	a := newApp(t, nil)
	for _, title := range []string{"North", "South"} {
		a.expect(201, a.alice, "POST", "/api/v1/adventures", map[string]any{"title": title, "kind": "hunt", "date": "2026-06-01"})
	}
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-06-01", nil); d["summary"] != "2 treasure hunts planned" || len(items(t, d)) != 2 || item(t, d, "North", "planned")["time"] != nil {
		t.Fatalf("two hunts: %v", d)
	}
	who := a.expect(201, a.alice, "POST", "/api/v1/adventures", map[string]any{"title": "Who did it", "kind": "mystery", "date": "2026-06-01"})
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-06-01", nil); d["summary"] != "3 games planned" {
		t.Fatalf("mixed: %v", d)
	}
	// the host's own test run, and a team that played it through the link
	for _, p := range []struct {
		via, player, key, at string
		ms                   int64
		hints                int
	}{{"me", "Test run", "key-of-the-test-run-0001", "2026-06-01T09:00:00Z", 750000, 0}, {"link", "Owls", "key-of-the-owls-00000001", "2026-06-01T10:00:00Z", 1870000, 3}} {
		if _, err := a.st.DB.Exec(`INSERT INTO plays (user_id, what, ref_id, kind, via, player, play_key, started_at, finished_at, ms, hints) VALUES (?, 'adventure', ?, 'mystery', ?, ?, ?, ?, ?, ?, ?)`,
			a.ids["Alice"], n(id(who)), p.via, p.player, p.key, p.at, p.at, p.ms, p.hints); err != nil {
			t.Fatal(err)
		}
	}
	d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-06-01?tz=UTC", nil)
	if d["summary"] != "1 test run, 1 solved by others, 3 games planned" || metric(d, "Solved") != "" || metric(d, "By others") != "1" {
		t.Fatalf("played: %v", d)
	}
	if it := item(t, d, "Who did it", "tested"); it["sub"] != "Test run finished in 12:30" || num(it, "minutes") != 12.5 || it["time"] != "09:00" || it["url"] != "/adventures/"+id(who) {
		t.Fatalf("test run: %v", it)
	}
	if it := item(t, d, "Who did it", "solved by others"); it["sub"] != "Owls finished it in 31:10, 3 hints" || it["minutes"] != nil || it["url"] != "/adventures/"+id(who)+"?tab=live" {
		t.Fatalf("a team: %v", it)
	}
}

func TestPocketWidget(t *testing.T) {
	a := newApp(t, nil)
	day := today()
	count := func(table string) (n int) {
		t.Helper()
		if err := a.st.DB.QueryRow(`SELECT COUNT(1) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	// nothing done yet: the three puzzles of the day wait, and reading makes nothing
	w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)
	rows := items(t, w)
	if w["line"] != "Today's 3 puzzles are waiting" || w["url"] != "/today" || len(rows) != 3 || metric(w, "Solved today") != "0 of 3" || metric(w, "Days in a row") != "0" || metric(w, "Puzzles") != "0" {
		t.Fatalf("a new day: %v", w)
	}
	kinds := puzzle.DailyKinds(day)
	for i, it := range rows {
		if it["title"] != "Puzzle of the day: "+puzzle.KindName[kinds[i]] || it["detail"] != "Not solved yet" || it["done"] != nil || it["url"] != "/today/"+day+"/"+kinds[i] || it["id"] != "daily:"+day+":"+kinds[i] {
			t.Fatalf("puzzle of the day %d: %v", i, it)
		}
	}
	if count("plays") != 0 || count("puzzles") != 0 {
		t.Fatalf("the widget made something: %d plays, %d puzzles", count("plays"), count("puzzles"))
	}

	// one started, one solved, on the third day in a row
	a.ok(a.alice, "POST", "/api/v1/daily/"+day+"/sudoku/play", nil)
	took(a, solveDaily(a, a.alice), 245000)
	for i := 1; i <= 2; i++ {
		d := store.AddDays(day, -i)
		if _, err := a.st.DB.Exec(`INSERT INTO plays (user_id, what, kind, day, via, play_key, started_at, finished_at) VALUES (?, 'daily', 'sudoku', ?, 'me', ?, ?, ?)`,
			a.ids["Alice"], d+"|sudoku", "key-of-an-earlier-day-"+d, d+"T10:00:00Z", d+"T10:05:00Z"); err != nil {
			t.Fatal(err)
		}
	}
	ids := things(a, a.alice)
	w = a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)
	if w["line"] != "1 of 3 solved today, 3 days in a row" || metric(w, "Solved today") != "1 of 3" || metric(w, "Days in a row") != "3" || metric(w, "Longest run") != "3" || metric(w, "Puzzles") != "1" {
		t.Fatalf("one solved: %v", w)
	}
	rows = items(t, w)
	if len(rows) != 4 || rows[3]["title"] != "Fruit" || rows[3]["id"] != "puzzle:"+ids["puzzle"] || rows[3]["url"] != "/puzzles/"+ids["puzzle"] || !strings.HasPrefix(rows[3]["detail"].(string), "Crossword · ") {
		t.Fatalf("rows: %v", rows)
	}
	if it := item(t, w, "Puzzle of the day: Word ladder", ""); it["done"] != true || it["detail"] != "Solved in 4:05" {
		t.Fatalf("solved: %v", it)
	}
	if it := item(t, w, "Puzzle of the day: Sudoku", ""); it["done"] != nil || it["detail"] != "Started" {
		t.Fatalf("started: %v", it)
	}

	// the puzzles of the day switched off: only what you made; with nothing made, nothing to show
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"daily": false})
	if w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil); w["line"] != "1 puzzle" || len(items(t, w)) != 1 || items(t, w)[0]["title"] != "Fruit" || metric(w, "Solved today") != "" {
		t.Fatalf("without the puzzles of the day: %v", w)
	}
	// Bob has nothing of Alice's: his own three puzzles of the day, none solved
	w = a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil)
	if w["line"] != "Today's 3 puzzles are waiting" || len(items(t, w)) != 3 || metric(w, "Puzzles") != "0" {
		t.Fatalf("bob: %v", w)
	}
	a.ok(a.bob, "PUT", "/api/v1/settings", map[string]any{"daily": false})
	if w := a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil); len(items(t, w)) != 0 || w["line"] != nil || w["metrics"] != nil {
		t.Fatalf("bob with nothing: %v", w)
	}
	a.expect(401, "", "GET", "/api/v1/pocket/widget", nil)
}

// capture posts a quick line as tok.
func capture(a *app, tok, requestID, text string, more map[string]any) (int, map[string]any) {
	body := map[string]any{"request_id": requestID, "kind": "words", "text": text}
	for k, v := range more {
		body[k] = v
	}
	code, m, _ := a.call(tok, "POST", "/api/v1/pocket/capture", body)
	return code, m
}

// listsOf is a person's word lists, by name.
func (a *app) listsOf(tok string) map[string]map[string]any {
	a.t.Helper()
	out := map[string]map[string]any{}
	for _, l := range a.ok(tok, "GET", "/api/v1/wordlists", nil)["wordlists"].([]any) {
		out[l.(map[string]any)["name"].(string)] = l.(map[string]any)
	}
	return out
}

func TestPocketCapture(t *testing.T) {
	a := newApp(t, nil)
	code, m := capture(a, a.alice, "req-0000000000000001", "apple, pear\n- plum - a stone fruit\n2. Apple", nil)
	if code != 201 || m["title"] != "Apple, pear, plum" || !strings.HasPrefix(m["id"].(string), "wordlist:") || m["url"] != "/words?list="+strings.TrimPrefix(m["id"].(string), "wordlist:") {
		t.Fatalf("capture: %d %v", code, m)
	}
	lists := a.listsOf(a.alice)
	l := lists["Apple, pear, plum"]
	if len(lists) != 1 || l == nil || num(l, "count") != 3 || l["source"] != "" || l["language"] != "en" || "wordlist:"+id(l) != m["id"] {
		t.Fatalf("lists: %v", lists)
	}
	if w := l["words"].([]any)[2].(map[string]any); w["word"] != "plum" || w["clue"] != "a stone fruit" {
		t.Fatalf("a word keeps its clue: %v", l["words"])
	}

	// the same request again: the same answer, nothing added; other words under the same id are refused
	if code, again := capture(a, a.alice, "req-0000000000000001", "apple, pear\n- plum - a stone fruit\n2. Apple", nil); code != 201 || again["id"] != m["id"] || again["title"] != m["title"] || again["url"] != m["url"] {
		t.Fatalf("retry: %d %v", code, again)
	}
	if code, other := capture(a, a.alice, "req-0000000000000001", "sun, moon", nil); code != 400 || other["error"] != "this was already kept with other words" {
		t.Fatalf("same id, other words: %d %v", code, other)
	}
	if lists := a.listsOf(a.alice); len(lists) != 1 {
		t.Fatalf("a retry added a list: %v", lists)
	}

	// the title names the list; more than three words end the made-up name with …; a lone line is the words
	if code, m := capture(a, a.alice, "req-0000000000000002", "sun, moon, star, comet", map[string]any{"title": "  Night   sky "}); code != 201 || m["title"] != "Night sky" {
		t.Fatalf("with a title: %d %v", code, m)
	}
	if code, m := capture(a, a.alice, "req-0000000000000003", "red, green, blue, pink", nil); code != 201 || m["title"] != "Red, green, blue…" {
		t.Fatalf("four words: %d %v", code, m)
	}
	if code, m := capture(a, a.alice, "req-0000000000000004", "", map[string]any{"title": "oak, elm"}); code != 201 || m["title"] != "Oak, elm" {
		t.Fatalf("only a line: %d %v", code, m)
	}
	if lists := a.listsOf(a.alice); len(lists) != 4 || num(lists["Night sky"], "count") != 4 || num(lists["Oak, elm"], "count") != 2 {
		t.Fatalf("lists: %v", lists)
	}

	// what cannot be a list is refused with what to do
	const need = "a word list needs at least two words, with commas between them"
	for _, text := range []string{"apple", "apple, Apple", "a, b, c", "?, !!", "apple pear plum", ", ,\n"} {
		if code, m := capture(a, a.alice, "req-refused-00000001", text, nil); code != 400 || m["error"] != need {
			t.Fatalf("capture %q → %d %v, want 400 %q", text, code, m, need)
		}
	}
	// a refused request left no receipt: the same id works once the line is right
	if code, m := capture(a, a.alice, "req-refused-00000001", "fig, date", nil); code != 201 || m["title"] != "Fig, date" {
		t.Fatalf("after fixing the line: %d %v", code, m)
	}
	for _, body := range []map[string]any{
		{"request_id": "req-0000000000000009", "kind": "puzzle", "text": "sun, moon"},
		{"request_id": "short", "kind": "words", "text": "sun, moon"},
		{"request_id": "req-0000000000000009", "kind": "words"},
	} {
		a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", body)
	}
	if lists := a.listsOf(a.alice); len(lists) != 5 {
		t.Fatalf("refusals added lists: %v", lists)
	}

	// a request id is one person's: Bob's same id makes his own list, and he sees none of Alice's
	if code, m := capture(a, a.bob, "req-0000000000000001", "cat, dog", nil); code != 201 || m["title"] != "Cat, dog" {
		t.Fatalf("bob: %d %v", code, m)
	}
	if lists := a.listsOf(a.bob); len(lists) != 1 || lists["Cat, dog"] == nil {
		t.Fatalf("bob's lists: %v", lists)
	}
	a.expect(401, "", "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-0000000000000010", "text": "sun, moon"})
}

// The same line sent several times at once is kept once.
func TestPocketCaptureAtOnce(t *testing.T) {
	a := newApp(t, nil)
	var wg sync.WaitGroup
	got := make([]map[string]any, 8)
	codes := make([]int, 8)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], got[i] = capture(a, a.alice, "req-at-once-00000001", "north, south, east, west", nil)
		}()
	}
	wg.Wait()
	for i := range got {
		if codes[i] != 201 || got[i]["id"] != got[0]["id"] {
			t.Fatalf("answer %d: %d %v (first %v)", i, codes[i], got[i], got[0])
		}
	}
	if lists := a.listsOf(a.alice); len(lists) != 1 {
		t.Fatalf("kept %d times: %v", len(lists), lists)
	}
}

// A link token with one pocket:* scope reaches its one route and nothing else.
func TestPocketLinkTokens(t *testing.T) {
	a, _ := newCapApp(t)
	ids := things(a, a.alice)
	day := today()
	line := func(requestID string) map[string]any {
		return map[string]any{"request_id": requestID, "kind": "words", "text": "sun, moon"}
	}
	for _, c := range []struct {
		tok, method, path string
		body              any
		want              int
	}{
		{"pcx_alice_search", "GET", "/api/v1/pocket/search?q=fruit", nil, 200},
		{"pcx_alice_day", "GET", "/api/v1/pocket/day/" + day, nil, 200},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/widget", nil, 200},
		{"pcx_alice_capture", "POST", "/api/v1/pocket/capture", line("req-0000000000000001"), 201},
		{"pcx_alice_widget", "GET", "/api/v1/auth/me", nil, 200},
		// another capability's route
		{"pcx_alice_search", "GET", "/api/v1/pocket/day/" + day, nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_search", "POST", "/api/v1/pocket/capture", line("req-0000000000000002"), 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/search?q=fruit", nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/search?q=fruit", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/day/" + day, nil, 403},
		{"pcx_alice_widget", "POST", "/api/v1/pocket/capture", line("req-0000000000000003"), 403},
		{"pcx_alice_capture", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_capture", "GET", "/api/v1/pocket/search?q=fruit", nil, 403},
		// the rest of the app
		{"pcx_alice_widget", "GET", "/api/v1/today", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/home", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/summary", nil, 403},
		{"pcx_alice_widget", "POST", "/api/v1/daily/" + day + "/ladder/play", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/search?q=fruit", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/puzzles", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/puzzles/" + ids["puzzle"], nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/day/" + day, nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/stats", nil, 403},
		{"pcx_alice_capture", "GET", "/api/v1/wordlists", nil, 403},
		{"pcx_alice_capture", "POST", "/api/v1/wordlists", map[string]any{"name": "x"}, 403},
		{"pcx_alice_capture", "PUT", "/api/v1/wordlists/" + ids["wordlist"], map[string]any{"name": "x"}, 403},
		{"pcx_alice_capture", "DELETE", "/api/v1/wordlists/" + ids["wordlist"], nil, 403},
		{"pcx_alice_capture", "POST", "/api/v1/puzzles", map[string]any{"kind": "sudoku"}, 403},
		{"pcx_alice_capture", "GET", "/api/v1/settings", nil, 403},
		{"pcx_alice_capture", "GET", "/api/v1/export.json", nil, 403},
		// the app's own scopes do not open the new routes
		{"pcx_alice_read", "GET", "/api/v1/pocket/search?q=fruit", nil, 403},
		{"pcx_alice_read", "GET", "/api/v1/pocket/day/" + day, nil, 403},
		{"pcx_alice_read", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_write", "POST", "/api/v1/pocket/capture", line("req-0000000000000004"), 403},
		{"pcx_alice_read", "GET", "/api/v1/search?q=fruit", nil, 200},
		{"pcx_nobody", "GET", "/api/v1/pocket/widget", nil, 401},
	} {
		if code, m, _ := a.call(c.tok, c.method, c.path, c.body); code != c.want {
			t.Errorf("%s %s %s → %d, want %d (%v)", c.tok, c.method, c.path, code, c.want, m)
		}
	}
	// the token acts for its person only
	if got := items(t, a.ok("pcx_alice_search", "GET", "/api/v1/pocket/search?q=fruit", nil)); len(got) != 1 || got[0]["title"] != "Fruit" {
		t.Fatalf("alice through the link: %v", got)
	}
	if got := items(t, a.ok("pcx_bob_search", "GET", "/api/v1/pocket/search?q=fruit", nil)); len(got) != 0 {
		t.Fatalf("bob through the link finds alice's puzzle: %v", got)
	}
	if d := a.ok("pcx_alice_day", "GET", "/api/v1/pocket/day/"+day, nil); item(t, d, "Fruit", "made") == nil {
		t.Fatalf("alice's day through the link: %v", d)
	}
	if d := a.ok("pcx_bob_day", "GET", "/api/v1/pocket/day/"+day, nil); len(items(t, d)) != 0 {
		t.Fatalf("bob's day through the link: %v", d)
	}
	if w := a.ok("pcx_bob_widget", "GET", "/api/v1/pocket/widget", nil); len(items(t, w)) != 3 || metric(w, "Puzzles") != "0" {
		t.Fatalf("bob's widget through the link: %v", w)
	}
	// only the one allowed capture landed, marked with the app that sent it; a retry adds nothing
	if code, m := capture(a, "pcx_alice_capture", "req-0000000000000001", "sun, moon", nil); code != 201 || m["title"] != "Sun, moon" {
		t.Fatalf("retry through the link: %d %v", code, m)
	}
	lists := a.listsOf(a.alice)
	if len(lists) != 2 || lists["Sun, moon"] == nil || lists["Sun, moon"]["source"] != "hub" {
		t.Fatalf("alice's lists: %v", lists)
	}
	if lists := a.listsOf(a.bob); len(lists) != 0 {
		t.Fatalf("bob's lists: %v", lists)
	}
}

func TestPocketLinkScopes(t *testing.T) {
	req := func(m, p string) *http.Request { return httptest.NewRequest(m, p, nil) }
	for _, c := range []struct {
		scope string
		r     *http.Request
		want  bool
	}{
		{core.ScopeSearch, req("GET", "/api/v1/pocket/search?q=x"), true},
		{core.ScopeDay, req("GET", "/api/v1/pocket/day/2026-01-02"), true},
		{core.ScopeWidget, req("GET", "/api/v1/pocket/widget"), true},
		{core.ScopeCapture, req("POST", "/api/v1/pocket/capture"), true},
		{core.ScopeWidget, req("GET", "/api/v1/auth/me"), true},
		{core.ScopeSearch, req("POST", "/api/v1/pocket/search"), false},
		{core.ScopeDay, req("GET", "/api/v1/pocket/day/today"), false},
		{core.ScopeDay, req("GET", "/api/v1/day/2026-01-02"), false},
		{core.ScopeSearch, req("GET", "/api/v1/search?q=x"), false},
		{core.ScopeWidget, req("GET", "/api/v1/today"), false},
		{core.ScopeCapture, req("GET", "/api/v1/pocket/capture"), false},
		{core.ScopeCapture, req("POST", "/api/v1/wordlists"), false},
		{"puzzlecraft:read", req("GET", "/api/v1/pocket/search?q=x"), false},
		{"puzzlecraft:read", req("GET", "/api/v1/pocket/day/2026-01-02"), false},
		{"puzzlecraft:read", req("GET", "/api/v1/pocket/widget"), false},
		{"puzzlecraft:write", req("POST", "/api/v1/pocket/capture"), false},
		{core.ScopeMoveExport, req("GET", "/api/v1/pocket/widget"), false},
	} {
		if got := linkAllows([]string{c.scope}, c.r); got != c.want {
			t.Errorf("%s %s %s = %v, want %v", c.scope, c.r.Method, c.r.URL.Path, got, c.want)
		}
	}
}

// Puzzlecraft tells Core what it can do, so Search, Day and Hub find it on their own.
func TestManifestCapabilities(t *testing.T) {
	a, fc := newCapApp(t)
	if err := a.s.registerWithCore(context.Background()); err != nil {
		t.Fatal(err)
	}
	fc.mu.Lock()
	m := fc.manifest
	fc.mu.Unlock()
	if m["slug"] != "puzzlecraft" || m["category"] != "Media" {
		t.Fatalf("manifest: %v %v", m["slug"], m["category"])
	}
	search, _ := m["search"].(map[string]any)
	day, _ := m["day"].(map[string]any)
	widget, _ := m["widget"].(map[string]any)
	cap, _ := m["capture"].(map[string]any)
	if search == nil || day == nil || day["label"] != "Puzzles" || day["one"] != "puzzle" || day["many"] != "puzzles" || widget == nil || widget["title"] != "Daily puzzle" || cap == nil {
		t.Fatalf("capabilities: %v %v %v %v", search, day, widget, cap)
	}
	bangs := []string{}
	for _, b := range search["bangs"].([]any) {
		bangs = append(bangs, b.(string))
	}
	kinds := map[string]bool{}
	for _, k := range search["kinds"].([]any) {
		kinds[k.(string)] = true
	}
	if strings.Join(bangs, " ") != "puzzles puzzle pz" || len(kinds) != 17 || !kinds["crossword"] || !kinds["word search"] || !kinds["treasure hunt"] || !kinds["escape room"] ||
		!kinds["puzzle book"] || !kinds["word list"] {
		t.Fatalf("search: %v", search)
	}
	if k := cap["kinds"].([]any)[0].(map[string]any); len(cap["kinds"].([]any)) != 1 || k["kind"] != "words" || len(k["fields"].([]any)) != 1 || k["fields"].([]any)[0] != "title" {
		t.Fatalf("capture kinds: %v", cap)
	}
	offered := map[string]bool{}
	for _, o := range m["offers"].([]any) {
		offered[o.(map[string]any)["scope"].(string)] = true
	}
	for _, sc := range []string{"puzzlecraft:read", "puzzlecraft:write", core.ScopeSearch, core.ScopeDay, core.ScopeWidget, core.ScopeCapture, core.ScopeMoveExport, core.ScopeMoveImport} {
		if !offered[sc] {
			t.Errorf("%s is not offered", sc)
		}
	}
}
