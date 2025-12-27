package api

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Mock Dependencies for Testing
// =============================================================================

// MockASTParser implements ASTParserInterface for testing
type MockASTParser struct {
	handlers map[string]*ASTHandlerInfo
	schemas  map[string]*OpenAPISchema
	structs  map[string]*StructInfo
}

func NewMockASTParser() *MockASTParser {
	return &MockASTParser{
		handlers: make(map[string]*ASTHandlerInfo),
		schemas:  make(map[string]*OpenAPISchema),
		structs:  make(map[string]*StructInfo),
	}
}

func (m *MockASTParser) ParseFile(filename string) error {
	return nil
}

func (m *MockASTParser) GetAllStructs() map[string]*StructInfo {
	return m.structs
}

func (m *MockASTParser) GetAllHandlers() map[string]*ASTHandlerInfo {
	return m.handlers
}

func (m *MockASTParser) GetStructByName(name string) (*StructInfo, bool) {
	handler, exists := m.structs[name]
	return handler, exists
}

func (m *MockASTParser) GetHandlerByName(name string) (*ASTHandlerInfo, bool) {
	handler, exists := m.handlers[name]
	return handler, exists
}

func (m *MockASTParser) GetParseErrors() []ParseError {
	return []ParseError{}
}

func (m *MockASTParser) ClearCache() {
	m.handlers = make(map[string]*ASTHandlerInfo)
	m.schemas = make(map[string]*OpenAPISchema)
	m.structs = make(map[string]*StructInfo)
}

func (m *MockASTParser) EnhanceEndpoint(endpoint *APIEndpoint) error {
	// Mock enhancement - modify endpoint to show enhancement occurred
	endpoint.Description = "Mock enhanced description"
	if len(endpoint.Tags) == 0 {
		endpoint.Tags = []string{"mock", "test"}
	}
	return nil
}

func (m *MockASTParser) GetHandlerDescription(handlerName string) string {
	if handler, exists := m.handlers[handlerName]; exists {
		return handler.APIDescription
	}
	return ""
}

func (m *MockASTParser) GetHandlerTags(handlerName string) []string {
	if handler, exists := m.handlers[handlerName]; exists {
		return handler.APITags
	}
	return []string{}
}

func (m *MockASTParser) GetStructsForFinding() map[string]*StructInfo {
	return m.structs
}

func (m *MockASTParser) AddMockHandler(name string, info *ASTHandlerInfo) {
	m.handlers[name] = info
}

func (m *MockASTParser) AddMockSchema(name string, schema *OpenAPISchema) {
	m.schemas[name] = schema
}

// MockSchemaGenerator implements SchemaGeneratorInterface for testing
type MockSchemaGenerator struct {
	components *OpenAPIComponents
}

func NewMockSchemaGenerator() *MockSchemaGenerator {
	return &MockSchemaGenerator{
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
	}
}

func (m *MockSchemaGenerator) AnalyzeRequestSchema(endpoint *APIEndpoint) (*OpenAPISchema, error) {
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"test_field": {
				Type:        "string",
				Description: "Mock request field",
			},
		},
	}, nil
}

func (m *MockSchemaGenerator) AnalyzeResponseSchema(endpoint *APIEndpoint) (*OpenAPISchema, error) {
	return &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"result": {
				Type:        "string",
				Description: "Mock response field",
			},
		},
	}, nil
}

func (m *MockSchemaGenerator) AnalyzeSchemaFromPath(method, path string) (*SchemaAnalysisResult, error) {
	return &SchemaAnalysisResult{
		RequestSchema:  &OpenAPISchema{Type: "object"},
		ResponseSchema: &OpenAPISchema{Type: "object"},
	}, nil
}

func (m *MockSchemaGenerator) GetOpenAPIEndpointSchema(endpoint *APIEndpoint) (*OpenAPIEndpointSchema, error) {
	return &OpenAPIEndpointSchema{
		Operation: &OpenAPIOperation{
			Summary:     "Mock operation",
			Description: "Mock endpoint operation",
		},
		Responses: map[string]*OpenAPIResponse{
			"200": {
				Description: "Success response",
			},
		},
	}, nil
}

