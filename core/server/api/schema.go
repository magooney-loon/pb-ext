package api

import (
	"fmt"
	"regexp"
)

// =============================================================================
// Schema Generator Implementation
// =============================================================================

// NewSchemaGenerator creates a new schema generator with the given AST parser
func NewSchemaGenerator(astParser ASTParserInterface) *SchemaGenerator {
	logger := &DefaultLogger{}
	return &SchemaGenerator{
		astParser:     astParser,
		schemaBuilder: NewOpenAPISchemaBuilder(logger),
		components: &OpenAPIComponents{
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
		validators: []TypeValidator{},
		logger:     logger,
	}
}

// AnalyzeRequestSchema analyzes and generates request schema for an endpoint
func (sg *SchemaGenerator) AnalyzeRequestSchema(endpoint *APIEndpoint) (*OpenAPISchema, error) {
	if endpoint == nil {
		return nil, fmt.Errorf("endpoint cannot be nil")
	}

	sg.mu.RLock()
	defer sg.mu.RUnlock()

	// Check if endpoint already has a request schema
	if endpoint.Request != nil {
		return endpoint.Request, nil
	}

	// Try to generate from AST
	if schema := sg.generateRequestSchemaFromAST(endpoint); schema != nil {
		return schema, nil
	}

	// Try pattern matching
	patterns := sg.getSchemaPatterns()
	for _, pattern := range patterns {
		if pattern.PathMatch != nil && pattern.PathMatch(endpoint.Path) {
			if pattern.RequestGen != nil {
				return pattern.RequestGen(), nil
			}
		}
	}

	// Generate basic schema for methods that typically have request bodies
	if endpoint.Method == "POST" || endpoint.Method == "PUT" || endpoint.Method == "PATCH" {
		return &OpenAPISchema{
			Type: "object",
			Properties: map[string]*OpenAPISchema{
				"data": {
					Type:        "object",
					Description: "Request data",
				},
			},
		}, nil
	}

	return nil, nil // No request schema needed (e.g., GET requests)
}

// AnalyzeResponseSchema analyzes and generates response schema for an endpoint
func (sg *SchemaGenerator) AnalyzeResponseSchema(endpoint *APIEndpoint) (*OpenAPISchema, error) {
	if endpoint == nil {
		return nil, fmt.Errorf("endpoint cannot be nil")
	}

	sg.mu.RLock()
	defer sg.mu.RUnlock()

	// Check if endpoint already has a response schema
	if endpoint.Response != nil {
		return endpoint.Response, nil
	}

	// Try to generate from AST
	if schema := sg.generateResponseSchemaFromAST(endpoint); schema != nil {
		return schema, nil
	}

	// Try pattern matching
	patterns := sg.getSchemaPatterns()
	for _, pattern := range patterns {
		if pattern.PathMatch != nil && pattern.PathMatch(endpoint.Path) {
			if pattern.ResponseGen != nil {
				return pattern.ResponseGen(), nil
			}
		}
	}

	// Generate basic response schema for all methods
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"data": {
				Type:        "object",
				Description: "Response data",
			},
			"message": {
				Type:        "string",
				Description: "Response message",
			},
		},
	}, nil
}

// AnalyzeSchemaFromPath analyzes schemas for a given method and path
func (sg *SchemaGenerator) AnalyzeSchemaFromPath(method, path string) (*SchemaAnalysisResult, error) {
	result := &SchemaAnalysisResult{
		Errors:   []error{},
		Warnings: []string{},
	}

	// Create a temporary endpoint for analysis
	endpoint := &APIEndpoint{
		Method: method,
		Path:   path,
	}

	// Analyze request schema
	if requestSchema, err := sg.AnalyzeRequestSchema(endpoint); err != nil {
		result.Errors = append(result.Errors, err)
	} else {
		result.RequestSchema = requestSchema
	}

	// Analyze response schema
	if responseSchema, err := sg.AnalyzeResponseSchema(endpoint); err != nil {
		result.Errors = append(result.Errors, err)
	} else {
		result.ResponseSchema = responseSchema
	}

	return result, nil
}

// GenerateComponentSchemas generates OpenAPI component schemas from AST data
func (sg *SchemaGenerator) GenerateComponentSchemas() *OpenAPIComponents {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	components := &OpenAPIComponents{
		Schemas:       sg.generateStructSchemas(),
		Responses:     sg.generateCommonResponses(),
		Parameters:    sg.generateCommonParameters(),
		RequestBodies: make(map[string]*OpenAPIRequestBody),
		Examples:      make(map[string]*OpenAPIExample),
		Headers:       make(map[string]*OpenAPIHeader),
		SecuritySchemes: map[string]*OpenAPISecurityScheme{
			"bearerAuth": {
				Type:   "http",
				Scheme: "bearer",
			},
		},
		Links:     make(map[string]*OpenAPILink),
		Callbacks: make(map[string]*OpenAPICallback),
	}

	return components
}

