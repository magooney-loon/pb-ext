package api

import (
	"fmt"
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
	pathPrefix      string // prefix to strip from paths (derived from server URL)
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

// RegisterRoute explicitly registers a route with optional middleware
func (r *APIRegistry) RegisterRoute(method, path string, handler func(*core.RequestEvent) error, middlewares ...interface{}) {
	if !r.config.Enabled {
		return
	}

	// Create registry helper for analysis
	helper := NewRegistryHelper()
	analysis := helper.AnalyzeRoute(method, path, handler, middlewares)

	// Create endpoint from analysis
	endpoint := r.createEndpointFromAnalysis(analysis)
	r.enhanceEndpointWithAST(&endpoint)
	r.RegisterEndpoint(endpoint)
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

// GetDocsWithComponents returns documentation with generated component schemas
// Only includes component schemas that are actually referenced by this version's endpoints
func (r *APIRegistry) GetDocsWithComponents() *APIDocs {

	docs := r.GetDocs()

	if r.schemaGenerator != nil {
		allComponents := r.schemaGenerator.GenerateComponentSchemas()

		// Collect all $ref targets used by this version's paths
		refs := make(map[string]bool)
		for _, pathItem := range docs.Paths {
			collectRefsFromPathItem(pathItem, refs)
		}
		// Also collect refs from common responses/parameters in components
		for _, resp := range allComponents.Responses {
			collectRefsFromResponse(resp, refs)
		}

		// Recursively resolve nested $refs from the collected schemas
		resolved := make(map[string]bool)
		pending := make([]string, 0, len(refs))
		for name := range refs {
			pending = append(pending, name)
		}
		for len(pending) > 0 {
			name := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if resolved[name] {
				continue
			}
			resolved[name] = true
			if schema, ok := allComponents.Schemas[name]; ok {
				nested := make(map[string]bool)
				collectRefsFromSchema(schema, nested)
				for n := range nested {
					if !resolved[n] {
						pending = append(pending, n)
					}
				}
			}
		}

		// Prune schemas to only those referenced by this version's endpoints
		pruned := make(map[string]*OpenAPISchema)
		for name, schema := range allComponents.Schemas {
			if resolved[name] {
				pruned[name] = schema
			}
		}
		// Always keep Error and PocketBaseRecord as they're used by common responses
		if s, ok := allComponents.Schemas["Error"]; ok {
			pruned["Error"] = s
		}
		if s, ok := allComponents.Schemas["PocketBaseRecord"]; ok {
			pruned["PocketBaseRecord"] = s
		}
		allComponents.Schemas = pruned

		docs.Components = allComponents
	}

	return docs
}

// collectRefsFromPathItem collects all $ref schema names from a path item's operations
func collectRefsFromPathItem(pathItem *OpenAPIPathItem, refs map[string]bool) {
	for _, op := range []*OpenAPIOperation{
		pathItem.Get, pathItem.Put, pathItem.Post, pathItem.Delete,
		pathItem.Patch, pathItem.Options, pathItem.Head, pathItem.Trace,
	} {
		if op == nil {
			continue
		}
		if op.RequestBody != nil {
			for _, mt := range op.RequestBody.Content {
				if mt.Schema != nil {
					collectRefsFromSchema(mt.Schema, refs)
				}
			}
		}
		for _, resp := range op.Responses {
			collectRefsFromResponse(resp, refs)
		}
		for _, param := range op.Parameters {
			if param.Schema != nil {
				collectRefsFromSchema(param.Schema, refs)
			}
		}
	}
}

// collectRefsFromResponse collects $ref schema names from a response object
func collectRefsFromResponse(resp *OpenAPIResponse, refs map[string]bool) {
	if resp == nil {
		return
	}
	for _, mt := range resp.Content {
		if mt.Schema != nil {
			collectRefsFromSchema(mt.Schema, refs)
		}
	}
}

// collectRefsFromSchema recursively collects $ref schema names from a schema
func collectRefsFromSchema(schema *OpenAPISchema, refs map[string]bool) {
	if schema == nil {
		return
	}
	if schema.Ref != "" {
		name := schemaNameFromRef(schema.Ref)
		if name != "" {
			refs[name] = true
		}
	}
	for _, prop := range schema.Properties {
		collectRefsFromSchema(prop, refs)
	}
	if schema.Items != nil {
		collectRefsFromSchema(schema.Items, refs)
	}
	if addl, ok := schema.AdditionalProperties.(*OpenAPISchema); ok {
		collectRefsFromSchema(addl, refs)
	}
	for _, s := range schema.AllOf {
		collectRefsFromSchema(s, refs)
	}
	for _, s := range schema.OneOf {
		collectRefsFromSchema(s, refs)
	}
	for _, s := range schema.AnyOf {
		collectRefsFromSchema(s, refs)
	}
	if schema.Not != nil {
		collectRefsFromSchema(schema.Not, refs)
	}
}

// schemaNameFromRef extracts the schema name from a $ref string like "#/components/schemas/Foo"
func schemaNameFromRef(ref string) string {
	const prefix = "#/components/schemas/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):]
	}
	return ""
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
}

// =============================================================================
// Private Helper Methods
// =============================================================================

// createEndpointFromAnalysis creates an APIEndpoint from route analysis
func (r *APIRegistry) createEndpointFromAnalysis(analysis *RouteAnalysis) APIEndpoint {
	endpoint := APIEndpoint{
		Method:      analysis.Method,
		Path:        analysis.Path,
		Description: analysis.Description,
		Tags:        analysis.Tags,
		Handler:     analysis.Handler.FullName,
		Auth:        analysis.Auth,
	}

	return endpoint
}

// enhanceEndpointWithAST enhances an endpoint with AST-extracted schema information
func (r *APIRegistry) enhanceEndpointWithAST(endpoint *APIEndpoint) {
	// Enhance with AST analysis first - this includes API_DESC and API_TAGS from comments
	if r.astParser != nil {
		// Try multiple handler name variations for better matching
		handlerNames := []string{
			endpoint.Handler, // Full name
			ExtractHandlerBaseName(endpoint.Handler, false), // Without package, keep suffixes
			ExtractHandlerBaseName(endpoint.Handler, true),  // Without package and suffixes
		}

		enhanced := false
		for _, handlerName := range handlerNames {
			if handlerInfo, exists := r.astParser.GetHandlerByName(handlerName); exists {
				// AST data takes priority - override generated descriptions/tags
				if handlerInfo.APIDescription != "" {
					endpoint.Description = handlerInfo.APIDescription
				}
				if len(handlerInfo.APITags) > 0 {
					endpoint.Tags = handlerInfo.APITags
				}

				// Set authentication info from AST
				if handlerInfo.RequiresAuth {
					endpoint.Auth = &AuthInfo{
						Required:    true,
						Type:        handlerInfo.AuthType,
						Description: r.getASTAuthDescription(handlerInfo.AuthType),
					}
				}

				// Set schemas from AST
				if handlerInfo.RequestSchema != nil {
					endpoint.Request = handlerInfo.RequestSchema
				}
				if handlerInfo.ResponseSchema != nil {
					endpoint.Response = handlerInfo.ResponseSchema
				}

				// Set AST-detected parameters (query, header, path from code analysis)
				if len(handlerInfo.Parameters) > 0 {
					endpoint.Parameters = handlerInfo.Parameters
				}

				enhanced = true
				break
			}
		}

		// Fallback to generic AST enhancement if specific handler not found
		if !enhanced {
			if err := r.astParser.EnhanceEndpoint(endpoint); err != nil {
				// Log error but don't fail - AST enhancement is optional
			}
		}
	}

	// Generate additional schemas if schema generator is available and AST didn't provide them
	if r.schemaGenerator != nil {
		// Only generate request schema if AST didn't provide one
		if endpoint.Request == nil {
			if requestSchema, err := r.schemaGenerator.AnalyzeRequestSchema(endpoint); err == nil {
				endpoint.Request = requestSchema
			}
		}

		// Only generate response schema if AST didn't provide one
		if endpoint.Response == nil {
			if responseSchema, err := r.schemaGenerator.AnalyzeResponseSchema(endpoint); err == nil {
				endpoint.Response = responseSchema
			}
		}
	}
}

// RegisterExplicitRoute registers a route with explicit information (no inference)
func (r *APIRegistry) RegisterExplicitRoute(endpoint APIEndpoint) {
	if !r.config.Enabled {
		return
	}

	// Only enhance with AST for schema extraction
	r.enhanceEndpointWithAST(&endpoint)
	r.RegisterEndpoint(endpoint)
}

// BatchRegisterRoutes registers multiple routes at once
func (r *APIRegistry) BatchRegisterRoutes(routes []RouteDefinition) {
	if !r.config.Enabled {
		return
	}

	for _, route := range routes {
		r.RegisterRoute(route.Method, route.Path, route.Handler, route.Middlewares...)
	}
}

// RouteDefinition represents an explicit route definition
type RouteDefinition struct {
	Method      string
	Path        string
	Handler     func(*core.RequestEvent) error
	Middlewares []interface{}
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

// endpointKey generates a unique key for an endpoint
func (r *APIRegistry) endpointKey(method, path string) string {
	return fmt.Sprintf("%s:%s", strings.ToUpper(method), path)
}

// rebuildEndpointsList rebuilds the endpoints slice and paths from the map (should be called with lock held)
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
}

// buildPaths converts internal endpoints to OpenAPI paths format
func (r *APIRegistry) buildPaths(endpoints []APIEndpoint) map[string]*OpenAPIPathItem {
	paths := make(map[string]*OpenAPIPathItem)

	for _, endpoint := range endpoints {
		// Strip path prefix so paths are relative to the server URL
		docPath := endpoint.Path
		if r.pathPrefix != "" && strings.HasPrefix(docPath, r.pathPrefix) {
			docPath = strings.TrimPrefix(docPath, r.pathPrefix)
			if docPath == "" {
				docPath = "/"
			}
		}

		// Get or create path item
		pathItem, exists := paths[docPath]
		if !exists {
			pathItem = &OpenAPIPathItem{}
			paths[docPath] = pathItem
		}

		// Create operation
		operation := r.endpointToOperation(endpoint)

		// Assign to correct HTTP method
		switch strings.ToUpper(endpoint.Method) {
		case "GET":
			pathItem.Get = operation
		case "POST":
			pathItem.Post = operation
		case "PUT":
			pathItem.Put = operation
		case "DELETE":
			pathItem.Delete = operation
		case "PATCH":
			pathItem.Patch = operation
		case "OPTIONS":
			pathItem.Options = operation
		case "HEAD":
			pathItem.Head = operation
		}
	}

	return paths
}

// endpointToOperation converts an APIEndpoint to an OpenAPIOperation
func (r *APIRegistry) endpointToOperation(endpoint APIEndpoint) *OpenAPIOperation {
	operation := &OpenAPIOperation{
		Summary:     endpoint.Description,
		Description: endpoint.Description,
		Tags:        endpoint.Tags,
		OperationId: r.generateOperationId(endpoint),
		Responses:   make(map[string]*OpenAPIResponse),
	}

	// Extract path parameters from URL pattern
	operation.Parameters = r.extractPathParameters(endpoint.Path)

	// Append AST-detected parameters (query, header, additional path params)
	if len(endpoint.Parameters) > 0 {
		existingNames := make(map[string]bool)
		for _, p := range operation.Parameters {
			existingNames[p.In+":"+p.Name] = true
		}
		for _, paramInfo := range endpoint.Parameters {
			key := paramInfo.Source + ":" + paramInfo.Name
			if !existingNames[key] {
				operation.Parameters = append(operation.Parameters, ConvertParamInfoToOpenAPIParameter(paramInfo))
				existingNames[key] = true
			}
		}
	}

	// Add request body if present
	if endpoint.Request != nil {
		operation.RequestBody = &OpenAPIRequestBody{
			Description: "Request body",
			Required:    boolPtr(true),
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: endpoint.Request,
				},
			},
		}
	}

	// Add response — use $ref for promotable response schemas
	if endpoint.Response != nil {
		responseSchema := endpoint.Response
		if responseSchema.Ref == "" && isPromotableSchema(responseSchema) && endpoint.Handler != "" {
			if name := handlerResponseSchemaName(endpoint.Handler); name != "" {
				responseSchema = &OpenAPISchema{
					Ref: "#/components/schemas/" + name,
				}
			}
		}
		operation.Responses["200"] = &OpenAPIResponse{
			Description: "Successful response",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: responseSchema,
				},
			},
		}
	} else {
		operation.Responses["200"] = &OpenAPIResponse{
			Description: "Successful response",
		}
	}

	// Add security requirement if auth is required
	if endpoint.Auth != nil && endpoint.Auth.Required {
		operation.Security = []map[string][]string{
			{"bearerAuth": {}},
		}
	}

	return operation
}

