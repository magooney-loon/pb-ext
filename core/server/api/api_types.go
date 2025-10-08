package api

import (
	"github.com/pocketbase/pocketbase/core"
)

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
	Title       string             `json:"title"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	BaseURL     string             `json:"base_url"`
	Endpoints   []APIEndpoint      `json:"endpoints"`
	Generated   string             `json:"generated_at"`
	Components  *OpenAPIComponents `json:"components,omitempty"`
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
// Configuration Types
// =============================================================================

// APIDocsConfig holds configuration for the API documentation system
type APIDocsConfig struct {
	Title         string               `json:"title"`
	Version       string               `json:"version"`
	Description   string               `json:"description"`
	Status        string               `json:"status,omitempty"` // "stable", "development", "deprecated", "beta", etc.
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
		Status:      "stable",
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