func (m *MockSchemaGenerator) GenerateComponentSchemas() *OpenAPIComponents {
	return m.components
}

// =============================================================================
// APIRegistry Constructor Tests
// =============================================================================

func TestNewAPIRegistry(t *testing.T) {
	config := &APIDocsConfig{
		Title:       "Test API",
		Version:     "1.0.0",
		Description: "Test Description",
		BaseURL:     "/api/v1",
		Enabled:     true,
	}

	astParser := NewMockASTParser()
	schemaGen := NewMockSchemaGenerator()

	registry := NewAPIRegistry(config, astParser, schemaGen)

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.config != config {
		t.Error("Expected config to be set")
	}

	if registry.docs.Info.Title != config.Title {
		t.Errorf("Expected title %s, got %s", config.Title, registry.docs.Info.Title)
	}

	if registry.docs.Info.Version != config.Version {
		t.Errorf("Expected version %s, got %s", config.Version, registry.docs.Info.Version)
	}

	if registry.astParser == nil {
		t.Error("Expected AST parser to be set")
	}

	if registry.schemaGenerator == nil {
		t.Error("Expected schema generator to be set")
	}

	if len(registry.endpoints) != 0 {
		t.Error("Expected empty endpoints map")
	}

	if len(registry.docs.endpoints) != 0 {
		t.Error("Expected empty endpoints slice")
	}
}

func TestNewAPIRegistryWithNilConfig(t *testing.T) {
	registry := NewAPIRegistry(nil, nil, nil)

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.config == nil {
		t.Error("Expected default config to be created")
	}

	// Should use default config values
	if registry.docs.Info.Title != "pb-ext API" {
		t.Error("Expected default title")
	}
}

func TestNewAPIRegistryWithDisabledConfig(t *testing.T) {
	config := &APIDocsConfig{
		Title:   "Test API",
		Version: "1.0.0",
		Enabled: false,
	}

	registry := NewAPIRegistry(config, nil, nil)

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	// Should still create registry even when disabled
	if registry.config.Enabled {
		t.Error("Expected registry to respect disabled config")
	}
}

// =============================================================================
// Endpoint Registration Tests
// =============================================================================

func TestRegisterEndpoint(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	endpoint := APIEndpoint{
		Method:      "GET",
		Path:        "/api/test",
		Description: "Test endpoint",
		Tags:        []string{"test"},
		Handler:     "testHandler",
	}

	registry.RegisterEndpoint(endpoint)

	if len(registry.endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(registry.endpoints))
	}

	if len(registry.docs.endpoints) != 1 {
		t.Errorf("Expected 1 endpoint in docs, got %d", len(registry.docs.endpoints))
	}

	key := registry.endpointKey("GET", "/api/test")
	registeredEndpoint, exists := registry.endpoints[key]
	if !exists {
		t.Error("Expected endpoint to be registered")
	}

	if registeredEndpoint.Method != endpoint.Method {
		t.Errorf("Expected method %s, got %s", endpoint.Method, registeredEndpoint.Method)
	}

	if registeredEndpoint.Path != endpoint.Path {
		t.Errorf("Expected path %s, got %s", endpoint.Path, registeredEndpoint.Path)
	}
}

func TestRegisterEndpointDisabled(t *testing.T) {
	config := DefaultAPIDocsConfig()
	config.Enabled = false
	registry := NewAPIRegistry(config, nil, nil)

	endpoint := APIEndpoint{
		Method: "GET",
		Path:   "/api/test",
	}

	registry.RegisterEndpoint(endpoint)

	// Should not register when disabled
	if len(registry.endpoints) != 0 {
		t.Error("Expected no endpoints to be registered when disabled")
	}
}

