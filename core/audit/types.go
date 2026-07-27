package audit

import (
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
)

// TableName is the admin access log. It lives in auxiliary.db, not data.db, and
// is a plain SQLite table rather than a PocketBase collection — see schema.go.
const TableName = "_admin_access"

// Event kinds.
const (
	// KindDashboard is a request to pb-ext's own dashboard at /_/_.
	KindDashboard = "pbext_dashboard"
	// KindAdminUI is a request to PocketBase's admin UI under /_/.
	KindAdminUI = "admin_ui"
	// KindAdminAPI is an API call carrying superuser authentication, or one
	// targeting the _superusers collection. This is the record of what an
	// administrator actually did.
	KindAdminAPI = "admin_api"
	// KindAuthSuccess is a successful superuser authentication, by any method.
	KindAuthSuccess = "auth_success"
	// KindAuthFailure is a rejected superuser password attempt. It is the only
	// place the attempted identity is recorded — PocketBase's own request log
	// sees a 400 and never learns which account was targeted.
	KindAuthFailure = "auth_failure"
)

// Outcomes.
const (
	// OutcomeAllowed means the request reached what it asked for.
	OutcomeAllowed = "allowed"
	// OutcomeDenied means authentication or authorization refused it. The
	// pb-ext dashboard is included: an unauthenticated GET renders the login
	// screen with a 200, which is a denial however it is numbered.
	OutcomeDenied = "denied"
	// OutcomeFailed means the request errored for some other reason.
	OutcomeFailed = "failed"
)

// Auth states.
const (
	AuthAnonymous = "anonymous"
	AuthSuperuser = "superuser"
	// AuthUser is an authenticated non-superuser. One of these touching an
	// admin route is somebody testing what their token can reach.
	AuthUser = "user"
)

// Event is one observed admin access. It is assembled on the request path and
// handed to the buffer; nothing here touches a database.
type Event struct {
	At         time.Time
	Kind       string
	Method     string
	Path       string
	Query      string
	Status     int
	Outcome    string
	AuthState  string
	Identity   string
	IP         string
	UserAgent  string
	Referer    string
	TraceID    string
	DurationMs float64
	Error      string
}

// Record is one row of the access log, as read back for the dashboard and the
// API. Repeated identical events inside a flush window collapse into a single
// row with Count above 1.
type Record struct {
	Created    time.Time `json:"created"`
	LastSeen   time.Time `json:"last_seen"`
	Kind       string    `json:"kind"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Query      string    `json:"query,omitempty"`
	Status     int       `json:"status"`
	Outcome    string    `json:"outcome"`
	AuthState  string    `json:"auth_state"`
	Identity   string    `json:"identity,omitempty"`
	IP         string    `json:"ip,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	Referer    string    `json:"referer,omitempty"`
	TraceID    string    `json:"trace_id,omitempty"`
	DurationMs float64   `json:"duration_ms"`
	Error      string    `json:"error,omitempty"`
	Count      int       `json:"count"`
}

// Stats summarises the recent window for the dashboard card.
type Stats struct {
	Enabled bool `json:"enabled"`
	// Pending is how many events are buffered but not yet written.
	Pending int `json:"pending"`
	// Dropped counts events lost to a full buffer since startup.
	Dropped uint64 `json:"dropped"`
	Written uint64 `json:"written"`

	// The counts below cover SummaryWindowDays.
	TotalEvents    int `json:"total_events"`
	AuthFailures   int `json:"auth_failures"`
	AuthSuccesses  int `json:"auth_successes"`
	DeniedAttempts int `json:"denied_attempts"`
	DistinctIPs    int `json:"distinct_ips"`

	LastFailure time.Time `json:"last_failure"`
	LastSuccess time.Time `json:"last_success"`
}

// IPSummary is a per-source rollup, newest activity first.
type IPSummary struct {
	IP        string    `json:"ip"`
	Events    int       `json:"events"`
	Failures  int       `json:"failures"`
	Successes int       `json:"successes"`
	LastSeen  time.Time `json:"last_seen"`
}

// Data is the dashboard payload.
type Data struct {
	Stats
	Recent []Record    `json:"recent"`
	TopIPs []IPSummary `json:"top_ips"`
}

// DefaultData returns an empty payload, for when auditing is not running.
func DefaultData() *Data {
	return &Data{Recent: []Record{}, TopIPs: []IPSummary{}}
}

// Display and query limits.
const (
	// RecentLimit is how many rows the dashboard card reads back.
	RecentLimit = 50
	// TopIPsLimit is how many distinct sources the summary rolls up.
	TopIPsLimit = 10
	// SummaryWindowDays is the window the Stats counters cover.
	SummaryWindowDays = 7
)

// Defaults for Config. Override with the With* options.
const (
	DefaultFlushInterval       = 5 * time.Second
	DefaultMaxPendingEvents    = 5000
	DefaultRetentionDays       = 90
	DefaultMaxFieldLength      = 2000
	DefaultBruteForceThreshold = 5
	DefaultBruteForceWindow    = 10 * time.Minute
)

