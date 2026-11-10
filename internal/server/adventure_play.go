package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// ---- playing an escape room, treasure hunt or mystery ----
//
// Players see one stage at a time: its story, its picture and its lock.
// A lock opens with the right answer (a word, a code, directions, colours),
// by scanning the QR card hidden at that stage, or by solving a puzzle.
// Hints come one by one, each only once its minutes at the stage have
// passed. The host's notes (where things are hidden, the props) never
// reach players.

func minutesSince(ts *string) float64 {
	if ts == nil {
		return 0
	}
	t, err := time.Parse(time.RFC3339Nano, *ts)
	if err != nil {
		return 0
	}
	return time.Since(t).Minutes()
}

// adventureView is the adventure as a player sees it now.
func (s *Server) adventureView(ctx context.Context, p *store.Play, t *target) map[string]any {
	a := t.adv
	st := readState(p)
	out := map[string]any{"id": a.ID, "kind": a.Kind, "title": a.Title, "intro": a.Intro, "theme": a.Theme, "minutes": a.Minutes, "hint_penalty": a.HintPenalty,
		"stages": len(a.Stages), "done": p.Stage, "finished": p.FinishedAt != nil, "over": st.Over, "photo": a.PhotoID != nil}
	if p.FinishedAt != nil {
		out["finale"] = a.Finale
		if a.Board {
			out["best"] = s.board(ctx, p.UserID, "adventure", a.ID, 10)
		}
	}
	// the stages done so far, with what solving them said
	past := []map[string]any{}
	for i := 0; i < p.Stage && i < len(a.Stages); i++ {
		x := a.Stages[i]
		past = append(past, map[string]any{"title": x.Title, "unlock": x.Unlock})
	}
	out["past"] = past
	if t.stage != nil && p.FinishedAt == nil {
		x := t.stage
		shown := st.Hints[strconv.FormatInt(x.ID, 10)]
		at := minutesSince(p.StageAt)
		hints := []string{}
		for i := 0; i < shown && i < len(x.Hints); i++ {
			hints = append(hints, x.Hints[i].Text)
		}
		next := -1.0 // minutes until the next hint can be had
		if shown < len(x.Hints) {
			next = float64(x.Hints[shown].After) - at
			if next < 0 {
				next = 0
			}
		}
		cur := map[string]any{"id": x.ID, "n": p.Stage + 1, "title": x.Title, "story": x.Story, "lock": x.Lock, "lock_hint": x.LockHint, "photo": x.PhotoID != nil,
			"photo_id": x.PhotoID, "place_name": x.PlaceName, "lat": x.Lat, "lng": x.Lng, "hints": hints, "hints_left": len(x.Hints) - shown, "next_hint_in": next,
			"started_at": p.StageAt, "length": lockLength(x)}
		if x.Lock == "puzzle" && t.puzzle != nil {
			cur["puzzle"] = s.publicPuzzle(t.puzzle)
		}
		if x.Lock == "colors" {
			cur["colors"] = colorChoices(x)
		}
		out["stage"] = cur
	}
	return out
}

// lockLength is how many wheels, letters or moves a lock has (from its first answer).
func lockLength(x *store.Stage) int {
	if len(x.Answers) == 0 {
		return 0
	}
	a := x.Answers[0]
	switch x.Lock {
	case "code":
		return len(digitsOf(a))
	case "word":
		return len(puzzle.Letters(a, "en"))
	case "directions":
		return len(directionsOf(a))
	case "colors":
		return len(colorsOf(a))
	}
	return 0
}

var colorWords = []string{"red", "orange", "yellow", "green", "blue", "purple", "pink", "white", "black", "brown"}

func colorChoices(x *store.Stage) []string {
	used := map[string]bool{}
	for _, a := range x.Answers {
		for _, c := range colorsOf(a) {
			used[c] = true
		}
	}
	out := []string{}
	for _, c := range colorWords {
		if used[c] || len(out) < 6 {
			out = append(out, c)
		}
	}
	return out
}

