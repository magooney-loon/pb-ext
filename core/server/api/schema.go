package api

import (
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// Schema Generator Implementation
// =============================================================================

// NewSchemaGenerator creates a new schema generator with the given AST parser
func NewSchemaGenerator(astParser ASTParserInterface) *SchemaGenerator {
	return &SchemaGenerator{
		astParser:   astParser,
		typeCache:   make(map[string]interface{}),
		schemaCache: make(map[string]map[string]interface{}),
		validators:  []TypeValidator{},
		logger:      &DefaultLogger{},
	}
}

// AnalyzeRequestSchema analyzes and generates request schema for an endpoint
func (sg *SchemaGenerator) AnalyzeRequestSchema(endpoint *APIEndpoint) (map[string]interface{}, error) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	// Check cache first
	cacheKey := fmt.Sprintf("req:%s:%s", endpoint.Method, endpoint.Path)
	if cached, exists := sg.schemaCache[cacheKey]; exists {
		return cached, nil
	}

	var schema map[string]interface{}

	// Try AST-based analysis first
	// Only use AST-based generation - no fallbacks
	if sg.astParser != nil {
		schema = sg.generateRequestSchemaFromAST(endpoint)
	}

	// Cache the result
	if schema != nil {
		sg.schemaCache[cacheKey] = schema
	}

	return schema, nil
}

// AnalyzeResponseSchema analyzes and generates response schema for an endpoint
func (sg *SchemaGenerator) AnalyzeResponseSchema(endpoint *APIEndpoint) (map[string]interface{}, error) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	// Check cache first
	cacheKey := fmt.Sprintf("resp:%s:%s", endpoint.Method, endpoint.Path)
	if cached, exists := sg.schemaCache[cacheKey]; exists {
		return cached, nil
	}

	var schema map[string]interface{}

	// Try AST-based analysis first
	// Only use AST-based generation - no fallbacks
	if sg.astParser != nil {
		schema = sg.generateResponseSchemaFromAST(endpoint)
	}

	// Cache the result
	if schema != nil {
		sg.schemaCache[cacheKey] = schema
	}

	return schema, nil
}

// AnalyzeSchemaFromPath generates schemas based on URL path patterns
func (sg *SchemaGenerator) AnalyzeSchemaFromPath(method, path string) (*SchemaAnalysisResult, error) {
	result := &SchemaAnalysisResult{
		Errors:   []error{},
		Warnings: []string{},
	}

	// Only provide schemas if we have AST data - no generic fallbacks

	return result, nil
}

// GenerateComponentSchemas generates OpenAPI component schemas
func (sg *SchemaGenerator) GenerateComponentSchemas() map[string]interface{} {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	components := map[string]interface{}{
		"schemas": sg.generateStructSchemas(),
	}

	return components
}

// =============================================================================
// AST-Based Schema Generation
// =============================================================================

// generateRequestSchemaFromAST generates request schema using AST information
func (sg *SchemaGenerator) generateRequestSchemaFromAST(endpoint *APIEndpoint) map[string]interface{} {
	if sg.astParser == nil {
		return nil
	}

	handlerName := ExtractHandlerNameFromPath(endpoint.Handler)
	if handlerInfo, exists := sg.astParser.GetHandlerByName(handlerName); exists {
		// First check if we have a pre-generated request schema from AST analysis
		if handlerInfo.RequestSchema != nil {
			return handlerInfo.RequestSchema
		}

		// Fallback to struct-based schema generation
		if handlerInfo.RequestType != "" {
			if structInfo, exists := sg.astParser.GetStructByName(handlerInfo.RequestType); exists {
				return structInfo.JSONSchema
			}
		}
	}

	return nil
}

// generateResponseSchemaFromAST generates response schema using AST information
func (sg *SchemaGenerator) generateResponseSchemaFromAST(endpoint *APIEndpoint) map[string]interface{} {
	if sg.astParser == nil {
		return nil
	}

	handlerName := ExtractHandlerNameFromPath(endpoint.Handler)
	if handlerInfo, exists := sg.astParser.GetHandlerByName(handlerName); exists {
		// First check for inline map schema (from map literals)
		if handlerInfo.ResponseSchema != nil {
			return handlerInfo.ResponseSchema
		}

		// Fallback to struct-based schema
		if handlerInfo.ResponseType != "" {
			if structInfo, exists := sg.astParser.GetStructByName(handlerInfo.ResponseType); exists {
				return structInfo.JSONSchema
			}
		}
	}

	return nil
}

