package core

import (
	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/audit"
	"github.com/magooney-loon/pb-ext/core/logging"
	"github.com/magooney-loon/pb-ext/core/server"
	"github.com/magooney-loon/pb-ext/core/server/api"
)

// Re-export server components
var New = server.New

// Re-export server options
var (
	WithConfig      = server.WithConfig
	WithPocketbase  = server.WithPocketbase
	WithMode        = server.WithMode
	WithAlerts      = server.WithAlerts
	WithAudit       = server.WithAudit
	InDeveloperMode = server.InDeveloperMode
	InNormalMode    = server.InNormalMode
)

// Re-export server types
type Server = server.Server
type Option = server.Option

// Re-export alert components.
//
// GetNotifier never returns nil, so application code can call
// app.GetNotifier().Send(...) without a configured-or-not check.
var (
	GetNotifier             = alerts.Get
	WithTelegram            = alerts.WithTelegram
	WithTelegramTopic       = alerts.WithTelegramTopic
	WithErrorRateAlert      = alerts.WithErrorRateAlert
	WithTrafficSurge        = alerts.WithTrafficSurgeAlert
	WithResourceAlerts      = alerts.WithResourceAlerts
	WithoutResourceAlerts   = alerts.WithoutResourceAlerts
	WithCPUAlert            = alerts.WithCPUAlert
	WithMemoryAlert         = alerts.WithMemoryAlert
	WithDiskAlert           = alerts.WithDiskAlert
	WithSwapAlert           = alerts.WithSwapAlert
	WithFileDescriptorAlert = alerts.WithFileDescriptorAlert
	WithAlertsEnabledInDev  = alerts.WithEnabledInDev
)

// Re-export alert types
type (
	Notifier     = alerts.Notifier
	AlertMessage = alerts.Message
	AlertRule    = alerts.Rule
	AlertOption  = alerts.Option
)

// Re-export admin access auditing.
//
// Auditing is on by default and records personal data — client addresses, user
// agents, and the accounts authentication attempts targeted — because an
// intrusion question cannot be answered without them. Narrow it with
// WithAuditPersonalData or switch it off with WithAuditEnabled(false).
var (
	GetAuditor             = audit.Get
	WithAuditEnabled       = audit.WithEnabled
	WithAuditRetentionDays = audit.WithRetentionDays
	WithAuditPersonalData  = audit.WithPersonalData
	WithBruteForceAlert    = audit.WithBruteForceAlert
)

// Re-export audit types
type (
	Auditor     = audit.Auditor
	AuditRecord = audit.Record
	AuditOption = audit.Option
)

// Alert severity levels
const (
	AlertInfo     = alerts.LevelInfo
	AlertWarn     = alerts.LevelWarn
	AlertError    = alerts.LevelError
	AlertCritical = alerts.LevelCritical
)

// Re-export logging components
var (
	SetupLogging  = logging.SetupLogging
	SetupRecovery = logging.SetupRecovery
)

// Re-export API spec generator components
var (
	NewSpecGeneratorWithInitializer = api.NewSpecGeneratorWithInitializer
	ValidateSpecs                   = api.ValidateSpecs
	ValidateSpecFile                = api.ValidateSpecFile
)

// Re-export API types
type APIVersionManager = api.APIVersionManager
