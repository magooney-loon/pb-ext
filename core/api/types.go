package api

import (
	"sync"
	"time"
)

// =============================================================================
// OpenAPI 3.0 Compatible Types
// =============================================================================

// OpenAPISpec represents a complete OpenAPI 3.0 specification
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"` // Always "3.0.3"
	Info       OpenAPIInfo            `json:"info"`
	Servers    []OpenAPIServer        `json:"servers,omitempty"`
	Paths      map[string]OpenAPIPath `json:"paths"`
	Components *OpenAPIComponents     `json:"components,omitempty"`
	Security   []OpenAPISecurity      `json:"security,omitempty"`
	Tags       []OpenAPITag           `json:"tags,omitempty"`
}

// OpenAPIInfo contains API metadata
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
	Contact     *struct {
		Name  string `json:"name,omitempty"`
		URL   string `json:"url,omitempty"`
		Email string `json:"email,omitempty"`
	} `json:"contact,omitempty"`
}

// OpenAPIServer describes API servers
type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// OpenAPIPath describes operations available on a single path
type OpenAPIPath struct {
	Get    *OpenAPIOperation `json:"get,omitempty"`
	Post   *OpenAPIOperation `json:"post,omitempty"`
	Put    *OpenAPIOperation `json:"put,omitempty"`
	Delete *OpenAPIOperation `json:"delete,omitempty"`
	Patch  *OpenAPIOperation `json:"patch,omitempty"`
}

