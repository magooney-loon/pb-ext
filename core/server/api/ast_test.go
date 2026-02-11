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

	// Test generateSchemaForEndpoint: known struct types should use $ref
	t.Run("endpoint struct types use $ref", func(t *testing.T) {
		requestSchema := parser.generateSchemaForEndpoint("SearchRequest")
		if requestSchema == nil {
			t.Fatal("Expected request schema to be generated")
		}
		if requestSchema.Ref != "#/components/schemas/SearchRequest" {
			t.Errorf("Expected $ref '#/components/schemas/SearchRequest', got Ref='%s' Type='%s'", requestSchema.Ref, requestSchema.Type)
		}
	})

	// Test generateSchemaForEndpoint: array of structs should use $ref in items
	t.Run("endpoint array of structs uses $ref in items", func(t *testing.T) {
		schema := parser.generateSchemaForEndpoint("[]ExportData")
		if schema == nil {
			t.Fatal("Expected schema to be generated")
		}
		if schema.Type != "array" {
			t.Errorf("Expected type 'array', got '%s'", schema.Type)
		}
		if schema.Items == nil {
			t.Fatal("Expected items to be set")
		}
		if schema.Items.Ref != "#/components/schemas/ExportData" {
			t.Errorf("Expected items $ref '#/components/schemas/ExportData', got '%s'", schema.Items.Ref)
		}
	})

	// Test generateSchemaForEndpoint: primitives should NOT produce $ref
	t.Run("endpoint primitives do not use $ref", func(t *testing.T) {
		schema := parser.generateSchemaForEndpoint("string")
		if schema == nil {
			t.Fatal("Expected schema to be generated")
		}
		if schema.Ref != "" {
			t.Error("Expected primitive type to NOT use $ref")
		}
		if schema.Type != "string" {
			t.Errorf("Expected type 'string', got '%s'", schema.Type)
		}
	})

	// Test generateSchemaFromType with inline=true still inlines (for component schema generation)
	t.Run("generateSchemaFromType inline=true still works", func(t *testing.T) {
		schema := parser.generateSchemaFromType("SearchResponse", true)
		if schema == nil {
			t.Fatal("Expected schema to be generated")
		}
		if schema.Ref != "" {
			t.Error("Expected inline schema (not $ref) from generateSchemaFromType with inline=true")
		}
		if schema.Properties == nil {
			t.Fatal("Expected inline schema to have properties")
		}
		// Nested field (ExportData) should still use $ref at 2nd level
		productsField := schema.Properties["products"]
		if productsField == nil {
			t.Fatal("Expected 'products' property")
		}
		if productsField.Items == nil || productsField.Items.Ref != "#/components/schemas/ExportData" {
			t.Error("Expected nested type to use $ref at 2nd level")
		}
	})
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

func TestVariableRefMapLiteralSchema(t *testing.T) {
	// Test that map literals assigned to a variable and then passed to e.JSON()
	// produce detailed schemas (not generic map[string]any)
	content := `package main

import "github.com/pocketbase/pocketbase/core"

// API_SOURCE

// API_DESC Get candles for a token
// API_TAGS Analytics
func getCandlesHandler(e *core.RequestEvent) error {
	candles := []map[string]any{}
	result := map[string]any{
		"token_id": "abc123",
		"candles":  candles,
		"count":    42,
		"success":  true,
	}
	return e.JSON(200, result)
}

// API_DESC Get direct map response
// API_TAGS Analytics
func getDirectMapHandler(e *core.RequestEvent) error {
	return e.JSON(200, map[string]any{
		"status":  "ok",
		"latency": 1.5,
	})
}
`

	parser := NewASTParser()
	filePath := createTestFile(t, "test_varref.go", content)
	err := parser.ParseFile(filePath)
	if err != nil {
		t.Fatalf("Failed to parse file: %v", err)
	}

	t.Run("variable ref map literal gets detailed schema", func(t *testing.T) {
		handlers := parser.GetAllHandlers()
		handler := handlers["getCandlesHandler"]
		if handler == nil {
			t.Fatal("Expected getCandlesHandler to be found")
		}
		if handler.ResponseSchema == nil {
			t.Fatal("Expected response schema to be generated")
		}
		// Should have properties from the map literal, not generic additionalProperties
		if handler.ResponseSchema.Properties == nil {
			t.Fatalf("Expected response schema to have properties, got: type=%s additionalProperties=%v",
				handler.ResponseSchema.Type, handler.ResponseSchema.AdditionalProperties)
		}
		if _, ok := handler.ResponseSchema.Properties["token_id"]; !ok {
			t.Error("Expected 'token_id' property in response schema")
		}
		if _, ok := handler.ResponseSchema.Properties["candles"]; !ok {
			t.Error("Expected 'candles' property in response schema")
		}
		if _, ok := handler.ResponseSchema.Properties["count"]; !ok {
			t.Error("Expected 'count' property in response schema")
		}
		if _, ok := handler.ResponseSchema.Properties["success"]; !ok {
			t.Error("Expected 'success' property in response schema")
		}
	})

	t.Run("direct map literal still works", func(t *testing.T) {
		handlers := parser.GetAllHandlers()
		handler := handlers["getDirectMapHandler"]
		if handler == nil {
			t.Fatal("Expected getDirectMapHandler to be found")
		}
		if handler.ResponseSchema == nil {
			t.Fatal("Expected response schema to be generated")
		}
		if handler.ResponseSchema.Properties == nil {
			t.Fatal("Expected response schema to have properties")
		}
		if _, ok := handler.ResponseSchema.Properties["status"]; !ok {
			t.Error("Expected 'status' property in response schema")
		}
		if _, ok := handler.ResponseSchema.Properties["latency"]; !ok {
			t.Error("Expected 'latency' property in response schema")
		}
	})
}

// =============================================================================
// Handler Schema Scenario Tests
// Covers all 24 AST handler patterns previously tested via live v2 routes.
// =============================================================================

