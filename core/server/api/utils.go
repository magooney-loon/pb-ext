package api

import (
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Core String Manipulation Utilities
// =============================================================================

// CleanTypeName cleans and normalizes type names for consistent usage
func CleanTypeName(typeName string) string {
	if typeName == "" {
		return ""
	}

	// Remove pointer indicators
	typeName = strings.TrimPrefix(typeName, "*")

	// Remove package prefixes but keep the last part
	parts := strings.Split(typeName, ".")
	if len(parts) > 1 {
		typeName = parts[len(parts)-1]
	}

	// Remove array/slice indicators
	typeName = strings.TrimPrefix(typeName, "[]")

	// Remove map indicators
	if strings.HasPrefix(typeName, "map[") {
		if idx := strings.Index(typeName, "]"); idx != -1 && idx+1 < len(typeName) {
			typeName = typeName[idx+1:]
		}
	}

	return typeName
}

// CamelCaseToSnakeCase converts camelCase to snake_case
func CamelCaseToSnakeCase(str string) string {
	if str == "" {
		return ""
	}

	var result strings.Builder
	for i, r := range str {
		if unicode.IsUpper(r) && i > 0 {
			result.WriteRune('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}

	return result.String()
}

// SnakeCaseToKebabCase converts snake_case to kebab-case
func SnakeCaseToKebabCase(str string) string {
	return strings.ReplaceAll(str, "_", "-")
}

// NormalizePathSegment normalizes a path segment for consistent usage
func NormalizePathSegment(segment string) string {
	if segment == "" {
		return ""
	}

	// Remove parameter indicators
	segment = strings.TrimPrefix(segment, ":")
	if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
		segment = segment[1 : len(segment)-1]
	}

	// Convert to lowercase and replace underscores
	segment = strings.ToLower(segment)
	segment = strings.ReplaceAll(segment, "_", "-")

	return segment
}

// =============================================================================
// Consolidated Handler Analysis
// =============================================================================

// GetHandlerName extracts the name of a handler function using reflection
func GetHandlerName(handler interface{}) string {
	if handler == nil {
		return "unknown"
	}

	// Handle function types
	if fn, ok := handler.(func(*core.RequestEvent) error); ok {
		return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	}

	// Handle other function types through reflection
	v := reflect.ValueOf(handler)
	if v.Kind() == reflect.Func {
		return runtime.FuncForPC(v.Pointer()).Name()
	}

	// Fallback to type name
	return reflect.TypeOf(handler).String()
}

// ExtractHandlerBaseName extracts a clean base name from handler function
// Consolidates ExtractBaseNameFromHandler and ExtractHandlerNameFromPath
func ExtractHandlerBaseName(handlerName string, stripSuffixes bool) string {
	if handlerName == "" {
		return ""
	}

	// Remove package path
	parts := strings.Split(handlerName, ".")
	if len(parts) > 0 {
		handlerName = parts[len(parts)-1]
	}

	// Optionally remove common suffixes
	if stripSuffixes {
		suffixes := []string{"Handler", "Func", "API", "Endpoint"}
		for _, suffix := range suffixes {
			if strings.HasSuffix(handlerName, suffix) {
				handlerName = strings.TrimSuffix(handlerName, suffix)
				break
			}
		}
	}

	return handlerName
}

// AnalyzeHandler provides comprehensive handler analysis
func AnalyzeHandler(handler interface{}) *HandlerInfo {
	if handler == nil {
		return &HandlerInfo{
			Name:        "unknown",
			Package:     "",
			Description: "Unknown handler",
		}
	}

	fullName := GetHandlerName(handler)
	baseName := ExtractHandlerBaseName(fullName, true)
	packageName := extractPackageName(fullName)

	return &HandlerInfo{
		Name:        baseName,
		Package:     packageName,
		FullName:    fullName,
		Description: GenerateDescription("", "", baseName),
	}
}

// =============================================================================
// Consolidated Description Generation
// =============================================================================

// GenerateDescription generates descriptions from various sources
// Consolidates DescriptionFromHandlerName, DescriptionFromPath, GenerateAPIDescription
func GenerateDescription(method, path, handlerName string) string {
	// Priority order: handler name -> path -> fallback

	// Try handler-based description first
	if handlerName != "" {
		if desc := descriptionFromHandler(handlerName); desc != "" {
			return desc
		}
	}

	// Try path-based description
	if method != "" && path != "" {
		return descriptionFromPath(method, path)
	}

	// Fallback
	if handlerName != "" {
		return strings.Title(strings.ToLower(handlerName))
	}

	return "API Endpoint"
}

// descriptionFromHandler generates a description from handler name
func descriptionFromHandler(handlerName string) string {
	if handlerName == "" {
		return ""
	}

	// Clean the handler name
	cleanName := ExtractHandlerBaseName(handlerName, true)
	if cleanName == "" {
		return ""
	}

	// Convert camelCase to space-separated words
	words := camelCaseToWords(cleanName)
	if len(words) == 0 {
		return cleanName
	}

	// Capitalize first word and join
	words[0] = strings.Title(words[0])
	return strings.Join(words, " ")
}

// descriptionFromPath generates a description from HTTP method and path
func descriptionFromPath(method, path string) string {
	if path == "" {
		return ""
	}

	// Extract meaningful parts from path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var meaningfulParts []string

	for _, part := range parts {
		if part != "" && !isPathParameter(part) {
			// Convert underscores to spaces and clean up
			cleaned := strings.ReplaceAll(part, "_", " ")
			cleaned = strings.Title(cleaned)
			meaningfulParts = append(meaningfulParts, cleaned)
		}
	}

	if len(meaningfulParts) == 0 {
		meaningfulParts = append(meaningfulParts, "Resource")
	}

	resource := strings.Join(meaningfulParts, " ")

	// Generate description based on HTTP method
	switch strings.ToUpper(method) {
	case "GET":
		if strings.Contains(strings.ToLower(path), "list") || strings.HasSuffix(path, "s") {
			return fmt.Sprintf("List %s", resource)
		}
		return fmt.Sprintf("Get %s", resource)
	case "POST":
		return fmt.Sprintf("Create %s", resource)
	case "PUT":
		return fmt.Sprintf("Update %s", resource)
	case "PATCH":
		return fmt.Sprintf("Partially Update %s", resource)
	case "DELETE":
		return fmt.Sprintf("Delete %s", resource)
	default:
		return fmt.Sprintf("%s %s", strings.Title(strings.ToLower(method)), resource)
	}
}

// =============================================================================
// Consolidated Tag Generation
// =============================================================================

// GenerateTags generates tags from multiple sources
// Consolidates GenerateTags, generateTagsFromPath, generateTagsFromHandler
func GenerateTags(method, path, handlerName string) []string {
	var tags []string
	tagSet := make(map[string]bool) // To avoid duplicates

	// Extract tags from path
	if path != "" {
		pathTags := extractTagsFromPath(path)
		for _, tag := range pathTags {
			if !tagSet[tag] && tag != "" {
				tags = append(tags, tag)
				tagSet[tag] = true
			}
		}
	}

	// Extract tags from handler name
	if handlerName != "" {
		handlerTags := extractTagsFromHandler(handlerName)
		for _, tag := range handlerTags {
			if !tagSet[tag] && tag != "" {
				tags = append(tags, tag)
				tagSet[tag] = true
			}
		}
	}

	// Add method-based tag
	if method != "" {
		methodTag := strings.ToLower(method)
		if !tagSet[methodTag] {
			tags = append(tags, methodTag)
			tagSet[methodTag] = true
		}
	}

	// Ensure we have at least one tag
	if len(tags) == 0 {
		tags = append(tags, "general")
	}

	return tags
}

// extractTagsFromPath extracts tags from URL path
func extractTagsFromPath(path string) []string {
	var tags []string
	parts := strings.Split(strings.Trim(path, "/"), "/")

	for _, part := range parts {
		if part != "" && !isPathParameter(part) {
			tag := NormalizePathSegment(part)
			if tag != "" && len(tag) > 1 { // Skip single characters
				tags = append(tags, tag)
			}
		}
	}

	return tags
}

// extractTagsFromHandler extracts tags from handler name
func extractTagsFromHandler(handlerName string) []string {
	var tags []string

	baseName := ExtractHandlerBaseName(handlerName, true)
	if baseName == "" {
		return tags
	}

	// Convert to snake_case and split
	snakeCase := CamelCaseToSnakeCase(baseName)
	parts := strings.Split(snakeCase, "_")

	for _, part := range parts {
		if part != "" && len(part) > 1 { // Skip single characters
			tag := strings.ToLower(part)
			tags = append(tags, tag)
		}
	}

	return tags
}

// =============================================================================
// Format Conversion Utilities
// =============================================================================

// ConvertToOpenAPIMethod converts HTTP method to OpenAPI format
func ConvertToOpenAPIMethod(method string) string {
	return strings.ToLower(method)
}

// ConvertToOpenAPIPath converts path to OpenAPI format
func ConvertToOpenAPIPath(path string) string {
	// Convert PocketBase style parameters (:param) to OpenAPI style ({param})
	re := regexp.MustCompile(`:([a-zA-Z_][a-zA-Z0-9_]*)`)
	return re.ReplaceAllString(path, "{$1}")
}

// FormatStatusCode formats an HTTP status code with description
func FormatStatusCode(code int) string {
	descriptions := map[int]string{
		200: "OK",
		201: "Created",
		204: "No Content",
		400: "Bad Request",
		401: "Unauthorized",
		403: "Forbidden",
		404: "Not Found",
		409: "Conflict",
		422: "Unprocessable Entity",
		500: "Internal Server Error",
	}

	if desc, exists := descriptions[code]; exists {
		return fmt.Sprintf("%d %s", code, desc)
	}
	return strconv.Itoa(code)
}

// =============================================================================
// Validation Utilities
// =============================================================================

// ValidateEndpoint performs basic validation on an API endpoint
func ValidateEndpoint(endpoint *APIEndpoint) []string {
	var errors []string

	if endpoint.Method == "" {
		errors = append(errors, "method is required")
	}

	if endpoint.Path == "" {
		errors = append(errors, "path is required")
	}

	// Validate HTTP method
	validMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	methodValid := false
	for _, validMethod := range validMethods {
		if strings.ToUpper(endpoint.Method) == validMethod {
			methodValid = true
			break
		}
	}
	if !methodValid {
		errors = append(errors, fmt.Sprintf("invalid HTTP method: %s", endpoint.Method))
	}

	// Validate path format
	if !strings.HasPrefix(endpoint.Path, "/") {
		errors = append(errors, "path must start with /")
	}

	return errors
}

// ValidateAuthInfo validates authentication information
func ValidateAuthInfo(auth *AuthInfo) []string {
	var errors []string

	if auth == nil {
		return errors
	}

	validTypes := []string{"guest_only", "auth", "superuser", "superuser_or_owner"}
	typeValid := false
	for _, validType := range validTypes {
		if auth.Type == validType {
			typeValid = true
			break
		}
	}
	if !typeValid && auth.Type != "" {
		errors = append(errors, fmt.Sprintf("invalid auth type: %s", auth.Type))
	}

	return errors
}

// =============================================================================
// Private Helper Functions
// =============================================================================

// extractPackageName extracts the package name from a full function name
func extractPackageName(fullName string) string {
	parts := strings.Split(fullName, ".")
	if len(parts) > 1 {
		// Return the package part, excluding the function name
		return strings.Join(parts[:len(parts)-1], ".")
	}
	return ""
}

// camelCaseToWords splits camelCase strings into separate words
func camelCaseToWords(str string) []string {
	if str == "" {
		return nil
	}

	var words []string
	var currentWord strings.Builder

	for i, r := range str {
		if unicode.IsUpper(r) && i > 0 {
			if word := currentWord.String(); word != "" {
				words = append(words, word)
			}
			currentWord.Reset()
		}
		currentWord.WriteRune(r)
	}

	if word := currentWord.String(); word != "" {
		words = append(words, word)
	}

	return words
}

// isPathParameter checks if a path segment is a parameter
func isPathParameter(segment string) bool {
	return strings.HasPrefix(segment, ":") ||
		(strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}"))
}

// sanitizeString sanitizes a string for use in various contexts
func sanitizeString(str string) string {
	// Remove special characters and normalize whitespace
	re := regexp.MustCompile(`[^\w\s-]`)
	str = re.ReplaceAllString(str, "")

	// Normalize whitespace
	re = regexp.MustCompile(`\s+`)
	str = re.ReplaceAllString(str, " ")

	return strings.TrimSpace(str)
}
