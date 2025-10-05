package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// APIRegistry manages automatic API endpoint documentation with clean separation of concerns
type APIRegistry struct {
	mu              sync.RWMutex
	config          *APIDocsConfig
	docs            *APIDocs
	endpoints       map[string]APIEndpoint
	astParser       ASTParserInterface
	schemaGenerator SchemaGeneratorInterface
}

// NewAPIRegistry creates a new API documentation registry with dependency injection
func NewAPIRegistry(config *APIDocsConfig, astParser ASTParserInterface, schemaGenerator SchemaGeneratorInterface) *APIRegistry {
	if config == nil {
		config = DefaultAPIDocsConfig()
	}

	registry := &APIRegistry{
		config: config,
		docs: &APIDocs{
			Title:       config.Title,
			Version:     config.Version,
			Description: config.Description,
			BaseURL:     config.BaseURL,
			Endpoints:   []APIEndpoint{},
			Generated:   time.Now().Format(time.RFC3339),
			Components:  make(map[string]interface{}),
		},
		endpoints:       make(map[string]APIEndpoint),
		astParser:       astParser,
		schemaGenerator: schemaGenerator,
	}

	return registry
}

// RegisterEndpoint manually registers an API endpoint
func (r *APIRegistry) RegisterEndpoint(endpoint APIEndpoint) {
	if !r.config.Enabled {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.endpointKey(endpoint.Method, endpoint.Path)
	r.endpoints[key] = endpoint
	r.rebuildEndpointsList()
}

// AutoRegisterRoute automatically registers a route during runtime discovery
func (r *APIRegistry) AutoRegisterRoute(method, path string, handler func(*core.RequestEvent) error) {
	if !r.config.Enabled || !r.config.AutoDiscovery.Enabled {
		return
	}

	endpoint := r.createEndpointFromRoute(method, path, handler)
	r.enhanceEndpointWithAnalysis(&endpoint)
	r.RegisterEndpoint(endpoint)
}

// GetDocs returns the current API documentation
func (r *APIRegistry) GetDocs() *APIDocs {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid race conditions
	docsCopy := &APIDocs{
		Title:       r.docs.Title,
		Version:     r.docs.Version,
		Description: r.docs.Description,
		BaseURL:     r.docs.BaseURL,
		Generated:   r.docs.Generated,
		Endpoints:   make([]APIEndpoint, len(r.docs.Endpoints)),
		Components:  make(map[string]interface{}),
	}

	copy(docsCopy.Endpoints, r.docs.Endpoints)
	for k, v := range r.docs.Components {
		docsCopy.Components[k] = v
	}

	return docsCopy
}

// GetDocsWithComponents returns documentation with generated component schemas
func (r *APIRegistry) GetDocsWithComponents() *APIDocs {
	docs := r.GetDocs()

	if r.schemaGenerator != nil {
		docs.Components = r.schemaGenerator.GenerateComponentSchemas()
	}

	return docs
}

// GetEndpoint retrieves a specific endpoint by method and path
func (r *APIRegistry) GetEndpoint(method, path string) (*APIEndpoint, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := r.endpointKey(method, path)
	endpoint, exists := r.endpoints[key]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modifications
	endpointCopy := endpoint
	return &endpointCopy, true
}

// GetEndpointsByTag returns all endpoints that have the specified tag
func (r *APIRegistry) GetEndpointsByTag(tag string) []APIEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matchingEndpoints []APIEndpoint
	for _, endpoint := range r.endpoints {
		for _, endpointTag := range endpoint.Tags {
			if endpointTag == tag {
				matchingEndpoints = append(matchingEndpoints, endpoint)
				break
			}
		}
	}

	return matchingEndpoints
}

// UpdateConfig updates the registry configuration
func (r *APIRegistry) UpdateConfig(config *APIDocsConfig) {
	if config == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.config = config
	r.docs.Title = config.Title
	r.docs.Version = config.Version
	r.docs.Description = config.Description
	r.docs.BaseURL = config.BaseURL
}

// ClearEndpoints removes all registered endpoints
func (r *APIRegistry) ClearEndpoints() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.endpoints = make(map[string]APIEndpoint)
	r.docs.Endpoints = []APIEndpoint{}
}

// =============================================================================
// Private Helper Methods
// =============================================================================

// createEndpointFromRoute creates an APIEndpoint from route information
func (r *APIRegistry) createEndpointFromRoute(method, path string, handler func(*core.RequestEvent) error) APIEndpoint {
	endpoint := APIEndpoint{
		Method:      method,
		Path:        path,
		Description: r.generateDescription(method, path, handler),
		Tags:        r.generateTags(method, path, handler),
		Handler:     r.getHandlerName(handler),
	}

	return endpoint
}

// enhanceEndpointWithAnalysis enhances an endpoint with AST and schema analysis
func (r *APIRegistry) enhanceEndpointWithAnalysis(endpoint *APIEndpoint) {
	// Enhance with AST analysis if available
	if r.astParser != nil {
		if err := r.astParser.EnhanceEndpoint(endpoint); err != nil {
			// Log error but don't fail - fallback to basic info
		}
	}

	// Generate schemas if schema generator is available
	if r.schemaGenerator != nil {
		if requestSchema, err := r.schemaGenerator.AnalyzeRequestSchema(endpoint); err == nil {
			endpoint.Request = requestSchema
		}

		// Only set response schema if AST didn't already provide one
		if endpoint.Response == nil {
			fmt.Printf("🔄 No AST schema found for %s %s, generating from path patterns\n", endpoint.Method, endpoint.Path)
			if responseSchema, err := r.schemaGenerator.AnalyzeResponseSchema(endpoint); err == nil {
				endpoint.Response = responseSchema
				fmt.Printf("📋 Applied path-based schema for %s %s\n", endpoint.Method, endpoint.Path)
			}
		} else {
			fmt.Printf("✅ Preserving AST schema for %s %s (has %d properties)\n", endpoint.Method, endpoint.Path, len(endpoint.Response))
		}
	}
}

