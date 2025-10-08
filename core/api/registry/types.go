package registry

import (
	"sync"
	"time"

	"github.com/magooney-loon/pb-ext/core/api"
)

// =============================================================================
// Registry Implementation (simplified & OpenAPI compatible)
// =============================================================================

// Registry manages API endpoints for a single version
type Registry struct {
	mu        sync.RWMutex
	config    *api.Config
	endpoints map[string]api.APIEndpoint // key: method:path
	schemas   map[string]*api.SchemaInfo
	tags      map[string]TagInfo
	stats     *Stats
	metadata  map[string]string
	createdAt time.Time
	updatedAt time.Time
}

// Stats contains registry statistics
type Stats struct {
	TotalEndpoints  int               `json:"total_endpoints"`
	EndpointsByTag  map[string]int    `json:"endpoints_by_tag"`
	EndpointsByAuth map[string]int    `json:"endpoints_by_auth"`
	LastDiscovered  time.Time         `json:"last_discovered"`
	DiscoveryErrors int               `json:"discovery_errors"`
	GenerationTime  time.Duration     `json:"generation_time"`
	LastGenerated   time.Time         `json:"last_generated"`
	CacheHitRatio   float64           `json:"cache_hit_ratio"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// TagInfo contains information about endpoint tags
type TagInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Count       int    `json:"count"`
	Color       string `json:"color,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// =============================================================================
// Registry Operations
// =============================================================================

// EndpointKey generates a unique key for an endpoint
func EndpointKey(method, path string) string {
	return method + ":" + path
}

// RegistryOperation represents an operation performed on the registry
type RegistryOperation struct {
	Type      OperationType     `json:"type"`
	Endpoint  string            `json:"endpoint,omitempty"`
	Schema    string            `json:"schema,omitempty"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	Duration  time.Duration     `json:"duration"`
	Timestamp time.Time         `json:"timestamp"`
	UserAgent string            `json:"user_agent,omitempty"`
	IPAddress string            `json:"ip_address,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// OperationType represents the type of registry operation
type OperationType string

const (
	OperationRegister   OperationType = "register"
	OperationUnregister OperationType = "unregister"
	OperationUpdate     OperationType = "update"
	OperationQuery      OperationType = "query"
	OperationGenerate   OperationType = "generate"
	OperationValidate   OperationType = "validate"
	OperationDiscover   OperationType = "discover"
	OperationClear      OperationType = "clear"
	OperationExport     OperationType = "export"
	OperationImport     OperationType = "import"
)

// =============================================================================
// Query and Filter Types
// =============================================================================

// Query represents a registry query
type Query struct {
	Methods    []string          `json:"methods,omitempty"`
	Paths      []string          `json:"paths,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	AuthTypes  []string          `json:"auth_types,omitempty"`
	Handlers   []string          `json:"handlers,omitempty"`
	SearchTerm string            `json:"search_term,omitempty"`
	Limit      int               `json:"limit,omitempty"`
	Offset     int               `json:"offset,omitempty"`
	SortBy     string            `json:"sort_by,omitempty"`
	SortOrder  SortOrder         `json:"sort_order,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// SortOrder represents sort direction
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// QueryResult contains query results
type QueryResult struct {
	Endpoints  []api.APIEndpoint `json:"endpoints"`
	TotalCount int               `json:"total_count"`
	HasMore    bool              `json:"has_more"`
	Query      *Query            `json:"query"`
	Duration   time.Duration     `json:"duration"`
	Timestamp  time.Time         `json:"timestamp"`
	CacheHit   bool              `json:"cache_hit,omitempty"`
}

// =============================================================================
// Export/Import Types
// =============================================================================

// Export contains exported registry data
type Export struct {
	Version    string                     `json:"version"`
	Config     *api.Config                `json:"config"`
	Endpoints  []api.APIEndpoint          `json:"endpoints"`
	Schemas    map[string]*api.SchemaInfo `json:"schemas"`
	Tags       map[string]TagInfo         `json:"tags"`
	Stats      *Stats                     `json:"stats"`
	ExportedAt time.Time                  `json:"exported_at"`
	ExportedBy string                     `json:"exported_by,omitempty"`
	Metadata   map[string]string          `json:"metadata,omitempty"`
}

// ImportOptions configures import behavior
type ImportOptions struct {
	MergeMode         MergeMode `json:"merge_mode"`
	OverwriteExisting bool      `json:"overwrite_existing"`
	ValidateSchemas   bool      `json:"validate_schemas"`
	PreserveMeta      bool      `json:"preserve_metadata"`
	SkipValidation    bool      `json:"skip_validation"`
	DryRun            bool      `json:"dry_run"`
}

// MergeMode defines how to merge imported data
type MergeMode string

const (
	MergeReplace MergeMode = "replace"
	MergeAppend  MergeMode = "append"
	MergeMerge   MergeMode = "merge"
	MergeSkip    MergeMode = "skip"
)

// ImportResult contains import operation results
type ImportResult struct {
	Success          bool              `json:"success"`
	EndpointsAdded   int               `json:"endpoints_added"`
	EndpointsUpdated int               `json:"endpoints_updated"`
	EndpointsSkipped int               `json:"endpoints_skipped"`
	SchemasAdded     int               `json:"schemas_added"`
	SchemasUpdated   int               `json:"schemas_updated"`
	TagsAdded        int               `json:"tags_added"`
	Errors           []string          `json:"errors,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	Duration         time.Duration     `json:"duration"`
	ImportedAt       time.Time         `json:"imported_at"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// =============================================================================
// Validation Types
// =============================================================================

// ValidationResult contains registry validation results
type ValidationResult struct {
	Valid          bool                      `json:"valid"`
	EndpointErrors []EndpointValidationError `json:"endpoint_errors,omitempty"`
	SchemaErrors   []SchemaValidationError   `json:"schema_errors,omitempty"`
	ConfigErrors   []string                  `json:"config_errors,omitempty"`
	Warnings       []string                  `json:"warnings,omitempty"`
	Suggestions    []string                  `json:"suggestions,omitempty"`
	ValidationTime time.Duration             `json:"validation_time"`
	ValidatedAt    time.Time                 `json:"validated_at"`
	ValidatorInfo  string                    `json:"validator_info,omitempty"`
}

// EndpointValidationError represents an endpoint validation error
type EndpointValidationError struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	Handler    string `json:"handler,omitempty"`
	Error      string `json:"error"`
	Suggestion string `json:"suggestion,omitempty"`
	Severity   string `json:"severity"` // "error", "warning", "info"
	CanAutoFix bool   `json:"can_auto_fix"`
}

// SchemaValidationError represents a schema validation error
type SchemaValidationError struct {
	SchemaName string `json:"schema_name"`
	Field      string `json:"field,omitempty"`
	Error      string `json:"error"`
	Suggestion string `json:"suggestion,omitempty"`
	Severity   string `json:"severity"`
	CanAutoFix bool   `json:"can_auto_fix"`
}

// =============================================================================
// Cache Types
// =============================================================================

// CacheEntry represents a cached item
type CacheEntry struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	Size       int64       `json:"size"`
	CreatedAt  time.Time   `json:"created_at"`
	ExpiresAt  time.Time   `json:"expires_at"`
	AccessedAt time.Time   `json:"accessed_at"`
	HitCount   int64       `json:"hit_count"`
	Tags       []string    `json:"tags,omitempty"`
}

// CacheStats contains cache statistics
type CacheStats struct {
	TotalEntries  int64         `json:"total_entries"`
	TotalSize     int64         `json:"total_size"`
	HitCount      int64         `json:"hit_count"`
	MissCount     int64         `json:"miss_count"`
	HitRatio      float64       `json:"hit_ratio"`
	EvictionCount int64         `json:"eviction_count"`
	LastEviction  time.Time     `json:"last_eviction"`
	MaxSize       int64         `json:"max_size"`
	TTL           time.Duration `json:"ttl"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// =============================================================================
// Configuration Types
// =============================================================================

// Config contains registry configuration
type Config struct {
	MaxEndpoints      int               `json:"max_endpoints"`
	MaxSchemas        int               `json:"max_schemas"`
	CacheEnabled      bool              `json:"cache_enabled"`
	CacheSize         int64             `json:"cache_size"`
	CacheTTL          time.Duration     `json:"cache_ttl"`
	ValidationEnabled bool              `json:"validation_enabled"`
	StrictMode        bool              `json:"strict_mode"`
	AutoDiscovery     bool              `json:"auto_discovery"`
	BackupEnabled     bool              `json:"backup_enabled"`
	BackupInterval    time.Duration     `json:"backup_interval"`
	MetricsEnabled    bool              `json:"metrics_enabled"`
	LogLevel          string            `json:"log_level"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// =============================================================================
// Health and Status Types
// =============================================================================

// Health represents registry health status
type Health struct {
	Status           HealthStatus      `json:"status"`
	Uptime           time.Duration     `json:"uptime"`
	Version          string            `json:"version"`
	EndpointCount    int               `json:"endpoint_count"`
	SchemaCount      int               `json:"schema_count"`
	ErrorCount       int               `json:"error_count"`
	LastError        string            `json:"last_error,omitempty"`
	LastErrorTime    time.Time         `json:"last_error_time,omitempty"`
	MemoryUsage      int64             `json:"memory_usage_bytes"`
	GoroutineCount   int               `json:"goroutine_count"`
	LastValidation   time.Time         `json:"last_validation"`
	ValidationErrors int               `json:"validation_errors"`
	CacheHealth      *CacheStats       `json:"cache_health,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	CheckedAt        time.Time         `json:"checked_at"`
}

// HealthStatus represents health status
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)

// =============================================================================
// Event Types
// =============================================================================

// Event represents a registry event
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"`
	Subject   string                 `json:"subject,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// EventType represents the type of event
type EventType string

const (
	EventEndpointAdded      EventType = "endpoint.added"
	EventEndpointRemoved    EventType = "endpoint.removed"
	EventEndpointUpdated    EventType = "endpoint.updated"
	EventSchemaAdded        EventType = "schema.added"
	EventSchemaRemoved      EventType = "schema.removed"
	EventSchemaUpdated      EventType = "schema.updated"
	EventRegistryCleared    EventType = "registry.cleared"
	EventValidationFailed   EventType = "validation.failed"
	EventDiscoveryStarted   EventType = "discovery.started"
	EventDiscoveryCompleted EventType = "discovery.completed"
	EventDiscoveryFailed    EventType = "discovery.failed"
	EventCacheEviction      EventType = "cache.eviction"
	EventHealthChanged      EventType = "health.changed"
	EventConfigChanged      EventType = "config.changed"
)

// =============================================================================
// Error Types
// =============================================================================

// Error represents registry errors
type Error struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Details   string            `json:"details,omitempty"`
	Endpoint  string            `json:"endpoint,omitempty"`
	Schema    string            `json:"schema,omitempty"`
	Operation string            `json:"operation,omitempty"`
	Time      time.Time         `json:"time"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Common error codes
const (
	ErrEndpointExists        = "ENDPOINT_EXISTS"
	ErrEndpointNotFound      = "ENDPOINT_NOT_FOUND"
	ErrSchemaExists          = "SCHEMA_EXISTS"
	ErrSchemaNotFound        = "SCHEMA_NOT_FOUND"
	ErrInvalidEndpoint       = "INVALID_ENDPOINT"
	ErrInvalidSchema         = "INVALID_SCHEMA"
	ErrInvalidConfig         = "INVALID_CONFIG"
	ErrRegistryFull          = "REGISTRY_FULL"
	ErrRegistryLocked        = "REGISTRY_LOCKED"
	ErrValidationFailed      = "VALIDATION_FAILED"
	ErrSerializationFailed   = "SERIALIZATION_FAILED"
	ErrDeserializationFailed = "DESERIALIZATION_FAILED"
	ErrCacheFailure          = "CACHE_FAILURE"
	ErrDiscoveryFailed       = "DISCOVERY_FAILED"
	ErrExportFailed          = "EXPORT_FAILED"
	ErrImportFailed          = "IMPORT_FAILED"
)

// =============================================================================
// Utility Types
// =============================================================================

// BatchOperation represents a batch of operations
type BatchOperation struct {
	Operations []RegistryOperation `json:"operations"`
	Atomic     bool                `json:"atomic"`
	DryRun     bool                `json:"dry_run"`
	Metadata   map[string]string   `json:"metadata,omitempty"`
}

// BatchResult contains batch operation results
type BatchResult struct {
	Success        bool              `json:"success"`
	SuccessCount   int               `json:"success_count"`
	ErrorCount     int               `json:"error_count"`
	Results        []OperationResult `json:"results"`
	Duration       time.Duration     `json:"duration"`
	ExecutedAt     time.Time         `json:"executed_at"`
	AtomicRollback bool              `json:"atomic_rollback,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// OperationResult represents the result of a single operation
type OperationResult struct {
	Operation RegistryOperation `json:"operation"`
	Success   bool              `json:"success"`
	Error     string            `json:"error,omitempty"`
	Duration  time.Duration     `json:"duration"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// =============================================================================
// Default Configurations
// =============================================================================

// DefaultConfig returns a default registry configuration
func DefaultConfig() *Config {
	return &Config{
		MaxEndpoints:      1000,
		MaxSchemas:        500,
		CacheEnabled:      true,
		CacheSize:         100 * 1024 * 1024, // 100MB
		CacheTTL:          30 * time.Minute,
		ValidationEnabled: true,
		StrictMode:        false,
		AutoDiscovery:     true,
		BackupEnabled:     false,
		BackupInterval:    24 * time.Hour,
		MetricsEnabled:    true,
		LogLevel:          "info",
	}
}

// DefaultImportOptions returns default import options
func DefaultImportOptions() *ImportOptions {
	return &ImportOptions{
		MergeMode:         MergeMerge,
		OverwriteExisting: false,
		ValidateSchemas:   true,
		PreserveMeta:      true,
		SkipValidation:    false,
		DryRun:            false,
	}
}

// =============================================================================
// Constants
// =============================================================================

const (
	// Version represents the registry format version
	Version = "1.0.0"

	// Default limits
	DefaultMaxEndpoints = 1000
	DefaultMaxSchemas   = 500
	DefaultCacheSize    = 100 * 1024 * 1024 // 100MB
	DefaultCacheTTL     = 30 * time.Minute

	// Query limits
	MaxQueryLimit     = 1000
	DefaultQueryLimit = 50
	MaxSearchTermLen  = 200

	// Validation
	MaxEndpointPathLen = 500
	MaxDescriptionLen  = 2000
	MaxTagNameLen      = 50

	// Export/Import
	MaxExportSize = 500 * 1024 * 1024 // 500MB
)
