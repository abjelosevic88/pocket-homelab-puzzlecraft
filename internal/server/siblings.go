package server

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/pocket"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// ---- the other Pocket apps ----
//
// Each person chooses how Puzzlecraft reaches each app: through a Pocket Core app
// link (Puzzlecraft asks Core for a five-minute token acting as that person,
// nothing to paste), directly with their own URL + API token, or not at
// all. Core tokens are asked for per request with only the scopes the work
// needs and never kept, so a link switched off in Core applies at once.

const (
	scopeCalendarRead  = "calendar:read"
	scopeCalendarWrite = "calendar:write"
	scopeContactsRead  = "contacts:read"
	scopeChatRead      = "chat:read"
	scopeChatWrite     = "chat:write"
	scopeMailSend      = "mail:send"
	scopePhotosRead    = "photos:read"
	scopeFilesWrite    = "files:write"
	scopeNotesWrite    = "notes:write"
	scopeTasksWrite    = "tasks:write"
	scopeItemsRead     = "items:read"
	scopePlacesRead    = "places:read"
	scopeMapWrite      = "map:write"
	scopeLearnRead     = "learn:read"
	scopeLearnWrite    = "learn:write"
	scopeBooksRead     = "books:read"
	scopeHabitsRead    = "habits:read"
	scopeEntriesWrite  = "entries:write"
	scopeQuestRead     = "quest:read"
	scopeQuestWrite    = "quest:write"
	scopeBlogWrite     = "blog:write"
	scopeFamilyRead    = "family:read"
	scopeFamilyWrite   = "family:write"
	scopeTranslate     = "translate:run"
	scopeAlertsWrite   = "alerts:write"
)

// pocketApp is one sibling Pocket app Puzzlecraft can reach.
type pocketApp struct {
	slug, name string
	scopes     []string
	reason     string
}

var (
	appCalendar  = pocketApp{"calendar", "Pocket Calendar", []string{scopeCalendarRead, scopeCalendarWrite}, "Put a puzzle night, an escape room or a treasure hunt in your calendar, with the link to play"}
	appContacts  = pocketApp{"contacts", "Pocket Contacts", []string{scopeContactsRead}, "Pick the people to invite and send a puzzle to, and make a birthday puzzle for someone"}
	appChat      = pocketApp{"chat", "Pocket Chat", []string{scopeChatRead, scopeChatWrite}, "Send a puzzle to play into a chat, and tell the group who solved it fastest"}
	appEmail     = pocketApp{"email", "Pocket Email", []string{scopeMailSend}, "Email a puzzle or an escape-room invitation, with the link to play, from your own mail account"}
	appPhotos    = pocketApp{"photos", "Pocket Photos", []string{scopePhotosRead}, "Make a jigsaw or a picture question from your photos, and put pictures in escape-room stages"}
	appFiles     = pocketApp{"files", "Pocket Files", []string{scopeFilesWrite}, "Save printable puzzles, answer keys and crossword files in your files"}
	appNotes     = pocketApp{"notes", "Pocket Notes", []string{scopeNotesWrite}, "Save an answer key, or an escape room's host sheet (where every clue is hidden), as a note"}
	appTasks     = pocketApp{"tasks", "Pocket Tasks", []string{scopeTasksWrite}, "Turn an escape room's setting-up list (hide the clues, buy padlocks) into tasks"}
	appInventory = pocketApp{"inventory", "Pocket Inventory", []string{scopeItemsRead}, "Find the props you already have (padlocks, torches, boxes) for an escape room"}
	appPlaces    = pocketApp{"places", "Pocket Places", []string{scopePlacesRead}, "Use your saved places as the stops of a treasure hunt"}
	appMap       = pocketApp{"map", "Pocket Map", []string{scopeMapWrite}, "Put a treasure hunt's stops on your map"}
	appLearn     = pocketApp{"learn", "Pocket Learn", []string{scopeLearnRead, scopeLearnWrite}, "Turn a puzzle's words and clues into flashcards to learn them"}
	appBookshelf = pocketApp{"bookshelf", "Pocket Bookshelf", []string{scopeBooksRead}, "Turn a quote you saved from a book into a secret code"}
	appHabits    = pocketApp{"habits", "Pocket Habits", []string{scopeHabitsRead, scopeEntriesWrite}, "Tick your daily-puzzle habit when you solve today's puzzle"}
	appQuest     = pocketApp{"quest", "Pocket Quest", []string{scopeQuestRead, scopeQuestWrite}, "Hand a puzzle to your tabletop players: add it to a campaign as a handout"}
	appBlog      = pocketApp{"blog", "Pocket Blog", []string{scopeBlogWrite}, "Start a blog post with a puzzle and the link to play it"}
	appFamily    = pocketApp{"family", "Pocket Family", []string{scopeFamilyRead, scopeFamilyWrite}, "Make puzzles for the children by name, and pin a puzzle to the family board"}
	appTranslate = pocketApp{"translate", "Pocket Translate", []string{scopeTranslate}, "Translate a puzzle's clues and questions into another language"}
	appAlerts    = pocketApp{"alerts", "Pocket Alerts", []string{scopeAlertsWrite}, "Tell you when someone finishes a puzzle or an escape room you shared"}
	pocketApps   = []pocketApp{appCalendar, appContacts, appChat, appEmail, appPhotos, appFiles, appNotes, appTasks, appInventory, appPlaces, appMap,
		appLearn, appBookshelf, appHabits, appQuest, appBlog, appFamily, appTranslate, appAlerts}
)

