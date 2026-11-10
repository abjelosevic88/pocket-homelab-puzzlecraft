package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// ---- AI help: words, clues, questions, stories, hints ----
//
// Only when the person presses a button; nothing is kept until they keep it.

func (s *Server) aiGate(w http.ResponseWriter, r *http.Request) (int64, store.Settings, bool) {
	ctx := r.Context()
	uid := actorID(ctx)
	st := s.settingsOf(ctx, uid)
	if !st.AIHelp {
		writeErr(w, http.StatusBadRequest, "AI help is switched off (Settings → AI)")
		return 0, st, false
	}
	select {
	case s.aiSem <- struct{}{}:
	default:
		writeErr(w, http.StatusTooManyRequests, "the AI is busy with other questions; try again in a moment")
		return 0, st, false
	}
	return uid, st, true
}

func (s *Server) aiDone() { <-s.aiSem }

func langName(l string) string {
	switch l {
	case "bcs":
		return "Bosnian/Croatian/Serbian (Latin letters)"
	case "en":
		return "English"
	}
	return "the language of the words"
}

func audience(a string) string {
	switch a {
	case "kids":
		return "children aged 6 to 10: simple, friendly words"
	case "teens":
		return "teenagers"
	case "cryptic":
		return "experienced solvers: cryptic-style clues with wordplay"
	}
	return "adults, everyday knowledge"
}

// handleAIClues writes a clue for each word.
func (s *Server) handleAIClues(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	var in struct {
		Words    []string `json:"words"`
		Language string   `json:"language"`
		Level    string   `json:"level"`
		Audience string   `json:"audience"`
		Theme    string   `json:"theme"`
	}
	if err := decode(r, &in); err != nil || len(in.Words) == 0 || len(in.Words) > 120 {
		writeErr(w, http.StatusBadRequest, "send 1 to 120 words")
		return
	}
	system := `You write clues for crossword and word puzzles. Answer with JSON only: {"clues": [{"word": "CAT", "clue": "Pet that purrs"}]}.
Rules: one clue per word, in the same order; never use the word itself (or a form of it) in its clue; keep clues short (under 12 words); a clue must have exactly one fair answer of that length.`
	prompt := fmt.Sprintf("Write clues in %s for %s, at a %s level%s.\nWords:\n%s", langName(firstNonEmpty(in.Language, st.Language)), audience(in.Audience),
		firstNonEmpty(in.Level, "medium"), themeLine(in.Theme), strings.Join(in.Words, "\n"))
	var out struct {
		Clues []puzzle.WordClue `json:"clues"`
	}
	if err := s.aiAsk(r.Context(), uid, st, system, prompt, nil, &out); err != nil {
		s.userFail(w, err, "ai")
		return
	}
	// keep them in the words' order, by word
	by := map[string]string{}
	for _, c := range out.Clues {
		by[strings.ToUpper(strings.Join(puzzle.Letters(c.Word, "bcs"), ""))] = strings.TrimSpace(c.Clue)
	}
	res := []puzzle.WordClue{}
	for i, wd := range in.Words {
		k := strings.ToUpper(strings.Join(puzzle.Letters(wd, "bcs"), ""))
		c := by[k]
		if c == "" && i < len(out.Clues) {
			c = out.Clues[i].Clue
		}
		res = append(res, puzzle.WordClue{Word: wd, Clue: c})
	}
	writeJSON(w, http.StatusOK, map[string]any{"clues": res})
}

func themeLine(t string) string {
	if t = strings.TrimSpace(t); t != "" {
		return ", on the theme “" + clip(t, 200) + "”"
	}
	return ""
}

