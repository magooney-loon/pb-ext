package api

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

// OpenAPIContact represents contact information in OpenAPI 3.0 spec
type OpenAPIContact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// OpenAPILicense represents license information in OpenAPI 3.0 spec
type OpenAPILicense struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// OpenAPIInfo represents the info object in OpenAPI 3.0 spec
type OpenAPIInfo struct {
	Title          string          `json:"title"`
	Version        string          `json:"version"`
	Description    string          `json:"description,omitempty"`
	TermsOfService string          `json:"termsOfService,omitempty"`
	Contact        *OpenAPIContact `json:"contact,omitempty"`
	License        *OpenAPILicense `json:"license,omitempty"`
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

// OpenAPITag represents a tag in OpenAPI 3.0 spec
type OpenAPITag struct {
	Name         string               `json:"name"`
	Description  string               `json:"description,omitempty"`
	ExternalDocs *OpenAPIExternalDocs `json:"externalDocs,omitempty"`
}

// =============================================================================
// Registry and Management Types
// =============================================================================

// =============================================================================
// Router and Route Types
// =============================================================================

// AutoAPIRouter wraps PocketBase router for automatic API documentation

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

	// Contact info shown in the OpenAPI spec
	ContactName  string `json:"contact_name,omitempty"`  // default: "API Support"
	ContactEmail string `json:"contact_email,omitempty"` // e.g. "support@example.com"
	ContactURL   string `json:"contact_url,omitempty"`   // e.g. "https://example.com/support"

	// License info shown in the OpenAPI spec
	LicenseName string `json:"license_name,omitempty"` // e.g. "Apache 2.0"
	LicenseURL  string `json:"license_url,omitempty"`  // e.g. "https://www.apache.org/licenses/LICENSE-2.0.html"

	// Additional OpenAPI info fields
	TermsOfService   string `json:"terms_of_service,omitempty"`   // URL to Terms of Service
	ExternalDocsURL  string `json:"external_docs_url,omitempty"`  // URL for external documentation
	ExternalDocsDesc string `json:"external_docs_desc,omitempty"` // Description for external docs link

	// Swagger UI visibility
	PublicSwagger bool `json:"public_swagger,omitempty"` // If true, serves Swagger UI publicly at /api/docs/{version}/swagger
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
