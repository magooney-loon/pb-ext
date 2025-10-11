package api

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// Test Setup and Mock Data
// =============================================================================

// createTestFile creates a temporary Go file with the given content
func createTestFile(t *testing.T, filename, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, filename)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	return filePath
}

// Sample Go file content with API_SOURCE directive
const testGoFileContent = `package main

// API_SOURCE
// This file contains API handlers

import (
	"github.com/pocketbase/pocketbase/core"
)

// User represents a user in the system
type User struct {
	ID       string ` + "`json:\"id\"`" + `
	Name     string ` + "`json:\"name\"`" + `
	Email    string ` + "`json:\"email\"`" + `
	Active   bool   ` + "`json:\"active\"`" + `
	Settings *UserSettings ` + "`json:\"settings,omitempty\"`" + `
}

// UserSettings represents user preferences
type UserSettings struct {
	Theme       string ` + "`json:\"theme\"`" + `
	Notifications bool ` + "`json:\"notifications\"`" + `
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Name  string ` + "`json:\"name\" validate:\"required\"`" + `
	Email string ` + "`json:\"email\" validate:\"required,email\"`" + `
}

// API_DESC Get all users from the system
// API_TAGS users,list,public
func getUsersHandler(c *core.RequestEvent) error {
	return nil
}

// API_DESC Create a new user account
// API_TAGS users,create,auth
func createUserHandler(c *core.RequestEvent) error {
	var req CreateUserRequest
	return nil
}

// Regular function without API comments
func helperFunction() {
	// This should not be picked up as a handler
}

// API_DESC Update user profile information
// API_TAGS users,update,auth
func updateUserHandler(c *core.RequestEvent) error {
	return nil
}
`

// Sample Go file without API_SOURCE directive
const testGoFileWithoutDirective = `package main

import (
	"github.com/pocketbase/pocketbase/core"
)

func regularHandler(c *core.RequestEvent) error {
	return nil
}
`

// =============================================================================
// AST Parser Tests
// =============================================================================

func TestNewASTParser(t *testing.T) {
	parser := NewASTParser()

	if parser == nil {
		t.Fatal("Expected non-nil AST parser")
	}

	if parser.fileSet == nil {
		t.Error("Expected fileSet to be initialized")
	}

	if parser.structs == nil {
		t.Error("Expected structs map to be initialized")
	}

	if parser.handlers == nil {
		t.Error("Expected handlers map to be initialized")
	}

	if parser.pocketbasePatterns == nil {
		t.Error("Expected pocketbasePatterns to be initialized")
	}

	if parser.logger == nil {
		t.Error("Expected logger to be initialized")
	}
}

func TestParseFile(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectError    bool
		expectHandlers int
		expectStructs  int
	}{
		{
			name:           "Valid file with API_SOURCE",
			content:        testGoFileContent,
			expectError:    false,
			expectHandlers: 3, // getUsersHandler, createUserHandler, updateUserHandler
			expectStructs:  3, // User, UserSettings, CreateUserRequest
		},
		{
			name:           "File without API_SOURCE",
			content:        testGoFileWithoutDirective,
			expectError:    false,
			expectHandlers: 0,
			expectStructs:  0,
		},
		{
			name:        "Invalid Go syntax",
			content:     "package main\n\nfunc invalid syntax {",
			expectError: true,
		},
		{
			name:        "Empty file",
			content:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewASTParser()
			filePath := createTestFile(t, "test.go", tt.content)

			err := parser.ParseFile(filePath)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError {
				handlers := parser.GetAllHandlers()
				if len(handlers) != tt.expectHandlers {
					t.Errorf("Expected %d handlers, got %d", tt.expectHandlers, len(handlers))
				}

				structs := parser.GetAllStructs()
				if len(structs) != tt.expectStructs {
					t.Errorf("Expected %d structs, got %d", tt.expectStructs, len(structs))
				}
			}
		})
	}
}

