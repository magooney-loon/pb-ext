package api

import (
	"encoding/json"
	"strings"
)

// =============================================================================
// OpenAPI 3.0 Schema Types
// =============================================================================

// OpenAPISchema represents an OpenAPI 3.0 schema object
type OpenAPISchema struct {
	Type        string                     `json:"type,omitempty"`
	Format      string                     `json:"format,omitempty"`
	Title       string                     `json:"title,omitempty"`
	Description string                     `json:"description,omitempty"`
	Default     interface{}                `json:"default,omitempty"`
	Example     interface{}                `json:"example,omitempty"`
	Examples    map[string]*OpenAPIExample `json:"examples,omitempty"`

	// Validation keywords
	MultipleOf       *float64      `json:"multipleOf,omitempty"`
	Maximum          *float64      `json:"maximum,omitempty"`
	ExclusiveMaximum *bool         `json:"exclusiveMaximum,omitempty"`
	Minimum          *float64      `json:"minimum,omitempty"`
	ExclusiveMinimum *bool         `json:"exclusiveMinimum,omitempty"`
	MaxLength        *int          `json:"maxLength,omitempty"`
	MinLength        *int          `json:"minLength,omitempty"`
	Pattern          string        `json:"pattern,omitempty"`
	MaxItems         *int          `json:"maxItems,omitempty"`
	MinItems         *int          `json:"minItems,omitempty"`
	UniqueItems      *bool         `json:"uniqueItems,omitempty"`
	MaxProperties    *int          `json:"maxProperties,omitempty"`
	MinProperties    *int          `json:"minProperties,omitempty"`
	Required         []string      `json:"required,omitempty"`
	Enum             []interface{} `json:"enum,omitempty"`

	// Object properties
	Properties           map[string]*OpenAPISchema `json:"properties,omitempty"`
	AdditionalProperties interface{}               `json:"additionalProperties,omitempty"` // bool or Schema

	// Array items
	Items *OpenAPISchema `json:"items,omitempty"`

	// Composition
	AllOf []*OpenAPISchema `json:"allOf,omitempty"`
	OneOf []*OpenAPISchema `json:"oneOf,omitempty"`
	AnyOf []*OpenAPISchema `json:"anyOf,omitempty"`
	Not   *OpenAPISchema   `json:"not,omitempty"`

	// Reference
	Ref string `json:"$ref,omitempty"`

	// Discriminator for inheritance
	Discriminator *OpenAPIDiscriminator `json:"discriminator,omitempty"`

	// Read/Write only
	ReadOnly  *bool `json:"readOnly,omitempty"`
	WriteOnly *bool `json:"writeOnly,omitempty"`

	// Deprecated
	Deprecated *bool `json:"deprecated,omitempty"`

	// External documentation
	ExternalDocs *OpenAPIExternalDocs `json:"externalDocs,omitempty"`

	// Nullable (OpenAPI 3.0)
	Nullable *bool `json:"nullable,omitempty"`

	// Extensions (x-* properties)
	Extensions map[string]interface{} `json:"-"`
}

// OpenAPIExample represents an OpenAPI example object
type OpenAPIExample struct {
	Summary       string      `json:"summary,omitempty"`
	Description   string      `json:"description,omitempty"`
	Value         interface{} `json:"value,omitempty"`
	ExternalValue string      `json:"externalValue,omitempty"`
}

// OpenAPIDiscriminator represents an OpenAPI discriminator object
type OpenAPIDiscriminator struct {
	PropertyName string            `json:"propertyName"`
	Mapping      map[string]string `json:"mapping,omitempty"`
}

// OpenAPIExternalDocs represents an OpenAPI external documentation object
type OpenAPIExternalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

// OpenAPIParameter represents an OpenAPI parameter object
type OpenAPIParameter struct {
	Name            string `json:"name"`
	In              string `json:"in"` // "query", "header", "path", "cookie"
	Description     string `json:"description,omitempty"`
	Required        *bool  `json:"required,omitempty"`
	Deprecated      *bool  `json:"deprecated,omitempty"`
	AllowEmptyValue *bool  `json:"allowEmptyValue,omitempty"`

	// Style and explode for serialization
	Style   string `json:"style,omitempty"`
	Explode *bool  `json:"explode,omitempty"`

	// Schema or content
	Schema  *OpenAPISchema               `json:"schema,omitempty"`
	Content map[string]*OpenAPIMediaType `json:"content,omitempty"`

	// Example
	Example  interface{}                `json:"example,omitempty"`
	Examples map[string]*OpenAPIExample `json:"examples,omitempty"`

	// Reference
	Ref string `json:"$ref,omitempty"`
}