// generateStructSchemas generates schemas for all parsed structs
func (sg *SchemaGenerator) generateStructSchemas() map[string]interface{} {
	schemas := make(map[string]interface{})

	if sg.astParser != nil {
		structs := sg.astParser.GetAllStructs()
		for name, structInfo := range structs {
			if structInfo.JSONSchema != nil {
				cleanName := CleanTypeName(name)
				schemas[cleanName] = structInfo.JSONSchema
			}
		}
	}

	// Only return actual parsed schemas - no generic ones
	return schemas
}

// =============================================================================
// Schema Pattern Matching
// =============================================================================

// getSchemaPatterns returns predefined schema patterns for common endpoints
func (sg *SchemaGenerator) getSchemaPatterns() []*SchemaPattern {
	return []*SchemaPattern{
		{
			Name: "Collection List",
			PathMatch: func(path string) bool {
				// Matches paths like /api/collections/:collection/records
				return regexp.MustCompile(`/collections/[^/]+/records$`).MatchString(path)
			},
			RequestGen: func() map[string]interface{} {
				return nil // GET requests don't have body
			},
			ResponseGen: func() map[string]interface{} {
				return sg.generatePaginatedResponseSchema()
			},
		},
		{
			Name: "Collection Create",
			PathMatch: func(path string) bool {
				return regexp.MustCompile(`/collections/[^/]+/records$`).MatchString(path)
			},
			RequestGen: func() map[string]interface{} {
				return sg.generateRecordCreateSchema()
			},
			ResponseGen: func() map[string]interface{} {
				return sg.generateSingleRecordSchema()
			},
		},
		{
			Name: "Collection Update",
			PathMatch: func(path string) bool {
				return regexp.MustCompile(`/collections/[^/]+/records/[^/]+$`).MatchString(path)
			},
			RequestGen: func() map[string]interface{} {
				return sg.generateRecordUpdateSchema()
			},
			ResponseGen: func() map[string]interface{} {
				return sg.generateSingleRecordSchema()
			},
		},
		{
			Name: "Auth Login",
			PathMatch: func(path string) bool {
				return strings.Contains(path, "auth") && strings.Contains(path, "login")
			},
			RequestGen: func() map[string]interface{} {
				return sg.generateLoginRequestSchema()
			},
			ResponseGen: func() map[string]interface{} {
				return sg.generateAuthResponseSchema()
			},
		},
		{
			Name: "Auth Register",
			PathMatch: func(path string) bool {
				return strings.Contains(path, "auth") && strings.Contains(path, "register")
			},
			RequestGen: func() map[string]interface{} {
				return sg.generateRegisterRequestSchema()
			},
			ResponseGen: func() map[string]interface{} {
				return sg.generateAuthResponseSchema()
			},
		},
	}
}

// =============================================================================
// List Endpoint Detection
// =============================================================================

// isListEndpoint determines if a path represents a list endpoint
func (sg *SchemaGenerator) isListEndpoint(path string) bool {
	// Simple heuristic: if the path doesn't end with an ID parameter, it's likely a list
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return false
	}

	lastPart := parts[len(parts)-1]
	// If the last part looks like a parameter (:id, {id}), it's not a list
	if strings.HasPrefix(lastPart, ":") ||
		(strings.HasPrefix(lastPart, "{") && strings.HasSuffix(lastPart, "}")) {
		return false
	}

	// Check if the path suggests a collection
	return strings.Contains(path, "list") ||
		strings.HasSuffix(path, "s") ||
		strings.Contains(path, "records")
}

// =============================================================================
// Specific Schema Generators
// =============================================================================

// generateErrorResponseSchema generates schema for error responses
func (sg *SchemaGenerator) generateErrorResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"code": map[string]interface{}{
				"type":        "integer",
				"description": "Error code",
				"example":     400,
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Error message",
				"example":     "Bad Request",
			},
			"data": map[string]interface{}{
				"type":                 "object",
				"description":          "Additional error data",
				"additionalProperties": true,
			},
		},
		"required": []string{"code", "message"},
	}
}

// generateSuccessResponseSchema generates schema for success responses
func (sg *SchemaGenerator) generateSuccessResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{
				"type":        "boolean",
				"description": "Operation success status",
				"example":     true,
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Success message",
				"example":     "Operation completed successfully",
			},
		},
		"required": []string{"success"},
	}
}

