package discovery

import (
	"go/token"
	"strings"
	"sync"
	"time"

	"github.com/magooney-loon/pb-ext/core/api"
)

// =============================================================================
// Discovery Engine (simplified & OpenAPI compatible)
// =============================================================================

// Engine implements automatic API endpoint discovery
type Engine struct {
	mu        sync.RWMutex
	config    *Config
	parser    *ASTParser
	generator *SchemaGenerator
	results   *DiscoveryResult
	errors    []error
}

// Config configures discovery behavior
type Config struct {
	PackagePaths    []string      `json:"package_paths"`
	IncludeInternal bool          `json:"include_internal"`
	DetectAuth      bool          `json:"detect_auth"`
	GenerateTags    bool          `json:"generate_tags"`
	AnalyzeSchemas  bool          `json:"analyze_schemas"`
	FollowImports   bool          `json:"follow_imports"`
	MaxDepth        int           `json:"max_depth"`
	Timeout         time.Duration `json:"timeout"`
	CacheResults    bool          `json:"cache_results"`
	LogLevel        string        `json:"log_level"`
}

// DiscoveryResult contains discovery results
type DiscoveryResult struct {
	Endpoints    []api.APIEndpoint          `json:"endpoints"`
	Schemas      map[string]*api.SchemaInfo `json:"schemas"`
	Handlers     []HandlerInfo              `json:"handlers"`
	Structs      []StructInfo               `json:"structs"`
	Packages     []PackageInfo              `json:"packages"`
	Dependencies []string                   `json:"dependencies"`
	Errors       []string                   `json:"errors,omitempty"`
	Warnings     []string                   `json:"warnings,omitempty"`
	Stats        *DiscoveryStats            `json:"stats"`
	StartedAt    time.Time                  `json:"started_at"`
	CompletedAt  time.Time                  `json:"completed_at"`
	Duration     time.Duration              `json:"duration"`
	CacheHit     bool                       `json:"cache_hit,omitempty"`
	Metadata     map[string]string          `json:"metadata,omitempty"`
}

// DiscoveryStats contains discovery statistics
type DiscoveryStats struct {
	FilesScanned     int `json:"files_scanned"`
	HandlersFound    int `json:"handlers_found"`
	EndpointsFound   int `json:"endpoints_found"`
	StructsAnalyzed  int `json:"structs_analyzed"`
	SchemasGenerated int `json:"schemas_generated"`
	TagsGenerated    int `json:"tags_generated"`
	AuthDetected     int `json:"auth_detected"`
	ErrorsFound      int `json:"errors_found"`
	WarningsFound    int `json:"warnings_found"`
}

// =============================================================================
// AST Parser (simplified)
// =============================================================================

// ASTParser implements Go AST parsing for API discovery
type ASTParser struct {
	mu       sync.RWMutex
	fileSet  *token.FileSet
	packages map[string]*PackageInfo
	handlers map[string]*HandlerInfo
	structs  map[string]*StructInfo
	imports  map[string][]string
	errors   []ParseError
	config   *ParserConfig
}

