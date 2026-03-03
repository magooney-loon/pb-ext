package core

import (
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
	InDeveloperMode = server.InDeveloperMode
	InNormalMode    = server.InNormalMode
)

// Re-export server types
type Server = server.Server
type Option = server.Option

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