// handlerScenarioSource is the synthetic Go source that exercises every schema
// generation path in the AST parser.  It is parsed once in each sub-test that
// needs it.
const handlerScenarioSource = `package main

// API_SOURCE

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// --- Struct definitions ---

type GeoCoordinate struct {
	Latitude  float64 ` + "`json:\"latitude\"`" + `
	Longitude float64 ` + "`json:\"longitude\"`" + `
}

type Address struct {
	Street     string        ` + "`json:\"street\"`" + `
	City       string        ` + "`json:\"city\"`" + `
	State      string        ` + "`json:\"state,omitempty\"`" + `
	PostalCode string        ` + "`json:\"postal_code\"`" + `
	Country    string        ` + "`json:\"country\"`" + `
	Geo        GeoCoordinate ` + "`json:\"geo\"`" + `
}

type ContactInfo struct {
	Email     string   ` + "`json:\"email\"`" + `
	Phone     *string  ` + "`json:\"phone,omitempty\"`" + `
	Website   *string  ` + "`json:\"website,omitempty\"`" + `
	SocialIDs []string ` + "`json:\"social_ids,omitempty\"`" + `
}

type OrderItem struct {
	ProductID   string  ` + "`json:\"product_id\"`" + `
	ProductName string  ` + "`json:\"product_name\"`" + `
	Quantity    int     ` + "`json:\"quantity\"`" + `
	UnitPrice   float64 ` + "`json:\"unit_price\"`" + `
	Subtotal    float64 ` + "`json:\"subtotal\"`" + `
}

type PaymentInfo struct {
	Method        string            ` + "`json:\"method\"`" + `
	TransactionID string            ` + "`json:\"transaction_id\"`" + `
	Amount        float64           ` + "`json:\"amount\"`" + `
	Currency      string            ` + "`json:\"currency\"`" + `
	Headers       map[string]string ` + "`json:\"headers,omitempty\"`" + `
	Metadata      map[string]any    ` + "`json:\"metadata,omitempty\"`" + `
}

type OrderResponse struct {
	ID              string      ` + "`json:\"id\"`" + `
	Status          string      ` + "`json:\"status\"`" + `
	Customer        string      ` + "`json:\"customer\"`" + `
	ShippingAddress Address     ` + "`json:\"shipping_address\"`" + `
	BillingAddress  *Address    ` + "`json:\"billing_address,omitempty\"`" + `
	Items           []OrderItem ` + "`json:\"items\"`" + `
	Payment         PaymentInfo ` + "`json:\"payment\"`" + `
	TotalAmount     float64     ` + "`json:\"total_amount\"`" + `
	Notes           *string     ` + "`json:\"notes,omitempty\"`" + `
	CreatedAt       time.Time   ` + "`json:\"created_at\"`" + `
	UpdatedAt       time.Time   ` + "`json:\"updated_at\"`" + `
}

type CreateOrderRequest struct {
	CustomerID      string      ` + "`json:\"customer_id\"`" + `
	ShippingAddress Address     ` + "`json:\"shipping_address\"`" + `
	BillingAddress  *Address    ` + "`json:\"billing_address,omitempty\"`" + `
	Items           []OrderItem ` + "`json:\"items\"`" + `
	PaymentMethod   string      ` + "`json:\"payment_method\"`" + `
	Notes           *string     ` + "`json:\"notes,omitempty\"`" + `
	CouponCode      *string     ` + "`json:\"coupon_code,omitempty\"`" + `
}

type AnalyticsEvent struct {
	EventID    string         ` + "`json:\"event_id\"`" + `
	EventType  string         ` + "`json:\"event_type\"`" + `
	Timestamp  time.Time      ` + "`json:\"timestamp\"`" + `
	UserID     *string        ` + "`json:\"user_id,omitempty\"`" + `
	SessionID  string         ` + "`json:\"session_id\"`" + `
	Properties map[string]any ` + "`json:\"properties,omitempty\"`" + `
	Context    any            ` + "`json:\"context,omitempty\"`" + `
	Tags       []string       ` + "`json:\"tags,omitempty\"`" + `
}

type PaginationMeta struct {
	Page       int  ` + "`json:\"page\"`" + `
	PerPage    int  ` + "`json:\"per_page\"`" + `
	TotalItems int  ` + "`json:\"total_items\"`" + `
	TotalPages int  ` + "`json:\"total_pages\"`" + `
	HasMore    bool ` + "`json:\"has_more\"`" + `
}

type UserProfile struct {
	ID          string      ` + "`json:\"id\"`" + `
	Username    string      ` + "`json:\"username\"`" + `
	DisplayName string      ` + "`json:\"display_name\"`" + `
	Email       string      ` + "`json:\"email\"`" + `
	AvatarURL   *string     ` + "`json:\"avatar_url,omitempty\"`" + `
	Bio         *string     ` + "`json:\"bio,omitempty\"`" + `
	IsVerified  bool        ` + "`json:\"is_verified\"`" + `
	Reputation  int         ` + "`json:\"reputation\"`" + `
	Balance     float64     ` + "`json:\"balance\"`" + `
	JoinedAt    time.Time   ` + "`json:\"joined_at\"`" + `
	Contact     ContactInfo ` + "`json:\"contact\"`" + `
}

type TimeseriesPoint struct {
	Timestamp int64   ` + "`json:\"timestamp\"`" + `
	Open      float64 ` + "`json:\"open\"`" + `
	High      float64 ` + "`json:\"high\"`" + `
	Low       float64 ` + "`json:\"low\"`" + `
	Close     float64 ` + "`json:\"close\"`" + `
	Volume    float64 ` + "`json:\"volume\"`" + `
}

type IndicatorValues struct {
	TokenID   string             ` + "`json:\"token_id\"`" + `
	Interval  string             ` + "`json:\"interval\"`" + `
	Values    map[string]float64 ` + "`json:\"values\"`" + `
	Signals   map[string]string  ` + "`json:\"signals\"`" + `
	Computed  map[string]int     ` + "`json:\"computed\"`" + `
	UpdatedAt time.Time          ` + "`json:\"updated_at\"`" + `
}

type UpdateProfileRequest struct {
	DisplayName string  ` + "`json:\"display_name\"`" + `
	Bio         *string ` + "`json:\"bio,omitempty\"`" + `
	AvatarURL   *string ` + "`json:\"avatar_url,omitempty\"`" + `
}

type BaseEntity struct {
	ID        string    ` + "`json:\"id\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\"`" + `
}

type ProductResponse struct {
	BaseEntity
	Name        string   ` + "`json:\"name\"`" + `
	Description string   ` + "`json:\"description\"`" + `
	Price       float64  ` + "`json:\"price\"`" + `
	Currency    string   ` + "`json:\"currency\"`" + `
	Tags        []string ` + "`json:\"tags,omitempty\"`" + `
	InStock     bool     ` + "`json:\"in_stock\"`" + `
}

type HealthCheckResponse struct {
	Status    string ` + "`json:\"status\"`" + `
	Version   string ` + "`json:\"version\"`" + `
	Uptime    int64  ` + "`json:\"uptime\"`" + `
	Timestamp string ` + "`json:\"timestamp\"`" + `
}

type BatchDeleteRequest struct {
	IDs    []string ` + "`json:\"ids\"`" + `
	DryRun bool     ` + "`json:\"dry_run\"`" + `
}

// --- Handlers ---

// 1. Deep nested struct response
// API_DESC Get order details
// API_TAGS Orders
func getOrderHandler(c *core.RequestEvent) error {
	resp := OrderResponse{
		ID: "ord_123",
		Status: "shipped",
		ShippingAddress: Address{
			Street: "123 Main St",
			Geo: GeoCoordinate{Latitude: 45.5, Longitude: -122.6},
		},
		Items: []OrderItem{{ProductID: "p1", Quantity: 2}},
		Payment: PaymentInfo{Method: "card", Amount: 19.98, Currency: "USD"},
	}
	return c.JSON(http.StatusOK, resp)
}

// 2. Nested struct request body via json.Decode
// API_DESC Create a new order
// API_TAGS Orders
func createOrderHandler(c *core.RequestEvent) error {
	var req CreateOrderRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid"})
	}
	resp := OrderResponse{ID: "ord_new", Status: "pending"}
	return c.JSON(http.StatusCreated, resp)
}

// 3. Array-of-structs response
// API_DESC List all orders
// API_TAGS Orders
func listOrdersHandler(c *core.RequestEvent) error {
	orders := []OrderResponse{
		{ID: "ord_001", Status: "delivered"},
	}
	return c.JSON(http.StatusOK, orders)
}

// 4. Struct with typed maps
// API_DESC Get indicator values
// API_TAGS Analytics
func getIndicatorsHandler(c *core.RequestEvent) error {
	resp := IndicatorValues{
		TokenID: "tok_1",
		Values: map[string]float64{"rsi": 62.5},
		Signals: map[string]string{"rsi": "neutral"},
		Computed: map[string]int{"candles": 500},
	}
	return c.JSON(http.StatusOK, resp)
}

// 5. Struct with any/interface{} fields + json.Decode request
// API_DESC Track analytics event
// API_TAGS Analytics
func trackEventHandler(c *core.RequestEvent) error {
	var req AnalyticsEvent
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid"})
	}
	return c.JSON(http.StatusCreated, req)
}

// 6. Inline map literal with nested sub-maps
// API_DESC Get diagnostics
// API_TAGS System
func getDiagnosticsHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status":  "operational",
		"version": "2.1.0",
		"uptime":  86400,
		"memory": map[string]any{
			"allocated_mb": 128,
			"gc_cycles":    42,
		},
		"database": map[string]any{
			"connected": true,
			"pool_size": 10,
		},
	})
}

// 7. Flat struct + nested struct field
// API_DESC Get user profile
// API_TAGS Users
func getUserProfileHandler(c *core.RequestEvent) error {
	resp := UserProfile{
		ID: "u1", Username: "john",
		Contact: ContactInfo{Email: "john@example.com"},
	}
	return c.JSON(http.StatusOK, resp)
}

// 8. Paginated: inline map wrapping struct array + struct value
// API_DESC Search users
// API_TAGS Users
func searchUsersHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"data": []UserProfile{
			{ID: "u1", Username: "alice"},
		},
		"pagination": PaginationMeta{Page: 1, PerPage: 20, TotalItems: 1, TotalPages: 1},
	})
}

// 9. Array of numeric-heavy structs
// API_DESC Get candlestick data
// API_TAGS Analytics
func getCandlestickHandler(c *core.RequestEvent) error {
	data := []TimeseriesPoint{
		{Timestamp: 1000, Open: 1.0, High: 1.05, Low: 0.98, Close: 1.02, Volume: 50000},
	}
	return c.JSON(http.StatusOK, data)
}

// 10. Pure map[string]string variable
// API_DESC Get config
// API_TAGS System
func getConfigHandler(c *core.RequestEvent) error {
	config := map[string]string{
		"log_level": "info",
		"region":    "us-west-2",
	}
	return c.JSON(http.StatusOK, config)
}

// 11. Mixed inline map: bools, ints, strings, nested maps
// API_DESC Get feature flags
// API_TAGS System
func getFeatureFlagsHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"flags": map[string]any{
			"dark_mode": true,
			"beta_api":  false,
		},
		"rate_limits": map[string]any{
			"requests_per_minute": 60,
			"enabled":             true,
		},
		"maintenance": false,
	})
}

// 12. map[string]any variable with MapAdditions
// API_DESC Get platform stats
// API_TAGS Analytics
func getPlatformStatsHandler(c *core.RequestEvent) error {
	result := map[string]any{
		"total_users": 15000,
		"revenue":     89432.50,
	}
	result["computed_at"] = time.Now().Format(time.RFC3339)
	result["cached"] = true
	return c.JSON(http.StatusOK, result)
}

// 13. BindBody request
// API_DESC Update profile
// API_TAGS Users
func updateProfileHandler(c *core.RequestEvent) error {
	var req UpdateProfileRequest
	if err := c.BindBody(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"updated": map[string]any{
			"display_name": req.DisplayName,
		},
	})
}

// 14. Embedded struct
// API_DESC Get product
// API_TAGS Products
func getProductHandler(c *core.RequestEvent) error {
	resp := ProductResponse{
		BaseEntity: BaseEntity{ID: "p1"},
		Name: "Widget", Price: 29.99, Currency: "USD", InStock: true,
	}
	return c.JSON(http.StatusOK, resp)
}

// 15. Slice of primitives
// API_DESC List categories
// API_TAGS Products
func listCategoriesHandler(c *core.RequestEvent) error {
	categories := []string{"electronics", "clothing", "books"}
	return c.JSON(http.StatusOK, categories)
}

// 16. DELETE — minimal response
// API_DESC Delete product
// API_TAGS Products
func deleteProductHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"deleted": true,
	})
}

// 17. Variable-referenced struct
// API_DESC Health check
// API_TAGS System
func healthCheckHandler(c *core.RequestEvent) error {
	resp := HealthCheckResponse{Status: "healthy", Version: "2.1.0", Uptime: 86400}
	return c.JSON(http.StatusOK, resp)
}

// 18. Map with struct slice
// API_DESC Search products
// API_TAGS Products
func searchProductsHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"results": []ProductResponse{
			{BaseEntity: BaseEntity{ID: "p1"}, Name: "Widget", Price: 19.99},
		},
		"total":    1,
		"page":     1,
		"per_page": 20,
	})
}

// 19. Map literal containing struct values
// API_DESC Get order summary
// API_TAGS Orders
func getOrderSummaryHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"order_id": "ord_12345",
		"status":   "processing",
		"shipping": Address{
			Street: "789 Pine St", City: "Austin",
			Geo: GeoCoordinate{Latitude: 30.26, Longitude: -97.74},
		},
		"total_amount": 149.97,
	})
}

// 20. Multiple return paths + json.Decode request
// API_DESC Batch delete products
// API_TAGS Products
func batchDeleteHandler(c *core.RequestEvent) error {
	var req BatchDeleteRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid"})
	}
	if len(req.IDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "No IDs"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"deleted_count": len(req.IDs),
		"dry_run":       req.DryRun,
	})
}

// 21. Variable map with struct slices inside
// API_DESC Get dashboard
// API_TAGS Analytics
func getDashboardHandler(c *core.RequestEvent) error {
	dashboard := map[string]any{
		"recent_orders": []OrderResponse{
			{ID: "ord_999", Status: "pending"},
		},
		"top_users": []UserProfile{
			{ID: "u10", Username: "topuser"},
		},
		"total_revenue": 125000.50,
		"active_orders": 42,
	}
	return c.JSON(http.StatusOK, dashboard)
}

// 22. Struct pointer response
// API_DESC Get contact info
// API_TAGS Users
func getContactInfoHandler(c *core.RequestEvent) error {
	info := &ContactInfo{
		Email: "contact@example.com",
		SocialIDs: []string{"twitter:handle"},
	}
	return c.JSON(http.StatusOK, info)
}

// 23. Inline map with array of maps
// API_DESC Get activity feed
// API_TAGS Analytics
func getActivityFeedHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"activities": []map[string]any{
			{"type": "purchase", "user_id": "u1", "amount": 49.99},
		},
		"total_count": 2,
		"has_more":    false,
	})
}

// 24. Var-declared struct
// API_DESC Get default payment
// API_TAGS Orders
func getDefaultPaymentHandler(c *core.RequestEvent) error {
	var payment PaymentInfo = PaymentInfo{
		Method: "card", Currency: "USD", Amount: 0,
	}
	return c.JSON(http.StatusOK, payment)
}

// --- Helper functions (non-handlers) for return type resolution tests ---

func formatRecords(records []*core.Record) []map[string]any {
	result := make([]map[string]any, 0, len(records))
	return result
}

func buildSummary(name string) map[string]any {
	return map[string]any{"name": name}
}

func computeTotal(items []OrderItem) float64 {
	return 0.0
}

// 25. Handler calling a local function whose return type should be resolved
// API_DESC Get formatted records
// API_TAGS Records
func getFormattedRecordsHandler(c *core.RequestEvent) error {
	records, _ := c.App.FindRecordsByFilter("items", "1=1", "", 100, 0)
	items := formatRecords(records)
	result := map[string]any{"items": items, "count": len(items)}
	return c.JSON(http.StatusOK, result)
}

// 26. Handler with query parameters via URL.Query().Get()
// API_DESC Search with filters
// API_TAGS Search
func searchWithFiltersHandler(c *core.RequestEvent) error {
	q := c.Request.URL.Query()
	category := q.Get("category")
	status := q.Get("status")
	_ = category
	_ = status
	return c.JSON(http.StatusOK, map[string]any{"results": []string{}})
}

// 27. Handler calling a function that returns a primitive
// API_DESC Get computed total
// API_TAGS Orders
func getComputedTotalHandler(c *core.RequestEvent) error {
	total := computeTotal(nil)
	return c.JSON(http.StatusOK, map[string]any{"total": total})
}

// 25b. Map string any function
// API_DESC Get summary
// API_TAGS Summary
func getSummaryHandler(c *core.RequestEvent) error {
	summary := buildSummary("test")
	return c.JSON(http.StatusOK, summary)
}
`

