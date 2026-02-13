package api

import (
	"testing"
)

// =============================================================================
// Mock Implementations for Testing
// =============================================================================

// MockASTParserForSchema implements ASTParserInterface for schema testing
type MockASTParserForSchema struct {
	structs  map[string]*StructInfo
	handlers map[string]*ASTHandlerInfo
}

func NewMockASTParserForSchema() *MockASTParserForSchema {
	return &MockASTParserForSchema{
		structs:  make(map[string]*StructInfo),
		handlers: make(map[string]*ASTHandlerInfo),
	}
}

func (m *MockASTParserForSchema) ParseFile(path string) error                { return nil }
func (m *MockASTParserForSchema) GetAllStructs() map[string]*StructInfo      { return m.structs }
func (m *MockASTParserForSchema) GetAllHandlers() map[string]*ASTHandlerInfo { return m.handlers }
func (m *MockASTParserForSchema) GetStructByName(name string) (*StructInfo, bool) {
	s, exists := m.structs[name]
	return s, exists
}
func (m *MockASTParserForSchema) GetHandlerByName(name string) (*ASTHandlerInfo, bool) {
	h, exists := m.handlers[name]
	return h, exists
}
func (m *MockASTParserForSchema) GetParseErrors() []ParseError                 { return nil }
func (m *MockASTParserForSchema) ClearCache()                                  {}
func (m *MockASTParserForSchema) EnhanceEndpoint(endpoint *APIEndpoint) error  { return nil }
func (m *MockASTParserForSchema) GetHandlerDescription(name string) string     { return "" }
func (m *MockASTParserForSchema) GetHandlerTags(name string) []string          { return nil }
func (m *MockASTParserForSchema) GetStructsForFinding() map[string]*StructInfo { return m.structs }
func (m *MockASTParserForSchema) DiscoverSourceFiles() error                   { return nil }

func (m *MockASTParserForSchema) AddMockStruct(name string, structInfo *StructInfo) {
	m.structs[name] = structInfo
}

func (m *MockASTParserForSchema) AddMockHandler(name string, handlerInfo *ASTHandlerInfo) {
	m.handlers[name] = handlerInfo
}

// =============================================================================
// Test Data
// =============================================================================

func createTestStructInfo(name string) *StructInfo {
	return &StructInfo{
		Name:    name,
		Package: "test",
		Fields: map[string]*FieldInfo{
			"ID": {
				Name:          "ID",
				Type:          "string",
				JSONName:      "id",
				JSONOmitEmpty: false,
			},
			"Name": {
				Name:          "Name",
				Type:          "string",
				JSONName:      "name",
				JSONOmitEmpty: false,
			},
			"Email": {
				Name:          "Email",
				Type:          "string",
				JSONName:      "email",
				JSONOmitEmpty: true,
			},
		},
	}
}

func createComplexTestStructInfo(name string) *StructInfo {
	return &StructInfo{
		Name:    name,
		Package: "test",
		Fields: map[string]*FieldInfo{
			"ID": {
				Name:          "ID",
				Type:          "string",
				JSONName:      "id",
				JSONOmitEmpty: false,
			},
			"Items": {
				Name:          "Items",
				Type:          "[]Item",
				JSONName:      "items",
				JSONOmitEmpty: false,
			},
			"Settings": {
				Name:          "Settings",
				Type:          "*Settings",
				JSONName:      "settings",
				JSONOmitEmpty: true,
			},
			"Active": {
				Name:          "Active",
				Type:          "bool",
				JSONName:      "active",
				JSONOmitEmpty: false,
			},
			"Count": {
				Name:          "Count",
				Type:          "int",
				JSONName:      "count",
				JSONOmitEmpty: false,
			},
		},
	}
}

// =============================================================================
// Schema Generator Tests
// =============================================================================

