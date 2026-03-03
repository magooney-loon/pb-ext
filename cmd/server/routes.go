package main

// API_SOURCE

import (
	"log"
	"time"

	"github.com/magooney-loon/pb-ext/core/server/api"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(app core.App) {
	versionManager := initVersionedSystem()

	// Register routes on serve and setup version management endpoints
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := versionManager.RegisterAllVersionRoutes(e); err != nil {
			return err
		}
		return e.Next()
	})

	versionManager.RegisterWithServer(app)
}

// initVersionedSystem creates and configures the API version manager.
// This function is reused by both the server startup and spec generation/validation.
func initVersionedSystem() *api.APIVersionManager {
	// Create version configurations
	baseConfig := &api.APIDocsConfig{
		Title:       "pb-ext demo api",
		Description: "Hello world",
		BaseURL:     "http://127.0.0.1:8090/",
		Enabled:     true,

		ContactName:  "pb-ext Team",
		ContactEmail: "contact@magooney.org",
		ContactURL:   "https://github.com/magooney-loon/pb-ext",

		LicenseName: "MIT",
		LicenseURL:  "https://opensource.org/licenses/MIT",

		TermsOfService: "https://example.com/terms",

		ExternalDocsURL:  "https://github.com/magooney-loon/pb-ext",
		ExternalDocsDesc: "pb-ext documentation",
	}

	// Create v1 config
	v1Config := *baseConfig
	v1Config.Version = "1.0.0"
	v1Config.Status = "stable"
	v1Config.PublicSwagger = true

	// Create v2 config
	v2Config := *baseConfig
	v2Config.Version = "2.0.0"
	v2Config.Status = "testing"
	v2Config.PublicSwagger = false

	// Initialize version manager with both configs and routes
	return api.InitializeVersionedSystemWithRoutes(map[string]*api.VersionSetup{
		"v1": {
			Config: &v1Config,
			Routes: registerV1Routes,
		},
		"v2": {
			Config: &v2Config,
			Routes: registerV2Routes,
		},
	}, "v1")
}

// registerV1Routes registers all v1 API routes
func registerV1Routes(router *api.VersionedAPIRouter) {
	// Option 1: Manual route registration (explicit control)
	prefix := "/api/v1"
	router.GET(prefix+"/todos", getTodosHandler)
	router.POST(prefix+"/todos", createTodoHandler).Bind(apis.RequireAuth())
	router.GET(prefix+"/todos/{id}", getTodoHandler)
	router.PATCH(prefix+"/todos/{id}", updateTodoHandler).Bind(apis.RequireAuth())
	router.DELETE(prefix+"/todos/{id}", deleteTodoHandler).Bind(apis.RequireAuth())

	// Option 2: CRUD convenience method (less boilerplate)
	//
	// v1 := router.SetPrefix("/api/v1")
	// v1.CRUD("todos", api.CRUDHandlers{
	// 	List:   getTodosHandler,
	// 	Create: createTodoHandler,
	// 	Get:    getTodoHandler,
	// 	Patch:  updateTodoHandler,
	// 	Delete: deleteTodoHandler,
	// }, apis.RequireAuth()) // Auth applied to Create, Update, Patch, Delete
}

// requestLoggerMW is a simple middleware that logs the method, path, and latency of each request.
// Demonstrates BindFunc with a plain func — no hook.Handler wrapper needed.
func requestLoggerMW(e *core.RequestEvent) error {
	start := time.Now()
	err := e.Next()
	log.Printf("[v2] %s %s — %s", e.Request.Method, e.Request.URL.Path, time.Since(start))
	return err
}

// registerV2Routes registers all v2 API routes
func registerV2Routes(router *api.VersionedAPIRouter) {
	// Using prefixed router for cleaner code
	v2 := router.SetPrefix("/api/v2")

	// Utility routes — requestLoggerMW is attached via BindFunc (plain func, no hook.Handler wrapper)
	v2.GET("/time", timeHandler).BindFunc(requestLoggerMW)
}
