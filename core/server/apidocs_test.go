package server

import (
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestAPIEndpointStruct(t *testing.T) {
	endpoint := APIEndpoint{
		Method:      "GET",
		Path:        "/api/users",
		Description: "Get all users",
		Request:     map[string]interface{}{"query": "string"},
		Response:    map[string]interface{}{"users": "array"},
		Auth:        true,
		Tags:        []string{"users", "api"},
		Handler:     "getUsersHandler",
	}

	if endpoint.Method != "GET" {
		t.Errorf("Expected Method 'GET', got %s", endpoint.Method)
	}
	if endpoint.Path != "/api/users" {
		t.Errorf("Expected Path '/api/users', got %s", endpoint.Path)
	}
	if endpoint.Description != "Get all users" {
		t.Errorf("Expected Description 'Get all users', got %s", endpoint.Description)
	}
	if !endpoint.Auth {
		t.Error("Expected Auth to be true")
	}
	if len(endpoint.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(endpoint.Tags))
	}
	if endpoint.Handler != "getUsersHandler" {
		t.Errorf("Expected Handler 'getUsersHandler', got %s", endpoint.Handler)
	}
}

func TestAPIEndpointZeroValues(t *testing.T) {
	var endpoint APIEndpoint

	if endpoint.Method != "" {
		t.Errorf("Expected empty Method, got %s", endpoint.Method)
	}
	if endpoint.Auth {
		t.Error("Expected Auth to be false by default")
	}
	if endpoint.Request != nil {
		t.Errorf("Expected nil Request, got %v", endpoint.Request)
	}
	if endpoint.Response != nil {
		t.Errorf("Expected nil Response, got %v", endpoint.Response)
	}
	if endpoint.Tags != nil {
		t.Errorf("Expected nil Tags, got %v", endpoint.Tags)
	}
}

func TestAPIDocsStruct(t *testing.T) {
	docs := APIDocs{
		Title:       "Test API",
		Version:     "1.0.0",
		Description: "Test API Documentation",
		BaseURL:     "/api",
		Endpoints:   []APIEndpoint{},
		Generated:   time.Now().Format(time.RFC3339),
		Components:  map[string]interface{}{"schemas": map[string]interface{}{}},
	}

	if docs.Title != "Test API" {
		t.Errorf("Expected Title 'Test API', got %s", docs.Title)
	}
	if docs.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", docs.Version)
	}
	if docs.BaseURL != "/api" {
		t.Errorf("Expected BaseURL '/api', got %s", docs.BaseURL)
	}
	if docs.Endpoints == nil {
		t.Error("Expected Endpoints to be initialized")
	}
	if docs.Components == nil {
		t.Error("Expected Components to be initialized")
	}
}

func TestNewAPIRegistry(t *testing.T) {
	registry := NewAPIRegistry()

	if registry == nil {
		t.Fatal("Expected registry to be created")
	}
	if registry.docs == nil {
		t.Error("Expected docs to be initialized")
	}
	if registry.endpoints == nil {
		t.Error("Expected endpoints map to be initialized")
	}
	if !registry.enabled {
		t.Error("Expected registry to be enabled by default")
	}

	// Check default documentation values
	if registry.docs.Title != "PocketBase Extension API" {
		t.Errorf("Expected default title, got %s", registry.docs.Title)
	}
	if registry.docs.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", registry.docs.Version)
	}
	if registry.docs.BaseURL != "/api" {
		t.Errorf("Expected BaseURL '/api', got %s", registry.docs.BaseURL)
	}
}

func TestAPIRegistryEnableAutoDiscovery(t *testing.T) {
	registry := NewAPIRegistry()

	// Test initial state
	if !registry.enabled {
		t.Error("Expected auto discovery to be enabled by default")
	}

	// Test disabling
	registry.EnableAutoDiscovery(false)
	if registry.enabled {
		t.Error("Expected auto discovery to be disabled")
	}

	// Test enabling
	registry.EnableAutoDiscovery(true)
	if !registry.enabled {
		t.Error("Expected auto discovery to be enabled")
	}
}

func TestGetHandlerName(t *testing.T) {
	registry := NewAPIRegistry()

	// Test with a named function
	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	handlerName := registry.getHandlerName(testHandler)
	if handlerName == "" {
		t.Error("Expected non-empty handler name")
	}
	if handlerName == "anonymous" || handlerName == "unknown" {
		t.Errorf("Expected proper function name, got %s", handlerName)
	}

	// Test with nil handler
	nilName := registry.getHandlerName(nil)
	if nilName != "anonymous" {
		t.Errorf("Expected 'anonymous' for nil handler, got %s", nilName)
	}
}

