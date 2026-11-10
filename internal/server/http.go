package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// Router builds the chi router.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	if s.cfg.TrustProxy {
		r.Use(middleware.RealIP)
	}
	r.Use(middleware.RequestID)
	r.Use(s.logging)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	r.Get("/pocket-ask.js", s.core.AskWidget("puzzlecraft"))
	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
	r.Get("/api/health", s.handleHealth)
	if s.cfg.MetricsEnabled {
		r.Get("/metrics", s.handleMetrics)
	}

	r.Route("/api/v1", func(r chi.Router) {
		// players: no account, only the link (or the key of their go)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(30 * time.Second))
			r.Get("/public/p/{token}", s.handlePublicPuzzle)
			r.Post("/public/p/{token}/play", s.handlePublicPuzzlePlay)
			r.Get("/public/p/{token}/photo/{id}", s.handlePublicPhoto)
			r.Get("/public/b/{token}", s.handlePublicBook)
			r.Post("/public/b/{token}/play", s.handlePublicBookPlay)
			r.Get("/public/a/{token}", s.handlePublicAdventure)
			r.Post("/public/a/{token}/play", s.handlePublicAdventurePlay)
			r.Get("/public/a/{token}/photo/{id}", s.handlePublicPhoto)
			r.Get("/public/play/{key}", s.handlePlayState)
			r.Post("/public/play/{key}/save", s.handlePlaySave)
			r.Post("/public/play/{key}/check", s.handlePlayCheck)
			r.Post("/public/play/{key}/reveal", s.handlePlayReveal)
			r.Post("/public/play/{key}/answer", s.handleStageAnswer)
			r.Post("/public/play/{key}/scan", s.handleStageScan)
			r.Post("/public/play/{key}/hint", s.handleStageHint)
			r.Get("/public/play/{key}/photo/{id}", s.handlePublicPhoto)
		})

		// imports, uploads, making puzzles and AI can take longer than a request may
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Post("/import", s.handleImport)
			r.Post("/photos", s.handleUploadPhoto)
			r.Get("/export.json", s.handleExportJSON)
			r.Post("/generate", s.handleGenerate)
			r.Post("/crossword/fill", s.handleCrosswordFill)
			r.Post("/apps/files", s.handleToFiles)
			r.Post("/apps/photos/{uuid}/copy", s.handlePhotoCopy)
			r.Post("/ai/clues", s.handleAIClues)
			r.Post("/ai/words", s.handleAIWords)
			r.Post("/ai/quiz", s.handleAIQuiz)
			r.Post("/ai/story", s.handleAIStory)
			r.Post("/ai/hints", s.handleAIHints)
			r.Post("/ai/check-clue", s.handleAICheckClue)
			r.Post("/ai/read-puzzle", s.handleAIReadPuzzle)
			r.Post("/puzzles", s.handleCreatePuzzle)
			r.Put("/puzzles/{id}", s.handleUpdatePuzzle)
			r.Post("/puzzles/{id}/test", s.handleTestPuzzle)
			r.Post("/test", s.handleTestData)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(120 * time.Second))
			r.Post("/auth/login", s.handleLogin)
			r.Post("/auth/setup", s.handleSetup)
			r.Get("/auth/status", s.handleAuthStatus)
			r.Get("/auth/oidc/start", s.handleOIDCStart)
			r.Get("/auth/oidc/callback", s.handleOIDCCallback)

			r.Group(func(r chi.Router) {
				r.Use(s.requireAuth)
				r.Post("/auth/logout", s.handleLogout)
				r.Get("/auth/me", s.handleMe)
				r.Put("/auth/me", s.handleUpdateMe)

				// overview, and for the rest of the box: Pocket Day, Search, AI
				r.Get("/today", s.handleToday)
				r.Get("/home", s.handleHome)
				r.Get("/summary", s.handleSummary)
				r.Get("/stats", s.handleStats)
				r.Get("/day/{date}", s.handleDay)
				r.Get("/search", s.handleSearch)
				r.Get("/kinds", s.handleKinds)
				r.Get("/themes", s.handleThemes)
				r.Get("/templates", s.handleTemplates)
				r.Get("/words", s.handleWords)
				r.Post("/crossword/grid", s.handleCrosswordGrid)

				// puzzles
				r.Get("/puzzles", s.handleListPuzzles)
				r.Get("/puzzles/{id}", s.handleGetPuzzle)
				r.Delete("/puzzles/{id}", s.handleDeletePuzzle)
				r.Post("/puzzles/{id}/duplicate", s.handleDuplicatePuzzle)
				r.Post("/puzzles/{id}/share", s.handleSharePuzzle)
				r.Post("/puzzles/{id}/play", s.handlePuzzlePlay)
				r.Get("/puzzles/{id}/plays", s.handlePuzzlePlays)
				r.Get("/puzzles/{id}/print", s.handlePuzzlePrint)
				r.Get("/puzzles/{id}/export.ipuz", s.handleExportIPUZ)
				r.Get("/puzzles/{id}/export.puz", s.handleExportPUZ)
				r.Delete("/plays/{id}", s.handleDeletePlay)
				r.Post("/daily/{date}/{kind}/play", s.handleDailyPlay)

				// books and word lists
				r.Get("/books", s.handleListBooks)
				r.Post("/books", s.handleSaveBook)
				r.Get("/books/{id}", s.handleGetBook)
				r.Put("/books/{id}", s.handleSaveBook)
				r.Delete("/books/{id}", s.handleDeleteBook)
				r.Get("/books/{id}/print", s.handleBookPrint)
				r.Get("/wordlists", s.handleListWordLists)
				r.Post("/wordlists", s.handleSaveWordList)
				r.Get("/wordlists/{id}", s.handleGetWordList)
				r.Put("/wordlists/{id}", s.handleSaveWordList)
				r.Delete("/wordlists/{id}", s.handleDeleteWordList)

				// escape rooms, treasure hunts, mysteries
				r.Get("/adventures", s.handleListAdventures)
				r.Post("/adventures", s.handleCreateAdventure)
				r.Get("/adventures/{id}", s.handleGetAdventure)
				r.Put("/adventures/{id}", s.handleUpdateAdventure)
				r.Delete("/adventures/{id}", s.handleDeleteAdventure)
				r.Post("/adventures/{id}/stages", s.handleCreateStage)
				r.Put("/adventures/{id}/order", s.handleReorderStages)
				r.Put("/stages/{id}", s.handleUpdateStage)
				r.Delete("/stages/{id}", s.handleDeleteStage)
				r.Post("/adventures/{id}/play", s.handleAdventurePlay)
				r.Get("/adventures/{id}/live", s.handleAdventureLive)
				r.Get("/adventures/{id}/plays", s.handlePuzzlePlays)
				r.Post("/adventures/{id}/tasks", s.handlePrepTasks)
				r.Post("/adventures/{id}/map", s.handleToMap)
				r.Get("/photos/{id}", s.handlePhoto)

				// the other Pocket apps
				r.Get("/apps", s.handleApps)
				r.Get("/apps/chats", s.handleChats)
				r.Post("/apps/chat", s.handleShareChat)
				r.Post("/apps/email", s.handleShareEmail)
				r.Get("/apps/contacts", s.handleContacts)
				r.Post("/apps/calendar", s.handleToCalendar)
				r.Get("/apps/photos", s.handlePhotosList)
				r.Get("/apps/photos/{uuid}/image", s.handlePhotoImage)
				r.Post("/apps/notes", s.handleToNotes)
				r.Get("/apps/inventory", s.handleInventory)
				r.Get("/apps/places", s.handlePlaces)
				r.Get("/apps/learn/decks", s.handleLearnDecks)
				r.Post("/apps/learn", s.handleToLearn)
				r.Get("/apps/books", s.handleBooksList)
				r.Get("/apps/books/{id}/quotes", s.handleBookQuotes)
				r.Get("/apps/habits", s.handleHabits)
				r.Get("/apps/quest/campaigns", s.handleQuestCampaigns)
				r.Post("/apps/quest", s.handleToQuest)
				r.Post("/apps/blog", s.handleToBlog)
				r.Get("/apps/family", s.handleFamily)
				r.Post("/apps/family", s.handleToFamily)
				r.Post("/apps/translate", s.handleTranslate)

				// AI
				r.Get("/ai/status", s.handleAIStatus)
				r.Get("/ai/models", s.handleAIModels)

				r.Post("/integrations/test", s.handleIntegrationTest)
				r.Post("/notify/test", s.handleNotifyTest)

				r.Get("/users", s.handleListUsers)
				r.Post("/users", s.handleCreateUser)
				r.Put("/users/{id}", s.handleUpdateUser)
				r.Delete("/users/{id}", s.handleDeleteUser)

				r.Get("/settings", s.handleGetSettings)
				r.Put("/settings", s.handlePutSettings)
				r.Get("/settings/system", s.handleSystemInfo)

				r.Get("/tokens", s.handleListTokens)
				r.Post("/tokens", s.handleCreateToken)
				r.Delete("/tokens/{id}", s.handleDeleteToken)

				r.Get("/backup", s.handleBackup)
				r.Get("/export.csv", s.handleExportCSV)
			})
		})
	})

	r.NotFound(s.handleSPA)
	return r
}