// generatePaginatedResponseSchema generates schema for paginated responses
func (sg *SchemaGenerator) generatePaginatedResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"page": map[string]interface{}{
				"type":        "integer",
				"description": "Current page number",
				"minimum":     1,
				"example":     1,
			},
			"perPage": map[string]interface{}{
				"type":        "integer",
				"description": "Items per page",
				"minimum":     1,
				"maximum":     500,
				"example":     30,
			},
			"totalItems": map[string]interface{}{
				"type":        "integer",
				"description": "Total number of items",
				"minimum":     0,
				"example":     150,
			},
			"totalPages": map[string]interface{}{
				"type":        "integer",
				"description": "Total number of pages",
				"minimum":     0,
				"example":     5,
			},
			"items": map[string]interface{}{
				"type":        "array",
				"description": "Array of items for current page",
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": true,
				},
			},
		},
		"required": []string{"page", "perPage", "totalItems", "totalPages", "items"},
	}
}

// generateSingleRecordSchema generates schema for single record responses
func (sg *SchemaGenerator) generateSingleRecordSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Record identifier",
				"example":     "abc123def456",
			},
			"created": map[string]interface{}{
				"type":        "string",
				"format":      "date-time",
				"description": "Creation timestamp",
				"example":     "2023-01-01T12:00:00Z",
			},
			"updated": map[string]interface{}{
				"type":        "string",
				"format":      "date-time",
				"description": "Last update timestamp",
				"example":     "2023-01-01T12:00:00Z",
			},
		},
		"required":             []string{"id", "created", "updated"},
		"additionalProperties": true,
	}
}

// generateRecordCreateSchema generates schema for record creation requests
func (sg *SchemaGenerator) generateRecordCreateSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"description":          "Data for creating a new record",
		"additionalProperties": true,
		"example": map[string]interface{}{
			"name":  "Example Record",
			"email": "example@test.com",
		},
	}
}

// generateRecordUpdateSchema generates schema for record update requests
func (sg *SchemaGenerator) generateRecordUpdateSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"description":          "Data for updating an existing record",
		"additionalProperties": true,
		"example": map[string]interface{}{
			"name": "Updated Record Name",
		},
	}
}

// generateLoginRequestSchema generates schema for login requests
func (sg *SchemaGenerator) generateLoginRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"identity": map[string]interface{}{
				"type":        "string",
				"description": "User email or username",
				"example":     "user@example.com",
			},
			"password": map[string]interface{}{
				"type":        "string",
				"description": "User password",
				"format":      "password",
				"example":     "password123",
			},
		},
		"required": []string{"identity", "password"},
	}
}

// generateRegisterRequestSchema generates schema for registration requests
func (sg *SchemaGenerator) generateRegisterRequestSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"email": map[string]interface{}{
				"type":        "string",
				"format":      "email",
				"description": "User email address",
				"example":     "newuser@example.com",
			},
			"password": map[string]interface{}{
				"type":        "string",
				"description": "User password",
				"format":      "password",
				"minLength":   8,
				"example":     "password123",
			},
			"passwordConfirm": map[string]interface{}{
				"type":        "string",
				"description": "Password confirmation",
				"format":      "password",
				"example":     "password123",
			},
		},
		"required": []string{"email", "password", "passwordConfirm"},
	}
}

// generateAuthResponseSchema generates schema for authentication responses
func (sg *SchemaGenerator) generateAuthResponseSchema() map[string]interface{} {
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
				"description": "User record information",
				"$ref":        "#/components/schemas/UserRecord",
			},
		},
		"required": []string{"token", "record"},
	}
}

// generateDeleteResponseSchema generates schema for delete responses
func (sg *SchemaGenerator) generateDeleteResponseSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"success": map[string]interface{}{
				"type":        "boolean",
				"description": "Deletion success status",
				"example":     true,
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "Deletion confirmation message",
				"example":     "Record deleted successfully",
			},
		},
		"required": []string{"success"},
	}
}

// =============================================================================
// Cache Management
// =============================================================================

// =============================================================================
// Cache Management
// =============================================================================

// ClearCache clears the schema cache
func (sg *SchemaGenerator) ClearCache() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	sg.schemaCache = make(map[string]map[string]interface{})
	sg.typeCache = make(map[string]interface{})
}

// GetCacheStats returns cache statistics
func (sg *SchemaGenerator) GetCacheStats() map[string]interface{} {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	return map[string]interface{}{
		"schema_cache_size": len(sg.schemaCache),
		"type_cache_size":   len(sg.typeCache),
	}
}
