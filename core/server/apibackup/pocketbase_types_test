package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestPocketBaseTypeDetection tests type detection for common PocketBase patterns
func TestPocketBaseTypeDetection(t *testing.T) {
	src := `
package handlers

import (
	"encoding/json"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

type TodoRequest struct {
	Title string ` + "`json:\"title\"`" + `
}

// PocketBaseHandler demonstrates common PocketBase patterns
func PocketBaseHandler(e *core.RequestEvent) error {
	// Path and query params
	todoID := e.PathParam("id")
	filter := e.QueryParam("filter")

	// Collection operations
	collection := e.App().FindCollectionByNameOrId("todos")
	record := collection.FindRecordById(todoID)

	// Record getters
	title := record.GetString("title")
	priority := record.GetInt("priority")
	completed := record.GetBool("completed")
	createdAt := record.GetDateTime("created")

	// String literals
	status := "active"
	count := 42
	score := 3.14

	// Make operations
	data := make(map[string]interface{})
	items := make([]string, 0)

	// Built-in functions
	length := len(items)
	capacity := cap(items)

	// Type conversions
	idStr := string(todoID)
	countInt := int(count)

	return e.JSON(200, map[string]interface{}{
		"data": data,
		"count": length,
	})
}

// ErrorHandler tests error return type detection
func ErrorHandler(e *core.RequestEvent) error {
	collection := e.App().FindCollectionByNameOrId("todos")
	if collection == nil {
		return e.JSON(404, map[string]string{"error": "not found"})
	}

	record := collection.FindRecordById("123")
	err := record.Save()
	if err != nil {
		return err
	}

	return e.NoContent(204)
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "pocketbase_test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	testCases := []struct {
		handlerName     string
		expectedVars    map[string]string
		shouldHaveError bool
		description     string
	}{
		{
			handlerName: "PocketBaseHandler",
			expectedVars: map[string]string{
				"todoID":     "string",
				"filter":     "string",
				"collection": "*models.Collection",
				"record":     "*models.Record",
				"title":      "string",
				"priority":   "int",
				"completed":  "bool",
				"createdAt":  "time.Time",
				"status":     "string",
				"count":      "int",
				"score":      "float64",
				"data":       "map[string]interface{}",
				"items":      "[]string",
				"length":     "int",
				"capacity":   "int",
				"idStr":      "string",
				"countInt":   "int",
			},
			shouldHaveError: false,
			description:     "Should detect all PocketBase-specific types correctly",
		},
		{
			handlerName: "ErrorHandler",
			expectedVars: map[string]string{
				"collection": "*models.Collection",
				"record":     "*models.Record",
				"err":        "error",
			},
			shouldHaveError: false,
			description:     "Should detect error return types",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.handlerName, func(t *testing.T) {
			var handlerFound bool

			ast.Inspect(file, func(n ast.Node) bool {
				if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == tc.handlerName {
					handlerFound = true
					handlerInfo := &ASTHandlerInfo{
						Name:      tc.handlerName,
						Variables: make(map[string]string),
					}

					// Analyze the handler body
					astParser.analyzeHandlerBody(fn.Body, handlerInfo)

					t.Logf("[%s] Found %d tracked variables", tc.description, len(handlerInfo.Variables))

					// Check expected variables
					for varName, expectedType := range tc.expectedVars {
						if actualType, exists := handlerInfo.Variables[varName]; !exists {
							t.Errorf("[%s] Expected variable '%s' to be tracked but it wasn't", tc.description, varName)
						} else if actualType != expectedType {
							t.Errorf("[%s] Variable '%s': expected type '%s', got '%s'",
								tc.description, varName, expectedType, actualType)
						} else {
							t.Logf("[%s] ✓ Variable '%s': %s", tc.description, varName, actualType)
						}
					}

					// Log all tracked variables for debugging
					t.Logf("[%s] All tracked variables:", tc.description)
					for varName, varType := range handlerInfo.Variables {
						t.Logf("  %s: %s", varName, varType)
					}

					// Check JSON response detection
					if handlerInfo.UsesJSONReturn {
						t.Logf("[%s] ✓ JSON response detected", tc.description)
					} else {
						t.Logf("[%s] ⚠ JSON response not detected", tc.description)
					}

					return false
				}
				return true
			})

			if !handlerFound {
				t.Fatalf("Handler function '%s' not found in test code", tc.handlerName)
			}
		})
	}
}

// TestMethodCallTypeDetection tests specific method call patterns
func TestMethodCallTypeDetection(t *testing.T) {
	src := `
package handlers

func TestMethodCalls(e *core.RequestEvent) error {
	// Test various method call patterns
	record := e.App().FindRecordById("123")

	// Record getters
	str := record.GetString("field")
	num := record.GetInt("number")
	flag := record.GetBool("active")
	dt := record.GetDateTime("created")
	val := record.Get("data")

	// Collection methods
	coll := e.App().FindCollectionByNameOrId("test")
	recs := coll.FindRecordsByFilter("active = true")
	first := coll.FindFirstRecordByFilter("id = '123'")

	// Error returning methods
	saveErr := record.Save()
	delErr := record.Delete()
	valErr := record.Validate()

	return e.JSON(200, map[string]interface{}{
		"message": "success",
	})
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "method_test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	expectedTypes := map[string]string{
		"record":  "*models.Record",
		"str":     "string",
		"num":     "int",
		"flag":    "bool",
		"dt":      "time.Time",
		"val":     "interface{}",
		"coll":    "*models.Collection",
		"recs":    "[]*models.Record",
		"first":   "*models.Record",
		"saveErr": "error",
		"delErr":  "error",
		"valErr":  "error",
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "TestMethodCalls" {
			handlerInfo := &ASTHandlerInfo{
				Name:      "TestMethodCalls",
				Variables: make(map[string]string),
			}

			astParser.analyzeHandlerBody(fn.Body, handlerInfo)

			t.Logf("Method call type detection results:")
			for varName, expectedType := range expectedTypes {
				if actualType, exists := handlerInfo.Variables[varName]; !exists {
					t.Errorf("Expected variable '%s' to be tracked but it wasn't", varName)
				} else if actualType != expectedType {
					t.Errorf("Variable '%s': expected type '%s', got '%s'", varName, expectedType, actualType)
				} else {
					t.Logf("✓ %s: %s", varName, actualType)
				}
			}

			return false
		}
		return true
	})
}