// ---------- helpers ----------

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.requests.Add(1)
		if ww.Status() >= 500 {
			s.errors.Add(1)
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			return
		}
		s.log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "dur", time.Since(start).String(), "ip", r.RemoteAddr)
	})
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func (s *Server) fail(w http.ResponseWriter, err error, context string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not found")
		return
	case errors.Is(err, store.ErrConflict), errors.Is(err, core.ErrOperationConflict):
		writeErr(w, http.StatusConflict, "already exists")
		return
	}
	s.log.Error(context, "err", err)
	writeErr(w, http.StatusInternalServerError, context+": "+err.Error())
}

func (s *Server) readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(http.MaxBytesReader(nil, r.Body, s.cfg.MaxBodyBytes))
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 32<<20))
	return dec.Decode(v)
}

// decodeKeys decodes JSON into v and also reports which top-level keys were
// present, so PUT handlers can tell "not sent" from "sent empty".
func decodeKeys(r *http.Request, v any) (map[string]bool, error) {
	b, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	keys := make(map[string]bool, len(raw))
	for k := range raw {
		keys[k] = true
	}
	return keys, json.Unmarshal(b, v)
}

func idParam(r *http.Request, name string) int64 {
	n, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return n
}

func queryInt(r *http.Request, name string) int {
	n, _ := strconv.Atoi(r.URL.Query().Get(name))
	return n
}

