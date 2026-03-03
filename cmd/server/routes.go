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

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := versionManager.RegisterAllVersionRoutes(e); err != nil {
			return err
		}
		return e.Next()
	})

	// Register version management endpoints
	versionManager.RegisterWithServer(app)
}

// initVersionedSystem initializes the version manager with configured API versions.
func initVersionedSystem() *api.APIVersionManager {
	vm := api.InitializeVersionedSystem(createAPIVersions(), "v1")
	registerVersionRouteRegistrars(vm)
	return vm
}

// registerVersionedRoutesForDocsGeneration registers all versioned routes against docs registries
// without requiring a running PocketBase server.
func registerVersionedRoutesForDocsGeneration(versionManager *api.APIVersionManager) error {
	if versionManager == nil {
		return nil
	}
	return versionManager.RegisterAllVersionRoutesForDocs()
}

// registerVersionRouteRegistrars wires per-version route registration callbacks once,
// then APIVersionManager reuses them for both serve-time and docs-only registration flows.
func registerVersionRouteRegistrars(versionManager *api.APIVersionManager) {
	if versionManager == nil {
		return
	}

	if err := versionManager.SetVersionRouteRegistrars(map[string]func(*api.VersionedAPIRouter){
		"v1": registerV1Routes,
		"v2": registerV2Routes,
	}); err != nil {
		panic(err)
	}
}

// createAPIVersions creates version configurations with reduced duplication
func createAPIVersions() map[string]*api.APIDocsConfig {
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

		PublicSwagger: true,
	}

	// Create v1 config
	v1Config := *baseConfig
	v1Config.Version = "1.0.0"
	v1Config.Status = "stable"

	// Create v2 config
	v2Config := *baseConfig
	v2Config.Version = "2.0.0"
	v2Config.Status = "testing"

	return map[string]*api.APIDocsConfig{
		"v1": &v1Config,
		"v2": &v2Config,
	}
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
	// Uncomment to use instead of manual registration above:
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