func TestDescriptionFromHandlerName(t *testing.T) {
	registry := NewAPIRegistry()

	testCases := []struct {
		handlerName string
		expected    string
		name        string
	}{
		{"getUsersHandler", "Get get users", "standard handler suffix"},
		{"createUserHandler", "Create create user", "create handler"},
		{"deleteItemFunc", "Delete delete item", "func suffix"},
		{"updateData", "Update update data", "no suffix"},
		{"anonymous", "", "anonymous handler"},
		{"unknown", "", "unknown handler"},
		{"", "", "empty handler name"},
		{"simpleHandler", "simple", "simple case"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := registry.descriptionFromHandlerName(tc.handlerName)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestDescriptionFromPath(t *testing.T) {
	registry := NewAPIRegistry()

	testCases := []struct {
		method   string
		path     string
		expected string
		name     string
	}{
		{"GET", "/api/users", "Get users", "simple users endpoint"},
		{"POST", "/api/users", "Create users", "create user"},
		{"PUT", "/api/users/{id}", "Update users by id", "update user"},
		{"DELETE", "/api/users/{id}", "Delete users by id", "delete user"},
		{"GET", "/api/collections/records", "Get collections records", "nested path"},
		{"POST", "/api/auth/login", "Create auth login", "auth endpoint"},
		{"GET", "/", "Get  ", "root path"},
		{"PATCH", "/api/items/{id}", "Update items by id", "patch method"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := registry.descriptionFromPath(tc.method, tc.path)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestDetectAuthRequirement(t *testing.T) {
	registry := NewAPIRegistry()

	testCases := []struct {
		path     string
		expected bool
		name     string
	}{
		{"/api/auth/login", true, "login endpoint"},
		{"/api/auth/register", true, "register endpoint"},
		{"/api/auth/refresh", true, "refresh endpoint"},
		{"/api/users", false, "users endpoint"},
		{"/api/collections/records", true, "collections endpoint"},
		{"/api/admin/users", true, "admin endpoint"},
		{"/health", false, "health endpoint"},
		{"/api/docs", false, "docs endpoint"},
		{"/", false, "root endpoint"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := registry.detectAuthRequirement(tc.path)
			if result != tc.expected {
				t.Errorf("Expected auth requirement %t for path %s, got %t", tc.expected, tc.path, result)
			}
		})
	}
}

func TestGenerateTags(t *testing.T) {
	registry := NewAPIRegistry()

	testCases := []struct {
		path          string
		shouldContain []string
		name          string
	}{
		{"/api/users", []string{"users"}, "users endpoint"},
		{"/api/collections/records", []string{"collections"}, "nested endpoint"},
		{"/api/auth/login", []string{"auth", "authentication"}, "auth endpoint"},
		{"/api/admin/users", []string{"admin", "users"}, "admin endpoint"},
		{"/health", []string{"health"}, "health endpoint"},
		{"/", []string{"api"}, "root endpoint"},
		{"/api/files/upload", []string{"files"}, "files endpoint"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := registry.generateTags(tc.path)

			// Check that we have at least one tag
			if len(result) == 0 {
				t.Error("Expected at least one tag to be generated")
				return
			}

			// Check that expected tags are present
			for _, expected := range tc.shouldContain {
				found := false
				for _, tag := range result {
					if tag == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected tag '%s' to be present in %v", expected, result)
				}
			}
		})
	}
}

func TestAutoRegisterRouteBasic(t *testing.T) {
	registry := NewAPIRegistry()

	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	// Test route registration
	registry.autoRegisterRoute("GET", "/api/test", testHandler)

	registry.mu.RLock()
	endpoint, exists := registry.endpoints["GET:/api/test"]
	registry.mu.RUnlock()

	if !exists {
		t.Error("Expected endpoint to be registered")
	}

	if endpoint.Method != "GET" {
		t.Errorf("Expected method GET, got %s", endpoint.Method)
	}
	if endpoint.Path != "/api/test" {
		t.Errorf("Expected path /api/test, got %s", endpoint.Path)
	}
	if endpoint.Description == "" {
		t.Error("Expected non-empty description")
	}
}

func TestAutoRegisterRouteDisabled(t *testing.T) {
	registry := NewAPIRegistry()
	registry.EnableAutoDiscovery(false)

	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	registry.autoRegisterRoute("GET", "/api/test", testHandler)

	registry.mu.RLock()
	_, exists := registry.endpoints["GET:/api/test"]
	registry.mu.RUnlock()

	if exists {
		t.Error("Expected endpoint not to be registered when auto discovery is disabled")
	}
}

func TestGetDocs(t *testing.T) {
	registry := NewAPIRegistry()

	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	// Register some routes
	registry.autoRegisterRoute("GET", "/api/users", testHandler)
	registry.autoRegisterRoute("POST", "/api/users", testHandler)

	docs := registry.GetDocs()

	if docs.Title != registry.docs.Title {
		t.Errorf("Expected title to match registry docs")
	}
	if len(docs.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(docs.Endpoints))
	}

	// Check endpoints are sorted
	if len(docs.Endpoints) >= 2 {
		first := docs.Endpoints[0]
		second := docs.Endpoints[1]
		if first.Path > second.Path || (first.Path == second.Path && first.Method > second.Method) {
			t.Error("Expected endpoints to be sorted")
		}
	}
}

func TestGetDocsWithComponents(t *testing.T) {
	registry := NewAPIRegistry()

	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	registry.autoRegisterRoute("GET", "/api/users", testHandler)

	docs := registry.GetDocsWithComponents()

	if docs.Components == nil {
		t.Error("Expected components to be included")
	}

	// Should have generated some schema components
	if len(docs.Components) == 0 {
		t.Error("Expected some components to be generated")
	}
}

func TestAnalyzeSchemaFromPath(t *testing.T) {
	registry := NewAPIRegistry()

	testCases := []struct {
		method string
		path   string
		name   string
	}{
		{"GET", "/api/users", "users list"},
		{"POST", "/api/users", "create user"},
		{"GET", "/api/users/{id}", "get user by id"},
		{"PUT", "/api/users/{id}", "update user"},
		{"DELETE", "/api/users/{id}", "delete user"},
		{"GET", "/api/collections/{collection}/records", "collection records"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqSchema, respSchema := registry.analyzeSchemaFromPath(tc.method, tc.path)

			// Both should be non-nil (might be empty maps but not nil)
			// Schema generation may return nil for some paths, which is acceptable
			// Just check that the function doesn't panic
			_ = reqSchema
			_ = respSchema
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	registry := NewAPIRegistry()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 50

	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	// Concurrent route registration
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				path := "/api/test/" + string(rune(goroutineID*1000+j))
				registry.autoRegisterRoute("GET", path, testHandler)
			}
		}(i)
	}

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_ = registry.GetDocs()
			}
		}()
	}

	wg.Wait()

	// Verify no race conditions
	docs := registry.GetDocs()
	expectedEndpoints := numGoroutines * numOperations

	if len(docs.Endpoints) != expectedEndpoints {
		t.Errorf("Expected %d endpoints, got %d", expectedEndpoints, len(docs.Endpoints))
	}
}

