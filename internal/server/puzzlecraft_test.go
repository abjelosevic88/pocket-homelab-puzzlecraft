package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/config"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
)

// things sets up a puzzle, a book, a word list and an adventure with a stage for tok.
func things(a *app, tok string) map[string]string {
	a.t.Helper()
	out := map[string]string{}
	p := a.expect(201, tok, "POST", "/api/v1/puzzles", map[string]any{"kind": "crossword", "title": "Fruit", "notes": "Private notes",
		"generate": map[string]any{"theme": "fruit"}})
	out["puzzle"] = id(p)
	b := a.expect(201, tok, "POST", "/api/v1/books", map[string]any{"title": "Book", "puzzle_ids": []int64{n(out["puzzle"])}})
	out["book"] = id(b)
	l := a.expect(201, tok, "POST", "/api/v1/wordlists", map[string]any{"name": "Words", "text": "apple - a fruit\npear: another fruit"})
	out["wordlist"] = id(l)
	adv := a.expect(201, tok, "POST", "/api/v1/adventures", map[string]any{"title": "Escape", "intro": "Get out!"})
	out["adventure"] = id(adv)
	st := a.expect(201, tok, "POST", "/api/v1/adventures/"+out["adventure"]+"/stages", map[string]any{"title": "Lock", "lock": "code", "answers": []string{"1234"},
		"hidden": "Under the rug", "hints": []map[string]any{{"text": "Count the chairs", "after": 0}}})
	out["stage"] = id(st)
	return out
}

func TestEverythingIsPerPerson(t *testing.T) {
	a := newApp(t, nil)
	ids := things(a, a.alice)
	for _, path := range []string{"/api/v1/puzzles", "/api/v1/books", "/api/v1/wordlists", "/api/v1/adventures"} {
		m := a.ok(a.bob, "GET", path, nil)
		for _, k := range []string{"puzzles", "books", "wordlists", "adventures"} {
			if l, ok := m[k].([]any); ok && len(l) != 0 {
				t.Fatalf("bob sees %d %s", len(l), k)
			}
		}
	}
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/v1/puzzles/" + ids["puzzle"]}, {"PUT", "/api/v1/puzzles/" + ids["puzzle"]}, {"DELETE", "/api/v1/puzzles/" + ids["puzzle"]},
		{"POST", "/api/v1/puzzles/" + ids["puzzle"] + "/share"}, {"POST", "/api/v1/puzzles/" + ids["puzzle"] + "/duplicate"},
		{"POST", "/api/v1/puzzles/" + ids["puzzle"] + "/test"}, {"POST", "/api/v1/puzzles/" + ids["puzzle"] + "/play"},
		{"GET", "/api/v1/puzzles/" + ids["puzzle"] + "/plays"}, {"GET", "/api/v1/puzzles/" + ids["puzzle"] + "/export.ipuz"},
		{"GET", "/api/v1/books/" + ids["book"]}, {"PUT", "/api/v1/books/" + ids["book"]}, {"DELETE", "/api/v1/books/" + ids["book"]}, {"GET", "/api/v1/books/" + ids["book"] + "/print"},
		{"GET", "/api/v1/wordlists/" + ids["wordlist"]}, {"PUT", "/api/v1/wordlists/" + ids["wordlist"]}, {"DELETE", "/api/v1/wordlists/" + ids["wordlist"]},
		{"GET", "/api/v1/adventures/" + ids["adventure"]}, {"PUT", "/api/v1/adventures/" + ids["adventure"]}, {"DELETE", "/api/v1/adventures/" + ids["adventure"]},
		{"POST", "/api/v1/adventures/" + ids["adventure"] + "/stages"}, {"POST", "/api/v1/adventures/" + ids["adventure"] + "/play"},
		{"GET", "/api/v1/adventures/" + ids["adventure"] + "/live"}, {"PUT", "/api/v1/stages/" + ids["stage"]}, {"DELETE", "/api/v1/stages/" + ids["stage"]},
	} {
		a.expect(404, a.bob, c.method, c.path, map[string]any{"title": "Mine", "name": "Mine"})
	}
	// Bob cannot put Alice's puzzle in his book
	b := a.expect(201, a.bob, "POST", "/api/v1/books", map[string]any{"title": "Mine", "puzzle_ids": []int64{n(ids["puzzle"])}})
	if len(b["items"].([]any)) != 0 {
		t.Fatalf("bob's book holds alice's puzzle: %v", b)
	}
	if d := a.ok(a.alice, "GET", "/api/v1/puzzles/"+ids["puzzle"], nil); d["notes"] != "Private notes" {
		t.Fatalf("alice's puzzle: %v", d)
	}
}

