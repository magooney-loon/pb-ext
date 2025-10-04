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
// Access: http://localhost:8090/api/docs/openapi

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
	Auth        *AuthInfo              `json:"auth,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Handler     string                 `json:"handler_name,omitempty"`
}

// AuthInfo represents detailed authentication requirements for an API endpoint
type AuthInfo struct {
	Required    bool     `json:"required"`
	Type        string   `json:"type"`                  // "guest_only", "auth", "superuser", "superuser_or_owner"
	Collections []string `json:"collections,omitempty"` // For RequireAuth with specific collections
	OwnerParam  string   `json:"owner_param,omitempty"` // For RequireSuperuserOrOwnerAuth
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
}

// APIDocs holds all API documentation
type APIDocs struct {
	Title       string                 `json:"title"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	BaseURL     string                 `json:"base_url"`
	Endpoints   []APIEndpoint          `json:"endpoints"`
	Generated   string                 `json:"generated_at"`
	Components  map[string]interface{} `json:"components,omitempty"`
}

// APIRegistry manages automatic API endpoint documentation
type APIRegistry struct {
	mu        sync.RWMutex
	docs      *APIDocs
	endpoints map[string]APIEndpoint
	enabled   bool
	astParser *ASTParser
}

// RouterWrapper is deprecated - use AutoAPIRouter instead

// NewAPIRegistry creates a new automatic API documentation registry
func NewAPIRegistry() *APIRegistry {
	registry := &APIRegistry{
		docs: &APIDocs{
			Title:       "PocketBase Extension API",
			Version:     "1.0.0",
			Description: "Automatically discovered API endpoints",
			BaseURL:     "/api",
			Endpoints:   []APIEndpoint{},
			Generated:   "runtime",
			Components:  make(map[string]interface{}),
		},
		endpoints: make(map[string]APIEndpoint),
		enabled:   true,
		astParser: NewASTParser(),
	}

	// Initialize AST parser with current project files
	registry.initializeASTParser()

	return registry
}

// EnableAutoDiscovery turns on/off automatic route discovery
func (r *APIRegistry) EnableAutoDiscovery(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = enabled
}

// WrapRouter is deprecated - use EnableAutoDocumentation instead

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

	// Use schema analysis to set initial schemas
	handlerName := r.getHandlerName(handler)
	schemaAnalyzer := GetSchemaAnalyzer()

	// Start with path-based analysis
	reqSchema, respSchema := r.analyzeSchemaFromPath(method, path)
	endpoint.Request = reqSchema
	endpoint.Response = respSchema

	// Enhance with handler-based analysis if still empty
	if endpoint.Request == nil {
		if schema := r.analyzeRequestSchema(handler); schema != nil {
			endpoint.Request = schema
		} else if schema := schemaAnalyzer.AnalyzeRequestSchema(handlerName, path); schema != nil {
			endpoint.Request = schema
		}
	}

	if endpoint.Response == nil {
		if schema := r.analyzeResponseSchema(handler); schema != nil {
			endpoint.Response = schema
		} else if schema := schemaAnalyzer.AnalyzeResponseSchema(handlerName, path); schema != nil {
			endpoint.Response = schema
		}
	}

	// Final step: AST enhancement with absolute authority - this overrides everything above
	r.EnhanceEndpointWithAST(&endpoint)

	key := endpoint.Method + ":" + endpoint.Path
	r.endpoints[key] = endpoint
	r.rebuildEndpoints()
}

