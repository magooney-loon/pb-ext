package alerts

import (
	"context"
	"os"
	"strconv"
	"time"
)

// Alert storage and transport constants.
const (
	// TableName is the delivery log. It lives in auxiliary.db, not data.db, and
	// is a plain SQLite table rather than a PocketBase collection — see schema.go.
	TableName = "_alerts"

	// DefaultAPIBaseURL is Telegram's Bot API root. Tests override it with
	// WithAPIBaseURL so nothing in this package ever touches the network.
	DefaultAPIBaseURL = "https://api.telegram.org"

	// MessageLimit is Telegram's hard cap on a sendMessage text, in UTF-16 code
	// units. format.go budgets against a lower figure; see renderMessage.
	MessageLimit = 4096

	// RecentLimit is how many delivered alerts the dashboard card reads back.
	RecentLimit = 20
)

// Delivery outcomes recorded in the _alerts table.
//
// Cooldown suppressions are deliberately not recorded: writing a row for every
// suppressed alert would flood the table with exactly what the cooldown exists
// to prevent. They are counted in memory and reported in the hourly digest.
const (
	StatusSent    = "sent"
	StatusFailed  = "failed"
	StatusDropped = "dropped"
)

// Level is an alert's severity. It selects the emoji prefix, and nothing else —
// every level is delivered the same way.
type Level int8

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
	LevelCritical
)

// String returns the lowercase level name, as stored in the delivery log.
func (l Level) String() string {
	switch l {
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelCritical:
		return "critical"
	default:
		return "info"
	}
}

// Emoji is the level's prefix in a rendered message. It carries the severity
// into the notification preview, where the body is truncated away.
func (l Level) Emoji() string {
	switch l {
	case LevelWarn:
		return "⚠️"
	case LevelError:
		return "🔴"
	case LevelCritical:
		return "🚨"
	default:
		return "ℹ️"
	}
}

// ParseLevel maps a stored level name back to a Level, defaulting to LevelInfo.
func ParseLevel(s string) Level {
	switch s {
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	case "critical":
		return LevelCritical
	default:
		return LevelInfo
	}
}

// Message is one outbound alert.
//
// Text and Fields are rendered as HTML by format.go, so callers pass plain
// strings and never escape anything themselves.
type Message struct {
	Level Level
	// Title is the bold first line.
	Title string
	// Text is the body. Long bodies (stack traces, error dumps) are truncated
	// rather than rejected.
	Text string
	// Fields render as sorted "key: value" lines between the title and the body.
	Fields map[string]string
	// Key groups messages for cooldown purposes. An empty Key is never
	// suppressed, which is what makes an explicit Send from user code always
	// deliver.
	Key string
	// Monospace wraps Text in <pre> — set for stack traces and error output.
	Monospace bool
}

// Transport delivers a rendered message. Telegram is the only implementation
// today; the interface exists so a webhook or Discord sender is a file rather
// than a redesign.
type Transport interface {
	// Name identifies the transport in the delivery log.
	Name() string
	// Send delivers one message. A *SendError tells the worker whether to retry
	// and how long to wait; any other error is treated as transient.
	Send(ctx context.Context, m Message, instance string) error
	// Verify checks the credentials, so a bad token surfaces at boot instead of
	// during the first incident. It must not be required for Send to work.
	Verify(ctx context.Context) error
	// Target describes where messages go, for the dashboard. It must not
	// disclose the credentials.
	Target() string
}

// Metrics is the counter snapshot the built-in threshold rules watch.
//
// It is supplied by a MetricsFunc rather than collected here, which keeps this
// package free of any pb-ext dependency: alerts imports only the stdlib and
// PocketBase, so core/server, core/jobs and core/logging can all emit into it
// without an import cycle.
//
// Requests, ClientErrors and ServerErrors are monotonic counters since process
// start. The evaluator differentiates them across ticks; it never trusts a
// pre-computed rate, because a rate that stops being recalculated when traffic
// stops reads as "still busy" forever.
type Metrics struct {
	Requests     uint64
	ClientErrors uint64
	ServerErrors uint64
	Goroutines   int

	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64

	// SwapPercent is meaningless without swap configured, so SwapTotal is
	// carried alongside it: a host with no swap reports 0% and must not be
	// read as "healthy" or alerted on either way.
	SwapPercent float64
	SwapTotal   uint64

	// OpenFilesPercent is descriptors in use as a share of the soft
	// RLIMIT_NOFILE. OpenFilesLimit is 0 where the ceiling is unknown, which
	// disables the check rather than reporting a ratio against nothing.
	OpenFilesPercent float64
	OpenFiles        int
	OpenFilesLimit   uint64
}

