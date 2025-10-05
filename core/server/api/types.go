package api

import (
	"fmt"
	"go/ast"
	"go/token"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// API Documentation Core Types
// =============================================================================

// APIEndpoint represents a single API endpoint documentation
type APIEndpoint struct {
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Description string                 `json:"description"`
	Request     map[string]interface{} `json:"request,omitempty"`
	Response    map[string]interface{} `json:"response,omitempty"`
	Auth        *AuthInfo              `json:"auth,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Handler     string                 `json:"handler_name,omitempty"`
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

// APIDocs holds all API documentation
type APIDocs struct {
	Title       string                 `json:"title"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	BaseURL     string                 `json:"base_url"`
	Endpoints   []APIEndpoint          `json:"endpoints"`
	Generated   string                 `json:"generated_at"`
	Components  map[string]interface{} `json:"components,omitempty"`
}

// =============================================================================
// Registry and Management Types
// =============================================================================

// AutoDiscoveryConfig controls how routes are automatically discovered
type AutoDiscoveryConfig struct {
	Enabled         bool `json:"enabled"`
	AnalyzeHandlers bool `json:"analyze_handlers"`
	GenerateTags    bool `json:"generate_tags"`
	DetectAuth      bool `json:"detect_auth"`
	IncludeInternal bool `json:"include_internal"`
}

// =============================================================================
// Router and Route Types
// =============================================================================

// AutoAPIRouter wraps PocketBase router for automatic API documentation
type AutoAPIRouter struct {
	router   interface{}
	registry *APIRegistry
}

// RouteChain represents a chainable route for middleware binding
type RouteChain struct {
	route      interface{}
	method     string
	path       string
	handler    func(*core.RequestEvent) error
	registry   *APIRegistry
	middleware []string
}

// HandlerInfo contains extracted handler information
type HandlerInfo struct {
	Name        string `json:"name"`
	Package     string `json:"package"`
	Description string `json:"description"`
}

// =============================================================================
// AST Parsing Types
// =============================================================================

// ASTParser provides robust AST parsing with improved error handling and performance
type ASTParser struct {
	mu         sync.RWMutex
	fileSet    *token.FileSet
	packages   map[string]*ast.Package
	structs    map[string]*StructInfo
	handlers   map[string]*ASTHandlerInfo
	imports    map[string]string
	typeCache  map[string]*TypeInfo
	fileCache  map[string]*FileParseResult
	validators []TypeValidator
	logger     Logger
}

// FileParseResult stores parsing results with metadata
type FileParseResult struct {
	ModTime  time.Time
	Structs  map[string]*StructInfo
	Handlers map[string]*ASTHandlerInfo
	Imports  map[string]string
	Errors   []ParseError
	ParsedAt time.Time
}

// ParseError represents a parsing error with context
type ParseError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Context string `json:"context,omitempty"`
}