// generateDescription creates a human-readable description from path and handler
func (r *APIRegistry) generateDescription(method, path string, handler func(*core.RequestEvent) error) string {
	// First try to get description from handler function name
	// Generate description based on handler function name and path
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

// detectAuthRequirement returns no auth requirement - only middleware detection sets auth
func (r *APIRegistry) detectAuthRequirement(path string) *AuthInfo {
	// No path-based auth guessing - only rely on actual middleware detection
	return nil
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

	// Add special tags based on PocketBase path patterns
	pathLower := strings.ToLower(path)
	if strings.Contains(pathLower, "auth") || strings.Contains(pathLower, "login") || strings.Contains(pathLower, "register") {
		tags = append(tags, "authentication")
	}
	if strings.Contains(pathLower, "user") || strings.Contains(pathLower, "profile") || strings.Contains(pathLower, "account") {
		tags = append(tags, "users")
	}
	if strings.Contains(pathLower, "admin") || strings.Contains(pathLower, "superuser") {
		tags = append(tags, "admin")
	}
	if strings.Contains(pathLower, "collection") || strings.Contains(pathLower, "/records") {
		tags = append(tags, "collections")
	}
	if strings.Contains(pathLower, "file") {
		tags = append(tags, "files")
	}
	if strings.Contains(pathLower, "setting") {
		tags = append(tags, "settings")
	}
	if strings.Contains(pathLower, "log") {
		tags = append(tags, "logs")
	}

	// Ensure we have at least one tag
	if len(tags) == 0 {
		tags = []string{"api"}
	}

	return tags
}

// analyzeRequestSchema attempts to extract request schema using reflection
func (r *APIRegistry) analyzeRequestSchema(handler func(*core.RequestEvent) error) map[string]interface{} {
	if handler == nil {
		return nil
	}

	// Get handler information for analysis
	handlerName := r.getHandlerName(handler)

	// First try to get schema from AST parser
	if r.astParser != nil {
		if requestSchema, _ := r.astParser.GenerateAPISchema(handlerName); requestSchema != nil {
			return requestSchema
		}
	}

	// Fall back to pattern-based analysis
	schema := r.generateRequestSchemaFromPattern(handlerName)
	if schema != nil {
		return schema
	}

	// For GET requests, usually no request body needed
	return nil
}

// analyzeResponseSchema attempts to extract response schema using reflection
func (r *APIRegistry) analyzeResponseSchema(handler func(*core.RequestEvent) error) map[string]interface{} {
	if handler == nil {
		return nil
	}

	// Get handler information for analysis
	handlerName := r.getHandlerName(handler)

	// First try to get schema from AST parser
	if r.astParser != nil {
		if _, responseSchema := r.astParser.GenerateAPISchema(handlerName); responseSchema != nil {
			return responseSchema
		}
	}

	// Fall back to pattern-based analysis
	schema := r.generateResponseSchemaFromPattern(handlerName)
	if schema != nil {
		return schema
	}

	// Return generic response schema for unrecognized patterns
	return r.getGenericResponseSchema()
}

// generateRequestSchemaFromPattern creates request schema based on handler name patterns
func (r *APIRegistry) generateRequestSchemaFromPattern(handlerName string) map[string]interface{} {
	lowerName := strings.ToLower(handlerName)

	// Login/Auth patterns
	if strings.Contains(lowerName, "login") || strings.Contains(lowerName, "signin") {
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
	}

	// Register/Signup patterns
	if strings.Contains(lowerName, "register") || strings.Contains(lowerName, "signup") {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"email": map[string]interface{}{
					"type":    "string",
					"format":  "email",
					"example": "user@example.com",
				},
				"password": map[string]interface{}{
					"type":      "string",
					"minLength": 6,
					"example":   "password123",
				},
				"passwordConfirm": map[string]interface{}{
					"type":    "string",
					"example": "password123",
				},
			},
			"required": []string{"email", "password", "passwordConfirm"},
		}
	}

	// Create patterns
	if strings.Contains(lowerName, "create") || strings.Contains(lowerName, "add") {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": map[string]interface{}{
					"type":                 "object",
					"description":          "Record data to create",
					"additionalProperties": true,
				},
			},
			"required": []string{"data"},
		}
	}

	// Update patterns
	if strings.Contains(lowerName, "update") || strings.Contains(lowerName, "patch") {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": map[string]interface{}{
					"type":                 "object",
					"description":          "Record data to update",
					"additionalProperties": true,
				},
			},
		}
	}

	// File upload patterns
	if strings.Contains(lowerName, "upload") || strings.Contains(lowerName, "file") {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"file": map[string]interface{}{
					"type":        "string",
					"format":      "binary",
					"description": "File to upload",
				},
			},
			"required": []string{"file"},
		}
	}

	return nil
}

