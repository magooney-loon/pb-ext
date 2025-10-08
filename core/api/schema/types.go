package schema

import (
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/magooney-loon/pb-ext/core/api"
)

// =============================================================================
// Schema Generator Implementation (simplified & OpenAPI compatible)
// =============================================================================

// Generator implements schema generation for OpenAPI 3.0
type Generator struct {
	mu      sync.RWMutex
	cache   map[string]*api.SchemaInfo
	structs map[string]api.StructInfo
	config  *Config
}

// Config contains schema generation configuration
type Config struct {
	CacheEnabled    bool          `json:"cache_enabled"`
	CacheTTL        time.Duration `json:"cache_ttl"`
	IncludeExamples bool          `json:"include_examples"`
	StrictMode      bool          `json:"strict_mode"`
	TypeMappings    TypeMappings  `json:"type_mappings"`
}

// TypeMappings defines Go type to JSON Schema type mappings
type TypeMappings map[string]SchemaType

// SchemaType represents JSON Schema types
type SchemaType struct {
	Type   string `json:"type"`
	Format string `json:"format,omitempty"`
}

// =============================================================================
// Schema Analysis Results
// =============================================================================

// AnalysisResult contains schema analysis results
type AnalysisResult struct {
	Schema      *api.SchemaInfo        `json:"schema"`
	Errors      []string               `json:"errors,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Stats       *AnalysisStats         `json:"stats,omitempty"`
	Generated   time.Time              `json:"generated"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AnalysisStats contains schema statistics
type AnalysisStats struct {
	PropertyCount   int `json:"property_count"`
	RequiredCount   int `json:"required_count"`
	OptionalCount   int `json:"optional_count"`
	NestedCount     int `json:"nested_count"`
	ArrayCount      int `json:"array_count"`
	MaxNestingLevel int `json:"max_nesting_level"`
	ComplexityScore int `json:"complexity_score"`
}

// =============================================================================
// Go Type Analysis
// =============================================================================

// FieldAnalysis contains field analysis results
type FieldAnalysis struct {
	Field       api.FieldInfo     `json:"field"`
	Schema      *api.SchemaInfo   `json:"schema"`
	Validations []ValidationRule  `json:"validations,omitempty"`
	Tags        map[string]string `json:"tags"`
	Errors      []string          `json:"errors,omitempty"`
}

// ValidationRule represents a field validation rule
type ValidationRule struct {
	Type        string      `json:"type"`        // "required", "min", "max", "pattern", etc.
	Value       interface{} `json:"value"`       // rule value
	Message     string      `json:"message"`     // validation message
	Conditional bool        `json:"conditional"` // if validation is conditional
}

// StructAnalysis contains complete struct analysis
type StructAnalysis struct {
	Struct       api.StructInfo         `json:"struct"`
	Schema       *api.SchemaInfo        `json:"schema"`
	Fields       []FieldAnalysis        `json:"fields"`
	Dependencies []string               `json:"dependencies"` // other struct dependencies
	Errors       []string               `json:"errors,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// =============================================================================
// Handler Schema Analysis
// =============================================================================

// HandlerAnalysis contains handler request/response analysis
type HandlerAnalysis struct {
	Handler        api.HandlerInfo `json:"handler"`
	RequestSchema  *api.SchemaInfo `json:"request_schema,omitempty"`
	ResponseSchema *api.SchemaInfo `json:"response_schema,omitempty"`
	Parameters     []ParameterInfo `json:"parameters,omitempty"`
	Errors         []string        `json:"errors,omitempty"`
	Generated      time.Time       `json:"generated"`
}

// ParameterInfo describes handler parameters
type ParameterInfo struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	In          string          `json:"in"` // "path", "query", "header", "body"
	Required    bool            `json:"required"`
	Schema      *api.SchemaInfo `json:"schema,omitempty"`
	Description string          `json:"description,omitempty"`
}

// =============================================================================
// Schema Utilities
// =============================================================================

// SchemaBuilder provides fluent API for building schemas
type SchemaBuilder struct {
	schema *api.SchemaInfo
}

// TypeResolver resolves Go types to JSON Schema types
type TypeResolver struct {
	mappings TypeMappings
}

// ValidationExtractor extracts validation rules from struct tags
type ValidationExtractor struct {
	supportedTags []string
}

// =============================================================================
// Component Schema Management
// =============================================================================

// ComponentManager manages reusable schema components
type ComponentManager struct {
	mu         sync.RWMutex
	components map[string]*api.SchemaInfo
	references map[string][]string // tracks which schemas reference others
}

// ComponentInfo contains component metadata
type ComponentInfo struct {
	Name         string            `json:"name"`
	Schema       *api.SchemaInfo   `json:"schema"`
	References   []string          `json:"references"`    // schemas this component references
	ReferencedBy []string          `json:"referenced_by"` // schemas that reference this component
	Version      string            `json:"version"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// =============================================================================
// Default Configurations
// =============================================================================

// DefaultConfig returns a default schema generator configuration
func DefaultConfig() *Config {
	return &Config{
		CacheEnabled:    true,
		CacheTTL:        30 * time.Minute,
		IncludeExamples: true,
		StrictMode:      false,
		TypeMappings:    DefaultTypeMappings(),
	}
}

// DefaultTypeMappings returns default Go type to JSON Schema mappings
func DefaultTypeMappings() TypeMappings {
	return TypeMappings{
		"string":      {Type: "string"},
		"int":         {Type: "integer", Format: "int32"},
		"int32":       {Type: "integer", Format: "int32"},
		"int64":       {Type: "integer", Format: "int64"},
		"uint":        {Type: "integer", Format: "int32"},
		"uint32":      {Type: "integer", Format: "int32"},
		"uint64":      {Type: "integer", Format: "int64"},
		"float32":     {Type: "number", Format: "float"},
		"float64":     {Type: "number", Format: "double"},
		"bool":        {Type: "boolean"},
		"time.Time":   {Type: "string", Format: "date-time"},
		"[]byte":      {Type: "string", Format: "byte"},
		"interface{}": {Type: "object"},
	}
}

// =============================================================================
// Common Schema Patterns
// =============================================================================

// CommonSchemas provides frequently used schema patterns
var CommonSchemas = map[string]*api.SchemaInfo{
	"Error": {
		Type:        "object",
		Description: "Standard error response",
		Properties: map[string]*api.SchemaInfo{
			"code": {
				Type:        "integer",
				Description: "Error code",
			},
			"message": {
				Type:        "string",
				Description: "Error message",
			},
			"data": {
				Type:        "object",
				Description: "Additional error data",
			},
		},
		Required: []string{"code", "message"},
	},
	"Success": {
		Type:        "object",
		Description: "Standard success response",
		Properties: map[string]*api.SchemaInfo{
			"data": {
				Type:        "object",
				Description: "Response data",
			},
		},
	},
	"PaginatedResponse": {
		Type:        "object",
		Description: "Paginated response wrapper",
		Properties: map[string]*api.SchemaInfo{
			"page": {
				Type:        "integer",
				Description: "Current page number",
				Example:     1,
			},
			"perPage": {
				Type:        "integer",
				Description: "Items per page",
				Example:     30,
			},
			"totalItems": {
				Type:        "integer",
				Description: "Total number of items",
			},
			"totalPages": {
				Type:        "integer",
				Description: "Total number of pages",
			},
			"items": {
				Type:        "array",
				Description: "Array of items",
				Items: &api.SchemaInfo{
					Type: "object",
				},
			},
		},
		Required: []string{"page", "perPage", "totalItems", "totalPages", "items"},
	},
}

// =============================================================================
// JSON Schema Validation Keywords
// =============================================================================

// ValidationKeywords contains supported JSON Schema validation keywords
var ValidationKeywords = map[string]bool{
	"required":      true,
	"minLength":     true,
	"maxLength":     true,
	"minimum":       true,
	"maximum":       true,
	"pattern":       true,
	"format":        true,
	"minItems":      true,
	"maxItems":      true,
	"uniqueItems":   true,
	"minProperties": true,
	"maxProperties": true,
	"enum":          true,
	"const":         true,
	"multipleOf":    true,
	"exclusiveMin":  true,
	"exclusiveMax":  true,
}

// =============================================================================
// Error Types
// =============================================================================

// Error represents schema generation errors
type Error struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Path    string    `json:"path,omitempty"`
	Details string    `json:"details,omitempty"`
	Time    time.Time `json:"time"`
}

// Common error codes
const (
	ErrTypeNotFound     = "TYPE_NOT_FOUND"
	ErrInvalidType      = "INVALID_TYPE"
	ErrCircularRef      = "CIRCULAR_REFERENCE"
	ErrUnsupportedType  = "UNSUPPORTED_TYPE"
	ErrInvalidTag       = "INVALID_TAG"
	ErrSchemaValidation = "SCHEMA_VALIDATION"
	ErrGenerationFailed = "GENERATION_FAILED"
)

// =============================================================================
// Utility Functions
// =============================================================================

// IsBasicType checks if a type is a basic JSON Schema type
func IsBasicType(typeName string) bool {
	basicTypes := []string{"string", "number", "integer", "boolean", "array", "object", "null"}
	for _, t := range basicTypes {
		if t == typeName {
			return true
		}
	}
	return false
}

// GetFieldJSONName extracts JSON field name from struct field
func GetFieldJSONName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return field.Name
	}

	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return ""
	}
	if parts[0] == "" {
		return field.Name
	}
	return parts[0]
}

// IsFieldRequired checks if struct field is required based on tags
func IsFieldRequired(field reflect.StructField) bool {
	// Check validate tag for required
	validateTag := field.Tag.Get("validate")
	if strings.Contains(validateTag, "required") {
		return true
	}

	// Check json tag for omitempty (opposite logic)
	jsonTag := field.Tag.Get("json")
	return !strings.Contains(jsonTag, "omitempty")
}

// GetFieldDescription extracts field description from tags or comments
func GetFieldDescription(field reflect.StructField) string {
	// Try description tag first
	if desc := field.Tag.Get("description"); desc != "" {
		return desc
	}

	// Try comment tag
	if comment := field.Tag.Get("comment"); comment != "" {
		return comment
	}

	// Try doc tag
	if doc := field.Tag.Get("doc"); doc != "" {
		return doc
	}

	return ""
}