func appBySlug(slug string) (pocketApp, bool) {
	for _, a := range pocketApps {
		if a.slug == slug {
			return a, true
		}
	}
	return pocketApp{}, false
}

// linkUses is the manifest's Uses.
func linkUses() []core.Use {
	out := make([]core.Use, 0, len(pocketApps))
	for _, a := range pocketApps {
		out = append(out, core.Use{App: a.slug, Scopes: a.scopes, Reason: a.reason})
	}
	return out
}

// Connection modes, chosen per person and per app in Settings.
const (
	modeCore   = "core"
	modeDirect = "direct"
	modeOff    = "off"
)

// connMode is the mode in effect: the chosen one, else direct when a token
// was entered, else Pocket Core when Puzzlecraft is connected to one, else direct.
func (s *Server) connMode(mode, token string) string {
	switch mode {
	case modeCore, modeDirect, modeOff:
		return mode
	}
	if token != "" || !s.core.Configured() {
		return modeDirect
	}
	return modeCore
}

// notConnected means the person has no working connection to the app.
type notConnected struct{ msg string }

func (e *notConnected) Error() string { return e.msg }

func isNotConnected(err error) bool {
	var nc *notConnected
	return errors.As(err, &nc)
}

// siblingConn is how Puzzlecraft reaches one person's copy of a sibling app.
type siblingConn struct {
	client *pocket.Client
	via    string
	public string
}

func linkReason(app, reason string) string {
	switch reason {
	case "off_person":
		return "you switched the Puzzlecraft → " + app + " link off in Pocket Core (Settings → Apps acting for me)"
	case "off_admin":
		return "the Puzzlecraft → " + app + " link is switched off in Pocket Core (Links)"
	case "target_missing":
		return app + " is not registered with Pocket Core"
	case "not_offered":
		return "this " + app + " version does not accept app links yet"
	case "not_declared":
		return "Pocket Core has not seen this Puzzlecraft version yet; it re-registers within a minute of starting"
	case "not_in_audience":
		return "you do not have access to " + app + " in Pocket Core"
	case "unknown_user":
		return "your email is not a person in Pocket Core"
	}
	return ""
}

// connSettings returns a person's saved mode, URL, token and public URL for an app.
func connSettings(st store.Settings, a pocketApp) (mode, url, token, public string) {
	c := st.Conns[a.slug]
	return c.Mode, c.URL, c.Token, c.PublicURL
}

