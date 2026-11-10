package server

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// ---- puzzle books ----

type bookIn struct {
	Title     *string  `json:"title"`
	Subtitle  *string  `json:"subtitle"`
	Notes     *string  `json:"notes"`
	Answers   *string  `json:"answers"`
	Large     *bool    `json:"large"`
	ShareOn   *bool    `json:"share_on"`
	PuzzleIDs *[]int64 `json:"puzzle_ids"`
	Renew     bool     `json:"renew"`
}

func (in bookIn) apply(b *store.Book) error {
	trimP(&b.Title, in.Title)
	b.Title = clip(b.Title, maxName)
	if b.Title == "" {
		return &userErr{"a book needs a title"}
	}
	trimP(&b.Subtitle, in.Subtitle)
	b.Subtitle = clip(b.Subtitle, maxLine)
	if in.Notes != nil {
		b.Notes = cleanText(*in.Notes, maxNotes)
	}
	if in.Answers != nil {
		if *in.Answers != "back" && *in.Answers != "none" {
			return &userErr{"answers go at the back or nowhere"}
		}
		b.Answers = *in.Answers
	}
	if in.Large != nil {
		b.Large = *in.Large
	}
	if in.ShareOn != nil {
		b.ShareOn = *in.ShareOn
	}
	return nil
}

func (s *Server) bookDoc(b *store.Book) map[string]any {
	var doc map[string]any
	_ = remarshal(b, &doc)
	doc["share_url"] = s.link("/b/" + b.ShareToken)
	return doc
}