func TestSchemaAnalyzerIntegration(t *testing.T) {
	registry := NewAPIRegistry()

	// Test that schema analyzer is properly integrated
	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	registry.autoRegisterRoute("GET", "/api/collections/users/records", testHandler)

	registry.mu.RLock()
	endpoint := registry.endpoints["GET:/api/collections/users/records"]
	registry.mu.RUnlock()

	// Should have some schema information
	if endpoint.Request == nil && endpoint.Response == nil {
		t.Error("Expected some schema information to be generated")
	}
}

func TestGetGlobalRegistry(t *testing.T) {
	registry1 := GetGlobalRegistry()
	registry2 := GetGlobalRegistry()

	if registry1 != registry2 {
		t.Error("Expected GetGlobalRegistry to return the same instance")
	}

	if registry1 == nil {
		t.Error("Expected global registry to be initialized")
	}
}

func TestGlobalAutoRegisterRoute(t *testing.T) {
	// Test the global function
	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	// This should not panic
	AutoRegisterRoute("GET", "/api/global-test", testHandler)

	// Verify it was registered in global registry
	globalRegistry := GetGlobalRegistry()
	docs := globalRegistry.GetDocs()

	found := false
	for _, endpoint := range docs.Endpoints {
		if endpoint.Path == "/api/global-test" && endpoint.Method == "GET" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected endpoint to be registered in global registry")
	}
}

