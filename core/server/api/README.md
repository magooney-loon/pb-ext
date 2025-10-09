# PocketBase API Documentation System

A streamlined, explicit API documentation system for PocketBase applications that combines the best of code analysis and clear route registration.

## What This System Does

### 🎯 **Explicit Route Documentation**
- Define routes once in code, get documentation automatically
- No complex auto-discovery - routes are explicitly registered
- Clean, predictable behavior with single source of truth

### 📝 **Smart Schema Extraction** 
- Analyzes Go code to extract request/response schemas
- Reads `API_DESC` and `API_TAGS` comments from handler functions
- Generates OpenAPI-compatible documentation from Go types

### 🌍 **Multi-Version API Support**
- Support multiple API versions simultaneously (v1, v2, etc.)
- Version-specific documentation and routing
- Clean version lifecycle management

### ⚡ **High Performance**
- No runtime route discovery overhead
- Schemas generated once at startup
- Fast, efficient documentation serving

## Architecture Overview

The streamlined flow is simple and predictable:

```
Routes.go → AST Parser → Registry → Documentation
    ↓           ↓           ↓            ↓
Source of   Schema     Coordinator   Final Output
Truth      Extraction   & Builder
```

**Core Components:**
- **Routes.go**: Single source of truth for all API routes
- **AST Parser**: Extracts type schemas and handler comments
- **Registry**: Combines route definitions with extracted schemas
- **Version Manager**: Coordinates multiple API versions

## Complete Setup Guide

### Step 1: Mark Your Go Files for Analysis

Add `// API_SOURCE` comment at the top of files containing handlers:

```go
package main

// API_SOURCE
// This file contains API handlers

import (
    "github.com/pocketbase/pocketbase/core"
)

// API_DESC Get current server time in multiple formats
// API_TAGS public,utility,time
func timeHandler(c *core.RequestEvent) error {
    // handler implementation...
}

// API_DESC Create a new todo item
// API_TAGS todos,create
func createTodoHandler(c *core.RequestEvent) error {
    var req CreateTodoRequest
    // handler implementation...
}
```

### Step 2: Configure API Versions

Create version configurations without unnecessary complexity:

```go
func createAPIVersions() map[string]*api.APIDocsConfig {
    baseConfig := &api.APIDocsConfig{
        Title:       "My API",
        Description: "API for my application",
        BaseURL:     "http://localhost:8090/",
        Enabled:     true,
    }

    // Version 1 - Stable
    v1Config := *baseConfig
    v1Config.Version = "1.0.0"
    v1Config.Status = "stable"

    // Version 2 - Testing
    v2Config := *baseConfig
    v2Config.Version = "2.0.0"
    v2Config.Status = "testing"

    return map[string]*api.APIDocsConfig{
        "v1": &v1Config,
        "v2": &v2Config,
    }
}
```

### Step 3: Register Routes Explicitly

Use the streamlined route registration approach:

```go
func registerRoutes(app core.App) {
    // Initialize with simplified config
    versionManager := api.InitializeVersionedSystem(createAPIVersions(), "v1")

    app.OnServe().BindFunc(func(e *core.ServeEvent) error {
        // Get version-specific routers
        v1Router, _ := versionManager.GetVersionRouter("v1", e)
        v2Router, _ := versionManager.GetVersionRouter("v2", e)

        // Register routes with clean organization
        registerV1Routes(v1Router)
        registerV2Routes(v2Router)

        return e.Next()
    })

    versionManager.RegisterWithServer(app)
}

// Option 1: Standard explicit registration
func registerV1Routes(router *api.VersionedAPIRouter) {
    prefix := "/api/v1"
    router.GET(prefix+"/todos", getTodosHandler)
    router.POST(prefix+"/todos", createTodoHandler).Bind(apis.RequireAuth())
    router.GET(prefix+"/todos/{id}", getTodoHandler)
    router.PATCH(prefix+"/todos/{id}", updateTodoHandler).Bind(apis.RequireAuth())
    router.DELETE(prefix+"/todos/{id}", deleteTodoHandler).Bind(apis.RequireAuth())
}

// Option 2: Using prefixed router (less repetition)
func registerV2Routes(router *api.VersionedAPIRouter) {
    v2 := router.SetPrefix("/api/v2")
    v2.GET("/time", timeHandler)
    v2.GET("/status", statusHandler)
}

// Option 3: CRUD convenience method (maximum efficiency)
func registerCRUDRoutes(router *api.VersionedAPIRouter) {
    v1 := router.SetPrefix("/api/v1")
    v1.CRUD("todos", api.CRUDHandlers{
        List:   getTodosHandler,
        Create: createTodoHandler,
        Get:    getTodoHandler,
        Patch:  updateTodoHandler,
        Delete: deleteTodoHandler,
    }, apis.RequireAuth()) // Auth applied to Create, Update, Patch, Delete automatically
}
```