func TestDiscoverSourceFiles(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create subdirectories
	apiDir := filepath.Join(tmpDir, "api")
	handlersDir := filepath.Join(tmpDir, "handlers")
	err := os.MkdirAll(apiDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create api directory: %v", err)
	}
	err = os.MkdirAll(handlersDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create handlers directory: %v", err)
	}

	// Create files with API_SOURCE directive
	apiFile := filepath.Join(apiDir, "routes.go")
	err = os.WriteFile(apiFile, []byte(testGoFileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create API file: %v", err)
	}

	// Create file without API_SOURCE directive
	regularFile := filepath.Join(handlersDir, "utils.go")
	err = os.WriteFile(regularFile, []byte(testGoFileWithoutDirective), 0644)
	if err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Change to temp directory for discovery
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	err = parser.DiscoverSourceFiles()

	if err != nil {
		t.Errorf("Expected no error during discovery, got: %v", err)
	}

	// Should have found handlers from the API_SOURCE file only
	handlers := parser.GetAllHandlers()
	if len(handlers) != 3 {
		t.Errorf("Expected 3 handlers from discovery, got %d", len(handlers))
	}

	structs := parser.GetAllStructs()
	if len(structs) != 3 {
		t.Errorf("Expected 3 structs from discovery, got %d", len(structs))
	}
}

func TestGetStructByName(t *testing.T) {
	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", testGoFileContent)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	tests := []struct {
		name       string
		structName string
		expectNil  bool
	}{
		{
			name:       "Existing struct",
			structName: "User",
			expectNil:  false,
		},
		{
			name:       "Existing nested struct",
			structName: "UserSettings",
			expectNil:  false,
		},
		{
			name:       "Non-existent struct",
			structName: "NonExistent",
			expectNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, exists := parser.GetStructByName(tt.structName)

			if tt.expectNil && result != nil {
				t.Error("Expected nil result but got struct")
			}
			if !tt.expectNil && result == nil {
				t.Error("Expected struct but got nil")
			}
			if !tt.expectNil && !exists {
				t.Error("Expected struct to exist but got false")
			}
			if !tt.expectNil && result != nil && result.Name != tt.structName {
				t.Errorf("Expected struct name %s, got %s", tt.structName, result.Name)
			}
		})
	}
}

func TestGetHandlerByName(t *testing.T) {
	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", testGoFileContent)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	tests := []struct {
		name         string
		handlerName  string
		expectNil    bool
		expectedDesc string
		expectedTags []string
	}{
		{
			name:         "Existing handler with description",
			handlerName:  "getUsersHandler",
			expectNil:    false,
			expectedDesc: "Get all users from the system",
			expectedTags: []string{"users", "list", "public"},
		},
		{
			name:         "Existing handler with create description",
			handlerName:  "createUserHandler",
			expectNil:    false,
			expectedDesc: "Create a new user account",
			expectedTags: []string{"users", "create", "auth"},
		},
		{
			name:        "Non-existent handler",
			handlerName: "nonExistentHandler",
			expectNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, exists := parser.GetHandlerByName(tt.handlerName)

			if tt.expectNil && result != nil {
				t.Error("Expected nil result but got handler")
			}
			if !tt.expectNil && result == nil {
				t.Error("Expected handler but got nil")
			}
			if !tt.expectNil && !exists {
				t.Error("Expected handler to exist but got false")
			}

			if !tt.expectNil && result != nil {
				if result.APIDescription != tt.expectedDesc {
					t.Errorf("Expected description %s, got %s", tt.expectedDesc, result.APIDescription)
				}

				if len(result.APITags) != len(tt.expectedTags) {
					t.Errorf("Expected %d tags, got %d", len(tt.expectedTags), len(result.APITags))
				}

				for i, tag := range tt.expectedTags {
					if i < len(result.APITags) && result.APITags[i] != tag {
						t.Errorf("Expected tag %s at position %d, got %s", tag, i, result.APITags[i])
					}
				}
			}
		})
	}
}

func TestGetAllStructs(t *testing.T) {
	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", testGoFileContent)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	structs := parser.GetAllStructs()

	expectedStructs := []string{"User", "UserSettings", "CreateUserRequest"}
	if len(structs) != len(expectedStructs) {
		t.Errorf("Expected %d structs, got %d", len(expectedStructs), len(structs))
	}

	structNames := make(map[string]bool)
	for _, s := range structs {
		structNames[s.Name] = true
	}

	for _, expected := range expectedStructs {
		if !structNames[expected] {
			t.Errorf("Expected struct %s not found", expected)
		}
	}
}

func TestGetAllHandlers(t *testing.T) {
	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", testGoFileContent)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	handlers := parser.GetAllHandlers()

	expectedHandlers := []string{"getUsersHandler", "createUserHandler", "updateUserHandler"}
	if len(handlers) != len(expectedHandlers) {
		t.Errorf("Expected %d handlers, got %d", len(expectedHandlers), len(handlers))
	}

	handlerNames := make(map[string]bool)
	for _, h := range handlers {
		handlerNames[h.Name] = true
	}

	for _, expected := range expectedHandlers {
		if !handlerNames[expected] {
			t.Errorf("Expected handler %s not found", expected)
		}
	}
}

func TestGetParseErrors(t *testing.T) {
	parser := NewASTParser()

	// Initially should have no errors
	errors := parser.GetParseErrors()
	if len(errors) != 0 {
		t.Errorf("Expected no initial parse errors, got %d", len(errors))
	}

	// Parse invalid file to generate errors
	invalidFile := createTestFile(t, "invalid.go", "invalid go syntax")
	parser.ParseFile(invalidFile) // This should generate errors

	errors = parser.GetParseErrors()
	if len(errors) == 0 {
		t.Error("Expected parse errors after parsing invalid file")
	}
}

