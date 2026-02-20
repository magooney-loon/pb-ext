package api

import (
	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Route Registration
// =============================================================================

// RouteDefinition represents an explicit route definition
type RouteDefinition struct {
	Method      string
	Path        string
	Handler     func(*core.RequestEvent) error
	Middlewares []interface{}
}

// RegisterRoute explicitly registers a route with optional middleware
func (r *APIRegistry) RegisterRoute(method, path string, handler func(*core.RequestEvent) error, middlewares ...interface{}) {
	if !r.config.Enabled {
		return
	}

	helper := NewRegistryHelper()
	analysis := helper.AnalyzeRoute(method, path, handler, middlewares)

	endpoint := r.createEndpointFromAnalysis(analysis)
	r.enhanceEndpointWithAST(&endpoint)
	r.RegisterEndpoint(endpoint)
}

// RegisterExplicitRoute registers a route with explicit information (no inference)
func (r *APIRegistry) RegisterExplicitRoute(endpoint APIEndpoint) {
	if !r.config.Enabled {
		return
	}

	r.enhanceEndpointWithAST(&endpoint)
	r.RegisterEndpoint(endpoint)
}

// BatchRegisterRoutes registers multiple routes at once
func (r *APIRegistry) BatchRegisterRoutes(routes []RouteDefinition) {
	if !r.config.Enabled {
		return
	}

	for _, route := range routes {
		r.RegisterRoute(route.Method, route.Path, route.Handler, route.Middlewares...)
	}
}

// =============================================================================
// Private Helpers
// =============================================================================

// createEndpointFromAnalysis creates an APIEndpoint from route analysis
func (r *APIRegistry) createEndpointFromAnalysis(analysis *RouteAnalysis) APIEndpoint {
	return APIEndpoint{
		Method:      analysis.Method,
		Path:        analysis.Path,
		Description: analysis.Description,
		Tags:        analysis.Tags,
		Handler:     analysis.Handler.FullName,
		Auth:        analysis.Auth,
	}
}

// enhanceEndpointWithAST enhances an endpoint with AST-extracted schema information
func (r *APIRegistry) enhanceEndpointWithAST(endpoint *APIEndpoint) {
	if r.astParser != nil {
		// Try multiple handler name variations for better matching
		handlerNames := []string{
			endpoint.Handler,
			ExtractHandlerBaseName(endpoint.Handler, false),
			ExtractHandlerBaseName(endpoint.Handler, true),
		}

		enhanced := false
		for _, handlerName := range handlerNames {
			if handlerInfo, exists := r.astParser.GetHandlerByName(handlerName); exists {
				if handlerInfo.APIDescription != "" {
					endpoint.Description = handlerInfo.APIDescription
				}
				if len(handlerInfo.APITags) > 0 {
					endpoint.Tags = handlerInfo.APITags
				}

				if handlerInfo.RequiresAuth {
					endpoint.Auth = &AuthInfo{
						Required:    true,
						Type:        handlerInfo.AuthType,
						Description: r.getASTAuthDescription(handlerInfo.AuthType),
					}
				}

				if handlerInfo.RequestSchema != nil {
					endpoint.Request = handlerInfo.RequestSchema
				}
				if handlerInfo.ResponseSchema != nil {
					endpoint.Response = handlerInfo.ResponseSchema
				}

				if len(handlerInfo.Parameters) > 0 {
					endpoint.Parameters = handlerInfo.Parameters
				}

				enhanced = true
				break
			}
		}

		// Fallback to generic AST enhancement if specific handler not found
		if !enhanced {
			if err := r.astParser.EnhanceEndpoint(endpoint); err != nil {
				// AST enhancement is optional; log error but don't fail
			}
		}
	}

	if r.schemaGenerator != nil {
		// Only generate request schema if AST didn't provide one
		if endpoint.Request == nil {
			if requestSchema, err := r.schemaGenerator.AnalyzeRequestSchema(endpoint); err == nil {
				endpoint.Request = requestSchema
			}
		}

		// Only generate response schema if AST didn't provide one
		if endpoint.Response == nil {
			if responseSchema, err := r.schemaGenerator.AnalyzeResponseSchema(endpoint); err == nil {
				endpoint.Response = responseSchema
			}
		}
	}
}

// getASTAuthDescription returns a user-friendly auth description for an AST-detected auth type
func (r *APIRegistry) getASTAuthDescription(authType string) string {
	descriptions := map[string]string{
		"guest_only":         "Requires no authentication (guest access only)",
		"auth":               "Requires user authentication",
		"superuser":          "Requires superuser privileges",
		"superuser_or_owner": "Requires superuser privileges or resource ownership",
	}

	if desc, exists := descriptions[authType]; exists {
		return desc
	}
	return "Authentication required"
}
