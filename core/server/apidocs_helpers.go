package server

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// AutoAPIRouter wraps PocketBase router for automatic API documentation
type AutoAPIRouter struct {
	router   interface{}
	registry *APIRegistry
}

// GET automatically documents GET routes
func (r *AutoAPIRouter) GET(path string, handler func(*core.RequestEvent) error) {
	if router, ok := r.router.(interface {
		GET(string, func(*core.RequestEvent) error) interface{}
	}); ok {
		router.GET(path, handler)
		if r.registry != nil {
			r.registry.autoRegisterRoute("GET", path, handler)
		}
	}
}

// POST automatically documents POST routes
func (r *AutoAPIRouter) POST(path string, handler func(*core.RequestEvent) error) {
	if router, ok := r.router.(interface {
		POST(string, func(*core.RequestEvent) error) interface{}
	}); ok {
		router.POST(path, handler)
		if r.registry != nil {
			r.registry.autoRegisterRoute("POST", path, handler)
		}
	}
}

// PATCH automatically documents PATCH routes
func (r *AutoAPIRouter) PATCH(path string, handler func(*core.RequestEvent) error) {
	if router, ok := r.router.(interface {
		PATCH(string, func(*core.RequestEvent) error) interface{}
	}); ok {
		router.PATCH(path, handler)
		if r.registry != nil {
			r.registry.autoRegisterRoute("PATCH", path, handler)
		}
	}
}

// PUT automatically documents PUT routes
func (r *AutoAPIRouter) PUT(path string, handler func(*core.RequestEvent) error) {
	if router, ok := r.router.(interface {
		PUT(string, func(*core.RequestEvent) error) interface{}
	}); ok {
		router.PUT(path, handler)
		if r.registry != nil {
			r.registry.autoRegisterRoute("PUT", path, handler)
		}
	}
}

// DELETE automatically documents DELETE routes
func (r *AutoAPIRouter) DELETE(path string, handler func(*core.RequestEvent) error) {
	if router, ok := r.router.(interface {
		DELETE(string, func(*core.RequestEvent) error) interface{}
	}); ok {
		router.DELETE(path, handler)
		if r.registry != nil {
			r.registry.autoRegisterRoute("DELETE", path, handler)
		}
	}
}

// EnableAutoDocumentation creates a router wrapper that automatically documents routes
func EnableAutoDocumentation(e *core.ServeEvent) *AutoAPIRouter {
	registry := GetGlobalRegistry()
	return &AutoAPIRouter{
		router:   e.Router,
		registry: registry,
	}
}

// RouteAnalyzer provides utilities for analyzing routes automatically
type RouteAnalyzer struct{}

// NewRouteAnalyzer creates a new route analyzer
func NewRouteAnalyzer() *RouteAnalyzer {
	return &RouteAnalyzer{}
}

// AnalyzeHandler extracts information about a handler function
func (ra *RouteAnalyzer) AnalyzeHandler(handler func(*core.RequestEvent) error) HandlerInfo {
	return HandlerInfo{
		Name:        ra.extractHandlerName(handler),
		Package:     ra.extractPackageName(handler),
		Description: ra.generateDescription(handler),
	}
}

// HandlerInfo contains analyzed information about a handler
type HandlerInfo struct {
	Name        string
	Package     string
	Description string
}

// extractHandlerName gets the function name from a handler
func (ra *RouteAnalyzer) extractHandlerName(handler func(*core.RequestEvent) error) string {
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

// extractPackageName gets the package name from a handler
func (ra *RouteAnalyzer) extractPackageName(handler func(*core.RequestEvent) error) string {
	if handler == nil {
		return "unknown"
	}

	funcPtr := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	if funcPtr == nil {
		return "unknown"
	}

	fullName := funcPtr.Name()
	parts := strings.Split(fullName, ".")
	if len(parts) > 1 {
		return parts[len(parts)-2]
	}
	return "main"
}

// generateDescription creates a description from handler analysis
func (ra *RouteAnalyzer) generateDescription(handler func(*core.RequestEvent) error) string {
	name := ra.extractHandlerName(handler)

	// Remove common suffixes
	cleanName := strings.TrimSuffix(name, "Handler")
	cleanName = strings.TrimSuffix(cleanName, "handler")
	cleanName = strings.TrimSuffix(cleanName, "Func")
	cleanName = strings.TrimSuffix(cleanName, "func")

	// Convert camelCase to readable format
	var result strings.Builder
	for i, r := range cleanName {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune(' ')
		}
		result.WriteRune(r)
	}

	desc := result.String()

	// Clean up common patterns
	desc = strings.ReplaceAll(desc, "Api", "API")
	desc = strings.ReplaceAll(desc, "Http", "HTTP")
	desc = strings.ReplaceAll(desc, "Json", "JSON")
	desc = strings.ReplaceAll(desc, "Url", "URL")
	desc = strings.TrimSpace(desc)

	if desc == "" || desc == "anonymous" || desc == "unknown" {
		return "Auto-discovered endpoint"
	}

	return desc
}

// PathAnalyzer provides utilities for analyzing URL paths
type PathAnalyzer struct{}

// NewPathAnalyzer creates a new path analyzer
func NewPathAnalyzer() *PathAnalyzer {
	return &PathAnalyzer{}
}