// GetOpenAPIEndpointSchema generates a complete OpenAPI endpoint schema
func (sg *SchemaGenerator) GetOpenAPIEndpointSchema(endpoint *APIEndpoint) (*OpenAPIEndpointSchema, error) {
	operation := &OpenAPIOperation{
		Summary:     endpoint.Description,
		Description: endpoint.Description,
		Tags:        endpoint.Tags,
		Responses:   make(map[string]*OpenAPIResponse),
	}

	// Generate operation ID
	if endpoint.Handler != "" {
		operation.OperationId = generateOperationId(endpoint.Handler)
	}

	// Extract path parameters from URL pattern
	pathParams := extractPathParameters(endpoint.Path)
	if len(pathParams) > 0 {
		operation.Parameters = make([]*OpenAPIParameter, len(pathParams))
		for i, param := range pathParams {
			operation.Parameters[i] = &OpenAPIParameter{
				Name:        param,
				In:          "path",
				Required:    boolPtr(true),
				Description: "Path parameter",
				Schema: &OpenAPISchema{
					Type: "string",
				},
			}
		}
	}

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

	// Analyze request schema
	if requestSchema, err := sg.AnalyzeRequestSchema(endpoint); err == nil && requestSchema != nil {
		operation.RequestBody = &OpenAPIRequestBody{
			Description: "Request body",
			Required:    boolPtr(true),
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: requestSchema,
				},
			},
		}
	}

	// Analyze response schema
	if responseSchema, err := sg.AnalyzeResponseSchema(endpoint); err == nil && responseSchema != nil {
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

	// Add common error responses
	sg.addCommonErrorResponses(operation.Responses)

	// Add security if authentication is required
	if endpoint.Auth != nil && endpoint.Auth.Required {
		operation.Security = []map[string][]string{
			{"bearerAuth": {}},
		}
	}

	return &OpenAPIEndpointSchema{
		Operation:   operation,
		Parameters:  operation.Parameters,
		RequestBody: operation.RequestBody,
		Responses:   operation.Responses,
		Security:    operation.Security,
	}, nil
}

// =============================================================================
// Private Helper Methods
// =============================================================================

// generateRequestSchemaFromAST generates request schema from AST analysis
func (sg *SchemaGenerator) generateRequestSchemaFromAST(endpoint *APIEndpoint) *OpenAPISchema {
	if sg.astParser == nil {
		// Return basic schema
		return &OpenAPISchema{
			Type: "object",
			Properties: map[string]*OpenAPISchema{
				"data": {
					Type:        "object",
					Description: "Schema data",
				},
			},
		}
	}

	handlerName := ExtractHandlerBaseName(endpoint.Handler, false)
	if handlerInfo, exists := sg.astParser.GetHandlerByName(handlerName); exists {
		// Return pre-generated request schema from AST analysis
		return handlerInfo.RequestSchema
	}

	return nil
}

// generateResponseSchemaFromAST generates response schema from AST analysis
func (sg *SchemaGenerator) generateResponseSchemaFromAST(endpoint *APIEndpoint) *OpenAPISchema {
	if sg.astParser == nil {
		return nil
	}

	handlerName := ExtractHandlerBaseName(endpoint.Handler, false)
	if handlerInfo, exists := sg.astParser.GetHandlerByName(handlerName); exists {
		// Return pre-generated response schema from AST analysis
		return handlerInfo.ResponseSchema
	}

	return nil
}

// generateStructSchemas generates OpenAPI schemas for all discovered structs
func (sg *SchemaGenerator) generateStructSchemas() map[string]*OpenAPISchema {
	schemas := make(map[string]*OpenAPISchema)

	if sg.astParser != nil {
		structs := sg.astParser.GetAllStructs()
		for name, structInfo := range structs {
			if structInfo.JSONSchema != nil {
				cleanName := CleanTypeName(name)
				schemas[cleanName] = structInfo.JSONSchema
			} else {
				// Convert StructInfo to OpenAPI schema
				cleanName := CleanTypeName(name)
				schemas[cleanName] = ConvertStructInfoToOpenAPISchema(structInfo)
			}
		}
	}

	// Add PocketBase common schemas
	schemas["PocketBaseRecord"] = PocketBaseRecordSchema
	schemas["Error"] = ErrorResponseSchema

	return schemas
}