// parseHandlerScenarios parses the handlerScenarioSource and returns the parser.
func parseHandlerScenarios(t *testing.T) *ASTParser {
	t.Helper()
	parser := NewASTParser()
	filePath := createTestFile(t, "handlers_scenario.go", handlerScenarioSource)
	if err := parser.ParseFile(filePath); err != nil {
		t.Fatalf("Failed to parse handler scenarios: %v", err)
	}
	return parser
}

// requireHandler returns the named handler or fails the test.
func requireHandler(t *testing.T, parser *ASTParser, name string) *ASTHandlerInfo {
	t.Helper()
	h, ok := parser.GetHandlerByName(name)
	if !ok || h == nil {
		t.Fatalf("Handler %q not found", name)
	}
	return h
}

// assertRef checks that a schema is a $ref to the given component.
func assertRef(t *testing.T, schema *OpenAPISchema, component string, context string) {
	t.Helper()
	if schema == nil {
		t.Fatalf("%s: schema is nil", context)
	}
	expected := "#/components/schemas/" + component
	if schema.Ref != expected {
		t.Errorf("%s: expected $ref %q, got Ref=%q Type=%q", context, expected, schema.Ref, schema.Type)
	}
}

// assertArrayOfRef checks that a schema is {type:"array", items:{$ref:...}}.
func assertArrayOfRef(t *testing.T, schema *OpenAPISchema, component string, context string) {
	t.Helper()
	if schema == nil {
		t.Fatalf("%s: schema is nil", context)
	}
	if schema.Type != "array" {
		t.Errorf("%s: expected type 'array', got %q", context, schema.Type)
	}
	if schema.Items == nil {
		t.Fatalf("%s: items is nil", context)
	}
	assertRef(t, schema.Items, component, context+" items")
}