func TestSharedPuzzlePlay(t *testing.T) {
	a := newApp(t, nil)
	ids := things(a, a.alice)
	pz := a.ok(a.alice, "GET", "/api/v1/puzzles/"+ids["puzzle"], nil)
	token := pz["share_token"].(string)
	a.expect(404, "", "GET", "/api/v1/public/p/"+token, nil)
	a.ok(a.alice, "POST", "/api/v1/puzzles/"+ids["puzzle"]+"/share", map[string]any{"on": true})
	pub := a.ok("", "GET", "/api/v1/public/p/"+token, nil)
	if pub["title"] != "Fruit" || pub["notes"] != nil {
		t.Fatalf("public: %v", pub)
	}
	play := a.ok("", "POST", "/api/v1/public/p/"+token+"/play", map[string]any{"player": "Zoe"})
	raw, _ := json.Marshal(play)
	var c puzzle.Crossword
	_ = json.Unmarshal(mustJSON(pz["data"]), &c)
	for _, e := range puzzle.Entries(&c) {
		if len(e.Answer) > 4 && strings.Contains(string(raw), e.Answer) {
			t.Fatalf("the player's view gives away %s", e.Answer)
		}
	}
	key := play["key"].(string)
	// a wrong letter, then the right grid
	wrong := append([]string(nil), c.Cells...)
	for i, v := range wrong {
		if v != "" {
			wrong[i] = "Q"
			break
		}
	}
	res := a.ok("", "POST", "/api/v1/public/play/"+key+"/check", map[string]any{"answer": map[string]any{"cells": wrong}})
	if res["result"].(map[string]any)["solved"] == true {
		t.Fatal("wrong grid solved")
	}
	a.ok("", "POST", "/api/v1/public/play/"+key+"/reveal", map[string]any{"what": 0})
	res = a.ok("", "POST", "/api/v1/public/play/"+key+"/check", map[string]any{"answer": map[string]any{"cells": c.Cells}, "final": true, "ms": 61000})
	if res["result"].(map[string]any)["solved"] != true || res["finished_at"] == nil {
		t.Fatalf("right grid: %v", res)
	}
	board := a.ok("", "GET", "/api/v1/public/p/"+token, nil)["best"].([]any)
	if len(board) != 1 || board[0].(map[string]any)["player"] != "Zoe" {
		t.Fatalf("board: %v", board)
	}
	// switched off: the go stops too
	a.ok(a.alice, "POST", "/api/v1/puzzles/"+ids["puzzle"]+"/share", map[string]any{"on": false})
	a.expect(404, "", "GET", "/api/v1/public/play/"+key, nil)
}