func (s *Server) handleListBooks(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListBooks(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "books")
		return
	}
	out := []map[string]any{}
	for i := range list {
		out = append(out, s.bookDoc(&list[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": out})
}

func (s *Server) handleGetBook(w http.ResponseWriter, r *http.Request) {
	b, err := s.store.GetBook(r.Context(), actorID(r.Context()), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "book")
		return
	}
	writeJSON(w, http.StatusOK, s.bookDoc(b))
}

func (s *Server) handleSaveBook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in bookIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	b := &store.Book{UserID: uid, Answers: "back", ShareToken: newToken()}
	status := http.StatusCreated
	if id := idParam(r, "id"); id > 0 {
		got, err := s.store.GetBook(ctx, uid, id)
		if err != nil {
			s.fail(w, err, "book")
			return
		}
		b, status = got, http.StatusOK
	}
	if err := in.apply(b); err != nil {
		s.userFail(w, err, "book")
		return
	}
	var ids []int64
	if in.PuzzleIDs != nil {
		ids = *in.PuzzleIDs
		if ids == nil {
			ids = []int64{}
		}
	}
	if err := s.store.SaveBook(ctx, b, ids); err != nil {
		s.fail(w, err, "save book")
		return
	}
	if in.Renew {
		_ = s.store.SetShareToken(ctx, "books", uid, b.ID, newToken())
	}
	got, _ := s.store.GetBook(ctx, uid, b.ID)
	writeJSON(w, status, s.bookDoc(got))
}

func (s *Server) handleDeleteBook(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteBook(r.Context(), actorID(r.Context()), idParam(r, "id")); err != nil {
		s.fail(w, err, "book")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleBookPrint is a book's puzzles with their answers, for the print page.
func (s *Server) handleBookPrint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	b, err := s.store.GetBook(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "book")
		return
	}
	puzzles := []map[string]any{}
	for _, it := range b.Items {
		if p, err := s.store.GetPuzzle(ctx, uid, it.PuzzleID); err == nil {
			puzzles = append(puzzles, s.printDoc(p))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"book": s.bookDoc(b), "puzzles": puzzles})
}

// handlePuzzlePrint is one puzzle for the print page: what players see and the answer.
func (s *Server) handlePuzzlePrint(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetPuzzle(r.Context(), actorID(r.Context()), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	st := s.settingsOf(r.Context(), p.UserID)
	writeJSON(w, http.StatusOK, map[string]any{"puzzle": s.printDoc(p), "paper": st.PaperSize, "large": st.LargePrint, "share_url": s.link("/p/" + p.ShareToken), "shared": p.ShareOn})
}

// printDoc is a puzzle for printing: what players see and the answer key.
func (s *Server) printDoc(p *store.Puzzle) map[string]any {
	out := s.publicPuzzle(p)
	out["answer"] = p.Data
	if p.Kind == "crossword" {
		out["entries"] = entriesOf(p.Data)
	}
	return out
}

func entriesOf(data []byte) []puzzle.Entry {
	c, err := decodeCrossword(data)
	if err != nil {
		return nil
	}
	return puzzle.Entries(c)
}

// handlePublicBook is a shared book: its puzzles to play one by one.
func (s *Server) handlePublicBook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	b, err := s.store.BookByShare(ctx, chiParam(r, "token"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this book is not shared (any more)")
		return
	}
	items := []map[string]any{}
	for _, it := range b.Items {
		items = append(items, map[string]any{"puzzle_id": it.PuzzleID, "title": it.Title, "kind": it.Kind, "kind_name": puzzle.KindName[it.Kind], "level": it.Level, "size_text": it.SizeText})
	}
	writeJSON(w, http.StatusOK, map[string]any{"title": b.Title, "subtitle": b.Subtitle, "from": s.ownerName(ctx, b.UserID), "items": items})
}

// handlePublicBookPlay starts a player's go at one puzzle of a shared book.
func (s *Server) handlePublicBookPlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	b, err := s.store.BookByShare(ctx, chiParam(r, "token"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this book is not shared (any more)")
		return
	}
	var in struct {
		PuzzleID int64  `json:"puzzle_id"`
		Player   string `json:"player"`
		Key      string `json:"key"`
	}
	_ = decode(r, &in)
	if !bookHas(b, in.PuzzleID) {
		writeErr(w, http.StatusNotFound, "that puzzle is not in this book")
		return
	}
	pz, err := s.store.GetPuzzle(ctx, b.UserID, in.PuzzleID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	p := &store.Play{UserID: b.UserID, What: "puzzle", RefID: pz.ID, Kind: pz.Kind, Via: "book", Player: in.Player}
	writeState(p, playState{Book: b.ID})
	p, err = s.startPlay(ctx, p, in.Key)
	if err != nil {
		s.fail(w, err, "play")
		return
	}
	t, err := s.targetOf(ctx, p)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, s.playView(ctx, p, t))
}

// ---- word lists ----

type wordlistIn struct {
	Name     *string           `json:"name"`
	Language *string           `json:"language"`
	Words    *[]store.WordClue `json:"words"`
	Text     *string           `json:"text"` // "word - clue" per line
}

// ParseWordLines reads a word per line, with its clue after " - ", ":", a tab or ";".
func parseWordLines(text string) []store.WordClue {
	out := []store.WordClue{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "-*•0123456789.) "))
		if line == "" {
			continue
		}
		word, clue := line, ""
		for _, sep := range []string{"\t", " - ", " – ", " — ", ": ", ";", ","} {
			if a, b, ok := strings.Cut(line, sep); ok {
				word, clue = a, b
				break
			}
		}
		word = strings.TrimSpace(strings.Trim(word, `"'`))
		if word != "" && len(word) <= 40 {
			out = append(out, store.WordClue{Word: word, Clue: clip(strings.TrimSpace(clue), maxLine)})
		}
	}
	return out
}

func cleanWords(in []store.WordClue) []store.WordClue {
	out := []store.WordClue{}
	seen := map[string]bool{}
	for _, w := range in {
		w.Word = clip(strings.TrimSpace(w.Word), 40)
		w.Clue = clip(strings.TrimSpace(w.Clue), maxLine)
		k := strings.ToUpper(w.Word)
		if w.Word == "" || seen[k] || len(out) >= 1000 {
			continue
		}
		seen[k] = true
		out = append(out, w)
	}
	return out
}

func (s *Server) handleListWordLists(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListWordLists(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "word lists")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"wordlists": list})
}

