package api

import (
	"fmt"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Test Handler Functions
// =============================================================================

// Mock handler functions for testing
func testGetUsersHandler(c *core.RequestEvent) error {
	return nil
}

func testCreateUserHandler(c *core.RequestEvent) error {
	return nil
}

func testUpdateUserHandler(c *core.RequestEvent) error {
	return nil
}

func testDeleteUserHandler(c *core.RequestEvent) error {
	return nil
}

// Handler in a different package-like name
func github_com_test_api_getUsersHandler(c *core.RequestEvent) error {
	return nil
}

// Anonymous function for testing
var anonymousHandler = func(c *core.RequestEvent) error {
	return nil
}

// =============================================================================
// String Manipulation Tests
// =============================================================================

func TestCleanTypeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple type name",
			input:    "User",
			expected: "User",
		},
		{
			name:     "Pointer type",
			input:    "*User",
			expected: "User",
		},
		{
			name:     "Package qualified type",
			input:    "github.com/test/models.User",
			expected: "User",
		},
		{
			name:     "Slice type",
			input:    "[]User",
			expected: "User",
		},
		{
			name:     "Map type",
			input:    "map[string]User",
			expected: "User",
		},
		{
			name:     "Complex pointer slice type",
			input:    "*[]github.com/test/models.User",
			expected: "User",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Map with complex value",
			input:    "map[string]*github.com/test/models.User",
			expected: "User",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("CleanTypeName(%q) = %q; expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCamelCaseToSnakeCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple camelCase",
			input:    "userName",
			expected: "user_name",
		},
		{
			name:     "PascalCase",
			input:    "UserName",
			expected: "user_name",
		},
		{
			name:     "Multiple words",
			input:    "getUserProfile",
			expected: "get_user_profile",
		},
		{
			name:     "Already lowercase",
			input:    "user",
			expected: "user",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Single character",
			input:    "A",
			expected: "a",
		},
		{
			name:     "Consecutive capitals",
			input:    "XMLHttpRequest",
			expected: "x_m_l_http_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CamelCaseToSnakeCase(tt.input)
			if result != tt.expected {
				t.Errorf("CamelCaseToSnakeCase(%q) = %q; expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSnakeCaseToKebabCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple snake_case",
			input:    "user_name",
			expected: "user-name",
		},
		{
			name:     "Multiple underscores",
			input:    "get_user_profile",
			expected: "get-user-profile",
		},
		{
			name:     "Already kebab-case",
			input:    "user-name",
			expected: "user-name",
		},
		{
			name:     "No underscores",
			input:    "username",
			expected: "username",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Mixed case with underscores",
			input:    "User_Profile_Settings",
			expected: "User-Profile-Settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SnakeCaseToKebabCase(tt.input)
			if result != tt.expected {
				t.Errorf("SnakeCaseToKebabCase(%q) = %q; expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizePathSegment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple segment",
			input:    "users",
			expected: "users",
		},
		{
			name:     "Parameter with colon",
			input:    ":id",
			expected: "id",
		},
		{
			name:     "Parameter with braces",
			input:    "{id}",
			expected: "id",
		},
		{
			name:     "Uppercase segment",
			input:    "USERS",
			expected: "users",
		},
		{
			name:     "Segment with underscores",
			input:    "user_profiles",
			expected: "user-profiles",
		},
		{
			name:     "Mixed case with underscores",
			input:    "User_Profiles",
			expected: "user-profiles",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Complex parameter",
			input:    "{userId}",
			expected: "userid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePathSegment(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePathSegment(%q) = %q; expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Handler Analysis Tests
// =============================================================================

func TestGetHandlerName(t *testing.T) {
	tests := []struct {
		name      string
		handler   interface{}
		expectNil bool
	}{
		{
			name:      "Standard handler",
			handler:   testGetUsersHandler,
			expectNil: false,
		},
		{
			name:      "Another handler",
			handler:   testCreateUserHandler,
			expectNil: false,
		},
		{
			name:      "Nil handler",
			handler:   nil,
			expectNil: false, // GetHandlerName should handle nil gracefully
		},
		{
			name:      "Anonymous function",
			handler:   anonymousHandler,
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHandlerName(tt.handler)

			if tt.expectNil && result != "" {
				t.Error("Expected empty result but got handler name")
			}
			if !tt.expectNil && result == "" && tt.handler != nil {
				t.Error("Expected handler name but got empty string")
			}

			if tt.handler == nil && result != "unknown" {
				t.Errorf("Expected 'unknown' for nil handler, got %s", result)
			}
		})
	}
}

func TestExtractHandlerBaseName(t *testing.T) {
	tests := []struct {
		name        string
		fullName    string
		cleanSuffix bool
		expected    string
		description string
	}{
		{
			name:        "Handler with suffix - clean enabled",
			fullName:    "getUsersHandler",
			cleanSuffix: true,
			expected:    "getUsers",
			description: "Should remove Handler suffix",
		},
		{
			name:        "Handler with suffix - clean disabled",
			fullName:    "getUsersHandler",
			cleanSuffix: false,
			expected:    "getUsersHandler",
			description: "Should keep Handler suffix",
		},
		{
			name:        "Function without Handler suffix",
			fullName:    "getUsers",
			cleanSuffix: true,
			expected:    "getUsers",
			description: "Should not change function without suffix",
		},
		{
			name:        "Complex package path with handler",
			fullName:    "github.com/test/api.createUserHandler",
			cleanSuffix: true,
			expected:    "createUser",
			description: "Should extract base name and remove suffix",
		},
		{
			name:        "Empty string",
			fullName:    "",
			cleanSuffix: true,
			expected:    "",
			description: "Should handle empty string",
		},
		{
			name:        "Handler with Func suffix",
			fullName:    "getUserFunc",
			cleanSuffix: true,
			expected:    "getUser",
			description: "Should remove Func suffix",
		},
		{
			name:        "Handler with API suffix",
			fullName:    "getUserAPI",
			cleanSuffix: true,
			expected:    "getUser",
			description: "Should remove API suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractHandlerBaseName(tt.fullName, tt.cleanSuffix)

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestAnalyzeHandler(t *testing.T) {
	tests := []struct {
		name      string
		handler   interface{}
		expectNil bool
	}{
		{
			name:      "Valid handler",
			handler:   testGetUsersHandler,
			expectNil: false,
		},
		{
			name:      "Nil handler",
			handler:   nil,
			expectNil: false, // Should return default HandlerInfo, not nil
		},
		{
			name:      "Anonymous handler",
			handler:   anonymousHandler,
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeHandler(tt.handler)

			if tt.expectNil && result != nil {
				t.Error("Expected nil result but got HandlerInfo")
			}
			if !tt.expectNil && result == nil {
				t.Error("Expected HandlerInfo but got nil")
			}

			if result != nil {
				if result.Name == "" {
					t.Error("Expected non-empty Name field")
				}
				if result.Description == "" {
					t.Error("Expected non-empty Description field")
				}
				if result.FullName == "" && tt.handler != nil {
					t.Error("Expected non-empty FullName field for non-nil handler")
				}
			}
		})
	}
}

// =============================================================================
// Description Generation Tests
// =============================================================================

func TestGenerateDescription(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		handlerName string
		expected    string
		description string
	}{
		{
			name:        "From handler name - get operation",
			method:      "",
			path:        "",
			handlerName: "getUsers",
			expected:    "Get Users",
			description: "Should generate description from get handler",
		},
		{
			name:        "From handler name - create operation",
			method:      "",
			path:        "",
			handlerName: "createUser",
			expected:    "Create User",
			description: "Should generate description from create handler",
		},
		{
			name:        "From method and path",
			method:      "GET",
			path:        "/api/users",
			handlerName: "",
			expected:    "List Api Users",
			description: "Should generate description from method and path",
		},
		{
			name:        "POST method",
			method:      "POST",
			path:        "/api/users",
			handlerName: "",
			expected:    "Create Api Users",
			description: "Should generate CREATE description for POST",
		},
		{
			name:        "DELETE method",
			method:      "DELETE",
			path:        "/api/users/{id}",
			handlerName: "",
			expected:    "Delete Api Users",
			description: "Should generate DELETE description",
		},
		{
			name:        "Empty inputs fallback",
			method:      "",
			path:        "",
			handlerName: "",
			expected:    "API Endpoint",
			description: "Should provide fallback description",
		},
		{
			name:        "Handler name takes priority",
			method:      "GET",
			path:        "/api/users",
			handlerName: "getUserProfile",
			expected:    "Get User Profile",
			description: "Handler name should take priority over path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateDescription(tt.method, tt.path, tt.handlerName)

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// =============================================================================
// Tag Generation Tests
// =============================================================================

func TestGenerateTags(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		handlerName   string
		expectedMin   int // Minimum number of tags expected
		shouldContain []string
	}{
		{
			name:          "From path only",
			method:        "GET",
			path:          "/api/v1/users",
			handlerName:   "",
			expectedMin:   2,
			shouldContain: []string{"get", "users"},
		},
		{
			name:          "From handler name",
			method:        "POST",
			path:          "",
			handlerName:   "createUserProfile",
			expectedMin:   2,
			shouldContain: []string{"post", "create"},
		},
		{
			name:          "From all sources",
			method:        "PUT",
			path:          "/api/posts",
			handlerName:   "updatePost",
			expectedMin:   3,
			shouldContain: []string{"put", "posts", "update"},
		},
		{
			name:          "Empty inputs fallback",
			method:        "",
			path:          "",
			handlerName:   "",
			expectedMin:   1,
			shouldContain: []string{"general"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTags(tt.method, tt.path, tt.handlerName)

			if len(result) < tt.expectedMin {
				t.Errorf("Expected at least %d tags, got %d: %v", tt.expectedMin, len(result), result)
			}

			for _, expectedTag := range tt.shouldContain {
				found := false
				for _, actualTag := range result {
					if actualTag == expectedTag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected tag %q not found in result: %v", expectedTag, result)
				}
			}

			// Check for no duplicate tags
			tagSet := make(map[string]bool)
			for _, tag := range result {
				if tagSet[tag] {
					t.Errorf("Duplicate tag found: %q in %v", tag, result)
				}
				tagSet[tag] = true
			}
		})
	}
}

// =============================================================================
// Format Conversion Tests
// =============================================================================

func TestConvertToOpenAPIMethod(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{"GET", "GET", "get"},
		{"POST", "POST", "post"},
		{"PUT", "PUT", "put"},
		{"PATCH", "PATCH", "patch"},
		{"DELETE", "DELETE", "delete"},
		{"Lowercase", "get", "get"},
		{"Mixed case", "Post", "post"},
		{"Empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToOpenAPIMethod(tt.method)
			if result != tt.expected {
				t.Errorf("ConvertToOpenAPIMethod(%q) = %q; expected %q", tt.method, result, tt.expected)
			}
		})
	}
}

func TestConvertToOpenAPIPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "Simple parameter",
			path:     "/users/:id",
			expected: "/users/{id}",
		},
		{
			name:     "Multiple parameters",
			path:     "/users/:userId/posts/:postId",
			expected: "/users/{userId}/posts/{postId}",
		},
		{
			name:     "No parameters",
			path:     "/users/list",
			expected: "/users/list",
		},
		{
			name:     "Already OpenAPI format",
			path:     "/users/{id}",
			expected: "/users/{id}",
		},
		{
			name:     "Mixed parameters",
			path:     "/users/:id/posts/{postId}",
			expected: "/users/{id}/posts/{postId}",
		},
		{
			name:     "Empty path",
			path:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToOpenAPIPath(tt.path)
			if result != tt.expected {
				t.Errorf("ConvertToOpenAPIPath(%q) = %q; expected %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestFormatStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		expected string
	}{
		{"200 OK", 200, "200 OK"},
		{"201 Created", 201, "201 Created"},
		{"204 No Content", 204, "204 No Content"},
		{"400 Bad Request", 400, "400 Bad Request"},
		{"401 Unauthorized", 401, "401 Unauthorized"},
		{"403 Forbidden", 403, "403 Forbidden"},
		{"404 Not Found", 404, "404 Not Found"},
		{"409 Conflict", 409, "409 Conflict"},
		{"422 Unprocessable Entity", 422, "422 Unprocessable Entity"},
		{"500 Internal Server Error", 500, "500 Internal Server Error"},
		{"Unknown status code", 999, "999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatStatusCode(tt.code)
			if result != tt.expected {
				t.Errorf("FormatStatusCode(%d) = %q; expected %q", tt.code, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Validation Tests
// =============================================================================

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     *APIEndpoint
		expectErrors int
	}{
		{
			name: "Valid endpoint",
			endpoint: &APIEndpoint{
				Method: "GET",
				Path:   "/api/users",
			},
			expectErrors: 0,
		},
		{
			name: "Missing method",
			endpoint: &APIEndpoint{
				Path: "/api/users",
			},
			expectErrors: 1,
		},
		{
			name: "Missing path",
			endpoint: &APIEndpoint{
				Method: "GET",
			},
			expectErrors: 1,
		},
		{
			name: "Invalid method",
			endpoint: &APIEndpoint{
				Method: "INVALID",
				Path:   "/api/users",
			},
			expectErrors: 1,
		},
		{
			name: "Path without leading slash",
			endpoint: &APIEndpoint{
				Method: "GET",
				Path:   "api/users",
			},
			expectErrors: 1,
		},
		{
			name: "Multiple errors",
			endpoint: &APIEndpoint{
				Method: "INVALID",
				Path:   "api/users",
			},
			expectErrors: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateEndpoint(tt.endpoint)
			if len(errors) != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, len(errors), errors)
			}
		})
	}
}

func TestValidateAuthInfo(t *testing.T) {
	tests := []struct {
		name         string
		auth         *AuthInfo
		expectErrors int
	}{
		{
			name:         "Nil auth info",
			auth:         nil,
			expectErrors: 0,
		},
		{
			name: "Valid guest_only",
			auth: &AuthInfo{
				Type: "guest_only",
			},
			expectErrors: 0,
		},
		{
			name: "Valid auth",
			auth: &AuthInfo{
				Type: "auth",
			},
			expectErrors: 0,
		},
		{
			name: "Valid superuser",
			auth: &AuthInfo{
				Type: "superuser",
			},
			expectErrors: 0,
		},
		{
			name: "Valid superuser_or_owner",
			auth: &AuthInfo{
				Type: "superuser_or_owner",
			},
			expectErrors: 0,
		},
		{
			name: "Invalid auth type",
			auth: &AuthInfo{
				Type: "invalid",
			},
			expectErrors: 1,
		},
		{
			name: "Empty auth type",
			auth: &AuthInfo{
				Type: "",
			},
			expectErrors: 0, // Empty is allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateAuthInfo(tt.auth)
			if len(errors) != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, len(errors), errors)
			}
		})
	}
}

// =============================================================================
// Performance Benchmarks
// =============================================================================

func BenchmarkCleanTypeName(b *testing.B) {
	input := "*[]github.com/test/models.User"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CleanTypeName(input)
	}
}

func BenchmarkCamelCaseToSnakeCase(b *testing.B) {
	input := "getUserProfileInformation"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CamelCaseToSnakeCase(input)
	}
}

func BenchmarkGetHandlerName(b *testing.B) {
	handler := testGetUsersHandler
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetHandlerName(handler)
	}
}