// generateCommonResponses generates common response schemas
func (sg *SchemaGenerator) generateCommonResponses() map[string]*OpenAPIResponse {
	return map[string]*OpenAPIResponse{
		"BadRequest": {
			Description: "Bad Request",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: ErrorResponseSchema,
				},
			},
		},
		"Unauthorized": {
			Description: "Unauthorized",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: ErrorResponseSchema,
				},
			},
		},
		"Forbidden": {
			Description: "Forbidden",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: ErrorResponseSchema,
				},
			},
		},
		"NotFound": {
			Description: "Not Found",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: ErrorResponseSchema,
				},
			},
		},
		"InternalServerError": {
			Description: "Internal Server Error",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: ErrorResponseSchema,
				},
			},
		},
	}
}

// generateCommonParameters generates common parameter definitions
func (sg *SchemaGenerator) generateCommonParameters() map[string]*OpenAPIParameter {
	return map[string]*OpenAPIParameter{
		"CollectionParam": {
			Name:        "collection",
			In:          "path",
			Description: "Collection name",
			Required:    boolPtr(true),
			Schema:      &OpenAPISchema{Type: "string"},
		},
		"RecordIdParam": {
			Name:        "id",
			In:          "path",
			Description: "Record ID",
			Required:    boolPtr(true),
			Schema:      &OpenAPISchema{Type: "string"},
		},
		"PageParam": {
			Name:        "page",
			In:          "query",
			Description: "Page number",
			Required:    boolPtr(false),
			Schema:      &OpenAPISchema{Type: "integer", Minimum: floatPtr(1), Default: 1},
		},
		"PerPageParam": {
			Name:        "perPage",
			In:          "query",
			Description: "Items per page",
			Required:    boolPtr(false),
			Schema:      &OpenAPISchema{Type: "integer", Minimum: floatPtr(1), Maximum: floatPtr(500), Default: 30},
		},
	}
}

// addCommonErrorResponses adds common error responses to an operation
func (sg *SchemaGenerator) addCommonErrorResponses(responses map[string]*OpenAPIResponse) {
	errorResponses := map[string]*OpenAPIResponse{
		"400": {
			Description: "Bad Request",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {Schema: ErrorResponseSchema},
			},
		},
		"401": {
			Description: "Unauthorized",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {Schema: ErrorResponseSchema},
			},
		},
		"403": {
			Description: "Forbidden",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {Schema: ErrorResponseSchema},
			},
		},
		"404": {
			Description: "Not Found",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {Schema: ErrorResponseSchema},
			},
		},
		"500": {
			Description: "Internal Server Error",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {Schema: ErrorResponseSchema},
			},
		},
	}

	for code, response := range errorResponses {
		if _, exists := responses[code]; !exists {
			responses[code] = response
		}
	}
}

// getSchemaPatterns returns pattern-based schema generators for common PocketBase endpoints
func (sg *SchemaGenerator) getSchemaPatterns() []*SchemaPattern {
	return []*SchemaPattern{
		{
			Name: "Collection List",
			PathMatch: func(path string) bool {
				return regexp.MustCompile(`/collections/[^/]+/records$`).MatchString(path)
			},
			RequestGen: func() *OpenAPISchema {
				return nil // GET requests don't have body
			},
			ResponseGen: func() *OpenAPISchema {
				return sg.generatePaginatedResponseSchema()
			},
		},
		{
			Name: "Collection Create",
			PathMatch: func(path string) bool {
				return regexp.MustCompile(`/collections/[^/]+/records$`).MatchString(path)
			},
			RequestGen: func() *OpenAPISchema {
				return sg.generateRecordCreateSchema()
			},
			ResponseGen: func() *OpenAPISchema {
				return sg.generateSingleRecordSchema()
			},
		},
		{
			Name: "Record Update",
			PathMatch: func(path string) bool {
				return regexp.MustCompile(`/collections/[^/]+/records/[^/]+$`).MatchString(path)
			},
			RequestGen: func() *OpenAPISchema {
				return sg.generateRecordUpdateSchema()
			},
			ResponseGen: func() *OpenAPISchema {
				return sg.generateSingleRecordSchema()
			},
		},
		{
			Name: "Auth Login",
			PathMatch: func(path string) bool {
				return regexp.MustCompile(`/collections/[^/]+/auth-with-password$`).MatchString(path)
			},
			RequestGen: func() *OpenAPISchema {
				return sg.generateLoginRequestSchema()
			},
			ResponseGen: func() *OpenAPISchema {
				return sg.generateAuthResponseSchema()
			},
		},
		{
			Name: "User Registration",
			PathMatch: func(path string) bool {
				return regexp.MustCompile(`/collections/users/records$`).MatchString(path)
			},
			RequestGen: func() *OpenAPISchema {
				return sg.generateRegisterRequestSchema()
			},
			ResponseGen: func() *OpenAPISchema {
				return sg.generateSingleRecordSchema()
			},
		},
		{
			Name: "Record Delete",
			PathMatch: func(path string) bool {
				return regexp.MustCompile(`/collections/[^/]+/records/[^/]+$`).MatchString(path)
			},
			RequestGen: func() *OpenAPISchema {
				return nil // DELETE requests don't have body
			},
			ResponseGen: func() *OpenAPISchema {
				return sg.generateDeleteResponseSchema()
			},
		},
	}
}

