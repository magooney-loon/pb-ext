package main

// API_SOURCE
// Route definitions showcasing all HTTP methods and authentication types

import (
	app "github.com/magooney-loon/pb-ext/core"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// registerRoutes sets up all HTTP routes demonstrating various methods and auth types
func registerRoutes(pbApp core.App) {
	pbApp.OnServe().BindFunc(func(e *core.ServeEvent) error {
		router := app.EnableAutoDocumentation(e)

		// Files marked with API_SOURCE directive are automatically discovered for AST parsing

		// =================================================================
		// Public Endpoints (No Authentication Required)
		// =================================================================

		// GET - Public utility endpoint
		router.GET("/api/time", timeHandler)

		// =================================================================
		// Guest-Only Endpoints (Unauthenticated Users Only)
		// =================================================================

		// GET - Guest-only information
		router.GET("/api/guest-info", guestInfoHandler).Bind(apis.RequireGuestOnly())

		// =================================================================
		// Authenticated User Endpoints (Any Authenticated User)
		// =================================================================

		// POST - Create new resource (authenticated users)
		router.POST("/api/posts", createPostHandler).Bind(apis.RequireAuth())

		// PATCH - Partial update (authenticated users, with ownership logic in handler)
		router.PATCH("/api/posts/:id", patchPostHandler).Bind(apis.RequireAuth())

		// =================================================================
		// Superuser or Owner Endpoints
		// =================================================================

		// PUT - Full update/replace (superuser or owner access)
		router.PUT("/api/posts/:id", updatePostHandler).Bind(apis.RequireSuperuserOrOwnerAuth("id"))

		// =================================================================
		// Superuser Only Endpoints (Admin)
		// =================================================================

		// DELETE - Remove resource (admin only)
		router.DELETE("/api/posts/:id", deletePostHandler).Bind(apis.RequireSuperuserAuth())

		// GET - Admin dashboard (superuser only)
		router.GET("/api/admin/stats", adminStatsHandler).Bind(apis.RequireSuperuserAuth())

		return e.Next()
	})
}

// HTTP Method & Authentication Coverage:
//
// HTTP Methods Demonstrated:
// ✅ GET    - timeHandler, guestInfoHandler, adminStatsHandler
// ✅ POST   - createPostHandler
// ✅ PUT    - updatePostHandler
// ✅ PATCH  - patchPostHandler
// ✅ DELETE - deletePostHandler
//
// Authentication Types Demonstrated:
// ✅ No Auth            - timeHandler
// ✅ Guest Only         - guestInfoHandler (RequireGuestOnly)
// ✅ Authenticated      - createPostHandler, patchPostHandler (RequireAuth)
// ✅ Superuser or Owner - updatePostHandler (RequireSuperuserOrOwnerAuth)
// ✅ Superuser Only     - deletePostHandler, adminStatsHandler (RequireSuperuserAuth)
//
// This provides a comprehensive example of RESTful API design with
// various authentication levels and HTTP methods for demonstration purposes.
