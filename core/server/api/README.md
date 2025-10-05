# API Documentation System

Automatic API documentation generation for PocketBase applications using AST analysis and OpenAPI-compatible output.

## Overview

This system provides runtime discovery and documentation of API routes through:
- **AST Analysis**: Parse Go source files to extract handler information, types, and schemas
- **Auto-Discovery**: Automatically detect and document routes as they're registered
- **Schema Generation**: Generate JSON schemas from Go types and struct tags
- **Middleware Detection**: Analyze authentication requirements from middleware chains
- **OpenAPI Output**: Generate OpenAPI 3.0 compatible documentation

## Architecture

```
APIDocumentationSystem
├── APIRegistry          # Route registration & management
├── ASTParser           # Go source code analysis  
├── SchemaGenerator     # JSON schema generation
└── AutoAPIRouter       # Route wrapper with auto-docs
```

## Core Types

### API Documentation
```go
type APIDocs struct {
    Title       string            `json:"title"`
    Version     string            `json:"version"`
    Description string            `json:"description"`
    BaseURL     string            `json:"base_url"`
    Endpoints   []APIEndpoint     `json:"endpoints"`
    Generated   string            `json:"generated_at"`
    Components  map[string]interface{} `json:"components"`
}

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

type AuthInfo struct {
    Required    bool     `json:"required"`
    Type        string   `json:"type"` // "guest_only", "auth", "superuser", "superuser_or_owner"
    Collections []string `json:"collections,omitempty"`
    OwnerParam  string   `json:"owner_param,omitempty"`
    Description string   `json:"description"`
    Icon        string   `json:"icon"`
}
```

### Configuration
```go
type APIDocsConfig struct {
    Title         string               `json:"title"`
    Version       string               `json:"version"`
    Description   string               `json:"description"`
    BaseURL       string               `json:"base_url"`
    Enabled       bool                 `json:"enabled"`
    AutoDiscovery *AutoDiscoveryConfig `json:"auto_discovery"`
}

type AutoDiscoveryConfig struct {
    Enabled         bool `json:"enabled"`
    AnalyzeHandlers bool `json:"analyze_handlers"`
    GenerateTags    bool `json:"generate_tags"`
    DetectAuth      bool `json:"detect_auth"`
    IncludeInternal bool `json:"include_internal"`
}
```

### AST Analysis Types
```go
type StructInfo struct {
    Name           string                 `json:"name"`
    Package        string                 `json:"package"`
    Fields         map[string]*FieldInfo  `json:"fields"`
    JSONSchema     map[string]interface{} `json:"json_schema"`
    Description    string                 `json:"description"`
    Tags           []string               `json:"tags"`
    Documentation  *Documentation         `json:"documentation,omitempty"`
    SourceLocation *SourceLocation        `json:"source_location,omitempty"`
}

type ASTHandlerInfo struct {
    Name           string                 `json:"name"`
    Package        string                 `json:"package"`
    RequestType    string                 `json:"request_type"`
    ResponseType   string                 `json:"response_type"`
    ResponseSchema map[string]interface{} `json:"response_schema,omitempty"`
    Parameters     []*ParamInfo           `json:"parameters,omitempty"`
    APIDescription string                 `json:"api_description"`
    APITags        []string               `json:"api_tags"`
    HTTPMethods    []string               `json:"http_methods"`
    RoutePath      string                 `json:"route_path,omitempty"`
    Middleware     []string               `json:"middleware,omitempty"`
    Documentation  *Documentation         `json:"documentation,omitempty"`
    SourceLocation *SourceLocation        `json:"source_location,omitempty"`
}

type FieldInfo struct {
    Name           string                 `json:"name"`
    Type           string                 `json:"type"`
    JSONName       string                 `json:"json_name"`
    JSONOmitEmpty  bool                   `json:"json_omit_empty"`
    Required       bool                   `json:"required"`
    Validation     map[string]string      `json:"validation"`
    Description    string                 `json:"description"`
    Schema         map[string]interface{} `json:"schema"`
    IsPointer      bool                   `json:"is_pointer"`
    IsEmbedded     bool                   `json:"is_embedded"`
    DefaultValue   interface{}            `json:"default_value,omitempty"`
    Constraints    map[string]interface{} `json:"constraints,omitempty"`
}
```

### Core Interfaces
```go
type ASTParserInterface interface {
    ParseFile(filename string) error
    GetAllStructs() map[string]*StructInfo
    GetAllHandlers() map[string]*ASTHandlerInfo
    GetStructByName(name string) (*StructInfo, bool)
    GetHandlerByName(name string) (*ASTHandlerInfo, bool)
    EnhanceEndpoint(endpoint *APIEndpoint) error
    GetHandlerDescription(handlerName string) string
    GetHandlerTags(handlerName string) []string
}

type SchemaGeneratorInterface interface {
    AnalyzeRequestSchema(endpoint *APIEndpoint) (map[string]interface{}, error)
    AnalyzeResponseSchema(endpoint *APIEndpoint) (map[string]interface{}, error)
    AnalyzeSchemaFromPath(method, path string) (*SchemaAnalysisResult, error)
    GenerateComponentSchemas() map[string]interface{}
}
```

## Main Components

### APIRegistry
- **Purpose**: Central registry for API endpoint documentation
- **Features**: Thread-safe endpoint storage, auto-discovery integration, component schema generation
- **Key Methods**: `RegisterEndpoint()`, `AutoRegisterRoute()`, `GetDocs()`, `GetDocsWithComponents()`

### ASTParser  
- **Purpose**: Parse Go source files to extract API-related information
- **Features**: Struct analysis, handler detection, comment extraction, type inference
- **Discovery**: Automatically finds files marked with `// API_SOURCE` directive

### SchemaGenerator
- **Purpose**: Generate JSON schemas from Go types and handler analysis
- **Features**: AST-based schema generation, path-based fallbacks, validation rule extraction
- **Output**: OpenAPI 3.0 compatible schemas

### AutoAPIRouter
- **Purpose**: Router wrapper that automatically documents registered routes  
- **Features**: Method chaining, middleware detection, authentication analysis
- **Integration**: Transparent wrapper around PocketBase's router

## Usage Patterns

### Basic Integration
```go
// Enable auto-documentation
router := EnableAutoDocumentation(e)
router.GET("/api/users", getUsersHandler)
```

### AST Directives
```go
// API_SOURCE - Mark file for AST analysis
// API_DESC Get user information  
// API_TAGS users,profile
func getUsersHandler(c *core.RequestEvent) error { }
```

### Access Documentation
- **OpenAPI JSON**: `GET /api/docs/openapi`
- **Statistics**: `GET /api/docs/stats`  
- **Components**: `GET /api/docs/components`

## Features

- ✅ **Zero Configuration**: Works out of the box with sensible defaults
- ✅ **AST Analysis**: Deep code analysis for accurate documentation  
- ✅ **Auto-Discovery**: Automatically detect routes and middleware
- ✅ **Schema Generation**: Generate schemas from Go types
- ✅ **Auth Detection**: Analyze middleware for authentication requirements
- ✅ **OpenAPI Compatible**: Standard OpenAPI 3.0 output format
- ✅ **Thread Safe**: Concurrent access support with proper locking
- ✅ **Extensible**: Plugin system for validators and custom analyzers