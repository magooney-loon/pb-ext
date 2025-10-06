package main

// API_SOURCE
// Route example

import (
	"github.com/magooney-loon/pb-ext/core/server/api"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(pbApp core.App) {
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
		Enabled:     true,
		AutoDiscovery: &api.AutoDiscoveryConfig{
			Enabled: true,
		},
	}

	// Initialize version manager with only v1
	versions := map[string]*api.APIDocsConfig{
		"v1": v1Config,
		"v2": v2Config,
	}

	versionManager := api.InitializeVersionedSystem(versions, "v1") // v1 is default/stable

	pbApp.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Get routers
		v1Router, err := versionManager.GetVersionRouter("v1", e)
		if err != nil {
			return err
		}

		v2Router, err := versionManager.GetVersionRouter("v2", e)
		if err != nil {
			return err
		}

		// v1 Example Todo CRUD routes
		v1Router.GET("/api/v1/todos", getTodosHandler)
		v1Router.POST("/api/v1/todos", createTodoHandler).Bind(apis.RequireAuth())
		v1Router.GET("/api/v1/todos/{id}", getTodoHandler)
		v1Router.PATCH("/api/v1/todos/{id}", updateTodoHandler).Bind(apis.RequireAuth())
		v1Router.DELETE("/api/v1/todos/{id}", deleteTodoHandler).Bind(apis.RequireAuth())

		// Version 2 routes
		v2Router.GET("/api/v2/time", timeHandler)

		return e.Next()
	})

	// Register version management endpoints
	versionManager.RegisterWithServer(pbApp)
}
