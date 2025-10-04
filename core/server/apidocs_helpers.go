package server

import (
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// AutoAPIRouter wraps PocketBase router for automatic API documentation
type AutoAPIRouter struct {
	router   interface{}
	registry *APIRegistry
}

// RouteChain represents a chainable route for middleware binding
type RouteChain struct {
	route      interface{}
	method     string
	path       string
	handler    func(*core.RequestEvent) error
	registry   *APIRegistry
	middleware []string
}

// Bind detects middleware binding and updates documentation
func (rc *RouteChain) Bind(middlewares ...interface{}) *RouteChain {
	if rc.route != nil {
		// Use reflection to call Bind on the actual route
		routeValue := reflect.ValueOf(rc.route)
		bindMethod := routeValue.MethodByName("Bind")
		if bindMethod.IsValid() {
			args := make([]reflect.Value, len(middlewares))
			for i, mw := range middlewares {
				args[i] = reflect.ValueOf(mw)
				// Detect auth middleware patterns
				if mwType := reflect.TypeOf(mw); mwType != nil {
					mwName := mwType.String()
					if strings.Contains(mwName, "RequireAuth") || strings.Contains(mwName, "Auth") {
						rc.middleware = append(rc.middleware, "auth")
					}
				}
			}
			bindMethod.Call(args)
		}
	}

	// Update registry with middleware info
	if rc.registry != nil && len(rc.middleware) > 0 {
		rc.updateEndpointAuth()
	}

	return rc
}

// BindFunc detects function middleware and updates documentation
func (rc *RouteChain) BindFunc(middlewareFunc func(*core.RequestEvent) error) *RouteChain {
	if rc.route != nil {
		routeValue := reflect.ValueOf(rc.route)
		bindFuncMethod := routeValue.MethodByName("BindFunc")
		if bindFuncMethod.IsValid() {
			bindFuncMethod.Call([]reflect.Value{reflect.ValueOf(middlewareFunc)})
		}
	}
	return rc
}

// updateEndpointAuth updates the endpoint's auth requirement based on detected middleware
func (rc *RouteChain) updateEndpointAuth() {
	if rc.registry == nil {
		return
	}

	rc.registry.mu.Lock()
	defer rc.registry.mu.Unlock()

	key := rc.method + ":" + rc.path
	if endpoint, exists := rc.registry.endpoints[key]; exists {
		for _, mw := range rc.middleware {
			if mw == "auth" {
				endpoint.Auth = true
				rc.registry.endpoints[key] = endpoint
				rc.registry.rebuildEndpoints()
				break
			}
		}
	}
}

// GET automatically documents GET routes
func (r *AutoAPIRouter) GET(path string, handler func(*core.RequestEvent) error) *RouteChain {
	// Use reflection to call the GET method
	routerValue := reflect.ValueOf(r.router)
	getMethod := routerValue.MethodByName("GET")

	var result interface{}
	if getMethod.IsValid() {
		results := getMethod.Call([]reflect.Value{
			reflect.ValueOf(path),
			reflect.ValueOf(handler),
		})
		if len(results) > 0 {
			result = results[0].Interface()
		}

		if r.registry != nil {
			r.registry.autoRegisterRoute("GET", path, handler)
		}
	}

	return &RouteChain{
		route:    result,
		method:   "GET",
		path:     path,
		handler:  handler,
		registry: r.registry,
	}
}

// POST automatically documents POST routes
func (r *AutoAPIRouter) POST(path string, handler func(*core.RequestEvent) error) *RouteChain {
	routerValue := reflect.ValueOf(r.router)
	postMethod := routerValue.MethodByName("POST")

	var result interface{}
	if postMethod.IsValid() {
		results := postMethod.Call([]reflect.Value{
			reflect.ValueOf(path),
			reflect.ValueOf(handler),
		})
		if len(results) > 0 {
			result = results[0].Interface()
		}

		if r.registry != nil {
			r.registry.autoRegisterRoute("POST", path, handler)
		}
	}

	return &RouteChain{
		route:    result,
		method:   "POST",
		path:     path,
		handler:  handler,
		registry: r.registry,
	}
}

// PATCH automatically documents PATCH routes
func (r *AutoAPIRouter) PATCH(path string, handler func(*core.RequestEvent) error) *RouteChain {
	routerValue := reflect.ValueOf(r.router)
	patchMethod := routerValue.MethodByName("PATCH")

	var result interface{}
	if patchMethod.IsValid() {
		results := patchMethod.Call([]reflect.Value{
			reflect.ValueOf(path),
			reflect.ValueOf(handler),
		})
		if len(results) > 0 {
			result = results[0].Interface()
		}

		if r.registry != nil {
			r.registry.autoRegisterRoute("PATCH", path, handler)
		}
	}

	return &RouteChain{
		route:    result,
		method:   "PATCH",
		path:     path,
		handler:  handler,
		registry: r.registry,
	}
}

// PUT automatically documents PUT routes
func (r *AutoAPIRouter) PUT(path string, handler func(*core.RequestEvent) error) *RouteChain {
	routerValue := reflect.ValueOf(r.router)
	putMethod := routerValue.MethodByName("PUT")

	var result interface{}
	if putMethod.IsValid() {
		results := putMethod.Call([]reflect.Value{
			reflect.ValueOf(path),
			reflect.ValueOf(handler),
		})
		if len(results) > 0 {
			result = results[0].Interface()
		}

		if r.registry != nil {
			r.registry.autoRegisterRoute("PUT", path, handler)
		}
	}

	return &RouteChain{
		route:    result,
		method:   "PUT",
		path:     path,
		handler:  handler,
		registry: r.registry,
	}
}

// DELETE automatically documents DELETE routes
func (r *AutoAPIRouter) DELETE(path string, handler func(*core.RequestEvent) error) *RouteChain {
	routerValue := reflect.ValueOf(r.router)
	deleteMethod := routerValue.MethodByName("DELETE")

	var result interface{}
	if deleteMethod.IsValid() {
		results := deleteMethod.Call([]reflect.Value{
			reflect.ValueOf(path),
			reflect.ValueOf(handler),
		})
		if len(results) > 0 {
			result = results[0].Interface()
		}

		if r.registry != nil {
			r.registry.autoRegisterRoute("DELETE", path, handler)
		}
	}

	return &RouteChain{route: result}
}

// ANY automatically documents routes that handle any HTTP method
func (r *AutoAPIRouter) ANY(pattern string, handler func(*core.RequestEvent) error) *RouteChain {
	routerValue := reflect.ValueOf(r.router)
	anyMethod := routerValue.MethodByName("Any")

	var result interface{}
	if anyMethod.IsValid() {
		results := anyMethod.Call([]reflect.Value{
			reflect.ValueOf(pattern),
			reflect.ValueOf(handler),
		})
		if len(results) > 0 {
			result = results[0].Interface()
		}

		if r.registry != nil {
			// Parse method from pattern if specified (e.g. "TRACE /example")
			method := "ANY"
			path := pattern
			if parts := strings.SplitN(pattern, " ", 2); len(parts) == 2 {
				method = parts[0]
				path = parts[1]
			}
			r.registry.autoRegisterRoute(method, path, handler)
		}
	}

	return &RouteChain{route: result}
}

// Group creates a route group for automatic documentation
func (r *AutoAPIRouter) Group(prefix string) *AutoAPIRouter {
	routerValue := reflect.ValueOf(r.router)
	groupMethod := routerValue.MethodByName("Group")

	if groupMethod.IsValid() {
		results := groupMethod.Call([]reflect.Value{
			reflect.ValueOf(prefix),
		})
		if len(results) > 0 {
			groupRouter := results[0].Interface()
			return &AutoAPIRouter{
				router:   groupRouter,
				registry: r.registry,
			}
		}
	}

	return r
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
		// Skip empty segments
		if segment == "" {
			continue
		}

		// Handle PocketBase parameter patterns
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			paramName := segment[1 : len(segment)-1]
			// Remove wildcard suffix {...} and end marker {$}
			paramName = strings.TrimSuffix(paramName, "...")
			if paramName == "$" {
				continue // Skip end marker
			}
			// Add parameter name as tag if meaningful
			if paramName != "id" && paramName != "path" && len(paramName) > 2 {
				tag := strings.ToLower(paramName)
				tag = strings.ReplaceAll(tag, "-", "_")
				if !pa.containsTag(tags, tag) {
					tags = append(tags, tag)
				}
			}
			continue
		}

		// Skip old-style colon parameters
		if strings.Contains(segment, ":") {
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
		if !pa.containsTag(tags, tag) {
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

// containsTag checks if a tag already exists in the slice
func (pa *PathAnalyzer) containsTag(tags []string, tag string) bool {
	for _, existing := range tags {
		if existing == tag {
			return true
		}
	}
	return false
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

	// Handle PocketBase parameter patterns
	cleanPath = pa.cleanPocketBaseParams(cleanPath)

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

// cleanPocketBaseParams converts PocketBase path parameters to readable text
func (pa *PathAnalyzer) cleanPocketBaseParams(path string) string {
	// Handle {param} -> "by param"
	re := regexp.MustCompile(`\{([^}]+)\}`)
	path = re.ReplaceAllStringFunc(path, func(match string) string {
		param := match[1 : len(match)-1] // Remove { and }

		// Handle special cases
		if param == "$" {
			return "" // End marker - remove entirely
		}
		if strings.HasSuffix(param, "...") {
			param = strings.TrimSuffix(param, "...")
			return "by " + param + " path"
		}

		// Convert common parameter names to readable text
		switch param {
		case "id":
			return "by ID"
		case "collection":
			return "in collection"
		case "record":
			return "for record"
		default:
			return "by " + strings.ReplaceAll(param, "_", " ")
		}
	})

	return path
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

// SchemaAnalyzer provides advanced schema analysis capabilities
type SchemaAnalyzer struct {
	patterns []SchemaPattern
}

// SchemaPattern represents a pattern for schema generation
type SchemaPattern struct {
	Name         string
	HandlerMatch func(string) bool
	PathMatch    func(string) bool
	RequestGen   func() map[string]interface{}
	ResponseGen  func() map[string]interface{}
}

// NewSchemaAnalyzer creates a new schema analyzer with default patterns
func NewSchemaAnalyzer() *SchemaAnalyzer {
	return &SchemaAnalyzer{
		patterns: getDefaultSchemaPatterns(),
	}
}

// AnalyzeRequestSchema analyzes and generates request schema based on patterns
func (sa *SchemaAnalyzer) AnalyzeRequestSchema(handlerName, path string) map[string]interface{} {
	for _, pattern := range sa.patterns {
		if (pattern.HandlerMatch != nil && pattern.HandlerMatch(handlerName)) ||
			(pattern.PathMatch != nil && pattern.PathMatch(path)) {
			if pattern.RequestGen != nil {
				return pattern.RequestGen()
			}
		}
	}
	return sa.getGenericRequestSchema()
}

// AnalyzeResponseSchema analyzes and generates response schema based on patterns
func (sa *SchemaAnalyzer) AnalyzeResponseSchema(handlerName, path string) map[string]interface{} {
	for _, pattern := range sa.patterns {
		if (pattern.HandlerMatch != nil && pattern.HandlerMatch(handlerName)) ||
			(pattern.PathMatch != nil && pattern.PathMatch(path)) {
			if pattern.ResponseGen != nil {
				return pattern.ResponseGen()
			}
		}
	}
	return sa.getGenericResponseSchema()
}

// getDefaultSchemaPatterns returns the default set of schema patterns
func getDefaultSchemaPatterns() []SchemaPattern {
	return []SchemaPattern{
		// Auth Login Pattern
		{
			Name: "auth_login",
			HandlerMatch: func(handler string) bool {
				lower := strings.ToLower(handler)
				return strings.Contains(lower, "login") || strings.Contains(lower, "signin")
			},
			PathMatch: func(path string) bool {
				return strings.Contains(strings.ToLower(path), "/auth/") &&
					strings.Contains(strings.ToLower(path), "login")
			},
			RequestGen: func() map[string]interface{} {
				return map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"identity": map[string]interface{}{
							"type":        "string",
							"description": "Email or username",
							"example":     "user@example.com",
						},
						"password": map[string]interface{}{
							"type":        "string",
							"description": "User password",
							"example":     "password123",
						},
					},
					"required": []string{"identity", "password"},
				}
			},
			ResponseGen: func() map[string]interface{} {
				return map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"token": map[string]interface{}{
							"type":        "string",
							"description": "JWT authentication token",
							"example":     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
						},
						"record": map[string]interface{}{
							"$ref": "#/components/schemas/UserRecord",
						},
					},
				}
			},
		},
		// Health Check Pattern
		{
			Name: "health_check",
			HandlerMatch: func(handler string) bool {
				lower := strings.ToLower(handler)
				return strings.Contains(lower, "health") || strings.Contains(lower, "status")
			},
			PathMatch: func(path string) bool {
				return strings.Contains(strings.ToLower(path), "/health") ||
					strings.Contains(strings.ToLower(path), "/status")
			},
			ResponseGen: func() map[string]interface{} {
				return map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"status": map[string]interface{}{
							"type":    "string",
							"enum":    []string{"ok", "error"},
							"example": "ok",
						},
						"timestamp": map[string]interface{}{
							"type":    "string",
							"format":  "date-time",
							"example": "2024-01-01T00:00:00Z",
						},
						"version": map[string]interface{}{
							"type":    "string",
							"example": "1.0.0",
						},
					},
				}
			},
		},
		// Collection List Pattern
		{
			Name: "collection_list",
			PathMatch: func(path string) bool {
				return strings.Contains(path, "/collections/") &&
					strings.Contains(path, "/records") &&
					!strings.Contains(path, "{id}")
			},
			ResponseGen: func() map[string]interface{} {
				return map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"page": map[string]interface{}{
							"type":    "integer",
							"example": 1,
						},
						"perPage": map[string]interface{}{
							"type":    "integer",
							"example": 30,
						},
						"totalItems": map[string]interface{}{
							"type":    "integer",
							"example": 100,
						},
						"totalPages": map[string]interface{}{
							"type":    "integer",
							"example": 4,
						},
						"items": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"$ref": "#/components/schemas/Record",
							},
						},
					},
				}
			},
		},
		// Time/Clock Pattern
		{
			Name: "time_endpoint",
			HandlerMatch: func(handler string) bool {
				lower := strings.ToLower(handler)
				return strings.Contains(lower, "time") || strings.Contains(lower, "clock")
			},
			PathMatch: func(path string) bool {
				return strings.Contains(strings.ToLower(path), "/time") ||
					strings.Contains(strings.ToLower(path), "/clock")
			},
			ResponseGen: func() map[string]interface{} {
				return map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"time": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"iso": map[string]interface{}{
									"type":    "string",
									"format":  "date-time",
									"example": "2024-01-01T00:00:00Z",
								},
								"unix": map[string]interface{}{
									"type":    "string",
									"example": "1704067200",
								},
								"unix_nano": map[string]interface{}{
									"type":    "string",
									"example": "1704067200000000000",
								},
								"utc": map[string]interface{}{
									"type":    "string",
									"example": "Mon, 01 Jan 2024 00:00:00 UTC",
								},
							},
						},
					},
				}
			},
		},
	}
}

