# PocketBase API Documentation System

A comprehensive **versioned API documentation system** for PocketBase applications that provides automatic route discovery, OpenAPI-compatible documentation generation, and AST-based code analysis for Go handlers.

## What This Package Does

This API documentation system transforms your PocketBase application into a **self-documenting API** with the following capabilities:

### 🔄 **Multi-Version API Management**
- Run multiple API versions simultaneously (v1, v2, v3, etc.)
- Independent configuration, routing, and documentation per version
- Version lifecycle management (stable, testing, deprecated, maintenance)
- Seamless version switching and backward compatibility

### 🤖 **Automatic Documentation Generation**
- **AST Analysis**: Deep parsing of Go source code to extract handler information
- **Route Auto-Discovery**: Automatically detect and document API endpoints as they're registered
- **Schema Generation**: Convert Go types to OpenAPI 3.0 schemas using struct tags
- **Middleware Detection**: Analyze authentication requirements from PocketBase middleware chains
- **OpenAPI Output**: Generate standards-compliant OpenAPI 3.0 documentation

### 🔍 **Intelligent Code Analysis**
- Parse handler functions to understand request/response types
- Extract API descriptions and tags from special comments (`// API_DESC`, `// API_TAGS`)
- Detect authentication middleware (`RequireAuth()`, `RequireSuperuserAuth()`, etc.)
- Analyze database operations and PocketBase-specific patterns
- Generate type-safe schemas from Go structs

### 📊 **Real-Time Documentation**
- Live documentation updates as routes are registered
- RESTful endpoints for accessing documentation (`/api/docs/v1`, `/api/docs/v2`)
- Debug endpoints for inspecting AST analysis results
- Version comparison and management interfaces

## Architecture Overview

```
PocketBase App
├── VersionedAPISystem
│   ├── VersionManager           # Manages multiple API versions
│   ├── APIRegistry (per version) # Route registration & documentation per version
│   ├── ASTParser               # Go source code analysis
│   ├── SchemaGenerator         # OpenAPI schema generation
│   └── AutoAPIRouter           # Self-documenting route wrapper
└── Generated Documentation
    ├── /api/docs/v1            # Version 1 OpenAPI spec
    ├── /api/docs/v2            # Version 2 OpenAPI spec
    └── /api/docs/versions      # Version management
```

## Complete Setup Guide

### Step 1: Mark Your Go Files for Analysis

Add the `API_SOURCE` directive to Go files containing API handlers:

```go
// API_SOURCE - Mark this file for AST analysis

package main

import (
    "github.com/pocketbase/pocketbase/core"
    "github.com/pocketbase/pocketbase/apis"
)

// API_DESC Get all todos for the authenticated user
// API_TAGS todos,list,get
func getTodosHandler(c *core.RequestEvent) error {
    // Your handler implementation
    return c.JSON(200, map[string]interface{}{
        "todos": []string{"Learn Go", "Build API", "Write docs"},
    })
}

// API_DESC Create a new todo item
// API_TAGS todos,create,post
func createTodoHandler(c *core.RequestEvent) error {
    type CreateTodoRequest struct {
        Title       string `json:"title" validate:"required"`
        Description string `json:"description,omitempty"`
        Priority    string `json:"priority" validate:"oneof=low medium high"`
    }
    
    var req CreateTodoRequest
    if err := c.BindBody(&req); err != nil {
        return err
    }
    
    // Your creation logic here
    return c.JSON(201, map[string]interface{}{
        "id": "todo_123",
        "title": req.Title,
        "created": "2024-01-01T00:00:00Z",
    })
}

// API_DESC Update an existing todo
// API_TAGS todos,update,patch
func updateTodoHandler(c *core.RequestEvent) error {
    // Handler with path parameter
    todoID := c.PathParam("id")
    // Update logic...
    return c.JSON(200, map[string]interface{}{"updated": todoID})
}
```

### Step 2: Configure API Versions

Create version-specific configurations for your APIs:

```go
// API_SOURCE
package main

import (
    "github.com/pocketbase/pocketbase/core"
    "github.com/your-org/pb-ext/core/server/api"
)

func setupAPIVersions() *api.APIVersionManager {
    // Version 1 - Stable Production API
    v1Config := &api.APIDocsConfig{
        Title:       "MyApp REST API",
        Version:     "1.0.0",
        Description: "Stable production API for MyApp with full todo management",
        Status:      "stable",
        BaseURL:     "https://api.myapp.com",
        Enabled:     true,
        AutoDiscovery: &api.AutoDiscoveryConfig{
            Enabled:         true,
            AnalyzeHandlers: true,
            GenerateTags:    true,
            DetectAuth:      true,
            IncludeInternal: false,
        },
    }

    // Version 2 - Development API with new features
    v2Config := &api.APIDocsConfig{
        Title:       "MyApp REST API",
        Version:     "2.0.0-beta",
        Description: "Next-generation API with enhanced features and improved performance",
        Status:      "testing",
        BaseURL:     "https://api-v2.myapp.com",
        Enabled:     true,
        AutoDiscovery: &api.AutoDiscoveryConfig{
            Enabled:         true,
            AnalyzeHandlers: true,
            GenerateTags:    true,
            DetectAuth:      true,
            IncludeInternal: true, // Include internal endpoints for testing
        },
    }

    // Version 3 - Experimental API
    v3Config := &api.APIDocsConfig{
        Title:       "MyApp REST API",
        Version:     "3.0.0-alpha",
        Description: "Experimental API with cutting-edge features",
        Status:      "development",
        BaseURL:     "https://api-experimental.myapp.com",
        Enabled:     false, // Disabled by default
        AutoDiscovery: &api.AutoDiscoveryConfig{
            Enabled:         true,
            AnalyzeHandlers: true,
            GenerateTags:    true,
            DetectAuth:      true,
            IncludeInternal: true,
        },
    }

    // Initialize version manager
    versions := map[string]*api.APIDocsConfig{
        "v1": v1Config,
        "v2": v2Config,
        "v3": v3Config,
    }

    return api.InitializeVersionedSystem(versions, "v1") // v1 is default
}
```

### Step 3: Register Routes with Version-Specific Routers

```go
// API_SOURCE
package main

import (
    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/core"
    "github.com/pocketbase/pocketbase/apis"
    "github.com/your-org/pb-ext/core/server/api"
)

func main() {
    app := pocketbase.New()

    // Setup versioned API system
    versionManager := setupAPIVersions()

    // Register routes when server starts
    app.OnServe().BindFunc(func(e *core.ServeEvent) error {
        // Get version-specific routers
        v1Router, err := versionManager.GetVersionRouter("v1", e)
        if err != nil {
            return err
        }

        v2Router, err := versionManager.GetVersionRouter("v2", e)
        if err != nil {
            return err
        }

        // ================================
        // V1 API Routes - Stable
        // ================================
        
        // Public endpoints (no authentication)
        v1Router.GET("/api/v1/health", healthCheckHandler)
        v1Router.GET("/api/v1/version", versionHandler)

        // Guest-only endpoints
        v1Router.POST("/api/v1/auth/login", loginHandler).Bind(apis.RequireGuestOnly())
        v1Router.POST("/api/v1/auth/register", registerHandler).Bind(apis.RequireGuestOnly())

        // User authentication required
        v1Router.GET("/api/v1/todos", getTodosHandler).Bind(apis.RequireAuth())
        v1Router.POST("/api/v1/todos", createTodoHandler).Bind(apis.RequireAuth())
        v1Router.GET("/api/v1/todos/{id}", getTodoHandler).Bind(apis.RequireAuth())
        v1Router.PATCH("/api/v1/todos/{id}", updateTodoHandler).Bind(apis.RequireAuth())
        v1Router.DELETE("/api/v1/todos/{id}", deleteTodoHandler).Bind(apis.RequireAuth())

        // User profile management
        v1Router.GET("/api/v1/profile", getProfileHandler).Bind(apis.RequireAuth())
        v1Router.PUT("/api/v1/profile", updateProfileHandler).Bind(apis.RequireAuth())

        // Admin-only endpoints
        v1Router.GET("/api/v1/admin/users", listUsersHandler).Bind(apis.RequireSuperuserAuth())
        v1Router.DELETE("/api/v1/admin/users/{id}", deleteUserHandler).Bind(apis.RequireSuperuserAuth())

        // Owner or admin access
        v1Router.GET("/api/v1/users/{id}/todos", getUserTodosHandler).Bind(
            apis.RequireSuperuserOrOwnerAuth("id"),
        )

        // ================================
        // V2 API Routes - Testing
        // ================================
        
        // Enhanced endpoints with new features
        v2Router.GET("/api/v2/todos", getTodosV2Handler).Bind(apis.RequireAuth())
        v2Router.POST("/api/v2/todos/batch", batchCreateTodosHandler).Bind(apis.RequireAuth())
        v2Router.GET("/api/v2/analytics/dashboard", analyticsHandler).Bind(apis.RequireAuth())
        v2Router.GET("/api/v2/time", timeHandler) // Public utility endpoint

        // New collaboration features
        v2Router.POST("/api/v2/todos/{id}/share", shareTodoHandler).Bind(apis.RequireAuth())
        v2Router.GET("/api/v2/todos/shared", getSharedTodosHandler).Bind(apis.RequireAuth())

        return e.Next()
    })

    // Register version management endpoints
    versionManager.RegisterWithServer(app)

    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### Step 4: Define Request/Response Types

Create well-structured types that the system can analyze:

```go
// API_SOURCE
package main