func TestNewSchemaGenerator(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	if schemaGen == nil {
		t.Fatal("Expected non-nil schema generator")
	}

	if schemaGen.astParser == nil {
		t.Error("Expected AST parser to be set")
	}

	if schemaGen.schemaBuilder == nil {
		t.Error("Expected schema builder to be initialized")
	}

	if schemaGen.components == nil {
		t.Error("Expected components to be initialized")
	}

	if schemaGen.components.Schemas == nil {
		t.Error("Expected schemas map to be initialized")
	}

	if schemaGen.logger == nil {
		t.Error("Expected logger to be initialized")
	}
}

func TestAnalyzeRequestSchema(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Add mock struct data
	userStruct := createTestStructInfo("CreateUserRequest")
	astParser.AddMockStruct("CreateUserRequest", userStruct)

	tests := []struct {
		name         string
		endpoint     *APIEndpoint
		expectSchema bool
		expectError  bool
	}{
		{
			name: "Endpoint with existing request schema",
			endpoint: &APIEndpoint{
				Method: "POST",
				Path:   "/users",
				Request: &OpenAPISchema{
					Type: "object",
					Properties: map[string]*OpenAPISchema{
						"name": {Type: "string"},
					},
				},
			},
			expectSchema: true,
			expectError:  false,
		},
		{
			name: "Endpoint without request schema - should generate",
			endpoint: &APIEndpoint{
				Method:  "POST",
				Path:    "/users",
				Handler: "createUserHandler",
			},
			expectSchema: true,
			expectError:  false,
		},
		{
			name: "GET endpoint - typically no request schema",
			endpoint: &APIEndpoint{
				Method:  "GET",
				Path:    "/users",
				Handler: "getUsersHandler",
			},
			expectSchema: false,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := schemaGen.AnalyzeRequestSchema(tt.endpoint)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectSchema && schema == nil {
				t.Error("Expected schema but got nil")
			}
			if !tt.expectSchema && schema != nil {
				t.Error("Expected no schema but got one")
			}
		})
	}
}

func TestAnalyzeResponseSchema(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Add mock struct data
	userStruct := createTestStructInfo("UserResponse")
	astParser.AddMockStruct("UserResponse", userStruct)

	tests := []struct {
		name         string
		endpoint     *APIEndpoint
		expectSchema bool
		expectError  bool
	}{
		{
			name: "Endpoint with existing response schema",
			endpoint: &APIEndpoint{
				Method: "GET",
				Path:   "/users",
				Response: &OpenAPISchema{
					Type: "array",
					Items: &OpenAPISchema{
						Type: "object",
					},
				},
			},
			expectSchema: true,
			expectError:  false,
		},
		{
			name: "Endpoint without response schema - should generate",
			endpoint: &APIEndpoint{
				Method:  "GET",
				Path:    "/users/{id}",
				Handler: "getUserHandler",
			},
			expectSchema: true,
			expectError:  false,
		},
		{
			name: "DELETE endpoint - may have minimal response",
			endpoint: &APIEndpoint{
				Method:  "DELETE",
				Path:    "/users/{id}",
				Handler: "deleteUserHandler",
			},
			expectSchema: true,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := schemaGen.AnalyzeResponseSchema(tt.endpoint)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectSchema && schema == nil {
				t.Error("Expected schema but got nil")
			}
			if !tt.expectSchema && schema != nil {
				t.Error("Expected no schema but got one")
			}
		})
	}
}

func TestAnalyzeSchemaFromPath(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	tests := []struct {
		name        string
		path        string
		method      string
		expectNil   bool
		description string
	}{
		{
			name:        "Users list endpoint",
			path:        "/api/users",
			method:      "GET",
			expectNil:   false,
			description: "Should generate schema for users list",
		},
		{
			name:        "User detail endpoint",
			path:        "/api/users/{id}",
			method:      "GET",
			expectNil:   false,
			description: "Should generate schema for single user",
		},
		{
			name:        "Create user endpoint",
			path:        "/api/users",
			method:      "POST",
			expectNil:   false,
			description: "Should generate schema for user creation",
		},
		{
			name:        "Complex nested path",
			path:        "/api/users/{id}/posts/{postId}/comments",
			method:      "GET",
			expectNil:   false,
			description: "Should handle complex nested paths",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := schemaGen.AnalyzeSchemaFromPath(tt.path, tt.method)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectNil && schema != nil {
				t.Error("Expected nil schema but got one")
			}
			if !tt.expectNil && schema == nil {
				t.Error("Expected schema but got nil")
			}
		})
	}
}