// MetricsFunc supplies a fresh snapshot on every evaluator tick.
type MetricsFunc func() Metrics

// Thresholds configures the built-in threshold rules. A zero value disables the
// corresponding rule.
//
// The resource ceilings are on by default; the traffic ones are not. The split
// is deliberate. A request rate has no universal danger zone — 50/s is idle for
// one deployment and an incident for another — so a shipped default would be
// either silent or deafening. Resource saturation is not like that: a disk at
// 95% is about to take the database down on every machine there has ever been,
// and memory, swap and descriptor exhaustion are similarly absolute. Shipping
// those switched off means the common case is a server that looks monitored and
// says nothing while it fills up.
type Thresholds struct {
	// ErrorRatePercent fires when 5xx responses exceed this share of requests
	// in one evaluation window.
	ErrorRatePercent float64
	// ErrorRateMinRequests is the floor below which the window is too small to
	// judge; 1 error in 3 requests is not a 33% error rate worth waking up for.
	ErrorRateMinRequests int

	// SurgeFactor fires when the current request rate exceeds this multiple of
	// the rolling baseline.
	SurgeFactor float64
	// SurgeFloorPerSec is the absolute rate below which a surge is ignored.
	// Without it, traffic doubling from 1 to 2 req/s pages someone at 3am.
	SurgeFloorPerSec float64

	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64

	// SwapPercent fires on swap usage, which is the clearest sign of real
	// memory pressure. Skipped entirely on hosts with no swap configured.
	SwapPercent float64

	// OpenFilesPercent fires on descriptors in use against the soft
	// RLIMIT_NOFILE. Skipped where the limit is unknown.
	OpenFilesPercent float64

	// Goroutines stays opt-in: a healthy busy server legitimately runs
	// thousands, so there is no defensible default, and a leak shows up as
	// unbounded growth rather than as any particular number.
	Goroutines int

	// SustainTicks is how many consecutive evaluations a resource threshold must
	// hold before firing, which filters out momentary spikes.
	SustainTicks int
}

// Config tunes the notifier. Every duration and ceiling has a bounded default.
type Config struct {
	// Enabled gates the whole subsystem. Resolved from options and environment
	// in Initialize; a notifier that is not enabled starts no goroutines.
	Enabled bool
	// EnabledInDev allows alerts when app.IsDev(). Off by default: pb-cli
	// restarts the server on every file save, and each restart would otherwise
	// fire a shutdown and a startup notice.
	EnabledInDev bool
	// Instance labels messages when several servers report into one chat.
	Instance string

	BotToken   string
	ChatID     string
	TopicID    int
	APIBaseURL string

	// QueueSize bounds the in-memory queue. A full queue drops rather than
	// blocks — see Notifier.Send.
	QueueSize int
	// Cooldown is the minimum gap between two alerts sharing a Key.
	Cooldown time.Duration
	// MaxAlertsPerHour caps total deliveries; the overflow is summarised in an
	// hourly digest instead.
	MaxAlertsPerHour int
	// EvaluateInterval is the rule evaluation and heartbeat cadence.
	EvaluateInterval time.Duration
	// SendTimeout bounds one HTTP attempt.
	SendTimeout time.Duration
	// MinSendInterval paces the worker, staying inside Telegram's ~20
	// messages/minute per-chat limit.
	MinSendInterval time.Duration
	// MaxRetries is how many times a transient failure is retried.
	MaxRetries int
	// DrainTimeout bounds how long Close waits for the queue. A wedged Telegram
	// must not delay process exit.
	DrainTimeout time.Duration
	// RetentionDays is how long delivery-log rows are kept.
	RetentionDays int
	// Persist writes delivery outcomes to the _alerts table.
	Persist bool
	// Lifecycle enables the start / crash-recovered / shutdown notices.
	Lifecycle bool

	Thresholds Thresholds

	// transport, metrics and the resolution flags are set by options rather
	// than by callers filling a literal, so they stay unexported.
	transport Transport
	metrics   MetricsFunc
	// backoff is the delay before each retry. It defaults to backoffSchedule
	// and exists as a field so tests can exercise the retry path without
	// spending ten seconds in it.
	backoff []time.Duration
	// enabledSet records that WithEnabled was passed, so an explicit choice is
	// not overridden by credential detection.
	enabledSet bool
	// disabledByEnv records a PBEXT_ALERTS_ENABLED=false kill switch, which
	// outranks everything including WithEnabled(true).
	disabledByEnv bool
}

