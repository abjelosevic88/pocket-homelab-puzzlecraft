package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// starter is an empty puzzle of a kind to start from.
func starter(kind, lang string) json.RawMessage {
	switch kind {
	case "crossword":
		return mustJSON(&puzzle.Crossword{W: 9, H: 9, Cells: make([]string, 81), Clues: map[string]string{}, Style: "classic", Symmetry: "rotational", Language: lang})
	case "wordsearch":
		m := puzzle.MakeWordSearch(nil, 12, 12, "medium", "", lang, time.Now().UnixNano())
		return mustJSON(m)
	case "sudoku":
		return mustJSON(&puzzle.Sudoku{Size: 9, BoxW: 3, BoxH: 3, Givens: make([]int, 81), Solution: make([]int, 81), Level: "medium"})
	case "nonogram":
		return mustJSON(&puzzle.Nonogram{W: 10, H: 10, Cells: make([]int, 100)})
	case "cryptogram":
		return mustJSON(puzzle.MakeCryptogram("Type the text to hide here.", "", "random", lang, 2, time.Now().UnixNano()))
	case "scramble":
		return mustJSON(puzzle.MakeScramble([]puzzle.WordClue{{Word: "puzzle", Clue: "What you are making"}}, nil, lang, time.Now().UnixNano()))
	case "quiz":
		return mustJSON(&puzzle.Quiz{Mode: "quiz", Questions: []puzzle.Question{{Q: "", Type: "text", Answers: []string{}, Points: 1}}})
	case "maze":
		return mustJSON(puzzle.MakeMaze(14, 14, time.Now().UnixNano()))
	case "logic":
		return mustJSON(&puzzle.Logic{Categories: []puzzle.Category{{Name: "Person", Items: []string{"", "", ""}}, {Name: "", Items: []string{"", "", ""}}},
			Clues: []puzzle.Clue{}, Solution: [][]int{{0, 1, 2}, {0, 1, 2}}})
	case "ladder":
		return mustJSON(&puzzle.Ladder{From: "COLD", To: "WARM", Steps: puzzle.LadderPath("COLD", "WARM"), Language: "en"})
	case "jigsaw":
		return mustJSON(&puzzle.Jigsaw{Cols: 5, Rows: 4, Shape: "classic"})
	}
	return json.RawMessage(`{}`)
}

type puzzleIn struct {
	Kind         *string           `json:"kind"`
	Title        *string           `json:"title"`
	Instructions *string           `json:"instructions"`
	Notes        *string           `json:"notes"`
	Level        *string           `json:"level"`
	Language     *string           `json:"language"`
	Tags         []string          `json:"tags"`
	Data         json.RawMessage   `json:"data"`
	Generate     *puzzle.Options   `json:"generate"`
	PhotoID      *int64            `json:"photo_id"`
	ShareOn      *bool             `json:"share_on"`
	Board        *bool             `json:"board"`
	Reveal       *bool             `json:"reveal"`
	Timer        *bool             `json:"timer"`
	Pinned       *bool             `json:"pinned"`
	Archived     *bool             `json:"archived"`
	SourceRef    *string           `json:"source_ref"`
	Words        []puzzle.WordClue `json:"words"` // shorthand: make it from these words
	// flat shorthands for scripts and Pocket AI (instead of generate):
	WordsText string `json:"words_text"` // "word - clue" per line
	Theme     string `json:"theme"`
	Size      int    `json:"size"`
	Message   string `json:"message"`
	Text      string `json:"text"`
	Author    string `json:"author"`
	Cipher    string `json:"cipher"`
	Picture   string `json:"picture"`
	Mode      string `json:"mode"`
	Count     int    `json:"count"`
	Style     string `json:"style"`
}

