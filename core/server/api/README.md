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
    Status:      "development",
    Enabled:     false,
    AutoDiscovery: &api.AutoDiscoveryConfig{
        Enabled: false,
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
v1Router.GET("/api/v1/time", timeHandler)
v1Router.POST("/api/v1/posts", createPostHandler).Bind(apis.RequireAuth())

v2Router.GET("/api/v2/time", timeHandler)
v2Router.POST("/api/v2/posts", createPostHandler).Bind(apis.RequireAuth())
```

## Usage Patterns

### Basic Versioned Setup

```go
func registerRoutes(pbApp core.App) {
    // Initialize versioned system
    versions := map[string]*api.APIDocsConfig{
        "v1": v1Config,
        "v2": v2Config,
    }
    versionManager := api.InitializeVersionedSystem(versions, "v1")

    pbApp.OnServe().BindFunc(func(e *core.ServeEvent) error {
        // Get version routers and register routes
        v1Router, _ := versionManager.GetVersionRouter("v1", e)
        v2Router, _ := versionManager.GetVersionRouter("v2", e)

        // Register version-specific routes
        v1Router.GET("/api/v1/endpoint", handler)
        v2Router.GET("/api/v2/endpoint", handler)

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
// API_DESC Get user information
// API_TAGS users,profile
func getUsersHandler(c *core.RequestEvent) error { }
```

### Authentication Middleware Detection

The system automatically detects authentication requirements:

```go
// Different auth types are automatically documented
router.GET("/public", handler)                                    // No auth
router.GET("/guest", handler).Bind(apis.RequireGuestOnly())       // Guest only
router.POST("/auth", handler).Bind(apis.RequireAuth())            // Auth required
router.DELETE("/admin", handler).Bind(apis.RequireSuperuserAuth()) // Admin only
router.PUT("/owner", handler).Bind(apis.RequireSuperuserOrOwnerAuth("id")) // Owner/Admin
```

## Documentation Endpoints

### Version-Specific Documentation
- **OpenAPI JSON**: `GET /api/v1/docs/openapi` (for v1)
- **OpenAPI JSON**: `GET /api/v2/docs/openapi` (for v2)
- **Statistics**: `GET /api/v1/docs/stats`
- **Components**: `GET /api/v1/docs/components`

### Version Management
- **List Versions**: `GET /api/versions`
- **Version Info**: `GET /api/versions/{version}`
- **Default Version**: `GET /api/versions/default`

## Features

- ✅ **Multi-Version Support**: Run multiple API versions simultaneously
- ✅ **Independent Configs**: Each version has its own configuration and status
- ✅ **Zero Configuration**: Works out of the box with sensible defaults
- ✅ **AST Analysis**: Deep code analysis for accurate documentation
- ✅ **Auto-Discovery**: Automatically detect routes and middleware per version
- ✅ **Schema Generation**: Generate schemas from Go types
- ✅ **Auth Detection**: Analyze middleware for authentication requirements
- ✅ **OpenAPI Compatible**: Standard OpenAPI 3.0 output format per version
- ✅ **Thread Safe**: Concurrent access support with proper locking
- ✅ **Status Management**: Mark versions as stable, development, deprecated
- ✅ **Selective Enabling**: Enable/disable versions independently

## Version Lifecycle Management

Versions can have different statuses:
- **stable**: Production-ready, fully enabled
- **development**: Preview/beta, may have limited availability
- **deprecated**: Legacy version, marked for removal
- **maintenance**: Bug fixes only, no new features

Each version can be independently enabled/disabled.
