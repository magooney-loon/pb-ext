package compiler

import (
	"go/ast"
	"go/token"
	"regexp"
	"time"
)

// =============================================================================
// Core Compiler Directive Types
// =============================================================================

// DirectiveProcessor manages parsing and processing of compiler directives
type DirectiveProcessor struct {
	parsers    map[string]DirectiveParser
	validators map[string]DirectiveValidator
	generators map[string]CodeGenerator
	config     *DirectiveConfig
	cache      *DirectiveCache
	metadata   map[string]interface{}
}

// DirectiveConfig configures directive processing
type DirectiveConfig struct {
	Enabled            bool                   `json:"enabled"`
	StrictMode         bool                   `json:"strict_mode"`
	AllowedDirectives  []string               `json:"allowed_directives"`
	RequiredDirectives []string               `json:"required_directives"`
	CustomPrefixes     []string               `json:"custom_prefixes"`
	ValidationRules    []ValidationRule       `json:"validation_rules"`
	GenerationRules    []GenerationRule       `json:"generation_rules"`
	CacheEnabled       bool                   `json:"cache_enabled"`
	CacheTTL           time.Duration          `json:"cache_ttl"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// Directive represents a parsed compiler directive
type Directive struct {
	ID             string                 `json:"id"`
	Type           DirectiveType          `json:"type"`
	Name           string                 `json:"name"`
	Value          string                 `json:"value,omitempty"`
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
	Modifiers      []string               `json:"modifiers,omitempty"`
	Conditions     []DirectiveCondition   `json:"conditions,omitempty"`
	SourceLocation *SourceLocation        `json:"source_location"`
	ParsedAt       time.Time              `json:"parsed_at"`
	Valid          bool                   `json:"valid"`
	Errors         []DirectiveError       `json:"errors,omitempty"`
	Warnings       []DirectiveWarning     `json:"warnings,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// DirectiveType represents the category of a directive
type DirectiveType string

const (
	DirectiveTypeAPI           DirectiveType = "api"
	DirectiveTypeEndpoint      DirectiveType = "endpoint"
	DirectiveTypeHandler       DirectiveType = "handler"
	DirectiveTypeMiddleware    DirectiveType = "middleware"
	DirectiveTypeAuth          DirectiveType = "auth"
	DirectiveTypeValidation    DirectiveType = "validation"
	DirectiveTypeDocumentation DirectiveType = "documentation"
	DirectiveTypeGeneration    DirectiveType = "generation"
	DirectiveTypeConfig        DirectiveType = "config"
	DirectiveTypeMetadata      DirectiveType = "metadata"
)

// =============================================================================
// API-Specific Directive Types
// =============================================================================

// APIDirective contains API-level directives
type APIDirective struct {
	*Directive
	Version     string            `json:"version,omitempty"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	BaseURL     string            `json:"base_url,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Contact     *ContactInfo      `json:"contact,omitempty"`
	License     *LicenseInfo      `json:"license,omitempty"`
	Servers     []ServerInfo      `json:"servers,omitempty"`
	Security    []SecurityScheme  `json:"security,omitempty"`
	Extensions  map[string]string `json:"extensions,omitempty"`
}

// EndpointDirective contains endpoint-level directives
type EndpointDirective struct {
	*Directive
	Method        string               `json:"method,omitempty"`
	Path          string               `json:"path,omitempty"`
	Summary       string               `json:"summary,omitempty"`
	Description   string               `json:"description,omitempty"`
	OperationID   string               `json:"operation_id,omitempty"`
	Tags          []string             `json:"tags,omitempty"`
	Consumes      []string             `json:"consumes,omitempty"`
	Produces      []string             `json:"produces,omitempty"`
	Deprecated    bool                 `json:"deprecated"`
	Internal      bool                 `json:"internal"`
	RateLimit     *RateLimitDirective  `json:"rate_limit,omitempty"`
	Cache         *CacheDirective      `json:"cache,omitempty"`
	Authorization *AuthDirective       `json:"authorization,omitempty"`
	Validation    *ValidationDirective `json:"validation,omitempty"`
	Examples      []ExampleDirective   `json:"examples,omitempty"`
	CustomHeaders map[string]string    `json:"custom_headers,omitempty"`
}

// HandlerDirective contains handler-level directives
type HandlerDirective struct {
	*Directive
	Handler      string            `json:"handler,omitempty"`
	Function     string            `json:"function,omitempty"`
	Receiver     string            `json:"receiver,omitempty"`
	Package      string            `json:"package,omitempty"`
	RequestType  string            `json:"request_type,omitempty"`
	ResponseType string            `json:"response_type,omitempty"`
	Middleware   []string          `json:"middleware,omitempty"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
	Retry        *RetryDirective   `json:"retry,omitempty"`
	Circuit      *CircuitDirective `json:"circuit,omitempty"`
	Metrics      *MetricsDirective `json:"metrics,omitempty"`
	Logging      *LoggingDirective `json:"logging,omitempty"`
}

// =============================================================================
// Specialized Directive Types
// =============================================================================

// AuthDirective defines authentication requirements
type AuthDirective struct {
	Required    bool              `json:"required"`
	Schemes     []string          `json:"schemes,omitempty"`
	Scopes      []string          `json:"scopes,omitempty"`
	Roles       []string          `json:"roles,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Collections []string          `json:"collections,omitempty"`
	Owner       *OwnershipCheck   `json:"owner,omitempty"`
	Custom      map[string]string `json:"custom,omitempty"`
}

// ValidationDirective defines validation rules
type ValidationDirective struct {
	RequestValidation  []FieldValidation `json:"request_validation,omitempty"`
	ResponseValidation []FieldValidation `json:"response_validation,omitempty"`
	CustomValidators   []string          `json:"custom_validators,omitempty"`
	StrictMode         bool              `json:"strict_mode"`
	FailOnError        bool              `json:"fail_on_error"`
}

// RateLimitDirective defines rate limiting
type RateLimitDirective struct {
	RequestsPerMinute int           `json:"requests_per_minute,omitempty"`
	RequestsPerHour   int           `json:"requests_per_hour,omitempty"`
	RequestsPerDay    int           `json:"requests_per_day,omitempty"`
	BurstLimit        int           `json:"burst_limit,omitempty"`
	WindowSize        time.Duration `json:"window_size,omitempty"`
	SkipAuthenticated bool          `json:"skip_authenticated"`
	CustomKey         string        `json:"custom_key,omitempty"`
	Policy            string        `json:"policy,omitempty"`
}

// CacheDirective defines caching behavior
type CacheDirective struct {
	Enabled      bool          `json:"enabled"`
	TTL          time.Duration `json:"ttl,omitempty"`
	MaxAge       time.Duration `json:"max_age,omitempty"`
	SMaxAge      time.Duration `json:"s_max_age,omitempty"`
	ETag         bool          `json:"etag"`
	LastModified bool          `json:"last_modified"`
	Vary         []string      `json:"vary,omitempty"`
	Keys         []string      `json:"keys,omitempty"`
	Policy       string        `json:"policy,omitempty"`
}

// RetryDirective defines retry behavior
type RetryDirective struct {
	Enabled     bool          `json:"enabled"`
	MaxRetries  int           `json:"max_retries"`
	BackoffType string        `json:"backoff_type"` // linear, exponential, fixed
	BaseDelay   time.Duration `json:"base_delay"`
	MaxDelay    time.Duration `json:"max_delay"`
	Jitter      bool          `json:"jitter"`
	RetryOn     []string      `json:"retry_on,omitempty"`
}

// CircuitDirective defines circuit breaker behavior
type CircuitDirective struct {
	Enabled                  bool          `json:"enabled"`
	FailureThreshold         int           `json:"failure_threshold"`
	RecoveryTimeout          time.Duration `json:"recovery_timeout"`
	HalfOpenMaxCalls         int           `json:"half_open_max_calls"`
	HalfOpenSuccessThreshold int           `json:"half_open_success_threshold"`
	OnStateChange            string        `json:"on_state_change,omitempty"`
}

// MetricsDirective defines metrics collection
type MetricsDirective struct {
	Enabled       bool              `json:"enabled"`
	ResponseTime  bool              `json:"response_time"`
	RequestCount  bool              `json:"request_count"`
	ErrorCount    bool              `json:"error_count"`
	StatusCodes   bool              `json:"status_codes"`
	CustomMetrics []string          `json:"custom_metrics,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Histogram     bool              `json:"histogram"`
	Percentiles   []float64         `json:"percentiles,omitempty"`
}

// LoggingDirective defines logging behavior
type LoggingDirective struct {
	Enabled       bool              `json:"enabled"`
	Level         string            `json:"level,omitempty"`
	Format        string            `json:"format,omitempty"`
	Fields        []string          `json:"fields,omitempty"`
	ExcludeFields []string          `json:"exclude_fields,omitempty"`
	Structured    bool              `json:"structured"`
	Sanitize      []string          `json:"sanitize,omitempty"`
	CustomFields  map[string]string `json:"custom_fields,omitempty"`
}

// ExampleDirective defines API examples
type ExampleDirective struct {
	Name        string                 `json:"name"`
	Summary     string                 `json:"summary,omitempty"`
	Description string                 `json:"description,omitempty"`
	Request     map[string]interface{} `json:"request,omitempty"`
	Response    map[string]interface{} `json:"response,omitempty"`
	StatusCode  int                    `json:"status_code,omitempty"`
	ContentType string                 `json:"content_type,omitempty"`
}

// =============================================================================
// Supporting Types
// =============================================================================

// DirectiveCondition defines conditional application of directives
type DirectiveCondition struct {
	Type     string      `json:"type"`     // env, build, version, custom
	Field    string      `json:"field"`    // field to check
	Operator string      `json:"operator"` // eq, ne, gt, lt, in, not_in, regex
	Value    interface{} `json:"value"`    // value to compare against
	Negated  bool        `json:"negated"`  // negate the condition
}

// FieldValidation defines validation for a specific field
type FieldValidation struct {
	Field           string                 `json:"field"`
	Type            string                 `json:"type"`
	Required        bool                   `json:"required"`
	Rules           []ValidationRule       `json:"rules"`
	CustomValidator string                 `json:"custom_validator,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ValidationRule defines a single validation rule
type ValidationRule struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Message    string                 `json:"message,omitempty"`
	Condition  *DirectiveCondition    `json:"condition,omitempty"`
}

// GenerationRule defines code generation rules
type GenerationRule struct {
	Name      string                 `json:"name"`
	Template  string                 `json:"template"`
	Output    string                 `json:"output"`
	Condition *DirectiveCondition    `json:"condition,omitempty"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// OwnershipCheck defines ownership validation
type OwnershipCheck struct {
	Enabled    bool   `json:"enabled"`
	Field      string `json:"field"`
	Parameter  string `json:"parameter"`
	Collection string `json:"collection,omitempty"`
	Relation   string `json:"relation,omitempty"`
}

// ContactInfo defines API contact information
type ContactInfo struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// LicenseInfo defines API license information
type LicenseInfo struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// ServerInfo defines server information
type ServerInfo struct {
	URL         string                 `json:"url"`
	Description string                 `json:"description,omitempty"`
	Variables   map[string]interface{} `json:"variables,omitempty"`
}

// SecurityScheme defines security scheme
type SecurityScheme struct {
	Type          string            `json:"type"`
	Name          string            `json:"name,omitempty"`
	In            string            `json:"in,omitempty"`
	Scheme        string            `json:"scheme,omitempty"`
	BearerFormat  string            `json:"bearer_format,omitempty"`
	Flows         map[string]string `json:"flows,omitempty"`
	OpenIDConnect string            `json:"open_id_connect,omitempty"`
}

// =============================================================================
// Parser and Processor Interfaces
// =============================================================================

// DirectiveParser defines interface for parsing directives
type DirectiveParser interface {
	Parse(comment string, location *SourceLocation) ([]*Directive, error)
	CanParse(comment string) bool
	GetSupportedDirectives() []string
	Name() string
}

// DirectiveValidator defines interface for validating directives
type DirectiveValidator interface {
	Validate(directive *Directive) []DirectiveError
	CanValidate(directiveType DirectiveType) bool
	Name() string
}

// CodeGenerator defines interface for generating code from directives
type CodeGenerator interface {
	Generate(directives []*Directive, config *GenerationConfig) (*GenerationResult, error)
	CanGenerate(directiveType DirectiveType) bool
	Name() string
}

// =============================================================================
// Processing Results and Errors
// =============================================================================

// DirectiveParseResult contains the result of parsing directives
type DirectiveParseResult struct {
	Directives []*Directive       `json:"directives"`
	Errors     []DirectiveError   `json:"errors"`
	Warnings   []DirectiveWarning `json:"warnings"`
	Stats      *ParseStats        `json:"stats"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    time.Time          `json:"end_time"`
	Duration   time.Duration      `json:"duration"`
}

// DirectiveError represents an error in directive processing
type DirectiveError struct {
	Type           string                 `json:"type"`
	Code           string                 `json:"code"`
	Message        string                 `json:"message"`
	Context        string                 `json:"context,omitempty"`
	SourceLocation *SourceLocation        `json:"source_location,omitempty"`
	Directive      string                 `json:"directive,omitempty"`
	Severity       ErrorSeverity          `json:"severity"`
	Timestamp      time.Time              `json:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// DirectiveWarning represents a warning in directive processing
type DirectiveWarning struct {
	Type           string                 `json:"type"`
	Code           string                 `json:"code"`
	Message        string                 `json:"message"`
	Context        string                 `json:"context,omitempty"`
	SourceLocation *SourceLocation        `json:"source_location,omitempty"`
	Directive      string                 `json:"directive,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// GenerationResult contains the result of code generation
type GenerationResult struct {
	Files     []*GeneratedFile       `json:"files"`
	Errors    []DirectiveError       `json:"errors"`
	Warnings  []DirectiveWarning     `json:"warnings"`
	Stats     *GenerationStats       `json:"stats"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Duration  time.Duration          `json:"duration"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// GeneratedFile represents a generated file
type GeneratedFile struct {
	Path        string                 `json:"path"`
	Name        string                 `json:"name"`
	Content     string                 `json:"content"`
	Type        string                 `json:"type"`
	Template    string                 `json:"template,omitempty"`
	Directives  []string               `json:"directives"`
	Size        int64                  `json:"size"`
	GeneratedAt time.Time              `json:"generated_at"`
	Checksum    string                 `json:"checksum,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// =============================================================================
// Statistics and Configuration Types
// =============================================================================

// ParseStats contains statistics about directive parsing
type ParseStats struct {
	DirectivesParsed  int                   `json:"directives_parsed"`
	DirectivesValid   int                   `json:"directives_valid"`
	DirectivesInvalid int                   `json:"directives_invalid"`
	ErrorsCount       int                   `json:"errors_count"`
	WarningsCount     int                   `json:"warnings_count"`
	ProcessingTime    time.Duration         `json:"processing_time"`
	AverageParseTime  time.Duration         `json:"average_parse_time"`
	DirectivesByType  map[DirectiveType]int `json:"directives_by_type"`
}

// GenerationStats contains statistics about code generation
type GenerationStats struct {
	FilesGenerated      int            `json:"files_generated"`
	LinesGenerated      int            `json:"lines_generated"`
	TemplatesUsed       int            `json:"templates_used"`
	DirectivesProcessed int            `json:"directives_processed"`
	ProcessingTime      time.Duration  `json:"processing_time"`
	AverageFileTime     time.Duration  `json:"average_file_time"`
	FilesByType         map[string]int `json:"files_by_type"`
}

// GenerationConfig configures code generation
type GenerationConfig struct {
	OutputDir         string                 `json:"output_dir"`
	TemplateDir       string                 `json:"template_dir"`
	FileNamePattern   string                 `json:"file_name_pattern"`
	OverwriteExisting bool                   `json:"overwrite_existing"`
	BackupExisting    bool                   `json:"backup_existing"`
	DryRun            bool                   `json:"dry_run"`
	Variables         map[string]interface{} `json:"variables,omitempty"`
	Filters           []string               `json:"filters,omitempty"`
	PostProcessors    []string               `json:"post_processors,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// =============================================================================
// AST and Source Code Types
// =============================================================================

// SourceLocation represents a location in source code
type SourceLocation struct {
	File      string `json:"file"`
	Package   string `json:"package"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	EndLine   int    `json:"end_line,omitempty"`
	EndColumn int    `json:"end_column,omitempty"`
	Function  string `json:"function,omitempty"`
	Receiver  string `json:"receiver,omitempty"`
}

// ASTDirectiveExtractor extracts directives from AST
type ASTDirectiveExtractor struct {
	parsers  []DirectiveParser
	config   *DirectiveConfig
	fileSet  *token.FileSet
	patterns []*DirectivePattern
	cache    map[string][]*Directive
}

// DirectivePattern defines patterns for matching directives
type DirectivePattern struct {
	Name        string         `json:"name"`
	Pattern     *regexp.Regexp `json:"-"`
	PatternStr  string         `json:"pattern"`
	Type        DirectiveType  `json:"type"`
	Required    bool           `json:"required"`
	Repeatable  bool           `json:"repeatable"`
	Context     []string       `json:"context"` // function, type, file, package
	Precedence  int            `json:"precedence"`
	Validator   string         `json:"validator,omitempty"`
	Transformer string         `json:"transformer,omitempty"`
}

// DirectiveContext represents the context where a directive was found
type DirectiveContext struct {
	Type        string                 `json:"type"` // function, type, field, file, package
	Name        string                 `json:"name"`
	Package     string                 `json:"package"`
	File        string                 `json:"file"`
	Node        ast.Node               `json:"-"`
	Parent      ast.Node               `json:"-"`
	Comments    []*ast.CommentGroup    `json:"-"`
	Position    token.Pos              `json:"-"`
	Annotations []string               `json:"annotations"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// =============================================================================
// Cache and Utility Types
// =============================================================================

// DirectiveCache manages caching of directive processing results
type DirectiveCache struct {
	entries         map[string]*DirectiveCacheEntry
	maxEntries      int
	ttl             time.Duration
	hits            int64
	misses          int64
	cleanupInterval time.Duration
}

// DirectiveCacheEntry represents a cache entry
type DirectiveCacheEntry struct {
	Key         string             `json:"key"`
	Directives  []*Directive       `json:"directives"`
	Errors      []DirectiveError   `json:"errors"`
	Warnings    []DirectiveWarning `json:"warnings"`
	CreatedAt   time.Time          `json:"created_at"`
	ExpiresAt   time.Time          `json:"expires_at"`
	AccessedAt  time.Time          `json:"accessed_at"`
	AccessCount int64              `json:"access_count"`
	FileHash    string             `json:"file_hash,omitempty"`
}

// ErrorSeverity defines the severity level of errors
type ErrorSeverity string

const (
	SeverityError   ErrorSeverity = "error"
	SeverityWarning ErrorSeverity = "warning"
	SeverityInfo    ErrorSeverity = "info"
	SeverityDebug   ErrorSeverity = "debug"
)

// =============================================================================
// Predefined Directive Patterns
// =============================================================================

// Common directive prefixes and patterns
const (
	APIDirectivePrefix      = "@api"
	EndpointDirectivePrefix = "@endpoint"
	HandlerDirectivePrefix  = "@handler"
	AuthDirectivePrefix     = "@auth"
	ValidationPrefix        = "@validate"
	CacheDirectivePrefix    = "@cache"
	RateLimitPrefix         = "@ratelimit"
	MetricsPrefix           = "@metrics"
	LoggingPrefix           = "@log"
	DocPrefix               = "@doc"
	ExamplePrefix           = "@example"
	DeprecatedPrefix        = "@deprecated"
	InternalPrefix          = "@internal"
	GeneratePrefix          = "@generate"
	ConfigPrefix            = "@config"
	MetadataPrefix          = "@meta"
)

// DefaultDirectivePatterns contains common directive patterns
var DefaultDirectivePatterns = []*DirectivePattern{
	{
		Name:       "api",
		PatternStr: `@api\s+(.+)`,
		Type:       DirectiveTypeAPI,
		Context:    []string{"package", "file"},
		Precedence: 100,
	},
	{
		Name:       "endpoint",
		PatternStr: `@endpoint\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+(.+)`,
		Type:       DirectiveTypeEndpoint,
		Context:    []string{"function"},
		Precedence: 90,
	},
	{
		Name:       "handler",
		PatternStr: `@handler\s+(.+)`,
		Type:       DirectiveTypeHandler,
		Context:    []string{"function"},
		Precedence: 80,
	},
	{
		Name:       "auth",
		PatternStr: `@auth\s+(.*)`,
		Type:       DirectiveTypeAuth,
		Context:    []string{"function", "type"},
		Precedence: 70,
	},
	{
		Name:       "validate",
		PatternStr: `@validate\s+(.+)`,
		Type:       DirectiveTypeValidation,
		Context:    []string{"function", "field"},
		Repeatable: true,
		Precedence: 60,
	},
	{
		Name:       "cache",
		PatternStr: `@cache\s*(.*)`,
		Type:       DirectiveTypeConfig,
		Context:    []string{"function"},
		Precedence: 50,
	},
	{
		Name:       "doc",
		PatternStr: `@doc\s+(.+)`,
		Type:       DirectiveTypeDocumentation,
		Context:    []string{"function", "type", "field"},
		Precedence: 40,
	},
	{
		Name:       "example",
		PatternStr: `@example\s+(.+)`,
		Type:       DirectiveTypeDocumentation,
		Context:    []string{"function"},
		Repeatable: true,
		Precedence: 30,
	},
	{
		Name:       "deprecated",
		PatternStr: `@deprecated\s*(.*)`,
		Type:       DirectiveTypeMetadata,
		Context:    []string{"function", "type"},
		Precedence: 20,
	},
	{
		Name:       "internal",
		PatternStr: `@internal\s*(.*)`,
		Type:       DirectiveTypeMetadata,
		Context:    []string{"function", "type"},
		Precedence: 10,
	},
}