// ExtractTags generates tags from a URL path
func (pa *PathAnalyzer) ExtractTags(path string) []string {
	var tags []string

	// Split path and analyze each segment
	segments := strings.Split(strings.Trim(path, "/"), "/")

	for _, segment := range segments {
		// Skip empty segments and parameter placeholders
		if segment == "" || strings.Contains(segment, "{") || strings.Contains(segment, ":") {
			continue
		}

		// Skip common prefixes
		if segment == "api" || segment == "v1" || segment == "v2" {
			continue
		}

		// Clean up the segment
		tag := strings.ToLower(segment)
		tag = strings.ReplaceAll(tag, "-", "_")

		// Avoid duplicates
		found := false
		for _, existing := range tags {
			if existing == tag {
				found = true
				break
			}
		}

		if !found {
			tags = append(tags, tag)
		}
	}

	// Add contextual tags
	pathLower := strings.ToLower(path)
	if strings.Contains(pathLower, "auth") {
		tags = append(tags, "authentication")
	}
	if strings.Contains(pathLower, "user") {
		tags = append(tags, "users")
	}
	if strings.Contains(pathLower, "admin") {
		tags = append(tags, "admin")
	}
	if strings.Contains(pathLower, "collection") {
		tags = append(tags, "collections")
	}

	// Ensure at least one tag
	if len(tags) == 0 {
		tags = []string{"api"}
	}

	return tags
}

// DetectAuthRequirement analyzes if a path likely requires authentication
func (pa *PathAnalyzer) DetectAuthRequirement(path string) bool {
	authIndicators := []string{
		"/auth/",
		"/admin/",
		"/protected/",
		"/user/",
		"/account/",
		"/profile/",
		"/settings/",
		"/dashboard/",
	}

	pathLower := strings.ToLower(path)
	for _, indicator := range authIndicators {
		if strings.Contains(pathLower, indicator) {
			return true
		}
	}

	// PocketBase collection records usually require auth
	if strings.Contains(pathLower, "/collections/") && strings.Contains(pathLower, "/records") {
		return true
	}

	return false
}

// GenerateDescription creates a description based on HTTP method and path
func (pa *PathAnalyzer) GenerateDescription(method, path string) string {
	// Clean path for description
	cleanPath := strings.TrimPrefix(path, "/api/")
	cleanPath = strings.ReplaceAll(cleanPath, "/{", " by ")
	cleanPath = strings.ReplaceAll(cleanPath, "}", "")
	cleanPath = strings.ReplaceAll(cleanPath, "/", " ")
	cleanPath = strings.ReplaceAll(cleanPath, "-", " ")
	cleanPath = strings.ReplaceAll(cleanPath, "_", " ")

	// Generate action-based description
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

// AutoDiscoveryConfig holds configuration for automatic discovery
type AutoDiscoveryConfig struct {
	Enabled         bool
	AnalyzeHandlers bool
	GenerateTags    bool
	DetectAuth      bool
	IncludeInternal bool
}

// DefaultAutoDiscoveryConfig returns the default configuration
func DefaultAutoDiscoveryConfig() AutoDiscoveryConfig {
	return AutoDiscoveryConfig{
		Enabled:         true,
		AnalyzeHandlers: true,
		GenerateTags:    true,
		DetectAuth:      true,
		IncludeInternal: false,
	}
}

// ConfigureAutoDiscovery sets up automatic discovery with custom configuration
func ConfigureAutoDiscovery(config AutoDiscoveryConfig) {
	registry := GetGlobalRegistry()
	registry.EnableAutoDiscovery(config.Enabled)
}

// GetDiscoveredEndpoints returns all automatically discovered endpoints
func GetDiscoveredEndpoints() []APIEndpoint {
	registry := GetGlobalRegistry()
	docs := registry.GetDocs()
	return docs.Endpoints
}

// GetEndpointByPath finds a specific endpoint by method and path
func GetEndpointByPath(method, path string) *APIEndpoint {
	endpoints := GetDiscoveredEndpoints()

	for _, endpoint := range endpoints {
		if strings.EqualFold(endpoint.Method, method) && endpoint.Path == path {
			return &endpoint
		}
	}

	return nil
}

// GetEndpointsByTag finds endpoints with a specific tag
func GetEndpointsByTag(tag string) []APIEndpoint {
	var results []APIEndpoint
	endpoints := GetDiscoveredEndpoints()

	for _, endpoint := range endpoints {
		for _, endpointTag := range endpoint.Tags {
			if strings.EqualFold(endpointTag, tag) {
				results = append(results, endpoint)
				break
			}
		}
	}

	return results
}

// Example response structures for common PocketBase patterns
type TimeResponse struct {
	Time struct {
		ISO      string `json:"iso"`
		Unix     string `json:"unix"`
		UnixNano string `json:"unix_nano"`
		UTC      string `json:"utc"`
	} `json:"time"`
}

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Common request structures
type LoginRequest struct {
	Identity string `json:"identity"`
	Password string `json:"password"`
}

type CreateRecordRequest struct {
	Data map[string]any `json:"data"`
}

type UpdateRecordRequest struct {
	Data map[string]any `json:"data"`
}
