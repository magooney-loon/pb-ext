package audit

import (
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Route prefixes for the administrative surfaces.
const (
	// DashboardPath is pb-ext's own dashboard.
	DashboardPath = "/_/_"
	// AdminUIPrefix is PocketBase's admin UI.
	AdminUIPrefix = "/_/"
	// traceIDHeader mirrors logging.TraceIDHeader. It is duplicated rather than
	// imported because core/logging imports core/server, which imports this
	// package — taking the constant would be an import cycle.
	traceIDHeader = "X-Trace-ID"
)

// sensitiveQueryParams are redacted before a query string is stored. File and
// auth tokens travel in the query, and an audit log is a place people paste
// into tickets.
var sensitiveQueryParams = map[string]bool{
	"token":        true,
	"password":     true,
	"secret":       true,
	"access_token": true,
	"apikey":       true,
	"api_key":      true,
}

// adminUIAssets are the static file extensions the admin UI loads. The SPA
// pulls dozens per page view; recording them turns one administrator opening
// the panel into forty rows that say nothing.
var adminUIAssets = []string{
	".js", ".css", ".map", ".png", ".jpg", ".jpeg", ".svg", ".ico", ".webp",
	".woff", ".woff2", ".ttf", ".eot", ".otf", ".json", ".webmanifest",
}

// RegisterRoutes attaches the audit middleware and the authentication hooks.
//
// The middleware runs after the handler and does no I/O, so it adds a fixed
// sub-microsecond cost to the requests it watches and nothing at all to the
// rest.
func (a *Auditor) RegisterRoutes(e *core.ServeEvent) {
	if !a.Recording() {
		return
	}

	e.Router.BindFunc(func(re *core.RequestEvent) error {
		start := time.Now()

		err := re.Next()

		if ev, ok := a.classify(re, start, err); ok {
			a.Track(ev)
		}

		return err
	})
}

// RegisterHooks attaches the authentication hooks. They are separate from
// RegisterRoutes because they bind to the app rather than the router, and must
// be registered before serving starts.
func (a *Auditor) RegisterHooks(app core.App) {
	if !a.Recording() || !a.cfg.TrackAuth {
		return
	}

	// Failures. This hook is the only place the attempted identity is
	// observable: it arrives in the request body, and PocketBase's own log
	// records a 400 without it. Successes are handled by OnRecordAuthRequest
	// below, so this only reports what was rejected — otherwise a successful
	// password login would be counted twice.
	//
	// e.Password is deliberately never read. Nothing in this package touches it.
	app.OnRecordAuthWithPasswordRequest(core.CollectionNameSuperusers).BindFunc(
		func(e *core.RecordAuthWithPasswordRequestEvent) error {
			err := e.Next()
			if err == nil {
				return nil
			}

			a.Track(Event{
				At:        time.Now(),
				Kind:      KindAuthFailure,
				Method:    e.Request.Method,
				Path:      e.Request.URL.Path,
				Status:    statusOf(e.RequestEvent),
				Outcome:   OutcomeDenied,
				AuthState: AuthAnonymous,
				Identity:  e.Identity,
				IP:        e.RealIP(),
				UserAgent: e.Request.UserAgent(),
				Referer:   e.Request.Referer(),
				TraceID:   e.Request.Header.Get(traceIDHeader),
				Error:     err.Error(),
			})

			return err
		})

	// Successes, by any method — password, OAuth2, OTP, token refresh.
	app.OnRecordAuthRequest(core.CollectionNameSuperusers).BindFunc(
		func(e *core.RecordAuthRequestEvent) error {
			identity := ""
			if e.Record != nil {
				identity = e.Record.Email()
				if identity == "" {
					identity = e.Record.Id
				}
			}

			a.Track(Event{
				At:        time.Now(),
				Kind:      KindAuthSuccess,
				Method:    e.Request.Method,
				Path:      e.Request.URL.Path,
				Status:    statusOf(e.RequestEvent),
				Outcome:   OutcomeAllowed,
				AuthState: AuthSuperuser,
				Identity:  identity,
				IP:        e.RealIP(),
				UserAgent: e.Request.UserAgent(),
				Referer:   e.Request.Referer(),
				TraceID:   e.Request.Header.Get(traceIDHeader),
			})

			return e.Next()
		})
}

// classifyKind decides whether a path counts as administrative access.
//
// It is split out from classify so it can be tested directly: building a
// core.RequestEvent needs a live app, and the routing rules are the part most
// likely to be got wrong.
func (a *Auditor) classifyKind(path, authState string) (string, bool) {
	switch {
	case path == DashboardPath || strings.HasPrefix(path, DashboardPath+"/"):
		if !a.cfg.TrackAdminUI {
			return "", false
		}
		return KindDashboard, true

	case path == "/_" || strings.HasPrefix(path, AdminUIPrefix):
		if !a.cfg.TrackAdminUI || isAdminAsset(path) {
			return "", false
		}
		return KindAdminUI, true

	case authState == AuthSuperuser && strings.HasPrefix(path, "/api/"):
		// Everything a signed-in administrator does through the API. This is
		// the record of what was actually changed, as opposed to who knocked.
		if !a.cfg.TrackAdminAPI {
			return "", false
		}
		return KindAdminAPI, true

	case strings.Contains(path, "/"+core.CollectionNameSuperusers+"/"):
		// Unauthenticated traffic aimed at the superuser collection — password
		// resets, OTP requests, enumeration attempts.
		if !a.cfg.TrackAdminAPI {
			return "", false
		}
		return KindAdminAPI, true

	default:
		return "", false
	}
}

// classify describes a finished request, if it was administrative.
func (a *Auditor) classify(re *core.RequestEvent, start time.Time, err error) (Event, bool) {
	path := re.Request.URL.Path
	authState := authStateOf(re)

	kind, tracked := a.classifyKind(path, authState)
	if !tracked {
		return Event{}, false
	}

	status := statusOf(re)

	ev := Event{
		At:         start,
		Kind:       kind,
		Method:     re.Request.Method,
		Path:       path,
		Query:      redactQuery(re.Request.URL.RawQuery),
		Status:     status,
		Outcome:    outcomeOf(kind, status, authState),
		AuthState:  authState,
		Identity:   identityOf(re),
		IP:         re.RealIP(),
		UserAgent:  re.Request.UserAgent(),
		Referer:    re.Request.Referer(),
		TraceID:    re.Request.Header.Get(traceIDHeader),
		DurationMs: float64(time.Since(start).Microseconds()) / 1000,
	}
	if err != nil {
		ev.Error = err.Error()
	}

	return ev, true
}

// statusOf reports the status a request finished with. A zero status means the
// handler wrote a body without an explicit code, which net/http reports as 200.
func statusOf(re *core.RequestEvent) int {
	if s := re.Status(); s != 0 {
		return s
	}
	return http.StatusOK
}

func authStateOf(re *core.RequestEvent) string {
	switch {
	case re.Auth == nil:
		return AuthAnonymous
	case re.Auth.IsSuperuser():
		return AuthSuperuser
	default:
		return AuthUser
	}
}

// identityOf names the authenticated principal, preferring the email so the log
// is readable without a second lookup.
func identityOf(re *core.RequestEvent) string {
	if re.Auth == nil {
		return ""
	}
	if email := re.Auth.Email(); email != "" {
		return email
	}
	return re.Auth.Id
}

// outcomeOf interprets the result.
//
// The pb-ext dashboard needs the special case: an unauthenticated GET of /_/_
// renders the login screen and returns 200, so by status alone every anonymous
// probe would read as "allowed". It is a denial, and the log should say so.
func outcomeOf(kind string, status int, authState string) string {
	if kind == KindDashboard && authState != AuthSuperuser {
		return OutcomeDenied
	}

	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return OutcomeDenied
	case status >= 400:
		return OutcomeFailed
	default:
		return OutcomeAllowed
	}
}

// isAdminAsset reports whether a path under /_/ is a static asset of the SPA.
func isAdminAsset(path string) bool {
	lower := strings.ToLower(path)
	for _, ext := range adminUIAssets {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// redactQuery replaces the values of sensitive parameters, preserving the shape
// of the query so it stays useful for an investigation.
//
// It parses manually rather than with url.ParseQuery so that a malformed query
// — which is exactly what a probe sends — is still recorded rather than
// discarded, and so the original ordering survives.
func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}

	parts := strings.Split(raw, "&")
	for i, part := range parts {
		name, _, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		if sensitiveQueryParams[strings.ToLower(name)] {
			parts[i] = name + "=<redacted>"
		}
	}
	return strings.Join(parts, "&")
}
