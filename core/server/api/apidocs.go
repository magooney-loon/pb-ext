package api

// API Documentation System - Main Entry Point
//
// This module provides automatic runtime discovery and documentation of API routes
// using AST analysis and OpenAPI-compatible output.
//
// Features:
//   - Automatic route discovery and documentation
//   - AST-based schema generation
//   - OpenAPI 3.0 compatible output
//   - Middleware detection for auth requirements
//   - Clean modular architecture
//
// Usage:
//   // API_SOURCE - Mark files for AST analysis
//
//   func registerRoutes(app core.App) {
//       app.OnServe().BindFunc(func(e *core.ServeEvent) error {
//           // Enable auto-documentation
//           router := EnableAutoDocumentation(e)
//
//           // Routes are automatically documented
//           router.GET("/api/time", timeHandler)
//           router.POST("/api/users", createUserHandler).Bind(apis.RequireAuth())
//
//           return e.Next()
//       })
//   }
//
//   // API_DESC Get current server time in multiple formats
//   // API_TAGS server,time,utilities
//   func timeHandler(c *core.RequestEvent) error {
//       now := time.Now()
//       return c.JSON(http.StatusOK, map[string]any{
//           "time": map[string]string{
//               "iso":       now.Format(time.RFC3339),
//               "unix":      strconv.FormatInt(now.Unix(), 10),
//               "unix_nano": strconv.FormatInt(now.UnixNano(), 10),
//               "utc":       now.UTC().Format(time.RFC3339),
//           },
//       })
//   }
//
// Access:
//   - OpenAPI JSON: http://localhost:8090/api/docs/openapi
//   - Endpoints List: http://localhost:8090/api/docs/endpoints
//   - Statistics: http://localhost:8090/api/docs/stats

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Main API Documentation System
// =============================================================================

// APIDocumentationSystem provides the main interface for API documentation
type APIDocumentationSystem struct {
	registry        *APIRegistry
	astParser       ASTParserInterface
	schemaGenerator SchemaGeneratorInterface
	config          *APIDocsConfig
}

// NewAPIDocumentationSystem creates a new API documentation system with all components
func NewAPIDocumentationSystem(config *APIDocsConfig) *APIDocumentationSystem {
	if config == nil {
		config = DefaultAPIDocsConfig()
	}

	// Initialize components
	astParser := NewASTParser()
	schemaGenerator := NewSchemaGenerator(astParser)
	registry := NewAPIRegistry(config, astParser, schemaGenerator)

	return &APIDocumentationSystem{
		registry:        registry,
		astParser:       astParser,
		schemaGenerator: schemaGenerator,
		config:          config,
	}
}

// GetRegistry returns the API registry
func (ads *APIDocumentationSystem) GetRegistry() *APIRegistry {
	return ads.registry
}

// GetDocs returns the complete API documentation
func (ads *APIDocumentationSystem) GetDocs() *APIDocs {
	return ads.registry.GetDocsWithComponents()
}

// CreateAutoRouter creates an auto-documenting router for the given serve event
func (ads *APIDocumentationSystem) CreateAutoRouter(e *core.ServeEvent) *AutoAPIRouter {
	return NewAutoAPIRouter(e.Router, ads.registry)
}

// RegisterWithServer registers the documentation system with a PocketBase server
func (ads *APIDocumentationSystem) RegisterWithServer(app core.App) {
	ads.registry.RegisterAPIDocsRoutes(app)
}

// UpdateConfig updates the system configuration
func (ads *APIDocumentationSystem) UpdateConfig(config *APIDocsConfig) {
	if config != nil {
		ads.config = config
		ads.registry.UpdateConfig(config)
	}
}

// =============================================================================
// Server Integration Methods
// =============================================================================

// RegisterAPIDocsRoutes registers API documentation routes with a PocketBase server
// This is the main integration point for PocketBase applications
func RegisterAPIDocsRoutes(app core.App) {
	system := GetGlobalDocumentationSystem()
	system.RegisterWithServer(app)
}

// =============================================================================
// Convenience Functions and Global Access
// =============================================================================

// EnableAutoDocumentationWithConfig enables documentation with custom configuration
func EnableAutoDocumentationWithConfig(e *core.ServeEvent, config *APIDocsConfig) *AutoAPIRouter {
	system := NewAPIDocumentationSystem(config)
	return system.CreateAutoRouter(e)
}

// GetAPIDocs returns the current API documentation from the global system
func GetAPIDocs() *APIDocs {
	system := GetGlobalDocumentationSystem()
	return system.GetDocs()
}

// GetAPIEndpoints returns all registered API endpoints (deprecated - use GetAPIDocs() instead)

// RegisterEndpoint manually registers an API endpoint (for backward compatibility)
func RegisterEndpoint(endpoint APIEndpoint) {
	registry := GetGlobalRegistry()
	registry.RegisterEndpoint(endpoint)
}

// ClearAllEndpoints clears all registered endpoints (useful for testing)
func ClearAllEndpoints() {
	registry := GetGlobalRegistry()
	registry.ClearEndpoints()
}