// ParserConfig configures AST parsing
type ParserConfig struct {
	ParseComments   bool     `json:"parse_comments"`
	ParseTests      bool     `json:"parse_tests"`
	ParseExamples   bool     `json:"parse_examples"`
	SkipVendor      bool     `json:"skip_vendor"`
	IncludePrivate  bool     `json:"include_private"`
	FilePatterns    []string `json:"file_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
}

// PackageInfo contains information about a Go package
type PackageInfo struct {
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Dir         string            `json:"dir"`
	Files       []string          `json:"files"`
	Imports     []string          `json:"imports"`
	Handlers    []string          `json:"handlers"` // handler names in this package
	Structs     []string          `json:"structs"`  // struct names in this package
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Module      string            `json:"module,omitempty"`
	GoVersion   string            `json:"go_version,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// HandlerInfo contains information about a discovered handler function
type HandlerInfo struct {
	Name        string            `json:"name"`
	Package     string            `json:"package"`
	File        string            `json:"file"`
	Line        int               `json:"line"`
	Column      int               `json:"column"`
	Receiver    string            `json:"receiver,omitempty"`
	Method      string            `json:"method,omitempty"` // HTTP method if detected
	Path        string            `json:"path,omitempty"`   // URL path if detected
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Auth        *AuthInfo         `json:"auth,omitempty"`
	Parameters  []ParameterInfo   `json:"parameters,omitempty"`
	Returns     []ReturnInfo      `json:"returns,omitempty"`
	Comments    []string          `json:"comments,omitempty"`
	Annotations []string          `json:"annotations,omitempty"`
	Complexity  int               `json:"complexity,omitempty"`
	IsExported  bool              `json:"is_exported"`
	IsHandler   bool              `json:"is_handler"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// StructInfo contains information about a Go struct
type StructInfo struct {
	Name        string            `json:"name"`
	Package     string            `json:"package"`
	File        string            `json:"file"`
	Line        int               `json:"line"`
	Column      int               `json:"column"`
	Fields      []FieldInfo       `json:"fields"`
	Methods     []MethodInfo      `json:"methods,omitempty"`
	Embedded    []string          `json:"embedded,omitempty"` // embedded struct names
	Tags        []string          `json:"tags,omitempty"`
	Description string            `json:"description,omitempty"`
	Comments    []string          `json:"comments,omitempty"`
	IsExported  bool              `json:"is_exported"`
	IsRequest   bool              `json:"is_request"`  // likely a request struct
	IsResponse  bool              `json:"is_response"` // likely a response struct
	IsModel     bool              `json:"is_model"`    // likely a data model
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// FieldInfo contains information about a struct field
type FieldInfo struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	JSONName     string            `json:"json_name,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	Description  string            `json:"description,omitempty"`
	Required     bool              `json:"required"`
	Embedded     bool              `json:"embedded"`
	IsExported   bool              `json:"is_exported"`
	IsPointer    bool              `json:"is_pointer"`
	IsSlice      bool              `json:"is_slice"`
	IsMap        bool              `json:"is_map"`
	DefaultValue interface{}       `json:"default_value,omitempty"`
}

// MethodInfo contains information about struct methods
type MethodInfo struct {
	Name        string            `json:"name"`
	Receiver    string            `json:"receiver"`
	Parameters  []ParameterInfo   `json:"parameters,omitempty"`
	Returns     []ReturnInfo      `json:"returns,omitempty"`
	Description string            `json:"description,omitempty"`
	IsExported  bool              `json:"is_exported"`
	IsHandler   bool              `json:"is_handler"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ParameterInfo contains information about function parameters
type ParameterInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Optional bool   `json:"optional"`
	In       string `json:"in,omitempty"` // "path", "query", "body", "header"
}

// ReturnInfo contains information about function return values
type ReturnInfo struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	IsError     bool   `json:"is_error"`
}

// AuthInfo contains authentication information discovered from code
type AuthInfo struct {
	Type        string   `json:"type"` // "none", "bearer", "admin", "owner"
	Required    bool     `json:"required"`
	Functions   []string `json:"functions"`  // auth functions called
	Middleware  []string `json:"middleware"` // middleware detected
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
}

// =============================================================================
// Schema Generator (simplified)
// =============================================================================

// SchemaGenerator generates JSON schemas from Go structs
type SchemaGenerator struct {
	mu            sync.RWMutex
	typeResolver  *TypeResolver
	cache         map[string]*api.SchemaInfo
	structs       map[string]*StructInfo
	config        *GeneratorConfig
	circularRefs  map[string]bool
	processedRefs map[string]*api.SchemaInfo
}

// GeneratorConfig configures schema generation
type GeneratorConfig struct {
	IncludeExamples   bool              `json:"include_examples"`
	IncludeDefaults   bool              `json:"include_defaults"`
	StrictValidation  bool              `json:"strict_validation"`
	ResolveReferences bool              `json:"resolve_references"`
	MaxDepth          int               `json:"max_depth"`
	TypeMappings      map[string]string `json:"type_mappings"`
	TagMappings       map[string]string `json:"tag_mappings"`
	SkipPrivate       bool              `json:"skip_private"`
	UseJSONTags       bool              `json:"use_json_tags"`
	UseValidateTags   bool              `json:"use_validate_tags"`
}

// TypeResolver resolves Go types to JSON Schema types
type TypeResolver struct {
	basicTypes    map[string]SchemaType
	customTypes   map[string]SchemaType
	packageTypes  map[string]string
	importAliases map[string]string
}

// SchemaType represents a JSON Schema type mapping
type SchemaType struct {
	Type        string      `json:"type"`
	Format      string      `json:"format,omitempty"`
	Items       *SchemaType `json:"items,omitempty"`
	Properties  string      `json:"properties,omitempty"`
	Description string      `json:"description,omitempty"`
}

// =============================================================================
// Route Detection
// =============================================================================

// RouteDetector detects HTTP routes from Go code patterns
type RouteDetector struct {
	patterns []RoutePattern
	config   *RouteConfig
}

// RoutePattern represents a pattern for detecting routes
type RoutePattern struct {
	Name        string            `json:"name"`
	Pattern     string            `json:"pattern"`    // regex pattern
	Method      string            `json:"method"`     // HTTP method extraction
	Path        string            `json:"path"`       // path extraction
	Handler     string            `json:"handler"`    // handler extraction
	Confidence  float64           `json:"confidence"` // confidence score
	Description string            `json:"description,omitempty"`
	Examples    []string          `json:"examples,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// RouteConfig configures route detection
type RouteConfig struct {
	EnabledPatterns []string          `json:"enabled_patterns"`
	CustomPatterns  []RoutePattern    `json:"custom_patterns,omitempty"`
	MinConfidence   float64           `json:"min_confidence"`
	FollowChains    bool              `json:"follow_chains"` // follow middleware chains
	DetectGroups    bool              `json:"detect_groups"` // detect route groups
	Framework       string            `json:"framework"`     // target framework (echo, gin, etc.)
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// DetectedRoute contains information about a detected route
type DetectedRoute struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Handler     string            `json:"handler"`
	Middleware  []string          `json:"middleware,omitempty"`
	Pattern     string            `json:"pattern"` // pattern that matched
	Confidence  float64           `json:"confidence"`
	File        string            `json:"file"`
	Line        int               `json:"line"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// =============================================================================
// Error Types
// =============================================================================

// ParseError represents a parsing error
type ParseError struct {
	File     string            `json:"file"`
	Line     int               `json:"line"`
	Column   int               `json:"column"`
	Message  string            `json:"message"`
	Type     string            `json:"type"`     // "syntax", "semantic", "warning"
	Severity string            `json:"severity"` // "error", "warning", "info"
	Context  string            `json:"context,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// DiscoveryError represents a discovery error
type DiscoveryError struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	Package     string            `json:"package,omitempty"`
	File        string            `json:"file,omitempty"`
	Handler     string            `json:"handler,omitempty"`
	Struct      string            `json:"struct,omitempty"`
	Details     string            `json:"details,omitempty"`
	Suggestions []string          `json:"suggestions,omitempty"`
	Time        time.Time         `json:"time"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Common error codes
const (
	ErrParsingFailed     = "PARSING_FAILED"
	ErrHandlerNotFound   = "HANDLER_NOT_FOUND"
	ErrStructNotFound    = "STRUCT_NOT_FOUND"
	ErrSchemaGeneration  = "SCHEMA_GENERATION_FAILED"
	ErrRouteDetection    = "ROUTE_DETECTION_FAILED"
	ErrAuthDetection     = "AUTH_DETECTION_FAILED"
	ErrCircularReference = "CIRCULAR_REFERENCE"
	ErrUnsupportedType   = "UNSUPPORTED_TYPE"
	ErrInvalidAnnotation = "INVALID_ANNOTATION"
	ErrPackageNotFound   = "PACKAGE_NOT_FOUND"
	ErrImportResolution  = "IMPORT_RESOLUTION_FAILED"
	ErrDiscoveryTimeout  = "DISCOVERY_TIMEOUT"
)

// =============================================================================
// Code Analysis Patterns
// =============================================================================

// AnalysisPattern represents a pattern for code analysis
type AnalysisPattern struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`    // "handler", "auth", "middleware", "route"
	Pattern     string            `json:"pattern"` // AST pattern or regex
	Confidence  float64           `json:"confidence"`
	Framework   string            `json:"framework"` // target framework
	Description string            `json:"description,omitempty"`
	Examples    []string          `json:"examples,omitempty"`
	Enabled     bool              `json:"enabled"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PatternMatch represents a pattern match result
type PatternMatch struct {
	Pattern    string            `json:"pattern"`
	Confidence float64           `json:"confidence"`
	Location   *SourceLocation   `json:"location"`
	Context    string            `json:"context,omitempty"`
	Extracted  map[string]string `json:"extracted,omitempty"` // extracted values
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// SourceLocation represents a location in source code
type SourceLocation struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Package string `json:"package"`
}

// =============================================================================
// Built-in Patterns
// =============================================================================

// Common patterns for different frameworks
var (
	// Echo framework patterns
	EchoPatterns = []AnalysisPattern{
		{
			Name:        "echo_handler",
			Type:        "handler",
			Pattern:     `func\s+(\w+)\s*\(\s*c\s+echo\.Context\s*\)\s+error`,
			Confidence:  0.9,
			Framework:   "echo",
			Description: "Echo handler function pattern",
		},
		{
			Name:        "echo_route",
			Type:        "route",
			Pattern:     `(\w+)\.(GET|POST|PUT|DELETE|PATCH)\s*\(\s*"([^"]+)"\s*,\s*(\w+)`,
			Confidence:  0.95,
			Framework:   "echo",
			Description: "Echo route registration pattern",
		},
	}

	// PocketBase patterns
	PocketBasePatterns = []AnalysisPattern{
		{
			Name:        "pb_handler",
			Type:        "handler",
			Pattern:     `func\s+(\w+)\s*\(\s*app\s+\*pocketbase\.PocketBase\s*\)`,
			Confidence:  0.9,
			Framework:   "pocketbase",
			Description: "PocketBase handler function pattern",
		},
		{
			Name:        "pb_auth",
			Type:        "auth",
			Pattern:     `RequireAuth|RequireSuperuserAuth|RequireGuestOnly`,
			Confidence:  0.85,
			Framework:   "pocketbase",
			Description: "PocketBase auth middleware pattern",
		},
	}

	// Generic patterns
	GenericPatterns = []AnalysisPattern{
		{
			Name:        "http_handler",
			Type:        "handler",
			Pattern:     `func\s+(\w+)\s*\(\s*w\s+http\.ResponseWriter\s*,\s*r\s+\*http\.Request\s*\)`,
			Confidence:  0.8,
			Framework:   "net/http",
			Description: "Standard HTTP handler pattern",
		},
		{
			Name:        "json_struct",
			Type:        "struct",
			Pattern:     `type\s+(\w+)\s+struct\s*\{[^}]*json:`,
			Confidence:  0.7,
			Framework:   "generic",
			Description: "Struct with JSON tags",
		},
	}
)