func (s *Server) handleGetWordList(w http.ResponseWriter, r *http.Request) {
	l, err := s.store.GetWordList(r.Context(), actorID(r.Context()), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "word list")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) handleSaveWordList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in wordlistIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	l := &store.WordList{UserID: uid, Language: s.settingsOf(ctx, uid).Language, Source: sourceOf(ctx)}
	status := http.StatusCreated
	if id := idParam(r, "id"); id > 0 {
		got, err := s.store.GetWordList(ctx, uid, id)
		if err != nil {
			s.fail(w, err, "word list")
			return
		}
		if !mayChange(ctx, got.Source) {
			writeErr(w, http.StatusForbidden, "this app may only change lists it added")
			return
		}
		l, status = got, http.StatusOK
	}
	trimP(&l.Name, in.Name)
	l.Name = clip(l.Name, maxName)
	if l.Name == "" {
		l.Name = "Words"
	}
	if in.Language != nil && oneOf(*in.Language, puzzle.Languages) {
		l.Language = *in.Language
	}
	if in.Words != nil {
		l.Words = cleanWords(*in.Words)
	}
	if in.Text != nil {
		l.Words = cleanWords(append(l.Words, parseWordLines(*in.Text)...))
	}
	if err := s.store.SaveWordList(ctx, l); err != nil {
		s.fail(w, err, "save word list")
		return
	}
	got, _ := s.store.GetWordList(ctx, uid, l.ID)
	writeJSON(w, status, got)
}

func (s *Server) handleDeleteWordList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, err := s.store.GetWordList(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "word list")
		return
	}
	if !mayChange(ctx, l.Source) {
		writeErr(w, http.StatusForbidden, "this app may only remove lists it added")
		return
	}
	if err := s.store.DeleteWordList(ctx, l.UserID, l.ID); err != nil {
		s.fail(w, err, "word list")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---- adventures ----

type adventureIn struct {
	Kind        *string           `json:"kind"`
	Title       *string           `json:"title"`
	Intro       *string           `json:"intro"`
	Finale      *string           `json:"finale"`
	Theme       *string           `json:"theme"`
	Date        *string           `json:"date"`
	Time        *string           `json:"time"`
	Place       *string           `json:"place"`
	Players     *string           `json:"players"`
	Minutes     *int              `json:"minutes"`
	HintPenalty *int              `json:"hint_penalty"`
	Status      *string           `json:"status"`
	Notes       *string           `json:"notes"`
	Prep        *[]store.PrepItem `json:"prep"`
	ShareOn     *bool             `json:"share_on"`
	Board       *bool             `json:"board"`
	PhotoID     *int64            `json:"photo_id"`
	Template    string            `json:"template"`
	Renew       bool              `json:"renew"`
}

func (s *Server) applyAdventure(ctx context.Context, uid int64, keys map[string]bool, in adventureIn, a *store.Adventure) error {
	if in.Kind != nil {
		if !oneOf(*in.Kind, store.AdventureKinds) {
			return &userErr{"an adventure is an escape room, a treasure hunt, a mystery or a party game"}
		}
		a.Kind = *in.Kind
	}
	trimP(&a.Title, in.Title)
	a.Title = clip(a.Title, maxName)
	if a.Title == "" {
		return &userErr{"give it a name"}
	}
	for _, f := range []struct {
		dst *string
		v   *string
		n   int
	}{{&a.Intro, in.Intro, 8000}, {&a.Finale, in.Finale, 8000}, {&a.Theme, in.Theme, maxLine}, {&a.Place, in.Place, maxLine}, {&a.Players, in.Players, 2000}, {&a.Notes, in.Notes, maxNotes}} {
		if f.v != nil {
			*f.dst = cleanText(*f.v, f.n)
		}
	}
	if in.Date != nil {
		if !validDate(*in.Date) {
			return &userErr{"the date is YYYY-MM-DD"}
		}
		a.Date = *in.Date
	}
	if in.Time != nil {
		t := strings.TrimSpace(*in.Time)
		if t != "" && !reTime.MatchString(t) {
			return &userErr{"the time is HH:MM"}
		}
		a.Time = t
	}
	if in.Minutes != nil {
		a.Minutes = clampInt(*in.Minutes, 0, 24*60)
	}
	if in.HintPenalty != nil {
		a.HintPenalty = clampInt(*in.HintPenalty, 0, 3600)
	}
	if in.Status != nil {
		if !oneOf(*in.Status, store.AdventureState) {
			return &userErr{"the state is draft, ready or done"}
		}
		a.Status = *in.Status
	}
	if in.Prep != nil {
		prep := []store.PrepItem{}
		for _, it := range *in.Prep {
			it.Text = cleanText(it.Text, maxLine)
			if it.Text == "" {
				continue
			}
			if it.ID == "" {
				it.ID = newToken()[:10]
			}
			prep = append(prep, it)
		}
		a.Prep = prep
	}
	if in.ShareOn != nil {
		a.ShareOn = *in.ShareOn
	}
	if in.Board != nil {
		a.Board = *in.Board
	}
	if keys["photo_id"] {
		if in.PhotoID == nil || *in.PhotoID == 0 {
			a.PhotoID = nil
		} else if !s.store.OwnsPhoto(ctx, uid, *in.PhotoID) {
			return &userErr{"unknown picture"}
		} else {
			a.PhotoID = in.PhotoID
		}
	}
	return nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (s *Server) adventureDoc(ctx context.Context, a *store.Adventure) map[string]any {
	var doc map[string]any
	_ = remarshal(a, &doc)
	doc["share_url"] = s.link("/a/" + a.ShareToken)
	if a.Stages != nil {
		cards := []map[string]any{}
		for _, st := range a.Stages {
			if st.Code != "" {
				cards = append(cards, map[string]any{"stage": st.ID, "url": s.link(fmt.Sprintf("/a/%s?card=%s", a.ShareToken, st.Code))})
			}
		}
		doc["cards"] = cards
	}
	if linkApp(ctx) != "" {
		// another app sees the story, never the answers or where things are hidden
		for _, k := range []string{"notes", "prep"} {
			delete(doc, k)
		}
		if sts, ok := doc["stages"].([]any); ok {
			for _, x := range sts {
				if m, ok := x.(map[string]any); ok {
					for _, k := range []string{"answers", "hidden", "code", "unlock", "hints", "props"} {
						delete(m, k)
					}
				}
			}
		}
	}
	return doc
}

func (s *Server) handleListAdventures(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListAdventures(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "adventures")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"adventures": list})
}

