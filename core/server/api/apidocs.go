package api

/// =============================================================================
// Documentation and Examples
// =============================================================================

/*
Example Usage - Versioned System Only:

   ```go
   func registerRoutes(pbApp core.App) {
       // Create configs for API versions
       v1Config := &api.APIDocsConfig{
           Title:       "pb-ext demo api",
           Version:     "1.0.0",
           Description: "Stable production API",
           Status:      "stable",
           Enabled:     true,
           AutoDiscovery: &api.AutoDiscoveryConfig{
               Enabled: true,
           },
       }

       v2Config := &api.APIDocsConfig{
           Title:       "pb-ext demo api",
           Version:     "2.0.0",
           Description: "Development API with new features",
           Status:      "testing",
           Enabled:     true,
           AutoDiscovery: &api.AutoDiscoveryConfig{
               Enabled: true,
           },
       }

       // Initialize version manager
       versions := map[string]*api.APIDocsConfig{
           "v1": v1Config,
           "v2": v2Config,
       }
       versionManager := api.InitializeVersionedSystem(versions, "v1")

       pbApp.OnServe().BindFunc(func(e *core.ServeEvent) error {
           // Get version-specific routers
           v1Router, _ := versionManager.GetVersionRouter("v1", e)
           v2Router, _ := versionManager.GetVersionRouter("v2", e)

           // v1 Example CRUD routes
           v1Router.GET("/api/v1/todos", getTodosHandler)
           v1Router.POST("/api/v1/todos", createTodoHandler).Bind(apis.RequireAuth())
           v1Router.GET("/api/v1/todos/{id}", getTodoHandler)
           v1Router.PATCH("/api/v1/todos/{id}", updateTodoHandler).Bind(apis.RequireAuth())
           v1Router.DELETE("/api/v1/todos/{id}", deleteTodoHandler).Bind(apis.RequireAuth())

           // v2 routes with new features
           v2Router.GET("/api/v2/time", timeHandler)

           return e.Next()
       })

       // Register version management endpoints
       versionManager.RegisterWithServer(pbApp)
   }

   // AST Analysis Directives:
   // API_SOURCE - Include this file in AST analysis

   // API_DESC Retrieves user profile information
   // API_TAGS users,profile,auth
   func getUserProfile(c *core.RequestEvent) error {
       // Implementation
   }
   ```
*/

import (
	"strings"
)

// =============================================================================
// Versioned System Only
// =============================================================================

// InitializeVersionedSystem initializes a versioned documentation system
func InitializeVersionedSystem(versions map[string]*APIDocsConfig, defaultVersion string) *APIVersionManager {
	return InitializeVersionManager(versions, defaultVersion)
}

// =============================================================================
// Statistics Calculation
// =============================================================================

// calculateComprehensiveStats calculates comprehensive statistics for all endpoints
func calculateComprehensiveStats(endpoints []APIEndpoint) map[string]interface{} {
	stats := map[string]interface{}{
		"total_endpoints": len(endpoints),
		"methods":         make(map[string]int),
		"auth_types":      make(map[string]int),
		"tags":            make(map[string]int),
	}

	methods := stats["methods"].(map[string]int)
	authTypes := stats["auth_types"].(map[string]int)
	tags := stats["tags"].(map[string]int)

	authRequired := 0
	pathsWithParams := 0
	uniquePaths := make(map[string]bool)

	// Calculate statistics from all endpoints
	for i := range endpoints {
		endpoint := &endpoints[i]
		// Count methods
		methods[endpoint.Method]++

		// Count auth types
		if endpoint.Auth != nil && endpoint.Auth.Required {
			authRequired++
			authTypes[endpoint.Auth.Type]++
		} else {
			authTypes["none"]++
		}

		// Count tags
		for _, tag := range endpoint.Tags {
			tags[tag]++
		}

		// Check for path parameters
		if strings.Contains(endpoint.Path, ":") || strings.Contains(endpoint.Path, "{") {
			pathsWithParams++
		}

		// Count unique paths (ignoring method)
		uniquePaths[endpoint.Path] = true
	}

	// Add summary statistics
	stats["summary"] = map[string]interface{}{
		"auth_required":     authRequired,
		"auth_not_required": len(endpoints) - authRequired,
		"paths_with_params": pathsWithParams,
		"unique_paths":      len(uniquePaths),
		"avg_tags_per_endpoint": func() float64 {
			if len(endpoints) == 0 {
				return 0
			}
			totalTags := 0
			for i := range endpoints {
				totalTags += len(endpoints[i].Tags)
			}
			return float64(totalTags) / float64(len(endpoints))
		}(),
	}

	return stats
}

// =============================================================================
// Configuration Utilities
// =============================================================================

// ValidateConfiguration validates an API documentation configuration
func ValidateConfiguration(config *APIDocsConfig) []string {
	var errors []string

	if config == nil {
		errors = append(errors, "configuration is nil")
		return errors
	}

	if config.Title == "" {
		errors = append(errors, "title is required")
	}
	if config.Version == "" {
		errors = append(errors, "version is required")
	}
	if config.BaseURL == "" {
		errors = append(errors, "base_url is required")
	}

	return errors
}