// =============================================================================
// Default Configurations
// =============================================================================

// DefaultConfig returns a default discovery configuration
func DefaultConfig() *Config {
	return &Config{
		PackagePaths:    []string{"./"},
		IncludeInternal: false,
		DetectAuth:      true,
		GenerateTags:    true,
		AnalyzeSchemas:  true,
		FollowImports:   false,
		MaxDepth:        3,
		Timeout:         5 * time.Minute,
		CacheResults:    true,
		LogLevel:        "info",
	}
}

// DefaultParserConfig returns a default parser configuration
func DefaultParserConfig() *ParserConfig {
	return &ParserConfig{
		ParseComments:   true,
		ParseTests:      false,
		ParseExamples:   false,
		SkipVendor:      true,
		IncludePrivate:  false,
		FilePatterns:    []string{"*.go"},
		ExcludePatterns: []string{"*_test.go", "vendor/*"},
	}
}

// DefaultGeneratorConfig returns a default generator configuration
func DefaultGeneratorConfig() *GeneratorConfig {
	return &GeneratorConfig{
		IncludeExamples:   true,
		IncludeDefaults:   true,
		StrictValidation:  false,
		ResolveReferences: true,
		MaxDepth:          10,
		TypeMappings:      make(map[string]string),
		TagMappings:       make(map[string]string),
		SkipPrivate:       true,
		UseJSONTags:       true,
		UseValidateTags:   true,
	}
}

