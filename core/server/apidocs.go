package server

// Zero-Configuration API Documentation System
//
// This module provides automatic runtime discovery and documentation of API routes.
//
// Key Features:
// - 🚫 ZERO CONFIG: No setup, directives, or manual registration required
// - 🤖 AUTO DISCOVERY: Routes automatically documented as they're registered
// - 🧠 INTELLIGENT: Smart analysis of function names, paths, and auth patterns
// - ⚡ ZERO OVERHEAD: Documentation generated only during route registration
// - 🔌 SIMPLE JSON API: Clean JSON endpoint at /api/docs/json
//
// Usage:
//   func registerRoutes(app core.App) {
//       app.OnServe().BindFunc(func(e *core.ServeEvent) error {
//           router := server.EnableAutoDocumentation(e)
//           router.GET("/api/users", getUsersHandler)  // Auto-documented!
//           return e.Next()
//       })
//   }
//
// Access: http://localhost:8090/api/docs/json

import (
	"net/http"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/pocketbase/pocketbase/core"
)

// APIEndpoint represents a single API endpoint documentation
type APIEndpoint struct {
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Description string                 `json:"description"`
	Request     map[string]interface{} `json:"request,omitempty"`
	Response    map[string]interface{} `json:"response,omitempty"`
	Auth        bool                   `json:"requires_auth"`
	Tags        []string               `json:"tags,omitempty"`
	Handler     string                 `json:"handler_name,omitempty"`
}