// Defaults for Config. Override with the With* options.
// Default resource ceilings. Each is deliberately conservative: the cost of a
// missed warning is an outage, and the cost of a false one is a muted channel,
// so these sit where a healthy server does not reach them.
const (
	// DefaultDiskPercent is the earliest warning of the lot. A full disk stops
	// SQLite writing and does not recover on its own.
	DefaultDiskPercent = 90
	// DefaultMemoryPercent reads MemoryInfo.UsedPercent, which excludes page
	// cache — so 90% here is genuine pressure, not a warm cache.
	DefaultMemoryPercent = 90
	// DefaultSwapPercent is high because light swap use is normal on plenty of
	// hosts; by the time it is this deep, memory has usually fired already.
	DefaultSwapPercent = 80
	// DefaultCPUPercent needs SustainTicks behind it — a batch job pinning the
	// cores for one tick is working as intended.
	DefaultCPUPercent = 90
	// DefaultOpenFilesPercent warns early because descriptor exhaustion is
	// abrupt and total rather than gradual.
	DefaultOpenFilesPercent = 80
)

const (
	DefaultQueueSize        = 256
	DefaultCooldown         = 15 * time.Minute
	DefaultMaxAlertsPerHour = 20
	DefaultEvaluateInterval = 30 * time.Second
	DefaultSendTimeout      = 10 * time.Second
	DefaultMinSendInterval  = 3 * time.Second
	DefaultMaxRetries       = 3
	DefaultDrainTimeout     = 5 * time.Second
	DefaultRetentionDays    = 30
	DefaultSustainTicks     = 3
)

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	host, _ := os.Hostname()

	return Config{
		EnabledInDev:     false,
		Instance:         host,
		APIBaseURL:       DefaultAPIBaseURL,
		QueueSize:        DefaultQueueSize,
		Cooldown:         DefaultCooldown,
		MaxAlertsPerHour: DefaultMaxAlertsPerHour,
		EvaluateInterval: DefaultEvaluateInterval,
		SendTimeout:      DefaultSendTimeout,
		MinSendInterval:  DefaultMinSendInterval,
		MaxRetries:       DefaultMaxRetries,
		DrainTimeout:     DefaultDrainTimeout,
		RetentionDays:    DefaultRetentionDays,
		Persist:          true,
		Lifecycle:        true,
		Thresholds: Thresholds{
			// Resource saturation is watched out of the box; the traffic
			// thresholds above stay at zero until asked for.
			CPUPercent:           DefaultCPUPercent,
			MemoryPercent:        DefaultMemoryPercent,
			DiskPercent:          DefaultDiskPercent,
			SwapPercent:          DefaultSwapPercent,
			OpenFilesPercent:     DefaultOpenFilesPercent,
			ErrorRateMinRequests: 20,
			SustainTicks:         DefaultSustainTicks,
		},
	}
}

// normalize replaces non-positive fields with their defaults so a partially
// filled Config is always usable.
func (c *Config) normalize() {
	d := DefaultConfig()

	if c.APIBaseURL == "" {
		c.APIBaseURL = d.APIBaseURL
	}
	if c.QueueSize <= 0 {
		c.QueueSize = d.QueueSize
	}
	if c.Cooldown < 0 {
		c.Cooldown = d.Cooldown
	}
	if c.MaxAlertsPerHour <= 0 {
		c.MaxAlertsPerHour = d.MaxAlertsPerHour
	}
	if c.EvaluateInterval <= 0 {
		c.EvaluateInterval = d.EvaluateInterval
	}
	if c.SendTimeout <= 0 {
		c.SendTimeout = d.SendTimeout
	}
	if c.MinSendInterval < 0 {
		c.MinSendInterval = d.MinSendInterval
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = d.MaxRetries
	}
	if c.DrainTimeout <= 0 {
		c.DrainTimeout = d.DrainTimeout
	}
	if c.RetentionDays <= 0 {
		c.RetentionDays = d.RetentionDays
	}
	if len(c.backoff) == 0 {
		c.backoff = backoffSchedule
	}
	if c.Thresholds.ErrorRateMinRequests <= 0 {
		c.Thresholds.ErrorRateMinRequests = d.Thresholds.ErrorRateMinRequests
	}
	if c.Thresholds.SustainTicks <= 0 {
		c.Thresholds.SustainTicks = d.Thresholds.SustainTicks
	}
}