// shorthand turns the flat fields into "make one for me" options (nil when none is set).
func (in puzzleIn) shorthand() *puzzle.Options {
	words := in.Words
	for _, w := range parseWordLines(in.WordsText) {
		words = append(words, puzzle.WordClue{Word: w.Word, Clue: w.Clue})
	}
	if len(words) == 0 && in.Theme == "" && in.Size == 0 && in.Message == "" && in.Text == "" && in.Cipher == "" && in.Picture == "" && in.Mode == "" && in.Count == 0 && in.Style == "" {
		return nil
	}
	o := &puzzle.Options{Words: words, Theme: in.Theme, Message: in.Message, Text: in.Text, Author: in.Author, Cipher: in.Cipher, Picture: in.Picture, Mode: in.Mode, Count: in.Count, Style: in.Style}
	if k := ptrStr(in.Kind); k == "sudoku" {
		o.Size = in.Size
	} else if in.Size > 0 {
		o.W, o.H = in.Size, in.Size
	}
	return o
}

// prepare tidies a puzzle's definition and tests it (sets size, ready, problems).
func prepare(p *store.Puzzle) error {
	clean, err := puzzle.Clean(p.Kind, p.Data)
	if err != nil {
		return &userErr{strings.TrimPrefix(err.Error(), puzzle.ErrBad.Error()+": ")}
	}
	p.Data = clean
	p.SizeText = puzzle.Size(p.Kind, p.Data)
	if rep, err := puzzle.Test(p.Kind, p.Data); err == nil {
		p.Ready, p.Problems = rep.OK, len(rep.Problems)
		if rep.Level != "" && p.Kind == "sudoku" {
			p.Level = rep.Level
		}
	}
	now := store.Now()
	p.TestedAt = &now
	return nil
}

func (s *Server) apply(ctx context.Context, uid int64, keys map[string]bool, in puzzleIn, p *store.Puzzle) error {
	trimP(&p.Title, in.Title)
	p.Title = clip(p.Title, maxName)
	if in.Instructions != nil {
		p.Instructions = cleanText(*in.Instructions, 4000)
	}
	if in.Notes != nil {
		p.Notes = cleanText(*in.Notes, maxNotes)
	}
	if in.Level != nil {
		if !oneOf(*in.Level, puzzle.Levels) {
			return &userErr{"the level is easy, medium, hard or expert"}
		}
		p.Level = *in.Level
	}
	if in.Language != nil {
		if !oneOf(*in.Language, puzzle.Languages) {
			return &userErr{"unknown language"}
		}
		p.Language = *in.Language
	}
	if keys["tags"] {
		p.Tags = store.CleanTags(in.Tags)
	}
	if keys["photo_id"] {
		if in.PhotoID == nil || *in.PhotoID == 0 {
			p.PhotoID = nil
		} else if !s.store.OwnsPhoto(ctx, uid, *in.PhotoID) {
			return &userErr{"unknown picture"}
		} else {
			p.PhotoID = in.PhotoID
		}
	}
	setB := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	setB(&p.ShareOn, in.ShareOn)
	setB(&p.Board, in.Board)
	setB(&p.Reveal, in.Reveal)
	setB(&p.Timer, in.Timer)
	setB(&p.Pinned, in.Pinned)
	setB(&p.Archived, in.Archived)
	if in.SourceRef != nil {
		p.SourceRef = cleanText(*in.SourceRef, 200)
	}
	if len(in.Data) > 0 && string(in.Data) != "null" {
		if len(in.Data) > 4<<20 {
			return &userErr{"the puzzle is too big"}
		}
		p.Data = in.Data
	}
	if p.Kind == "jigsaw" {
		var j puzzle.Jigsaw
		_ = json.Unmarshal(p.Data, &j)
		if j.PhotoID != 0 && !s.store.OwnsPhoto(ctx, uid, j.PhotoID) {
			return &userErr{"unknown picture"}
		}
	}
	return prepare(p)
}

