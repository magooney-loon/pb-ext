package versioning

import (
	"sync"
	"time"

	"github.com/magooney-loon/pb-ext/core/api"
)

// =============================================================================
// Version Management Types (simplified & OpenAPI compatible)
// =============================================================================

// Manager manages multiple API versions with separate registries
type Manager struct {
	mu             sync.RWMutex
	versions       []string                 // ordered list of versions
	defaultVersion string                   // default version to use
	registries     map[string]*api.Registry // separate registry per version
	configs        map[string]*api.Config   // version-specific configs
	createdAt      time.Time                // when manager was created
	lastModified   time.Time                // last time versions were modified
}

// Info contains information about a specific API version
type Info struct {
	Version     string            `json:"version"`
	Status      string            `json:"status"` // "stable", "beta", "deprecated"
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Config      *api.Config       `json:"config"`
	Stats       map[string]int    `json:"stats"`
	Endpoints   int               `json:"endpoints"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Router provides version-specific route registration
type Router struct {
	*AutoRouter
	version  string
	manager  *Manager
	registry *api.Registry // version-specific registry
}

// AutoRouter wraps HTTP router with auto-documentation
type AutoRouter struct {
	router    interface{} // HTTP router (echo, gin, etc.)
	registry  *api.Registry
	parser    api.ASTParser
	generator api.SchemaGenerator
	discovery api.DiscoveryEngine
}

// =============================================================================
// Discovery & Generation Components (simplified)
// =============================================================================

// ASTParser implements simplified AST parsing for Go code
type ASTParser struct {
	mu       sync.RWMutex
	handlers map[string]api.HandlerInfo
	structs  map[string]api.StructInfo
	errors   []error
}

// SchemaGenerator implements simplified schema generation
type SchemaGenerator struct {
	mu      sync.RWMutex
	schemas map[string]*api.SchemaInfo
	parser  *ASTParser
}

// DiscoveryEngine implements automatic endpoint discovery
type DiscoveryEngine struct {
	parser    api.ASTParser
	generator api.SchemaGenerator
	config    *api.DiscoveryConfig
}

// OpenAPIGenerator generates OpenAPI 3.0 specs
type OpenAPIGenerator struct {
	config *api.Config
}

// =============================================================================
// Registry Operations (simplified)
// =============================================================================

// RegistryStats contains registry statistics
type RegistryStats struct {
	TotalEndpoints  int               `json:"total_endpoints"`
	EndpointsByTag  map[string]int    `json:"endpoints_by_tag"`
	EndpointsByAuth map[string]int    `json:"endpoints_by_auth"`
	LastDiscovered  time.Time         `json:"last_discovered"`
	DiscoveryErrors int               `json:"discovery_errors"`
	GenerationTime  time.Duration     `json:"generation_time"`
	LastGenerated   time.Time         `json:"last_generated"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// =============================================================================
// Auto-Discovery Configuration (simplified)
// =============================================================================

// AutoDiscoveryOptions configures automatic API endpoint discovery
type AutoDiscoveryOptions struct {
	Enabled         bool     `json:"enabled"`
	PackagePaths    []string `json:"package_paths"`
	AnalyzeHandlers bool     `json:"analyze_handlers"`
	GenerateSchemas bool     `json:"generate_schemas"`
	DetectAuth      bool     `json:"detect_auth"`
	GenerateTags    bool     `json:"generate_tags"`
	IncludeInternal bool     `json:"include_internal"`
	RefreshInterval int      `json:"refresh_interval_minutes"` // auto-refresh interval
}

// =============================================================================
// Error Types
// =============================================================================

// Error represents API system errors
type Error struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	Time    time.Time `json:"time"`
}

// Common error codes
const (
	ErrVersionNotFound  = "VERSION_NOT_FOUND"
	ErrVersionExists    = "VERSION_EXISTS"
	ErrInvalidVersion   = "INVALID_VERSION"
	ErrEndpointExists   = "ENDPOINT_EXISTS"
	ErrEndpointNotFound = "ENDPOINT_NOT_FOUND"
	ErrParsingFailed    = "PARSING_FAILED"
	ErrSchemaGenFailed  = "SCHEMA_GENERATION_FAILED"
	ErrDiscoveryFailed  = "DISCOVERY_FAILED"
	ErrRegistryLocked   = "REGISTRY_LOCKED"
)

// =============================================================================
// Configuration Builders (simplified)
// =============================================================================

// DefaultVersionConfig returns a default version configuration
func DefaultVersionConfig(version string) *api.Config {
	return &api.Config{
		Title:       "pb-ext API",
		Version:     version,
		Description: "Auto-discovered API endpoints for version " + version,
		BaseURL:     "/api/" + version,
		Enabled:     true,
	}
}

// DefaultAutoDiscoveryOptions returns default auto-discovery options
func DefaultAutoDiscoveryOptions() *AutoDiscoveryOptions {
	return &AutoDiscoveryOptions{
		Enabled:         true,
		PackagePaths:    []string{"./"},
		AnalyzeHandlers: true,
		GenerateSchemas: true,
		DetectAuth:      true,
		GenerateTags:    true,
		IncludeInternal: false,
		RefreshInterval: 0, // disabled by default
	}
}

// =============================================================================
// Version Status Constants
// =============================================================================

const (
	StatusStable     = "stable"
	StatusBeta       = "beta"
	StatusAlpha      = "alpha"
	StatusDeprecated = "deprecated"
	StatusDev        = "development"
)

// =============================================================================
// Common Security Schemes for OpenAPI
// =============================================================================

// GetSecuritySchemes returns common PocketBase security schemes for OpenAPI
func GetSecuritySchemes() map[string]interface{} {
	return map[string]interface{}{
		"bearerAuth": map[string]interface{}{
			"type":        "http",
			"scheme":      "bearer",
			"description": "JWT token authentication",
		},
		"adminAuth": map[string]interface{}{
			"type":        "http",
			"scheme":      "bearer",
			"description": "Admin/superuser JWT token",
		},
	}
}

// =============================================================================
// HTTP Method Constants
// =============================================================================

const (
	MethodGET     = "GET"
	MethodPOST    = "POST"
	MethodPUT     = "PUT"
	MethodDELETE  = "DELETE"
	MethodPATCH   = "PATCH"
	MethodHEAD    = "HEAD"
	MethodOPTIONS = "OPTIONS"
)

// ValidHTTPMethods returns list of valid HTTP methods
func ValidHTTPMethods() []string {
	return []string{
		MethodGET, MethodPOST, MethodPUT, MethodDELETE,
		MethodPATCH, MethodHEAD, MethodOPTIONS,
	}
}