// GetAPIStats returns comprehensive statistics about the registered API endpoints
func GetAPIStats() map[string]interface{} {
	docs := GetAPIDocs()
	return calculateComprehensiveStats(docs.Endpoints)
}

// =============================================================================
// Global System Management
// =============================================================================

var globalDocSystem *APIDocumentationSystem

// GetGlobalDocumentationSystem returns the global documentation system instance
func GetGlobalDocumentationSystem() *APIDocumentationSystem {
	if globalDocSystem == nil {
		globalDocSystem = NewAPIDocumentationSystem(nil)

		// Set the global registry for backward compatibility
		SetGlobalRegistry(globalDocSystem.registry)
	}
	return globalDocSystem
}

// SetGlobalDocumentationSystem sets the global documentation system
func SetGlobalDocumentationSystem(system *APIDocumentationSystem) {
	globalDocSystem = system
	if system != nil {
		SetGlobalRegistry(system.registry)
	}
}

// InitializeWithConfig initializes the global system with custom configuration
func InitializeWithConfig(config *APIDocsConfig) *APIDocumentationSystem {
	system := NewAPIDocumentationSystem(config)
	SetGlobalDocumentationSystem(system)
	return system
}

// =============================================================================
// HTTP Handlers for API Documentation Endpoints
// =============================================================================

// OpenAPIHandler returns the OpenAPI specification as JSON
func OpenAPIHandler(c *core.RequestEvent) error {
	docs := GetAPIDocs()
	return c.JSON(http.StatusOK, docs)
}

// StatsHandler returns comprehensive statistics about the API documentation including health info
func StatsHandler(c *core.RequestEvent) error {
	system := GetGlobalDocumentationSystem()
	docs := system.GetDocs()

	// Calculate comprehensive statistics
	stats := calculateComprehensiveStats(docs.Endpoints)

	// Add health information
	stats["health"] = map[string]interface{}{
		"status":         "healthy",
		"enabled":        system.config.Enabled,
		"auto_discovery": system.config.AutoDiscovery.Enabled,
		"version":        system.config.Version,
		"generated_at":   docs.Generated,
	}

	return c.JSON(http.StatusOK, stats)
}

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

// ComponentsHandler returns the OpenAPI components/schemas
func ComponentsHandler(c *core.RequestEvent) error {
	docs := GetAPIDocs()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"components": docs.Components,
	})
}

// =============================================================================
// Backward Compatibility Functions
// =============================================================================

// These functions maintain backward compatibility with the previous API

// DefaultAutoDiscoveryConfig returns default auto-discovery configuration
func DefaultAutoDiscoveryConfig() *AutoDiscoveryConfig {
	return &AutoDiscoveryConfig{
		Enabled:         true,
		AnalyzeHandlers: true,
		GenerateTags:    true,
		DetectAuth:      true,
		IncludeInternal: false,
	}
}

// =============================================================================
// Server Type Definition (if not defined elsewhere)
// =============================================================================

// =============================================================================
// Migration and Upgrade Utilities
// =============================================================================

// MigrateFromLegacyConfig migrates from old configuration format
func MigrateFromLegacyConfig(legacyConfig map[string]interface{}) *APIDocsConfig {
	config := DefaultAPIDocsConfig()

	if title, ok := legacyConfig["title"].(string); ok {
		config.Title = title
	}
	if version, ok := legacyConfig["version"].(string); ok {
		config.Version = version
	}
	if description, ok := legacyConfig["description"].(string); ok {
		config.Description = description
	}
	if baseURL, ok := legacyConfig["base_url"].(string); ok {
		config.BaseURL = baseURL
	}
	if enabled, ok := legacyConfig["enabled"].(bool); ok {
		config.Enabled = enabled
	}

	return config
}

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

// =============================================================================
// Documentation and Examples
// =============================================================================

/*
Example Usage:

1. Basic Setup:
   ```go
   func init() {
       app.OnServe().BindFunc(func(e *core.ServeEvent) error {
           router := EnableAutoDocumentation(e)

           router.GET("/api/hello", helloHandler)
           router.POST("/api/users", createUserHandler).Bind(apis.RequireAuth())

           return e.Next()
       })
   }
   ```

2. Custom Configuration:
   ```go
   config := &APIDocsConfig{
       Title: "My API",
       Version: "2.0.0",
       Description: "Custom API documentation",
       Enabled: true,
   }

   system := InitializeWithConfig(config)
   ```

3. Manual Endpoint Registration:
   ```go
   endpoint := APIEndpoint{
       Method: "GET",
       Path: "/api/custom",
       Description: "Custom endpoint",
       Tags: []string{"custom"},
   }

   RegisterEndpoint(endpoint)
   ```

4. AST Analysis Directives:
   ```go
   // API_SOURCE - Include this file in AST analysis

   // API_DESC Retrieves user profile information
   // API_TAGS users,profile,auth
   func getUserProfile(c *core.RequestEvent) error {
       // Implementation
   }
   ```
*/
