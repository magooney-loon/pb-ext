package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestOriginalJSONDecodeIssue tests the specific issue mentioned:
// When json.NewDecoder().Decode(&req) is processed, extractTypeFromExpression
// was receiving &req and returning "req" instead of the actual type "TodoRequest"
func TestOriginalJSONDecodeIssue(t *testing.T) {
	// This is the exact scenario that was failing before the fix
	src := `
package handlers

import (
	"encoding/json"
	"github.com/pocketbase/pocketbase/core"
)

type TodoRequest struct {
	Title       string ` + "`json:\"title\" validate:\"required\"`" + `
	Description string ` + "`json:\"description\"`" + `
	Priority    int    ` + "`json:\"priority\" validate:\"min=1,max=5\"`" + `
	Done        bool   ` + "`json:\"done\"`" + `
}

type TodoResponse struct {
	ID          string ` + "`json:\"id\"`" + `
	Title       string ` + "`json:\"title\"`" + `
	Description string ` + "`json:\"description\"`" + `
	Priority    int    ` + "`json:\"priority\"`" + `
	Done        bool   ` + "`json:\"done\"`" + `
	CreatedAt   string ` + "`json:\"created_at\"`" + `
	UpdatedAt   string ` + "`json:\"updated_at\"`" + `
}

// CreateTodoHandler creates a new todo item
func CreateTodoHandler(e *core.RequestEvent) error {
	var req TodoRequest

	// This was the problematic line - it should detect TodoRequest, not "req"
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return err
	}

	// Process the request...
	response := TodoResponse{
		ID:          "todo_123",
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Done:        req.Done,
		CreatedAt:   "2024-01-01T00:00:00Z",
		UpdatedAt:   "2024-01-01T00:00:00Z",
	}

	return e.JSON(200, response)
}

// UpdateTodoHandler updates an existing todo item
func UpdateTodoHandler(e *core.RequestEvent) error {
	// Test short variable declaration with pointer
	req := &TodoRequest{}

	// This should also detect TodoRequest, not "req"
	if err := json.NewDecoder(e.Request.Body).Decode(req); err != nil {
		return err
	}

	return e.JSON(200, map[string]interface{}{
		"message": "Todo updated successfully",
		"data":    req,
	})
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "handlers.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	testCases := []struct {
		handlerName          string
		expectedRequestType  string
		expectedResponseType string
		description          string
	}{
		{
			handlerName:          "CreateTodoHandler",
			expectedRequestType:  "TodoRequest",
			expectedResponseType: "TodoResponse",
			description:          "var declaration with json.NewDecoder().Decode(&req)",
		},
		{
			handlerName:          "UpdateTodoHandler",
			expectedRequestType:  "TodoRequest",
			expectedResponseType: "map[string]any",
			description:          "short declaration req := &TodoRequest{} with json.NewDecoder().Decode(req)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.handlerName, func(t *testing.T) {
			// Find the handler function
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

					// Verify request type detection
					if handlerInfo.RequestType != tc.expectedRequestType {
						t.Errorf("[%s] Expected RequestType to be '%s', got '%s'",
							tc.description, tc.expectedRequestType, handlerInfo.RequestType)
					}

					// Verify response type detection
					if handlerInfo.ResponseType != tc.expectedResponseType {
						t.Errorf("[%s] Expected ResponseType to be '%s', got '%s'",
							tc.description, tc.expectedResponseType, handlerInfo.ResponseType)
					}

					// Verify that JSON decode was detected
					if !handlerInfo.UsesJSONDecode {
						t.Errorf("[%s] Expected UsesJSONDecode to be true", tc.description)
					}

					// Verify that JSON response was detected
					if !handlerInfo.UsesJSONReturn {
						t.Errorf("[%s] Expected UsesJSONReturn to be true", tc.description)
					}

					// Verify variable tracking worked correctly
					if tc.handlerName == "CreateTodoHandler" {
						// Should track req as TodoRequest from var declaration
						if reqType, exists := handlerInfo.Variables["req"]; !exists {
							t.Errorf("[%s] Expected variable 'req' to be tracked", tc.description)
						} else if reqType != "TodoRequest" {
							t.Errorf("[%s] Expected variable 'req' type to be 'TodoRequest', got '%s'",
								tc.description, reqType)
						}
					}

					if tc.handlerName == "UpdateTodoHandler" {
						// Should track req as TodoRequest from short declaration
						if reqType, exists := handlerInfo.Variables["req"]; !exists {
							t.Errorf("[%s] Expected variable 'req' to be tracked", tc.description)
						} else if reqType != "TodoRequest" {
							t.Errorf("[%s] Expected variable 'req' type to be 'TodoRequest', got '%s'",
								tc.description, reqType)
						}
					}

					t.Logf("[%s] ✓ Successfully detected RequestType: %s, ResponseType: %s",
						tc.description, handlerInfo.RequestType, handlerInfo.ResponseType)

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

// TestEdgeCasesForTypeDetection tests edge cases that could break type detection
func TestEdgeCasesForTypeDetection(t *testing.T) {
	src := `
package handlers

import (
	"encoding/json"
	"github.com/pocketbase/pocketbase/core"
)

type RequestData struct {
	Name string ` + "`json:\"name\"`" + `
}

// MultipleVariableHandler tests multiple variables and reassignments
func MultipleVariableHandler(e *core.RequestEvent) error {
	var req RequestData
	var req2 *RequestData

	// First decode should set RequestType
	json.NewDecoder().Decode(&req)

	// Second decode should not override RequestType
	req2 = &RequestData{}
	json.NewDecoder().Decode(req2)

	return e.JSON(200, req)
}

// AnonymousStructHandler tests anonymous struct types
func AnonymousStructHandler(e *core.RequestEvent) error {
	req := struct {
		Message string ` + "`json:\"message\"`" + `
	}{}

	json.NewDecoder().Decode(&req)

	return e.JSON(200, map[string]string{"status": "ok"})
}

// ComplexPointerHandler tests complex pointer operations
func ComplexPointerHandler(e *core.RequestEvent) error {
	var req *RequestData
	req = new(RequestData)

	json.NewDecoder().Decode(req)

	return e.JSON(200, req)
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "edge_cases.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	testCases := []struct {
		handlerName         string
		expectedRequestType string
		shouldDetectDecode  bool
		description         string
	}{
		{
			handlerName:         "MultipleVariableHandler",
			expectedRequestType: "RequestData",
			shouldDetectDecode:  true,
			description:         "First variable declaration should set type, subsequent decodes shouldn't override",
		},
		{
			handlerName:         "ComplexPointerHandler",
			expectedRequestType: "*RequestData",
			shouldDetectDecode:  true,
			description:         "Complex pointer operations with new() should work",
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

					astParser.analyzeHandlerBody(fn.Body, handlerInfo)

					if handlerInfo.RequestType != tc.expectedRequestType {
						t.Errorf("[%s] Expected RequestType '%s', got '%s'",
							tc.description, tc.expectedRequestType, handlerInfo.RequestType)
					}

					if handlerInfo.UsesJSONDecode != tc.shouldDetectDecode {
						t.Errorf("[%s] Expected UsesJSONDecode to be %v, got %v",
							tc.description, tc.shouldDetectDecode, handlerInfo.UsesJSONDecode)
					}

					t.Logf("[%s] ✓ RequestType: %s, Decode detected: %v",
						tc.description, handlerInfo.RequestType, handlerInfo.UsesJSONDecode)

					return false
				}
				return true
			})

			if !handlerFound {
				t.Fatalf("Handler function '%s' not found", tc.handlerName)
			}
		})
	}
}

// TestBeforeAndAfterFix demonstrates what the behavior was before and after the fix
func TestBeforeAndAfterFix(t *testing.T) {
	t.Log("=== Demonstration of the fix ===")
	t.Log("BEFORE: json.NewDecoder().Decode(&req) would return 'req' as the type")
	t.Log("AFTER:  json.NewDecoder().Decode(&req) correctly returns 'TodoRequest' as the type")
	t.Log("")

	src := `
package main

func handler(e *core.RequestEvent) error {
	var req TodoRequest
	json.NewDecoder().Decode(&req)
	return nil
}

type TodoRequest struct {
	Title string
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "demo.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse demo file: %v", err)
	}

	astParser := NewASTParser()

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "handler" {
			handlerInfo := &ASTHandlerInfo{
				Name:      "handler",
				Variables: make(map[string]string),
			}

			astParser.analyzeHandlerBody(fn.Body, handlerInfo)

			t.Logf("✓ Variable tracking working: req -> %s", handlerInfo.Variables["req"])
			t.Logf("✓ Request type correctly detected: %s", handlerInfo.RequestType)
			t.Logf("✓ JSON decode detected: %v", handlerInfo.UsesJSONDecode)

			// This should now be "TodoRequest", not "req"
			if handlerInfo.RequestType != "TodoRequest" {
				t.Errorf("Fix not working! Expected 'TodoRequest', got '%s'", handlerInfo.RequestType)
			} else {
				t.Log("✓ Fix is working correctly!")
			}

			return false
		}
		return true
	})
}
