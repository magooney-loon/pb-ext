package api

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Test Handler Functions for Discovery
// =============================================================================

// Discovery-specific mock handler functions
func discoveryTestHandler(c *core.RequestEvent) error {
	return nil
}

func discoveryGetUsersHandler(c *core.RequestEvent) error {
	return nil
}

func discoveryCreateUserHandler(c *core.RequestEvent) error {
	return nil
}

func discoveryComplexPackageHandler(c *core.RequestEvent) error {
	return nil
}

// =============================================================================
// RouteAnalyzer Tests
// =============================================================================

func TestNewRouteAnalyzer(t *testing.T) {
	analyzer := NewRouteAnalyzer()

	if analyzer == nil {
		t.Fatal("Expected non-nil RouteAnalyzer")
	}
}

func TestRouteAnalyzerAnalyzeHandler(t *testing.T) {
	analyzer := NewRouteAnalyzer()

	tests := []struct {
		name         string
		handler      func(*core.RequestEvent) error
		expectNil    bool
		expectedName string
	}{
		{
			name:         "Standard handler function",
			handler:      discoveryTestHandler,
			expectNil:    false,
			expectedName: "discoveryTestHandler",
		},
		{
			name:         "Get users handler",
			handler:      discoveryGetUsersHandler,
			expectNil:    false,
			expectedName: "discoveryGetUsersHandler",
		},
		{
			name:         "Create user handler",
			handler:      discoveryCreateUserHandler,
			expectNil:    false,
			expectedName: "discoveryCreateUserHandler",
		},
		{
			name:      "Nil handler",
			handler:   nil,
			expectNil: false, // Should return default HandlerInfo, not nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.AnalyzeHandler(tt.handler)

			if tt.expectNil && result != nil {
				t.Error("Expected nil result but got HandlerInfo")
			}
			if !tt.expectNil && result == nil {
				t.Error("Expected HandlerInfo but got nil")
			}

			if result != nil {
				if tt.handler != nil {
					// For non-nil handlers, check the extracted name
					if result.Name == "" {
						t.Error("Expected non-empty handler name")
					}
					if result.FullName == "" {
						t.Error("Expected non-empty full handler name")
					}
					if result.Description == "" {
						t.Error("Expected non-empty description")
					}
				} else {
					// For nil handler, should return unknown handler info
					if result.Name != "unknown" {
						t.Errorf("Expected name 'unknown' for nil handler, got %s", result.Name)
					}
				}
			}
		})
	}
}

func TestRouteAnalyzerExtractPackageName(t *testing.T) {
	analyzer := NewRouteAnalyzer()

	tests := []struct {
		name        string
		fullName    string
		expected    string
		description string
	}{
		{
			name:        "Simple package.function",
			fullName:    "main.handleUser",
			expected:    "main",
			description: "Should extract package from simple format",
		},
		{
			name:        "Complex package path",
			fullName:    "github.com/user/repo/pkg.handleUser",
			expected:    "github.com/user/repo/pkg",
			description: "Should extract full package path",
		},
		{
			name:        "Nested package structure",
			fullName:    "github.com/pocketbase/pb-ext/core/server/api.discoveryTestHandler",
			expected:    "github.com/pocketbase/pb-ext/core/server/api",
			description: "Should extract nested package structure",
		},
		{
			name:        "Function without package",
			fullName:    "handleUser",
			expected:    "",
			description: "Should return empty string for function without package",
		},
		{
			name:        "Empty string",
			fullName:    "",
			expected:    "",
			description: "Should handle empty string gracefully",
		},
		{
			name:        "Multiple dots in function name",
			fullName:    "pkg.subpkg.Type.Method",
			expected:    "pkg.subpkg.Type",
			description: "Should extract package from method call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.extractPackageName(tt.fullName)

			if result != tt.expected {
				t.Errorf("Expected package name %q, got %q", tt.expected, result)
			}
		})
	}
}

// =============================================================================
// Handler Discovery Integration Tests
// =============================================================================

