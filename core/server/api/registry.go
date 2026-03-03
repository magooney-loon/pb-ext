package api

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// APIRegistry manages automatic API endpoint documentation with clean separation of concerns
type APIRegistry struct {
	mu              sync.RWMutex
	version         string
	config          *APIDocsConfig
	docs            *APIDocs
	endpoints       map[string]APIEndpoint
	astParser       ASTParserInterface
	schemaGenerator SchemaGeneratorInterface
	pathPrefix      string // prefix to strip from paths (derived from server URL)

	// Spec cache — invalidated whenever an endpoint is registered or removed
	cachedSpecDocs *APIDocs
	specDirty      bool
}

// buildOpenAPIInfo builds an OpenAPIInfo and optional ExternalDocs from an APIDocsConfig.
func buildOpenAPIInfo(config *APIDocsConfig) (*OpenAPIInfo, *OpenAPIExternalDocs) {
	contactName := config.ContactName
	if contactName == "" {
		contactName = "API Support"
	}

	info := &OpenAPIInfo{
		Title:          config.Title,
		Version:        config.Version,
		Description:    config.Description,
		TermsOfService: config.TermsOfService,
		Contact: &OpenAPIContact{
			Name:  contactName,
			Email: config.ContactEmail,
			URL:   config.ContactURL,
		},
	}

	if config.LicenseName != "" {
		info.License = &OpenAPILicense{
			Name: config.LicenseName,
			URL:  config.LicenseURL,
		}
	}

	var externalDocs *OpenAPIExternalDocs
	if config.ExternalDocsURL != "" {
		externalDocs = &OpenAPIExternalDocs{
			URL:         config.ExternalDocsURL,
			Description: config.ExternalDocsDesc,
		}
	}

	return info, externalDocs
}

// NewAPIRegistry creates a new API documentation registry with dependency injection
func NewAPIRegistry(config *APIDocsConfig, astParser ASTParserInterface, schemaGenerator SchemaGeneratorInterface) *APIRegistry {
	if config == nil {
		config = DefaultAPIDocsConfig()
	}

	info, externalDocs := buildOpenAPIInfo(config)

	registry := &APIRegistry{
		config: config,
		docs: &APIDocs{
			OpenAPI:      "3.0.3",
			Info:         info,
			ExternalDocs: externalDocs,
			Servers: []*OpenAPIServer{
				{
					URL:         config.BaseURL,
					Description: "API Server",
				},
			},
			Paths:     make(map[string]*OpenAPIPathItem),
			endpoints: []APIEndpoint{},
			generated: time.Now().Format(time.RFC3339),
			Components: &OpenAPIComponents{
				Schemas:         make(map[string]*OpenAPISchema),
				Responses:       make(map[string]*OpenAPIResponse),
				Parameters:      make(map[string]*OpenAPIParameter),
				RequestBodies:   make(map[string]*OpenAPIRequestBody),
				Examples:        make(map[string]*OpenAPIExample),
				Headers:         make(map[string]*OpenAPIHeader),
				SecuritySchemes: make(map[string]*OpenAPISecurityScheme),
				Links:           make(map[string]*OpenAPILink),
				Callbacks:       make(map[string]*OpenAPICallback),
			},
		},
		endpoints:       make(map[string]APIEndpoint),
		astParser:       astParser,
		schemaGenerator: schemaGenerator,
	}

	return registry
}

// SetVersion sets the API version associated with this registry.
func (r *APIRegistry) SetVersion(version string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.version = version
}

// GetVersion returns the API version associated with this registry.
func (r *APIRegistry) GetVersion() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.version
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

// GetDocs returns the current API documentation
func (r *APIRegistry) GetDocs() *APIDocs {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy to avoid race conditions
	docsCopy := &APIDocs{
		OpenAPI:    r.docs.OpenAPI,
		Info:       r.docs.Info,
		Servers:    r.docs.Servers,
		Paths:      r.docs.Paths,
		Components: r.docs.Components,
		Tags:       r.docs.Tags,
		endpoints:  make([]APIEndpoint, len(r.docs.endpoints)),
		generated:  r.docs.generated,
	}

	copy(docsCopy.endpoints, r.docs.endpoints)

	return docsCopy
}

// GetEndpointsInternal returns the internal endpoints list (for backwards compatibility)
func (r *APIRegistry) GetEndpointsInternal() []APIEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.docs.endpoints
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
	info, externalDocs := buildOpenAPIInfo(config)
	r.docs.Info = info
	r.docs.ExternalDocs = externalDocs
	r.docs.Servers = []*OpenAPIServer{
		{
			URL:         config.BaseURL,
			Description: "API Server",
		},
	}
}

// SetServers sets the OpenAPI servers and derives a path prefix to strip from registered paths.
// When a server URL ends with a path (e.g., http://host/api/v1), registered paths like
// /api/v1/todos will be stored as /todos in the spec, since the server URL already includes the prefix.
func (r *APIRegistry) SetServers(servers []*OpenAPIServer) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.docs.Servers = servers

	// Derive path prefix from the first server's URL path component
	if len(servers) > 0 {
		serverURL := servers[0].URL
		// Extract path portion after the host (find 3rd slash: http://host/path)
		slashCount := 0
		for i, c := range serverURL {
			if c == '/' {
				slashCount++
				if slashCount == 3 {
					r.pathPrefix = serverURL[i:]
					return
				}
			}
		}
	}
	r.pathPrefix = ""
}

// ClearEndpoints removes all registered endpoints
func (r *APIRegistry) ClearEndpoints() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.endpoints = make(map[string]APIEndpoint)
	r.docs.endpoints = []APIEndpoint{}
	r.docs.Paths = make(map[string]*OpenAPIPathItem)
	r.specDirty = true
	r.cachedSpecDocs = nil
}

// GetRegisteredEndpoints returns all currently registered endpoints
func (r *APIRegistry) GetRegisteredEndpoints() []APIEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endpoints := make([]APIEndpoint, 0, len(r.endpoints))
	for _, endpoint := range r.endpoints {
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

// GetEndpointCount returns the number of registered endpoints
func (r *APIRegistry) GetEndpointCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.endpoints)
}

// =============================================================================
// Private Helpers
// =============================================================================

// endpointKey generates a unique key for an endpoint
func (r *APIRegistry) endpointKey(method, path string) string {
	return fmt.Sprintf("%s:%s", strings.ToUpper(method), path)
}

// rebuildEndpointsList rebuilds the endpoints slice and paths from the map (must be called with lock held)
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

	r.docs.endpoints = endpoints
	r.docs.generated = time.Now().Format(time.RFC3339)

	// Build OpenAPI paths from endpoints
	r.docs.Paths = r.buildPaths(endpoints)

	// Collect unique tags
	r.docs.Tags = r.buildTags(endpoints)

	// Invalidate the component-schema cache
	r.specDirty = true
	r.cachedSpecDocs = nil
}