func (s *Server) handleGetAdventure(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, err := s.store.GetAdventure(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "adventure")
		return
	}
	writeJSON(w, http.StatusOK, s.adventureDoc(ctx, a))
}

func (s *Server) handleCreateAdventure(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in adventureIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	a := &store.Adventure{UserID: uid, Kind: "escape", Minutes: 60, HintPenalty: 60, Status: "draft", ShareToken: newToken(), Board: true, Source: sourceOf(ctx), Prep: []store.PrepItem{}}
	tpl, hasTpl := adventureTemplates[in.Template]
	if hasTpl {
		a.Kind, a.Title, a.Intro, a.Finale, a.Minutes, a.Theme = tpl.kind, tpl.title, tpl.intro, tpl.finale, tpl.minutes, tpl.theme
		for _, p := range tpl.prep {
			a.Prep = append(a.Prep, store.PrepItem{ID: newToken()[:10], Text: p})
		}
		if in.Title == nil {
			in.Title = &a.Title
		}
	}
	if err := s.applyAdventure(ctx, uid, keys, in, a); err != nil {
		s.userFail(w, err, "adventure")
		return
	}
	if err := s.store.SaveAdventure(ctx, a); err != nil {
		s.fail(w, err, "create adventure")
		return
	}
	if hasTpl {
		for _, st := range tpl.stages {
			x := st
			x.UserID, x.AdventureID = uid, a.ID
			if x.Lock == "qr" {
				x.Code = newToken()[:12]
			}
			_ = s.store.SaveStage(ctx, &x)
		}
	}
	s.publish(ctx, uid, "puzzlecraft.adventure_created", fmt.Sprintf("adventure:%d", a.ID), map[string]any{"adventure_id": a.ID, "title": a.Title, "kind": a.Kind, "date": a.Date})
	got, _ := s.store.GetAdventure(ctx, uid, a.ID)
	writeJSON(w, http.StatusCreated, s.adventureDoc(ctx, got))
}

