# OpenAPI Schema Migration Guide

## Overview

This document outlines the major refactoring of the pb-ext API schema system to ensure full **OpenAPI 3.0 compatibility** and **consistent schema generation** across the entire codebase.

## What Changed

### Before: Generic Maps
Previously, schemas were represented as loose `map[string]interface{}` structures:

```go
// Old approach - inconsistent and not OpenAPI compatible
type APIEndpoint struct {
    Request  map[string]interface{} `json:"request,omitempty"`
    Response map[string]interface{} `json:"response,omitempty"`
}

type StructInfo struct {
    JSONSchema map[string]interface{} `json:"json_schema"`
}
```

### After: Strict OpenAPI Types
Now all schemas use proper OpenAPI 3.0 compatible structures:

```go
// New approach - type-safe and OpenAPI 3.0 compatible
type APIEndpoint struct {
    Request  *OpenAPISchema `json:"request,omitempty"`
    Response *OpenAPISchema `json:"response,omitempty"`
}

type StructInfo struct {
    JSONSchema *OpenAPISchema `json:"json_schema"`
}
```

## Files Modified

### New Files Created
1. **`openapi_schema_types.go`** - Complete OpenAPI 3.0 type definitions
2. **`schema_conversion.go`** - Utilities for converting Go types to OpenAPI schemas

### Files Updated
1. **`schema_types.go`** - Updated to use OpenAPI types
2. **`ast_types.go`** - All schema fields now use `*OpenAPISchema`
3. **`api_types.go`** - APIEndpoint and APIDocs use OpenAPI components
4. **`schema.go`** - Complete rewrite to generate OpenAPI-compatible schemas
5. **`ast.go`** - Updated all schema generation methods
6. **`registry.go`** - Updated to handle OpenAPIComponents structure

## Key OpenAPI Types Introduced

### Core Schema Type
```go
type OpenAPISchema struct {
    Type        string                     `json:"type,omitempty"`
    Format      string                     `json:"format,omitempty"`
    Title       string                     `json:"title,omitempty"`
    Description string                     `json:"description,omitempty"`
    Default     interface{}                `json:"default,omitempty"`
    Example     interface{}                `json:"example,omitempty"`
    
    // Validation
    Maximum     *float64  `json:"maximum,omitempty"`
    Minimum     *float64  `json:"minimum,omitempty"`
    MaxLength   *int      `json:"maxLength,omitempty"`
    MinLength   *int      `json:"minLength,omitempty"`
    Pattern     string    `json:"pattern,omitempty"`
    Required    []string  `json:"required,omitempty"`
    Enum        []interface{} `json:"enum,omitempty"`
    
    // Object properties
    Properties           map[string]*OpenAPISchema `json:"properties,omitempty"`
    AdditionalProperties interface{}               `json:"additionalProperties,omitempty"`
    
    // Array items
    Items *OpenAPISchema `json:"items,omitempty"`
    
    // References and composition
    Ref   string             `json:"$ref,omitempty"`
    AllOf []*OpenAPISchema   `json:"allOf,omitempty"`
    OneOf []*OpenAPISchema   `json:"oneOf,omitempty"`
    AnyOf []*OpenAPISchema   `json:"anyOf,omitempty"`
}
```

### Component System
```go
type OpenAPIComponents struct {
    Schemas         map[string]*OpenAPISchema         `json:"schemas,omitempty"`
    Responses       map[string]*OpenAPIResponse       `json:"responses,omitempty"`
    Parameters      map[string]*OpenAPIParameter      `json:"parameters,omitempty"`
    RequestBodies   map[string]*OpenAPIRequestBody    `json:"requestBodies,omitempty"`
    SecuritySchemes map[string]*OpenAPISecurityScheme `json:"securitySchemes,omitempty"`
    // ... other components
}
```

## Interface Changes

### Schema Generation Interface
```go
// Updated interface with OpenAPI return types
type SchemaGeneratorInterface interface {
    AnalyzeRequestSchema(endpoint *APIEndpoint) (*OpenAPISchema, error)
    AnalyzeResponseSchema(endpoint *APIEndpoint) (*OpenAPISchema, error)
    AnalyzeSchemaFromPath(method, path string) (*SchemaAnalysisResult, error)
    GenerateComponentSchemas() *OpenAPIComponents
    GetOpenAPIEndpointSchema(endpoint *APIEndpoint) (*OpenAPIEndpointSchema, error)
}
```

### New Schema Analysis Result
```go
type SchemaAnalysisResult struct {
    RequestSchema  *OpenAPISchema      `json:"request_schema,omitempty"`
    ResponseSchema *OpenAPISchema      `json:"response_schema,omitempty"`
    Parameters     []*OpenAPIParameter `json:"parameters,omitempty"`
    Errors         []error             `json:"errors,omitempty"`
    Warnings       []string            `json:"warnings,omitempty"`
}
```

## Conversion Utilities

### Go Type to OpenAPI Schema
```go
// Convert Go types to OpenAPI schemas
schema := ConvertGoTypeToOpenAPISchema("[]string", fieldInfo)
// Returns: &OpenAPISchema{Type: "array", Items: &OpenAPISchema{Type: "string"}}

// Convert struct info to OpenAPI object schema
objectSchema := ConvertStructInfoToOpenAPISchema(structInfo)

// Convert parameters
param := ConvertParamInfoToOpenAPIParameter(paramInfo)
```