func TestAdventureLocksAndHints(t *testing.T) {
	a := newApp(t, nil)
	ids := things(a, a.alice)
	a.ok(a.alice, "POST", "/api/v1/adventures/"+ids["adventure"]+"/stages", map[string]any{"title": "Arrows", "lock": "directions", "answers": []string{"up up left"}})
	a.ok(a.alice, "POST", "/api/v1/adventures/"+ids["adventure"]+"/stages", map[string]any{"title": "Card", "lock": "qr", "hints": []map[string]any{{"text": "Look up", "after": 30}}})
	adv := a.ok(a.alice, "PUT", "/api/v1/adventures/"+ids["adventure"], map[string]any{"share_on": true})
	token := adv["share_token"].(string)
	stages := adv["stages"].([]any)
	code := stages[2].(map[string]any)["code"].(string)
	pub := a.ok("", "GET", "/api/v1/public/a/"+token, nil)
	b, _ := json.Marshal(pub)
	if strings.Contains(string(b), "Under the rug") || strings.Contains(string(b), "1234") {
		t.Fatalf("players see the host's notes: %s", b)
	}
	play := a.ok("", "POST", "/api/v1/public/a/"+token+"/play", map[string]any{"player": "Owls"})
	key := play["key"].(string)
	b, _ = json.Marshal(play)
	if strings.Contains(string(b), "1234") || strings.Contains(string(b), code) || strings.Contains(string(b), "Under the rug") {
		t.Fatalf("the player's view gives too much away: %s", b)
	}
	h := a.ok("", "POST", "/api/v1/public/play/"+key+"/hint", nil)
	if hs := h["adventure"].(map[string]any)["stage"].(map[string]any)["hints"].([]any); len(hs) != 1 {
		t.Fatalf("hint: %v", h)
	}
	if r := a.ok("", "POST", "/api/v1/public/play/"+key+"/answer", map[string]any{"answer": "4321"}); r["opened"] == true {
		t.Fatal("wrong code opened the lock")
	}
	if r := a.ok("", "POST", "/api/v1/public/play/"+key+"/answer", map[string]any{"answer": "1-2-3-4"}); r["opened"] != true {
		t.Fatalf("code: %v", r)
	}
	if r := a.ok("", "POST", "/api/v1/public/play/"+key+"/answer", map[string]any{"answer": "↑↑←"}); r["opened"] != true {
		t.Fatalf("directions: %v", r)
	}
	// the card's hint is not due for 30 minutes
	a.expect(400, "", "POST", "/api/v1/public/play/"+key+"/hint", nil)
	if r := a.ok("", "POST", "/api/v1/public/play/"+key+"/scan", map[string]any{"code": "nope"}); r["opened"] == true {
		t.Fatal("a made-up card opened the stage")
	}
	r := a.ok("", "POST", "/api/v1/public/play/"+key+"/scan", map[string]any{"code": code})
	if r["opened"] != true || r["play"].(map[string]any)["finished_at"] == nil {
		t.Fatalf("finish: %v", r)
	}
	live := a.ok(a.alice, "GET", "/api/v1/adventures/"+ids["adventure"]+"/live", nil)
	if p := live["plays"].([]any); len(p) != 1 || num(p[0].(map[string]any), "done") != 3 {
		t.Fatalf("live: %v", live)
	}
}

func TestDailyAndStats(t *testing.T) {
	a := newApp(t, nil)
	td := a.ok(a.alice, "GET", "/api/v1/today", nil)
	daily := td["daily"].([]any)
	if len(daily) != 3 {
		t.Fatalf("today: %v", td)
	}
	day := td["date"].(string)
	p := a.ok(a.alice, "POST", "/api/v1/daily/"+day+"/ladder/play", nil)
	m, _ := puzzle.Daily("ladder", day, "")
	var l puzzle.Ladder
	_ = json.Unmarshal(m.Data, &l)
	res := a.ok("", "POST", "/api/v1/public/play/"+p["key"].(string)+"/check", map[string]any{"answer": map[string]any{"answers": l.Steps[1 : len(l.Steps)-1]}, "final": true})
	if res["result"].(map[string]any)["solved"] != true {
		t.Fatalf("ladder: %v", res)
	}
	if s := a.ok(a.alice, "GET", "/api/v1/today", nil); num(s, "streak") != 1 {
		t.Fatalf("streak: %v", s)
	}
	if st := a.ok(a.alice, "GET", "/api/v1/stats", nil); num(st, "solved") != 1 {
		t.Fatalf("stats: %v", st)
	}
	if d := a.ok(a.alice, "GET", "/api/v1/day/"+day, nil); num(d, "count") != 1 {
		t.Fatalf("day: %v", d)
	}
	a.expect(400, a.alice, "POST", "/api/v1/daily/2999-01-01/ladder/play", nil)
}