func TestRegisterMultipleEndpoints(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	endpoints := []APIEndpoint{
		{Method: "GET", Path: "/api/users", Handler: "getUsers"},
		{Method: "POST", Path: "/api/users", Handler: "createUser"},
		{Method: "GET", Path: "/api/users/{id}", Handler: "getUser"},
		{Method: "PUT", Path: "/api/users/{id}", Handler: "updateUser"},
		{Method: "DELETE", Path: "/api/users/{id}", Handler: "deleteUser"},
	}

	for _, endpoint := range endpoints {
		registry.RegisterEndpoint(endpoint)
	}

	if len(registry.endpoints) != len(endpoints) {
		t.Errorf("Expected %d endpoints, got %d", len(endpoints), len(registry.endpoints))
	}

	if len(registry.docs.endpoints) != len(endpoints) {
		t.Errorf("Expected %d endpoints in docs, got %d", len(endpoints), len(registry.docs.endpoints))
	}

	// Verify endpoints are sorted by path then method
	docs := registry.GetDocs()
	for i := 1; i < len(docs.endpoints); i++ {
		prev := docs.endpoints[i-1]
		curr := docs.endpoints[i]

		if prev.Path > curr.Path {
			t.Error("Endpoints should be sorted by path")
		}
		if prev.Path == curr.Path && prev.Method > curr.Method {
			t.Error("Endpoints with same path should be sorted by method")
		}
	}
}

// =============================================================================
// Endpoint Retrieval Tests
// =============================================================================

func TestGetEndpoint(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	original := APIEndpoint{
		Method:      "GET",
		Path:        "/api/test",
		Description: "Test endpoint",
		Tags:        []string{"test"},
	}

	registry.RegisterEndpoint(original)

	retrieved, exists := registry.GetEndpoint("GET", "/api/test")
	if !exists {
		t.Fatal("Expected endpoint to exist")
	}

	if retrieved.Method != original.Method {
		t.Errorf("Expected method %s, got %s", original.Method, retrieved.Method)
	}

	if retrieved.Path != original.Path {
		t.Errorf("Expected path %s, got %s", original.Path, retrieved.Path)
	}

	if retrieved.Description != original.Description {
		t.Errorf("Expected description %s, got %s", original.Description, retrieved.Description)
	}

	// Test case sensitivity
	_, exists = registry.GetEndpoint("get", "/api/test")
	if !exists {
		t.Error("Expected case-insensitive method matching")
	}

	// Test non-existent endpoint
	_, exists = registry.GetEndpoint("POST", "/api/test")
	if exists {
		t.Error("Expected non-existent endpoint to not exist")
	}
}

func TestGetEndpointsByTag(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	endpoints := []APIEndpoint{
		{Method: "GET", Path: "/api/users", Tags: []string{"users", "read"}},
		{Method: "POST", Path: "/api/users", Tags: []string{"users", "write"}},
		{Method: "GET", Path: "/api/posts", Tags: []string{"posts", "read"}},
		{Method: "POST", Path: "/api/posts", Tags: []string{"posts", "write"}},
	}

	for _, endpoint := range endpoints {
		registry.RegisterEndpoint(endpoint)
	}

	// Test filtering by "users" tag
	userEndpoints := registry.GetEndpointsByTag("users")
	if len(userEndpoints) != 2 {
		t.Errorf("Expected 2 user endpoints, got %d", len(userEndpoints))
	}

	// Test filtering by "read" tag
	readEndpoints := registry.GetEndpointsByTag("read")
	if len(readEndpoints) != 2 {
		t.Errorf("Expected 2 read endpoints, got %d", len(readEndpoints))
	}

	// Test non-existent tag
	nonExistent := registry.GetEndpointsByTag("nonexistent")
	if len(nonExistent) != 0 {
		t.Error("Expected no endpoints for non-existent tag")
	}
}

