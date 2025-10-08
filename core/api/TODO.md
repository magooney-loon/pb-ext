# pb-ext Core API System - Migration TODO

Simple migration of existing API system to new structure with 100% OpenAPI 3.0 compatibility.

## 🎯 Current System Features (to be migrated exactly)

- ✅ Multi-version API management
- ✅ AST-based Go code analysis
- ✅ Auto-discovery of routes and handlers
- ✅ Authentication middleware detection
- ✅ JSON schema generation from Go structs
- ✅ Version-specific route registration
- ✅ OpenAPI-compatible output (needs improvement)

---

## 📁 File Migration Map

### ✅ Completed
- `core/server/api/version_manager.go` → `core/api/versioning/manager.go` (DONE)
- `core/server/api/types.go` → `core/api/versioning/types.go` (DONE)

### 🔄 TODO: Direct Migration
- `core/server/api/ast.go` → `core/api/discovery/ast.go`
- `core/server/api/discovery.go` → `core/api/discovery/engine.go`
- `core/server/api/registry.go` → `core/api/registry/registry.go`
- `core/server/api/schema.go` → `core/api/schema/generator.go`
- `core/server/api/utils.go` → `core/api/utils/` (split utilities)

---

## 🎯 Implementation Tasks

### 1. Discovery Module (`core/api/discovery/`)
- [ ] `ast.go` - **Migrate AST parser exactly as-is**
  - ASTParser struct and methods
  - Go file parsing and analysis
  - Handler detection from AST
  - Comment directive parsing (`// API_DESC`, `// API_TAGS`)
  - Type extraction and analysis

- [ ] `engine.go` - **Migrate discovery engine exactly as-is**
  - Auto-discovery functionality
  - Route detection
  - Middleware analysis
  - Handler signature analysis

### 2. Registry Module (`core/api/registry/`)
- [ ] `registry.go` - **Migrate APIRegistry exactly as-is**
  - Route registration and storage
  - Endpoint management per version
  - Thread-safe operations
  - Statistics collection

### 3. Schema Module (`core/api/schema/`)
- [ ] `generator.go` - **Migrate SchemaGenerator exactly as-is**
  - JSON Schema generation from Go types
  - Struct tag parsing (json, validate, etc.)
  - Type mapping and conversion
  - **PLUS: Add proper OpenAPI 3.0 schema output**

### 4. Utils Module (`core/api/utils/`)
- [ ] `auth.go` - Authentication utility functions
- [ ] `http.go` - HTTP helper functions
- [ ] `reflect.go` - Reflection utilities
- [ ] `validation.go` - Validation helpers

---

## 🚀 OpenAPI 3.0 Enhancement

### Current Output → OpenAPI 3.0
- [ ] **Upgrade schema format** from basic JSON to OpenAPI 3.0 Schema
- [ ] **Add proper OpenAPI structure**:
  ```json
  {
    "openapi": "3.0.3",
    "info": { ... },
    "servers": [ ... ],
    "paths": { ... },
    "components": { 
      "schemas": { ... },
      "securitySchemes": { ... }
    }
  }
  ```
- [ ] **Enhanced auth documentation** - Map PocketBase auth to OpenAPI security schemes
- [ ] **Response status codes** - Document all possible response codes
- [ ] **Request/Response examples** - Add example payloads

### Auth Mapping to OpenAPI Security
```go
// Current auth detection → OpenAPI security schemes
RequireAuth() → bearerAuth security scheme
RequireSuperuserAuth() → bearerAuth + admin scope
RequireGuestOnly() → no security
RequireSuperuserOrOwnerAuth() → bearerAuth + ownership check
```

---

## 📋 Implementation Checklist

### Phase 1: Direct Migration (Week 1)
- [ ] Copy `ast.go` to `core/api/discovery/ast.go` with minimal changes
- [ ] Copy `discovery.go` to `core/api/discovery/engine.go` with minimal changes
- [ ] Copy `registry.go` to `core/api/registry/registry.go` with minimal changes  
- [ ] Copy `schema.go` to `core/api/schema/generator.go` with minimal changes
- [ ] Split `utils.go` into appropriate utility modules
- [ ] Update import paths in all files
- [ ] Ensure all existing functionality works exactly the same

### Phase 2: OpenAPI Enhancement (Week 2)
- [ ] Enhance schema generator to output proper OpenAPI 3.0 schemas
- [ ] Add OpenAPI structure wrapper around existing output
- [ ] Map authentication middleware to OpenAPI security schemes
- [ ] Add proper response status code documentation
- [ ] Add request/response examples where possible
- [ ] Validate output against OpenAPI 3.0 spec

### Phase 3: Integration & Testing (Week 3)
- [ ] Create adapter layer to maintain existing API
- [ ] Test all existing functionality works unchanged
- [ ] Test new OpenAPI output with validators
- [ ] Update documentation and examples
- [ ] Prepare deprecation notices for old paths

---

## 🎯 Success Criteria

1. **Exact Feature Parity**: Everything that works now still works exactly the same
2. **100% OpenAPI 3.0 Compatible**: Output validates against OpenAPI spec
3. **Same Performance**: No performance regression
4. **Drop-in Replacement**: Can switch with minimal code changes
5. **Better Structure**: Cleaner module organization

---

## 🚫 What NOT to Add

- Complex enterprise features
- Advanced caching systems  
- Plugin architectures
- Multiple storage backends
- Advanced security features
- Monitoring and metrics
- Complex configuration systems

Keep it simple - just migrate existing features with better OpenAPI output!

---

## 🔧 Quick Start Commands

```bash
# Phase 1: Copy files with structure
cp core/server/api/ast.go core/api/discovery/ast.go
cp core/server/api/discovery.go core/api/discovery/engine.go  
cp core/server/api/registry.go core/api/registry/registry.go
cp core/server/api/schema.go core/api/schema/generator.go

# Update package declarations and imports
# Test existing functionality
# Enhance OpenAPI output
```