### Standard Schemas Available
```go
// Pre-defined common schemas
var (
    StringSchema    = &OpenAPISchema{Type: "string"}
    IntegerSchema   = &OpenAPISchema{Type: "integer"}
    BooleanSchema   = &OpenAPISchema{Type: "boolean"}
    DateTimeSchema  = &OpenAPISchema{Type: "string", Format: "date-time"}
    EmailSchema     = &OpenAPISchema{Type: "string", Format: "email"}
    UUIDSchema      = &OpenAPISchema{Type: "string", Format: "uuid"}
    
    // PocketBase specific
    PocketBaseRecordSchema = &OpenAPISchema{ /* complete record schema */ }
    ErrorResponseSchema    = &OpenAPISchema{ /* error response schema */ }
)
```

## Migration Guide for Existing Code

### 1. Schema Generation
**Before:**
```go
// Old generic map approach
func generateSchema() map[string]interface{} {
    return map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "name": map[string]interface{}{"type": "string"},
        },
    }
}
```

**After:**
```go
// New OpenAPI schema approach
func generateSchema() *OpenAPISchema {
    return &OpenAPISchema{
        Type: "object",
        Properties: map[string]*OpenAPISchema{
            "name": {Type: "string"},
        },
    }
}
```

### 2. Accessing Schema Properties
**Before:**
```go
// Unsafe type assertions
if props, ok := schema["properties"].(map[string]interface{}); ok {
    if nameSchema, ok := props["name"].(map[string]interface{}); ok {
        schemaType := nameSchema["type"].(string)
    }
}
```

**After:**
```go
// Type-safe property access
if nameSchema := schema.Properties["name"]; nameSchema != nil {
    schemaType := nameSchema.Type
}
```

### 3. Creating Complete Endpoint Schemas
**Before:**
```go
// Manual map construction
endpoint.Request = map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{ /* ... */ },
}
```

**After:**
```go
// Use schema builder or conversion utilities
endpoint.Request = ConvertGoTypeToOpenAPISchema("MyRequestStruct", nil)
// Or get complete endpoint schema
endpointSchema, err := schemaGenerator.GetOpenAPIEndpointSchema(endpoint)
```

## Benefits of the Migration

### 1. **Type Safety**
- No more unsafe type assertions
- Compile-time checks for schema structure
- IDE autocompletion for schema properties

### 2. **OpenAPI 3.0 Compliance**
- Full compatibility with OpenAPI spec
- Proper validation constraints support
- Standard schema references (`$ref`)
- Complete component system

### 3. **Consistency**
- Unified schema representation across all components
- Consistent JSON serialization
- Standardized error handling

### 4. **Extensibility**
- Support for OpenAPI extensions (`x-*` properties)
- Proper inheritance with `allOf`, `oneOf`, `anyOf`
- Component reusability

### 5. **Better Documentation**
- Rich schema metadata (title, description, examples)
- Validation constraints clearly defined
- Deprecation support

## Common Patterns

### Creating Object Schemas
```go
schema := &OpenAPISchema{
    Type: "object",
    Properties: map[string]*OpenAPISchema{
        "id": {
            Type: "string",
            Description: "Unique identifier",
            Example: "abc123",
        },
        "name": {
            Type: "string",
            Description: "Display name",
            MinLength: intPtr(1),
            MaxLength: intPtr(100),
        },
        "email": {
            Type: "string",
            Format: "email",
            Description: "Email address",
        },
    },
    Required: []string{"id", "name"},
}
```

### Creating Array Schemas
```go
schema := &OpenAPISchema{
    Type: "array",
    Items: &OpenAPISchema{
        Type: "object",
        Ref: "#/components/schemas/UserRecord",
    },
    Description: "List of users",
}
```

### Using References
```go
schema := &OpenAPISchema{
    AllOf: []*OpenAPISchema{
        {Ref: "#/components/schemas/BaseRecord"},
        {
            Type: "object",
            Properties: map[string]*OpenAPISchema{
                "customField": {Type: "string"},
            },
        },
    },
}
```

## Backwards Compatibility

- All existing API endpoints continue to work
- JSON serialization format remains the same
- Only internal representation has changed
- Type conversion utilities handle legacy formats

## Testing the Migration

1. **Compile Check**: `go build ./...`
2. **Schema Validation**: All schemas now validate against OpenAPI 3.0 spec
3. **JSON Output**: Generated JSON should be valid OpenAPI 3.0
4. **Component References**: `$ref` links should resolve correctly

## Future Improvements

With this foundation, we can now add:

1. **OpenAPI 3.0 Document Generation**: Complete spec files
2. **Schema Validation**: Runtime validation against schemas  
3. **Code Generation**: Generate client SDKs from schemas
4. **Interactive Documentation**: Swagger UI integration
5. **Schema Testing**: Automated schema compliance testing

---

This migration provides a solid, type-safe foundation for OpenAPI-compatible API documentation and schema management in pb-ext.