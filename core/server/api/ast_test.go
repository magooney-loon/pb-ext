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
// Edge Cases: Order, Circular References, Self-References
// =============================================================================

func TestStructOrderIndependence(t *testing.T) {
	// Test that structs can reference types declared later in the file
	content := `package main

// API_SOURCE
type User struct {
	ID       string ` + "`json:\"id\"`" + `
	Settings *UserSettings ` + "`json:\"settings,omitempty\"`" + `
}

type UserSettings struct {
	Theme         string ` + "`json:\"theme\"`" + `
	Notifications bool   ` + "`json:\"notifications\"`" + `
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Verify both structs are parsed
	userStruct, exists := parser.GetStructByName("User")
	if !exists || userStruct == nil {
		t.Fatal("Expected User struct to be found")
	}

	settingsStruct, exists := parser.GetStructByName("UserSettings")
	if !exists || settingsStruct == nil {
		t.Fatal("Expected UserSettings struct to be found")
	}

	// Verify JSONSchema is generated for both
	if userStruct.JSONSchema == nil {
		t.Error("Expected User JSONSchema to be generated")
	}

	if settingsStruct.JSONSchema == nil {
		t.Error("Expected UserSettings JSONSchema to be generated")
	}

	// Verify User schema references UserSettings correctly
	if userStruct.JSONSchema.Properties == nil {
		t.Fatal("Expected User schema to have properties")
	}

	settingsFieldSchema, ok := userStruct.JSONSchema.Properties["settings"]
	if !ok {
		t.Error("Expected User schema to have 'settings' property")
	} else {
		// Should use $ref for nested types (2nd level)
		if settingsFieldSchema.Ref == "" {
			t.Error("Expected settings field to use $ref for nested type, but got inline schema")
		}
		if settingsFieldSchema.Ref != "#/components/schemas/UserSettings" {
			t.Errorf("Expected $ref to be '#/components/schemas/UserSettings', got '%s'", settingsFieldSchema.Ref)
		}
	}

	// Verify that if we generate a schema for User at endpoint level (inline=true), it's inline
	endpointSchema := parser.generateSchemaFromType("User", true)
	if endpointSchema == nil {
		t.Fatal("Expected endpoint schema to be generated")
	}
	// Endpoint-level schema should be inline (not $ref)
	if endpointSchema.Ref != "" {
		t.Error("Expected endpoint-level schema to be inline, not $ref")
	}
	if endpointSchema.Type != "object" {
		t.Errorf("Expected endpoint schema type to be 'object', got '%s'", endpointSchema.Type)
	}
	if endpointSchema.Properties == nil {
		t.Fatal("Expected endpoint schema to have properties")
	}
}

func TestCircularReferences(t *testing.T) {
	// Test circular references: A references B, B references A
	content := `package main

// API_SOURCE
type NodeA struct {
	ID    string ` + "`json:\"id\"`" + `
	Child *NodeB ` + "`json:\"child,omitempty\"`" + `
}

type NodeB struct {
	ID    string ` + "`json:\"id\"`" + `
	Child *NodeA ` + "`json:\"child,omitempty\"`" + `
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Verify both structs are parsed
	nodeA, exists := parser.GetStructByName("NodeA")
	if !exists || nodeA == nil {
		t.Fatal("Expected NodeA struct to be found")
	}

	nodeB, exists := parser.GetStructByName("NodeB")
	if !exists || nodeB == nil {
		t.Fatal("Expected NodeB struct to be found")
	}

	// Verify JSONSchema is generated for both
	if nodeA.JSONSchema == nil {
		t.Error("Expected NodeA JSONSchema to be generated")
	}

	if nodeB.JSONSchema == nil {
		t.Error("Expected NodeB JSONSchema to be generated")
	}

	// Verify circular references use $ref
	if nodeA.JSONSchema.Properties == nil {
		t.Fatal("Expected NodeA schema to have properties")
	}

	childFieldSchema, ok := nodeA.JSONSchema.Properties["child"]
	if !ok {
		t.Error("Expected NodeA schema to have 'child' property")
	} else {
		if childFieldSchema.Ref == "" {
			t.Error("Expected child field to use $ref for circular reference")
		}
		if childFieldSchema.Ref != "#/components/schemas/NodeB" {
			t.Errorf("Expected $ref to be '#/components/schemas/NodeB', got '%s'", childFieldSchema.Ref)
		}
	}
}

func TestSelfReference(t *testing.T) {
	// Test self-referencing structures (recursive types)
	content := `package main

// API_SOURCE
type TreeNode struct {
	Value    string     ` + "`json:\"value\"`" + `
	Children []TreeNode ` + "`json:\"children,omitempty\"`" + `
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Verify struct is parsed
	treeNode, exists := parser.GetStructByName("TreeNode")
	if !exists || treeNode == nil {
		t.Fatal("Expected TreeNode struct to be found")
	}

	// Verify JSONSchema is generated
	if treeNode.JSONSchema == nil {
		t.Error("Expected TreeNode JSONSchema to be generated")
	}

	// Verify self-reference in array
	if treeNode.JSONSchema.Properties == nil {
		t.Fatal("Expected TreeNode schema to have properties")
	}

	childrenFieldSchema, ok := treeNode.JSONSchema.Properties["children"]
	if !ok {
		t.Error("Expected TreeNode schema to have 'children' property")
	} else {
		if childrenFieldSchema.Type != "array" {
			t.Errorf("Expected children field to be array, got '%s'", childrenFieldSchema.Type)
		}
		if childrenFieldSchema.Items == nil {
			t.Error("Expected children array to have items schema")
		} else {
			// Should use $ref for self-reference
			if childrenFieldSchema.Items.Ref == "" {
				t.Error("Expected children items to use $ref for self-reference")
			}
			if childrenFieldSchema.Items.Ref != "#/components/schemas/TreeNode" {
				t.Errorf("Expected $ref to be '#/components/schemas/TreeNode', got '%s'", childrenFieldSchema.Items.Ref)
			}
		}
	}
}

func TestSchemaGenerationWithNilCheck(t *testing.T) {
	// Test that nil JSONSchema is handled gracefully
	content := `package main

// API_SOURCE
type SimpleStruct struct {
	Name string ` + "`json:\"name\"`" + `
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Manually set JSONSchema to nil to test nil check
	simpleStruct, exists := parser.GetStructByName("SimpleStruct")
	if !exists || simpleStruct == nil {
		t.Fatal("Expected SimpleStruct to be found")
	}

	// Temporarily set to nil to test fallback
	originalSchema := simpleStruct.JSONSchema
	simpleStruct.JSONSchema = nil

	// Generate schema for a field that references this struct (inline=false, should use $ref)
	schema := parser.generateSchemaFromType("SimpleStruct", false)
	if schema == nil {
		t.Fatal("Expected schema to be generated even when struct JSONSchema is nil")
	}

	// Should return $ref as fallback
	if schema.Ref == "" {
		t.Error("Expected $ref fallback when JSONSchema is nil")
	}
	if schema.Ref != "#/components/schemas/SimpleStruct" {
		t.Errorf("Expected $ref to be '#/components/schemas/SimpleStruct', got '%s'", schema.Ref)
	}

	// Test inline=true (should also return $ref when JSONSchema is nil)
	inlineSchema := parser.generateSchemaFromType("SimpleStruct", true)
	if inlineSchema == nil {
		t.Fatal("Expected inline schema to be generated even when struct JSONSchema is nil")
	}
	if inlineSchema.Ref == "" {
		t.Error("Expected $ref fallback for inline schema when JSONSchema is nil")
	}

	// Restore original schema
	simpleStruct.JSONSchema = originalSchema
}

func TestInlineSchemaForEndpoints(t *testing.T) {
	// Test that endpoint request/response schemas are inline, not $ref
	content := `package main

// API_SOURCE
type SearchRequest struct {
	Query string ` + "`json:\"query\"`" + `
	Limit int    ` + "`json:\"limit\"`" + `
}

type ExportData struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type SearchResponse struct {
	Products []ExportData ` + "`json:\"products\"`" + `
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Test that endpoint-level schema (inline=true) returns full schema
	requestSchema := parser.generateSchemaFromType("SearchRequest", true)
	if requestSchema == nil {
		t.Fatal("Expected request schema to be generated")
	}

	// Should be inline (not $ref)
	if requestSchema.Ref != "" {
		t.Error("Expected endpoint request schema to be inline, not $ref")
	}
	if requestSchema.Type != "object" {
		t.Errorf("Expected request schema type to be 'object', got '%s'", requestSchema.Type)
	}
	if requestSchema.Properties == nil {
		t.Fatal("Expected request schema to have properties")
	}

	// Test that nested field schema (inline=false) uses $ref
	responseSchema := parser.generateSchemaFromType("SearchResponse", true)
	if responseSchema == nil {
		t.Fatal("Expected response schema to be generated")
	}

	// Response schema itself should be inline
	if responseSchema.Ref != "" {
		t.Error("Expected endpoint response schema to be inline, not $ref")
	}
	if responseSchema.Properties == nil {
		t.Fatal("Expected response schema to have properties")
	}

	// But nested field (ExportData) should use $ref
	productsField := responseSchema.Properties["products"]
	if productsField == nil {
		t.Fatal("Expected response schema to have 'products' property")
	}
	if productsField.Type != "array" {
		t.Errorf("Expected products field to be array, got '%s'", productsField.Type)
	}
	if productsField.Items == nil {
		t.Fatal("Expected products array to have items")
	}

	// Items should use $ref for nested type (2nd level)
	if productsField.Items.Ref == "" {
		t.Error("Expected nested type (ExportData) to use $ref at 2nd level")
	}
	if productsField.Items.Ref != "#/components/schemas/ExportData" {
		t.Errorf("Expected $ref to be '#/components/schemas/ExportData', got '%s'", productsField.Items.Ref)
	}
}

func TestTypeAliasResolution(t *testing.T) {
	// Test that type aliases generate correct $ref instead of additionalProperties
	content := `package main

// API_SOURCE
type RealStruct struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type AliasType = RealStruct

type Response struct {
	Data []AliasType ` + "`json:\"data\"`" + `
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Verify that the alias was registered
	if _, exists := parser.typeAliases["AliasType"]; !exists {
		t.Error("Expected AliasType to be registered as an alias")
	}

	// Test that resolving the alias returns the real type
	resolved, isAlias := parser.resolveTypeAlias("AliasType", nil)
	if !isAlias {
		t.Error("Expected AliasType to be identified as an alias")
	}
	if resolved != "RealStruct" {
		t.Errorf("Expected resolved type to be 'RealStruct', got '%s'", resolved)
	}

	// Test schema generation for a field using the alias
	responseSchema := parser.generateSchemaFromType("Response", true)
	if responseSchema == nil {
		t.Fatal("Expected response schema to be generated")
	}

	// Check that the data field uses $ref to RealStruct, not additionalProperties
	dataField := responseSchema.Properties["data"]
	if dataField == nil {
		t.Fatal("Expected response schema to have 'data' property")
	}
	if dataField.Type != "array" {
		t.Errorf("Expected data field to be array, got '%s'", dataField.Type)
	}
	if dataField.Items == nil {
		t.Fatal("Expected data array to have items")
	}

	// Items should use $ref to RealStruct, not additionalProperties
	if dataField.Items.Ref == "" {
		t.Error("Expected nested type (AliasType -> RealStruct) to use $ref")
	}
	if dataField.Items.Ref != "#/components/schemas/RealStruct" {
		t.Errorf("Expected $ref to be '#/components/schemas/RealStruct', got '%s'", dataField.Items.Ref)
	}
	if dataField.Items.AdditionalProperties == true {
		t.Error("Expected items to NOT have additionalProperties: true")
	}
}

func TestTypeAliasWithQualifiedType(t *testing.T) {
	// Test alias towards a qualified type (e.g., searchresult.ExportData)
	content := `package main

// API_SOURCE
type ExportData struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type AliasExportData = searchresult.ExportData

type Response struct {
	Products []AliasExportData ` + "`json:\"products\"`" + `
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Verify that the alias was registered with the qualified type
	if realType, exists := parser.typeAliases["AliasExportData"]; !exists {
		t.Error("Expected AliasExportData to be registered as an alias")
	} else if realType != "searchresult.ExportData" {
		t.Errorf("Expected alias to point to 'searchresult.ExportData', got '%s'", realType)
	}

	// Test that resolving the alias handles qualified types
	resolved, isAlias := parser.resolveTypeAlias("AliasExportData", nil)
	if !isAlias {
		t.Error("Expected AliasExportData to be identified as an alias")
	}
	// The resolved type should extract the simple name if the struct is registered locally
	// Since ExportData is registered, it should resolve to "ExportData"
	if resolved != "ExportData" && resolved != "searchresult.ExportData" {
		t.Errorf("Expected resolved type to be 'ExportData' or 'searchresult.ExportData', got '%s'", resolved)
	}

	// Test schema generation
	responseSchema := parser.generateSchemaFromType("Response", true)
	if responseSchema == nil {
		t.Fatal("Expected response schema to be generated")
	}

	productsField := responseSchema.Properties["products"]
	if productsField == nil {
		t.Fatal("Expected response schema to have 'products' property")
	}
	if productsField.Items == nil {
		t.Fatal("Expected products array to have items")
	}

	// Should use $ref to ExportData (the locally registered struct)
	if productsField.Items.Ref == "" {
		t.Error("Expected nested type to use $ref")
	}
	if productsField.Items.Ref != "#/components/schemas/ExportData" {
		t.Errorf("Expected $ref to be '#/components/schemas/ExportData', got '%s'", productsField.Items.Ref)
	}
	if productsField.Items.AdditionalProperties == true {
		t.Error("Expected items to NOT have additionalProperties: true")
	}
}

func TestRecursiveTypeAlias(t *testing.T) {
	// Test that alias chains are resolved recursively
	content := `package main

// API_SOURCE
type RealStruct struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type FirstAlias = RealStruct
type SecondAlias = FirstAlias
type ThirdAlias = SecondAlias

type Response struct {
	Data []ThirdAlias ` + "`json:\"data\"`" + `
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Test recursive resolution
	resolved, isAlias := parser.resolveTypeAlias("ThirdAlias", nil)
	if !isAlias {
		t.Error("Expected ThirdAlias to be identified as an alias")
	}
	if resolved != "RealStruct" {
		t.Errorf("Expected resolved type to be 'RealStruct' after recursive resolution, got '%s'", resolved)
	}

	// Test that circular references don't cause infinite loops
	// (This is handled by the visited map in resolveTypeAlias)
	visited := make(map[string]bool)
	resolved2, _ := parser.resolveTypeAlias("ThirdAlias", visited)
	if resolved2 != "RealStruct" {
		t.Errorf("Expected resolved type to be 'RealStruct', got '%s'", resolved2)
	}

	// Test schema generation
	responseSchema := parser.generateSchemaFromType("Response", true)
	if responseSchema == nil {
		t.Fatal("Expected response schema to be generated")
	}

	dataField := responseSchema.Properties["data"]
	if dataField == nil {
		t.Fatal("Expected response schema to have 'data' property")
	}
	if dataField.Items == nil {
		t.Fatal("Expected data array to have items")
	}

	// Should resolve to RealStruct through the alias chain
	if dataField.Items.Ref != "#/components/schemas/RealStruct" {
		t.Errorf("Expected $ref to be '#/components/schemas/RealStruct' after recursive resolution, got '%s'", dataField.Items.Ref)
	}
}

func TestMapTypeSchemaGeneration(t *testing.T) {
	// Test that map types generate correct schemas with additionalProperties containing the value type
	content := `package main

// API_SOURCE
type SearchResult struct {
	Codes  []string           ` + "`json:\"codes\"`" + `
	Scores map[string]float64 ` + "`json:\"scores\"`" + `
	Tags   map[string]string  ` + "`json:\"tags\"`" + `
	Counts map[string]int     ` + "`json:\"counts\"`" + `
	Flags  map[string]bool    ` + "`json:\"flags\"`" + `
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	// Generate schema for SearchResult
	resultSchema := parser.generateSchemaFromType("SearchResult", true)
	if resultSchema == nil {
		t.Fatal("Expected SearchResult schema to be generated")
	}

	// Test Scores field: map[string]float64
	scoresField := resultSchema.Properties["scores"]
	if scoresField == nil {
		t.Fatal("Expected SearchResult schema to have 'scores' property")
	}
	if scoresField.Type != "object" {
		t.Errorf("Expected scores field type to be 'object', got '%s'", scoresField.Type)
	}
	if scoresField.AdditionalProperties == nil {
		t.Fatal("Expected scores field to have additionalProperties")
	}
	// Check if additionalProperties is a schema (not just true)
	if additionalProps, ok := scoresField.AdditionalProperties.(*OpenAPISchema); ok {
		if additionalProps.Type != "number" {
			t.Errorf("Expected scores additionalProperties type to be 'number', got '%s'", additionalProps.Type)
		}
	} else if scoresField.AdditionalProperties != true {
		t.Errorf("Expected scores additionalProperties to be a schema with type 'number', got %v", scoresField.AdditionalProperties)
	}

	// Test Tags field: map[string]string
	tagsField := resultSchema.Properties["tags"]
	if tagsField == nil {
		t.Fatal("Expected SearchResult schema to have 'tags' property")
	}
	if tagsField.Type != "object" {
		t.Errorf("Expected tags field type to be 'object', got '%s'", tagsField.Type)
	}
	if additionalProps, ok := tagsField.AdditionalProperties.(*OpenAPISchema); ok {
		if additionalProps.Type != "string" {
			t.Errorf("Expected tags additionalProperties type to be 'string', got '%s'", additionalProps.Type)
		}
	} else {
		t.Errorf("Expected tags additionalProperties to be a schema with type 'string', got %v", tagsField.AdditionalProperties)
	}

	// Test Counts field: map[string]int
	countsField := resultSchema.Properties["counts"]
	if countsField == nil {
		t.Fatal("Expected SearchResult schema to have 'counts' property")
	}
	if countsField.Type != "object" {
		t.Errorf("Expected counts field type to be 'object', got '%s'", countsField.Type)
	}
	if additionalProps, ok := countsField.AdditionalProperties.(*OpenAPISchema); ok {
		if additionalProps.Type != "integer" {
			t.Errorf("Expected counts additionalProperties type to be 'integer', got '%s'", additionalProps.Type)
		}
	} else {
		t.Errorf("Expected counts additionalProperties to be a schema with type 'integer', got %v", countsField.AdditionalProperties)
	}

	// Test Flags field: map[string]bool
	flagsField := resultSchema.Properties["flags"]
	if flagsField == nil {
		t.Fatal("Expected SearchResult schema to have 'flags' property")
	}
	if flagsField.Type != "object" {
		t.Errorf("Expected flags field type to be 'object', got '%s'", flagsField.Type)
	}
	if additionalProps, ok := flagsField.AdditionalProperties.(*OpenAPISchema); ok {
		if additionalProps.Type != "boolean" {
			t.Errorf("Expected flags additionalProperties type to be 'boolean', got '%s'", additionalProps.Type)
		}
	} else {
		t.Errorf("Expected flags additionalProperties to be a schema with type 'boolean', got %v", flagsField.AdditionalProperties)
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