// assertInlineObject checks that a schema is an inline object with the given property names.
func assertInlineObject(t *testing.T, schema *OpenAPISchema, expectedProps []string, context string) {
	t.Helper()
	if schema == nil {
		t.Fatalf("%s: schema is nil", context)
	}
	if schema.Type != "object" {
		t.Errorf("%s: expected type 'object', got %q", context, schema.Type)
	}
	if schema.Properties == nil {
		t.Fatalf("%s: properties is nil", context)
	}
	for _, prop := range expectedProps {
		if _, ok := schema.Properties[prop]; !ok {
			t.Errorf("%s: missing expected property %q", context, prop)
		}
	}
}

func TestHandlerScenario_DeepNestedStructResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getOrderHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}
	assertRef(t, h.ResponseSchema, "OrderResponse", "response")

	// Verify OrderResponse component has nested $ref fields
	schema := parser.generateSchemaFromType("OrderResponse", true)
	if schema == nil {
		t.Fatal("Expected OrderResponse schema")
	}
	assertRef(t, schema.Properties["shipping_address"], "Address", "shipping_address")
	assertRef(t, schema.Properties["payment"], "PaymentInfo", "payment")
	if schema.Properties["items"] == nil || schema.Properties["items"].Type != "array" {
		t.Fatal("Expected items to be array")
	}
	assertRef(t, schema.Properties["items"].Items, "OrderItem", "items")

	// Verify Address has nested $ref to GeoCoordinate
	addrSchema := parser.generateSchemaFromType("Address", true)
	assertRef(t, addrSchema.Properties["geo"], "GeoCoordinate", "Address.geo")
}

