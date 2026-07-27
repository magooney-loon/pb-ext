package audit

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func newTestAuditor(t testing.TB, opts ...Option) *Auditor {
	t.Helper()
	// No app: New does not touch a database, so classification, buffering and
	// the bounds can all be exercised without one.
	a := New(nil, opts...)
	return a
}

func TestTrack_AggregatesRepeatsOfTheSameEvent(t *testing.T) {
	a := newTestAuditor(t)

	for range 5 {
		a.Track(Event{
			At:        time.Now(),
			Kind:      KindAdminUI,
			Method:    "GET",
			Path:      "/_/",
			Status:    200,
			Outcome:   OutcomeAllowed,
			AuthState: AuthAnonymous,
			IP:        "203.0.113.7",
		})
	}

	if got := a.Pending(); got != 1 {
		t.Fatalf("Pending = %d, want 5 identical events collapsed into 1", got)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	for _, agg := range a.pending {
		if agg.Count != 5 {
			t.Fatalf("Count = %d, want 5", agg.Count)
		}
	}
}

func TestTrack_KeepsDistinctEventsApart(t *testing.T) {
	a := newTestAuditor(t)

	a.Track(Event{Kind: KindAdminUI, Path: "/_/", IP: "203.0.113.7"})
	a.Track(Event{Kind: KindAdminUI, Path: "/_/", IP: "198.51.100.2"})
	a.Track(Event{Kind: KindAdminUI, Path: "/_/settings", IP: "203.0.113.7"})

	if got := a.Pending(); got != 3 {
		t.Fatalf("Pending = %d, want 3 distinct events", got)
	}
}

// At the ceiling, a flood against one path must keep being counted exactly —
// that flood is the reason the ceiling was reached, so losing its count is
// losing the thing worth knowing.
func TestTrack_CeilingRefusesNewKeysButKeepsCountingKnownOnes(t *testing.T) {
	a := newTestAuditor(t, WithMaxPendingEvents(10))

	for i := range 10 {
		a.Track(Event{Kind: KindAdminUI, Path: fmt.Sprintf("/_/%d", i), IP: "203.0.113.7"})
	}
	if got := a.Pending(); got != 10 {
		t.Fatalf("Pending = %d, want the buffer filled to 10", got)
	}

	// A new distinct key is refused.
	a.Track(Event{Kind: KindAdminUI, Path: "/_/overflow", IP: "203.0.113.7"})
	if got := a.Pending(); got != 10 {
		t.Fatalf("Pending = %d, want the ceiling held", got)
	}
	a.mu.Lock()
	dropped := a.dropped
	a.mu.Unlock()
	if dropped != 1 {
		t.Fatalf("dropped = %d, want 1", dropped)
	}

	// An already-known key keeps aggregating for free.
	for range 1000 {
		a.Track(Event{Kind: KindAdminUI, Path: "/_/0", IP: "203.0.113.7"})
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.dropped != 1 {
		t.Fatalf("dropped = %d, want the flood on a known key to cost nothing", a.dropped)
	}
	for key, agg := range a.pending {
		if key.Path == "/_/0" && agg.Count != 1001 {
			t.Fatalf("Count for the flooded path = %d, want 1001", agg.Count)
		}
	}
}

// An unbounded user agent would let one client mint unlimited distinct keys.
func TestTrack_TruncatesAttackerControlledFields(t *testing.T) {
	a := newTestAuditor(t, WithMaxPendingEvents(100))

	huge := strings.Repeat("A", 50000)
	a.Track(Event{Kind: KindAdminUI, Path: "/_/" + huge, UserAgent: huge, Identity: huge, IP: "203.0.113.7"})

	a.mu.Lock()
	defer a.mu.Unlock()
	for key := range a.pending {
		if len([]rune(key.Path)) > a.cfg.MaxFieldLength+1 {
			t.Errorf("path stored at %d runes, want it capped", len([]rune(key.Path)))
		}
		if len([]rune(key.UserAgent)) > a.cfg.MaxFieldLength+1 {
			t.Errorf("user agent stored at %d runes, want it capped", len([]rune(key.UserAgent)))
		}
		if len([]rune(key.Identity)) > 256 {
			t.Errorf("identity stored at %d runes, want it capped", len([]rune(key.Identity)))
		}
	}
}

func TestTrack_HonoursThePersonalDataPolicy(t *testing.T) {
	a := newTestAuditor(t, WithPersonalData(false, false, false))

	a.Track(Event{
		Kind:      KindAuthFailure,
		Path:      "/api/collections/_superusers/auth-with-password",
		IP:        "203.0.113.7",
		UserAgent: "curl/8.0",
		Identity:  "admin@example.com",
	})

	a.mu.Lock()
	defer a.mu.Unlock()
	for key := range a.pending {
		if key.IP != "" || key.UserAgent != "" || key.Identity != "" {
			t.Fatalf("personal data retained despite the policy: %+v", key)
		}
		if key.Path == "" {
			t.Fatal("the non-identifying fields were dropped too")
		}
	}
}

func TestTrack_IsANoOpWhenNotRecording(t *testing.T) {
	a := newTestAuditor(t, WithEnabled(false))
	a.Track(Event{Kind: KindAdminUI, Path: "/_/"})

	if got := a.Pending(); got != 0 {
		t.Fatalf("Pending = %d on a disabled auditor", got)
	}

	// And a nil auditor must be safe, since Get can hand one out.
	var nilAuditor *Auditor
	nilAuditor.Track(Event{Kind: KindAdminUI})
	if nilAuditor.Recording() {
		t.Fatal("a nil auditor reports that it is recording")
	}
	if nilAuditor.Data() == nil {
		t.Fatal("Data returned nil")
	}
}

func TestGet_NeverReturnsNil(t *testing.T) {
	if Get() == nil {
		t.Fatal("Get returned nil")
	}
}

// --- classification ---

func TestClassify_SelectsTheAdminSurfaces(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		wantKind  string
		wantTrack bool
	}{
		{"pb-ext dashboard", "/_/_", KindDashboard, true},
		{"pocketbase admin ui", "/_/", KindAdminUI, true},
		{"pocketbase admin ui page", "/_/settings", KindAdminUI, true},
		{"bare underscore", "/_", KindAdminUI, true},
		{"admin ui script", "/_/main.js", "", false},
		{"admin ui stylesheet", "/_/assets/index.css", "", false},
		{"admin ui font", "/_/fonts/x.woff2", "", false},
		{"public page", "/about", "", false},
		{"public api", "/api/collections/todos/records", "", false},
		{"superuser collection", "/api/collections/_superusers/auth-with-password", KindAdminAPI, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestAuditor(t)
			kind, tracked := a.classifyKind(tc.path, AuthAnonymous)

			if tracked != tc.wantTrack {
				t.Fatalf("tracked = %v, want %v", tracked, tc.wantTrack)
			}
			if tracked && kind != tc.wantKind {
				t.Fatalf("kind = %q, want %q", kind, tc.wantKind)
			}
		})
	}
}

