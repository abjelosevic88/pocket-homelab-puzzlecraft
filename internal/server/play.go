package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// ---- playing ----
//
// A go at anything is a row in plays, found by a random key the player's
// browser keeps (that key is all a player through a share link has). The
// answer never leaves the server: the browser sends what the player filled
// in, the server says what is right (puzzle.Check) and gives away single
// squares on request (puzzle.Reveal). The same endpoints serve you in the
// app, a player through a link, a puzzle book and an escape-room stage.

// playState is what the server keeps about a go; V is the player's own
// progress (letters filled in so far) which the browser saves.
type playState struct {
	Level string          `json:"level,omitempty"` // daily: the level it was made at
	Book  int64           `json:"book,omitempty"`  // played from this book
	Shown []int           `json:"shown,omitempty"` // parts given away
	Hints map[string]int  `json:"hints,omitempty"` // adventure: hints shown per stage id
	Log   []stageLog      `json:"log,omitempty"`   // adventure: each stage's time
	Team  string          `json:"team,omitempty"`
	Over  bool            `json:"over,omitempty"` // adventure: past the time limit
	V     json.RawMessage `json:"v,omitempty"`
}

type stageLog struct {
	Stage int64 `json:"stage"`
	Ms    int64 `json:"ms"`
	Hints int   `json:"hints"`
}

func readState(p *store.Play) playState {
	var st playState
	_ = json.Unmarshal(p.State, &st)
	if st.Hints == nil {
		st.Hints = map[string]int{}
	}
	return st
}

func writeState(p *store.Play, st playState) { p.State = mustJSON(st) }

// target is what a go is at: the puzzle definition, who owns it, whether
// it is still open to this player.
type target struct {
	kind   string
	data   json.RawMessage
	puzzle *store.Puzzle // nil for a daily puzzle
	adv    *store.Adventure
	stage  *store.Stage // the adventure's current stage
	reveal bool
	title  string
}

// targetOf loads what a go is at, checking a link player may still play it.
func (s *Server) targetOf(ctx context.Context, p *store.Play) (*target, error) {
	switch p.What {
	case "daily":
		st := readState(p)
		m, err := puzzle.Daily(p.Kind, dayOf(p), st.Level)
		if err != nil {
			return nil, err
		}
		return &target{kind: p.Kind, data: m.Data, reveal: true, title: m.Title}, nil
	case "puzzle":
		pz, err := s.store.GetPuzzle(ctx, p.UserID, p.RefID)
		if err != nil {
			return nil, err
		}
		switch p.Via {
		case "link":
			if !pz.ShareOn || pz.Archived {
				return nil, store.ErrNotFound
			}
		case "book":
			b, err := s.store.GetBook(ctx, p.UserID, readState(p).Book)
			if err != nil || !b.ShareOn || !bookHas(b, pz.ID) {
				return nil, store.ErrNotFound
			}
		}
		return &target{kind: pz.Kind, data: pz.Data, puzzle: pz, reveal: pz.Reveal || p.Via == "me", title: pz.Title}, nil
	case "adventure":
		a, err := s.store.GetAdventure(ctx, p.UserID, p.RefID)
		if err != nil {
			return nil, err
		}
		if p.Via == "link" && !a.ShareOn {
			return nil, store.ErrNotFound
		}
		t := &target{adv: a, title: a.Title, reveal: false}
		if p.Stage < len(a.Stages) {
			st := a.Stages[p.Stage]
			t.stage = &st
			if st.Lock == "puzzle" && st.PuzzleID != nil {
				if pz, err := s.store.GetPuzzle(ctx, p.UserID, *st.PuzzleID); err == nil {
					t.kind, t.data, t.puzzle = pz.Kind, pz.Data, pz
				}
			}
		}
		return t, nil
	}
	return nil, store.ErrNotFound
}

func bookHas(b *store.Book, pid int64) bool {
	for _, it := range b.Items {
		if it.PuzzleID == pid {
			return true
		}
	}
	return false
}