// generateDescription generates a description for an endpoint
func (r *APIRegistry) generateDescription(method, path string, handler func(*core.RequestEvent) error) string {
	if r.astParser != nil {
		handlerName := r.getHandlerName(handler)
		if desc := r.astParser.GetHandlerDescription(handlerName); desc != "" {
			return desc
		}
	}

	// Fallback to path-based description
	return r.descriptionFromPath(method, path)
}

// generateTags generates tags for an endpoint
func (r *APIRegistry) generateTags(method, path string, handler func(*core.RequestEvent) error) []string {
	if !r.config.AutoDiscovery.GenerateTags {
		return []string{}
	}

	var tags []string

	// Get tags from AST analysis
	if r.astParser != nil {
		handlerName := r.getHandlerName(handler)
		if astTags := r.astParser.GetHandlerTags(handlerName); len(astTags) > 0 {
			tags = append(tags, astTags...)
		}
	}

	// Add path-based tags if no AST tags found
	if len(tags) == 0 {
		tags = r.generateTagsFromPath(path)
	}

	return tags
}

// generateTagsFromPath generates tags based on the URL path
func (r *APIRegistry) generateTagsFromPath(path string) []string {
	var tags []string
	parts := strings.Split(strings.Trim(path, "/"), "/")

	for _, part := range parts {
		if part != "" && !strings.HasPrefix(part, ":") && !strings.HasPrefix(part, "{") {
			// Clean up the part and add as tag
			tag := strings.ToLower(part)
			tag = strings.ReplaceAll(tag, "_", "-")
			tags = append(tags, tag)
		}
	}

	if len(tags) == 0 {
		tags = append(tags, "general")
	}

	return tags
}

// getHandlerName extracts the name of a handler function
func (r *APIRegistry) getHandlerName(handler func(*core.RequestEvent) error) string {
	return ExtractHandlerNameFromPath(GetHandlerName(handler))
}

// descriptionFromPath generates a description from the HTTP method and path
func (r *APIRegistry) descriptionFromPath(method, path string) string {
	// Clean up the path for description
	cleanPath := strings.ReplaceAll(path, "/", " ")
	cleanPath = strings.ReplaceAll(cleanPath, "_", " ")
	cleanPath = strings.Title(strings.TrimSpace(cleanPath))

	switch strings.ToUpper(method) {
	case "GET":
		return fmt.Sprintf("Get %s", cleanPath)
	case "POST":
		return fmt.Sprintf("Create %s", cleanPath)
	case "PUT":
		return fmt.Sprintf("Update %s", cleanPath)
	case "PATCH":
		return fmt.Sprintf("Modify %s", cleanPath)
	case "DELETE":
		return fmt.Sprintf("Delete %s", cleanPath)
	default:
		return fmt.Sprintf("%s %s", method, cleanPath)
	}
}

// endpointKey generates a unique key for an endpoint
func (r *APIRegistry) endpointKey(method, path string) string {
	return fmt.Sprintf("%s:%s", strings.ToUpper(method), path)
}

// rebuildEndpointsList rebuilds the endpoints slice from the map (should be called with lock held)
func (r *APIRegistry) rebuildEndpointsList() {
	endpoints := make([]APIEndpoint, 0, len(r.endpoints))
	for _, endpoint := range r.endpoints {
		endpoints = append(endpoints, endpoint)
	}

	// Sort endpoints by path then method for consistent ordering
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path == endpoints[j].Path {
			return endpoints[i].Method < endpoints[j].Method
		}
		return endpoints[i].Path < endpoints[j].Path
	})

	r.docs.Endpoints = endpoints
	r.docs.Generated = time.Now().Format(time.RFC3339)
}

// =============================================================================
// Server Integration Methods
// =============================================================================

// RegisterAPIDocsRoutes registers the API documentation routes with a PocketBase server
func (r *APIRegistry) RegisterAPIDocsRoutes(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.GET("/api/docs/openapi", func(c *core.RequestEvent) error {
			docs := r.GetDocsWithComponents()
			return c.JSON(http.StatusOK, docs)
		})

		e.Router.GET("/api/docs/stats", func(c *core.RequestEvent) error {
			stats := GetAPIStats()
			return c.JSON(http.StatusOK, stats)
		})

		return e.Next()
	})
}

// =============================================================================
// Global Registry Management (for backward compatibility)
// =============================================================================

var globalAPIRegistry *APIRegistry
var globalRegistryMu sync.RWMutex

// GetGlobalRegistry returns the global API registry instance
func GetGlobalRegistry() *APIRegistry {
	globalRegistryMu.RLock()
	if globalAPIRegistry != nil {
		globalRegistryMu.RUnlock()
		return globalAPIRegistry
	}
	globalRegistryMu.RUnlock()

	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()

	if globalAPIRegistry == nil {
		// Create with default dependencies - these should be injected properly in real usage
		config := DefaultAPIDocsConfig()
		astParser := NewASTParser()
		schemaGenerator := NewSchemaGenerator(astParser)
		globalAPIRegistry = NewAPIRegistry(config, astParser, schemaGenerator)
	}

	return globalAPIRegistry
}

// SetGlobalRegistry sets the global API registry instance
func SetGlobalRegistry(registry *APIRegistry) {
	globalRegistryMu.Lock()
	defer globalRegistryMu.Unlock()
	globalAPIRegistry = registry
}