// generateResponseSchemaFromPattern creates response schema based on handler name patterns
func (r *APIRegistry) generateResponseSchemaFromPattern(handlerName string) map[string]interface{} {
	lowerName := strings.ToLower(handlerName)

	// Health check patterns
	if strings.Contains(lowerName, "health") || strings.Contains(lowerName, "status") {
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
			},
		}
	}

	// Login/Auth patterns
	if strings.Contains(lowerName, "login") || strings.Contains(lowerName, "signin") || strings.Contains(lowerName, "auth") {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"token": map[string]interface{}{
					"type":        "string",
					"description": "JWT authentication token",
					"example":     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
				},
				"record": map[string]interface{}{
					"type":        "object",
					"description": "Authenticated user record",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":    "string",
							"example": "k5r4y36w2hgzm7p",
						},
						"email": map[string]interface{}{
							"type":    "string",
							"example": "user@example.com",
						},
						"verified": map[string]interface{}{
							"type":    "boolean",
							"example": true,
						},
					},
				},
			},
		}
	}

	// List patterns
	if strings.Contains(lowerName, "list") || strings.Contains(lowerName, "get") && strings.Contains(lowerName, "all") {
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
						"type": "object",
						"properties": map[string]interface{}{
							"id": map[string]interface{}{
								"type":    "string",
								"example": "k5r4y36w2hgzm7p",
							},
							"created": map[string]interface{}{
								"type":    "string",
								"format":  "date-time",
								"example": "2024-01-01T00:00:00Z",
							},
							"updated": map[string]interface{}{
								"type":    "string",
								"format":  "date-time",
								"example": "2024-01-01T00:00:00Z",
							},
						},
					},
				},
			},
		}
	}

	// Single record patterns
	if strings.Contains(lowerName, "get") || strings.Contains(lowerName, "show") || strings.Contains(lowerName, "find") {
		return r.getSingleRecordSchema()
	}

	// Create patterns
	if strings.Contains(lowerName, "create") || strings.Contains(lowerName, "add") {
		return r.getSingleRecordSchema()
	}

	// Update patterns
	if strings.Contains(lowerName, "update") || strings.Contains(lowerName, "patch") {
		return r.getSingleRecordSchema()
	}

	// Delete patterns
	if strings.Contains(lowerName, "delete") || strings.Contains(lowerName, "remove") {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":    "boolean",
					"example": true,
				},
			},
		}
	}

	// Time/Clock patterns
	if strings.Contains(lowerName, "time") || strings.Contains(lowerName, "clock") {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"time": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"iso": map[string]interface{}{
							"type":        "string",
							"format":      "date-time",
							"description": "ISO 8601 formatted time",
							"example":     "2024-01-01T00:00:00Z",
						},
						"unix": map[string]interface{}{
							"type":        "string",
							"description": "Unix timestamp",
							"example":     "1704067200",
						},
						"unix_nano": map[string]interface{}{
							"type":        "string",
							"description": "Unix timestamp in nanoseconds",
							"example":     "1704067200000000000",
						},
						"utc": map[string]interface{}{
							"type":        "string",
							"description": "UTC formatted time",
							"example":     "2024-01-01T00:00:00Z",
						},
					},
				},
			},
		}
	}

	return nil
}

// getSingleRecordSchema returns a generic single record response schema
func (r *APIRegistry) getSingleRecordSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":    "string",
				"example": "k5r4y36w2hgzm7p",
			},
			"created": map[string]interface{}{
				"type":    "string",
				"format":  "date-time",
				"example": "2024-01-01T00:00:00Z",
			},
			"updated": map[string]interface{}{
				"type":    "string",
				"format":  "date-time",
				"example": "2024-01-01T00:00:00Z",
			},
		},
		"additionalProperties": true,
	}
}

// getGenericRequestSchema returns a generic request schema
func (r *APIRegistry) getGenericRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"description":          "Request body",
		"additionalProperties": true,
	}
}

// getGenericResponseSchema returns a generic response schema
func (r *APIRegistry) getGenericResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"description":          "Response data",
		"additionalProperties": true,
	}
}