// handleAIWords makes a word list on a topic (with clues).
func (s *Server) handleAIWords(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	var in struct {
		Topic    string `json:"topic"`
		Count    int    `json:"count"`
		Language string `json:"language"`
		Audience string `json:"audience"`
		MaxLen   int    `json:"max_len"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Topic) == "" {
		writeErr(w, http.StatusBadRequest, "what should the words be about?")
		return
	}
	n := clampInt(firstPos(in.Count, 15), 3, 60)
	maxLen := clampInt(firstPos(in.MaxLen, 12), 3, 20)
	system := `You make word lists for puzzles (crosswords, word searches). Answer with JSON only: {"words": [{"word": "APPLE", "clue": "Fruit that keeps the doctor away"}]}.
Rules: single words only (no spaces), no repeats, real words people know, each with a short fair clue that does not contain the word.`
	prompt := fmt.Sprintf("Make %d words about “%s” in %s for %s. Each word at most %d letters.", n, clip(in.Topic, 200), langName(firstNonEmpty(in.Language, st.Language)), audience(in.Audience), maxLen)
	var out struct {
		Words []puzzle.WordClue `json:"words"`
	}
	if err := s.aiAsk(r.Context(), uid, st, system, prompt, nil, &out); err != nil {
		s.userFail(w, err, "ai")
		return
	}
	res := []puzzle.WordClue{}
	seen := map[string]bool{}
	for _, x := range out.Words {
		wd := strings.TrimSpace(x.Word)
		k := strings.ToUpper(wd)
		if wd == "" || strings.Contains(wd, " ") || seen[k] || len([]rune(wd)) > maxLen {
			continue
		}
		seen[k] = true
		res = append(res, puzzle.WordClue{Word: wd, Clue: strings.TrimSpace(x.Clue)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"words": res})
}

// handleAIQuiz writes quiz questions (or riddles) on a topic.
func (s *Server) handleAIQuiz(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	var in struct {
		Topic    string `json:"topic"`
		Count    int    `json:"count"`
		Mode     string `json:"mode"` // quiz | riddles | trivia
		Choices  bool   `json:"choices"`
		Level    string `json:"level"`
		Language string `json:"language"`
		Audience string `json:"audience"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Topic) == "" {
		writeErr(w, http.StatusBadRequest, "what should the questions be about?")
		return
	}
	n := clampInt(firstPos(in.Count, 10), 1, 40)
	system := `You write quiz questions. Answer with JSON only: {"questions": [{"q": "...", "type": "choice", "choices": ["A", "B", "C", "D"], "answers": ["B"], "explain": "one short sentence", "hint": "a gentle hint"}]}.
Rules: type is "choice" (then 4 choices, answers holds the right one exactly as written in choices), "text" (answers: the right answer and any other accepted spellings), "number" (answers: the number) or "truefalse" (answers: "True" or "False").
Facts must be correct and not change over time; if unsure, leave the question out.`
	kind := "quiz questions"
	if in.Mode == "riddles" {
		kind = "riddles (type text, short answers)"
	}
	style := "mix multiple choice, short text answers and numbers"
	if in.Choices {
		style = "all multiple choice"
	}
	prompt := fmt.Sprintf("Write %d %s about “%s” in %s for %s, %s difficulty; %s.", n, kind, clip(in.Topic, 200), langName(firstNonEmpty(in.Language, st.Language)),
		audience(in.Audience), firstNonEmpty(in.Level, "medium"), style)
	var out struct {
		Questions []puzzle.Question `json:"questions"`
	}
	if err := s.aiAsk(r.Context(), uid, st, system, prompt, nil, &out); err != nil {
		s.userFail(w, err, "ai")
		return
	}
	q := &puzzle.Quiz{Mode: firstNonEmpty(in.Mode, "quiz"), Questions: out.Questions}
	clean, err := puzzle.Clean("quiz", mustJSON(q))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "the AI's questions did not come out right; try again")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": clean})
}