func BenchmarkAnalyzeHandler(b *testing.B) {
	handler := testGetUsersHandler
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AnalyzeHandler(handler)
	}
}

func BenchmarkGenerateDescription(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateDescription("GET", "/api/users/{id}", "getUserHandler")
	}
}

func BenchmarkGenerateTags(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateTags("POST", "/api/v1/users", "createUserHandler")
	}
}

func BenchmarkValidateEndpoint(b *testing.B) {
	endpoint := &APIEndpoint{
		Method: "GET",
		Path:   "/api/users",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateEndpoint(endpoint)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestUtilitiesIntegration(t *testing.T) {
	// Test utilities working together in a realistic workflow

	// 1. Analyze a handler
	handler := testGetUsersHandler
	handlerInfo := AnalyzeHandler(handler)

	if handlerInfo == nil {
		t.Fatal("Expected HandlerInfo but got nil")
	}

	// 2. Generate description and tags
	description := GenerateDescription("GET", "/api/users", handlerInfo.Name)
	tags := GenerateTags("GET", "/api/users", handlerInfo.Name)

	if description == "" {
		t.Error("Expected non-empty description")
	}
	if len(tags) == 0 {
		t.Error("Expected at least one tag")
	}

	// 3. Create and validate an endpoint
	endpoint := &APIEndpoint{
		Method:      "GET",
		Path:        "/api/users",
		Handler:     handlerInfo.FullName,
		Description: description,
		Tags:        tags,
	}

	errors := ValidateEndpoint(endpoint)
	if len(errors) != 0 {
		t.Errorf("Expected valid endpoint but got errors: %v", errors)
	}

	// 4. Convert to OpenAPI format
	openAPIMethod := ConvertToOpenAPIMethod(endpoint.Method)
	openAPIPath := ConvertToOpenAPIPath(endpoint.Path)

	if openAPIMethod != "get" {
		t.Errorf("Expected OpenAPI method 'get', got %s", openAPIMethod)
	}
	if openAPIPath != "/api/users" {
		t.Errorf("Expected OpenAPI path '/api/users', got %s", openAPIPath)
	}

	// 5. Test type name cleaning workflow
	typeName := "*[]github.com/test/models.UserResponse"
	cleanedType := CleanTypeName(typeName)
	snakeCase := CamelCaseToSnakeCase(cleanedType)
	kebabCase := SnakeCaseToKebabCase(snakeCase)

	if cleanedType != "UserResponse" {
		t.Errorf("Expected cleaned type 'UserResponse', got %s", cleanedType)
	}
	if snakeCase != "user_response" {
		t.Errorf("Expected snake_case 'user_response', got %s", snakeCase)
	}
	if kebabCase != "user-response" {
		t.Errorf("Expected kebab-case 'user-response', got %s", kebabCase)
	}
}

// =============================================================================
// Examples
// =============================================================================

func ExampleCleanTypeName() {
	result := CleanTypeName("*[]github.com/test/models.User")
	fmt.Println(result)
	// Output: User
}

func ExampleCamelCaseToSnakeCase() {
	result := CamelCaseToSnakeCase("getUserProfile")
	fmt.Println(result)
	// Output: get_user_profile
}

func ExampleAnalyzeHandler() {
	handler := func(c *core.RequestEvent) error {
		return nil
	}

	info := AnalyzeHandler(handler)
	fmt.Println("Handler Name:", info.Name)
	fmt.Println("Description:", info.Description)
	// Output:
	// Handler Name: func1
	// Description: Func1
}

func ExampleGenerateDescription() {
	desc := GenerateDescription("GET", "/api/users/{id}", "getUserHandler")
	fmt.Println(desc)
	// Output: Get User
}

func ExampleGenerateTags() {
	tags := GenerateTags("POST", "/api/v1/users", "createUserHandler")
	for i, tag := range tags {
		fmt.Println("Tag", i, ":", tag)
	}
	// Output:
	// Tag 0 : api
	// Tag 1 : v1
	// Tag 2 : users
	// Tag 3 : create
	// Tag 4 : user
	// Tag 5 : post
}

func ExampleConvertToOpenAPIPath() {
	result := ConvertToOpenAPIPath("/users/:id/posts/:postId")
	fmt.Println(result)
	// Output: /users/{id}/posts/{postId}
}

func ExampleValidateEndpoint() {
	endpoint := &APIEndpoint{
		Method: "GET",
		Path:   "/api/users",
	}

	errors := ValidateEndpoint(endpoint)
	if len(errors) == 0 {
		fmt.Println("Endpoint is valid")
	} else {
		for _, err := range errors {
			fmt.Println("Error:", err)
		}
	}
	// Output: Endpoint is valid
}