// siblingFor connects to app a for person u with exactly these scopes.
func (s *Server) siblingFor(ctx context.Context, u *store.User, st store.Settings, a pocketApp, scopes ...string) (*siblingConn, error) {
	if u == nil {
		return nil, &notConnected{"nobody to act for"}
	}
	mode, url, token, public := connSettings(st, a)
	switch s.connMode(mode, token) {
	case modeOff:
		return nil, &notConnected{a.name + " is switched off in Settings → Connected apps"}
	case modeDirect:
		if token == "" {
			return nil, &notConnected{a.name + " is not set up (Settings → Connected apps): add a URL and token, or switch to Pocket Core"}
		}
		base := url
		if base == "" {
			base = s.resolveSibling(ctx, a.slug)
		}
		if base == "" {
			return nil, &notConnected{"set the " + a.name + " URL in Settings → Connected apps"}
		}
		return &siblingConn{client: pocket.New(base, token, 15*time.Second), via: modeDirect, public: firstNonEmpty(public, base)}, nil
	}
	if !s.core.Configured() {
		return nil, &notConnected{a.name + " through Pocket Core: there is no Pocket Core on this box; use a direct connection"}
	}
	tok, err := s.core.ActAs(ctx, u.Email, a.slug, scopes...)
	if err != nil {
		var le *core.LinkError
		if errors.As(err, &le) {
			if t := linkReason(a.name, le.Reason); t != "" {
				return nil, &notConnected{a.name + " is not set up: " + t}
			}
			return nil, &notConnected{a.name + " is not set up: " + le.Error()}
		}
		return nil, fmt.Errorf("%s through Pocket Core: %w", a.name, err)
	}
	if tok.URL == "" || tok.AccessToken == "" {
		return nil, fmt.Errorf("%s through Pocket Core: Core gave no address or token", a.name)
	}
	return &siblingConn{client: pocket.New(tok.URL, tok.AccessToken, 15*time.Second), via: modeCore, public: firstNonEmpty(public, firstNonEmpty(tok.PublicURL, tok.URL))}, nil
}

// connStatus says, without issuing anything, how a person reaches an app.
func (s *Server) connStatus(ctx context.Context, u *store.User, st store.Settings, a pocketApp) (eff, via, detail string) {
	as := s.appStatus(ctx, u, st, a)
	return as.Mode, as.Via, as.Detail
}

// appState is how one person reaches one app, in words a person can act on.
type appState struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
	Mode   string `json:"mode"` // core | direct | off
	Via    string `json:"via"`  // core | direct | "" (does not work)
	// State: on | partly | off (switched off here) | link_off (switched off
	// in Pocket Core) | missing (the app is not on this box) | old (the app
	// is too old for this) | no_core | not_set_up | error
	State  string   `json:"state"`
	Detail string   `json:"detail"`
	Fix    string   `json:"fix"`
	On     []string `json:"on"` // the scopes that work
	Open   string   `json:"open_url"`
}

// Can reports whether a scope works.
func (a appState) Can(scope string) bool {
	for _, s := range a.On {
		if s == scope {
			return true
		}
	}
	return false
}

func stateOf(reason string) (state, fix string) {
	switch reason {
	case "off_person":
		return "link_off", "Switch it on in Pocket Core → Settings → Apps acting for me"
	case "off_admin":
		return "link_off", "Switch the link on in Pocket Core → Links"
	case "target_missing":
		return "missing", "Install it and connect it to Pocket Core; until then Puzzlecraft keeps working without it"
	case "not_offered":
		return "old", "Update it to the newest version"
	case "not_declared":
		return "error", "Wait a minute: Puzzlecraft re-registers with Pocket Core shortly after it starts"
	case "not_in_audience", "unknown_user":
		return "error", "Ask whoever runs Pocket Core to give you access"
	}
	return "error", ""
}