func TestExportImportTwice(t *testing.T) {
	a := newApp(t, nil)
	things(a, a.alice)
	req, _ := http.NewRequest("GET", a.url+"/api/v1/export.json", nil)
	req.Header.Set("Authorization", "Bearer "+a.alice)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("export: %v", err)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(res.Body)
	res.Body.Close()
	for i := 0; i < 2; i++ {
		req, _ = http.NewRequest("POST", a.url+"/api/v1/import", bytes.NewReader(buf.Bytes()))
		req.Header.Set("Authorization", "Bearer "+a.bob)
		req.Header.Set("Content-Type", "application/json")
		res, err = http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("import %d: %v %v", i, err, res.StatusCode)
		}
		res.Body.Close()
	}
	m := a.ok(a.bob, "GET", "/api/v1/puzzles", nil)
	if len(m["puzzles"].([]any)) != 1 || len(a.ok(a.bob, "GET", "/api/v1/books", nil)["books"].([]any)) != 1 ||
		len(a.ok(a.bob, "GET", "/api/v1/adventures", nil)["adventures"].([]any)) != 1 || len(a.ok(a.bob, "GET", "/api/v1/wordlists", nil)["wordlists"].([]any)) != 1 {
		t.Fatalf("after two imports bob has %v", m)
	}
	advs := a.ok(a.bob, "GET", "/api/v1/adventures", nil)["adventures"].([]any)
	if got := a.ok(a.bob, "GET", "/api/v1/adventures/"+id(advs[0].(map[string]any)), nil); len(got["stages"].([]any)) != 1 {
		t.Fatalf("imported adventure: %v", got)
	}
}

func TestCrosswordFilesRoundTrip(t *testing.T) {
	a := newApp(t, nil)
	mini := &puzzle.Crossword{W: 5, H: 5, Style: "classic", Language: "en", Cells: []string{"V", "I", "A", "L", "S", "R", "#", "L", "#", "L", "O", "L", "I", "V", "E", "O", "#", "B", "#", "E", "M", "O", "I", "S", "T"},
		Clues: map[string]string{"1A": "Bottles", "1D": "Engine sound", "2D": "Excuse", "3D": "Icy rain", "4A": "Fruit", "5A": "Damp"}}
	p := a.expect(201, a.alice, "POST", "/api/v1/puzzles", map[string]any{"kind": "crossword", "title": "Mini", "data": mini})
	if p["ready"] != true {
		t.Fatalf("mini not ready: %v", p)
	}
	for _, ext := range []string{"puz", "ipuz"} {
		req, _ := http.NewRequest("GET", a.url+"/api/v1/puzzles/"+id(p)+"/export."+ext, nil)
		req.Header.Set("Authorization", "Bearer "+a.alice)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("export %s: %v", ext, res.StatusCode)
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(res.Body)
		res.Body.Close()
		req, _ = http.NewRequest("POST", a.url+"/api/v1/import?name=x."+ext, bytes.NewReader(buf.Bytes()))
		req.Header.Set("Authorization", "Bearer "+a.bob)
		res, err = http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("import %s: %v", ext, res.StatusCode)
		}
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		got := a.ok(a.bob, "GET", "/api/v1/puzzles/"+itoa(int64(num(out, "puzzle_id"))), nil)
		d := got["data"].(map[string]any)
		cl := d["clues"].(map[string]any)
		if cl["4A"] != "Fruit" || cl["3D"] != "Icy rain" || d["cells"].([]any)[10] != "O" {
			t.Fatalf("%s round trip: %v", ext, d)
		}
	}
	// a sudoku as 81 digits
	req, _ := http.NewRequest("POST", a.url+"/api/v1/import?name=s.txt", strings.NewReader("530070000600195000098000060800060003400803001700020006060000280000419005000080079"))
	req.Header.Set("Authorization", "Bearer "+a.alice)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("sudoku text: %v", res.StatusCode)
	}
	res.Body.Close()
}

func TestLinkTokensSeeNoAnswers(t *testing.T) {
	a := newApp(t, nil)
	ids := things(a, a.alice)
	pz, _ := a.s.store.GetPuzzle(t.Context(), a.ids["Alice"], n(ids["puzzle"]))
	ctx := t.Context()
	ctx = withLinkApp(ctx, "ai")
	doc := a.s.puzzleDoc(ctx, nil, pz)
	b, _ := json.Marshal(doc)
	var c puzzle.Crossword
	_ = json.Unmarshal(pz.Data, &c)
	for _, e := range puzzle.Entries(&c) {
		if len(e.Answer) > 4 && strings.Contains(string(b), e.Answer) {
			t.Fatalf("another app sees the answer %s", e.Answer)
		}
	}
	if doc["notes"] != "" {
		t.Fatal("another app sees private notes")
	}
}