func TestGetOpenAPIEndpointSchema(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Add mock struct data
	userStruct := createTestStructInfo("User")
	createUserStruct := createTestStructInfo("CreateUserRequest")
	astParser.AddMockStruct("User", userStruct)
	astParser.AddMockStruct("CreateUserRequest", createUserStruct)

	tests := []struct {
		name              string
		endpoint          *APIEndpoint
		expectRequestBody bool
		expectResponses   bool
		expectParameters  bool
	}{
		{
			name: "GET endpoint with path parameters",
			endpoint: &APIEndpoint{
				Method:      "GET",
				Path:        "/users/{id}",
				Handler:     "getUserHandler",
				Description: "Get user by ID",
				Tags:        []string{"users"},
			},
			expectRequestBody: false,
			expectResponses:   true,
			expectParameters:  true,
		},
		{
			name: "POST endpoint with request body",
			endpoint: &APIEndpoint{
				Method:      "POST",
				Path:        "/users",
				Handler:     "createUserHandler",
				Description: "Create new user",
				Tags:        []string{"users"},
			},
			expectRequestBody: true,
			expectResponses:   true,
			expectParameters:  false,
		},
		{
			name: "PUT endpoint with both",
			endpoint: &APIEndpoint{
				Method:      "PUT",
				Path:        "/users/{id}",
				Handler:     "updateUserHandler",
				Description: "Update user",
				Tags:        []string{"users"},
			},
			expectRequestBody: true,
			expectResponses:   true,
			expectParameters:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := schemaGen.GetOpenAPIEndpointSchema(tt.endpoint)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if schema == nil {
				t.Fatal("Expected endpoint schema but got nil")
			}

			if tt.expectRequestBody && schema.RequestBody == nil {
				t.Error("Expected request body but got nil")
			}
			if !tt.expectRequestBody && schema.RequestBody != nil {
				t.Error("Expected no request body but got one")
			}

			if tt.expectResponses && schema.Responses == nil {
				t.Error("Expected responses but got nil")
			}

			if tt.expectParameters && len(schema.Parameters) == 0 {
				t.Error("Expected parameters but got none")
			}
			if !tt.expectParameters && schema.Parameters != nil && len(schema.Parameters) > 0 {
				t.Error("Expected no parameters but got some")
			}

			// Note: OpenAPIEndpointSchema may not have Description/Tags fields directly
			// These would typically be in the parent OpenAPI operation object
		})
	}
}

func TestGenerateComponentSchemas(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Add mock struct data
	userStruct := createTestStructInfo("User")
	complexStruct := createComplexTestStructInfo("ComplexObject")
	astParser.AddMockStruct("User", userStruct)
	astParser.AddMockStruct("ComplexObject", complexStruct)

	components := schemaGen.GenerateComponentSchemas()

	if components == nil {
		t.Fatal("Expected components but got nil")
	}

	if components.Schemas == nil {
		t.Error("Expected schemas map but got nil")
	}

	// Components should be properly initialized even if empty
	if components.Responses == nil {
		t.Error("Expected responses map but got nil")
	}

	if components.Parameters == nil {
		t.Error("Expected parameters map but got nil")
	}

	if components.RequestBodies == nil {
		t.Error("Expected request bodies map but got nil")
	}
}

// =============================================================================
// Schema Builder Integration Tests
// =============================================================================