// applyEnv fills unset credentials from the environment, so the zero-code path
// is "export two variables and restart". Explicit options always win.
func (c *Config) applyEnv() {
	if c.BotToken == "" {
		c.BotToken = os.Getenv("PBEXT_TELEGRAM_BOT_TOKEN")
	}
	if c.ChatID == "" {
		c.ChatID = os.Getenv("PBEXT_TELEGRAM_CHAT_ID")
	}
	if c.TopicID == 0 {
		if id, err := strconv.Atoi(os.Getenv("PBEXT_TELEGRAM_TOPIC_ID")); err == nil {
			c.TopicID = id
		}
	}
	if v := os.Getenv("PBEXT_ALERTS_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil && !b {
			c.Enabled = false
			c.disabledByEnv = true
		}
	}
}

// Option customizes a Notifier.
type Option func(*Config)

// WithTelegram configures the Telegram transport. Both values are required;
// either one empty leaves the notifier disabled.
func WithTelegram(botToken, chatID string) Option {
	return func(c *Config) {
		c.BotToken = botToken
		c.ChatID = chatID
	}
}

// WithTelegramTopic targets a thread inside a forum-style group.
func WithTelegramTopic(threadID int) Option {
	return func(c *Config) { c.TopicID = threadID }
}

// WithTransport replaces the built-in Telegram transport.
func WithTransport(t Transport) Option {
	return func(c *Config) { c.transport = t }
}

// WithEnabled forces the subsystem on or off regardless of credentials.
func WithEnabled(v bool) Option {
	return func(c *Config) {
		c.Enabled = v
		c.enabledSet = true
	}
}

// WithEnabledInDev allows alerts while app.IsDev() is true.
func WithEnabledInDev(v bool) Option {
	return func(c *Config) { c.EnabledInDev = v }
}

// WithInstance labels messages when several servers share one chat.
func WithInstance(name string) Option {
	return func(c *Config) { c.Instance = name }
}

// WithAPIBaseURL points the transport somewhere other than api.telegram.org.
// Tests use it to aim at an httptest server.
func WithAPIBaseURL(u string) Option {
	return func(c *Config) { c.APIBaseURL = u }
}

// WithQueueSize bounds the in-memory queue.
func WithQueueSize(n int) Option {
	return func(c *Config) { c.QueueSize = n }
}

// WithCooldown sets the minimum gap between alerts sharing a Key.
func WithCooldown(d time.Duration) Option {
	return func(c *Config) { c.Cooldown = d }
}

// WithMaxAlertsPerHour caps deliveries per hour before digesting.
func WithMaxAlertsPerHour(n int) Option {
	return func(c *Config) { c.MaxAlertsPerHour = n }
}

// WithEvaluateInterval sets the rule evaluation and heartbeat cadence.
func WithEvaluateInterval(d time.Duration) Option {
	return func(c *Config) { c.EvaluateInterval = d }
}

// WithSendTimeout bounds one delivery attempt.
func WithSendTimeout(d time.Duration) Option {
	return func(c *Config) { c.SendTimeout = d }
}

// WithMinSendInterval paces the worker between sends.
func WithMinSendInterval(d time.Duration) Option {
	return func(c *Config) { c.MinSendInterval = d }
}

// WithMaxRetries sets how often a transient failure is retried.
func WithMaxRetries(n int) Option {
	return func(c *Config) { c.MaxRetries = n }
}

// WithDrainTimeout bounds how long Close waits for the queue to empty.
func WithDrainTimeout(d time.Duration) Option {
	return func(c *Config) { c.DrainTimeout = d }
}

// WithPersistence toggles writing delivery outcomes to the _alerts table.
func WithPersistence(v bool) Option {
	return func(c *Config) { c.Persist = v }
}

// WithRetentionDays sets how long delivery-log rows are kept.
func WithRetentionDays(n int) Option {
	return func(c *Config) { c.RetentionDays = n }
}

// WithLifecycleAlerts toggles the start / crash-recovered / shutdown notices.
func WithLifecycleAlerts(v bool) Option {
	return func(c *Config) { c.Lifecycle = v }
}

// WithErrorRateAlert fires when 5xx responses exceed percent of requests in an
// evaluation window covering at least minRequests requests.
func WithErrorRateAlert(percent float64, minRequests int) Option {
	return func(c *Config) {
		c.Thresholds.ErrorRatePercent = percent
		c.Thresholds.ErrorRateMinRequests = minRequests
	}
}