// handleCreatePuzzle makes a puzzle: from a definition, from words, or
// generated ("make one for me").
func (s *Server) handleCreatePuzzle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in puzzleIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	kind := ""
	if in.Kind != nil {
		kind = *in.Kind
	}
	if !oneOf(kind, puzzle.Kinds) {
		writeErr(w, http.StatusBadRequest, "the kind is one of: "+strings.Join(puzzle.Kinds, ", "))
		return
	}
	st := s.settingsOf(ctx, uid)
	p := &store.Puzzle{UserID: uid, Kind: kind, Level: st.Level, Language: st.Language, ShareToken: newToken(), Board: st.ShareBoard, Reveal: true, Timer: true,
		Source: sourceOf(ctx), Tags: []string{}}
	if in.Language != nil && oneOf(*in.Language, puzzle.Languages) {
		p.Language = *in.Language
	}
	if in.SourceRef != nil && p.Source != "" {
		if old, err := s.store.PuzzleBySourceRef(ctx, uid, p.Source, *in.SourceRef); err == nil {
			s.writePuzzle(w, r, old, http.StatusOK)
			return
		}
	}
	msg := ""
	var missing []string
	if len(in.Data) == 0 || string(in.Data) == "null" {
		var o *puzzle.Options
		switch {
		case in.Generate != nil:
			o = in.Generate
		default:
			o = in.shorthand()
		}
		if o != nil {
			if o.Language == "" {
				o.Language = p.Language
			}
			if o.Level == "" {
				o.Level = firstNonEmpty(ptrStr(in.Level), p.Level)
			}
			m, err := puzzle.Generate(kind, *o)
			if err != nil {
				writeErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), puzzle.ErrBad.Error()+": "))
				return
			}
			in.Data, msg, missing = m.Data, m.Message, m.Missing
			if in.Title == nil || strings.TrimSpace(*in.Title) == "" {
				in.Title = &m.Title
			}
			if m.Level != "" && in.Level == nil {
				in.Level = &m.Level
			}
		} else {
			in.Data = starter(kind, p.Language)
		}
		keys["data"] = true
	}
	if in.Title == nil || strings.TrimSpace(*in.Title) == "" {
		t := puzzle.KindName[kind]
		in.Title = &t
	}
	if err := s.apply(ctx, uid, keys, in, p); err != nil {
		s.userFail(w, err, "puzzle")
		return
	}
	if err := s.store.CreatePuzzle(ctx, p); err != nil {
		s.fail(w, err, "create puzzle")
		return
	}
	s.publish(ctx, uid, "puzzlecraft.puzzle_created", fmt.Sprintf("puzzle:%d", p.ID), map[string]any{"puzzle_id": p.ID, "kind": p.Kind, "title": p.Title})
	got, _ := s.store.GetPuzzle(ctx, uid, p.ID)
	if got == nil {
		got = p
	}
	out := s.puzzleDoc(ctx, r, got)
	if msg != "" {
		out["message"] = msg
	}
	if len(missing) > 0 {
		out["missing"] = missing
	}
	writeJSON(w, http.StatusCreated, out)
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// puzzleDoc is a puzzle for its owner (or, for another app, without the answer).
func (s *Server) puzzleDoc(ctx context.Context, r *http.Request, p *store.Puzzle) map[string]any {
	var doc map[string]any
	_ = remarshal(p, &doc)
	if linkApp(ctx) != "" {
		view, _ := puzzle.Public(p.Kind, p.Data)
		doc["data"] = view
		doc["notes"] = ""
	}
	doc["kind_name"] = puzzle.KindName[p.Kind]
	doc["share_url"] = s.link("/p/" + p.ShareToken)
	doc["url"] = s.link(fmt.Sprintf("/puzzles/%d", p.ID))
	doc["uses"] = s.store.PuzzleUses(ctx, p.UserID, p.ID)
	if mine, err := s.store.MyPlay(ctx, p.UserID, "puzzle", p.ID, ""); err == nil {
		doc["mine"] = map[string]any{"finished_at": mine.FinishedAt, "ms": mine.Ms, "hints": mine.Hints, "started_at": mine.StartedAt}
	}
	if p.Kind == "crossword" && linkApp(ctx) == "" {
		var c puzzle.Crossword
		_ = json.Unmarshal(p.Data, &c)
		doc["entries"] = puzzle.Entries(&c)
	}
	return doc
}

func (s *Server) writePuzzle(w http.ResponseWriter, r *http.Request, p *store.Puzzle, status int) {
	writeJSON(w, status, s.puzzleDoc(r.Context(), r, p))
}