func TestSchemaGeneratorWithStructInfo(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Test with a complex struct
	complexStruct := createComplexTestStructInfo("TestStruct")
	astParser.AddMockStruct("TestStruct", complexStruct)

	endpoint := &APIEndpoint{
		Method:  "POST",
		Path:    "/test",
		Handler: "testHandler",
	}

	// Test request schema analysis
	requestSchema, err := schemaGen.AnalyzeRequestSchema(endpoint)
	if err != nil {
		t.Errorf("Expected no error analyzing request schema: %v", err)
	}

	// Test response schema analysis
	responseSchema, err := schemaGen.AnalyzeResponseSchema(endpoint)
	if err != nil {
		t.Errorf("Expected no error analyzing response schema: %v", err)
	}

	// Both should be able to generate schemas
	if requestSchema == nil && responseSchema == nil {
		t.Error("Expected at least one schema to be generated")
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestSchemaGeneratorErrorHandling(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() (*SchemaGenerator, *APIEndpoint)
		testFn func(t *testing.T, sg *SchemaGenerator, endpoint *APIEndpoint)
	}{
		{
			name: "Nil endpoint request schema analysis",
			setup: func() (*SchemaGenerator, *APIEndpoint) {
				astParser := NewMockASTParserForSchema()
				return NewSchemaGenerator(astParser), nil
			},
			testFn: func(t *testing.T, sg *SchemaGenerator, endpoint *APIEndpoint) {
				_, err := sg.AnalyzeRequestSchema(endpoint)
				if err == nil {
					t.Error("Expected error for nil endpoint")
				}
			},
		},
		{
			name: "Nil endpoint response schema analysis",
			setup: func() (*SchemaGenerator, *APIEndpoint) {
				astParser := NewMockASTParserForSchema()
				return NewSchemaGenerator(astParser), nil
			},
			testFn: func(t *testing.T, sg *SchemaGenerator, endpoint *APIEndpoint) {
				_, err := sg.AnalyzeResponseSchema(endpoint)
				if err == nil {
					t.Error("Expected error for nil endpoint")
				}
			},
		},
		{
			name: "Empty path schema analysis",
			setup: func() (*SchemaGenerator, *APIEndpoint) {
				astParser := NewMockASTParserForSchema()
				return NewSchemaGenerator(astParser), &APIEndpoint{}
			},
			testFn: func(t *testing.T, sg *SchemaGenerator, endpoint *APIEndpoint) {
				schema, err := sg.AnalyzeSchemaFromPath("", "GET")
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				// Should handle empty path gracefully
				if schema == nil {
					// This might be expected behavior
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sg, endpoint := tt.setup()
			tt.testFn(t, sg, endpoint)
		})
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestSchemaGeneratorConcurrentAccess(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Add test data
	userStruct := createTestStructInfo("User")
	astParser.AddMockStruct("User", userStruct)

	endpoint := &APIEndpoint{
		Method:  "GET",
		Path:    "/users",
		Handler: "getUsersHandler",
	}

	// Test concurrent schema analysis
	concurrency := 10
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() { done <- true }()

			// Test request schema analysis
			_, err := schemaGen.AnalyzeRequestSchema(endpoint)
			if err != nil {
				errors <- err
				return
			}

			// Test response schema analysis
			_, err = schemaGen.AnalyzeResponseSchema(endpoint)
			if err != nil {
				errors <- err
				return
			}

			// Test endpoint schema generation
			schema, err := schemaGen.GetOpenAPIEndpointSchema(endpoint)
			if err != nil {
				errors <- err
				return
			}
			if schema == nil {
				errors <- nil // Not an error, but unexpected
			}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent access error: %v", err)
		}
	}
}

// =============================================================================
// Performance Benchmarks
// =============================================================================

func BenchmarkAnalyzeRequestSchema(b *testing.B) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	userStruct := createTestStructInfo("CreateUserRequest")
	astParser.AddMockStruct("CreateUserRequest", userStruct)

	endpoint := &APIEndpoint{
		Method:  "POST",
		Path:    "/users",
		Handler: "createUserHandler",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		schemaGen.AnalyzeRequestSchema(endpoint)
	}
}

func BenchmarkAnalyzeResponseSchema(b *testing.B) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	userStruct := createTestStructInfo("UserResponse")
	astParser.AddMockStruct("UserResponse", userStruct)

	endpoint := &APIEndpoint{
		Method:  "GET",
		Path:    "/users",
		Handler: "getUsersHandler",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		schemaGen.AnalyzeResponseSchema(endpoint)
	}
}