func (s *Server) handleUpdateAdventure(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	a, err := s.store.GetAdventure(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "adventure")
		return
	}
	var in adventureIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.applyAdventure(ctx, uid, keys, in, a); err != nil {
		s.userFail(w, err, "adventure")
		return
	}
	if err := s.store.SaveAdventure(ctx, a); err != nil {
		s.fail(w, err, "save adventure")
		return
	}
	if in.Renew {
		_ = s.store.SetShareToken(ctx, "adventures", uid, a.ID, newToken())
	}
	got, _ := s.store.GetAdventure(ctx, uid, a.ID)
	writeJSON(w, http.StatusOK, s.adventureDoc(ctx, got))
}

func (s *Server) handleDeleteAdventure(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteAdventure(r.Context(), actorID(r.Context()), idParam(r, "id")); err != nil {
		s.fail(w, err, "adventure")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type stageIn struct {
	Title     *string       `json:"title"`
	Story     *string       `json:"story"`
	PhotoID   *int64        `json:"photo_id"`
	Lock      *string       `json:"lock"`
	PuzzleID  *int64        `json:"puzzle_id"`
	Answers   *[]string     `json:"answers"`
	LockHint  *string       `json:"lock_hint"`
	Hints     *[]store.Hint `json:"hints"`
	Hidden    *string       `json:"hidden"`
	PlaceName *string       `json:"place_name"`
	Lat       *float64      `json:"lat"`
	Lng       *float64      `json:"lng"`
	PlaceRef  *string       `json:"place_ref"`
	Unlock    *string       `json:"unlock"`
	Props     *string       `json:"props"`
	NewCode   bool          `json:"new_code"`
}

func (s *Server) applyStage(ctx context.Context, uid int64, keys map[string]bool, in stageIn, st *store.Stage) error {
	for _, f := range []struct {
		dst *string
		v   *string
		n   int
	}{{&st.Title, in.Title, maxName}, {&st.Story, in.Story, 8000}, {&st.LockHint, in.LockHint, maxLine}, {&st.Hidden, in.Hidden, 2000}, {&st.PlaceName, in.PlaceName, maxLine},
		{&st.PlaceRef, in.PlaceRef, 200}, {&st.Unlock, in.Unlock, 4000}, {&st.Props, in.Props, 2000}} {
		if f.v != nil {
			*f.dst = cleanText(*f.v, f.n)
		}
	}
	if in.Lock != nil {
		if !oneOf(*in.Lock, store.Locks) {
			return &userErr{"unknown kind of lock"}
		}
		st.Lock = *in.Lock
	}
	if keys["puzzle_id"] {
		if in.PuzzleID == nil || *in.PuzzleID == 0 {
			st.PuzzleID = nil
		} else if _, err := s.store.GetPuzzle(ctx, uid, *in.PuzzleID); err != nil {
			return &userErr{"unknown puzzle"}
		} else {
			st.PuzzleID = in.PuzzleID
		}
	}
	if keys["photo_id"] {
		if in.PhotoID == nil || *in.PhotoID == 0 {
			st.PhotoID = nil
		} else if !s.store.OwnsPhoto(ctx, uid, *in.PhotoID) {
			return &userErr{"unknown picture"}
		} else {
			st.PhotoID = in.PhotoID
		}
	}
	if in.Answers != nil {
		ans := []string{}
		for _, a := range *in.Answers {
			if a = cleanText(a, maxLine); a != "" && len(ans) < 20 {
				ans = append(ans, a)
			}
		}
		st.Answers = ans
	}
	if in.Hints != nil {
		hs := []store.Hint{}
		for _, h := range *in.Hints {
			if h.Text = cleanText(h.Text, 2000); h.Text != "" && len(hs) < 10 {
				h.After = clampInt(h.After, 0, 600)
				hs = append(hs, h)
			}
		}
		st.Hints = hs
	}
	if keys["lat"] || keys["lng"] {
		st.Lat, st.Lng = in.Lat, in.Lng
		if st.Lat != nil && (*st.Lat < -90 || *st.Lat > 90) || st.Lng != nil && (*st.Lng < -180 || *st.Lng > 180) {
			return &userErr{"that is not a place on Earth"}
		}
	}
	if st.Lock == "qr" && (st.Code == "" || in.NewCode) {
		st.Code = newToken()[:12]
	}
	return nil
}

func (s *Server) handleCreateStage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	a, err := s.store.GetAdventure(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "adventure")
		return
	}
	var in stageIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st := &store.Stage{UserID: uid, AdventureID: a.ID, Lock: "answer"}
	if in.Title == nil {
		t := fmt.Sprintf("Stage %d", len(a.Stages)+1)
		in.Title = &t
	}
	if err := s.applyStage(ctx, uid, keys, in, st); err != nil {
		s.userFail(w, err, "stage")
		return
	}
	if err := s.store.SaveStage(ctx, st); err != nil {
		s.fail(w, err, "stage")
		return
	}
	got, _ := s.store.GetStage(ctx, uid, st.ID)
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) handleUpdateStage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st, err := s.store.GetStage(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "stage")
		return
	}
	var in stageIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.applyStage(ctx, uid, keys, in, st); err != nil {
		s.userFail(w, err, "stage")
		return
	}
	if err := s.store.SaveStage(ctx, st); err != nil {
		s.fail(w, err, "stage")
		return
	}
	got, _ := s.store.GetStage(ctx, uid, st.ID)
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) handleDeleteStage(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteStage(r.Context(), actorID(r.Context()), idParam(r, "id")); err != nil {
		s.fail(w, err, "stage")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleReorderStages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.store.ReorderStages(ctx, uid, idParam(r, "id"), in.IDs); err != nil {
		s.fail(w, err, "order")
		return
	}
	a, err := s.store.GetAdventure(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "adventure")
		return
	}
	writeJSON(w, http.StatusOK, s.adventureDoc(ctx, a))
}