func (s *Server) handleListPuzzles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := r.URL.Query()
	list, err := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{Kind: q.Get("kind"), Q: q.Get("q"), Tag: q.Get("tag"), Archived: queryBool(r, "archived"),
		Shared: queryBool(r, "shared"), Limit: queryInt(r, "limit")})
	if err != nil {
		s.fail(w, err, "puzzles")
		return
	}
	kinds, _ := s.store.KindCounts(ctx, uid)
	tags, _ := s.store.PuzzleTags(ctx, uid)
	writeJSON(w, http.StatusOK, map[string]any{"puzzles": list, "kinds": kinds, "tags": tags})
}

func (s *Server) handleGetPuzzle(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetPuzzle(r.Context(), actorID(r.Context()), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	s.writePuzzle(w, r, p, http.StatusOK)
}

func (s *Server) handleUpdatePuzzle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPuzzle(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	if !mayChange(ctx, p.Source) {
		writeErr(w, http.StatusForbidden, "this app may only change puzzles it added")
		return
	}
	var in puzzleIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Kind != nil && *in.Kind != p.Kind {
		writeErr(w, http.StatusBadRequest, "a puzzle keeps its kind; make a new one instead")
		return
	}
	if in.Generate != nil {
		o := *in.Generate
		if o.Language == "" {
			o.Language = p.Language
		}
		if o.Level == "" {
			o.Level = p.Level
		}
		m, err := puzzle.Generate(p.Kind, o)
		if err != nil {
			writeErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), puzzle.ErrBad.Error()+": "))
			return
		}
		in.Data = m.Data
	}
	if err := s.apply(ctx, uid, keys, in, p); err != nil {
		s.userFail(w, err, "puzzle")
		return
	}
	if err := s.store.UpdatePuzzle(ctx, p); err != nil {
		s.fail(w, err, "save puzzle")
		return
	}
	got, _ := s.store.GetPuzzle(ctx, uid, p.ID)
	s.writePuzzle(w, r, got, http.StatusOK)
}