// OpenAPIMediaType represents an OpenAPI media type object
type OpenAPIMediaType struct {
	Schema   *OpenAPISchema              `json:"schema,omitempty"`
	Example  interface{}                 `json:"example,omitempty"`
	Examples map[string]*OpenAPIExample  `json:"examples,omitempty"`
	Encoding map[string]*OpenAPIEncoding `json:"encoding,omitempty"`
}

// OpenAPIEncoding represents an OpenAPI encoding object
type OpenAPIEncoding struct {
	ContentType   string                    `json:"contentType,omitempty"`
	Headers       map[string]*OpenAPIHeader `json:"headers,omitempty"`
	Style         string                    `json:"style,omitempty"`
	Explode       *bool                     `json:"explode,omitempty"`
	AllowReserved *bool                     `json:"allowReserved,omitempty"`
}

// OpenAPIHeader represents an OpenAPI header object
type OpenAPIHeader struct {
	Description     string                       `json:"description,omitempty"`
	Required        *bool                        `json:"required,omitempty"`
	Deprecated      *bool                        `json:"deprecated,omitempty"`
	AllowEmptyValue *bool                        `json:"allowEmptyValue,omitempty"`
	Style           string                       `json:"style,omitempty"`
	Explode         *bool                        `json:"explode,omitempty"`
	Schema          *OpenAPISchema               `json:"schema,omitempty"`
	Content         map[string]*OpenAPIMediaType `json:"content,omitempty"`
	Example         interface{}                  `json:"example,omitempty"`
	Examples        map[string]*OpenAPIExample   `json:"examples,omitempty"`
}

// OpenAPIRequestBody represents an OpenAPI request body object
type OpenAPIRequestBody struct {
	Description string                       `json:"description,omitempty"`
	Content     map[string]*OpenAPIMediaType `json:"content"`
	Required    *bool                        `json:"required,omitempty"`
	Ref         string                       `json:"$ref,omitempty"`
}

// OpenAPIResponse represents an OpenAPI response object
type OpenAPIResponse struct {
	Description string                       `json:"description"`
	Headers     map[string]*OpenAPIHeader    `json:"headers,omitempty"`
	Content     map[string]*OpenAPIMediaType `json:"content,omitempty"`
	Links       map[string]*OpenAPILink      `json:"links,omitempty"`
	Ref         string                       `json:"$ref,omitempty"`
}

// OpenAPILink represents an OpenAPI link object
type OpenAPILink struct {
	OperationRef string                 `json:"operationRef,omitempty"`
	OperationId  string                 `json:"operationId,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	RequestBody  interface{}            `json:"requestBody,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Server       *OpenAPIServer         `json:"server,omitempty"`
}

// OpenAPIServer represents an OpenAPI server object
type OpenAPIServer struct {
	URL         string                            `json:"url"`
	Description string                            `json:"description,omitempty"`
	Variables   map[string]*OpenAPIServerVariable `json:"variables,omitempty"`
}

// OpenAPIServerVariable represents an OpenAPI server variable object
type OpenAPIServerVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
}

// OpenAPIComponents represents an OpenAPI components object
type OpenAPIComponents struct {
	Schemas         map[string]*OpenAPISchema         `json:"schemas,omitempty"`
	Responses       map[string]*OpenAPIResponse       `json:"responses,omitempty"`
	Parameters      map[string]*OpenAPIParameter      `json:"parameters,omitempty"`
	Examples        map[string]*OpenAPIExample        `json:"examples,omitempty"`
	RequestBodies   map[string]*OpenAPIRequestBody    `json:"requestBodies,omitempty"`
	Headers         map[string]*OpenAPIHeader         `json:"headers,omitempty"`
	SecuritySchemes map[string]*OpenAPISecurityScheme `json:"securitySchemes,omitempty"`
	Links           map[string]*OpenAPILink           `json:"links,omitempty"`
	Callbacks       map[string]*OpenAPICallback       `json:"callbacks,omitempty"`
}

