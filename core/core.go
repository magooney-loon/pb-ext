package core

import (
	"github.com/magooney-loon/pb-ext/core/logging"
	"github.com/magooney-loon/pb-ext/core/server"
)

// Re-export server components
var New = server.New

// Re-export logging components
var (
	SetupLogging  = logging.SetupLogging
	SetupRecovery = logging.SetupRecovery
)