func digitsOf(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// directionsOf reads ↑↓←→, U/D/L/R or up/down/left/right as "UDLR".
func directionsOf(s string) string {
	s = strings.ToUpper(s)
	for k, v := range map[string]string{"UP": "U", "DOWN": "D", "LEFT": "L", "RIGHT": "R", "↑": "U", "↓": "D", "←": "L", "→": "R"} {
		s = strings.ReplaceAll(s, k, v)
	}
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("UDLR", r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func colorsOf(s string) []string {
	out := []string{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ',' || r == ' ' || r == '-' || r == '/' }) {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// lockOpens says whether an answer opens a stage's lock.
func lockOpens(x *store.Stage, answer string) bool {
	for _, a := range x.Answers {
		switch x.Lock {
		case "code":
			if d := digitsOf(answer); d != "" && d == digitsOf(a) {
				return true
			}
		case "directions":
			if d := directionsOf(answer); d != "" && d == directionsOf(a) {
				return true
			}
		case "colors":
			if strings.Join(colorsOf(answer), ",") == strings.Join(colorsOf(a), ",") && answer != "" {
				return true
			}
		default:
			if puzzle.SameAnswer(answer, a) {
				return true
			}
		}
	}
	return false
}

// passStage moves a go to the next stage (or to the end) and answers with
// what the player sees now.
func (s *Server) passStage(ctx context.Context, w http.ResponseWriter, p *store.Play, t *target, browserMs int64, out map[string]any) {
	st := readState(p)
	x := t.stage
	stageMs := int64(minutesSince(p.StageAt) * 60000)
	st.Log = append(st.Log, stageLog{Stage: x.ID, Ms: stageMs, Hints: st.Hints[strconv.FormatInt(x.ID, 10)]})
	p.Stage++
	now := store.Now()
	p.StageAt = &now
	if t.adv.Minutes > 0 && minutesSince(&p.StartedAt) > float64(t.adv.Minutes) {
		st.Over = true
	}
	finished := p.Stage >= len(t.adv.Stages)
	if finished {
		p.FinishedAt = &now
		p.Ms = elapsed(p, browserMs) + int64(p.Hints)*int64(t.adv.HintPenalty)*1000
	}
	writeState(p, st)
	_ = s.store.UpdatePlay(ctx, p)
	if finished {
		s.solved(ctx, p, t)
	}
	nt, _ := s.targetOf(ctx, p)
	if nt == nil {
		nt = t
	}
	out["opened"] = true
	out["unlock"] = x.Unlock
	out["play"] = s.playView(ctx, p, nt)
	writeJSON(w, http.StatusOK, out)
}

// handleStageAnswer tries an answer on the current stage's lock.
func (s *Server) handleStageAnswer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, t, ok := s.playByKey(w, r)
	if !ok {
		return
	}
	if t.adv == nil || t.stage == nil || p.FinishedAt != nil {
		writeErr(w, http.StatusBadRequest, "there is no lock to open here")
		return
	}
	var in struct {
		Answer string `json:"answer"`
		Ms     int64  `json:"ms"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	x := t.stage
	if x.Lock == "puzzle" || x.Lock == "qr" {
		writeErr(w, http.StatusBadRequest, "this stage opens another way")
		return
	}
	if x.Lock == "none" || lockOpens(x, in.Answer) {
		s.passStage(ctx, w, p, t, in.Ms, map[string]any{})
		return
	}
	p.Mistakes++
	_ = s.store.UpdatePlay(ctx, p)
	writeJSON(w, http.StatusOK, map[string]any{"opened": false, "mistakes": p.Mistakes, "message": "Not yet — try again"})
}

// handleStageScan opens a "find the card" stage with the code in its QR card.
func (s *Server) handleStageScan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, t, ok := s.playByKey(w, r)
	if !ok {
		return
	}
	var in struct {
		Code string `json:"code"`
		Ms   int64  `json:"ms"`
	}
	_ = decode(r, &in)
	if t.adv == nil || t.stage == nil || p.FinishedAt != nil {
		writeJSON(w, http.StatusOK, map[string]any{"opened": false, "message": "Nothing to find right now"})
		return
	}
	code := strings.TrimSpace(in.Code)
	if code != "" && t.stage.Code != "" && code == t.stage.Code {
		s.passStage(ctx, w, p, t, in.Ms, map[string]any{})
		return
	}
	// a card from another stage: say whether it is ahead or behind
	for i, x := range t.adv.Stages {
		if x.Code != "" && x.Code == code {
			msg := "You found this one already"
			if i > p.Stage {
				msg = "Good find — but that card is for later. Keep it!"
			}
			writeJSON(w, http.StatusOK, map[string]any{"opened": false, "message": msg})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"opened": false, "message": "That card is not part of this game"})
}

// handleStageHint shows the next hint at the current stage, once its time has come.
func (s *Server) handleStageHint(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, t, ok := s.playByKey(w, r)
	if !ok {
		return
	}
	if t.adv == nil || t.stage == nil || p.FinishedAt != nil {
		writeErr(w, http.StatusBadRequest, "no hints here")
		return
	}
	st := readState(p)
	k := strconv.FormatInt(t.stage.ID, 10)
	shown := st.Hints[k]
	if shown >= len(t.stage.Hints) {
		writeErr(w, http.StatusBadRequest, "there are no more hints for this stage")
		return
	}
	if wait := float64(t.stage.Hints[shown].After) - minutesSince(p.StageAt); wait > 0.05 {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("the next hint comes in %d %s", int(wait+0.99), plural(int(wait+0.99), "minute", "minutes")))
		return
	}
	st.Hints[k] = shown + 1
	p.Hints++
	writeState(p, st)
	_ = s.store.UpdatePlay(ctx, p)
	writeJSON(w, http.StatusOK, s.playView(ctx, p, t))
}

// handlePublicAdventure is a shared adventure's first page.
func (s *Server) handlePublicAdventure(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, err := s.store.AdventureByShare(ctx, chiParam(r, "token"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this game is not shared (any more)")
		return
	}
	out := map[string]any{"title": a.Title, "kind": a.Kind, "intro": a.Intro, "theme": a.Theme, "minutes": a.Minutes, "stages": len(a.Stages), "date": a.Date,
		"time": a.Time, "place": a.Place, "from": s.ownerName(ctx, a.UserID), "photo": a.PhotoID != nil, "photo_id": a.PhotoID, "hint_penalty": a.HintPenalty}
	if a.Board {
		out["best"] = s.board(ctx, a.UserID, "adventure", a.ID, 10)
	}
	writeJSON(w, http.StatusOK, out)
}

// handlePublicAdventurePlay starts a team's go.
func (s *Server) handlePublicAdventurePlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, err := s.store.AdventureByShare(ctx, chiParam(r, "token"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this game is not shared (any more)")
		return
	}
	if len(a.Stages) == 0 {
		writeErr(w, http.StatusBadRequest, "this game has no stages yet")
		return
	}
	var in struct {
		Player string `json:"player"`
		Key    string `json:"key"`
	}
	_ = decode(r, &in)
	p, err := s.startPlay(ctx, &store.Play{UserID: a.UserID, What: "adventure", RefID: a.ID, Kind: a.Kind, Via: "link", Player: in.Player}, in.Key)
	if err != nil {
		s.fail(w, err, "play")
		return
	}
	t, _ := s.targetOf(ctx, p)
	writeJSON(w, http.StatusOK, s.playView(ctx, p, t))
}

// handleAdventurePlay starts your own test run of an adventure (shared or not).
func (s *Server) handleAdventurePlay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	a, err := s.store.GetAdventure(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "adventure")
		return
	}
	if len(a.Stages) == 0 {
		writeErr(w, http.StatusBadRequest, "add a stage first")
		return
	}
	p, err := s.startPlay(ctx, &store.Play{UserID: uid, What: "adventure", RefID: a.ID, Kind: a.Kind, Via: "me", Player: "Test run"}, "")
	if err != nil {
		s.fail(w, err, "play")
		return
	}
	t, _ := s.targetOf(ctx, p)
	writeJSON(w, http.StatusOK, s.playView(ctx, p, t))
}

// handleAdventureLive is the host's view: every team, where they are, how long, hints.
func (s *Server) handleAdventureLive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	a, err := s.store.GetAdventure(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "adventure")
		return
	}
	list, _ := s.store.ListPlays(ctx, uid, store.PlayFilter{What: "adventure", RefID: a.ID, Limit: 200})
	out := []map[string]any{}
	for _, p := range list {
		st := readState(&p)
		stage := ""
		if p.Stage < len(a.Stages) {
			stage = firstNonEmpty(a.Stages[p.Stage].Title, fmt.Sprintf("Stage %d", p.Stage+1))
		}
		ms := p.Ms
		if p.FinishedAt == nil {
			ms = int64(minutesSince(&p.StartedAt) * 60000)
		}
		out = append(out, map[string]any{"id": p.ID, "player": firstNonEmpty(p.Player, "A team"), "via": p.Via, "done": p.Stage, "stages": len(a.Stages), "stage": stage,
			"started_at": p.StartedAt, "finished_at": p.FinishedAt, "ms": ms, "hints": p.Hints, "mistakes": p.Mistakes, "last_at": p.LastAt, "over": st.Over,
			"at_stage_min": minutesSince(p.StageAt), "log": st.Log})
	}
	writeJSON(w, http.StatusOK, map[string]any{"plays": out, "stages": len(a.Stages), "minutes": a.Minutes})
}