// OpenAPIOperation describes a single API operation on a path
type OpenAPIOperation struct {
	Summary     string                     `json:"summary,omitempty"`
	Description string                     `json:"description,omitempty"`
	Tags        []string                   `json:"tags,omitempty"`
	OperationID string                     `json:"operationId,omitempty"`
	Parameters  []OpenAPIParameter         `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse `json:"responses"`
	Security    []OpenAPISecurity          `json:"security,omitempty"`
}

// OpenAPIParameter describes a single operation parameter
type OpenAPIParameter struct {
	Name        string         `json:"name"`
	In          string         `json:"in"` // "query", "header", "path", "cookie"
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Schema      *OpenAPISchema `json:"schema,omitempty"`
}

// OpenAPIRequestBody describes a request body
type OpenAPIRequestBody struct {
	Description string                      `json:"description,omitempty"`
	Content     map[string]OpenAPIMediaType `json:"content"`
	Required    bool                        `json:"required,omitempty"`
}

// OpenAPIResponse describes a single response from an API Operation
type OpenAPIResponse struct {
	Description string                      `json:"description"`
	Headers     map[string]OpenAPIHeader    `json:"headers,omitempty"`
	Content     map[string]OpenAPIMediaType `json:"content,omitempty"`
}

// OpenAPIMediaType provides schema and examples for the media type
type OpenAPIMediaType struct {
	Schema   *OpenAPISchema            `json:"schema,omitempty"`
	Example  interface{}               `json:"example,omitempty"`
	Examples map[string]OpenAPIExample `json:"examples,omitempty"`
}

// OpenAPISchema represents a JSON Schema (OpenAPI 3.0 compatible subset)
type OpenAPISchema struct {
	Type        string                    `json:"type,omitempty"`
	Format      string                    `json:"format,omitempty"`
	Title       string                    `json:"title,omitempty"`
	Description string                    `json:"description,omitempty"`
	Properties  map[string]*OpenAPISchema `json:"properties,omitempty"`
	Items       *OpenAPISchema            `json:"items,omitempty"`
	Required    []string                  `json:"required,omitempty"`
	Enum        []interface{}             `json:"enum,omitempty"`
	Example     interface{}               `json:"example,omitempty"`
	Default     interface{}               `json:"default,omitempty"`
	Ref         string                    `json:"$ref,omitempty"`
}

// OpenAPIComponents holds reusable objects for different aspects of the OAS
type OpenAPIComponents struct {
	Schemas         map[string]*OpenAPISchema     `json:"schemas,omitempty"`
	Responses       map[string]OpenAPIResponse    `json:"responses,omitempty"`
	Parameters      map[string]OpenAPIParameter   `json:"parameters,omitempty"`
	RequestBodies   map[string]OpenAPIRequestBody `json:"requestBodies,omitempty"`
	Headers         map[string]OpenAPIHeader      `json:"headers,omitempty"`
	SecuritySchemes map[string]OpenAPISecurity    `json:"securitySchemes,omitempty"`
}

// OpenAPIHeader represents a header parameter
type OpenAPIHeader struct {
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Schema      *OpenAPISchema `json:"schema,omitempty"`
}

// OpenAPIExample represents an example value
type OpenAPIExample struct {
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Value       interface{} `json:"value,omitempty"`
}

// OpenAPISecurity represents security scheme
type OpenAPISecurity map[string][]string

// OpenAPITag represents a tag for operations
type OpenAPITag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// =============================================================================
// Core API Types (simplified from old system)
// =============================================================================

// APIEndpoint represents a discovered API endpoint
type APIEndpoint struct {
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Auth        *AuthInfo   `json:"auth,omitempty"`
	Handler     string      `json:"handler,omitempty"`
	Request     *SchemaInfo `json:"request,omitempty"`
	Response    *SchemaInfo `json:"response,omitempty"`
}

// AuthInfo represents authentication requirements
type AuthInfo struct {
	Type        string   `json:"type"` // "none", "bearer", "admin", "owner"
	Required    bool     `json:"required"`
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
}

// SchemaInfo represents request/response schema information
type SchemaInfo struct {
	Type        string                 `json:"type,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Properties  map[string]*SchemaInfo `json:"properties,omitempty"`
	Items       *SchemaInfo            `json:"items,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Example     interface{}            `json:"example,omitempty"`
	Description string                 `json:"description,omitempty"`
}

// =============================================================================
// Version Management
// =============================================================================

// VersionManager manages multiple API versions
type VersionManager struct {
	mu             sync.RWMutex
	versions       []string
	defaultVersion string
	registries     map[string]*Registry
	configs        map[string]*Config
}

// VersionInfo contains version metadata
type VersionInfo struct {
	Version     string    `json:"version"`
	Status      string    `json:"status"` // "stable", "beta", "deprecated"
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Endpoints   int       `json:"endpoints"`
	Description string    `json:"description,omitempty"`
}

// Registry manages API endpoints for a single version
type Registry struct {
	mu        sync.RWMutex
	config    *Config
	endpoints map[string]APIEndpoint // key: method:path
	schemas   map[string]*SchemaInfo
}

// Config contains API configuration
type Config struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
	BaseURL     string `json:"base_url"`
	Enabled     bool   `json:"enabled"`
}

// =============================================================================
// Discovery & Generation Interfaces
// =============================================================================

// ASTParser interface for Go code analysis
type ASTParser interface {
	ParseFile(filename string) error
	GetHandlers() []HandlerInfo
	GetStructs() []StructInfo
}

// HandlerInfo represents a discovered handler function
type HandlerInfo struct {
	Name        string
	Package     string
	File        string
	Method      string
	Path        string
	Description string
	Tags        []string
	Auth        *AuthInfo
}

// StructInfo represents a Go struct for schema generation
type StructInfo struct {
	Name        string
	Package     string
	Fields      []FieldInfo
	Description string
}

// FieldInfo represents a struct field
type FieldInfo struct {
	Name        string
	Type        string
	JSONTag     string
	Required    bool
	Description string
}

// SchemaGenerator interface for schema generation
type SchemaGenerator interface {
	GenerateSchema(structName string) (*SchemaInfo, error)
	GenerateRequestSchema(handlerName string) (*SchemaInfo, error)
	GenerateResponseSchema(handlerName string) (*SchemaInfo, error)
}

// DiscoveryEngine interface for automatic discovery
type DiscoveryEngine interface {
	DiscoverEndpoints(packagePath string) ([]APIEndpoint, error)
	DiscoverSchemas(packagePath string) (map[string]*SchemaInfo, error)
}

// OpenAPIGenerator interface for OpenAPI spec generation
type OpenAPIGenerator interface {
	GenerateSpec(endpoints []APIEndpoint, config *Config) (*OpenAPISpec, error)
	AddSecuritySchemes(spec *OpenAPISpec, authTypes []string)
	AddComponents(spec *OpenAPISpec, schemas map[string]*SchemaInfo)
}

// =============================================================================
// Auto-Discovery Configuration
// =============================================================================

// DiscoveryConfig configures automatic endpoint discovery
type DiscoveryConfig struct {
	Enabled         bool     `json:"enabled"`
	PackagePaths    []string `json:"package_paths"`
	IncludeInternal bool     `json:"include_internal"`
	DetectAuth      bool     `json:"detect_auth"`
	GenerateTags    bool     `json:"generate_tags"`
	AnalyzeSchemas  bool     `json:"analyze_schemas"`
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		Title:       "pb-ext API",
		Version:     "1.0.0",
		Description: "Auto-discovered API endpoints",
		BaseURL:     "/api",
		Enabled:     true,
	}
}

// DefaultDiscoveryConfig returns a default discovery configuration
func DefaultDiscoveryConfig() *DiscoveryConfig {
	return &DiscoveryConfig{
		Enabled:         true,
		PackagePaths:    []string{"./"},
		IncludeInternal: false,
		DetectAuth:      true,
		GenerateTags:    true,
		AnalyzeSchemas:  true,
	}
}

// =============================================================================
// Common Auth Types for PocketBase
// =============================================================================

var (
	// Common auth configurations for PocketBase patterns
	AuthNone = &AuthInfo{
		Type:        "none",
		Required:    false,
		Description: "No authentication required",
	}

	AuthBearer = &AuthInfo{
		Type:        "bearer",
		Required:    true,
		Description: "Requires valid auth token",
		Scopes:      []string{"authenticated"},
	}

	AuthAdmin = &AuthInfo{
		Type:        "bearer",
		Required:    true,
		Description: "Requires admin/superuser privileges",
		Scopes:      []string{"admin"},
	}

	AuthOwner = &AuthInfo{
		Type:        "bearer",
		Required:    true,
		Description: "Requires ownership or admin privileges",
		Scopes:      []string{"owner", "admin"},
	}
)

// =============================================================================
// Common Response Schemas
// =============================================================================

var (
	// Standard response schemas
	ErrorResponseSchema = &SchemaInfo{
		Type: "object",
		Properties: map[string]*SchemaInfo{
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
	}

	SuccessResponseSchema = &SchemaInfo{
		Type: "object",
		Properties: map[string]*SchemaInfo{
			"data": {
				Type:        "object",
				Description: "Response data",
			},
		},
	}
)