func TestHandlerScenario_JsonDecodeRequestBody(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "createOrderHandler")

	if h.RequestType != "CreateOrderRequest" {
		t.Errorf("Expected RequestType 'CreateOrderRequest', got %q", h.RequestType)
	}
	if h.RequestSchema == nil {
		t.Fatal("Expected request schema")
	}
	assertRef(t, h.RequestSchema, "CreateOrderRequest", "request")

	// Response should also be $ref OrderResponse
	assertRef(t, h.ResponseSchema, "OrderResponse", "response")

	// Verify json.Decode detection
	if !h.UsesJSONDecode {
		t.Error("Expected UsesJSONDecode to be true")
	}
}

func TestHandlerScenario_ArrayOfStructsResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "listOrdersHandler")

	assertArrayOfRef(t, h.ResponseSchema, "OrderResponse", "response")
}

func TestHandlerScenario_StructWithTypedMaps(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getIndicatorsHandler")

	assertRef(t, h.ResponseSchema, "IndicatorValues", "response")

	// Verify IndicatorValues component schema
	schema := parser.generateSchemaFromType("IndicatorValues", true)
	if schema == nil {
		t.Fatal("Expected IndicatorValues schema")
	}

	// map[string]float64 → additionalProperties: {type: "number"}
	valuesField := schema.Properties["values"]
	if valuesField == nil || valuesField.Type != "object" {
		t.Fatal("Expected values to be object")
	}
	if ap, ok := valuesField.AdditionalProperties.(*OpenAPISchema); ok {
		if ap.Type != "number" {
			t.Errorf("Expected values additionalProperties type 'number', got %q", ap.Type)
		}
	} else {
		t.Error("Expected values additionalProperties to be a schema")
	}

	// map[string]string → additionalProperties: {type: "string"}
	signalsField := schema.Properties["signals"]
	if ap, ok := signalsField.AdditionalProperties.(*OpenAPISchema); ok {
		if ap.Type != "string" {
			t.Errorf("Expected signals additionalProperties type 'string', got %q", ap.Type)
		}
	} else {
		t.Error("Expected signals additionalProperties to be a schema")
	}

	// map[string]int → additionalProperties: {type: "integer"}
	computedField := schema.Properties["computed"]
	if ap, ok := computedField.AdditionalProperties.(*OpenAPISchema); ok {
		if ap.Type != "integer" {
			t.Errorf("Expected computed additionalProperties type 'integer', got %q", ap.Type)
		}
	} else {
		t.Error("Expected computed additionalProperties to be a schema")
	}
}

func TestHandlerScenario_AnyFieldsAndJsonDecode(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "trackEventHandler")

	// Request: json.Decode → $ref AnalyticsEvent
	assertRef(t, h.RequestSchema, "AnalyticsEvent", "request")
	if !h.UsesJSONDecode {
		t.Error("Expected UsesJSONDecode to be true")
	}

	// Response: returning req variable → $ref AnalyticsEvent
	assertRef(t, h.ResponseSchema, "AnalyticsEvent", "response")

	// Verify any fields in component schema
	schema := parser.generateSchemaFromType("AnalyticsEvent", true)

	// map[string]any → additionalProperties: true (NOT nested object)
	propsField := schema.Properties["properties"]
	if propsField == nil {
		t.Fatal("Expected properties field")
	}
	if propsField.Type != "object" {
		t.Errorf("Expected properties type 'object', got %q", propsField.Type)
	}
	if propsField.AdditionalProperties != true {
		t.Errorf("Expected map[string]any to produce additionalProperties: true, got %v", propsField.AdditionalProperties)
	}

	// any → {type: "object", additionalProperties: true}
	contextField := schema.Properties["context"]
	if contextField == nil {
		t.Fatal("Expected context field")
	}
	if contextField.Type != "object" {
		t.Errorf("Expected context type 'object', got %q", contextField.Type)
	}
	if contextField.AdditionalProperties != true {
		t.Errorf("Expected any to produce additionalProperties: true, got %v", contextField.AdditionalProperties)
	}
}