func TestSettingsArePersonal(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"language": "bcs", "notes_folder": "Fun/Puzzles", "level": "hard"})
	if s := a.ok(a.bob, "GET", "/api/v1/settings", nil); s["language"] != "en" || s["notes_folder"] != "Puzzles" {
		t.Fatalf("bob's settings changed: %v", s)
	}
	if s := a.ok(a.alice, "GET", "/api/v1/settings", nil); s["language"] != "bcs" || s["level"] != "hard" {
		t.Fatalf("alice's settings: %v", s)
	}
	a.expect(400, a.alice, "PUT", "/api/v1/settings", map[string]any{"language": "klingon"})
	a.expect(400, a.alice, "PUT", "/api/v1/settings", map[string]any{"paper_size": "a3"})
}

func TestLinkScopes(t *testing.T) {
	r := func(method, path string) *http.Request { return httptest.NewRequest(method, path, nil) }
	read, write := []string{"puzzlecraft:read"}, []string{"puzzlecraft:write"}
	for _, c := range []struct {
		scopes []string
		req    *http.Request
		want   bool
	}{
		{read, r("GET", "/api/v1/summary"), true},
		{read, r("GET", "/api/v1/puzzles/3"), true},
		{read, r("GET", "/api/v1/adventures/3"), true},
		{read, r("GET", "/api/v1/puzzles/3/plays"), true},
		{read, r("GET", "/api/v1/day/2026-11-13"), true},
		{read, r("POST", "/api/v1/puzzles"), false},
		{read, r("GET", "/api/v1/settings"), false},
		{read, r("GET", "/api/v1/export.json"), false},
		{write, r("POST", "/api/v1/puzzles"), true},
		{write, r("PUT", "/api/v1/puzzles/4"), true},
		{write, r("POST", "/api/v1/puzzles/4/share"), true},
		{write, r("POST", "/api/v1/generate"), true},
		{write, r("DELETE", "/api/v1/adventures/4"), false},
		{write, r("PUT", "/api/v1/stages/4"), false},
		{nil, r("GET", "/api/v1/auth/me"), true},
	} {
		if got := linkAllows(c.scopes, c.req); got != c.want {
			t.Errorf("%v %s %s = %v, want %v", c.scopes, c.req.Method, c.req.URL.Path, got, c.want)
		}
	}
}

func TestDemo(t *testing.T) {
	a := newApp(t, func(c *config.Config) { c.DemoData = true })
	m := a.ok(a.alice, "GET", "/api/v1/puzzles", nil)
	got := len(m["puzzles"].([]any))
	if got < 15 {
		t.Fatalf("%d demo puzzles", got)
	}
	for _, p := range m["puzzles"].([]any) {
		pm := p.(map[string]any)
		if pm["ready"] != true {
			t.Errorf("demo puzzle %q is not ready", pm["title"])
		}
	}
	if err := a.s.seedDemo(t.Context(), a.ids["Alice"]); err != nil {
		t.Fatal(err)
	}
	if l := a.ok(a.alice, "GET", "/api/v1/puzzles", nil)["puzzles"].([]any); len(l) != got {
		t.Fatalf("%d puzzles after a second seed", len(l))
	}
	if s := a.ok(a.alice, "GET", "/api/v1/today", nil); num(s, "best_streak") < 10 {
		t.Fatalf("demo streak: %v", s["best_streak"])
	}
	if adv := a.ok(a.alice, "GET", "/api/v1/adventures", nil)["adventures"].([]any); len(adv) != 3 || num(adv[0].(map[string]any), "live") != 1 {
		t.Fatalf("demo adventures: %v", adv)
	}
}

func TestShorthandMake(t *testing.T) {
	a := newApp(t, nil)
	p := a.expect(201, a.alice, "POST", "/api/v1/puzzles", map[string]any{"kind": "wordsearch", "title": "Spelling", "words_text": "because\nfriend\nschool\nlater"})
	if p["ready"] != true || !strings.Contains(p["size_text"].(string), "4 words") {
		t.Fatalf("word search from text: %v", p)
	}
	s := a.expect(201, a.alice, "POST", "/api/v1/puzzles", map[string]any{"kind": "sudoku", "level": "easy", "size": 4})
	if !strings.HasPrefix(s["size_text"].(string), "4 × 4") {
		t.Fatalf("4×4 sudoku: %v", s["size_text"])
	}
}