// appStatus works out an appState (cached for half a minute per person).
func (s *Server) appStatus(ctx context.Context, u *store.User, st store.Settings, a pocketApp) appState {
	mode, _, token, public := connSettings(st, a)
	out := appState{Slug: a.slug, Name: a.name, Reason: a.reason, Mode: s.connMode(mode, token), On: []string{}}
	switch out.Mode {
	case modeOff:
		out.State, out.Detail, out.Fix = "off", "switched off", "Switch it on in Settings → Connected apps"
		return out
	case modeDirect:
		if token == "" {
			out.State, out.Detail = "not_set_up", "no token yet"
			out.Fix = "Add its address and an API token in Settings → Connected apps"
			if !s.core.Configured() {
				out.Fix += " (or connect Puzzlecraft to Pocket Core: then there is nothing to paste)"
			}
			return out
		}
		out.Via, out.State, out.On, out.Open = modeDirect, "on", append([]string(nil), a.scopes...), public
		return out
	}
	if u == nil || !s.core.Configured() {
		out.State, out.Detail, out.Fix = "no_core", "there is no Pocket Core on this box", "Use a direct connection (Settings → Connected apps → Advanced)"
		return out
	}
	key := fmt.Sprintf("%d %s", u.ID, a.slug)
	s.statusMu.Lock()
	if c, ok := s.statusCache[key]; ok && time.Since(c.at) < 30*time.Second {
		s.statusMu.Unlock()
		return c.st
	}
	s.statusMu.Unlock()
	defer func() {
		if out.State != "error" || out.Detail == "" || !strings.HasPrefix(out.Detail, "Pocket Core did not answer") {
			s.statusMu.Lock()
			s.statusCache[key] = cachedState{st: out, at: time.Now()}
			s.statusMu.Unlock()
		}
	}()
	list, err := s.core.LinkStatus(ctx, u.Email, a.slug)
	if err != nil {
		out.State, out.Detail, out.Fix = "error", "Pocket Core did not answer: "+err.Error(), "Try again in a moment"
		return out
	}
	state := map[string]core.LinkState{}
	for _, l := range list {
		state[l.Scope] = l
	}
	var on, off []string
	reason := ""
	for _, sc := range a.scopes {
		l, ok := state[sc]
		if ok && l.On {
			on = append(on, sc)
			continue
		}
		off = append(off, sc)
		if reason == "" {
			reason = "not_declared"
			if ok {
				reason = l.State
			}
		}
	}
	text := linkReason(a.name, reason)
	if text == "" {
		text = reason
	}
	out.On = on
	if out.On == nil {
		out.On = []string{}
	}
	out.Open = firstNonEmpty(public, s.resolvePublic(ctx, a.slug))
	switch {
	case len(off) == 0:
		out.Via, out.State = modeCore, "on"
		return out
	case len(on) == 0:
		out.State, out.Fix = stateOf(reason)
		out.Detail = text
		return out
	}
	sort.Strings(off)
	out.Via, out.State = modeCore, "partly"
	_, out.Fix = stateOf(reason)
	out.Detail = "partly: " + strings.Join(off, ", ") + " off (" + text + ")"
	return out
}

type cachedState struct {
	st appState
	at time.Time
}

// forgetStatus drops a person's cached app states (their settings changed).
func (s *Server) forgetStatus(uid int64) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	for k := range s.statusCache {
		if strings.HasPrefix(k, fmt.Sprintf("%d ", uid)) {
			delete(s.statusCache, k)
		}
	}
}

// clients carries one person's settings and their connections for one request.
type clients struct {
	s      *Server
	u      *store.User
	st     store.Settings
	notify *notify.Client
	conns  map[string]connResult
	states map[string]appState
}

type connResult struct {
	c   *siblingConn
	err error
}

// clients builds the connections from ONE user's own settings.
func (s *Server) clients(ctx context.Context, uid int64) *clients {
	st, _ := s.store.GetSettings(ctx, uid)
	u := userFrom(ctx)
	if u == nil || u.ID != uid {
		u, _ = s.store.GetUser(ctx, uid)
	}
	return &clients{s: s, u: u, st: st, notify: notify.New(st.NotifyURL, st.NotifyFormat), conns: map[string]connResult{}}
}

func (c *clients) conn(ctx context.Context, a pocketApp, scopes ...string) (*siblingConn, error) {
	sc := append([]string(nil), scopes...)
	sort.Strings(sc)
	key := a.slug + " " + strings.Join(sc, " ")
	if r, ok := c.conns[key]; ok {
		return r.c, r.err
	}
	sib, err := c.s.siblingFor(ctx, c.u, c.st, a, scopes...)
	c.conns[key] = connResult{sib, err}
	return sib, err
}

// status is a person's appState for one app, once per request.
func (c *clients) status(ctx context.Context, a pocketApp) appState {
	if c.states == nil {
		c.states = map[string]appState{}
	}
	if st, ok := c.states[a.slug]; ok {
		return st
	}
	st := c.s.appStatus(ctx, c.u, c.st, a)
	c.states[a.slug] = st
	return st
}

// optional turns "not connected" into (nil, nil).
func optional(sib *siblingConn, err error) (*siblingConn, error) {
	if isNotConnected(err) {
		return nil, nil
	}
	return sib, err
}
