package api

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Auto-Router Implementation
// =============================================================================

// NewAutoAPIRouter creates a new auto-documenting API router
func NewAutoAPIRouter(router interface{}, registry *APIRegistry) *AutoAPIRouter {
	return &AutoAPIRouter{
		router:   router,
		registry: registry,
	}
}

// GET registers a GET route with automatic documentation
func (r *AutoAPIRouter) GET(path string, handler func(*core.RequestEvent) error) *RouteChain {
	return r.registerRoute("GET", path, handler)
}

// POST registers a POST route with automatic documentation
func (r *AutoAPIRouter) POST(path string, handler func(*core.RequestEvent) error) *RouteChain {
	return r.registerRoute("POST", path, handler)
}

// PUT registers a PUT route with automatic documentation
func (r *AutoAPIRouter) PUT(path string, handler func(*core.RequestEvent) error) *RouteChain {
	return r.registerRoute("PUT", path, handler)
}

// PATCH registers a PATCH route with automatic documentation
func (r *AutoAPIRouter) PATCH(path string, handler func(*core.RequestEvent) error) *RouteChain {
	return r.registerRoute("PATCH", path, handler)
}

// DELETE registers a DELETE route with automatic documentation
func (r *AutoAPIRouter) DELETE(path string, handler func(*core.RequestEvent) error) *RouteChain {
	return r.registerRoute("DELETE", path, handler)
}

// HEAD registers a HEAD route with automatic documentation
func (r *AutoAPIRouter) HEAD(path string, handler func(*core.RequestEvent) error) *RouteChain {
	return r.registerRoute("HEAD", path, handler)
}

// OPTIONS registers an OPTIONS route with automatic documentation
func (r *AutoAPIRouter) OPTIONS(path string, handler func(*core.RequestEvent) error) *RouteChain {
	return r.registerRoute("OPTIONS", path, handler)
}

// ANY registers a route for any HTTP method with automatic documentation
func (r *AutoAPIRouter) ANY(path string, handler func(*core.RequestEvent) error) *RouteChain {
	// Register for multiple methods
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	var lastChain *RouteChain

	for _, method := range methods {
		lastChain = r.registerRoute(method, path, handler)
	}

	return lastChain
}

// Group creates a route group with the given prefix
func (r *AutoAPIRouter) Group(prefix string) *AutoAPIRouter {
	// Create a new router for the group
	// Implementation depends on the underlying router interface
	return &AutoAPIRouter{
		router:   r.router, // This would need to be adapted for actual router grouping
		registry: r.registry,
	}
}

// registerRoute is the core method that registers routes and auto-documents them
func (r *AutoAPIRouter) registerRoute(method, path string, handler func(*core.RequestEvent) error) *RouteChain {
	// Register with the underlying router using reflection
	if err := r.callRouterMethod(method, path, handler); err != nil {
		fmt.Printf("[ERROR] registerRoute - Failed to register route %s %s: %v\n", method, path, err)
		// Continue with documentation registration even if router registration fails
	}

	// Auto-register with documentation system
	if r.registry != nil {
		r.registry.AutoRegisterRoute(method, path, handler)
	}

	// Return a route chain for middleware binding
	return &RouteChain{
		route:    nil, // This would be the actual route from the underlying router
		method:   method,
		path:     path,
		handler:  handler,
		registry: r.registry,
	}
}

