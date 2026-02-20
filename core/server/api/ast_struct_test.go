package api

import (
	"testing"
)

// =============================================================================
// Struct Parsing, Schema Generation, and Type Alias Tests
// These tests exercise ast_struct.go: struct extraction, field schema generation,
// circular references, type aliases, and map type schemas.
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
