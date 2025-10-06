package main

// API_SOURCE
// Route example

import (
	"github.com/magooney-loon/pb-ext/core/server/api"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(pbApp core.App) {
	// Create config for v1 API
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

	// Initialize version manager with only v1
	versions := map[string]*api.APIDocsConfig{
		"v1": v1Config,
	}

	versionManager := api.InitializeVersionedSystem(versions, "v1") // v1 is default/stable

	pbApp.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Get v1 router
		v1Router, err := versionManager.GetVersionRouter("v1", e)
		if err != nil {
			return err
		}

		// Version 1 routes
		v1Router.GET("/api/v1/time", timeHandler)

		// v1 Example Todo CRUD routes
		v1Router.GET("/api/v1/todos", getTodosHandler)
		v1Router.POST("/api/v1/todos", createTodoHandler).Bind(apis.RequireAuth())
		v1Router.GET("/api/v1/todos/{id}", getTodoHandler)
		v1Router.PATCH("/api/v1/todos/{id}", updateTodoHandler).Bind(apis.RequireAuth())
		v1Router.DELETE("/api/v1/todos/{id}", deleteTodoHandler).Bind(apis.RequireAuth())

		return e.Next()
	})

	// Register version management endpoints
	versionManager.RegisterWithServer(pbApp)
}
