package api

import (
	"fmt"
	"sort"
	"strings"
)

// =============================================================================
// OpenAPI Spec Assembly
// =============================================================================

// GetDocsWithComponents returns documentation with generated component schemas.
// Only includes component schemas that are actually referenced by this version's endpoints.
// The assembled spec is cached and only regenerated when endpoints change.
func (r *APIRegistry) GetDocsWithComponents() *APIDocs {
	r.mu.RLock()
	if !r.specDirty && r.cachedSpecDocs != nil {
		cached := r.cachedSpecDocs
		r.mu.RUnlock()
		return cached
	}
	r.mu.RUnlock()

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

	// Store in cache
	r.mu.Lock()
	r.cachedSpecDocs = docs
	r.specDirty = false
	r.mu.Unlock()

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
	path := strings.ReplaceAll(endpoint.Path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	parts := strings.Split(path, "/")

	var result strings.Builder
	result.WriteString(strings.ToLower(endpoint.Method))

	for _, part := range parts {
		if part != "" && part != "api" {
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