// Request types for API endpoints
type CreateTodoRequest struct {
    Title       string   `json:"title" validate:"required" description:"Todo item title"`
    Description string   `json:"description,omitempty" description:"Optional description"`
    Priority    string   `json:"priority" validate:"oneof=low medium high" description:"Priority level"`
    Tags        []string `json:"tags,omitempty" description:"Associated tags"`
    DueDate     string   `json:"due_date,omitempty" description:"Due date in ISO 8601 format"`
}

type UpdateTodoRequest struct {
    Title       *string   `json:"title,omitempty" description:"Updated title"`
    Description *string   `json:"description,omitempty" description:"Updated description"`
    Priority    *string   `json:"priority,omitempty" validate:"omitempty,oneof=low medium high"`
    Completed   *bool     `json:"completed,omitempty" description:"Completion status"`
    Tags        *[]string `json:"tags,omitempty" description:"Updated tags"`
}

type BatchCreateRequest struct {
    Todos []CreateTodoRequest `json:"todos" validate:"required,min=1,max=100" description:"Array of todos to create"`
}

// Response types
type TodoResponse struct {
    ID          string   `json:"id" description:"Unique todo identifier"`
    Title       string   `json:"title" description:"Todo title"`
    Description string   `json:"description,omitempty" description:"Todo description"`
    Priority    string   `json:"priority" description:"Priority level (low/medium/high)"`
    Completed   bool     `json:"completed" description:"Completion status"`
    Tags        []string `json:"tags,omitempty" description:"Associated tags"`
    CreatedAt   string   `json:"created_at" description:"Creation timestamp"`
    UpdatedAt   string   `json:"updated_at" description:"Last update timestamp"`
    DueDate     *string  `json:"due_date,omitempty" description:"Due date if set"`
}

type TodoListResponse struct {
    Todos      []TodoResponse `json:"todos" description:"Array of todo items"`
    TotalCount int            `json:"total_count" description:"Total number of todos"`
    Page       int            `json:"page" description:"Current page number"`
    PerPage    int            `json:"per_page" description:"Items per page"`
}

type AnalyticsResponse struct {
    TotalTodos     int            `json:"total_todos" description:"Total number of todos"`
    CompletedTodos int            `json:"completed_todos" description:"Number of completed todos"`
    PriorityBreakdown map[string]int `json:"priority_breakdown" description:"Todos by priority"`
    RecentActivity []ActivityItem `json:"recent_activity" description:"Recent todo activities"`
}