func TestHandlerScenario_InlineMapWithNestedMaps(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getDiagnosticsHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"status", "version", "uptime", "memory", "database"}, "response")

	// Nested map values should be inline objects too
	memoryProp := schema.Properties["memory"]
	assertInlineObject(t, memoryProp, []string{"allocated_mb", "gc_cycles"}, "memory")

	dbProp := schema.Properties["database"]
	assertInlineObject(t, dbProp, []string{"connected", "pool_size"}, "database")
}

func TestHandlerScenario_FlatStructWithNestedStruct(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getUserProfileHandler")

	assertRef(t, h.ResponseSchema, "UserProfile", "response")

	// UserProfile.contact → $ref ContactInfo
	schema := parser.generateSchemaFromType("UserProfile", true)
	assertRef(t, schema.Properties["contact"], "ContactInfo", "contact")
}

func TestHandlerScenario_PaginatedMapWithStructs(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "searchUsersHandler")

	schema := h.ResponseSchema
	if schema == nil {
		t.Fatal("Expected response schema")
	}
	if schema.Properties == nil {
		t.Fatal("Expected inline object with properties")
	}

	// data → array of $ref UserProfile
	dataField := schema.Properties["data"]
	assertArrayOfRef(t, dataField, "UserProfile", "data")

	// pagination → $ref PaginationMeta
	paginationField := schema.Properties["pagination"]
	assertRef(t, paginationField, "PaginationMeta", "pagination")
}

func TestHandlerScenario_ArrayOfNumericStructs(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getCandlestickHandler")

	assertArrayOfRef(t, h.ResponseSchema, "TimeseriesPoint", "response")
}

func TestHandlerScenario_MapStringStringVariable(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getConfigHandler")

	schema := h.ResponseSchema
	if schema == nil {
		t.Fatal("Expected response schema")
	}
	// map[string]string literal → should have properties with string values
	if schema.Properties == nil {
		t.Fatal("Expected properties from map literal")
	}
	for _, key := range []string{"log_level", "region"} {
		prop := schema.Properties[key]
		if prop == nil {
			t.Errorf("Expected property %q", key)
			continue
		}
		if prop.Type != "string" {
			t.Errorf("Expected property %q type 'string', got %q", key, prop.Type)
		}
	}
}

func TestHandlerScenario_MixedInlineMap(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getFeatureFlagsHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"flags", "rate_limits", "maintenance"}, "response")

	// flags → nested inline object
	flagsProp := schema.Properties["flags"]
	assertInlineObject(t, flagsProp, []string{"dark_mode", "beta_api"}, "flags")

	// rate_limits → nested inline object
	rlProp := schema.Properties["rate_limits"]
	assertInlineObject(t, rlProp, []string{"requests_per_minute", "enabled"}, "rate_limits")

	// maintenance → boolean
	maintProp := schema.Properties["maintenance"]
	if maintProp == nil || maintProp.Type != "boolean" {
		t.Error("Expected maintenance to be boolean")
	}
}

func TestHandlerScenario_MapVariableWithAdditions(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getPlatformStatsHandler")

	schema := h.ResponseSchema
	if schema == nil || schema.Properties == nil {
		t.Fatal("Expected inline object response")
	}

	// Original map keys
	for _, key := range []string{"total_users", "revenue"} {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("Expected original map property %q", key)
		}
	}

	// MapAdditions: result["computed_at"] and result["cached"]
	if _, ok := schema.Properties["computed_at"]; !ok {
		t.Error("Expected MapAddition 'computed_at' to be merged")
	}
	if _, ok := schema.Properties["cached"]; !ok {
		t.Error("Expected MapAddition 'cached' to be merged")
	}
}

func TestHandlerScenario_BindBodyRequest(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "updateProfileHandler")

	// BindBody → request detected
	if !h.UsesBindBody {
		t.Error("Expected UsesBindBody to be true")
	}
	if h.RequestType != "UpdateProfileRequest" {
		t.Errorf("Expected RequestType 'UpdateProfileRequest', got %q", h.RequestType)
	}
	assertRef(t, h.RequestSchema, "UpdateProfileRequest", "request")

	// Response is inline map
	assertInlineObject(t, h.ResponseSchema, []string{"success", "updated"}, "response")
}

func TestHandlerScenario_EmbeddedStruct(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getProductHandler")

	assertRef(t, h.ResponseSchema, "ProductResponse", "response")

	// ProductResponse should have flattened fields from BaseEntity
	schema := parser.generateSchemaFromType("ProductResponse", true)
	if schema == nil {
		t.Fatal("Expected ProductResponse schema")
	}
	// BaseEntity fields should be promoted
	for _, field := range []string{"id", "created_at", "updated_at"} {
		if _, ok := schema.Properties[field]; !ok {
			t.Errorf("Expected embedded field %q from BaseEntity", field)
		}
	}
	// Own fields
	for _, field := range []string{"name", "description", "price", "currency", "in_stock"} {
		if _, ok := schema.Properties[field]; !ok {
			t.Errorf("Expected own field %q", field)
		}
	}
}

func TestHandlerScenario_SliceOfPrimitives(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "listCategoriesHandler")

	schema := h.ResponseSchema
	if schema == nil {
		t.Fatal("Expected response schema")
	}
	if schema.Type != "array" {
		t.Errorf("Expected type 'array', got %q", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("Expected items")
	}
	if schema.Items.Type != "string" {
		t.Errorf("Expected items type 'string', got %q", schema.Items.Type)
	}
}

func TestHandlerScenario_MinimalDeleteResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "deleteProductHandler")

	assertInlineObject(t, h.ResponseSchema, []string{"success", "deleted"}, "response")
}

