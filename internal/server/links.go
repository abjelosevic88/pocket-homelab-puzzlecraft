package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// Pocket Core app links (docs/links.md in pocket-homelab-core), Puzzlecraft as
// the target: another Pocket app the person allowed in Core may act here for
// them with a pcx_ token, only within these scopes. Every route not listed
// in linkRoutes stays closed to it (403).

var linkOffers = []core.Offer{
	{Scope: "puzzlecraft:read", Name: "See your puzzles", Description: "Read your puzzles (crosswords, word searches, sudoku, codes, quizzes and the rest — what they are and how they were played, without the answers), your puzzle books, escape rooms and treasure hunts, word lists, the puzzles of the day and your solving times — for Pocket AI, search and your journal."},
	{Scope: "puzzlecraft:write", Name: "Make puzzles", Description: "Make a puzzle (from words and clues, a text, or \"make one for me\"), add a word list, and change or remove only the puzzles and lists it added itself."},
}

type linkRoute struct {
	methods string
	path    *regexp.Regexp
	scope   string // "" = any linked app
}

var linkRoutes = []linkRoute{
	{"GET", regexp.MustCompile(`^/api/v1/auth/me$`), ""},
	{"GET", regexp.MustCompile(`^/api/v1/(summary|home|search|stats|today|kinds|themes|puzzles|books|adventures|wordlists|words|day/\d{4}-\d{2}-\d{2})$`), "puzzlecraft:read"},
	{"GET", regexp.MustCompile(`^/api/v1/(puzzles|books|adventures|wordlists)/\d+$`), "puzzlecraft:read"},
	{"GET", regexp.MustCompile(`^/api/v1/(puzzles|adventures)/\d+/plays$`), "puzzlecraft:read"},
	{"POST", regexp.MustCompile(`^/api/v1/(puzzles|wordlists|generate)$`), "puzzlecraft:write"},
	{"PUT DELETE", regexp.MustCompile(`^/api/v1/(puzzles|wordlists)/\d+$`), "puzzlecraft:write"},
	{"POST", regexp.MustCompile(`^/api/v1/puzzles/\d+/(share|test)$`), "puzzlecraft:write"},
}

var errLinkScope = errors.New("outside the link's scope")

// linkAllows reports whether a link token with these scopes may make this request.
func linkAllows(scopes []string, r *http.Request) bool {
	for _, lr := range linkRoutes {
		if !strings.Contains(" "+lr.methods+" ", " "+r.Method+" ") || !lr.path.MatchString(r.URL.Path) {
			continue
		}
		if lr.scope == "" {
			return true
		}
		for _, sc := range scopes {
			if sc == lr.scope {
				return true
			}
		}
	}
	return false
}

// linkUser validates a pcx_ token, checks the route against its scopes and
// maps it to the person it acts for. The second value is the acting app's slug.
func (s *Server) linkUser(ctx context.Context, tok string, r *http.Request) (*store.User, string, error) {
	in, err := s.core.Introspect(ctx, tok)
	if err != nil {
		return nil, "", err
	}
	if !in.Active || !in.Delegated() || in.Audience != "puzzlecraft" || in.User == nil || in.User.Email == "" || in.App == nil || in.App.Slug == "" {
		return nil, "", http.ErrNoCookie
	}
	if !linkAllows(strings.Fields(in.Scope), r) {
		return nil, "", errLinkScope
	}
	u, err := s.coreIdentityUser(ctx, in)
	return u, in.App.Slug, err
}

const linkAppKey ctxKey = 2

// linkApp is the acting app's slug when the request came in on a link token.
func linkApp(ctx context.Context) string {
	a, _ := ctx.Value(linkAppKey).(string)
	return a
}

// withLinkApp marks a context as acting for another app (tests, and the auth middleware).
func withLinkApp(ctx context.Context, slug string) context.Context {
	return context.WithValue(ctx, linkAppKey, slug)
}
