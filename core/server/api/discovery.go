package api

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

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
	cleanName := ExtractHandlerBaseName(name, true)

	return &HandlerInfo{
		Name:        cleanName,
		Package:     packageName,
		FullName:    name,
		Description: GenerateDescription("", "", cleanName),
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

// =============================================================================
// Middleware Analysis
// =============================================================================

// MiddlewareAnalyzer provides utilities for analyzing middleware
type MiddlewareAnalyzer struct{}

// NewMiddlewareAnalyzer creates a new middleware analyzer
func NewMiddlewareAnalyzer() *MiddlewareAnalyzer {
	return &MiddlewareAnalyzer{}
}

// AnalyzeAuth analyzes middleware to determine authentication requirements
func (ma *MiddlewareAnalyzer) AnalyzeAuth(middlewares []interface{}) *AuthInfo {
	if len(middlewares) == 0 {
		return nil
	}

	authType := ma.detectAuthType(middlewares)
	if authType == "" {
		return nil
	}

	return &AuthInfo{
		Required:    true,
		Type:        authType,
		Description: ma.generateAuthDescription(authType),
		Icon:        ma.getAuthIcon(authType),
	}
}

// detectAuthType determines the most restrictive auth type from middleware
func (ma *MiddlewareAnalyzer) detectAuthType(middlewares []interface{}) string {
	// Define auth type hierarchy (most to least restrictive)
	hierarchy := map[string]int{
		"superuser":          4,
		"superuser_or_owner": 3,
		"auth":               2,
		"guest_only":         1,
	}

	mostRestrictive := ""
	highestLevel := 0

	for _, mw := range middlewares {
		authType := ma.extractAuthFromMiddleware(mw)
		if level, exists := hierarchy[authType]; exists && level > highestLevel {
			mostRestrictive = authType
			highestLevel = level
		}
	}

	return mostRestrictive
}

// extractAuthFromMiddleware analyzes middleware to determine auth type
func (ma *MiddlewareAnalyzer) extractAuthFromMiddleware(mw interface{}) string {
	if mw == nil {
		return ""
	}

	mwValue := reflect.ValueOf(mw)
	mwType := reflect.TypeOf(mw)

	// Handle PocketBase hook handler wrappers
	if strings.Contains(mwType.String(), "hook.Handler") {
		return ma.extractAuthFromHookHandler(mwValue)
	}

	// Handle function types
	if mwType.Kind() == reflect.Func {
		funcName := runtime.FuncForPC(mwValue.Pointer()).Name()
		return ma.extractAuthFromFunctionName(funcName)
	}

	// Handle struct types with embedded functions
	if mwType.Kind() == reflect.Struct {
		return ma.extractAuthFromStruct(mwValue)
	}

	return ""
}

// extractAuthFromHookHandler extracts auth type from PocketBase hook handlers
func (ma *MiddlewareAnalyzer) extractAuthFromHookHandler(handlerValue reflect.Value) string {
	if handlerValue.Kind() == reflect.Ptr && !handlerValue.IsNil() {
		elem := handlerValue.Elem()
		if elem.Kind() == reflect.Struct {
			// Look for a field that might contain the actual function
			for i := 0; i < elem.NumField(); i++ {
				field := elem.Field(i)
				if field.Kind() == reflect.Func {
					funcName := runtime.FuncForPC(field.Pointer()).Name()
					if authType := ma.extractAuthFromFunctionName(funcName); authType != "" {
						return authType
					}
				}
			}
		}
	}
	return ""
}

// extractAuthFromFunctionName extracts auth type from function name
func (ma *MiddlewareAnalyzer) extractAuthFromFunctionName(funcName string) string {
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

	return ""
}

// extractAuthFromStruct extracts auth type from struct-based middleware
func (ma *MiddlewareAnalyzer) extractAuthFromStruct(structValue reflect.Value) string {
	structType := structValue.Type()

	// Look at struct name
	if authType := ma.extractAuthFromFunctionName(structType.Name()); authType != "" {
		return authType
	}

	// Look at struct fields
	for i := 0; i < structValue.NumField(); i++ {
		field := structValue.Field(i)
		fieldType := structType.Field(i)

		if authType := ma.extractAuthFromFunctionName(fieldType.Name); authType != "" {
			return authType
		}

		// If field is a function, analyze it
		if field.Kind() == reflect.Func {
			funcName := runtime.FuncForPC(field.Pointer()).Name()
			if authType := ma.extractAuthFromFunctionName(funcName); authType != "" {
				return authType
			}
		}
	}

	return ""
}

// generateAuthDescription generates a description for the auth type
func (ma *MiddlewareAnalyzer) generateAuthDescription(authType string) string {
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
func (ma *MiddlewareAnalyzer) getAuthIcon(authType string) string {
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
// Path Analysis Utilities
// =============================================================================

// PathAnalyzer provides utilities for analyzing URL paths
type PathAnalyzer struct{}

// NewPathAnalyzer creates a new path analyzer
func NewPathAnalyzer() *PathAnalyzer {
	return &PathAnalyzer{}
}

// ExtractTags extracts meaningful tags from a URL path
func (pa *PathAnalyzer) ExtractTags(path string) []string {
	return extractTagsFromPath(path)
}

// GenerateDescription generates a description based on the path structure
func (pa *PathAnalyzer) GenerateDescription(method, path string) string {
	return GenerateDescription(method, path, "")
}

// ExtractParameters extracts parameter information from a path
func (pa *PathAnalyzer) ExtractParameters(path string) []string {
	var params []string
	parts := strings.Split(path, "/")

	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			// PocketBase style parameter
			params = append(params, strings.TrimPrefix(part, ":"))
		} else if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			// OpenAPI style parameter
			param := part[1 : len(part)-1]
			params = append(params, param)
		}
	}

	return params
}

// =============================================================================
// Registry Helper Functions
// =============================================================================

// RegistryHelper provides utilities to help the registry build documentation
type RegistryHelper struct {
	routeAnalyzer      *RouteAnalyzer
	middlewareAnalyzer *MiddlewareAnalyzer
	pathAnalyzer       *PathAnalyzer
}

// NewRegistryHelper creates a new registry helper
func NewRegistryHelper() *RegistryHelper {
	return &RegistryHelper{
		routeAnalyzer:      NewRouteAnalyzer(),
		middlewareAnalyzer: NewMiddlewareAnalyzer(),
		pathAnalyzer:       NewPathAnalyzer(),
	}
}

// AnalyzeRoute provides comprehensive analysis of a route
func (rh *RegistryHelper) AnalyzeRoute(method, path string, handler func(*core.RequestEvent) error, middlewares []interface{}) *RouteAnalysis {
	handlerInfo := rh.routeAnalyzer.AnalyzeHandler(handler)
	authInfo := rh.middlewareAnalyzer.AnalyzeAuth(middlewares)

	return &RouteAnalysis{
		Method:      method,
		Path:        path,
		Handler:     handlerInfo,
		Auth:        authInfo,
		Description: GenerateDescription(method, path, handlerInfo.Name),
		Tags:        GenerateTags(method, path, handlerInfo.Name),
		Parameters:  rh.pathAnalyzer.ExtractParameters(path),
	}
}

// RouteAnalysis contains the result of route analysis
type RouteAnalysis struct {
	Method      string       `json:"method"`
	Path        string       `json:"path"`
	Handler     *HandlerInfo `json:"handler"`
	Auth        *AuthInfo    `json:"auth,omitempty"`
	Description string       `json:"description"`
	Tags        []string     `json:"tags"`
	Parameters  []string     `json:"parameters"`
}