// OpenAPISecurityScheme represents an OpenAPI security scheme object
type OpenAPISecurityScheme struct {
	Type             string             `json:"type"`
	Description      string             `json:"description,omitempty"`
	Name             string             `json:"name,omitempty"`
	In               string             `json:"in,omitempty"`
	Scheme           string             `json:"scheme,omitempty"`
	BearerFormat     string             `json:"bearerFormat,omitempty"`
	Flows            *OpenAPIOAuthFlows `json:"flows,omitempty"`
	OpenIdConnectUrl string             `json:"openIdConnectUrl,omitempty"`
}

// OpenAPIOAuthFlows represents an OpenAPI OAuth flows object
type OpenAPIOAuthFlows struct {
	Implicit          *OpenAPIOAuthFlow `json:"implicit,omitempty"`
	Password          *OpenAPIOAuthFlow `json:"password,omitempty"`
	ClientCredentials *OpenAPIOAuthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode *OpenAPIOAuthFlow `json:"authorizationCode,omitempty"`
}

// OpenAPIOAuthFlow represents an OpenAPI OAuth flow object
type OpenAPIOAuthFlow struct {
	AuthorizationUrl string            `json:"authorizationUrl,omitempty"`
	TokenUrl         string            `json:"tokenUrl,omitempty"`
	RefreshUrl       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

// OpenAPICallback represents an OpenAPI callback object
type OpenAPICallback map[string]*OpenAPIPathItem

// OpenAPIPathItem represents an OpenAPI path item object
type OpenAPIPathItem struct {
	Ref         string              `json:"$ref,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	Get         *OpenAPIOperation   `json:"get,omitempty"`
	Put         *OpenAPIOperation   `json:"put,omitempty"`
	Post        *OpenAPIOperation   `json:"post,omitempty"`
	Delete      *OpenAPIOperation   `json:"delete,omitempty"`
	Options     *OpenAPIOperation   `json:"options,omitempty"`
	Head        *OpenAPIOperation   `json:"head,omitempty"`
	Patch       *OpenAPIOperation   `json:"patch,omitempty"`
	Trace       *OpenAPIOperation   `json:"trace,omitempty"`
	Servers     []*OpenAPIServer    `json:"servers,omitempty"`
	Parameters  []*OpenAPIParameter `json:"parameters,omitempty"`
}

// OpenAPIOperation represents an OpenAPI operation object
type OpenAPIOperation struct {
	Tags         []string                    `json:"tags,omitempty"`
	Summary      string                      `json:"summary,omitempty"`
	Description  string                      `json:"description,omitempty"`
	ExternalDocs *OpenAPIExternalDocs        `json:"externalDocs,omitempty"`
	OperationId  string                      `json:"operationId,omitempty"`
	Parameters   []*OpenAPIParameter         `json:"parameters,omitempty"`
	RequestBody  *OpenAPIRequestBody         `json:"requestBody,omitempty"`
	Responses    map[string]*OpenAPIResponse `json:"responses"`
	Callbacks    map[string]*OpenAPICallback `json:"callbacks,omitempty"`
	Deprecated   *bool                       `json:"deprecated,omitempty"`
	Security     []map[string][]string       `json:"security,omitempty"`
	Servers      []*OpenAPIServer            `json:"servers,omitempty"`
}

// =============================================================================
// Schema Builder Types
// =============================================================================

// OpenAPISchemaBuilder provides methods to build OpenAPI schemas from Go types
type OpenAPISchemaBuilder struct {
	components *OpenAPIComponents
	logger     Logger
}

// SchemaConversionResult contains the result of schema conversion
type SchemaConversionResult struct {
	Schema     *OpenAPISchema            `json:"schema"`
	Components map[string]*OpenAPISchema `json:"components,omitempty"`
	Errors     []error                   `json:"errors,omitempty"`
	Warnings   []string                  `json:"warnings,omitempty"`
}

// OpenAPIEndpointSchema represents a complete OpenAPI endpoint schema
type OpenAPIEndpointSchema struct {
	Operation   *OpenAPIOperation           `json:"operation"`
	Parameters  []*OpenAPIParameter         `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody         `json:"requestBody,omitempty"`
	Responses   map[string]*OpenAPIResponse `json:"responses"`
	Security    []map[string][]string       `json:"security,omitempty"`
}

// =============================================================================
// Conversion Utilities
// =============================================================================

// NewOpenAPISchemaBuilder creates a new OpenAPI schema builder
func NewOpenAPISchemaBuilder(logger Logger) *OpenAPISchemaBuilder {
	return &OpenAPISchemaBuilder{
		components: &OpenAPIComponents{
			Schemas:         make(map[string]*OpenAPISchema),
			Responses:       make(map[string]*OpenAPIResponse),
			Parameters:      make(map[string]*OpenAPIParameter),
			Examples:        make(map[string]*OpenAPIExample),
			RequestBodies:   make(map[string]*OpenAPIRequestBody),
			Headers:         make(map[string]*OpenAPIHeader),
			SecuritySchemes: make(map[string]*OpenAPISecurityScheme),
			Links:           make(map[string]*OpenAPILink),
			Callbacks:       make(map[string]*OpenAPICallback),
		},
		logger: logger,
	}
}

// MarshalJSON implements custom JSON marshaling for OpenAPISchema to handle extensions
func (s *OpenAPISchema) MarshalJSON() ([]byte, error) {
	type Alias OpenAPISchema

	// Convert to a map to handle extensions
	schemaMap := make(map[string]interface{})

	// Marshal the main struct first
	aliasBytes, err := json.Marshal((*Alias)(s))
	if err != nil {
		return nil, err
	}

	// Unmarshal into map
	if err := json.Unmarshal(aliasBytes, &schemaMap); err != nil {
		return nil, err
	}

	// Add extensions
	for key, value := range s.Extensions {
		if !strings.HasPrefix(key, "x-") {
			key = "x-" + key
		}
		schemaMap[key] = value
	}

	return json.Marshal(schemaMap)
}

// UnmarshalJSON implements custom JSON unmarshaling for OpenAPISchema to handle extensions
func (s *OpenAPISchema) UnmarshalJSON(data []byte) error {
	type Alias OpenAPISchema

	// Unmarshal into map first
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(data, &schemaMap); err != nil {
		return err
	}

	// Extract extensions
	s.Extensions = make(map[string]interface{})
	for key, value := range schemaMap {
		if strings.HasPrefix(key, "x-") {
			s.Extensions[key] = value
			delete(schemaMap, key)
		}
	}

	// Marshal back to JSON without extensions
	cleanBytes, err := json.Marshal(schemaMap)
	if err != nil {
		return err
	}

	// Unmarshal into the alias type
	return json.Unmarshal(cleanBytes, (*Alias)(s))
}

// =============================================================================
// Standard OpenAPI Data Types
// =============================================================================

// Common OpenAPI schema patterns
var (
	// Basic types
	StringSchema  = &OpenAPISchema{Type: "string"}
	IntegerSchema = &OpenAPISchema{Type: "integer"}
	NumberSchema  = &OpenAPISchema{Type: "number"}
	BooleanSchema = &OpenAPISchema{Type: "boolean"}
	ArraySchema   = &OpenAPISchema{Type: "array"}
	ObjectSchema  = &OpenAPISchema{Type: "object"}

	// Common formats
	DateTimeSchema = &OpenAPISchema{Type: "string", Format: "date-time"}
	DateSchema     = &OpenAPISchema{Type: "string", Format: "date"}
	EmailSchema    = &OpenAPISchema{Type: "string", Format: "email"}
	UUIDSchema     = &OpenAPISchema{Type: "string", Format: "uuid"}
	URISchema      = &OpenAPISchema{Type: "string", Format: "uri"}

	// PocketBase common schemas
	PocketBaseRecordSchema = &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"id":      {Type: "string", Description: "Record ID"},
			"created": {Type: "string", Format: "date-time", Description: "Creation timestamp"},
			"updated": {Type: "string", Format: "date-time", Description: "Last update timestamp"},
		},
		Required: []string{"id", "created", "updated"},
	}

	// Error response schema
	ErrorResponseSchema = &OpenAPISchema{
		Type: "object",
		Properties: map[string]*OpenAPISchema{
			"code":    {Type: "integer", Description: "Error code"},
			"message": {Type: "string", Description: "Error message"},
			"data":    {Type: "object", Description: "Additional error data"},
		},
		Required: []string{"code", "message"},
	}
)