func (s *Server) handleDeletePuzzle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPuzzle(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	if !mayChange(ctx, p.Source) {
		writeErr(w, http.StatusForbidden, "this app may only remove puzzles it added")
		return
	}
	if err := s.store.DeletePuzzle(ctx, uid, p.ID); err != nil {
		s.fail(w, err, "delete puzzle")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDuplicatePuzzle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPuzzle(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	cp := *p
	cp.ID, cp.Title, cp.ShareToken, cp.ShareOn, cp.Source, cp.SourceRef, cp.Pinned = 0, clip(p.Title+" (copy)", maxName), newToken(), false, "", "", false
	if err := s.store.CreatePuzzle(ctx, &cp); err != nil {
		s.fail(w, err, "copy")
		return
	}
	got, _ := s.store.GetPuzzle(ctx, uid, cp.ID)
	s.writePuzzle(w, r, got, http.StatusCreated)
}

// handleSharePuzzle switches the link on or off (or gives it a new address).
func (s *Server) handleSharePuzzle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPuzzle(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	var in struct {
		On     *bool `json:"on"`
		Board  *bool `json:"board"`
		Reveal *bool `json:"reveal"`
		Timer  *bool `json:"timer"`
		Renew  bool  `json:"renew"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.On != nil {
		p.ShareOn = *in.On
	}
	for _, x := range []struct {
		dst *bool
		v   *bool
	}{{&p.Board, in.Board}, {&p.Reveal, in.Reveal}, {&p.Timer, in.Timer}} {
		if x.v != nil {
			*x.dst = *x.v
		}
	}
	if in.Renew {
		p.ShareToken = newToken()
		_ = s.store.SetShareToken(ctx, "puzzles", uid, p.ID, p.ShareToken)
	}
	if err := s.store.UpdatePuzzle(ctx, p); err != nil {
		s.fail(w, err, "share")
		return
	}
	if p.ShareOn {
		s.publish(ctx, uid, "puzzlecraft.puzzle_shared", fmt.Sprintf("puzzle:%d", p.ID), map[string]any{"puzzle_id": p.ID, "title": p.Title, "url": s.link("/p/" + p.ShareToken)})
	}
	got, _ := s.store.GetPuzzle(ctx, uid, p.ID)
	s.writePuzzle(w, r, got, http.StatusOK)
}

// handleTestPuzzle says whether a puzzle is fair: one answer, nothing missing.
func (s *Server) handleTestPuzzle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPuzzle(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	data := p.Data
	var in struct {
		Data json.RawMessage `json:"data"`
	}
	_ = decode(r, &in)
	if len(in.Data) > 0 && string(in.Data) != "null" {
		if clean, err := puzzle.Clean(p.Kind, in.Data); err == nil {
			data = clean
		}
	}
	rep, err := puzzle.Test(p.Kind, data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if string(data) == string(p.Data) {
		now := store.Now()
		p.Ready, p.Problems, p.TestedAt = rep.OK, len(rep.Problems), &now
		_ = s.store.UpdatePuzzle(ctx, p)
	}
	writeJSON(w, http.StatusOK, rep)
}

// handleTestData tests a definition that is not saved yet.
func (s *Server) handleTestData(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind string          `json:"kind"`
		Data json.RawMessage `json:"data"`
	}
	if err := decode(r, &in); err != nil || !oneOf(in.Kind, puzzle.Kinds) {
		writeErr(w, http.StatusBadRequest, "send a kind and its data")
		return
	}
	clean, err := puzzle.Clean(in.Kind, in.Data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), puzzle.ErrBad.Error()+": "))
		return
	}
	rep, _ := puzzle.Test(in.Kind, clean)
	writeJSON(w, http.StatusOK, map[string]any{"report": rep, "data": clean, "size_text": puzzle.Size(in.Kind, clean)})
}

// handleGenerate makes a puzzle without saving it (the editor shows it first).
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind string `json:"kind"`
		puzzle.Options
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !oneOf(in.Kind, puzzle.Kinds) {
		writeErr(w, http.StatusBadRequest, "unknown kind")
		return
	}
	if in.Language == "" {
		in.Language = s.settingsOf(r.Context(), actorID(r.Context())).Language
	}
	m, err := puzzle.Generate(in.Kind, in.Options)
	if err != nil {
		writeErr(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), puzzle.ErrBad.Error()+": "))
		return
	}
	clean, err := puzzle.Clean(in.Kind, m.Data)
	if err == nil {
		m.Data = clean
	}
	rep, _ := puzzle.Test(in.Kind, m.Data)
	writeJSON(w, http.StatusOK, map[string]any{"made": m, "report": rep, "size_text": puzzle.Size(in.Kind, m.Data)})
}

// handleWords finds English words that fit a pattern (the crossword fill helper).
func (s *Server) handleWords(w http.ResponseWriter, r *http.Request) {
	pat := r.URL.Query().Get("pattern")
	n := queryInt(r, "limit")
	if n <= 0 || n > 200 {
		n = 60
	}
	common := puzzle.Match(pat, n, true)
	more := []string{}
	if len(common) < n {
		for _, w := range puzzle.Match(pat, n*3, false) {
			if !puzzle.IsCommon(w) && len(more) < n-len(common) {
				more = append(more, w)
			}
		}
	}
	// the person's own word lists first
	ctx := r.Context()
	mine := []map[string]any{}
	if lists, err := s.store.ListWordLists(ctx, actorID(ctx)); err == nil && pat != "" {
		p := []rune(strings.ToUpper(pat))
		for _, l := range lists {
			for _, wc := range l.Words {
				lt := puzzle.Letters(wc.Word, l.Language)
				if len(lt) != len(p) {
					continue
				}
				ok := true
				for i, x := range lt {
					if c := p[i]; c != '?' && c != '.' && string(c) != x {
						ok = false
					}
				}
				if ok && len(mine) < 30 {
					mine = append(mine, map[string]any{"word": strings.Join(lt, ""), "clue": wc.Clue, "list": l.Name})
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"common": common, "more": more, "mine": mine})
}

// handleCrosswordFill fills a classic grid's empty squares with words.
func (s *Server) handleCrosswordFill(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Data  json.RawMessage `json:"data"`
		Words []string        `json:"words"`
		Seed  int64           `json:"seed"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	var c puzzle.Crossword
	if err := json.Unmarshal(in.Data, &c); err != nil || c.W*c.H != len(c.Cells) {
		writeErr(w, http.StatusBadRequest, "send the crossword")
		return
	}
	if in.Seed == 0 {
		in.Seed = time.Now().UnixNano()
	}
	filled, ok := puzzle.Fill(&c, in.Words, in.Seed, 8*time.Second)
	msg := "Filled"
	if !ok {
		msg = "Could not fill every word — move some black squares, or fill a few words yourself, then try again"
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": ok, "data": filled, "message": msg})
}

// handleCrosswordGrid makes an empty classic grid with black squares.
func (s *Server) handleCrosswordGrid(w http.ResponseWriter, r *http.Request) {
	var in struct {
		W, H int
		Seed int64
	}
	_ = decode(r, &in)
	if in.W < 3 || in.W > 25 {
		in.W = 9
	}
	if in.H < 3 || in.H > 25 {
		in.H = in.W
	}
	if in.Seed == 0 {
		in.Seed = time.Now().UnixNano()
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": puzzle.BlankGrid(in.W, in.H, in.Seed)})
}

// handleThemes lists what "make one for me" can start from.
func (s *Server) handleThemes(w http.ResponseWriter, r *http.Request) {
	themes := []map[string]any{}
	for _, t := range puzzle.Themes {
		themes = append(themes, map[string]any{"key": t.Key, "name": t.Name, "language": t.Language, "count": len(t.Words), "words": t.Words})
	}
	sets := []map[string]any{}
	for _, l := range puzzle.LogicSets {
		names := []string{}
		for _, c := range l.Cats {
			names = append(names, c.Name)
		}
		sets = append(sets, map[string]any{"key": l.Key, "name": l.Name, "categories": names})
	}
	pics := map[string][]string{}
	for _, k := range puzzle.PictureNames() {
		pics[k] = puzzle.Pictures[k]
	}
	writeJSON(w, http.StatusOK, map[string]any{"themes": themes, "logic_sets": sets, "pictures": pics, "quotes": puzzle.Quotes, "riddles": len(puzzle.Riddles)})
}

// handleKinds lists the kinds of puzzle.
func (s *Server) handleKinds(w http.ResponseWriter, r *http.Request) {
	out := []map[string]any{}
	for _, k := range puzzle.Kinds {
		out = append(out, map[string]any{"kind": k, "name": puzzle.KindName[k]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"kinds": out, "levels": puzzle.Levels, "languages": puzzle.Languages})
}

// handlePuzzlePlays is how a puzzle was played: every go, best first.
func (s *Server) handlePuzzlePlays(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	what := "puzzle"
	if strings.Contains(r.URL.Path, "/adventures/") {
		what = "adventure"
	}
	id := idParam(r, "id")
	var err error
	if what == "puzzle" {
		_, err = s.store.GetPuzzle(ctx, uid, id)
	} else {
		_, err = s.store.GetAdventure(ctx, uid, id)
	}
	if err != nil {
		s.fail(w, err, what)
		return
	}
	list, err := s.store.ListPlays(ctx, uid, store.PlayFilter{What: what, RefID: id, Limit: 500})
	if err != nil {
		s.fail(w, err, "plays")
		return
	}
	done, totalMs := 0, int64(0)
	for _, p := range list {
		if p.FinishedAt != nil {
			done++
			totalMs += p.Ms
		}
	}
	avg := int64(0)
	if done > 0 {
		avg = totalMs / int64(done)
	}
	writeJSON(w, http.StatusOK, map[string]any{"plays": list, "best": s.board(ctx, uid, what, id, 20), "finished": done, "started": len(list), "avg_ms": avg})
}

func (s *Server) handleDeletePlay(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeletePlay(r.Context(), actorID(r.Context()), idParam(r, "id")); err != nil {
		s.fail(w, err, "play")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
