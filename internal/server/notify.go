package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// pushes runs every minute; at each person's hour it sends (once a day)
// the puzzles of the day, and the day before an adventure what is still
// to set up.
func (s *Server) pushes(ctx context.Context) {
	now := time.Now()
	if now.Minute() > 10 {
		return
	}
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	tomorrow := store.AddDays(today(), 1)
	for _, uid := range ids {
		st, err := s.store.GetSettings(ctx, uid)
		if err != nil || st.NotifyURL == "" || now.Hour() != st.NotifyHour {
			continue
		}
		if st.NotifyPrep {
			advs, _ := s.store.ListAdventures(ctx, uid)
			for _, a := range advs {
				if a.Date != tomorrow || a.Status == "done" || !s.once(ctx, fmt.Sprintf("prep:%d:%d:%s", uid, a.ID, a.Date)) {
					continue
				}
				lines := []string{}
				for _, p := range a.Prep {
					if !p.Done {
						lines = append(lines, "• "+p.Text)
					}
				}
				body := "Everything is set up."
				if len(lines) > 0 {
					body = "Still to do:\n" + strings.Join(lines, "\n")
				}
				s.push(ctx, uid, st, "Tomorrow: "+a.Title, body, s.link(fmt.Sprintf("/adventures/%d?tab=prep", a.ID)))
			}
		}
		if st.NotifyDaily && st.Daily && s.once(ctx, fmt.Sprintf("daily:%d:%s", uid, today())) {
			names := []string{}
			for _, k := range puzzle.DailyKinds(today()) {
				names = append(names, strings.ToLower(puzzle.KindName[k]))
			}
			s.push(ctx, uid, st, "Today's puzzles are ready", "A "+strings.Join(names, ", a ")+".", s.link("/today"))
		}
	}
}

// once is true the first time it is asked for a key.
func (s *Server) once(ctx context.Context, key string) bool {
	if _, _, done := s.store.CacheGet(ctx, key); done {
		return false
	}
	_ = s.store.CachePut(ctx, key, "1")
	return true
}

// push sends one message to a person's own notification address.
func (s *Server) push(ctx context.Context, uid int64, st store.Settings, title, body, link string) bool {
	if st.NotifyURL == "" {
		return false
	}
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: title, Body: body, Priority: "info", Tags: []string{"jigsaw"}, Link: link}); err != nil {
		s.log.Warn("notify failed", "user", uid, "err", err)
		return false
	}
	return true
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, _ := s.store.GetSettings(ctx, actorID(ctx))
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Pocket Puzzlecraft", Body: notifyTestBody, Priority: "info", Tags: []string{"jigsaw"}}); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

const notifyTestBody = "Notifications work. You hear when someone finishes a puzzle you shared, and the day before an escape room what is still to set up."