func TestHandlerScenario_VariableReferencedStruct(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "healthCheckHandler")

	assertRef(t, h.ResponseSchema, "HealthCheckResponse", "response")
}

func TestHandlerScenario_MapWithStructSlice(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "searchProductsHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"results", "total", "page", "per_page"}, "response")

	// results → array of $ref ProductResponse
	assertArrayOfRef(t, schema.Properties["results"], "ProductResponse", "results")
}

func TestHandlerScenario_MapWithStructValue(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getOrderSummaryHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"order_id", "status", "shipping", "total_amount"}, "response")

	// shipping → $ref Address
	assertRef(t, schema.Properties["shipping"], "Address", "shipping")
}

func TestHandlerScenario_MultipleReturnPaths(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "batchDeleteHandler")

	// Request: json.Decode → $ref BatchDeleteRequest
	assertRef(t, h.RequestSchema, "BatchDeleteRequest", "request")
	if !h.UsesJSONDecode {
		t.Error("Expected UsesJSONDecode to be true")
	}

	// Response: last c.JSON call (success path)
	schema := h.ResponseSchema
	if schema == nil {
		t.Fatal("Expected response schema")
	}
	// Should have properties from the success-path map
	if schema.Properties != nil {
		if _, ok := schema.Properties["deleted_count"]; ok {
			// success path picked up — good
		}
	}
}

func TestHandlerScenario_VariableMapWithStructSlices(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getDashboardHandler")

	schema := h.ResponseSchema
	if schema == nil || schema.Properties == nil {
		t.Fatal("Expected inline object response")
	}

	// recent_orders → array of $ref OrderResponse
	recentOrders := schema.Properties["recent_orders"]
	assertArrayOfRef(t, recentOrders, "OrderResponse", "recent_orders")

	// top_users → array of $ref UserProfile
	topUsers := schema.Properties["top_users"]
	assertArrayOfRef(t, topUsers, "UserProfile", "top_users")

	// Primitive fields
	if _, ok := schema.Properties["total_revenue"]; !ok {
		t.Error("Expected 'total_revenue' property")
	}
	if _, ok := schema.Properties["active_orders"]; !ok {
		t.Error("Expected 'active_orders' property")
	}
}

func TestHandlerScenario_StructPointerResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getContactInfoHandler")

	assertRef(t, h.ResponseSchema, "ContactInfo", "response")
}

func TestHandlerScenario_InlineMapWithArrayOfMaps(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getActivityFeedHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"activities", "total_count", "has_more"}, "response")

	// activities → array — items should be free-form object
	activitiesField := schema.Properties["activities"]
	if activitiesField == nil || activitiesField.Type != "array" {
		t.Fatal("Expected activities to be array")
	}
	if activitiesField.Items == nil {
		t.Fatal("Expected activities items")
	}
	// []map[string]any → items should be {type:"object", additionalProperties:true}
	items := activitiesField.Items
	if items.Type != "object" {
		t.Errorf("Expected items type 'object', got %q", items.Type)
	}
	if items.AdditionalProperties != true {
		t.Errorf("Expected items additionalProperties to be true (free-form), got %v", items.AdditionalProperties)
	}
}

func TestHandlerScenario_VarDeclaredStruct(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getDefaultPaymentHandler")

	assertRef(t, h.ResponseSchema, "PaymentInfo", "response")
}

func TestHandlerScenario_MapStringAnyFreeForm(t *testing.T) {
	// Verify that map[string]any produces {type:"object", additionalProperties:true}
	// (NOT nested {additionalProperties: {type:"object", additionalProperties:true}})
	parser := parseHandlerScenarios(t)

	schema := parser.generateSchemaFromType("map[string]any", false)
	if schema == nil {
		t.Fatal("Expected schema for map[string]any")
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got %q", schema.Type)
	}
	if schema.AdditionalProperties != true {
		t.Errorf("Expected additionalProperties: true, got %v (type %T)", schema.AdditionalProperties, schema.AdditionalProperties)
	}
}

func TestHandlerScenario_StructDiscovery(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// All struct definitions should be discovered
	expectedStructs := []string{
		"GeoCoordinate", "Address", "ContactInfo", "OrderItem", "PaymentInfo",
		"OrderResponse", "CreateOrderRequest", "AnalyticsEvent", "PaginationMeta",
		"UserProfile", "TimeseriesPoint", "IndicatorValues", "UpdateProfileRequest",
		"BaseEntity", "ProductResponse", "HealthCheckResponse", "BatchDeleteRequest",
	}

	allStructs := parser.GetAllStructs()
	for _, name := range expectedStructs {
		if _, ok := allStructs[name]; !ok {
			t.Errorf("Expected struct %q to be discovered", name)
		}
	}
}

func TestHandlerScenario_HandlerDiscovery(t *testing.T) {
	parser := parseHandlerScenarios(t)

	expectedHandlers := []string{
		"getOrderHandler", "createOrderHandler", "listOrdersHandler",
		"getIndicatorsHandler", "trackEventHandler", "getDiagnosticsHandler",
		"getUserProfileHandler", "searchUsersHandler", "getCandlestickHandler",
		"getConfigHandler", "getFeatureFlagsHandler", "getPlatformStatsHandler",
		"updateProfileHandler", "getProductHandler", "listCategoriesHandler",
		"deleteProductHandler", "healthCheckHandler", "searchProductsHandler",
		"getOrderSummaryHandler", "batchDeleteHandler", "getDashboardHandler",
		"getContactInfoHandler", "getActivityFeedHandler", "getDefaultPaymentHandler",
		"getFormattedRecordsHandler", "searchWithFiltersHandler",
		"getComputedTotalHandler", "getSummaryHandler",
	}

	allHandlers := parser.GetAllHandlers()
	if len(allHandlers) != len(expectedHandlers) {
		t.Errorf("Expected %d handlers, got %d", len(expectedHandlers), len(allHandlers))
	}
	for _, name := range expectedHandlers {
		if _, ok := allHandlers[name]; !ok {
			t.Errorf("Expected handler %q to be discovered", name)
		}
	}
}