func TestClearCache(t *testing.T) {
	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", testGoFileContent)

	// Parse file to populate cache
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Verify data exists
	if len(parser.GetAllHandlers()) == 0 {
		t.Error("Expected handlers before clearing cache")
	}
	if len(parser.GetAllStructs()) == 0 {
		t.Error("Expected structs before clearing cache")
	}

	// Clear cache
	parser.ClearCache()

	// Verify cache is cleared
	if len(parser.GetAllHandlers()) != 0 {
		t.Error("Expected no handlers after clearing cache")
	}
	if len(parser.GetAllStructs()) != 0 {
		t.Error("Expected no structs after clearing cache")
	}
	if len(parser.GetParseErrors()) != 0 {
		t.Error("Expected no parse errors after clearing cache")
	}
}

func TestEnhanceEndpoint(t *testing.T) {
	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", testGoFileContent)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	tests := []struct {
		name           string
		endpoint       *APIEndpoint
		expectEnhanced bool
		expectedDesc   string
		expectedTags   []string
	}{
		{
			name: "Enhance with existing handler",
			endpoint: &APIEndpoint{
				Method:  "GET",
				Path:    "/users",
				Handler: "getUsersHandler",
			},
			expectEnhanced: true,
			expectedDesc:   "Get all users from the system",
			expectedTags:   []string{"users", "list", "public"},
		},
		{
			name: "No enhancement for non-existent handler",
			endpoint: &APIEndpoint{
				Method:      "POST",
				Path:        "/test",
				Handler:     "nonExistentHandler",
				Description: "Original description",
			},
			expectEnhanced: false,
			expectedDesc:   "Original description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser.EnhanceEndpoint(tt.endpoint)

			if tt.expectEnhanced {
				if tt.endpoint.Description != tt.expectedDesc {
					t.Errorf("Expected description %s, got %s", tt.expectedDesc, tt.endpoint.Description)
				}
				if len(tt.endpoint.Tags) != len(tt.expectedTags) {
					t.Errorf("Expected %d tags, got %d", len(tt.expectedTags), len(tt.endpoint.Tags))
				}
			} else {
				if tt.endpoint.Description != tt.expectedDesc {
					t.Errorf("Expected description to remain %s, got %s", tt.expectedDesc, tt.endpoint.Description)
				}
			}
		})
	}
}

func TestGetStructsForFinding(t *testing.T) {
	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", testGoFileContent)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	structs := parser.GetStructsForFinding()

	// Should return all structs in a format suitable for finding/searching
	if len(structs) == 0 {
		t.Error("Expected structs for finding, got none")
	}

	// Verify the structs contain expected information
	found := false
	for _, s := range structs {
		if s.Name == "User" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find User struct in finding results")
	}
}

// =============================================================================
// Performance Benchmarks
// =============================================================================

func BenchmarkParseFile(b *testing.B) {
	content := testGoFileContent

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewASTParser()
		filePath := createTestFileForBenchmark(b, "bench.go", content)
		parser.ParseFile(filePath)
	}
}

func BenchmarkGetHandlerByName(b *testing.B) {
	parser := NewASTParser()
	filePath := createTestFileForBenchmark(b, "bench.go", testGoFileContent)
	parser.ParseFile(filePath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.GetHandlerByName("getUsersHandler")
	}
}

func BenchmarkGetStructByName(b *testing.B) {
	parser := NewASTParser()
	filePath := createTestFileForBenchmark(b, "bench.go", testGoFileContent)
	parser.ParseFile(filePath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.GetStructByName("User")
	}
}

func BenchmarkEnhanceEndpoint(b *testing.B) {
	parser := NewASTParser()
	filePath := createTestFileForBenchmark(b, "bench.go", testGoFileContent)
	parser.ParseFile(filePath)

	endpoint := &APIEndpoint{
		Method:  "GET",
		Path:    "/users",
		Handler: "getUsersHandler",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset endpoint for each iteration
		endpoint.Description = ""
		endpoint.Tags = nil
		parser.EnhanceEndpoint(endpoint)
	}
}

// =============================================================================
// Helper Functions for Benchmarks
// =============================================================================