func TestGetRegisteredEndpoints(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	endpoints := []APIEndpoint{
		{Method: "GET", Path: "/api/users"},
		{Method: "POST", Path: "/api/users"},
	}

	for _, endpoint := range endpoints {
		registry.RegisterEndpoint(endpoint)
	}

	registered := registry.GetRegisteredEndpoints()

	if len(registered) != len(endpoints) {
		t.Errorf("Expected %d endpoints, got %d", len(endpoints), len(registered))
	}

	// Should return copies, not originals
	registered[0].Description = "Modified"

	original, _ := registry.GetEndpoint("GET", "/api/users")
	if original.Description == "Modified" {
		t.Error("Returned endpoints should be copies")
	}
}

func TestGetEndpointCount(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	if registry.GetEndpointCount() != 0 {
		t.Error("Expected initial count to be 0")
	}

	registry.RegisterEndpoint(APIEndpoint{Method: "GET", Path: "/test1"})
	if registry.GetEndpointCount() != 1 {
		t.Error("Expected count to be 1")
	}

	registry.RegisterEndpoint(APIEndpoint{Method: "POST", Path: "/test2"})
	if registry.GetEndpointCount() != 2 {
		t.Error("Expected count to be 2")
	}
}

// =============================================================================
// Route Registration Tests
// =============================================================================

func TestRegisterRoute(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	handler := func(c *core.RequestEvent) error { return nil }

	registry.RegisterRoute("GET", "/api/test", handler)

	if len(registry.endpoints) != 1 {
		t.Error("Expected route to be registered as endpoint")
	}

	endpoint, exists := registry.GetEndpoint("GET", "/api/test")
	if !exists {
		t.Fatal("Expected endpoint to exist")
	}

	if endpoint.Method != "GET" {
		t.Errorf("Expected method GET, got %s", endpoint.Method)
	}

	if endpoint.Path != "/api/test" {
		t.Errorf("Expected path /api/test, got %s", endpoint.Path)
	}
}

func TestRegisterRouteWithMiddleware(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	handler := func(c *core.RequestEvent) error { return nil }
	middleware1 := "middleware1"
	middleware2 := "middleware2"

	registry.RegisterRoute("POST", "/api/secure", handler, middleware1, middleware2)

	endpoint, exists := registry.GetEndpoint("POST", "/api/secure")
	if !exists {
		t.Fatal("Expected endpoint to exist")
	}

	// Middleware analysis should be performed by the registry helper
	// We can't easily test this without more complex setup
	if endpoint.Method != "POST" {
		t.Error("Route registration should work with middleware")
	}
}

func TestBatchRegisterRoutes(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	routes := []RouteDefinition{
		{Method: "GET", Path: "/api/users", Handler: func(c *core.RequestEvent) error { return nil }},
		{Method: "POST", Path: "/api/users", Handler: func(c *core.RequestEvent) error { return nil }},
		{Method: "GET", Path: "/api/posts", Handler: func(c *core.RequestEvent) error { return nil }},
	}

	registry.BatchRegisterRoutes(routes)

	if len(registry.endpoints) != len(routes) {
		t.Errorf("Expected %d endpoints, got %d", len(routes), len(registry.endpoints))
	}

	for _, route := range routes {
		if _, exists := registry.GetEndpoint(route.Method, route.Path); !exists {
			t.Errorf("Expected endpoint %s %s to exist", route.Method, route.Path)
		}
	}
}

func TestBatchRegisterRoutesDisabled(t *testing.T) {
	config := DefaultAPIDocsConfig()
	config.Enabled = false
	registry := NewAPIRegistry(config, nil, nil)

	routes := []RouteDefinition{
		{Method: "GET", Path: "/api/test", Handler: func(c *core.RequestEvent) error { return nil }},
	}

	registry.BatchRegisterRoutes(routes)

	if len(registry.endpoints) != 0 {
		t.Error("Expected no routes to be registered when disabled")
	}
}

// =============================================================================
// AST Enhancement Tests
// =============================================================================