func TestClassify_SuperuserAPICallsAreRecorded(t *testing.T) {
	a := newTestAuditor(t)

	if _, tracked := a.classifyKind("/api/collections/todos/records", AuthAnonymous); tracked {
		t.Fatal("an anonymous API call was recorded as admin access")
	}

	kind, tracked := a.classifyKind("/api/collections/todos/records", AuthSuperuser)
	if !tracked || kind != KindAdminAPI {
		t.Fatalf("kind = %q tracked = %v, want a recorded admin_api event", kind, tracked)
	}
}

// An unauthenticated GET of /_/_ renders the login screen and returns 200. By
// status alone every anonymous probe would read as "allowed"; it is a denial.
func TestOutcomeOf_AnonymousDashboardHitsAreDenials(t *testing.T) {
	if got := outcomeOf(KindDashboard, http.StatusOK, AuthAnonymous); got != OutcomeDenied {
		t.Fatalf("outcome = %q, want %q", got, OutcomeDenied)
	}
	if got := outcomeOf(KindDashboard, http.StatusOK, AuthUser); got != OutcomeDenied {
		t.Fatalf("outcome for a non-superuser = %q, want %q", got, OutcomeDenied)
	}
	if got := outcomeOf(KindDashboard, http.StatusOK, AuthSuperuser); got != OutcomeAllowed {
		t.Fatalf("outcome for a superuser = %q, want %q", got, OutcomeAllowed)
	}
	if got := outcomeOf(KindAdminAPI, http.StatusUnauthorized, AuthAnonymous); got != OutcomeDenied {
		t.Fatalf("outcome for a 401 = %q, want %q", got, OutcomeDenied)
	}
	if got := outcomeOf(KindAdminAPI, http.StatusInternalServerError, AuthSuperuser); got != OutcomeFailed {
		t.Fatalf("outcome for a 500 = %q, want %q", got, OutcomeFailed)
	}
}

// Tokens travel in the query string, and an audit log is a place people paste
// into tickets.
func TestRedactQuery_HidesCredentials(t *testing.T) {
	cases := map[string]string{
		"filter=x&token=abc123":         "filter=x&token=<redacted>",
		"TOKEN=abc123":                  "TOKEN=<redacted>",
		"password=hunter2&page=1":       "password=<redacted>&page=1",
		"access_token=zzz":              "access_token=<redacted>",
		"filter=name~'a'&sort=-created": "filter=name~'a'&sort=-created",
		"":                              "",
		"malformed&&token=abc":          "malformed&&token=<redacted>",
	}

	for in, want := range cases {
		if got := redactQuery(in); got != want {
			t.Errorf("redactQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactQuery_NeverKeepsASecretValue(t *testing.T) {
	got := redactQuery("token=SUPERSECRETVALUE&x=1")
	if strings.Contains(got, "SUPERSECRETVALUE") {
		t.Fatalf("the token value survived redaction: %q", got)
	}
}

func TestIsAdminAsset(t *testing.T) {
	assets := []string{"/_/main.js", "/_/x.CSS", "/_/logo.svg", "/_/f.woff2", "/_/i.png"}
	pages := []string{"/_/", "/_/settings", "/_/collections", "/_/_"}

	for _, p := range assets {
		if !isAdminAsset(p) {
			t.Errorf("isAdminAsset(%q) = false, want true", p)
		}
	}
	for _, p := range pages {
		if isAdminAsset(p) {
			t.Errorf("isAdminAsset(%q) = true, want false", p)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abc", 10); got != "abc" {
		t.Errorf("truncate short string = %q", got)
	}
	if got := truncate("abcdef", 3); got != "abc…" {
		t.Errorf("truncate = %q, want the cut marked", got)
	}
	if got := truncate("日本語のテキスト", 3); len([]rune(got)) != 4 {
		t.Errorf("truncate counted bytes rather than runes: %q", got)
	}
	if got := truncate("abc", 0); got != "" {
		t.Errorf("truncate with a zero cap = %q, want empty", got)
	}
}