// APIDocs holds all API documentation
type APIDocs struct {
	Title       string        `json:"title"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	BaseURL     string        `json:"base_url"`
	Endpoints   []APIEndpoint `json:"endpoints"`
	Generated   string        `json:"generated_at"`
}

// APIRegistry manages automatic API endpoint documentation
type APIRegistry struct {
	mu        sync.RWMutex
	docs      *APIDocs
	endpoints map[string]APIEndpoint
	enabled   bool
}

// RouterWrapper holds the router and registry for automatic documentation
type RouterWrapper struct {
	router interface {
		GET(string, func(*core.RequestEvent) error) interface{}
		POST(string, func(*core.RequestEvent) error) interface{}
		PATCH(string, func(*core.RequestEvent) error) interface{}
		PUT(string, func(*core.RequestEvent) error) interface{}
		DELETE(string, func(*core.RequestEvent) error) interface{}
	}
	registry *APIRegistry
}

// NewAPIRegistry creates a new automatic API documentation registry
func NewAPIRegistry() *APIRegistry {
	return &APIRegistry{
		docs: &APIDocs{
			Title:       "PocketBase Extension API",
			Version:     "1.0.0",
			Description: "Automatically discovered API endpoints",
			BaseURL:     "/api",
			Endpoints:   []APIEndpoint{},
		},
		endpoints: make(map[string]APIEndpoint),
		enabled:   true,
	}
}

// EnableAutoDiscovery turns on/off automatic route discovery
func (r *APIRegistry) EnableAutoDiscovery(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = enabled
}

// WrapRouter creates a wrapper for the router with automatic documentation
func (r *APIRegistry) WrapRouter(router interface {
	GET(string, func(*core.RequestEvent) error) interface{}
	POST(string, func(*core.RequestEvent) error) interface{}
	PATCH(string, func(*core.RequestEvent) error) interface{}
	PUT(string, func(*core.RequestEvent) error) interface{}
	DELETE(string, func(*core.RequestEvent) error) interface{}
}) *RouterWrapper {
	return &RouterWrapper{
		router:   router,
		registry: r,
	}
}

// GET intercepts GET route registration
func (rw *RouterWrapper) GET(path string, handler func(*core.RequestEvent) error) {
	rw.router.GET(path, handler)
	rw.registry.autoRegisterRoute("GET", path, handler)
}

// POST intercepts POST route registration
func (rw *RouterWrapper) POST(path string, handler func(*core.RequestEvent) error) {
	rw.router.POST(path, handler)
	rw.registry.autoRegisterRoute("POST", path, handler)
}

// PATCH intercepts PATCH route registration
func (rw *RouterWrapper) PATCH(path string, handler func(*core.RequestEvent) error) {
	rw.router.PATCH(path, handler)
	rw.registry.autoRegisterRoute("PATCH", path, handler)
}

// PUT intercepts PUT route registration
func (rw *RouterWrapper) PUT(path string, handler func(*core.RequestEvent) error) {
	rw.router.PUT(path, handler)
	rw.registry.autoRegisterRoute("PUT", path, handler)
}

// DELETE intercepts DELETE route registration
func (rw *RouterWrapper) DELETE(path string, handler func(*core.RequestEvent) error) {
	rw.router.DELETE(path, handler)
	rw.registry.autoRegisterRoute("DELETE", path, handler)
}

// autoRegisterRoute automatically analyzes and registers a route
func (r *APIRegistry) autoRegisterRoute(method, path string, handler func(*core.RequestEvent) error) {
	if !r.enabled {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Create endpoint documentation automatically
	endpoint := APIEndpoint{
		Method:      strings.ToUpper(method),
		Path:        path,
		Description: r.generateDescription(method, path, handler),
		Auth:        r.detectAuthRequirement(path),
		Tags:        r.generateTags(path),
		Handler:     r.getHandlerName(handler),
	}

	// Try to extract request/response schemas using reflection
	endpoint.Request = r.analyzeRequestSchema(handler)
	endpoint.Response = r.analyzeResponseSchema(handler)

	key := endpoint.Method + ":" + endpoint.Path
	r.endpoints[key] = endpoint
	r.rebuildEndpoints()
}

// generateDescription creates a human-readable description from path and handler
func (r *APIRegistry) generateDescription(method, path string, handler func(*core.RequestEvent) error) string {
	// First try to get description from handler function name
	handlerName := r.getHandlerName(handler)
	if desc := r.descriptionFromHandlerName(handlerName); desc != "" {
		return desc
	}

	// Fall back to path-based description generation
	return r.descriptionFromPath(method, path)
}

// getHandlerName extracts the function name from a handler
func (r *APIRegistry) getHandlerName(handler func(*core.RequestEvent) error) string {
	if handler == nil {
		return "anonymous"
	}

	funcPtr := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	if funcPtr == nil {
		return "unknown"
	}

	fullName := funcPtr.Name()
	parts := strings.Split(fullName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fullName
}

// descriptionFromHandlerName converts handler function names to descriptions
func (r *APIRegistry) descriptionFromHandlerName(handlerName string) string {
	if handlerName == "anonymous" || handlerName == "unknown" {
		return ""
	}

	// Remove common suffixes
	name := strings.TrimSuffix(handlerName, "Handler")
	name = strings.TrimSuffix(name, "handler")
	name = strings.TrimSuffix(name, "Func")
	name = strings.TrimSuffix(name, "func")

	// Convert camelCase to readable format
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune(' ')
		}
		if i == 0 {
			result.WriteRune(r)
		} else {
			result.WriteRune(r)
		}
	}

	desc := result.String()
	desc = strings.ReplaceAll(desc, "Api", "API")
	desc = strings.ReplaceAll(desc, "Http", "HTTP")
	desc = strings.ReplaceAll(desc, "Json", "JSON")
	desc = strings.ReplaceAll(desc, "Url", "URL")
	desc = strings.TrimSpace(desc)

	if desc == "" {
		return ""
	}

	// Add action context based on common patterns
	if strings.Contains(strings.ToLower(desc), "get") || strings.Contains(strings.ToLower(desc), "list") || strings.Contains(strings.ToLower(desc), "fetch") {
		return "Get " + strings.ToLower(desc)
	}
	if strings.Contains(strings.ToLower(desc), "create") || strings.Contains(strings.ToLower(desc), "add") {
		return "Create " + strings.ToLower(desc)
	}
	if strings.Contains(strings.ToLower(desc), "update") || strings.Contains(strings.ToLower(desc), "modify") {
		return "Update " + strings.ToLower(desc)
	}
	if strings.Contains(strings.ToLower(desc), "delete") || strings.Contains(strings.ToLower(desc), "remove") {
		return "Delete " + strings.ToLower(desc)
	}

	return desc
}

// descriptionFromPath generates description based on HTTP method and path
func (r *APIRegistry) descriptionFromPath(method, path string) string {
	// Clean up path for description
	cleanPath := strings.TrimPrefix(path, "/api/")
	cleanPath = strings.ReplaceAll(cleanPath, "/{", " by ")
	cleanPath = strings.ReplaceAll(cleanPath, "}", "")
	cleanPath = strings.ReplaceAll(cleanPath, "/", " ")
	cleanPath = strings.ReplaceAll(cleanPath, "-", " ")
	cleanPath = strings.ReplaceAll(cleanPath, "_", " ")

	// Generate description based on method
	switch strings.ToUpper(method) {
	case "GET":
		if strings.Contains(path, "{") || strings.Contains(path, ":") {
			return "Get specific " + cleanPath
		}
		return "Get " + cleanPath
	case "POST":
		return "Create " + cleanPath
	case "PATCH", "PUT":
		return "Update " + cleanPath
	case "DELETE":
		return "Delete " + cleanPath
	default:
		return "Handle " + cleanPath
	}
}

// detectAuthRequirement analyzes path to determine if authentication is required
func (r *APIRegistry) detectAuthRequirement(path string) bool {
	// Check for common auth-related paths
	authIndicators := []string{
		"/auth/",
		"/admin/",
		"/protected/",
		"/user/",
		"/account/",
		"/profile/",
		"/settings/",
	}

	pathLower := strings.ToLower(path)
	for _, indicator := range authIndicators {
		if strings.Contains(pathLower, indicator) {
			return true
		}
	}

	// Collections with users typically require auth for non-GET operations
	if strings.Contains(pathLower, "/collections/") && strings.Contains(pathLower, "/records") {
		return true
	}

	return false
}

// generateTags creates tags based on the path structure
func (r *APIRegistry) generateTags(path string) []string {
	var tags []string

	// Extract meaningful parts from path
	parts := strings.Split(strings.Trim(path, "/"), "/")

	for _, part := range parts {
		// Skip parameter placeholders and common prefixes
		if strings.Contains(part, "{") || strings.Contains(part, ":") {
			continue
		}
		if part == "api" || part == "v1" || part == "v2" {
			continue
		}
		if part == "" {
			continue
		}

		// Clean up the part
		tag := strings.ToLower(part)
		tag = strings.ReplaceAll(tag, "-", "_")

		// Don't add duplicates
		found := false
		for _, existingTag := range tags {
			if existingTag == tag {
				found = true
				break
			}
		}
		if !found {
			tags = append(tags, tag)
		}
	}

	// Add special tags based on path patterns
	pathLower := strings.ToLower(path)
	if strings.Contains(pathLower, "auth") || strings.Contains(pathLower, "login") || strings.Contains(pathLower, "register") {
		tags = append(tags, "authentication")
	}
	if strings.Contains(pathLower, "user") || strings.Contains(pathLower, "profile") || strings.Contains(pathLower, "account") {
		tags = append(tags, "users")
	}
	if strings.Contains(pathLower, "admin") {
		tags = append(tags, "admin")
	}
	if strings.Contains(pathLower, "collection") {
		tags = append(tags, "collections")
	}

	// Ensure we have at least one tag
	if len(tags) == 0 {
		tags = []string{"api"}
	}

	return tags
}

// analyzeRequestSchema attempts to extract request schema using reflection
func (r *APIRegistry) analyzeRequestSchema(handler func(*core.RequestEvent) error) map[string]interface{} {
	// This is a simplified version - in a full implementation you might
	// analyze the handler function body or use static analysis
	return nil // For now, return nil - could be enhanced with more sophisticated analysis
}

// analyzeResponseSchema attempts to extract response schema using reflection
func (r *APIRegistry) analyzeResponseSchema(handler func(*core.RequestEvent) error) map[string]interface{} {
	// This is a simplified version - in a full implementation you might
	// analyze the handler function body or use static analysis
	return nil // For now, return nil - could be enhanced with more sophisticated analysis
}

// GetDocs returns the complete API documentation
func (r *APIRegistry) GetDocs() *APIDocs {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Update generated timestamp
	r.docs.Generated = "runtime"
	return r.docs
}

// rebuildEndpoints rebuilds the endpoints slice from the map (must be called with lock held)
func (r *APIRegistry) rebuildEndpoints() {
	endpoints := make([]APIEndpoint, 0, len(r.endpoints))
	for _, endpoint := range r.endpoints {
		endpoints = append(endpoints, endpoint)
	}

	// Sort by path and then by method
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path == endpoints[j].Path {
			return endpoints[i].Method < endpoints[j].Method
		}
		return endpoints[i].Path < endpoints[j].Path
	})

	r.docs.Endpoints = endpoints
}

// Global registry instance
var globalAPIRegistry = NewAPIRegistry()

// GetGlobalRegistry returns the global API registry
func GetGlobalRegistry() *APIRegistry {
	return globalAPIRegistry
}

// RegisterAPIDocsRoutes adds the API documentation routes to the server
func (s *Server) RegisterAPIDocsRoutes(e *core.ServeEvent) {
	registry := globalAPIRegistry

	// JSON API documentation endpoint
	e.Router.GET("/api/docs/json", func(c *core.RequestEvent) error {
		docs := registry.GetDocs()
		return c.JSON(http.StatusOK, docs)
	})

	// Auto-register some built-in routes for demonstration
	s.registerBuiltinRoutes()
}

// registerBuiltinRoutes automatically documents some standard routes
func (s *Server) registerBuiltinRoutes() {
	registry := globalAPIRegistry

	// Health check endpoint
	registry.autoRegisterRoute("GET", "/api/health", func(c *core.RequestEvent) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// API Documentation JSON endpoint
	registry.autoRegisterRoute("GET", "/api/docs/json", func(c *core.RequestEvent) error {
		return nil // JSON API documentation endpoint
	})

	// Standard PocketBase collection routes (examples)
	registry.autoRegisterRoute("GET", "/api/collections/{collection}/records", func(c *core.RequestEvent) error {
		return nil // List collection records
	})

	registry.autoRegisterRoute("POST", "/api/collections/{collection}/records", func(c *core.RequestEvent) error {
		return nil // Create collection record
	})

	registry.autoRegisterRoute("GET", "/api/collections/{collection}/records/{id}", func(c *core.RequestEvent) error {
		return nil // Get specific collection record
	})

	registry.autoRegisterRoute("PATCH", "/api/collections/{collection}/records/{id}", func(c *core.RequestEvent) error {
		return nil // Update collection record
	})

	registry.autoRegisterRoute("DELETE", "/api/collections/{collection}/records/{id}", func(c *core.RequestEvent) error {
		return nil // Delete collection record
	})
}

// AutoRegisterRoute can be used to manually register routes that bypass normal registration
func AutoRegisterRoute(method, path string, handler func(*core.RequestEvent) error) {
	globalAPIRegistry.autoRegisterRoute(method, path, handler)
}