// getGenericRequestSchema returns a generic request schema
func (sa *SchemaAnalyzer) getGenericRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"description":          "Request body",
		"additionalProperties": true,
	}
}

// getGenericResponseSchema returns a generic response schema
func (sa *SchemaAnalyzer) getGenericResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"description":          "Response data",
		"additionalProperties": true,
	}
}

// TypeInferencer helps infer data types from various sources
type TypeInferencer struct{}

// NewTypeInferencer creates a new type inferencer
func NewTypeInferencer() *TypeInferencer {
	return &TypeInferencer{}
}

// InferTypeFromValue attempts to infer JSON schema type from a value
func (ti *TypeInferencer) InferTypeFromValue(value interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{"type": "null"}
	}

	switch v := value.(type) {
	case bool:
		return map[string]interface{}{"type": "boolean", "example": v}
	case int, int8, int16, int32, int64:
		return map[string]interface{}{"type": "integer", "example": v}
	case uint, uint8, uint16, uint32, uint64:
		return map[string]interface{}{"type": "integer", "example": v}
	case float32, float64:
		return map[string]interface{}{"type": "number", "example": v}
	case string:
		schema := map[string]interface{}{"type": "string", "example": v}
		// Detect special string formats
		if ti.isEmail(v) {
			schema["format"] = "email"
		} else if ti.isDateTime(v) {
			schema["format"] = "date-time"
		} else if ti.isURL(v) {
			schema["format"] = "uri"
		}
		return schema
	case []interface{}:
		return map[string]interface{}{
			"type":  "array",
			"items": ti.inferArrayItemType(v),
		}
	case map[string]interface{}:
		return ti.inferObjectSchema(v)
	default:
		return map[string]interface{}{"type": "object", "additionalProperties": true}
	}
}