func TestASTParsersEnhancement(t *testing.T) {
	astParser := NewMockASTParser()
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), astParser, nil)

	// Add a mock handler to the AST parser
	handlerInfo := &ASTHandlerInfo{
		APIDescription: "AST Enhanced Description",
		APITags:        []string{"ast", "enhanced"},
		RequiresAuth:   true,
		AuthType:       "auth",
	}
	astParser.AddMockHandler("testHandler", handlerInfo)

	endpoint := APIEndpoint{
		Method:  "GET",
		Path:    "/api/test",
		Handler: "testHandler",
	}

	registry.RegisterExplicitRoute(endpoint)

	retrieved, _ := registry.GetEndpoint("GET", "/api/test")

	// Should be enhanced with AST data
	if retrieved.Description != "AST Enhanced Description" {
		t.Errorf("Expected AST description, got %s", retrieved.Description)
	}

	if len(retrieved.Tags) != 2 || retrieved.Tags[0] != "ast" {
		t.Errorf("Expected AST tags, got %v", retrieved.Tags)
	}

	if retrieved.Auth == nil || !retrieved.Auth.Required {
		t.Error("Expected AST auth info to be set")
	}
}

func TestASTEnhancementFallback(t *testing.T) {
	astParser := NewMockASTParser()
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), astParser, nil)

	endpoint := APIEndpoint{
		Method:      "GET",
		Path:        "/api/test",
		Handler:     "unknownHandler",
		Description: "Original Description",
	}

	registry.RegisterExplicitRoute(endpoint)

	retrieved, _ := registry.GetEndpoint("GET", "/api/test")

	// Should fall back to mock enhancement when specific handler not found
	if retrieved.Description == "Original Description" {
		t.Error("Expected fallback AST enhancement to modify description")
	}
}

// =============================================================================
// Schema Generation Tests
// =============================================================================

func TestSchemaGeneration(t *testing.T) {
	schemaGen := NewMockSchemaGenerator()
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, schemaGen)

	endpoint := APIEndpoint{
		Method: "POST",
		Path:   "/api/test",
	}

	registry.RegisterExplicitRoute(endpoint)

	retrieved, _ := registry.GetEndpoint("POST", "/api/test")

	// Should have generated schemas
	if retrieved.Request == nil {
		t.Error("Expected request schema to be generated")
	}

	if retrieved.Response == nil {
		t.Error("Expected response schema to be generated")
	}

	if retrieved.Request != nil && retrieved.Request.Type != "object" {
		t.Error("Expected request schema to be object type")
	}
}

func TestGetDocsWithComponents(t *testing.T) {
	schemaGen := NewMockSchemaGenerator()
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, schemaGen)

	endpoint := APIEndpoint{Method: "GET", Path: "/api/test"}
	registry.RegisterExplicitRoute(endpoint)

	docs := registry.GetDocsWithComponents()

	if docs.Components == nil {
		t.Error("Expected components to be generated")
	}

	if docs.Components.Schemas == nil {
		t.Error("Expected schemas in components")
	}
}

// =============================================================================
// Configuration Management Tests
// =============================================================================

func TestUpdateConfig(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	newConfig := &APIDocsConfig{
		Title:       "Updated API",
		Version:     "2.0.0",
		Description: "Updated Description",
		BaseURL:     "/api/v2",
		Enabled:     true,
	}

	registry.UpdateConfig(newConfig)

	if registry.config != newConfig {
		t.Error("Expected config to be updated")
	}

	docs := registry.GetDocs()
	if docs.Info.Title != newConfig.Title {
		t.Errorf("Expected title %s, got %s", newConfig.Title, docs.Info.Title)
	}

	if docs.Info.Version != newConfig.Version {
		t.Errorf("Expected version %s, got %s", newConfig.Version, docs.Info.Version)
	}
}

func TestUpdateConfigNil(t *testing.T) {
	originalConfig := DefaultAPIDocsConfig()
	registry := NewAPIRegistry(originalConfig, nil, nil)

	registry.UpdateConfig(nil)

	// Should not change config when nil is passed
	if registry.config != originalConfig {
		t.Error("Expected config to remain unchanged")
	}
}

// =============================================================================
// Clear Endpoints Tests
// =============================================================================

