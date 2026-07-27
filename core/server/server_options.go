package server

import (
	"errors"
	"log"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/audit"
	"github.com/pocketbase/pocketbase"
)

// options are for internal argument passing when constructing a server.
type options struct {
	config         *pocketbase.Config
	pocketbase     *pocketbase.PocketBase
	developer_mode bool
	alerts         []alerts.Option
	audit          []audit.Option
}

// ErrConfigurationConflict is returned if both a config and an initialized PocketBase are provided.
var ErrConfigurationConflict = errors.New(
	`WithConfig cannot be used together with WithPocketbase, cause second contains already initialized pocketbase.Config instance. Just pass your config into pocketbase.NewWithConfig func, that's enough.`,
)

// Option is the functional option type for modifying options.
type Option func(*options)

// WithConfig sets the PocketBase configuration to use.
// Using this together with WithPocketbase will panic.
func WithConfig(config *pocketbase.Config) Option {
	return func(opts *options) {
		opts.config = config
	}
}

// WithPocketbase sets a fully initialized PocketBase instance to use.
// Cannot be used together with WithConfig; will panic if a config is already set.
func WithPocketbase(pocketbase *pocketbase.PocketBase) Option {
	return func(opts *options) {
		if opts.config != nil {
			pocketbase.Logger().Error(ErrConfigurationConflict.Error())
			panic(ErrConfigurationConflict)
		}
		opts.pocketbase = pocketbase
	}
}

// WithMode sets whether developer mode is enabled.
func WithMode(developer_mode bool) Option {
	return func(opts *options) {
		opts.developer_mode = developer_mode
	}
}

// InDeveloperMode is a shortcut to enable developer mode.
func InDeveloperMode() Option {
	return func(opts *options) {
		opts.developer_mode = true
		log.Println("🔧 Developer mode")
	}
}

// WithAlerts configures the notification subsystem. Without it, alerts fall
// back to the PBEXT_TELEGRAM_* environment variables, and stay disabled if
// those are unset.
//
//	srv := server.New(server.WithAlerts(
//	    alerts.WithTelegram(token, chatID),
//	    alerts.WithErrorRateAlert(10, 20),
//	))
func WithAlerts(opts ...alerts.Option) Option {
	return func(o *options) {
		o.alerts = append(o.alerts, opts...)
	}
}

// WithAudit configures admin access auditing, which is on by default.
//
// It records requests to the admin surfaces and every superuser authentication
// attempt, including the account a failed attempt targeted — so unlike the rest
// of pb-ext it stores personal data. Use audit.WithPersonalData to narrow what
// is kept, or audit.WithEnabled(false) to turn it off entirely.
//
//	srv := server.New(server.WithAudit(
//	    audit.WithRetentionDays(30),
//	    audit.WithBruteForceAlert(3, 5*time.Minute),
//	))
func WithAudit(opts ...audit.Option) Option {
	return func(o *options) {
		o.audit = append(o.audit, opts...)
	}
}

// InNormalMode is a shortcut to disable developer mode.
func InNormalMode() Option {
	return func(opts *options) {
		opts.developer_mode = false
		log.Println("🚀 Production mode")
	}
}