type ActivityItem struct {
    Action    string `json:"action" description:"Action type (created/updated/completed)"`
    TodoID    string `json:"todo_id" description:"Related todo ID"`
    Timestamp string `json:"timestamp" description:"When the action occurred"`
}
```

### Step 5: Access Your Documentation

Once set up, your API documentation will be available at these endpoints:

#### Version-Specific Documentation
- **V1 OpenAPI Spec**: `GET /api/docs/v1` - Complete OpenAPI 3.0 specification for version 1
- **V2 OpenAPI Spec**: `GET /api/docs/v2` - Complete OpenAPI 3.0 specification for version 2
- **Schema Config**: `GET /api/v1/schema/config` - Configuration details for version 1

#### Version Management
- **List All Versions**: `GET /api/docs/versions` - Overview of all API versions and their status
- **AST Debug Info**: `GET /api/docs/debug/ast` - Debug information about AST analysis

#### Example Response from `/api/docs/versions`:

```json
{
  "versions": [
    {
      "version": "v1",
      "status": "stable",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z",
      "config": {
        "title": "MyApp REST API",
        "version": "1.0.0",
        "description": "Stable production API",
        "enabled": true
      },
      "stats": {
        "endpoint_count": 15,
        "authenticated_endpoints": 12,
        "public_endpoints": 3
      }
    },
    {
      "version": "v2",
      "status": "testing",
      "created_at": "2024-01-02T00:00:00Z",
      "updated_at": "2024-01-02T00:00:00Z",
      "config": {
        "title": "MyApp REST API",
        "version": "2.0.0-beta",
        "description": "Next-generation API",
        "enabled": true
      },
      "stats": {
        "endpoint_count": 8,
        "authenticated_endpoints": 7,
        "public_endpoints": 1
      }
    }
  ],
  "default_version": "v1"
}
```

## Advanced Features

### AST Directives

Use special comments to enhance documentation:

```go
// API_SOURCE - Marks file for analysis
// API_DESC Create a new todo item with validation and error handling
// API_TAGS todos,create,validation,errors
func createTodoHandler(c *core.RequestEvent) error {
    // Implementation...
}
```

### Authentication Detection

The system automatically detects and documents authentication requirements:

```go
// Automatically documented as "No authentication required"
v1Router.GET("/api/v1/health", healthHandler)

// Automatically documented as "Guest only (not authenticated users)"
v1Router.POST("/api/v1/register", registerHandler).Bind(apis.RequireGuestOnly())

// Automatically documented as "User authentication required"
v1Router.GET("/api/v1/todos", getTodosHandler).Bind(apis.RequireAuth())

// Automatically documented as "Admin/superuser authentication required"
v1Router.DELETE("/api/v1/admin/users/{id}", deleteUserHandler).Bind(apis.RequireSuperuserAuth())

// Automatically documented as "Owner or admin access required"
v1Router.GET("/api/v1/users/{id}/profile", getProfileHandler).Bind(
    apis.RequireSuperuserOrOwnerAuth("id"),
)
```

### Version Lifecycle Management

Control version availability and status:

- **stable**: Production-ready, fully supported
- **testing**: Preview/beta version, limited support
- **development**: Experimental features, frequent changes
- **deprecated**: Legacy version, scheduled for removal
- **maintenance**: Bug fixes only, no new features

### Global Version Manager

Access the version manager globally in your application:

```go
// Set global version manager for app-wide access
api.SetGlobalVersionManager(versionManager)

// Access from anywhere in your application
globalVM := api.GetGlobalVersionManager()
if globalVM != nil {
    versions := globalVM.GetAllVersions()
    // Use version information...
}
```

## Benefits

✅ **Zero Configuration Overhead** - Works out of the box with minimal setup  
✅ **Type-Safe Documentation** - Schemas generated from actual Go types  
✅ **Always Up-to-Date** - Documentation updates automatically with code changes  
✅ **Multi-Version Support** - Manage multiple API versions simultaneously  
✅ **Standards Compliant** - Full OpenAPI 3.0 compatibility  
✅ **PocketBase Optimized** - Built specifically for PocketBase patterns and middleware  
✅ **Production Ready** - Thread-safe, performant, and battle-tested  
✅ **Developer Friendly** - Rich debugging tools and clear error messages  

## Integration with API Tools

The generated OpenAPI specifications work seamlessly with:

- **Swagger UI** - Interactive API documentation
- **Postman** - Import specifications for testing
- **OpenAPI Generator** - Generate client SDKs in multiple languages
- **API Testing Tools** - Automated testing based on specifications
- **Documentation Sites** - Embed interactive API docs

This system transforms your PocketBase application into a **self-documenting, version-aware API platform** that scales with your development workflow and provides exceptional developer experience for both API creators and consumers.