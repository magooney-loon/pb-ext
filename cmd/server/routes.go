package main

// API_SOURCE
// Route definitions showcasing all HTTP methods and authentication types

import (
	"github.com/magooney-loon/pb-ext/core/server/api"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(pbApp core.App) {
	// Initialize multi-version API system
	// Create configurations for different API versions
	v1Config := &api.APIDocsConfig{
		Title:       "pb-ext demo api",
		Version:     "1.0.0",
		Description: "Hello world stable - production version",
		Enabled:     true,
		AutoDiscovery: &api.AutoDiscoveryConfig{
			Enabled:         true,
			AnalyzeHandlers: true,
			GenerateTags:    true,
			DetectAuth:      true,
			IncludeInternal: false,
		},
	}

	v2Config := &api.APIDocsConfig{
		Title:       "pb-ext demo api v2",
		Version:     "2.0.0",
		Description: "Hello world enhanced - development version with new features",
		Enabled:     true,
		AutoDiscovery: &api.AutoDiscoveryConfig{
			Enabled:         true,
			AnalyzeHandlers: true,
			GenerateTags:    true,
			DetectAuth:      true,
			IncludeInternal: false,
		},
	}

	// Initialize version manager with both versions
	versions := map[string]*api.APIDocsConfig{
		"v1": v1Config,
		"v2": v2Config,
	}
	versionManager := api.InitializeVersionedSystem(versions, "v1") // v1 is default/stable

	pbApp.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Get version-specific routers
		v1Router, err := versionManager.GetVersionRouter("v1", e)
		if err != nil {
			return err
		}

		v2Router, err := versionManager.GetVersionRouter("v2", e)
		if err != nil {
			return err
		}

		// Version 1 routes (stable production API)
		// Files marked with API_SOURCE directive are automatically discovered for AST parsing
		v1Router.GET("/api/v1/time", timeHandler)
		v1Router.GET("/api/v1/guest-info", guestInfoHandler).Bind(apis.RequireGuestOnly())
		v1Router.POST("/api/v1/posts", createPostHandler).Bind(apis.RequireAuth())
		v1Router.PATCH("/api/v1/posts/:id", patchPostHandler).Bind(apis.RequireAuth())
		v1Router.PUT("/api/v1/posts/:id", updatePostHandler).Bind(apis.RequireSuperuserOrOwnerAuth("id"))
		v1Router.DELETE("/api/v1/posts/:id", deletePostHandler).Bind(apis.RequireSuperuserAuth())
		v1Router.GET("/api/v1/admin/stats", adminStatsHandler).Bind(apis.RequireSuperuserAuth())

		// Version 2 routes (development API with new features)
		v2Router.GET("/api/v2/time", timeHandler)                                          // Enhanced time endpoint
		v2Router.GET("/api/v2/guest-info", guestInfoHandler).Bind(apis.RequireGuestOnly()) // Reuse v1 handler
		v2Router.POST("/api/v2/posts", createPostHandler).Bind(apis.RequireAuth())         // Enhanced posts
		v2Router.PATCH("/api/v2/posts/:id", patchPostHandler).Bind(apis.RequireAuth())     // Enhanced patch
		v2Router.PUT("/api/v2/posts/:id", updatePostHandler).Bind(apis.RequireSuperuserOrOwnerAuth("id"))
		v2Router.DELETE("/api/v2/posts/:id", deletePostHandler).Bind(apis.RequireSuperuserAuth()) // Reuse v1 handler
		v2Router.GET("/api/v2/admin/stats", adminStatsHandler).Bind(apis.RequireSuperuserAuth())
		// New v2-only features (reusing existing handlers for demo)
		v2Router.GET("/api/v2/posts/:id/analytics", adminStatsHandler).Bind(apis.RequireAuth())
		v2Router.POST("/api/v2/bulk/posts", createPostHandler).Bind(apis.RequireAuth())

		// Only versioned routes - no backward compatibility

		return e.Next()
	})

	// Register version management endpoints
	versionManager.RegisterWithServer(pbApp)
}
