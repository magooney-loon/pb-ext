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

	// --- Schema test routes: exercises every AST schema generation path ---

	// 1. Deep nested struct response (Order → Address → GeoCoordinate, []OrderItem, PaymentInfo)
	v2.GET("/orders/{id}", getOrderHandler)

	// 2. Nested struct request body via json.Decode
	v2.POST("/orders", createOrderHandler).Bind(apis.RequireAuth())

	// 3. Array-of-structs response
	v2.GET("/orders", listOrdersHandler)

	// 4. Struct with typed maps (map[string]float64, map[string]string, map[string]int)
	v2.GET("/tokens/{id}/indicators", getIndicatorsHandler)

	// 5. Struct with any/interface{} fields + json.Decode request
	v2.POST("/events", trackEventHandler)

	// 6. Inline map literal with deeply nested sub-maps
	v2.GET("/diagnostics", getDiagnosticsHandler)

	// 7. Flat struct + nested struct field (UserProfile → ContactInfo)
	v2.GET("/users/{id}", getUserProfileHandler)

	// 8. Paginated response: inline map wrapping struct array + struct value
	v2.GET("/users", searchUsersHandler)

	// 9. Array of numeric-heavy structs (timeseries / OHLCV)
	v2.GET("/tokens/{id}/candles", getCandlestickHandler)

	// 10. Pure map[string]string variable response (typed map, not literal)
	v2.GET("/config", getConfigHandler)

	// 11. Mixed inline map: bools, ints, strings, nested map literals
	v2.GET("/feature-flags", getFeatureFlagsHandler)

	// 12. map[string]any variable response (the original bug case)
	v2.GET("/stats", getPlatformStatsHandler)

	// 13. BindBody request — PocketBase-native body parsing
	v2.PATCH("/profile", updateProfileHandler).Bind(apis.RequireAuth())

	// 14. Embedded struct (ProductResponse embeds BaseEntity)
	v2.GET("/products/{id}", getProductHandler)

	// 15. Slice of primitives ([]string)
	v2.GET("/categories", listCategoriesHandler)

	// 16. DELETE handler — minimal success response
	v2.DELETE("/products/{id}", deleteProductHandler).Bind(apis.RequireAuth())

	// 17. Variable-referenced struct response
	v2.GET("/health", healthCheckHandler)

	// 18. Query parameters + map with struct slice
	v2.GET("/products", searchProductsHandler)

	// 19. Map literal containing struct values
	v2.GET("/orders/{id}/summary", getOrderSummaryHandler)

	// 20. Multiple conditional return paths + json.Decode request
	v2.POST("/products/batch-delete", batchDeleteHandler).Bind(apis.RequireAuth())

	// 21. Variable map literal with struct slices inside
	v2.GET("/dashboard", getDashboardHandler)

	// 22. Struct pointer response (&ContactInfo{...})
	v2.GET("/users/{id}/contact", getContactInfoHandler)

	// 23. Inline map with array of maps
	v2.GET("/activity", getActivityFeedHandler)

	// 24. Var-declared struct (var x Type = ...)
	v2.GET("/payments/default", getDefaultPaymentHandler)
}
