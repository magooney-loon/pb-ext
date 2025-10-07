package main

// API_SOURCE
// Route example

import (
	"github.com/magooney-loon/pb-ext/core/server/api"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(app core.App) {
	// Create configs for API
	v1Config := &api.APIDocsConfig{
		Title:       "pb-ext demo api",
		Version:     "1.0.0",
		Description: "Hello world",
		Status:      "stable",
		Enabled:     true,
		AutoDiscovery: &api.AutoDiscoveryConfig{
			Enabled: true,
		},
	}

	v2Config := &api.APIDocsConfig{
		Title:       "pb-ext demo api",
		Version:     "2.0.0",
		Description: "Hello world",
		Status:      "testing",
		BaseURL:     "http://127.0.0.1:8090/",
		Enabled:     true,
		AutoDiscovery: &api.AutoDiscoveryConfig{
			Enabled: true,
		},
	}

	// Initialize version manager with configs
	versions := map[string]*api.APIDocsConfig{
		"v1": v1Config,
		"v2": v2Config,
	}

	versionManager := api.InitializeVersionedSystem(versions, "v1") // v1 is default/stable

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Get routers
		v1Router, err := versionManager.GetVersionRouter("v1", e)
		if err != nil {
			return err
		}

		v2Router, err := versionManager.GetVersionRouter("v2", e)
		if err != nil {
			return err
		}

		// API prefixes
		v1Prefix := "/api/v1"
		v2Prefix := "/api/v2"

		// v1 Example Todo CRUD routes
		v1Router.GET(v1Prefix+"/todos", getTodosHandler)
		v1Router.POST(v1Prefix+"/todos", createTodoHandler).Bind(apis.RequireAuth())
		v1Router.GET(v1Prefix+"/todos/{id}", getTodoHandler)
		v1Router.PATCH(v1Prefix+"/todos/{id}", updateTodoHandler).Bind(apis.RequireAuth())
		v1Router.DELETE(v1Prefix+"/todos/{id}", deleteTodoHandler).Bind(apis.RequireAuth())

		// Version 2 routes
		v2Router.GET(v2Prefix+"/time", timeHandler)

		return e.Next()
	})

	// Register version management endpoints
	versionManager.RegisterWithServer(app)
}
