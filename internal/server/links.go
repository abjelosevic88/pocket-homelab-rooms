package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// Pocket Core app links (docs/links.md in pocket-homelab-core), Rooms as
// the target: another Pocket app the person allowed in Core may act here for
// them with a pcx_ token, only within these scopes. Every route not listed
// in linkRoutes stays closed to it (403).

var linkOffers = []core.Offer{
	{Scope: "rooms:read", Name: "See your floor plans", Description: "Read your plans (rooms with their sizes and areas, doors, windows, the furniture on each layout and what is still to buy), your measured furniture, the ways in and the checklists, and check whether a piece fits through your doors — for Pocket AI, search, your journal and your other apps."},
	{Scope: "rooms:write", Name: "Add furniture and checklist items", Description: "Add a piece of furniture (with its size) to check or to place, start a plan, and add checklist items. It may change or remove only the pieces and items it added itself."},
}

type linkRoute struct {
	methods string
	path    *regexp.Regexp
	scope   string // "" = any linked app
}

var linkRoutes = []linkRoute{
	{"GET", regexp.MustCompile(`^/api/v1/auth/me$`), ""},
	{"GET", regexp.MustCompile(`^/api/v1/(summary|home|search|plans|pieces|todos|day/\d{4}-\d{2}-\d{2})$`), "rooms:read"},
	{"GET", regexp.MustCompile(`^/api/v1/(plans|pieces|layouts)/\d+$`), "rooms:read"},
	{"GET", regexp.MustCompile(`^/api/v1/plans/\d+/(summary|routes)$`), "rooms:read"},
	{"POST", regexp.MustCompile(`^/api/v1/fit$`), "rooms:read"},
	{"POST", regexp.MustCompile(`^/api/v1/(pieces|plans|todos)$`), "rooms:write"},
	{"POST", regexp.MustCompile(`^/api/v1/plans/\d+/todos$`), "rooms:write"},
	{"PUT DELETE", regexp.MustCompile(`^/api/v1/(pieces|todos)/\d+$`), "rooms:write"},
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
	if !in.Active || !in.Delegated() || in.Audience != "rooms" || in.User == nil || in.User.Email == "" || in.App == nil || in.App.Slug == "" {
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