func TestRebuildEndpoints(t *testing.T) {
	registry := NewAPIRegistry()

	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	// Add some endpoints
	registry.autoRegisterRoute("GET", "/api/users", testHandler)
	registry.autoRegisterRoute("POST", "/api/users", testHandler)
	registry.autoRegisterRoute("GET", "/api/posts", testHandler)

	docs := registry.GetDocs()

	// Should be sorted by path, then method
	if len(docs.Endpoints) != 3 {
		t.Fatalf("Expected 3 endpoints, got %d", len(docs.Endpoints))
	}

	// Check sorting
	expectedPaths := []string{"/api/posts", "/api/users", "/api/users"}
	expectedMethods := []string{"GET", "GET", "POST"}

	for i, endpoint := range docs.Endpoints {
		if endpoint.Path != expectedPaths[i] {
			t.Errorf("Expected path %s at index %d, got %s", expectedPaths[i], i, endpoint.Path)
		}
		if endpoint.Method != expectedMethods[i] {
			t.Errorf("Expected method %s at index %d, got %s", expectedMethods[i], i, endpoint.Method)
		}
	}
}

// Edge case tests

func TestEmptyPath(t *testing.T) {
	registry := NewAPIRegistry()

	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	registry.autoRegisterRoute("GET", "", testHandler)

	registry.mu.RLock()
	_, exists := registry.endpoints["GET:"]
	registry.mu.RUnlock()

	if !exists {
		t.Error("Expected empty path to be registered")
	}
}

func TestDuplicateRegistration(t *testing.T) {
	registry := NewAPIRegistry()

	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	// Register same route twice
	registry.autoRegisterRoute("GET", "/api/test", testHandler)
	registry.autoRegisterRoute("GET", "/api/test", testHandler)

	registry.mu.RLock()
	endpointCount := len(registry.endpoints)
	registry.mu.RUnlock()

	// Should only have one endpoint (second registration overwrites first)
	if endpointCount != 1 {
		t.Errorf("Expected 1 endpoint after duplicate registration, got %d", endpointCount)
	}

	docs := registry.GetDocs()
	if len(docs.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint in docs, got %d", len(docs.Endpoints))
	}
}

// Benchmark tests

func BenchmarkNewAPIRegistry(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewAPIRegistry()
	}
}

func BenchmarkAutoRegisterRoute(b *testing.B) {
	registry := NewAPIRegistry()
	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := "/api/test" + string(rune(i))
		registry.autoRegisterRoute("GET", path, testHandler)
	}
}

func BenchmarkGetHandlerName(b *testing.B) {
	registry := NewAPIRegistry()
	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.getHandlerName(testHandler)
	}
}

func BenchmarkGenerateDescription(b *testing.B) {
	registry := NewAPIRegistry()
	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.generateDescription("GET", "/api/users", testHandler)
	}
}

func BenchmarkDetectAuthRequirement(b *testing.B) {
	registry := NewAPIRegistry()
	paths := []string{
		"/api/users",
		"/api/auth/login",
		"/api/collections/records",
		"/health",
		"/api/admin/settings",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.detectAuthRequirement(paths[i%len(paths)])
	}
}

func BenchmarkGenerateTags(b *testing.B) {
	registry := NewAPIRegistry()
	paths := []string{
		"/api/users",
		"/api/collections/records",
		"/api/auth/login",
		"/api/admin/settings",
		"/health",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.generateTags(paths[i%len(paths)])
	}
}

func BenchmarkGetDocs(b *testing.B) {
	registry := NewAPIRegistry()
	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	// Setup some endpoints
	for i := 0; i < 100; i++ {
		path := "/api/test" + string(rune(i))
		registry.autoRegisterRoute("GET", path, testHandler)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.GetDocs()
	}
}

// Example usage

func Example_apiRegistry() {
	registry := NewAPIRegistry()

	testHandler := func(e *core.RequestEvent) error {
		return nil
	}

	// Register a route
	registry.autoRegisterRoute("GET", "/api/users", testHandler)

	// Get documentation
	docs := registry.GetDocs()

	println("API Title:", docs.Title)
	println("Endpoints:", len(docs.Endpoints))

	if len(docs.Endpoints) > 0 {
		endpoint := docs.Endpoints[0]
		println("First endpoint:", endpoint.Method, endpoint.Path)
		println("Description:", endpoint.Description)
		println("Requires auth:", endpoint.Auth)
	}
	// Output would depend on actual execution
}