// Config tunes what is captured and for how long.
//
// The privacy defaults here are the opposite of core/analytics, deliberately.
// Analytics answers "how is the site used" and needs no identities, so it keeps
// none. This package answers "who tried to get into the admin panel", which is
// unanswerable without the client address, the user agent and the account that
// was targeted. Each is individually switchable for deployments that must not
// retain them, and everything is deleted after RetentionDays.
type Config struct {
	// Enabled gates the whole subsystem.
	Enabled bool

	// FlushInterval is how often buffered events are written.
	FlushInterval time.Duration
	// MaxPendingEvents bounds the buffer. Hitting it triggers an early flush;
	// events beyond it are dropped and counted rather than blocking a request.
	MaxPendingEvents int
	// RetentionDays is how long rows are kept.
	RetentionDays int
	// MaxFieldLength truncates attacker-controlled strings — paths, user
	// agents, referers, attempted identities — before they are stored.
	MaxFieldLength int

	// TrackAdminUI records requests to /_/ and /_/_.
	TrackAdminUI bool
	// TrackAdminAPI records superuser-authenticated API calls.
	TrackAdminAPI bool
	// TrackAuth records superuser authentication successes and failures.
	TrackAuth bool

	// RecordIP stores the client address, honouring the admin-configured
	// TrustedProxy settings.
	RecordIP bool
	// RecordUserAgent stores the user agent string.
	RecordUserAgent bool
	// RecordIdentity stores the account an authentication attempt targeted.
	RecordIdentity bool

	// AlertOnFailure raises an alert for each rejected superuser login.
	AlertOnFailure bool
	// AlertOnNewIP raises an alert when a superuser signs in from an address
	// with no successful sign-in on record.
	AlertOnNewIP bool
	// BruteForceThreshold is how many failures from one address inside
	// BruteForceWindow escalate to a critical alert.
	BruteForceThreshold int
	BruteForceWindow    time.Duration

	// notifier is set by WithNotifier rather than by callers filling a literal.
	notifier *alerts.Notifier
}

// DefaultConfig returns the default configuration: auditing on, everything
// captured, 90-day retention.
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		FlushInterval:       DefaultFlushInterval,
		MaxPendingEvents:    DefaultMaxPendingEvents,
		RetentionDays:       DefaultRetentionDays,
		MaxFieldLength:      DefaultMaxFieldLength,
		TrackAdminUI:        true,
		TrackAdminAPI:       true,
		TrackAuth:           true,
		RecordIP:            true,
		RecordUserAgent:     true,
		RecordIdentity:      true,
		AlertOnFailure:      true,
		AlertOnNewIP:        true,
		BruteForceThreshold: DefaultBruteForceThreshold,
		BruteForceWindow:    DefaultBruteForceWindow,
	}
}

// normalize replaces non-positive fields with their defaults so a partially
// filled Config is always usable.
func (c *Config) normalize() {
	d := DefaultConfig()

	if c.FlushInterval <= 0 {
		c.FlushInterval = d.FlushInterval
	}
	if c.MaxPendingEvents <= 0 {
		c.MaxPendingEvents = d.MaxPendingEvents
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = d.RetentionDays
	}
	if c.MaxFieldLength <= 0 {
		c.MaxFieldLength = d.MaxFieldLength
	}
	if c.BruteForceThreshold <= 0 {
		c.BruteForceThreshold = d.BruteForceThreshold
	}
	if c.BruteForceWindow <= 0 {
		c.BruteForceWindow = d.BruteForceWindow
	}
}

// Option customizes an Auditor.
type Option func(*Config)

// WithEnabled turns auditing on or off.
func WithEnabled(v bool) Option {
	return func(c *Config) { c.Enabled = v }
}

// WithFlushInterval sets how often buffered events are written.
func WithFlushInterval(d time.Duration) Option {
	return func(c *Config) { c.FlushInterval = d }
}

// WithMaxPendingEvents bounds the in-memory buffer.
func WithMaxPendingEvents(n int) Option {
	return func(c *Config) { c.MaxPendingEvents = n }
}

// WithRetentionDays sets how long access records are kept.
func WithRetentionDays(n int) Option {
	return func(c *Config) { c.RetentionDays = n }
}

// WithTracking selects which classes of access are recorded.
func WithTracking(adminUI, adminAPI, auth bool) Option {
	return func(c *Config) {
		c.TrackAdminUI = adminUI
		c.TrackAdminAPI = adminAPI
		c.TrackAuth = auth
	}
}

// WithPersonalData selects which identifying fields are stored.
//
// Turning all three off leaves a usable record of *what* happened and when,
// with no record of who — enough for capacity questions, not enough for an
// intrusion investigation. That trade is the deployment's to make.
func WithPersonalData(ip, userAgent, identity bool) Option {
	return func(c *Config) {
		c.RecordIP = ip
		c.RecordUserAgent = userAgent
		c.RecordIdentity = identity
	}
}

// WithBruteForceAlert sets how many failures from one address inside a window
// escalate to a critical alert.
func WithBruteForceAlert(threshold int, window time.Duration) Option {
	return func(c *Config) {
		c.BruteForceThreshold = threshold
		c.BruteForceWindow = window
	}
}

// WithAlerts toggles the failed-login and new-address alerts.
func WithAlerts(onFailure, onNewIP bool) Option {
	return func(c *Config) {
		c.AlertOnFailure = onFailure
		c.AlertOnNewIP = onNewIP
	}
}

// WithNotifier routes alerts through a specific notifier instead of the package
// singleton. Tests use it to capture what would have been sent.
func WithNotifier(n *alerts.Notifier) Option {
	return func(c *Config) { c.notifier = n }
}
