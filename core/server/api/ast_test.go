package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestTypeDetectionFromDecodeCall(t *testing.T) {
	src := `
package main

func handler(e *core.RequestEvent) error {
	var req TodoRequest
	json.NewDecoder().Decode(&req)
	return nil
}

type TodoRequest struct {
	Title string ` + "`json:\"title\"`" + `
	Done  bool   ` + "`json:\"done\"`" + `
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	// Find the handler function
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "handler" {
			handlerInfo := &ASTHandlerInfo{
				Name:      "handler",
				Variables: make(map[string]string),
			}

			// Analyze the handler body
			astParser.analyzeHandlerBody(fn.Body, handlerInfo)

			// Check if the request type was detected correctly
			if handlerInfo.RequestType != "TodoRequest" {
				t.Errorf("Expected RequestType to be 'TodoRequest', got '%s'", handlerInfo.RequestType)
			}

			// Check if the variable was tracked
			if varType, exists := handlerInfo.Variables["req"]; !exists {
				t.Error("Expected variable 'req' to be tracked")
			} else if varType != "TodoRequest" {
				t.Errorf("Expected variable 'req' to have type 'TodoRequest', got '%s'", varType)
			}

			return false
		}
		return true
	})
}

func TestTypeDetectionFromShortDeclaration(t *testing.T) {
	src := `
package main

func handler(e *core.RequestEvent) error {
	req := &TodoRequest{}
	json.NewDecoder().Decode(req)
	return nil
}

type TodoRequest struct {
	Title string ` + "`json:\"title\"`" + `
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	// Find the handler function
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "handler" {
			handlerInfo := &ASTHandlerInfo{
				Name:      "handler",
				Variables: make(map[string]string),
			}

			// Analyze the handler body
			astParser.analyzeHandlerBody(fn.Body, handlerInfo)

			// Check if the request type was detected correctly
			if handlerInfo.RequestType != "TodoRequest" {
				t.Errorf("Expected RequestType to be 'TodoRequest', got '%s'", handlerInfo.RequestType)
			}

			// Check if the variable was tracked
			if varType, exists := handlerInfo.Variables["req"]; !exists {
				t.Error("Expected variable 'req' to be tracked")
			} else if varType != "TodoRequest" {
				t.Errorf("Expected variable 'req' to have type 'TodoRequest', got '%s'", varType)
			}

			return false
		}
		return true
	})
}

func TestTypeDetectionFromAssignment(t *testing.T) {
	src := `
package main

func handler(e *core.RequestEvent) error {
	var req TodoRequest
	req = TodoRequest{Title: "test"}
	json.NewDecoder().Decode(&req)
	return nil
}

type TodoRequest struct {
	Title string ` + "`json:\"title\"`" + `
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	// Find the handler function
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "handler" {
			handlerInfo := &ASTHandlerInfo{
				Name:      "handler",
				Variables: make(map[string]string),
			}

			// Analyze the handler body
			astParser.analyzeHandlerBody(fn.Body, handlerInfo)

			// Check if the request type was detected correctly
			if handlerInfo.RequestType != "TodoRequest" {
				t.Errorf("Expected RequestType to be 'TodoRequest', got '%s'", handlerInfo.RequestType)
			}

			return false
		}
		return true
	})
}

func TestVariableTracking(t *testing.T) {
	src := `
package main

func handler(e *core.RequestEvent) error {
	var req TodoRequest
	resp := &TodoResponse{}
	data := make(map[string]interface{})
	result := NewSomeResult()
	return nil
}

type TodoRequest struct {
	Title string
}

type TodoResponse struct {
	ID string
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	astParser := NewASTParser()

	// Find the handler function
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "handler" {
			handlerInfo := &ASTHandlerInfo{
				Name:      "handler",
				Variables: make(map[string]string),
			}

			// Analyze the handler body
			astParser.analyzeHandlerBody(fn.Body, handlerInfo)

			// Check variable tracking
			expectedVars := map[string]string{
				"req":    "TodoRequest",
				"resp":   "TodoResponse",
				"data":   "map[string]interface{}",
				"result": "SomeResult", // Should detect from NewSomeResult()
			}

			for varName, expectedType := range expectedVars {
				if actualType, exists := handlerInfo.Variables[varName]; !exists {
					t.Errorf("Expected variable '%s' to be tracked", varName)
				} else if actualType != expectedType {
					t.Errorf("Expected variable '%s' to have type '%s', got '%s'", varName, expectedType, actualType)
				}
			}

			return false
		}
		return true
	})
}

func TestExtractTypeFromExpressionWithContext(t *testing.T) {
	astParser := NewASTParser()
	handlerInfo := &ASTHandlerInfo{
		Name: "testHandler",
		Variables: map[string]string{
			"req":  "TodoRequest",
			"resp": "TodoResponse",
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tracked variable",
			input:    "req",
			expected: "TodoRequest",
		},
		{
			name:     "pointer to tracked variable",
			input:    "&req",
			expected: "TodoRequest",
		},
		{
			name:     "unknown variable",
			input:    "unknown",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the expression
			expr, err := parser.ParseExpr(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse expression '%s': %v", tt.input, err)
			}

			result := astParser.extractTypeFromExpressionWithContext(expr, handlerInfo)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