// inferArrayItemType infers the type of array items
func (ti *TypeInferencer) inferArrayItemType(arr []interface{}) map[string]interface{} {
	if len(arr) == 0 {
		return map[string]interface{}{"type": "object", "additionalProperties": true}
	}

	// Use the first item to infer type
	return ti.InferTypeFromValue(arr[0])
}

// inferObjectSchema infers schema for object
func (ti *TypeInferencer) inferObjectSchema(obj map[string]interface{}) map[string]interface{} {
	properties := make(map[string]interface{})

	for key, value := range obj {
		properties[key] = ti.InferTypeFromValue(value)
	}

	return map[string]interface{}{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": true,
	}
}

// isEmail checks if string looks like an email
func (ti *TypeInferencer) isEmail(s string) bool {
	return strings.Contains(s, "@") && strings.Contains(s, ".")
}

// isDateTime checks if string looks like a datetime
func (ti *TypeInferencer) isDateTime(s string) bool {
	// Simple heuristic - contains date-like patterns
	return strings.Contains(s, "T") && (strings.Contains(s, "Z") || strings.Contains(s, "+"))
}

// isURL checks if string looks like a URL
func (ti *TypeInferencer) isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// SchemaComponents generates common schema components
type SchemaComponents struct{}

// NewSchemaComponents creates a new schema components generator
func NewSchemaComponents() *SchemaComponents {
	return &SchemaComponents{}
}