func TestHandlerDiscoveryWorkflow(t *testing.T) {
	analyzer := NewRouteAnalyzer()

	// Test complete workflow with different types of handlers
	handlers := []func(*core.RequestEvent) error{
		discoveryGetUsersHandler,
		discoveryCreateUserHandler,
		discoveryTestHandler,
	}

	var handlerInfos []*HandlerInfo

	for _, handler := range handlers {
		info := analyzer.AnalyzeHandler(handler)
		if info == nil {
			t.Error("Expected HandlerInfo but got nil")
			continue
		}
		handlerInfos = append(handlerInfos, info)
	}

	if len(handlerInfos) != len(handlers) {
		t.Errorf("Expected %d handler infos, got %d", len(handlers), len(handlerInfos))
	}

	// Verify all handlers have unique names
	nameSet := make(map[string]bool)
	for _, info := range handlerInfos {
		if nameSet[info.Name] {
			t.Errorf("Duplicate handler name found: %s", info.Name)
		}
		nameSet[info.Name] = true
	}

	// Verify all handlers have proper descriptions
	for _, info := range handlerInfos {
		if info.Description == "" {
			t.Errorf("Handler %s has empty description", info.Name)
		}
	}

	// Verify package extraction
	for _, info := range handlerInfos {
		if info.Package == "" {
			t.Errorf("Handler %s has empty package", info.Name)
		}
	}
}

func TestDiscoveryWithAnonymousHandlers(t *testing.T) {
	analyzer := NewRouteAnalyzer()

	// Test with various anonymous handler patterns
	anonymousHandlers := []func(*core.RequestEvent) error{
		func(c *core.RequestEvent) error { return nil },
		func(c *core.RequestEvent) error { return c.NoContent(204) },
	}

	for i, handler := range anonymousHandlers {
		t.Run("anonymous_"+string(rune(i+'0')), func(t *testing.T) {
			info := analyzer.AnalyzeHandler(handler)

			if info == nil {
				t.Fatal("Expected HandlerInfo but got nil")
			}

			// Anonymous functions should still be analyzed
			if info.Name == "" {
				t.Error("Expected non-empty name for anonymous handler")
			}
			if info.FullName == "" {
				t.Error("Expected non-empty full name for anonymous handler")
			}
			if info.Description == "" {
				t.Error("Expected non-empty description for anonymous handler")
			}
		})
	}
}

// =============================================================================
// Discovery Edge Cases
// =============================================================================

func TestDiscoveryEdgeCases(t *testing.T) {
	analyzer := NewRouteAnalyzer()

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "Multiple consecutive calls produce consistent results",
			test: func(t *testing.T) {
				handler := discoveryTestHandler

				info1 := analyzer.AnalyzeHandler(handler)
				info2 := analyzer.AnalyzeHandler(handler)

				// Should produce consistent results
				if info1.Name != info2.Name {
					t.Errorf("Expected consistent names: %q != %q", info1.Name, info2.Name)
				}
				if info1.FullName != info2.FullName {
					t.Errorf("Expected consistent full names: %q != %q", info1.FullName, info2.FullName)
				}
				if info1.Package != info2.Package {
					t.Errorf("Expected consistent packages: %q != %q", info1.Package, info2.Package)
				}
			},
		},
		{
			name: "Different handlers produce different results",
			test: func(t *testing.T) {
				info1 := analyzer.AnalyzeHandler(discoveryGetUsersHandler)
				info2 := analyzer.AnalyzeHandler(discoveryCreateUserHandler)

				if info1.Name == info2.Name {
					t.Error("Expected different handler names")
				}
				if info1.FullName == info2.FullName {
					t.Error("Expected different full handler names")
				}
				// Descriptions might be different based on handler name analysis
			},
		},
		{
			name: "Handler analysis consistency",
			test: func(t *testing.T) {
				handler := discoveryComplexPackageHandler
				info := analyzer.AnalyzeHandler(handler)

				// Verify that both Name and FullName are populated and reasonable
				if info.FullName == "" {
					t.Error("Expected non-empty FullName")
				}
				if info.Name == "" {
					t.Error("Expected non-empty Name")
				}
				// FullName should contain some form of the function name
				// (Note: Name might be cleaned with suffixes removed, so we just verify structure)
				if info.FullName != "" && !strings.Contains(info.FullName, "discoveryComplexPackage") {
					t.Errorf("Expected FullName %q to contain function identifier", info.FullName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestDiscoveryConcurrentAccess(t *testing.T) {
	analyzer := NewRouteAnalyzer()
	handler := discoveryTestHandler

	concurrency := 10
	done := make(chan bool, concurrency)
	results := make(chan *HandlerInfo, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() { done <- true }()
			info := analyzer.AnalyzeHandler(handler)
			results <- info
		}()
	}

	// Wait for all goroutines
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// Collect results
	close(results)
	var infos []*HandlerInfo
	for info := range results {
		infos = append(infos, info)
	}

	if len(infos) != concurrency {
		t.Errorf("Expected %d results, got %d", concurrency, len(infos))
	}

	// All results should be consistent
	if len(infos) > 0 {
		first := infos[0]
		for i, info := range infos {
			if info.Name != first.Name {
				t.Errorf("Result %d has different name: %q != %q", i, info.Name, first.Name)
			}
			if info.FullName != first.FullName {
				t.Errorf("Result %d has different full name: %q != %q", i, info.FullName, first.FullName)
			}
			if info.Package != first.Package {
				t.Errorf("Result %d has different package: %q != %q", i, info.Package, first.Package)
			}
		}
	}
}

// =============================================================================
// Performance Benchmarks
// =============================================================================

func BenchmarkRouteAnalyzerAnalyzeHandler(b *testing.B) {
	analyzer := NewRouteAnalyzer()
	handler := discoveryTestHandler

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeHandler(handler)
	}
}

func BenchmarkDiscoveryGetHandlerName(b *testing.B) {
	handler := discoveryTestHandler

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetHandlerName(handler)
	}
}

func BenchmarkDiscoveryExtractPackageName(b *testing.B) {
	analyzer := NewRouteAnalyzer()
	fullName := "github.com/pocketbase/pb-ext/core/server/api.discoveryTestHandler"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.extractPackageName(fullName)
	}
}