func (e ParseError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s:%d:%d: %s: %s", e.File, e.Line, e.Column, e.Type, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// StructInfo contains comprehensive struct information
type StructInfo struct {
	Name           string                 `json:"name"`
	Package        string                 `json:"package"`
	Fields         map[string]*FieldInfo  `json:"fields"`
	JSONSchema     map[string]interface{} `json:"json_schema"`
	Description    string                 `json:"description"`
	Tags           []string               `json:"tags"`
	Embedded       []string               `json:"embedded,omitempty"`
	Methods        []string               `json:"methods,omitempty"`
	Implements     []string               `json:"implements,omitempty"`
	IsGeneric      bool                   `json:"is_generic"`
	TypeParams     []string               `json:"type_params,omitempty"`
	Documentation  *Documentation         `json:"documentation,omitempty"`
	SourceLocation *SourceLocation        `json:"source_location,omitempty"`
}

// FieldInfo contains detailed field information
type FieldInfo struct {
	Name           string                 `json:"name"`
	Type           string                 `json:"type"`
	JSONName       string                 `json:"json_name"`
	JSONOmitEmpty  bool                   `json:"json_omit_empty"`
	Required       bool                   `json:"required"`
	Validation     map[string]string      `json:"validation"`
	Description    string                 `json:"description"`
	Example        interface{}            `json:"example,omitempty"`
	Schema         map[string]interface{} `json:"schema"`
	IsPointer      bool                   `json:"is_pointer"`
	IsEmbedded     bool                   `json:"is_embedded"`
	IsExported     bool                   `json:"is_exported"`
	DefaultValue   interface{}            `json:"default_value,omitempty"`
	Constraints    map[string]interface{} `json:"constraints,omitempty"`
	SourceLocation *SourceLocation        `json:"source_location,omitempty"`
}

// ASTHandlerInfo contains comprehensive handler information
type ASTHandlerInfo struct {
	Name           string                 `json:"name"`
	Package        string                 `json:"package"`
	RequestType    string                 `json:"request_type"`
	ResponseType   string                 `json:"response_type"`
	ResponseSchema map[string]interface{} `json:"response_schema,omitempty"`
	Parameters     []*ParamInfo           `json:"parameters,omitempty"`
	UsesJSONDecode bool                   `json:"uses_json_decode"`
	UsesJSONReturn bool                   `json:"uses_json_return"`
	APIDescription string                 `json:"api_description"`
	APITags        []string               `json:"api_tags"`
	HTTPMethods    []string               `json:"http_methods"`
	Middleware     []string               `json:"middleware,omitempty"`
	Documentation  *Documentation         `json:"documentation,omitempty"`
	Complexity     int                    `json:"complexity"`
	SourceLocation *SourceLocation        `json:"source_location,omitempty"`
}

// ParamInfo contains parameter information
type ParamInfo struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Source       string                 `json:"source"` // "path", "query", "body", "header"
	Required     bool                   `json:"required"`
	Description  string                 `json:"description"`
	Example      interface{}            `json:"example,omitempty"`
	Validation   map[string]string      `json:"validation,omitempty"`
	DefaultValue interface{}            `json:"default_value,omitempty"`
	Constraints  map[string]interface{} `json:"constraints,omitempty"`
}

