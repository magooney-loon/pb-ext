# API Documentation System

Automatic API documentation generation for PocketBase applications with versioned API support using AST analysis and OpenAPI-compatible output.

## Overview

This system provides runtime discovery and documentation of versioned API routes through:
- **Versioned APIs**: Support for multiple API versions with independent configurations
- **AST Analysis**: Parse Go source files to extract handler information, types, and schemas
- **Auto-Discovery**: Automatically detect and document routes as they're registered
- **Schema Generation**: Generate JSON schemas from Go types and struct tags
- **Middleware Detection**: Analyze authentication requirements from middleware chains
- **OpenAPI Output**: Generate OpenAPI 3.0 compatible documentation per version

## OpenAPI Schema System

This system has been refactored to use **OpenAPI 3.0 compatible schemas** throughout, ensuring type safety and standards compliance:

- **Type-Safe Schemas**: All schemas use proper `*OpenAPISchema` types instead of generic maps
- **OpenAPI 3.0 Compatible**: Full compliance with OpenAPI specification
- **Component System**: Reusable schema components with proper `$ref` references
- **Validation Support**: Rich validation constraints (min/max, patterns, etc.)
- **Conversion Utilities**: Automatic conversion from Go types to OpenAPI schemas

### Key Improvements
- **Before**: `map[string]interface{}` - loose, error-prone
- **After**: `*OpenAPISchema` - type-safe, IDE-friendly, spec-compliant

```go
// Old approach
endpoint.Request = map[string]interface{}{
    "type": "object",
    "properties": map[string]interface{}{"name": map[string]interface{}{"type": "string"}},
}

// New OpenAPI approach  
endpoint.Request = &OpenAPISchema{
    Type: "object", 
    Properties: map[string]*OpenAPISchema{
        "name": {Type: "string", Description: "User name"},
    },
}
```

See **[OPENAPI_MIGRATION.md](./OPENAPI_MIGRATION.md)** for complete migration details and examples.

## Architecture

```
VersionedAPISystem
├── VersionManager       # Manages multiple API versions
├── APIRegistry          # Per-version route registration & management
├── ASTParser           # Go source code analysis
├── SchemaGenerator     # JSON schema generation
└── AutoAPIRouter       # Route wrapper with auto-docs
```

## Versioned API System

### Version Management

The system supports multiple API versions running simultaneously with independent configurations:

```go
// Create version-specific configurations
v1Config := &api.APIDocsConfig{
    Title:       "pb-ext demo api",
    Version:     "1.0.0",
    Description: "Production stable API",
    Status:      "stable",
    Enabled:     true,
    AutoDiscovery: &api.AutoDiscoveryConfig{
        Enabled: true,
    },
}

v2Config := &api.APIDocsConfig{
    Title:       "pb-ext demo api",
    Version:     "2.0.0",
    Description: "Development preview API",
    Status:      "testing",
    Enabled:     true,
    AutoDiscovery: &api.AutoDiscoveryConfig{
        Enabled: true,
    },
}

// Initialize version manager
versions := map[string]*api.APIDocsConfig{
    "v1": v1Config,
    "v2": v2Config,
}
versionManager := api.InitializeVersionedSystem(versions, "v1") // v1 is default
```

### Version-Specific Routing

Each version gets its own router with independent route registration:

```go
// Get version-specific routers
v1Router, err := versionManager.GetVersionRouter("v1", e)
v2Router, err := versionManager.GetVersionRouter("v2", e)

// Register routes per version
v1Router.GET("/api/v1/todos", getTodosHandler)
v1Router.POST("/api/v1/todos", createTodoHandler).Bind(apis.RequireAuth())
v1Router.PATCH("/api/v1/todos/{id}", updateTodoHandler).Bind(apis.RequireAuth())

v2Router.GET("/api/v2/time", timeHandler)
v2Router.GET("/api/v2/analytics", analyticsHandler).Bind(apis.RequireAuth())
```

## Usage Pattern

### Complete Implementation