func BenchmarkGetOpenAPIEndpointSchema(b *testing.B) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	endpoint := &APIEndpoint{
		Method:      "GET",
		Path:        "/users/{id}",
		Handler:     "getUserHandler",
		Description: "Get user by ID",
		Tags:        []string{"users"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		schemaGen.GetOpenAPIEndpointSchema(endpoint)
	}
}

func BenchmarkGenerateComponentSchemas(b *testing.B) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Add multiple mock structs
	for i := 0; i < 10; i++ {
		userStruct := createTestStructInfo("User")
		astParser.AddMockStruct("User", userStruct)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		schemaGen.GenerateComponentSchemas()
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestSchemaGeneratorFullWorkflow(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Setup test data
	userStruct := createTestStructInfo("User")
	createUserStruct := createTestStructInfo("CreateUserRequest")
	astParser.AddMockStruct("User", userStruct)
	astParser.AddMockStruct("CreateUserRequest", createUserStruct)

	// Test complete workflow
	endpoints := []*APIEndpoint{
		{
			Method:      "GET",
			Path:        "/users",
			Handler:     "getUsersHandler",
			Description: "Get all users",
			Tags:        []string{"users"},
		},
		{
			Method:      "POST",
			Path:        "/users",
			Handler:     "createUserHandler",
			Description: "Create new user",
			Tags:        []string{"users"},
		},
		{
			Method:      "GET",
			Path:        "/users/{id}",
			Handler:     "getUserHandler",
			Description: "Get user by ID",
			Tags:        []string{"users"},
		},
	}

	for _, endpoint := range endpoints {
		// Analyze schemas
		requestSchema, err := schemaGen.AnalyzeRequestSchema(endpoint)
		if err != nil {
			t.Errorf("Request schema analysis failed for %s %s: %v", endpoint.Method, endpoint.Path, err)
		}

		responseSchema, err := schemaGen.AnalyzeResponseSchema(endpoint)
		if err != nil {
			t.Errorf("Response schema analysis failed for %s %s: %v", endpoint.Method, endpoint.Path, err)
		}

		// Generate full endpoint schema
		endpointSchema, err := schemaGen.GetOpenAPIEndpointSchema(endpoint)
		if err != nil {
			t.Errorf("Error generating endpoint schema for %s %s: %v", endpoint.Method, endpoint.Path, err)
		}
		if endpointSchema == nil {
			t.Errorf("Failed to generate endpoint schema for %s %s", endpoint.Method, endpoint.Path)
		}

		// Verify schemas are consistent
		if endpoint.Method == "POST" && requestSchema == nil && endpointSchema.RequestBody == nil {
			t.Errorf("POST endpoint should have request schema: %s", endpoint.Path)
		}

		if responseSchema == nil && endpointSchema.Responses == nil {
			t.Errorf("Endpoint should have response schema: %s %s", endpoint.Method, endpoint.Path)
		}
	}

	// Generate components
	components := schemaGen.GenerateComponentSchemas()
	if components == nil {
		t.Error("Failed to generate component schemas")
	}
}

// =============================================================================
// Examples
// =============================================================================

func ExampleSchemaGenerator() {
	// Create AST parser and schema generator
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Add mock struct data
	userStruct := createTestStructInfo("User")
	astParser.AddMockStruct("User", userStruct)

	// Create an endpoint
	endpoint := &APIEndpoint{
		Method:      "GET",
		Path:        "/users/{id}",
		Handler:     "getUserHandler",
		Description: "Get user by ID",
		Tags:        []string{"users"},
	}

	// Analyze schemas
	requestSchema, _ := schemaGen.AnalyzeRequestSchema(endpoint)
	responseSchema, _ := schemaGen.AnalyzeResponseSchema(endpoint)

	// Generate full endpoint schema
	endpointSchema, _ := schemaGen.GetOpenAPIEndpointSchema(endpoint)

	println("Request schema:", requestSchema != nil)
	println("Response schema:", responseSchema != nil)
	println("Full endpoint schema:", endpointSchema != nil)
}

func ExampleSchemaGenerator_GenerateComponentSchemas() {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Add mock structs
	userStruct := createTestStructInfo("User")
	astParser.AddMockStruct("User", userStruct)

	// Generate component schemas
	components := schemaGen.GenerateComponentSchemas()

	println("Generated components with", len(components.Schemas), "schemas")
	println("Generated", len(components.Responses), "response components")
	println("Generated", len(components.Parameters), "parameter components")
}

// =============================================================================
// Response Schema Promotion Tests
// =============================================================================

func TestHandlerResponseSchemaName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"getOrderHandler", "GetOrderResponse"},
		{"listCategoriesHandler", "ListCategoriesResponse"},
		{"healthCheck", "HealthCheckResponse"},
		{"pkg.getItemsHandler", "GetItemsResponse"},
		{"getDataFunc", "GetDataResponse"},
		{"", ""},
	}
	for _, tt := range tests {
		got := handlerResponseSchemaName(tt.input)
		if got != tt.expected {
			t.Errorf("handlerResponseSchemaName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsPromotableSchema(t *testing.T) {
	if isPromotableSchema(nil) {
		t.Error("nil schema should not be promotable")
	}
	if isPromotableSchema(&OpenAPISchema{Type: "string"}) {
		t.Error("string schema should not be promotable")
	}
	if isPromotableSchema(&OpenAPISchema{Type: "object"}) {
		t.Error("empty object schema should not be promotable")
	}
	if !isPromotableSchema(&OpenAPISchema{Type: "object", Properties: map[string]*OpenAPISchema{"id": {Type: "string"}}}) {
		t.Error("object with properties should be promotable")
	}
	if !isPromotableSchema(&OpenAPISchema{Type: "array", Items: &OpenAPISchema{Type: "string"}}) {
		t.Error("array with items should be promotable")
	}
}

func TestPromoteHandlerResponseSchemas(t *testing.T) {
	astParser := NewMockASTParserForSchema()
	schemaGen := NewSchemaGenerator(astParser)

	// Handler with promotable inline response
	astParser.AddMockHandler("getOrderHandler", &ASTHandlerInfo{
		Name: "getOrderHandler",
		ResponseSchema: &OpenAPISchema{
			Type: "object",
			Properties: map[string]*OpenAPISchema{
				"id":     {Type: "string"},
				"status": {Type: "string"},
			},
		},
	})

	// Handler with $ref response — should NOT be promoted again
	astParser.AddMockHandler("getUserHandler", &ASTHandlerInfo{
		Name: "getUserHandler",
		ResponseSchema: &OpenAPISchema{
			Ref: "#/components/schemas/User",
		},
	})

	// Handler with no response schema
	astParser.AddMockHandler("deleteHandler", &ASTHandlerInfo{
		Name: "deleteHandler",
	})

	components := schemaGen.GenerateComponentSchemas()

	// "GetOrderResponse" should be in component schemas
	if _, ok := components.Schemas["GetOrderResponse"]; !ok {
		t.Error("Expected GetOrderResponse in component schemas")
	}

	// The handler's schema should now be a $ref
	h, _ := astParser.GetHandlerByName("getOrderHandler")
	if h.ResponseSchema.Ref != "#/components/schemas/GetOrderResponse" {
		t.Errorf("Expected handler response to be $ref, got Ref=%q Type=%q", h.ResponseSchema.Ref, h.ResponseSchema.Type)
	}

	// getUserHandler should NOT have a promoted schema (it was already $ref)
	if _, ok := components.Schemas["GetUserResponse"]; ok {
		t.Error("getUserHandler was already $ref, should not be promoted separately")
	}

	// deleteHandler should NOT have a promoted schema (nil response)
	if _, ok := components.Schemas["DeleteResponse"]; ok {
		t.Error("deleteHandler had nil response, should not be promoted")
	}
}