// publicPuzzle is a puzzle as a player sees it.
func (s *Server) publicPuzzle(pz *store.Puzzle) map[string]any {
	view, err := puzzle.Public(pz.Kind, pz.Data)
	if err != nil {
		view = map[string]any{}
	}
	return map[string]any{"id": pz.ID, "kind": pz.Kind, "kind_name": puzzle.KindName[pz.Kind], "title": pz.Title, "instructions": pz.Instructions,
		"level": pz.Level, "language": pz.Language, "size_text": pz.SizeText, "timer": pz.Timer, "reveal": pz.Reveal, "board": pz.Board, "data": view,
		"photo": pz.PhotoID != nil}
}

// playView is a go as the player's browser gets it.
func (s *Server) playView(ctx context.Context, p *store.Play, t *target) map[string]any {
	st := readState(p)
	out := map[string]any{"key": p.Key, "what": p.What, "kind": t.kind, "player": p.Player, "started_at": p.StartedAt, "finished_at": p.FinishedAt,
		"ms": p.Ms, "hints": p.Hints, "mistakes": p.Mistakes, "score": p.Score, "max_score": p.MaxScore, "progress": st.V, "shown": st.Shown, "title": t.title,
		"server_now": store.Now(), "via": p.Via}
	if out["progress"] == nil || len(st.V) == 0 {
		out["progress"] = nil
	}
	switch {
	case t.adv != nil:
		out["adventure"] = s.adventureView(ctx, p, t)
	case t.puzzle != nil:
		out["puzzle"] = s.publicPuzzle(t.puzzle)
	default:
		view, _ := puzzle.Public(t.kind, t.data)
		out["puzzle"] = map[string]any{"kind": t.kind, "kind_name": puzzle.KindName[t.kind], "title": t.title, "level": st.Level, "timer": true, "reveal": true,
			"data": view, "size_text": puzzle.Size(t.kind, t.data), "daily": dayOf(p)}
	}
	return out
}

func newKey() string { return newToken() + newToken()[:8] }