// analyzeSchemaFromPath generates schema based on URL path patterns
func (r *APIRegistry) analyzeSchemaFromPath(method, path string) (map[string]interface{}, map[string]interface{}) {
	var requestSchema, responseSchema map[string]interface{}

	pathLower := strings.ToLower(path)
	methodUpper := strings.ToUpper(method)

	// Collection record operations
	if strings.Contains(pathLower, "/collections/") && strings.Contains(pathLower, "/records") {
		switch methodUpper {
		case "GET":
			if strings.Contains(path, "{id}") || strings.Contains(path, ":id") {
				responseSchema = r.getSingleRecordSchema()
			} else {
				responseSchema = map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"page":       map[string]interface{}{"type": "integer", "example": 1},
						"perPage":    map[string]interface{}{"type": "integer", "example": 30},
						"totalItems": map[string]interface{}{"type": "integer", "example": 100},
						"totalPages": map[string]interface{}{"type": "integer", "example": 4},
						"items": map[string]interface{}{
							"type":  "array",
							"items": r.getSingleRecordSchema(),
						},
					},
				}
			}
		case "POST":
			requestSchema = map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true,
				"description":          "Record data to create",
			}
			responseSchema = r.getSingleRecordSchema()
		case "PATCH", "PUT":
			requestSchema = map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true,
				"description":          "Record data to update",
			}
			responseSchema = r.getSingleRecordSchema()
		case "DELETE":
			responseSchema = map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"success": map[string]interface{}{"type": "boolean", "example": true},
				},
			}
		}
	}

	// Auth endpoints
	if strings.Contains(pathLower, "auth") {
		if strings.Contains(pathLower, "login") || strings.Contains(pathLower, "signin") {
			requestSchema = map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"identity": map[string]interface{}{"type": "string", "example": "user@example.com"},
					"password": map[string]interface{}{"type": "string", "example": "password123"},
				},
				"required": []string{"identity", "password"},
			}
			responseSchema = map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"token":  map[string]interface{}{"type": "string"},
					"record": r.getSingleRecordSchema(),
				},
			}
		}
	}

	return requestSchema, responseSchema
}

// enhanceEndpointWithPathAnalysis enhances endpoint with path-based schema analysis
func (r *APIRegistry) enhanceEndpointWithPathAnalysis(endpoint *APIEndpoint) {
	if endpoint.Request == nil || endpoint.Response == nil {
		reqSchema, respSchema := r.analyzeSchemaFromPath(endpoint.Method, endpoint.Path)

		if endpoint.Request == nil && reqSchema != nil {
			endpoint.Request = reqSchema
		}

		if endpoint.Response == nil && respSchema != nil {
			endpoint.Response = respSchema
		}
	}
}

// GetDocs returns the complete API documentation
func (r *APIRegistry) GetDocs() *APIDocs {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getEnhancedDocs()
}

// GetDocsWithComponents returns API documentation with schema components
func (r *APIRegistry) GetDocsWithComponents() *APIDocs {
	r.mu.RLock()
	defer r.mu.RUnlock()

	docs := r.getEnhancedDocs()
	docs.Components = r.generateComponents()
	return docs
}

// getEnhancedDocs returns enhanced documentation (internal method, must be called with lock)
func (r *APIRegistry) getEnhancedDocs() *APIDocs {
	// Update generated timestamp
	r.docs.Generated = "runtime"

	// Skip additional path analysis enhancement since AST enhancement provides better results
	// for i := range r.docs.Endpoints {
	//     r.enhanceEndpointWithPathAnalysis(&r.docs.Endpoints[i])
	// }

	return r.docs
}