func createTestFileForBenchmark(b *testing.B, filename, content string) string {
	b.Helper()

	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, filename)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	return filePath
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestASTParserIntegration(t *testing.T) {
	// Test full workflow: create parser, discover files, enhance endpoints
	tmpDir := t.TempDir()

	// Create a realistic API file structure
	apiFile := filepath.Join(tmpDir, "api.go")
	err := os.WriteFile(apiFile, []byte(testGoFileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create API file: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	// Create and initialize parser
	parser := NewASTParser()
	err = parser.DiscoverSourceFiles()
	if err != nil {
		t.Fatalf("Failed to discover source files: %v", err)
	}

	// Test endpoint enhancement
	endpoint := &APIEndpoint{
		Method:  "POST",
		Path:    "/users",
		Handler: "createUserHandler",
	}

	parser.EnhanceEndpoint(endpoint)

	if endpoint.Description != "Create a new user account" {
		t.Errorf("Expected enhanced description, got: %s", endpoint.Description)
	}

	expectedTags := []string{"users", "create", "auth"}
	if len(endpoint.Tags) != len(expectedTags) {
		t.Errorf("Expected %d tags, got %d", len(expectedTags), len(endpoint.Tags))
	}

	// Test struct extraction for schema generation
	userStruct, exists := parser.GetStructByName("User")
	if userStruct == nil || !exists {
		t.Fatal("Expected User struct to be found")
	}

	if userStruct.Name != "User" {
		t.Errorf("Expected struct name User, got %s", userStruct.Name)
	}
}

// =============================================================================
// Edge Cases and Error Handling
// =============================================================================

func TestASTParserEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		content string
		test    func(t *testing.T, parser *ASTParser)
	}{
		{
			name: "File with only comments",
			content: `package main
// API_SOURCE
// Only comments here
`,
			test: func(t *testing.T, parser *ASTParser) {
				if len(parser.GetAllHandlers()) != 0 {
					t.Error("Expected no handlers in comment-only file")
				}
			},
		},
		{
			name: "Handlers without API comments",
			content: `package main
// API_SOURCE
import "github.com/pocketbase/pocketbase/core"

func handlerWithoutComments(c *core.RequestEvent) error {
	return nil
}
`,
			test: func(t *testing.T, parser *ASTParser) {
				handlers := parser.GetAllHandlers()
				if len(handlers) != 1 {
					t.Errorf("Expected 1 handler, got %d", len(handlers))
				}
				// Get the first (and only) handler from the map
				var handler *ASTHandlerInfo
				for _, h := range handlers {
					handler = h
					break
				}
				if handler != nil && handler.APIDescription != "" {
					t.Errorf("Expected empty description, got: %s", handler.APIDescription)
				}
			},
		},
		{
			name: "Malformed API comments",
			content: `package main
// API_SOURCE
import "github.com/pocketbase/pocketbase/core"

// API_DESC
// API_TAGS
func handlerWithMalformedComments(c *core.RequestEvent) error {
	return nil
}
`,
			test: func(t *testing.T, parser *ASTParser) {
				handlers := parser.GetAllHandlers()
				if len(handlers) != 1 {
					t.Errorf("Expected 1 handler, got %d", len(handlers))
				}
				// Should handle malformed comments gracefully
				// Get the first handler to verify it exists
				var handler *ASTHandlerInfo
				for _, h := range handlers {
					handler = h
					break
				}
				if handler == nil {
					t.Error("Expected to find a handler")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewASTParser()
			filePath := createTestFile(t, "test.go", tt.content)

			err := parser.ParseFile(filePath)
			if err != nil {
				t.Fatalf("Failed to parse file: %v", err)
			}

			tt.test(t, parser)
		})
	}
}

// =============================================================================
// Examples
// =============================================================================

func ExampleASTParser() {
	// Create a new AST parser
	parser := NewASTParser()

	// Parse a specific file
	err := parser.ParseFile("handlers.go")
	if err != nil {
		panic(err)
	}

	// Get all discovered handlers
	handlers := parser.GetAllHandlers()
	for _, handler := range handlers {
		println("Handler:", handler.Name, "Description:", handler.APIDescription)
	}

	// Get a specific struct
	userStruct, exists := parser.GetStructByName("User")
	if userStruct != nil && exists {
		println("Found User struct with", len(userStruct.Fields), "fields")
	}

	// Enhance an endpoint with AST information
	endpoint := &APIEndpoint{
		Method:  "GET",
		Path:    "/users",
		Handler: "getUsersHandler",
	}
	parser.EnhanceEndpoint(endpoint)
	println("Enhanced endpoint description:", endpoint.Description)
}

func ExampleASTParser_DiscoverSourceFiles() {
	// Create parser and auto-discover all API source files
	parser := NewASTParser()

	err := parser.DiscoverSourceFiles()
	if err != nil {
		panic(err)
	}

	// All files with // API_SOURCE directive have been parsed
	handlers := parser.GetAllHandlers()
	structs := parser.GetAllStructs()

	println("Discovered", len(handlers), "handlers and", len(structs), "structs")
}
