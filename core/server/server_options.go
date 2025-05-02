package server

import (
	"errors"

	"github.com/pocketbase/pocketbase"
)

// options are for internal argument passing when constructing a server.
type options struct {
	config         *pocketbase.Config
	pocketbase     *pocketbase.PocketBase
	developer_mode bool
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
	}
}

// InNormalMode is a shortcut to disable developer mode.
func InNormalMode() Option {
	return func(opts *options) {
		opts.developer_mode = false
	}
}