func TestClearEndpoints(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	// Add some endpoints
	endpoints := []APIEndpoint{
		{Method: "GET", Path: "/api/test1"},
		{Method: "POST", Path: "/api/test2"},
	}

	for _, endpoint := range endpoints {
		registry.RegisterEndpoint(endpoint)
	}

	if len(registry.endpoints) == 0 {
		t.Error("Expected endpoints to be registered")
	}

	registry.ClearEndpoints()

	if len(registry.endpoints) != 0 {
		t.Error("Expected endpoints to be cleared")
	}

	if len(registry.docs.endpoints) != 0 {
		t.Error("Expected docs endpoints to be cleared")
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestConcurrentEndpointRegistration(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	numGoroutines := 10
	endpointsPerGoroutine := 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < endpointsPerGoroutine; j++ {
				endpoint := APIEndpoint{
					Method: "GET",
					Path:   fmt.Sprintf("/api/test-%d-%d", routineID, j),
				}
				registry.RegisterEndpoint(endpoint)
			}
		}(i)
	}

	wg.Wait()

	expectedCount := numGoroutines * endpointsPerGoroutine
	if registry.GetEndpointCount() != expectedCount {
		t.Errorf("Expected %d endpoints, got %d", expectedCount, registry.GetEndpointCount())
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	// Add initial endpoints
	for i := 0; i < 5; i++ {
		endpoint := APIEndpoint{
			Method: "GET",
			Path:   fmt.Sprintf("/api/initial-%d", i),
		}
		registry.RegisterEndpoint(endpoint)
	}

	numReaders := 5
	numWriters := 3
	var wg sync.WaitGroup
	wg.Add(numReaders + numWriters)

	// Start readers
	for i := 0; i < numReaders; i++ {
		go func(readerID int) {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				docs := registry.GetDocs()
				_ = docs.Paths // Access the data

				registry.GetEndpointCount()

				registry.GetEndpointsByTag("test")

				_, _ = registry.GetEndpoint("GET", "/api/initial-0")
			}
		}(i)
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < 50; j++ {
				endpoint := APIEndpoint{
					Method: "POST",
					Path:   fmt.Sprintf("/api/concurrent-%d-%d", writerID, j),
				}
				registry.RegisterEndpoint(endpoint)
			}
		}(i)
	}

	wg.Wait()

	// Should have initial + writer endpoints
	expectedMin := 5 + (numWriters * 50)
	if registry.GetEndpointCount() < expectedMin {
		t.Errorf("Expected at least %d endpoints, got %d", expectedMin, registry.GetEndpointCount())
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestDuplicateEndpointRegistration(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	endpoint1 := APIEndpoint{
		Method:      "GET",
		Path:        "/api/test",
		Description: "First description",
	}

	endpoint2 := APIEndpoint{
		Method:      "GET",
		Path:        "/api/test",
		Description: "Second description",
	}

	registry.RegisterEndpoint(endpoint1)
	registry.RegisterEndpoint(endpoint2)

	// Should have only one endpoint (second one overwrites first)
	if registry.GetEndpointCount() != 1 {
		t.Errorf("Expected 1 endpoint, got %d", registry.GetEndpointCount())
	}

	retrieved, _ := registry.GetEndpoint("GET", "/api/test")
	if retrieved.Description != "Second description" {
		t.Error("Expected second endpoint to overwrite first")
	}
}

func TestEmptyPathAndMethod(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	endpoint := APIEndpoint{
		Method: "",
		Path:   "",
	}

	registry.RegisterEndpoint(endpoint)

	if registry.GetEndpointCount() != 1 {
		t.Error("Should allow empty method and path")
	}

	retrieved, exists := registry.GetEndpoint("", "")
	if !exists {
		t.Error("Should be able to retrieve endpoint with empty method and path")
	}

	if retrieved.Method != "" || retrieved.Path != "" {
		t.Error("Should preserve empty values")
	}
}

func TestLongEndpointData(t *testing.T) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	longDescription := strings.Repeat("A", 10000)
	manyTags := make([]string, 1000)
	for i := range manyTags {
		manyTags[i] = fmt.Sprintf("tag-%d", i)
	}

	endpoint := APIEndpoint{
		Method:      "GET",
		Path:        "/api/long-test",
		Description: longDescription,
		Tags:        manyTags,
	}

	registry.RegisterEndpoint(endpoint)

	retrieved, exists := registry.GetEndpoint("GET", "/api/long-test")
	if !exists {
		t.Fatal("Expected endpoint to exist")
	}

	if retrieved.Description != longDescription {
		t.Error("Should preserve long description")
	}

	if len(retrieved.Tags) != len(manyTags) {
		t.Error("Should preserve all tags")
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkRegisterEndpoint(b *testing.B) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	endpoint := APIEndpoint{
		Method:      "GET",
		Path:        "/api/test",
		Description: "Benchmark test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		endpoint.Path = fmt.Sprintf("/api/test-%d", i)
		registry.RegisterEndpoint(endpoint)
	}
}

func BenchmarkGetEndpoint(b *testing.B) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	// Pre-register many endpoints
	for i := 0; i < 1000; i++ {
		endpoint := APIEndpoint{
			Method: "GET",
			Path:   fmt.Sprintf("/api/test-%d", i),
		}
		registry.RegisterEndpoint(endpoint)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := fmt.Sprintf("/api/test-%d", i%1000)
		_, _ = registry.GetEndpoint("GET", path)
	}
}