// TestBuiltinFunctionTypes tests detection of Go builtin function return types
func TestBuiltinFunctionTypes(t *testing.T) {
	src := `
package handlers

func TestBuiltins(e *core.RequestEvent) error {
	items := make([]string, 5, 10)
	data := make(map[string]int)

	// Builtin functions
	length := len(items)
	capacity := cap(items)

	// Type conversions
	str := string(123)
	num := int("456")
	fl := float64(789)
	bl := bool(1)

	// Constructor patterns
	result := NewResult()
	config := CreateConfig()

	return nil
}

type Result struct {
	Value string
}

type Config struct {
	Name string
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "builtins_test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	expectedTypes := map[string]string{
		"items":    "[]string",
		"data":     "map[string]int",
		"length":   "int",
		"capacity": "int",
		"str":      "string",
		"num":      "int",
		"fl":       "float64",
		"bl":       "bool",
		"result":   "Result",
		"config":   "Config",
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "TestBuiltins" {
			handlerInfo := &ASTHandlerInfo{
				Name:      "TestBuiltins",
				Variables: make(map[string]string),
			}

			astParser.analyzeHandlerBody(fn.Body, handlerInfo)

			t.Logf("Builtin function type detection results:")
			for varName, expectedType := range expectedTypes {
				if actualType, exists := handlerInfo.Variables[varName]; !exists {
					t.Errorf("Expected variable '%s' to be tracked but it wasn't", varName)
				} else if actualType != expectedType {
					t.Errorf("Variable '%s': expected type '%s', got '%s'", varName, expectedType, actualType)
				} else {
					t.Logf("✓ %s: %s", varName, actualType)
				}
			}

			// Also log all variables for debugging
			t.Logf("All tracked variables:")
			for varName, varType := range handlerInfo.Variables {
				t.Logf("  %s: %s", varName, varType)
			}

			return false
		}
		return true
	})
}

// TestLiteralTypeDetection tests detection of literal value types
func TestLiteralTypeDetection(t *testing.T) {
	src := `
package handlers

func TestLiterals(e *core.RequestEvent) error {
	// String literals
	message := "hello world"
	path := "/api/todos"

	// Numeric literals
	count := 42
	price := 29.99

	// Boolean would be handled differently since Go doesn't have bool literals
	// that can be assigned directly without explicit type

	return e.JSON(200, map[string]interface{}{
		"message": message,
		"count": count,
		"price": price,
	})
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "literals_test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	expectedTypes := map[string]string{
		"message": "string",
		"path":    "string",
		"count":   "int",
		"price":   "float64",
	}

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "TestLiterals" {
			handlerInfo := &ASTHandlerInfo{
				Name:      "TestLiterals",
				Variables: make(map[string]string),
			}

			astParser.analyzeHandlerBody(fn.Body, handlerInfo)

			t.Logf("Literal type detection results:")
			for varName, expectedType := range expectedTypes {
				if actualType, exists := handlerInfo.Variables[varName]; !exists {
					t.Errorf("Expected variable '%s' to be tracked but it wasn't", varName)
				} else if actualType != expectedType {
					t.Errorf("Variable '%s': expected type '%s', got '%s'", varName, expectedType, actualType)
				} else {
					t.Logf("✓ %s: %s", varName, actualType)
				}
			}

			return false
		}
		return true
	})
}