### Step 4: Define Request/Response Types

Create clear Go types for your API schemas:

```go
// Request types - automatically documented
type CreateTodoRequest struct {
    Title       string `json:"title"`
    Description string `json:"description,omitempty"`
    Priority    string `json:"priority,omitempty"` // "low", "medium", "high"
    Tags        []string `json:"tags,omitempty"`
    DueDate     *string `json:"due_date,omitempty"`
}

type UpdateTodoRequest struct {
    Title       *string `json:"title,omitempty"`
    Description *string `json:"description,omitempty"`
    Priority    *string `json:"priority,omitempty"`
    Completed   *bool   `json:"completed,omitempty"`
    Tags        []string `json:"tags,omitempty"`
}

// Response types - automatically documented
type TodoResponse struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Priority    string    `json:"priority"`
    Completed   bool      `json:"completed"`
    Tags        []string  `json:"tags"`
    CreatedAt   string    `json:"created_at"`
    UpdatedAt   string    `json:"updated_at"`
    DueDate     *string   `json:"due_date,omitempty"`
}

type TodoListResponse struct {
    Todos      []TodoResponse `json:"todos"`
    TotalCount int           `json:"total_count"`
    Page       int           `json:"page"`
    PerPage    int           `json:"per_page"`
}
```

### Step 5: Access Your Documentation

#### Version-Specific Documentation
- **V1 Docs**: `GET /api/docs/v1` - Complete OpenAPI spec for version 1
- **V2 Docs**: `GET /api/docs/v2` - Complete OpenAPI spec for version 2

#### Version Management
- **All Versions**: `GET /api/docs/versions` - List all available versions
- **Default Version**: `GET /api/docs` - Documentation for default version

#### Debug Information
- **AST Debug**: `GET /api/docs/debug/ast` - View parsed schemas and handlers (requires auth)

Example response from `/api/docs/v1`:
```json
{
  "openapi": "3.0.0",
  "info": {
    "title": "My API",
    "version": "1.0.0",
    "description": "API for my application"
  },
  "paths": {
    "/api/v1/todos": {
      "get": {
        "description": "Get todos list",
        "tags": ["todos", "list"],
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/TodoListResponse"
                }
              }
            }
          }
        }
      },
      "post": {
        "description": "Create a new todo item",
        "tags": ["todos", "create"],
        "security": [{"auth": []}],
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/CreateTodoRequest"
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "CreateTodoRequest": {
        "type": "object",
        "properties": {
          "title": {"type": "string"},
          "description": {"type": "string"},
          "priority": {"type": "string"}
        },
        "required": ["title"]
      }
    }
  }
}
```

## Key Features

### Handler Comment Directives
Use special comments to enhance documentation:

```go
// API_DESC Create a new todo item with validation
// API_TAGS todos,create,validation
func createTodoHandler(c *core.RequestEvent) error {
    // Implementation automatically documented
}
```

### Automatic Authentication Detection
Authentication requirements are automatically detected from middleware:

```go
// Automatically documented as requiring authentication
router.POST("/todos", createTodoHandler).Bind(apis.RequireAuth())

// Automatically documented as requiring superuser auth
router.DELETE("/admin/users/{id}", deleteUserHandler).Bind(apis.RequireSuperuserAuth())
```

### Multiple Registration Patterns
Choose the style that fits your needs:

```go
// Explicit (full control)
router.GET("/api/v1/todos", getTodosHandler)

// Prefixed (less repetition)
v1 := router.SetPrefix("/api/v1")
v1.GET("/todos", getTodosHandler)

// CRUD convenience (maximum efficiency)
v1.CRUD("todos", api.CRUDHandlers{...}, apis.RequireAuth())
```

## Benefits of This Approach

### ✅ **Predictable & Reliable**
- Routes are explicitly defined - no surprises
- Clear, single source of truth for all endpoints
- Consistent behavior across environments

### ✅ **High Performance** 
- No runtime discovery overhead
- Schemas generated once at startup
- Fast documentation serving

### ✅ **Developer Friendly**
- Clean, readable route definitions
- Multiple convenience methods for different use cases
- Excellent error messages and debugging

### ✅ **Maintainable**
- Clear separation of concerns
- Easy to understand and modify
- Scales well with project growth

### ✅ **Flexible**
- Support for multiple API versions
- Various registration patterns
- Easy integration with existing PocketBase apps

## Integration with API Tools

The generated OpenAPI 3.0 documentation works seamlessly with:

- **Postman** - Import OpenAPI spec for instant API collection
- **Swagger UI** - Interactive API documentation and testing
- **Insomnia** - API client with OpenAPI import
- **curl** - Copy-paste ready HTTP requests
- **SDK Generation** - Generate client libraries for various languages

This streamlined approach provides all the power you need for professional API documentation while keeping the code clean, simple, and maintainable.