// DefaultRouteConfig returns a default route detection configuration
func DefaultRouteConfig() *RouteConfig {
	return &RouteConfig{
		EnabledPatterns: []string{"echo_handler", "echo_route", "pb_handler", "pb_auth"},
		MinConfidence:   0.7,
		FollowChains:    true,
		DetectGroups:    true,
		Framework:       "auto",
	}
}

// =============================================================================
// Utility Functions
// =============================================================================

// IsExportedName checks if a name is exported (starts with uppercase)
func IsExportedName(name string) bool {
	if len(name) == 0 {
		return false
	}
	return name[0] >= 'A' && name[0] <= 'Z'
}

// IsHandlerFunction checks if a function looks like an HTTP handler
func IsHandlerFunction(name string) bool {
	handlerSuffixes := []string{"Handler", "Handle", "Endpoint", "API"}
	for _, suffix := range handlerSuffixes {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

// IsRequestStruct checks if a struct looks like a request struct
func IsRequestStruct(name string) bool {
	requestSuffixes := []string{"Request", "Req", "Input", "Params", "Args"}
	for _, suffix := range requestSuffixes {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

// IsResponseStruct checks if a struct looks like a response struct
func IsResponseStruct(name string) bool {
	responseSuffixes := []string{"Response", "Resp", "Output", "Result", "Reply"}
	for _, suffix := range responseSuffixes {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

// GenerateTagFromPath generates a tag from an API path
func GenerateTagFromPath(path string) string {
	// Simple tag generation from path
	// e.g., "/api/v1/users" -> "users"
	parts := []string{}
	for _, part := range strings.Split(path, "/") {
		if part != "" && part != "api" && !strings.HasPrefix(part, "v") {
			parts = append(parts, part)
		}
	}
	if len(parts) > 0 {
		return parts[0]
	}
	return "default"
}

// Constants
const (
	MaxDiscoveryTimeout = 10 * time.Minute
	MaxParseDepth       = 10
	MaxSchemaDepth      = 20
	MaxPatternMatches   = 1000
)
