package main

// API_SOURCE
// Route example

import (
	"github.com/magooney-loon/pb-ext/core/server/api"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(pbApp core.App) {
	// Create configs for API versions
	v0Config := &api.APIDocsConfig{
		Title:       "pb-ext legacy",
		Version:     "0.0.1",
		Description: "Hello",
		Status:      "deprecated",
		Enabled:     true,
		AutoDiscovery: &api.AutoDiscoveryConfig{
			Enabled: true,
		},
	}

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
		Description: "Hello world?",
		Status:      "development",
		Enabled:     true,
		AutoDiscovery: &api.AutoDiscoveryConfig{
			Enabled: true,
		},
	}

	// Initialize version manager
	versions := map[string]*api.APIDocsConfig{
		"v0": v0Config,
		"v1": v1Config,
		"v2": v2Config,
	}

	versionManager := api.InitializeVersionedSystem(versions, "v1") // v1 is default/stable

	pbApp.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Get version-specific routers
		v0Router, err := versionManager.GetVersionRouter("v0", e)
		if err != nil {
			return err
		}

		v1Router, err := versionManager.GetVersionRouter("v1", e)
		if err != nil {
			return err
		}

		v2Router, err := versionManager.GetVersionRouter("v2", e)
		if err != nil {
			return err
		}

		// Version 0 routes
		v0Router.GET("/api/v0/time", timeHandler)

		// Version 1 routes
		v1Router.GET("/api/v1/time", timeHandler)
		v1Router.GET("/api/v1/guest-info", guestInfoHandler).Bind(apis.RequireGuestOnly())
		v1Router.GET("/api/v1/admin/stats", adminStatsHandler).Bind(apis.RequireSuperuserAuth())

		// Version 2 routes
		v2Router.GET("/api/v2/time", timeHandler)
		v2Router.GET("/api/v2/guest-info", guestInfoHandler).Bind(apis.RequireGuestOnly())
		v2Router.POST("/api/v2/posts", createPostHandler).Bind(apis.RequireAuth())
		v2Router.PATCH("/api/v2/posts/:id", patchPostHandler).Bind(apis.RequireAuth())
		v2Router.PUT("/api/v2/posts/:id", updatePostHandler).Bind(apis.RequireSuperuserOrOwnerAuth("id"))
		v2Router.DELETE("/api/v2/posts/:id", deletePostHandler).Bind(apis.RequireSuperuserAuth())
		v2Router.GET("/api/v2/admin/stats", adminStatsHandler).Bind(apis.RequireSuperuserAuth())

		return e.Next()
	})

	// Register version management endpoints
	versionManager.RegisterWithServer(pbApp)
}