```go
func registerRoutes(pbApp core.App) {
    // Create configs for API versions
    v1Config := &api.APIDocsConfig{
        Title:       "pb-ext demo api",
        Version:     "1.0.0",
        Description: "Stable production API",
        Status:      "stable",
        Enabled:     true,
        AutoDiscovery: &api.AutoDiscoveryConfig{
            Enabled: true,
        },
    }

    v2Config := &api.APIDocsConfig{
        Title:       "pb-ext demo api", 
        Version:     "2.0.0",
        Description: "Development API with new features",
        Status:      "testing",
        Enabled:     true,
        AutoDiscovery: &api.AutoDiscoveryConfig{
            Enabled: true,
        },
    }

    // Initialize version manager
    versions := map[string]*api.APIDocsConfig{
        "v1": v1Config,
        "v2": v2Config,
    }
    versionManager := api.InitializeVersionedSystem(versions, "v1")

    pbApp.OnServe().BindFunc(func(e *core.ServeEvent) error {
        // Get version-specific routers
        v1Router, _ := versionManager.GetVersionRouter("v1", e)
        v2Router, _ := versionManager.GetVersionRouter("v2", e)

        // v1 API routes
        v1Router.GET("/api/v1/todos", getTodosHandler)
        v1Router.POST("/api/v1/todos", createTodoHandler).Bind(apis.RequireAuth())
        v1Router.GET("/api/v1/todos/{id}", getTodoHandler)
        v1Router.PATCH("/api/v1/todos/{id}", updateTodoHandler).Bind(apis.RequireAuth())
        v1Router.DELETE("/api/v1/todos/{id}", deleteTodoHandler).Bind(apis.RequireAuth())

        // v2 API routes with new features
        v2Router.GET("/api/v2/time", timeHandler)

        return e.Next()
    })

    // Register version management endpoints
    versionManager.RegisterWithServer(pbApp)
}
```

### AST Directives

Mark your Go files for automatic analysis:

```go
// API_SOURCE - Mark file for AST analysis
// API_DESC Get todos list
// API_TAGS todos,list
func getTodosHandler(c *core.RequestEvent) error { }

// API_DESC Create a new todo
// API_TAGS todos,create
func createTodoHandler(c *core.RequestEvent) error { }
```

### Authentication Middleware Detection

The system automatically detects authentication requirements:

```go
// Different auth types are automatically documented
v1Router.GET("/api/v1/public", handler)                                    // No auth
v1Router.GET("/api/v1/guest", handler).Bind(apis.RequireGuestOnly())       // Guest only
v1Router.POST("/api/v1/todos", handler).Bind(apis.RequireAuth())            // Auth required
v1Router.DELETE("/api/v1/admin", handler).Bind(apis.RequireSuperuserAuth()) // Admin only
v1Router.PUT("/api/v1/todos/{id}", handler).Bind(apis.RequireSuperuserOrOwnerAuth("id")) // Owner/Admin
```

## Documentation Endpoints

### Version-Specific Documentation
- **Version OpenAPI**: `GET /api/docs/v1` (for v1)
- **Version OpenAPI**: `GET /api/docs/v2` (for v2)
- **Schema Config**: `GET /api/v1/schema/config`
- **AST Debug**: `GET /api/docs/debug/ast`

### Version Management
- **List Versions**: `GET /api/docs/versions`

## Features

- ✅ **Multi-Version Support**: Run multiple API versions simultaneously
- ✅ **Independent Configs**: Each version has its own configuration and status
- ✅ **AST Analysis**: Deep code analysis for accurate documentation
- ✅ **Auto-Discovery**: Automatically detect routes and middleware per version
- ✅ **Schema Generation**: Generate schemas from Go types using AST
- ✅ **Auth Detection**: Analyze middleware for authentication requirements
- ✅ **OpenAPI Compatible**: Standard OpenAPI 3.0 output format per version
- ✅ **Thread Safe**: Concurrent access support with proper locking
- ✅ **Status Management**: Mark versions as stable, testing, deprecated
- ✅ **Selective Enabling**: Enable/disable versions independently
- ✅ **Clean Architecture**: No legacy code, versioned system only

## Version Lifecycle Management

Versions can have different statuses:
- **stable**: Production-ready, fully enabled
- **testing**: Preview/beta, may have limited availability
- **deprecated**: Legacy version, marked for removal
- **maintenance**: Bug fixes only, no new features

Each version can be independently enabled/disabled through configuration.
