package api

import (
	"sync"
)

// =============================================================================
// Schema Generation Types
// =============================================================================

// SchemaGenerator handles all schema generation and analysis operations
type SchemaGenerator struct {
	mu            sync.RWMutex
	astParser     ASTParserInterface
	schemaBuilder *OpenAPISchemaBuilder
	components    *OpenAPIComponents
	validators    []TypeValidator
	logger        Logger
}

// SchemaAnalysisResult contains the result of schema analysis
type SchemaAnalysisResult struct {
	RequestSchema  *OpenAPISchema      `json:"request_schema,omitempty"`
	ResponseSchema *OpenAPISchema      `json:"response_schema,omitempty"`
	Parameters     []*OpenAPIParameter `json:"parameters,omitempty"`
	Errors         []error             `json:"errors,omitempty"`
	Warnings       []string            `json:"warnings,omitempty"`
}

// SchemaPattern represents a pattern for matching handlers/paths to generate schemas
type SchemaPattern struct {
	Name         string                `json:"name"`
	HandlerMatch func(string) bool     `json:"-"`
	PathMatch    func(string) bool     `json:"-"`
	RequestGen   func() *OpenAPISchema `json:"-"`
	ResponseGen  func() *OpenAPISchema `json:"-"`
}

// RequestResponseAnalyzer analyzes handler functions to extract request/response types
type RequestResponseAnalyzer struct {
	astParser ASTParserInterface
	logger    Logger
}

// TypeSchemaBuilder converts Go types to OpenAPI schema representations
type TypeSchemaBuilder struct {
	schemaBuilder *OpenAPISchemaBuilder
	logger        Logger
}

// ValidationExtractor extracts validation rules from struct tags and comments
type ValidationExtractor struct {
	logger Logger
}

// =============================================================================
// Interface Definitions
// =============================================================================

// SchemaGeneratorInterface defines the contract for schema generation operations
type SchemaGeneratorInterface interface {
	AnalyzeRequestSchema(endpoint *APIEndpoint) (*OpenAPISchema, error)
	AnalyzeResponseSchema(endpoint *APIEndpoint) (*OpenAPISchema, error)
	AnalyzeSchemaFromPath(method, path string) (*SchemaAnalysisResult, error)
	GenerateComponentSchemas() *OpenAPIComponents
	GetOpenAPIEndpointSchema(endpoint *APIEndpoint) (*OpenAPIEndpointSchema, error)
}

// TypeValidator interface for type validation
type TypeValidator interface {
	Validate(typeInfo *TypeInfo) error
	Name() string
}

// =============================================================================
// Configuration Types
// =============================================================================

// SchemaConfig holds configuration for schema processing and form generation
type SchemaConfig struct {
	SystemFields          []string          `json:"system_fields"`
	DefaultParameterType  string            `json:"default_parameter_type"`
	DefaultParameterIn    string            `json:"default_parameter_in"`
	DefaultContentType    string            `json:"default_content_type"`
	DescriptionTemplates  map[string]string `json:"description_templates"`
	SupportedContentTypes []string          `json:"supported_content_types"`
}

// DefaultSchemaConfig returns a default schema configuration
func DefaultSchemaConfig() *SchemaConfig {
	return &SchemaConfig{
		SystemFields:         []string{"id", "created_at", "updated_at"},
		DefaultParameterType: "string",
		DefaultParameterIn:   "query",
		DefaultContentType:   "application/json",
		DescriptionTemplates: map[string]string{
			"path":   "Path parameter: {name}",
			"query":  "Query parameter: {name}",
			"header": "Header parameter: {name}",
			"body":   "Request body parameter: {name}",
		},
		SupportedContentTypes: []string{
			"application/json",
			"application/x-www-form-urlencoded",
			"multipart/form-data",
			"text/plain",
		},
	}
}