// WithTrafficSurgeAlert fires when the request rate exceeds factor times the
// rolling baseline while also above floorPerSec.
func WithTrafficSurgeAlert(factor, floorPerSec float64) Option {
	return func(c *Config) {
		c.Thresholds.SurgeFactor = factor
		c.Thresholds.SurgeFloorPerSec = floorPerSec
	}
}

// WithResourceAlerts overrides the CPU, memory and disk ceilings together.
// Pass 0 for any dimension to switch that one off.
func WithResourceAlerts(cpuPercent, memoryPercent, diskPercent float64) Option {
	return func(c *Config) {
		c.Thresholds.CPUPercent = cpuPercent
		c.Thresholds.MemoryPercent = memoryPercent
		c.Thresholds.DiskPercent = diskPercent
	}
}

// WithCPUAlert sets the sustained CPU ceiling; 0 disables it.
func WithCPUAlert(percent float64) Option {
	return func(c *Config) { c.Thresholds.CPUPercent = percent }
}

// WithMemoryAlert sets the memory ceiling; 0 disables it.
func WithMemoryAlert(percent float64) Option {
	return func(c *Config) { c.Thresholds.MemoryPercent = percent }
}

// WithDiskAlert sets the disk ceiling for the filesystem holding the data
// directory; 0 disables it.
func WithDiskAlert(percent float64) Option {
	return func(c *Config) { c.Thresholds.DiskPercent = percent }
}

// WithSwapAlert sets the swap ceiling; 0 disables it. Hosts with no swap are
// skipped regardless.
func WithSwapAlert(percent float64) Option {
	return func(c *Config) { c.Thresholds.SwapPercent = percent }
}

// WithFileDescriptorAlert sets the ceiling for descriptors in use against the
// soft RLIMIT_NOFILE; 0 disables it.
func WithFileDescriptorAlert(percent float64) Option {
	return func(c *Config) { c.Thresholds.OpenFilesPercent = percent }
}

// WithoutResourceAlerts switches off every resource ceiling, for deployments
// that watch saturation with something else.
func WithoutResourceAlerts() Option {
	return func(c *Config) {
		c.Thresholds.CPUPercent = 0
		c.Thresholds.MemoryPercent = 0
		c.Thresholds.DiskPercent = 0
		c.Thresholds.SwapPercent = 0
		c.Thresholds.OpenFilesPercent = 0
	}
}

// WithGoroutineAlert fires on a sustained goroutine count above n.
func WithGoroutineAlert(n int) Option {
	return func(c *Config) { c.Thresholds.Goroutines = n }
}

// WithSustainTicks sets how many consecutive evaluations a resource threshold
// must hold before firing.
func WithSustainTicks(n int) Option {
	return func(c *Config) { c.Thresholds.SustainTicks = n }
}

// WithMetrics supplies the counter snapshot the threshold rules read.
func WithMetrics(f MetricsFunc) Option {
	return func(c *Config) { c.metrics = f }
}

// Stats is a point-in-time view of the notifier, for the dashboard and the
// status endpoint. It never contains the bot token.
type Stats struct {
	Enabled       bool      `json:"enabled"`
	Misconfigured bool      `json:"misconfigured"`
	Reason        string    `json:"reason,omitempty"`
	Transport     string    `json:"transport,omitempty"`
	Target        string    `json:"target,omitempty"`
	Instance      string    `json:"instance,omitempty"`
	Queued        int       `json:"queued"`
	QueueSize     int       `json:"queue_size"`
	Sent          uint64    `json:"sent"`
	Failed        uint64    `json:"failed"`
	Dropped       uint64    `json:"dropped"`
	Suppressed    uint64    `json:"suppressed"`
	Rules         int       `json:"rules"`
	Firing        int       `json:"firing"`
	LastSent      time.Time `json:"last_sent"`
	LastError     string    `json:"last_error,omitempty"`
	LastErrorTime time.Time `json:"last_error_time"`
}

// Record is one row of the delivery log.
type Record struct {
	Created   time.Time `json:"created"`
	Level     string    `json:"level"`
	Key       string    `json:"key,omitempty"`
	Title     string    `json:"title"`
	Text      string    `json:"text,omitempty"`
	Transport string    `json:"transport"`
	Status    string    `json:"status"`
	Attempts  int       `json:"attempts"`
	Error     string    `json:"error,omitempty"`
}

// Data is the dashboard payload: current stats plus the tail of the log.
type Data struct {
	Stats
	Recent []Record `json:"recent"`
}

// DefaultData returns an empty payload, for when alerts are not configured.
func DefaultData() *Data {
	return &Data{Recent: []Record{}}
}