// GetCommonSchemas returns commonly used schema definitions
func (sc *SchemaComponents) GetCommonSchemas() map[string]interface{} {
	return map[string]interface{}{
		"Record": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Unique record identifier",
					"example":     "k5r4y36w2hgzm7p",
				},
				"created": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Record creation timestamp",
					"example":     "2024-01-01T00:00:00Z",
				},
				"updated": map[string]interface{}{
					"type":        "string",
					"format":      "date-time",
					"description": "Record last update timestamp",
					"example":     "2024-01-01T00:00:00Z",
				},
			},
			"additionalProperties": true,
		},
		"UserRecord": map[string]interface{}{
			"allOf": []interface{}{
				map[string]interface{}{"$ref": "#/components/schemas/Record"},
				map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"email": map[string]interface{}{
							"type":    "string",
							"format":  "email",
							"example": "user@example.com",
						},
						"verified": map[string]interface{}{
							"type":    "boolean",
							"example": true,
						},
						"username": map[string]interface{}{
							"type":    "string",
							"example": "johndoe",
						},
					},
				},
			},
		},
		"ErrorResponse": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"code": map[string]interface{}{
					"type":    "integer",
					"example": 400,
				},
				"message": map[string]interface{}{
					"type":    "string",
					"example": "Something went wrong",
				},
				"data": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
		"PaginatedResponse": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"page": map[string]interface{}{
					"type":    "integer",
					"minimum": 1,
					"example": 1,
				},
				"perPage": map[string]interface{}{
					"type":    "integer",
					"minimum": 1,
					"maximum": 500,
					"example": 30,
				},
				"totalItems": map[string]interface{}{
					"type":    "integer",
					"minimum": 0,
					"example": 100,
				},
				"totalPages": map[string]interface{}{
					"type":    "integer",
					"minimum": 0,
					"example": 4,
				},
			},
		},
	}
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

// Enhanced pattern matching utilities
func MatchesPattern(text string, patterns []string) bool {
	lower := strings.ToLower(text)
	for _, pattern := range patterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// GetSchemaAnalyzer returns a global schema analyzer instance
func GetSchemaAnalyzer() *SchemaAnalyzer {
	return NewSchemaAnalyzer()
}

// GetTypeInferencer returns a global type inferencer instance
func GetTypeInferencer() *TypeInferencer {
	return NewTypeInferencer()
}

// GetSchemaComponents returns a global schema components generator
func GetSchemaComponents() *SchemaComponents {
	return NewSchemaComponents()
}