func BenchmarkGetDocs(b *testing.B) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	// Pre-register endpoints
	for i := 0; i < 100; i++ {
		endpoint := APIEndpoint{
			Method: "GET",
			Path:   fmt.Sprintf("/api/test-%d", i),
		}
		registry.RegisterEndpoint(endpoint)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.GetDocs()
	}
}

func BenchmarkConcurrentAccess(b *testing.B) {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	// Pre-register endpoints
	for i := 0; i < 50; i++ {
		endpoint := APIEndpoint{
			Method: "GET",
			Path:   fmt.Sprintf("/api/test-%d", i),
		}
		registry.RegisterEndpoint(endpoint)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Mix of read and write operations
			if i := time.Now().UnixNano() % 4; i == 0 {
				endpoint := APIEndpoint{
					Method: "POST",
					Path:   fmt.Sprintf("/api/bench-%d", time.Now().UnixNano()),
				}
				registry.RegisterEndpoint(endpoint)
			} else {
				_ = registry.GetDocs()
			}
		}
	})
}

// =============================================================================
// Helper Functions and Utilities
// =============================================================================

// Helper function to create a test endpoint
func createTestEndpoint(method, path, description string) APIEndpoint {
	return APIEndpoint{
		Method:      method,
		Path:        path,
		Description: description,
		Tags:        []string{"test"},
		Handler:     "testHandler",
	}
}

// Helper function to create multiple test endpoints
func createTestEndpoints(count int) []APIEndpoint {
	endpoints := make([]APIEndpoint, count)
	for i := 0; i < count; i++ {
		endpoints[i] = APIEndpoint{
			Method:      "GET",
			Path:        fmt.Sprintf("/api/test-%d", i),
			Description: fmt.Sprintf("Test endpoint %d", i),
			Tags:        []string{fmt.Sprintf("tag-%d", i)},
			Handler:     fmt.Sprintf("testHandler%d", i),
		}
	}
	return endpoints
}

// Helper function to verify endpoint equality
func compareEndpoints(t *testing.T, expected, actual APIEndpoint) {
	if expected.Method != actual.Method {
		t.Errorf("Expected method %s, got %s", expected.Method, actual.Method)
	}
	if expected.Path != actual.Path {
		t.Errorf("Expected path %s, got %s", expected.Path, actual.Path)
	}
	if expected.Description != actual.Description {
		t.Errorf("Expected description %s, got %s", expected.Description, actual.Description)
	}
	if expected.Handler != actual.Handler {
		t.Errorf("Expected handler %s, got %s", expected.Handler, actual.Handler)
	}
}