// =============================================================================
// Discovery Integration Test
// =============================================================================

func TestDiscoveryFullIntegration(t *testing.T) {
	// Test the full discovery workflow
	analyzer := NewRouteAnalyzer()

	// Simulate discovering handlers from different sources
	handlers := map[string]func(*core.RequestEvent) error{
		"getUsers":    discoveryGetUsersHandler,
		"createUser":  discoveryCreateUserHandler,
		"testHandler": discoveryTestHandler,
	}

	handlerInfoMap := make(map[string]*HandlerInfo)

	// Analyze all handlers
	for name, handler := range handlers {
		info := analyzer.AnalyzeHandler(handler)
		if info == nil {
			t.Errorf("Failed to analyze handler %s", name)
			continue
		}
		handlerInfoMap[name] = info
	}

	// Verify we analyzed all handlers
	if len(handlerInfoMap) != len(handlers) {
		t.Errorf("Expected %d analyzed handlers, got %d", len(handlers), len(handlerInfoMap))
	}

	// Verify each handler has complete information
	for name, info := range handlerInfoMap {
		if info.Name == "" {
			t.Errorf("Handler %s has empty Name", name)
		}
		if info.FullName == "" {
			t.Errorf("Handler %s has empty FullName", name)
		}
		if info.Package == "" {
			t.Errorf("Handler %s has empty Package", name)
		}
		if info.Description == "" {
			t.Errorf("Handler %s has empty Description", name)
		}
	}

	// Test that we can identify different types of operations
	expectedOperations := map[string]string{
		"getUsers":   "get",    // Should identify as a GET operation
		"createUser": "create", // Should identify as a CREATE operation
	}

	for handlerKey, expectedOp := range expectedOperations {
		if info, exists := handlerInfoMap[handlerKey]; exists {
			// Check if the description contains the expected operation type
			descLower := strings.ToLower(info.Description)
			if !strings.Contains(descLower, expectedOp) {
				t.Errorf("Handler %s description %q should contain operation %q",
					handlerKey, info.Description, expectedOp)
			}
		}
	}
}

// =============================================================================
// Examples
// =============================================================================

func ExampleRouteAnalyzer() {
	// Create a new route analyzer
	analyzer := NewRouteAnalyzer()

	// Define a handler function
	handler := func(c *core.RequestEvent) error {
		// Handle the request
		return nil
	}

	// Analyze the handler
	info := analyzer.AnalyzeHandler(handler)

	println("Handler Name:", info.Name)
	println("Package:", info.Package)
	println("Description:", info.Description)
}

func ExampleRouteAnalyzer_AnalyzeHandler() {
	analyzer := NewRouteAnalyzer()

	// Analyze a specific handler
	info := analyzer.AnalyzeHandler(discoveryTestHandler)

	println("Analyzed handler:")
	println("  Name:", info.Name)
	println("  Full Name:", info.FullName)
	println("  Package:", info.Package)
	println("  Description:", info.Description)

	// This information can be used for automatic API documentation
	if info.Name != "" {
		println("This handler can be documented automatically")
	}
}