// callRouterMethod calls the appropriate method on the underlying router using reflection
func (r *AutoAPIRouter) callRouterMethod(method, path string, handler func(*core.RequestEvent) error) error {
	fmt.Printf("[DEBUG] callRouterMethod - Attempting to register: %s %s\n", method, path)

	if r.router == nil {
		fmt.Printf("[DEBUG] callRouterMethod - Error: router is nil\n")
		return fmt.Errorf("router is nil")
	}

	routerValue := reflect.ValueOf(r.router)
	fmt.Printf("[DEBUG] callRouterMethod - Router type: %v\n", routerValue.Type())

	methodValue := routerValue.MethodByName(strings.ToUpper(method))

	if !methodValue.IsValid() {
		fmt.Printf("[DEBUG] callRouterMethod - Error: Method %s not found on router type %v\n", strings.ToUpper(method), routerValue.Type())
		return fmt.Errorf("method %s not found on router", method)
	}

	fmt.Printf("[DEBUG] callRouterMethod - Found method %s, calling with args: path=%s, handler=%p\n", strings.ToUpper(method), path, handler)

	// Call the method with path and handler
	args := []reflect.Value{
		reflect.ValueOf(path),
		reflect.ValueOf(handler),
	}

	// Use recover to catch any panics from the reflection call
	var results []reflect.Value
	var callErr error

	func() {
		defer func() {
			if r := recover(); r != nil {
				callErr = fmt.Errorf("panic during method call: %v", r)
			}
		}()
		results = methodValue.Call(args)
	}()

	if callErr != nil {
		fmt.Printf("[DEBUG] callRouterMethod - Error during method call: %v\n", callErr)
		return callErr
	}

	fmt.Printf("[DEBUG] callRouterMethod - Method call successful, results count: %d\n", len(results))

	// Store the returned route for middleware binding if available
	if len(results) > 0 && !results[0].IsNil() {
		fmt.Printf("[DEBUG] callRouterMethod - Route result received: %v\n", results[0].Type())
		// The actual route object would be stored here for middleware binding
	}

	fmt.Printf("[DEBUG] callRouterMethod - Successfully registered: %s %s\n", method, path)
	return nil
}