// TypeInfo contains type information for complex types
type TypeInfo struct {
	Name        string                 `json:"name"`
	Kind        string                 `json:"kind"` // "struct", "slice", "map", "interface", etc.
	ElementType *TypeInfo              `json:"element_type,omitempty"`
	KeyType     *TypeInfo              `json:"key_type,omitempty"`
	ValueType   *TypeInfo              `json:"value_type,omitempty"`
	Fields      map[string]*FieldInfo  `json:"fields,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	IsPointer   bool                   `json:"is_pointer"`
	IsGeneric   bool                   `json:"is_generic"`
	TypeParams  []string               `json:"type_params,omitempty"`
	Package     string                 `json:"package"`
	Constraints map[string]interface{} `json:"constraints,omitempty"`
}

// Documentation contains extracted documentation information
type Documentation struct {
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Returns     string            `json:"returns,omitempty"`
	Examples    []string          `json:"examples,omitempty"`
	SeeAlso     []string          `json:"see_also,omitempty"`
	Since       string            `json:"since,omitempty"`
	Deprecated  string            `json:"deprecated,omitempty"`
	Authors     []string          `json:"authors,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// SourceLocation contains source code location information
type SourceLocation struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// =============================================================================
// Schema Generation Types
// =============================================================================

// SchemaGenerator handles all schema generation and analysis operations
type SchemaGenerator struct {
	mu          sync.RWMutex
	astParser   ASTParserInterface
	typeCache   map[string]interface{}
	schemaCache map[string]map[string]interface{}
	validators  []TypeValidator
	logger      Logger
}

// SchemaAnalysisResult contains the result of schema analysis
type SchemaAnalysisResult struct {
	RequestSchema  map[string]interface{} `json:"request_schema,omitempty"`
	ResponseSchema map[string]interface{} `json:"response_schema,omitempty"`
	Errors         []error                `json:"errors,omitempty"`
	Warnings       []string               `json:"warnings,omitempty"`
}

// SchemaPattern represents a pattern for matching handlers/paths to generate schemas
type SchemaPattern struct {
	Name         string                        `json:"name"`
	HandlerMatch func(string) bool             `json:"-"`
	PathMatch    func(string) bool             `json:"-"`
	RequestGen   func() map[string]interface{} `json:"-"`
	ResponseGen  func() map[string]interface{} `json:"-"`
}

// RequestResponseAnalyzer analyzes handler functions to extract request/response types
type RequestResponseAnalyzer struct {
	astParser ASTParserInterface
	logger    Logger
}

// TypeSchemaBuilder converts Go types to JSON schema representations
type TypeSchemaBuilder struct {
	typeCache map[string]map[string]interface{}
	logger    Logger
}

// ValidationExtractor extracts validation rules from struct tags and comments
type ValidationExtractor struct {
	logger Logger
}

// =============================================================================
// Interface Definitions
// =============================================================================

// ASTParserInterface defines the contract for AST parsing operations
type ASTParserInterface interface {
	ParseFile(filename string) error
	GetAllStructs() map[string]*StructInfo
	GetAllHandlers() map[string]*ASTHandlerInfo
	GetStructByName(name string) (*StructInfo, bool)
	GetHandlerByName(name string) (*ASTHandlerInfo, bool)
	GetParseErrors() []ParseError
	ClearCache()
	EnhanceEndpoint(endpoint *APIEndpoint) error
	GetHandlerDescription(handlerName string) string
	GetHandlerTags(handlerName string) []string
	GetStructsForFinding() map[string]*StructInfo
}

// SchemaGeneratorInterface defines the contract for schema generation operations
type SchemaGeneratorInterface interface {
	AnalyzeRequestSchema(endpoint *APIEndpoint) (map[string]interface{}, error)
	AnalyzeResponseSchema(endpoint *APIEndpoint) (map[string]interface{}, error)
	AnalyzeSchemaFromPath(method, path string) (*SchemaAnalysisResult, error)
	GenerateComponentSchemas() map[string]interface{}
}

// Logger interface for structured logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// DefaultLogger provides a default logger implementation
type DefaultLogger struct{}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) { /* no-op or implement */ }
func (l *DefaultLogger) Info(msg string, args ...interface{})  { /* no-op or implement */ }
func (l *DefaultLogger) Warn(msg string, args ...interface{})  { /* no-op or implement */ }
func (l *DefaultLogger) Error(msg string, args ...interface{}) { /* no-op or implement */ }

// TypeValidator interface for type validation
type TypeValidator interface {
	Validate(typeInfo *TypeInfo) error
	Name() string
}

// =============================================================================
// Configuration Types
// =============================================================================

// APIDocsConfig holds configuration for the API documentation system
type APIDocsConfig struct {
	Title         string               `json:"title"`
	Version       string               `json:"version"`
	Description   string               `json:"description"`
	BaseURL       string               `json:"base_url"`
	Enabled       bool                 `json:"enabled"`
	AutoDiscovery *AutoDiscoveryConfig `json:"auto_discovery,omitempty"`
}

// DefaultAPIDocsConfig returns a default configuration
func DefaultAPIDocsConfig() *APIDocsConfig {
	return &APIDocsConfig{
		Title:       "pb-ext API",
		Version:     "1.0.0",
		Description: "AST discovered API endpoints",
		BaseURL:     "/api",
		Enabled:     true,
		AutoDiscovery: &AutoDiscoveryConfig{
			Enabled:         true,
			AnalyzeHandlers: true,
			GenerateTags:    true,
			DetectAuth:      true,
			IncludeInternal: false,
		},
	}
}
