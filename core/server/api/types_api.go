package api

import "sync"

// =============================================================================
// API Documentation Core Types
// =============================================================================

// APIEndpoint represents a single API endpoint documentation
type APIEndpoint struct {
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Description string         `json:"description"`
	Request     *OpenAPISchema `json:"request,omitempty"`
	Response    *OpenAPISchema `json:"response,omitempty"`
	Auth        *AuthInfo      `json:"auth,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Handler     string         `json:"handler_name,omitempty"`
	Parameters  []*ParamInfo   `json:"parameters,omitempty"`
}

// AuthInfo represents detailed authentication requirements for an API endpoint
type AuthInfo struct {
	Required    bool     `json:"required"`
	Type        string   `json:"type"`                  // "guest_only", "auth", "superuser", "superuser_or_owner"
	Collections []string `json:"collections,omitempty"` // For RequireAuth with specific collections
	OwnerParam  string   `json:"owner_param,omitempty"` // For RequireSuperuserOrOwnerAuth
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
}

// APIDocs holds all API documentation in OpenAPI 3.0 format
type APIDocs struct {
	OpenAPI      string                      `json:"openapi"`
	Info         *OpenAPIInfo                `json:"info"`
	Servers      []*OpenAPIServer            `json:"servers,omitempty"`
	Paths        map[string]*OpenAPIPathItem `json:"paths"`
	Components   *OpenAPIComponents          `json:"components,omitempty"`
	Security     []map[string][]string       `json:"security,omitempty"`
	Tags         []*OpenAPITag               `json:"tags,omitempty"`
	ExternalDocs *OpenAPIExternalDocs        `json:"externalDocs,omitempty"`

	// Internal fields (not serialized to JSON)
	endpoints []APIEndpoint `json:"-"`
	generated string        `json:"-"`
}

// HandlerInfo contains extracted handler information
type HandlerInfo struct {
	Name        string `json:"name"`
	Package     string `json:"package"`
	FullName    string `json:"full_name"`
	Description string `json:"description"`
}

// =============================================================================
// Configuration Types
// =============================================================================

// APIDocsConfig holds configuration for the API documentation system
type APIDocsConfig struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Status      string `json:"status,omitempty"` // "stable", "development", "deprecated", "beta", etc.
	BaseURL     string `json:"base_url"`
	Enabled     bool   `json:"enabled"`

	ContactName  string `json:"contact_name,omitempty"`
	ContactEmail string `json:"contact_email,omitempty"`
	ContactURL   string `json:"contact_url,omitempty"`

	LicenseName string `json:"license_name,omitempty"`
	LicenseURL  string `json:"license_url,omitempty"`

	TermsOfService   string `json:"terms_of_service,omitempty"`
	ExternalDocsURL  string `json:"external_docs_url,omitempty"`
	ExternalDocsDesc string `json:"external_docs_desc,omitempty"`

	PublicSwagger bool `json:"public_swagger,omitempty"`
}

// DefaultAPIDocsConfig returns a default configuration
func DefaultAPIDocsConfig() *APIDocsConfig {
	return &APIDocsConfig{
		Title:       "pb-ext API",
		Version:     "1.0.0",
		Description: "AST discovered API endpoints",
		Status:      "stable",
		BaseURL:     "/api",
		Enabled:     true,
	}
}

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

// SchemaConfig holds configuration for schema processing
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