// generateComponents creates schema components for the API documentation
func (r *APIRegistry) generateComponents() map[string]interface{} {
	return map[string]interface{}{
		"schemas": r.generateSchemasWithAST(),
		"responses": map[string]interface{}{
			"ErrorResponse": map[string]interface{}{
				"description": "Error response",
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"$ref": "#/components/schemas/ErrorResponse",
						},
					},
				},
			},
			"SuccessResponse": map[string]interface{}{
				"description": "Success response",
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"success": map[string]interface{}{
									"type":    "boolean",
									"example": true,
								},
							},
						},
					},
				},
			},
		},
		"parameters": map[string]interface{}{
			"PageParam": map[string]interface{}{
				"name":        "page",
				"in":          "query",
				"description": "Page number for pagination",
				"schema": map[string]interface{}{
					"type":    "integer",
					"minimum": 1,
					"default": 1,
				},
			},
			"PerPageParam": map[string]interface{}{
				"name":        "perPage",
				"in":          "query",
				"description": "Number of items per page",
				"schema": map[string]interface{}{
					"type":    "integer",
					"minimum": 1,
					"maximum": 500,
					"default": 30,
				},
			},
			"RecordIdParam": map[string]interface{}{
				"name":        "id",
				"in":          "path",
				"required":    true,
				"description": "Record identifier",
				"schema": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"securitySchemes": map[string]interface{}{
			"bearerAuth": map[string]interface{}{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "JWT",
			},
		},
	}
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

	// OpenAPI documentation endpoint with components - NOT auto-documented
	e.Router.GET("/api/docs/openapi", func(c *core.RequestEvent) error {
		docs := registry.GetDocsWithComponents()
		return c.JSON(http.StatusOK, docs)
	})
}

// registerBuiltinRoutes is completely removed - only user routes are documented
func (s *Server) registerBuiltinRoutes() {
	// Intentionally empty - we only document user-registered routes
}

// AutoRegisterRoute can be used to manually register routes that bypass normal registration
func AutoRegisterRoute(method, path string, handler func(*core.RequestEvent) error) {
	globalAPIRegistry.autoRegisterRoute(method, path, handler)
}

// initializeASTParser initializes the AST parser with project files
func (r *APIRegistry) initializeASTParser() {
	if r.astParser == nil {
		return
	}

	// Try different possible paths for main.go
	possiblePaths := []string{
		"cmd/server/main.go",
		"main.go",
		"./cmd/server/main.go",
		"./main.go",
	}

	var lastErr error
	parsed := false

	for _, path := range possiblePaths {
		err := r.astParser.ParseFile(path)
		if err == nil {
			parsed = true
			// Debug: log successful parsing
			if handler, exists := r.astParser.GetHandlerByName("createUserHandler"); exists {
				_ = handler // Successfully found handler
			}
			break
		}
		lastErr = err
	}

	if !parsed && lastErr != nil {
		// Debug: Could add logging here if needed
		_ = lastErr // Fall back to pattern matching
	}
}

// EnhanceEndpointWithAST enhances an endpoint using AST analysis
func (r *APIRegistry) EnhanceEndpointWithAST(endpoint *APIEndpoint) {
	if r.astParser == nil {
		return
	}

	r.astParser.EnhanceEndpoint(endpoint)
}

// GetParsedStructs returns all structs parsed by the AST parser
func (r *APIRegistry) GetParsedStructs() map[string]*StructInfo {
	if r.astParser == nil {
		return make(map[string]*StructInfo)
	}
	return r.astParser.GetAllStructs()
}

// GetParsedHandlers returns all handlers parsed by the AST parser
func (r *APIRegistry) GetParsedHandlers() map[string]*ASTHandlerInfo {
	if r.astParser == nil {
		return make(map[string]*ASTHandlerInfo)
	}
	return r.astParser.GetAllHandlers()
}

// generateSchemasWithAST generates schemas including AST-parsed structs
func (r *APIRegistry) generateSchemasWithAST() map[string]interface{} {
	schemas := map[string]interface{}{
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
		"Record": map[string]interface{}{
			"type":                 "object",
			"additionalProperties": true,
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
		},
		"UserRecord": map[string]interface{}{
			"allOf": []interface{}{
				map[string]interface{}{"$ref": "#/components/schemas/Record"},
				map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"username": map[string]interface{}{
							"type":    "string",
							"example": "johndoe",
						},
						"email": map[string]interface{}{
							"type":    "string",
							"format":  "email",
							"example": "user@example.com",
						},
						"verified": map[string]interface{}{
							"type":    "boolean",
							"example": true,
						},
					},
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

	// Add AST-parsed struct schemas
	if r.astParser != nil {
		parsedStructs := r.astParser.GetAllStructs()
		for name, structInfo := range parsedStructs {
			schemas[name] = structInfo.JSONSchema
		}
	}

	return schemas
}