// =============================================================================
// Route Chain Implementation
// =============================================================================

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
			}
			bindMethod.Call(args)
		}
	}

	// Analyze middleware for documentation
	for _, mw := range middlewares {
		authType := rc.extractAuthMiddlewareType(mw)
		if authType != "" {
			rc.middleware = append(rc.middleware, authType)
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

	// Analyze function for auth patterns
	funcName := GetHandlerName(middlewareFunc)
	authType := rc.extractAuthFromFunctionName(funcName)
	if authType != "" {
		rc.middleware = append(rc.middleware, authType)
		rc.updateEndpointAuth()
	}

	return rc
}

// =============================================================================
// Middleware Analysis
// =============================================================================

// extractAuthMiddlewareType analyzes middleware to determine auth type
func (rc *RouteChain) extractAuthMiddlewareType(mw interface{}) string {
	if mw == nil {
		return ""
	}

	mwValue := reflect.ValueOf(mw)
	mwType := reflect.TypeOf(mw)

	// Handle PocketBase hook handler wrappers
	if strings.Contains(mwType.String(), "hook.Handler") {
		return rc.extractAuthFromHookHandler(mwValue)
	}

	// Handle function types
	if mwType.Kind() == reflect.Func {
		funcName := runtime.FuncForPC(mwValue.Pointer()).Name()
		return rc.extractAuthFromFunctionName(funcName)
	}

	// Handle struct types with embedded functions
	if mwType.Kind() == reflect.Struct {
		return rc.extractAuthFromStruct(mwValue)
	}

	return ""
}

// extractAuthFromHookHandler extracts auth type from PocketBase hook handlers
func (rc *RouteChain) extractAuthFromHookHandler(handlerValue reflect.Value) string {
	if handlerValue.Kind() == reflect.Ptr && !handlerValue.IsNil() {
		elem := handlerValue.Elem()
		if elem.Kind() == reflect.Struct {
			// Look for a field that might contain the actual function
			for i := 0; i < elem.NumField(); i++ {
				field := elem.Field(i)
				if field.Kind() == reflect.Func {
					funcName := runtime.FuncForPC(field.Pointer()).Name()
					if authType := rc.extractAuthFromFunctionName(funcName); authType != "" {
						return authType
					}
				}
			}
		}
	}
	return ""
}

// extractAuthFromFunctionName extracts auth type from function name
func (rc *RouteChain) extractAuthFromFunctionName(funcName string) string {
	funcName = strings.ToLower(funcName)

	if strings.Contains(funcName, "requireguestonly") {
		return "guest_only"
	}
	if strings.Contains(funcName, "requiresuperuserorownerauth") {
		return "superuser_or_owner"
	}
	if strings.Contains(funcName, "requiresuperuserauth") {
		return "superuser"
	}
	if strings.Contains(funcName, "requireauth") {
		return "auth"
	}
	if strings.Contains(funcName, "requirerecordauth") {
		return "auth"
	}

	// Check for common auth patterns
	authPatterns := []string{
		"auth", "authenticate", "authorize", "login", "token",
		"jwt", "bearer", "session", "permission", "access",
	}

	for _, pattern := range authPatterns {
		if strings.Contains(funcName, pattern) {
			return "auth"
		}
	}

	return ""
}

// extractAuthFromStruct extracts auth type from struct-based middleware
func (rc *RouteChain) extractAuthFromStruct(structValue reflect.Value) string {
	structType := structValue.Type()

	// Look at struct name
	if authType := rc.extractAuthFromFunctionName(structType.Name()); authType != "" {
		return authType
	}

	// Look at struct fields
	for i := 0; i < structValue.NumField(); i++ {
		field := structValue.Field(i)
		fieldType := structType.Field(i)

		if authType := rc.extractAuthFromFunctionName(fieldType.Name); authType != "" {
			return authType
		}

		// If field is a function, analyze it
		if field.Kind() == reflect.Func {
			funcName := runtime.FuncForPC(field.Pointer()).Name()
			if authType := rc.extractAuthFromFunctionName(funcName); authType != "" {
				return authType
			}
		}
	}

	return ""
}

// updateEndpointAuth updates the endpoint's auth information based on detected middleware
func (rc *RouteChain) updateEndpointAuth() {
	if rc.registry == nil {
		return
	}

	// Get the current endpoint
	if endpoint, exists := rc.registry.GetEndpoint(rc.method, rc.path); exists {
		// Create or update auth info
		if endpoint.Auth == nil {
			endpoint.Auth = &AuthInfo{}
		}

		// Determine the most restrictive auth type
		authType := rc.getMostRestrictiveAuthType()
		endpoint.Auth.Required = authType != ""
		endpoint.Auth.Type = authType
		endpoint.Auth.Description = rc.generateAuthDescription(authType)
		endpoint.Auth.Icon = rc.getAuthIcon(authType)

		// Re-register the updated endpoint
		rc.registry.RegisterEndpoint(*endpoint)
	}
}

// getMostRestrictiveAuthType returns the most restrictive auth type from middleware
func (rc *RouteChain) getMostRestrictiveAuthType() string {
	if len(rc.middleware) == 0 {
		return ""
	}

	// Define auth type hierarchy (most to least restrictive)
	hierarchy := map[string]int{
		"superuser":          4,
		"superuser_or_owner": 3,
		"auth":               2,
		"guest_only":         1,
	}

	mostRestrictive := ""
	highestLevel := 0

	for _, authType := range rc.middleware {
		if level, exists := hierarchy[authType]; exists && level > highestLevel {
			mostRestrictive = authType
			highestLevel = level
		}
	}

	return mostRestrictive
}

// generateAuthDescription generates a description for the auth type
func (rc *RouteChain) generateAuthDescription(authType string) string {
	descriptions := map[string]string{
		"guest_only":         "Requires no authentication (guest access only)",
		"auth":               "Requires user authentication",
		"superuser":          "Requires superuser privileges",
		"superuser_or_owner": "Requires superuser privileges or resource ownership",
	}

	if desc, exists := descriptions[authType]; exists {
		return desc
	}
	return "Authentication required"
}

// getAuthIcon returns an appropriate icon for the auth type
func (rc *RouteChain) getAuthIcon(authType string) string {
	icons := map[string]string{
		"guest_only":         "👤",
		"auth":               "🔒",
		"superuser":          "👑",
		"superuser_or_owner": "🔑",
	}

	if icon, exists := icons[authType]; exists {
		return icon
	}
	return "🔒"
}

// =============================================================================
// Global Functions for Easy Access
// =============================================================================

// EnableAutoDocumentation wraps a router with automatic documentation capabilities
func EnableAutoDocumentation(e *core.ServeEvent) *AutoAPIRouter {
	// Initialize global documentation system to ensure same registry is used
	GetGlobalDocumentationSystem()
	registry := GetGlobalRegistry()
	return NewAutoAPIRouter(e.Router, registry)
}

// EnableAutoDocumentationWithRegistry wraps a router with a specific registry
func EnableAutoDocumentationWithRegistry(e *core.ServeEvent, registry *APIRegistry) *AutoAPIRouter {
	return NewAutoAPIRouter(e.Router, registry)
}

// AutoRegisterRoute provides backward compatible global route registration
func AutoRegisterRoute(method, path string, handler func(*core.RequestEvent) error) {
	GetGlobalRegistry().AutoRegisterRoute(method, path, handler)
}

// ConfigureAutoDiscovery configures the global registry's auto-discovery settings
func ConfigureAutoDiscovery(config *AutoDiscoveryConfig) {
	registry := GetGlobalRegistry()
	if registry.config != nil {
		registry.config.AutoDiscovery = config
	}
}

// GetDiscoveredEndpoints returns all endpoints discovered by the global registry
func GetDiscoveredEndpoints() []APIEndpoint {
	registry := GetGlobalRegistry()
	docs := registry.GetDocs()
	return docs.Endpoints
}

// GetEndpointByPath retrieves a specific endpoint by method and path from global registry
func GetEndpointByPath(method, path string) (*APIEndpoint, bool) {
	registry := GetGlobalRegistry()
	return registry.GetEndpoint(method, path)
}

// GetEndpointsByTag returns all endpoints with a specific tag from global registry
func GetEndpointsByTag(tag string) []APIEndpoint {
	registry := GetGlobalRegistry()
	return registry.GetEndpointsByTag(tag)
}

// =============================================================================
// Route Analysis Utilities
// =============================================================================

// RouteAnalyzer provides utilities for analyzing routes and handlers
type RouteAnalyzer struct{}

// NewRouteAnalyzer creates a new route analyzer
func NewRouteAnalyzer() *RouteAnalyzer {
	return &RouteAnalyzer{}
}

// AnalyzeHandler analyzes a handler function and returns information about it
func (ra *RouteAnalyzer) AnalyzeHandler(handler func(*core.RequestEvent) error) *HandlerInfo {
	if handler == nil {
		return &HandlerInfo{
			Name:        "unknown",
			Package:     "",
			Description: "Unknown handler",
		}
	}

	name := GetHandlerName(handler)
	packageName := ra.extractPackageName(name)
	cleanName := ExtractBaseNameFromHandler(name)

	return &HandlerInfo{
		Name:        cleanName,
		Package:     packageName,
		Description: DescriptionFromHandlerName(cleanName),
	}
}

// extractPackageName extracts the package name from a full function name
func (ra *RouteAnalyzer) extractPackageName(fullName string) string {
	parts := strings.Split(fullName, ".")
	if len(parts) > 1 {
		// Return the package part, excluding the function name
		return strings.Join(parts[:len(parts)-1], ".")
	}
	return ""
}

// PathAnalyzer provides utilities for analyzing URL paths
type PathAnalyzer struct{}

// NewPathAnalyzer creates a new path analyzer
func NewPathAnalyzer() *PathAnalyzer {
	return &PathAnalyzer{}
}

// ExtractTags extracts meaningful tags from a URL path
func (pa *PathAnalyzer) ExtractTags(path string) []string {
	return generateTagsFromPath(path)
}

// GenerateDescription generates a description based on the path structure
func (pa *PathAnalyzer) GenerateDescription(method, path string) string {
	return DescriptionFromPath(method, path)
}
