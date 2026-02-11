package main

// API_SOURCE

import (
	"github.com/magooney-loon/pb-ext/core/server/api"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(app core.App) {
	// Initialize version manager with configs
	versionManager := api.InitializeVersionedSystem(createAPIVersions(), "v1")

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Get version-specific routers
		v1Router, err := versionManager.GetVersionRouter("v1", e)
		if err != nil {
			return err
		}

		v2Router, err := versionManager.GetVersionRouter("v2", e)
		if err != nil {
			return err
		}

		// Register v1 routes
		registerV1Routes(v1Router)

		// Register v2 routes
		registerV2Routes(v2Router)

		return e.Next()
	})

	// Register version management endpoints
	versionManager.RegisterWithServer(app)
}

// createAPIVersions creates version configurations with reduced duplication
func createAPIVersions() map[string]*api.APIDocsConfig {
	baseConfig := &api.APIDocsConfig{
		Title:       "pb-ext demo api",
		Description: "Hello world",
		BaseURL:     "http://127.0.0.1:8090/",
		Enabled:     true,
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

// registerV2Routes registers all v2 API routes
func registerV2Routes(router *api.VersionedAPIRouter) {
	// Using prefixed router for cleaner code
	v2 := router.SetPrefix("/api/v2")

	// Utility routes (no auth required)
	v2.GET("/time", timeHandler)
}