// ---- pictures ----

const maxPhoto = 8 << 20

func (s *Server) handleUploadPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, 2*maxPhoto+(1<<20))
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "the picture is too big (8 MB at most)")
		return
	}
	read := func(field string) ([]byte, string, error) {
		f, h, err := r.FormFile(field)
		if err != nil {
			return nil, "", err
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, maxPhoto+1))
		if err != nil {
			return nil, "", err
		}
		if len(b) > maxPhoto {
			return nil, "", fmt.Errorf("too big")
		}
		return b, h.Header.Get("Content-Type"), nil
	}
	data, _, err := read("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "send the picture as “file”")
		return
	}
	p, err := s.savePhoto(ctx, uid, r.FormValue("kind"), data)
	if err != nil {
		s.userFail(w, err, "photo")
		return
	}
	if p.Width == 0 {
		p.Width, _ = strconv.Atoi(r.FormValue("width"))
		p.Height, _ = strconv.Atoi(r.FormValue("height"))
	}
	if t, tm, err := read("thumb"); err == nil && len(t) > 0 {
		if http.DetectContentType(t) == "image/jpeg" || tm == "image/jpeg" {
			_, _ = s.store.DB.ExecContext(ctx, `UPDATE photos SET thumb = ?, width = ?, height = ? WHERE id = ? AND user_id = ?`, t, p.Width, p.Height, p.ID, uid)
		}
	}
	u, t := photoURL(&p.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": p.ID, "url": u, "thumb_url": t, "width": p.Width, "height": p.Height})
}

func (s *Server) savePhoto(ctx context.Context, uid int64, kind string, data []byte) (*store.Photo, error) {
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
		return nil, &userErr{"send a JPEG, PNG or WebP picture"}
	}
	p := &store.Photo{UserID: uid, Kind: kind, Mime: mime, Data: data}
	if !oneOf(p.Kind, store.PhotoKinds) {
		p.Kind = "puzzle"
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		p.Width, p.Height = cfg.Width, cfg.Height
	}
	return p, s.store.CreatePhoto(ctx, p)
}

func (s *Server) handlePhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := s.store.GetPhoto(ctx, actorID(ctx), idParam(r, "id"), r.URL.Query().Get("thumb") == "1")
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	servePhoto(w, r, p)
}

func servePhoto(w http.ResponseWriter, r *http.Request, p *store.Photo) {
	w.Header().Set("Content-Type", p.Mime)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("Content-Length", strconv.Itoa(len(p.Data)))
	_, _ = w.Write(p.Data)
}