// startPlay starts (or carries on with) a go.
func (s *Server) startPlay(ctx context.Context, p *store.Play, resume string) (*store.Play, error) {
	if resume != "" {
		if old, err := s.store.PlayByKey(ctx, resume); err == nil && old.UserID == p.UserID && old.What == p.What && old.RefID == p.RefID && old.Day == p.Day && old.Via == p.Via {
			if strings.TrimSpace(p.Player) != "" && p.Player != old.Player {
				old.Player = p.Player
				_ = s.store.UpdatePlay(ctx, old)
			}
			return old, nil
		}
	}
	p.Key = newKey()
	p.Player = cleanText(p.Player, 60)
	if err := s.store.CreatePlay(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Server) playByKey(w http.ResponseWriter, r *http.Request) (*store.Play, *target, bool) {
	p, err := s.store.PlayByKey(r.Context(), chiParam(r, "key"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this game is not open (any more)")
		return nil, nil, false
	}
	t, err := s.targetOf(r.Context(), p)
	if err != nil {
		writeErr(w, http.StatusNotFound, "this puzzle is not shared (any more)")
		return nil, nil, false
	}
	return p, t, true
}

// handlePlayState is where a go stands.
func (s *Server) handlePlayState(w http.ResponseWriter, r *http.Request) {
	p, t, ok := s.playByKey(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.playView(r.Context(), p, t))
}

// handlePlaySave keeps the player's progress (so a reload carries on).
func (s *Server) handlePlaySave(w http.ResponseWriter, r *http.Request) {
	p, _, ok := s.playByKey(w, r)
	if !ok {
		return
	}
	var in struct {
		Progress json.RawMessage `json:"progress"`
		Player   *string         `json:"player"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(in.Progress) > 256<<10 {
		writeErr(w, http.StatusBadRequest, "too much to save")
		return
	}
	st := readState(p)
	if len(in.Progress) > 0 && json.Valid(in.Progress) {
		st.V = in.Progress
	}
	if in.Player != nil {
		p.Player = cleanText(*in.Player, 60)
	}
	writeState(p, st)
	if err := s.store.UpdatePlay(r.Context(), p); err != nil {
		s.fail(w, err, "save")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// elapsed is how long a go took: the browser's own count (which leaves
// out time the page was hidden) unless it is more than the clock says.
func elapsed(p *store.Play, browserMs int64) int64 {
	start, err := time.Parse(time.RFC3339Nano, p.StartedAt)
	wall := int64(0)
	if err == nil {
		wall = time.Since(start).Milliseconds()
	}
	if browserMs > 0 && (wall == 0 || browserMs <= wall+5000) {
		return browserMs
	}
	return wall
}

// handlePlayCheck marks what the player filled in.
func (s *Server) handlePlayCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, t, ok := s.playByKey(w, r)
	if !ok {
		return
	}
	var in struct {
		Answer   json.RawMessage `json:"answer"`
		Final    bool            `json:"final"`
		Ms       int64           `json:"ms"`
		Progress json.RawMessage `json:"progress"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if t.kind == "" || len(t.data) == 0 {
		writeErr(w, http.StatusBadRequest, "there is no puzzle to check here")
		return
	}
	res, err := puzzle.Check(t.kind, t.data, in.Answer, in.Final)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	st := readState(p)
	if len(in.Progress) > 0 && json.Valid(in.Progress) && len(in.Progress) <= 256<<10 {
		st.V = in.Progress
	}
	if res.Found == nil {
		p.Mistakes += len(res.Wrong)
	}
	if res.Max > 0 {
		p.Score, p.MaxScore = res.Score, res.Max
	}
	out := map[string]any{"result": res}
	if t.adv != nil {
		// a puzzle stage of an escape room: solving it opens the next stage
		writeState(p, st)
		if res.Solved {
			s.passStage(ctx, w, p, t, in.Ms, out)
			return
		}
		_ = s.store.UpdatePlay(ctx, p)
		writeJSON(w, http.StatusOK, out)
		return
	}
	if res.Solved && p.FinishedAt == nil {
		now := store.Now()
		p.FinishedAt = &now
		p.Ms = elapsed(p, in.Ms)
		writeState(p, st)
		_ = s.store.UpdatePlay(ctx, p)
		s.solved(ctx, p, t)
	} else {
		writeState(p, st)
		_ = s.store.UpdatePlay(ctx, p)
	}
	out["finished_at"], out["ms"], out["hints"], out["mistakes"] = p.FinishedAt, p.Ms, p.Hints, p.Mistakes
	writeJSON(w, http.StatusOK, out)
}

// handlePlayReveal gives away one part (a letter, a word, the answer) and
// counts it as a hint.
func (s *Server) handlePlayReveal(w http.ResponseWriter, r *http.Request) {
	p, t, ok := s.playByKey(w, r)
	if !ok {
		return
	}
	if !t.reveal {
		writeErr(w, http.StatusForbidden, "this puzzle gives nothing away")
		return
	}
	var in struct {
		What int `json:"what"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	v, err := puzzle.Reveal(t.kind, t.data, in.What)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	st := readState(p)
	st.Shown = append(st.Shown, in.What)
	p.Hints++
	writeState(p, st)
	_ = s.store.UpdatePlay(r.Context(), p)
	writeJSON(w, http.StatusOK, map[string]any{"reveal": v, "hints": p.Hints})
}

// solved runs when a go finishes: the results board, the habit of the
// day, a message to the owner when someone else finished their puzzle.
func (s *Server) solved(ctx context.Context, p *store.Play, t *target) {
	ctx = context.WithoutCancel(ctx)
	title := t.title
	payload := map[string]any{"what": p.What, "kind": t.kind, "title": title, "ms": p.Ms, "hints": p.Hints, "mistakes": p.Mistakes, "player": p.Player, "via": p.Via}
	switch p.What {
	case "daily":
		payload["day"] = dayOf(p)
		s.publish(ctx, p.UserID, "puzzlecraft.daily_solved", "daily:"+dayOf(p)+":"+p.Kind, payload)
		go s.tickHabit(context.WithoutCancel(ctx), p.UserID)
	case "puzzle":
		payload["puzzle_id"] = p.RefID
		s.publish(ctx, p.UserID, "puzzlecraft.puzzle_solved", fmt.Sprintf("puzzle:%d", p.RefID), payload)
	case "adventure":
		payload["adventure_id"] = p.RefID
		s.publish(ctx, p.UserID, "puzzlecraft.adventure_finished", fmt.Sprintf("adventure:%d", p.RefID), payload)
	}
	if p.Via == "me" {
		return
	}
	who := firstNonEmpty(p.Player, "Someone")
	what := fmt.Sprintf("%s solved “%s” in %s", who, title, duration(p.Ms))
	if p.What == "adventure" {
		what = fmt.Sprintf("%s finished “%s” in %s", who, title, duration(p.Ms))
	}
	if p.Hints > 0 {
		what += fmt.Sprintf(" with %d %s", p.Hints, plural(p.Hints, "hint", "hints"))
	}
	link := s.link(fmt.Sprintf("/puzzles/%d?tab=results", p.RefID))
	if p.What == "adventure" {
		link = s.link(fmt.Sprintf("/adventures/%d?tab=live", p.RefID))
	}
	st := s.settingsOf(ctx, p.UserID)
	if st.NotifyPlays && s.once(ctx, fmt.Sprintf("solved:%d", p.ID)) {
		s.push(ctx, p.UserID, st, what, "Pocket Puzzlecraft", link)
	}
	if st.AlertPlays {
		go s.alert(context.WithoutCancel(ctx), p.UserID, what, link)
	}
}

// ---- starting a go ----

// handlePuzzlePlay starts your own go at one of your puzzles.
func (s *Server) handlePuzzlePlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	pz, err := s.store.GetPuzzle(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	var in struct {
		Restart bool `json:"restart"`
	}
	_ = decode(r, &in)
	resume := ""
	if !in.Restart {
		if old, err := s.store.MyPlay(ctx, uid, "puzzle", pz.ID, ""); err == nil {
			resume = old.Key
		}
	}
	p, err := s.startPlay(ctx, &store.Play{UserID: uid, What: "puzzle", RefID: pz.ID, Kind: pz.Kind, Via: "me", Player: userFrom(ctx).Name}, resume)
	if err != nil {
		s.fail(w, err, "play")
		return
	}
	t, _ := s.targetOf(ctx, p)
	writeJSON(w, http.StatusOK, s.playView(ctx, p, t))
}

// handleDailyPlay starts (or carries on with) today's puzzle of a kind.
func (s *Server) handleDailyPlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	day, kind := chiParam(r, "date"), chiParam(r, "kind")
	if !validDate(day) || day == "" || day > today() {
		writeErr(w, http.StatusBadRequest, "no puzzles for that day yet")
		return
	}
	if !oneOf(kind, puzzle.DailyKinds(day)) {
		writeErr(w, http.StatusNotFound, "not a puzzle of that day")
		return
	}
	resume := ""
	if old, err := s.store.MyPlay(ctx, uid, "daily", 0, day+"|"+kind); err == nil {
		resume = old.Key
	}
	st := s.settingsOf(ctx, uid)
	p := &store.Play{UserID: uid, What: "daily", Kind: kind, Day: day + "|" + kind, Via: "me", Player: userFrom(ctx).Name}
	if resume == "" {
		writeState(p, playState{Level: st.DailyLevel})
	}
	p, err := s.startPlay(ctx, p, resume)
	if err != nil {
		s.fail(w, err, "play")
		return
	}
	t, err := s.targetOf(ctx, p)
	if err != nil {
		s.fail(w, err, "daily")
		return
	}
	writeJSON(w, http.StatusOK, s.playView(ctx, p, t))
}

// handlePublicPuzzle is a shared puzzle's first page: what it is, the best times.
func (s *Server) handlePublicPuzzle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pz, err := s.store.PuzzleByShare(ctx, chiParam(r, "token"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this puzzle is not shared (any more)")
		return
	}
	out := s.publicPuzzle(pz)
	delete(out, "data")
	out["from"] = s.ownerName(ctx, pz.UserID)
	if pz.Board {
		out["best"] = s.board(ctx, pz.UserID, "puzzle", pz.ID, 10)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) ownerName(ctx context.Context, uid int64) string {
	if u, err := s.store.GetUser(ctx, uid); err == nil {
		return firstNonEmpty(strings.Fields(u.Name + " ")[0], "")
	}
	return ""
}

// handlePublicPuzzlePlay starts a link player's go.
func (s *Server) handlePublicPuzzlePlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pz, err := s.store.PuzzleByShare(ctx, chiParam(r, "token"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this puzzle is not shared (any more)")
		return
	}
	var in struct {
		Player string `json:"player"`
		Key    string `json:"key"`
	}
	_ = decode(r, &in)
	p, err := s.startPlay(ctx, &store.Play{UserID: pz.UserID, What: "puzzle", RefID: pz.ID, Kind: pz.Kind, Via: "link", Player: in.Player}, in.Key)
	if err != nil {
		s.fail(w, err, "play")
		return
	}
	t, _ := s.targetOf(ctx, p)
	writeJSON(w, http.StatusOK, s.playView(ctx, p, t))
}

// board is the best finished goes: fastest first, fewer hints breaking ties.
func (s *Server) board(ctx context.Context, uid int64, what string, ref int64, n int) []map[string]any {
	done := true
	list, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{What: what, RefID: ref, Finished: &done})
	sort.SliceStable(list, func(a, b int) bool {
		sa, sb := list[a].Ms+int64(list[a].Hints)*30000, list[b].Ms+int64(list[b].Hints)*30000
		if what == "adventure" {
			return list[a].Ms < list[b].Ms
		}
		if list[a].MaxScore > 0 && list[a].Score != list[b].Score {
			return list[a].Score > list[b].Score
		}
		return sa < sb
	})
	out := []map[string]any{}
	for _, p := range list {
		if len(out) >= n {
			break
		}
		name := firstNonEmpty(p.Player, "Someone")
		out = append(out, map[string]any{"player": name, "ms": p.Ms, "hints": p.Hints, "mistakes": p.Mistakes, "score": p.Score, "max_score": p.MaxScore,
			"finished_at": p.FinishedAt, "time": duration(p.Ms)})
	}
	return out
}

// handlePublicPhoto serves a picture a shared puzzle or stage uses.
func (s *Server) handlePublicPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := idParam(r, "id")
	var uid int64
	allowed := false
	if key := chiParam(r, "key"); key != "" {
		p, err := s.store.PlayByKey(ctx, key)
		if err == nil {
			if t, err := s.targetOf(ctx, p); err == nil {
				uid = p.UserID
				allowed = usesPhoto(t, id)
			}
		}
	} else if pz, err := s.store.PuzzleByShare(ctx, chiParam(r, "token")); err == nil {
		uid = pz.UserID
		allowed = pz.PhotoID != nil && *pz.PhotoID == id || usesPhoto(&target{kind: pz.Kind, data: pz.Data, puzzle: pz}, id)
	} else if a, err := s.store.AdventureByShare(ctx, chiParam(r, "token")); err == nil {
		uid = a.UserID
		allowed = a.PhotoID != nil && *a.PhotoID == id
	}
	if !allowed {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	ph, err := s.store.GetPhoto(ctx, uid, id, r.URL.Query().Get("thumb") == "1")
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	servePhoto(w, r, ph)
}

func usesPhoto(t *target, id int64) bool {
	if t.puzzle != nil && t.puzzle.PhotoID != nil && *t.puzzle.PhotoID == id {
		return true
	}
	if t.stage != nil && t.stage.PhotoID != nil && *t.stage.PhotoID == id {
		return true
	}
	if t.adv != nil && t.adv.PhotoID != nil && *t.adv.PhotoID == id {
		return true
	}
	switch t.kind {
	case "jigsaw":
		var j puzzle.Jigsaw
		_ = json.Unmarshal(t.data, &j)
		return j.PhotoID == id
	case "quiz":
		var q puzzle.Quiz
		_ = json.Unmarshal(t.data, &q)
		for _, x := range q.Questions {
			if x.PhotoID == id {
				return true
			}
		}
	}
	return false
}

// dayOf is a daily go's date (it is kept as "date|kind").
func dayOf(p *store.Play) string { return strings.Split(p.Day, "|")[0] }