// handleAIStory writes an escape room, hunt or mystery: story, stages,
// answers and hints, ready to change.
func (s *Server) handleAIStory(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	var in struct {
		Kind     string `json:"kind"`
		Theme    string `json:"theme"`
		Stages   int    `json:"stages"`
		Players  string `json:"players"`
		Place    string `json:"place"`
		Minutes  int    `json:"minutes"`
		Language string `json:"language"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Theme) == "" {
		writeErr(w, http.StatusBadRequest, "give it a theme (a pirate birthday, a haunted library, a spy mission…)")
		return
	}
	n := clampInt(firstPos(in.Stages, 5), 2, 12)
	system := `You design home escape rooms, treasure hunts and party mysteries that people can set up with ordinary things at home or outdoors.
Answer with JSON only: {"title": "...", "intro": "the story players read first (3-5 sentences)", "finale": "what they read when they win (2-3 sentences)",
"stages": [{"title": "...", "story": "what players read at this stage, ending with the puzzle to solve", "lock": "answer|code|word|directions|colors|qr",
"answer": "the answer (code: digits only; directions: letters U D L R; colors: comma separated colour names)", "lock_hint": "e.g. A 4-digit lock",
"hints": ["gentle hint", "stronger hint", "the answer, spelled out"], "hidden": "for the host: what to hide where", "unlock": "what players learn when they solve it", "props": "things the host needs"}],
"prep": ["setting-up step", "..."]}
Rules: each stage's answer must follow from what players have (the story, an object, an earlier unlock); use a mix of lock kinds; "qr" means a printed card hidden somewhere that players find and scan (give no answer for it); keep it safe and fun; no real dangers or locks on doors.`
	prompt := fmt.Sprintf("Make a %s with %d stages, theme: “%s”. Players: %s. Place: %s. About %d minutes. Write it in %s.", map[string]string{"escape": "home escape room", "hunt": "treasure hunt", "mystery": "party mystery", "party": "children's party game"}[firstNonEmpty(in.Kind, "escape")],
		n, clip(in.Theme, 300), firstNonEmpty(clip(in.Players, 200), "a small group of friends"), firstNonEmpty(clip(in.Place, 200), "at home"), firstPos(in.Minutes, 60), langName(firstNonEmpty(in.Language, st.Language)))
	var out struct {
		Title  string `json:"title"`
		Intro  string `json:"intro"`
		Finale string `json:"finale"`
		Stages []struct {
			Title    string   `json:"title"`
			Story    string   `json:"story"`
			Lock     string   `json:"lock"`
			Answer   string   `json:"answer"`
			LockHint string   `json:"lock_hint"`
			Hints    []string `json:"hints"`
			Hidden   string   `json:"hidden"`
			Unlock   string   `json:"unlock"`
			Props    string   `json:"props"`
		} `json:"stages"`
		Prep []string `json:"prep"`
	}
	if err := s.aiAsk(r.Context(), uid, st, system, prompt, nil, &out); err != nil {
		s.userFail(w, err, "ai")
		return
	}
	stages := []map[string]any{}
	for _, x := range out.Stages {
		lock := x.Lock
		if !oneOf(lock, store.Locks) || lock == "puzzle" {
			lock = "answer"
		}
		hs := []store.Hint{}
		for i, h := range x.Hints {
			hs = append(hs, store.Hint{Text: h, After: 3 + i*4})
		}
		ans := []string{}
		if strings.TrimSpace(x.Answer) != "" && lock != "qr" {
			ans = append(ans, strings.TrimSpace(x.Answer))
		}
		stages = append(stages, map[string]any{"title": x.Title, "story": x.Story, "lock": lock, "answers": ans, "lock_hint": x.LockHint, "hints": hs,
			"hidden": x.Hidden, "unlock": x.Unlock, "props": x.Props})
	}
	writeJSON(w, http.StatusOK, map[string]any{"title": out.Title, "intro": out.Intro, "finale": out.Finale, "stages": stages, "prep": out.Prep})
}

// handleAIHints writes three hints, gentle to strong, for a question and its answer.
func (s *Server) handleAIHints(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	var in struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
		Language string `json:"language"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Question) == "" {
		writeErr(w, http.StatusBadRequest, "send the question (and its answer)")
		return
	}
	system := `You write hints for puzzles. Answer with JSON only: {"hints": ["gentle nudge", "stronger hint", "almost the answer"]}. Never give the answer itself in the first two.`
	prompt := fmt.Sprintf("Language: %s.\nPuzzle: %s\nAnswer: %s", langName(firstNonEmpty(in.Language, st.Language)), clip(in.Question, 3000), clip(in.Answer, 300))
	var out struct {
		Hints []string `json:"hints"`
	}
	if err := s.aiAsk(r.Context(), uid, st, system, prompt, nil, &out); err != nil {
		s.userFail(w, err, "ai")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAICheckClue says whether a clue is fair and suggests a better one.
func (s *Server) handleAICheckClue(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	var in struct {
		Clue   string `json:"clue"`
		Answer string `json:"answer"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Clue) == "" || strings.TrimSpace(in.Answer) == "" {
		writeErr(w, http.StatusBadRequest, "send the clue and its answer")
		return
	}
	system := `You check crossword clues. Answer with JSON only: {"fair": true, "problem": "", "others": ["other answers of the same length that also fit"], "better": "a clearer clue"}.`
	prompt := fmt.Sprintf("Clue: %s\nAnswer (%d letters): %s", clip(in.Clue, 300), len(puzzle.Letters(in.Answer, "bcs")), clip(in.Answer, 60))
	var out map[string]any
	if err := s.aiAsk(r.Context(), uid, st, system, prompt, nil, &out); err != nil {
		s.userFail(w, err, "ai")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAIReadPuzzle reads a photo of a printed sudoku into its numbers.
func (s *Server) handleAIReadPuzzle(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	var in struct {
		PhotoID int64 `json:"photo_id"`
	}
	if err := decode(r, &in); err != nil || in.PhotoID == 0 {
		writeErr(w, http.StatusBadRequest, "pick a photo of the sudoku")
		return
	}
	imgs, err := s.photoForAI(r.Context(), uid, in.PhotoID)
	if err != nil {
		s.fail(w, err, "photo")
		return
	}
	system := `You read a photo of a printed 9 × 9 sudoku. Answer with JSON only: {"rows": ["53..7....", "6..195...", ...]} — nine strings of nine characters, a digit for each printed number and "." for each empty square. Ignore pencil marks and handwriting. If it is not a sudoku, answer {"rows": []}.`
	var out struct {
		Rows []string `json:"rows"`
	}
	if err := s.aiAsk(r.Context(), uid, st, system, "Read this sudoku.", imgs, &out); err != nil {
		s.userFail(w, err, "ai")
		return
	}
	if len(out.Rows) != 9 {
		writeErr(w, http.StatusBadRequest, "that does not look like a sudoku — try a straighter, sharper photo")
		return
	}
	givens := make([]int, 81)
	for r, row := range out.Rows {
		for c, ch := range []rune(row) {
			if c < 9 && ch >= '1' && ch <= '9' {
				givens[r*9+c] = int(ch - '0')
			}
		}
	}
	data, err := puzzle.Clean("sudoku", mustJSON(&puzzle.Sudoku{Size: 9, Givens: givens}))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	rep, _ := puzzle.Test("sudoku", data)
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "report": rep})
}