// extractPathParameters extracts path parameters from a path like /api/v1/todos/{id}
func (r *APIRegistry) extractPathParameters(path string) []*OpenAPIParameter {
	var params []*OpenAPIParameter

	// Find all {param} patterns
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			paramName := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			params = append(params, &OpenAPIParameter{
				Name:        paramName,
				In:          "path",
				Required:    boolPtr(true),
				Description: fmt.Sprintf("The %s parameter", paramName),
				Schema: &OpenAPISchema{
					Type: "string",
				},
			})
		}
	}

	return params
}

// generateOperationId generates a unique operation ID for an endpoint
func (r *APIRegistry) generateOperationId(endpoint APIEndpoint) string {
	// Convert path to camelCase operation ID
	// e.g., GET /api/v1/todos/{id} -> getTodosById
	path := strings.ReplaceAll(endpoint.Path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	parts := strings.Split(path, "/")

	var result strings.Builder
	result.WriteString(strings.ToLower(endpoint.Method))

	for _, part := range parts {
		if part != "" && part != "api" {
			// Capitalize first letter
			if len(part) > 0 {
				result.WriteString(strings.ToUpper(part[:1]))
				if len(part) > 1 {
					result.WriteString(part[1:])
				}
			}
		}
	}

	return result.String()
}

// buildTags collects unique tags from endpoints
func (r *APIRegistry) buildTags(endpoints []APIEndpoint) []*OpenAPITag {
	tagMap := make(map[string]bool)
	for _, endpoint := range endpoints {
		for _, tag := range endpoint.Tags {
			tagMap[tag] = true
		}
	}

	var tags []*OpenAPITag
	for tag := range tagMap {
		tags = append(tags, &OpenAPITag{
			Name: tag,
		})
	}

	// Sort tags alphabetically
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}

// getASTAuthDescription returns user-friendly auth description for AST-detected auth
func (r *APIRegistry) getASTAuthDescription(authType string) string {
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