// extractPathParameters extracts parameter names from a path
func extractPathParameters(path string) []string {
	var params []string

	// Match {param} style parameters
	re := regexp.MustCompile(`\{([^}]+)\}`)
	matches := re.FindAllStringSubmatch(path, -1)

	for _, match := range matches {
		if len(match) > 1 {
			params = append(params, match[1])
		}
	}

	return params
}

// =============================================================================
// Schema Generation Methods
// =============================================================================

// generateSuccessResponseSchema generates a basic success response schema
func (sg *SchemaGenerator) generateSuccessResponseSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"success": {
				Type:        "boolean",
				Description: "Operation success status",
				Example:     true,
			},
			"message": {
				Type:        "string",
				Description: "Success message",
				Example:     "Operation completed successfully",
			},
		},
		Required: []string{"success"},
	}
}

// generatePaginatedResponseSchema generates a paginated response schema
func (sg *SchemaGenerator) generatePaginatedResponseSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"page": {
				Type:        "integer",
				Description: "Current page number",
				Minimum:     floatPtr(1),
				Example:     1,
			},
			"perPage": {
				Type:        "integer",
				Description: "Items per page",
				Minimum:     floatPtr(1),
				Example:     30,
			},
			"totalItems": {
				Type:        "integer",
				Description: "Total number of items",
				Minimum:     floatPtr(0),
				Example:     150,
			},
			"totalPages": {
				Type:        "integer",
				Description: "Total number of pages",
				Minimum:     floatPtr(1),
				Example:     5,
			},
			"items": {
				Type: "array",
				Items: &OpenAPISchema{
					Ref: "#/components/schemas/PocketBaseRecord",
				},
				Description: "Array of records",
			},
		},
		Required: []string{"page", "perPage", "totalItems", "totalPages", "items"},
	}
}

// generateSingleRecordSchema generates a single record response schema
func (sg *SchemaGenerator) generateSingleRecordSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Ref: "#/components/schemas/PocketBaseRecord",
	}
}

// generateRecordCreateSchema generates a schema for creating records
func (sg *SchemaGenerator) generateRecordCreateSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type:                 "object",
		Description:          "Data for creating a new record",
		AdditionalProperties: true,
		Example: map[string]interface{}{
			"name":  "Example Record",
			"email": "example@test.com",
		},
	}
}

// generateRecordUpdateSchema generates a schema for updating records
func (sg *SchemaGenerator) generateRecordUpdateSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type:                 "object",
		Description:          "Data for updating an existing record",
		AdditionalProperties: true,
		Example: map[string]interface{}{
			"name": "Updated Record Name",
		},
	}
}

// generateLoginRequestSchema generates a login request schema
func (sg *SchemaGenerator) generateLoginRequestSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"identity": {
				Type:        "string",
				Description: "User email or username",
				Example:     "user@example.com",
			},
			"password": {
				Type:        "string",
				Description: "User password",
				Format:      "password",
				Example:     "password123",
			},
		},
		Required: []string{"identity", "password"},
	}
}

// generateRegisterRequestSchema generates a user registration schema
func (sg *SchemaGenerator) generateRegisterRequestSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"email": {
				Type:        "string",
				Format:      "email",
				Description: "User email address",
				Example:     "newuser@example.com",
			},
			"password": {
				Type:        "string",
				Description: "User password",
				Format:      "password",
				MinLength:   intPtr(8),
				Example:     "securepassword123",
			},
			"passwordConfirm": {
				Type:        "string",
				Description: "Password confirmation",
				Format:      "password",
				Example:     "securepassword123",
			},
		},
		Required: []string{"email", "password", "passwordConfirm"},
	}
}

// generateAuthResponseSchema generates an authentication response schema
func (sg *SchemaGenerator) generateAuthResponseSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"token": {
				Type:        "string",
				Description: "JWT authentication token",
				Example:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			},
			"record": {
				Ref:         "#/components/schemas/PocketBaseRecord",
				Description: "User record data",
			},
		},
		Required: []string{"token", "record"},
	}
}

// generateDeleteResponseSchema generates a delete operation response schema
func (sg *SchemaGenerator) generateDeleteResponseSchema() *OpenAPISchema {
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"success": {
				Type:        "boolean",
				Description: "Deletion success status",
				Example:     true,
			},
			"message": {
				Type:        "string",
				Description: "Deletion confirmation message",
				Example:     "Record deleted successfully",
			},
		},
		Required: []string{"success"},
	}
}