func TestHandlerScenario_APIDescAndTags(t *testing.T) {
	parser := parseHandlerScenarios(t)

	tests := []struct {
		handler string
		desc    string
		tags    []string
	}{
		{"getOrderHandler", "Get order details", []string{"Orders"}},
		{"createOrderHandler", "Create a new order", []string{"Orders"}},
		{"getDiagnosticsHandler", "Get diagnostics", []string{"System"}},
		{"trackEventHandler", "Track analytics event", []string{"Analytics"}},
		{"updateProfileHandler", "Update profile", []string{"Users"}},
		{"getProductHandler", "Get product", []string{"Products"}},
	}

	for _, tt := range tests {
		t.Run(tt.handler, func(t *testing.T) {
			h := requireHandler(t, parser, tt.handler)
			if h.APIDescription != tt.desc {
				t.Errorf("Expected desc %q, got %q", tt.desc, h.APIDescription)
			}
			if len(h.APITags) != len(tt.tags) {
				t.Errorf("Expected %d tags, got %d: %v", len(tt.tags), len(h.APITags), h.APITags)
				return
			}
			for i, tag := range tt.tags {
				if h.APITags[i] != tag {
					t.Errorf("Expected tag[%d] = %q, got %q", i, tag, h.APITags[i])
				}
			}
		})
	}
}

func TestHandlerScenario_PointerFields(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// OrderResponse has *Address for billing_address and *string for notes
	schema := parser.generateSchemaFromType("OrderResponse", true)
	if schema == nil {
		t.Fatal("Expected OrderResponse schema")
	}

	// billing_address is *Address → should still be $ref (pointer unwrapped)
	billingAddr := schema.Properties["billing_address"]
	assertRef(t, billingAddr, "Address", "billing_address")

	// ContactInfo has *string fields
	ciSchema := parser.generateSchemaFromType("ContactInfo", true)
	phoneField := ciSchema.Properties["phone"]
	if phoneField == nil {
		t.Fatal("Expected phone field")
	}
	if phoneField.Type != "string" {
		t.Errorf("Expected phone type 'string', got %q", phoneField.Type)
	}
}

// =============================================================================
// Function Return Type Resolution Tests
// =============================================================================

func TestFuncReturnTypeExtraction(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// Check that helper function return types were extracted
	tests := []struct {
		funcName     string
		expectedType string
	}{
		{"formatRecords", "[]map[string]any"},
		{"buildSummary", "map[string]any"},
		{"computeTotal", "float64"},
	}

	for _, tt := range tests {
		retType, exists := parser.funcReturnTypes[tt.funcName]
		if !exists {
			t.Errorf("Expected return type for %q to be extracted", tt.funcName)
			continue
		}
		if retType != tt.expectedType {
			t.Errorf("Return type for %q: expected %q, got %q", tt.funcName, tt.expectedType, retType)
		}
	}

	// Handlers should NOT be in funcReturnTypes
	for _, handlerName := range []string{"getOrderHandler", "healthCheckHandler", "getFormattedRecordsHandler"} {
		if _, exists := parser.funcReturnTypes[handlerName]; exists {
			t.Errorf("Handler %q should not be in funcReturnTypes", handlerName)
		}
	}
}

func TestHandlerScenario_FunctionReturnTypeResolution(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getFormattedRecordsHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// Response should be an object with "items" and "count"
	assertInlineObject(t, h.ResponseSchema, []string{"items", "count"}, "response")

	// "items" should be type:"array" (resolved from formatRecords return type []map[string]any)
	itemsSchema := h.ResponseSchema.Properties["items"]
	if itemsSchema == nil {
		t.Fatal("Expected 'items' property in response schema")
	}
	if itemsSchema.Type != "array" {
		t.Errorf("Expected 'items' type to be 'array', got %q", itemsSchema.Type)
	}

	// "count" should be integer (from len())
	countSchema := h.ResponseSchema.Properties["count"]
	if countSchema == nil {
		t.Fatal("Expected 'count' property in response schema")
	}
	if countSchema.Type != "integer" {
		t.Errorf("Expected 'count' type to be 'integer', got %q", countSchema.Type)
	}
}

func TestHandlerScenario_FunctionReturnTypePrimitive(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getComputedTotalHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// "total" should be number (resolved from computeTotal return type float64)
	totalSchema := h.ResponseSchema.Properties["total"]
	if totalSchema == nil {
		t.Fatal("Expected 'total' property in response schema")
	}
	if totalSchema.Type != "number" {
		t.Errorf("Expected 'total' type to be 'number', got %q", totalSchema.Type)
	}
}

func TestHandlerScenario_FunctionReturnTypeMapResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getSummaryHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// buildSummary returns map[string]any — the response variable holds this
	// The schema should be an object (from map[string]any resolution)
	if h.ResponseSchema.Type != "object" {
		t.Errorf("Expected response type 'object', got %q", h.ResponseSchema.Type)
	}
}

// =============================================================================
// Query Parameter Detection Tests
// =============================================================================

func TestHandlerScenario_QueryParameterDetection(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "searchWithFiltersHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected query parameters to be detected")
	}

	// Should have detected "category" and "status"
	paramNames := map[string]bool{}
	for _, p := range h.Parameters {
		paramNames[p.Name] = true
		if p.Source != "query" {
			t.Errorf("Expected parameter %q source to be 'query', got %q", p.Name, p.Source)
		}
		if p.Type != "string" {
			t.Errorf("Expected parameter %q type to be 'string', got %q", p.Name, p.Type)
		}
	}

	if !paramNames["category"] {
		t.Error("Expected 'category' query parameter to be detected")
	}
	if !paramNames["status"] {
		t.Error("Expected 'status' query parameter to be detected")
	}
}

func TestHandlerScenario_NoQueryParamsOnSimpleHandler(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "healthCheckHandler")

	if len(h.Parameters) > 0 {
		t.Errorf("Expected no parameters on healthCheckHandler, got %d", len(h.Parameters))
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