func queryBool(r *http.Request, name string) bool {
	v := strings.ToLower(r.URL.Query().Get(name))
	return v == "1" || v == "true" || v == "yes"
}

func validDate(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// ---------- health / metrics ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"core_outbox": s.coreOutboxHealth(r.Context()), "status": "ok", "version": Version, "uptime": time.Since(s.started).Round(time.Second).String()})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DB.PingContext(r.Context()); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	counts := s.store.Counts(ctx)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP pocket_puzzlecraft_info Build info.\n# TYPE pocket_puzzlecraft_info gauge\npocket_puzzlecraft_info{version=%q} 1\n", Version)
	fmt.Fprintf(w, "# HELP pocket_puzzlecraft_uptime_seconds Process uptime.\n# TYPE pocket_puzzlecraft_uptime_seconds gauge\npocket_puzzlecraft_uptime_seconds %d\n", int(time.Since(s.started).Seconds()))
	fmt.Fprintf(w, "# HELP pocket_puzzlecraft_http_requests_total HTTP requests served.\n# TYPE pocket_puzzlecraft_http_requests_total counter\npocket_puzzlecraft_http_requests_total %d\n", s.requests.Load())
	fmt.Fprintf(w, "# HELP pocket_puzzlecraft_http_errors_total HTTP 5xx responses.\n# TYPE pocket_puzzlecraft_http_errors_total counter\npocket_puzzlecraft_http_errors_total %d\n", s.errors.Load())
	fmt.Fprintf(w, "# HELP pocket_puzzlecraft_rows Rows per table.\n# TYPE pocket_puzzlecraft_rows gauge\n")
	for k, v := range counts {
		fmt.Fprintf(w, "pocket_puzzlecraft_rows{table=%q} %d\n", k, v)
	}
}

// ---------- static / SPA ----------

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if s.webFS == nil {
		http.Error(w, "web UI not built (run `make web`)", http.StatusNotFound)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if f, err := s.webFS.Open(path); err == nil {
		f.Close()
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.FileServerFS(s.webFS).ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	r.URL.Path = "/"
	http.FileServerFS(s.webFS).ServeHTTP(w, r)
}