// Helper function to create registry with test data
func createRegistryWithTestData() *APIRegistry {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), NewMockASTParser(), NewMockSchemaGenerator())

	endpoints := []APIEndpoint{
		{Method: "GET", Path: "/api/users", Description: "Get users", Tags: []string{"users"}},
		{Method: "POST", Path: "/api/users", Description: "Create user", Tags: []string{"users"}},
		{Method: "GET", Path: "/api/posts", Description: "Get posts", Tags: []string{"posts"}},
	}

	for _, endpoint := range endpoints {
		registry.RegisterEndpoint(endpoint)
	}

	return registry
}

// Test helper function to validate OpenAPI components
func validateOpenAPIComponents(t *testing.T, components *OpenAPIComponents) {
	if components == nil {
		t.Fatal("Expected non-nil components")
	}

	if components.Schemas == nil {
		t.Error("Expected schemas map to be initialized")
	}

	if components.Responses == nil {
		t.Error("Expected responses map to be initialized")
	}

	if components.Parameters == nil {
		t.Error("Expected parameters map to be initialized")
	}
}

// Example function showing how to use the registry
func ExampleAPIRegistry() {
	// Create a new registry
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	// Register an endpoint
	endpoint := APIEndpoint{
		Method:      "GET",
		Path:        "/api/users",
		Description: "Get all users",
		Tags:        []string{"users", "public"},
	}

	registry.RegisterEndpoint(endpoint)

	// Get the endpoint back
	retrieved, exists := registry.GetEndpoint("GET", "/api/users")
	if exists {
		fmt.Printf("Found endpoint: %s %s\n", retrieved.Method, retrieved.Path)
	}

	// Output: Found endpoint: GET /api/users
}

// Example function showing registry with AST enhancement
func ExampleAPIRegistry_withAST() {
	astParser := NewMockASTParser()
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), astParser, nil)

	// Add mock handler info
	handlerInfo := &ASTHandlerInfo{
		APIDescription: "Enhanced by AST",
		APITags:        []string{"enhanced", "ast"},
	}
	astParser.AddMockHandler("testHandler", handlerInfo)

	// Register endpoint
	endpoint := APIEndpoint{
		Method:  "POST",
		Path:    "/api/test",
		Handler: "testHandler",
	}

	registry.RegisterExplicitRoute(endpoint)

	// The endpoint should be enhanced with AST data
	docs := registry.GetDocs()
	fmt.Printf("Enhanced endpoint description: %s\n", docs.endpoints[0].Description)

	// Output: Enhanced endpoint description: Enhanced by AST
}

// Table-driven test helper
type endpointTestCase struct {
	name     string
	endpoint APIEndpoint
	wantErr  bool
}

// Generate test cases for various endpoint scenarios
func generateEndpointTestCases() []endpointTestCase {
	return []endpointTestCase{
		{
			name: "basic GET endpoint",
			endpoint: APIEndpoint{
				Method: "GET",
				Path:   "/api/test",
			},
		},
		{
			name: "POST endpoint with description",
			endpoint: APIEndpoint{
				Method:      "POST",
				Path:        "/api/create",
				Description: "Create resource",
			},
		},
		{
			name: "endpoint with tags",
			endpoint: APIEndpoint{
				Method: "DELETE",
				Path:   "/api/delete/{id}",
				Tags:   []string{"admin", "delete"},
			},
		},
	}
}

// Performance test helper
func setupPerformanceTest(endpointCount int) *APIRegistry {
	registry := NewAPIRegistry(DefaultAPIDocsConfig(), nil, nil)

	for i := 0; i < endpointCount; i++ {
		endpoint := APIEndpoint{
			Method:      "GET",
			Path:        fmt.Sprintf("/api/endpoint-%d", i),
			Description: fmt.Sprintf("Performance test endpoint %d", i),
			Tags:        []string{fmt.Sprintf("perf-%d", i%10)},
		}
		registry.RegisterEndpoint(endpoint)
	}

	return registry
}
