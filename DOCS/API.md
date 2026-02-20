# API Documentation System — Refactoring Plan

## Overview

The API doc system (`core/server/api/`) uses Go AST parsing at startup to auto-generate OpenAPI 3.0.3 docs from handler source files. It's already the most well-structured part of the codebase.

**Status**: Well-architected, targeted improvements only
**Priority**: Low — system works correctly, existing interfaces are solid
**Backward Compatibility**: Hard requirement

---

## Current Architecture

```
core/server/api/
├── registry.go              # APIRegistry — main orchestrator
├── ast.go                   # ASTParser — source file parsing (large)
├── schema.go                # SchemaGenerator — reflect → OpenAPI schema
├── schema_conversion.go     # Go type → OpenAPI type conversion
├── version_manager.go       # VersionManager, VersionedAPIRouter
├── discovery.go             # Auto-discovery of handler files
├── debug_dump.go            # /api/docs/debug/ast endpoint
├── utils.go                 # Shared helpers (naming, paths, etc.)
├── api_types.go             # APIEndpoint, HandlerInfo, AuthInfo
├── ast_types.go             # StructInfo, FieldInfo, TypeInfo, FunctionInfo
├── openapi_schema_types.go  # Full OpenAPI 3.0.3 spec types
├── common_types.go          # Shared primitive types
└── schema_types.go          # Schema-specific types
```

### How It Works

1. At startup, `VersionedAPIRouter` calls `discovery.go` to find all `// API_SOURCE` files
2. `ast.go` parses each file with a two-pass algorithm: structs first, then function handlers
3. Handler comments (`// API_DESC`, `// API_TAGS`) become endpoint metadata
4. `schema.go` reflects Go structs into OpenAPI schema objects
5. `registry.go` assembles the full OpenAPI spec and serves it at `/api/docs`

**Key existing interfaces** (already in the code):
```go
type ASTParserInterface interface {
    ParseSourceFiles(paths []string) (*FileParseResult, error)
    ExtractMetadata(filePath, fileContent string) (*ASTHandlerInfo, error)
}

type SchemaGeneratorInterface interface {
    GenerateSchema(typ reflect.Type) *OpenAPISchema
}
```

These are already injected into `APIRegistry` via `NewAPIRegistry(config, astParser, schemaGenerator)`.

---

## Assessment

### What's Working Well
- Clean dependency injection via interfaces (already done)
- Comprehensive test coverage across all components
- The two-pass AST algorithm is well-documented in `AGENTS.md`
- `VersionedAPIRouter` cleanly wraps PocketBase's router
- Schema generation correctly handles nested structs, slices, maps, pointers

### Actual Issues (not theoretical)

**1. `registry.go` is too large**
At 783 lines / 44 functions, it's hard to navigate. Functions range from spec assembly to swagger UI serving to validation. These concerns should be in separate files within the same package.

**2. Type files are confusing**
Five type files (`api_types.go`, `ast_types.go`, `openapi_schema_types.go`, `common_types.go`, `schema_types.go`) with no clear rule for which type goes where. Makes it slow to find a type definition.

**3. `ast.go` is a single 73K file**
The AST parsing logic is complex enough that a 73K file makes it hard to work with. The parsing pipeline (file reading → struct extraction → function extraction → metadata assembly) could be split into focused files.

**4. Silent errors in AST parsing**
Some parse errors in `ast.go` are logged and skipped rather than surfaced. This means a malformed handler file might silently produce no docs without telling the user why.

**5. No spec caching**
The OpenAPI spec is re-assembled from scratch on every `/api/docs` request. For small services this is fine, but it's unnecessary work.

---

## Refactoring Plan

All changes stay **within the `api` package** (flat Go package, not sub-packages). Backward compatibility is maintained — no public API changes.

### Phase 1: Split `registry.go` (Low Risk)

Split the 783-line file into 4 focused files in the same package:

```
registry.go          # APIRegistry struct, NewAPIRegistry(), core state management
registry_spec.go     # OpenAPI spec assembly (GenerateOpenAPISpec, buildPaths, etc.)
registry_routes.go   # HTTP route registration (swagger UI, /api/docs endpoint)
registry_validate.go # Endpoint/path validation logic
```

This is purely a file-organization change. No exported names change, no behavior changes. The split just makes the file navigable.

### Phase 2: Consolidate Type Files (Low Risk)

Merge the 5 type files into 3 with clear ownership:

| New File | Contains | Replaces |
|---|---|---|
| `types_openapi.go` | Full OpenAPI 3.0.3 spec types | `openapi_schema_types.go` + OpenAPI types from `common_types.go` |
| `types_ast.go` | AST node types (StructInfo, FieldInfo, FunctionInfo, etc.) | `ast_types.go` + AST types from `common_types.go` |
| `types_api.go` | APIEndpoint, HandlerInfo, AuthInfo, schema request/response types | `api_types.go` + `schema_types.go` |

Same package, same exported names — just consolidated files.

### Phase 3: Split `ast.go` (Medium Risk)

Break the 73K file into a pipeline of files:

```
ast.go               # ASTParser struct, NewASTParser(), ParseSourceFiles() — entry points only
ast_file.go          # File reading, tokenization, source hash
ast_struct.go        # First pass: struct extraction and field analysis
ast_func.go          # Second pass: function/handler extraction
ast_metadata.go      # Comment parsing (API_DESC, API_TAGS, etc.), metadata assembly
```

Same package, same exported interface. The `ASTParserInterface` doesn't change. This is a file-split-only change — test suite verifies no behavioral change.

### Phase 4: Add Spec Caching (Low Risk)

Cache the assembled OpenAPI spec in `APIRegistry`. Invalidate on new endpoint registration.

```go
// Add to APIRegistry struct:
type APIRegistry struct {
    // ... existing fields ...
    cachedSpec     *APIDocs
    specDirty      bool
}

// In RegisterEndpoint: set specDirty = true
// In GetDocs/spec handler: regenerate only if specDirty, then set specDirty = false
```

No interface changes. Transparent performance improvement.

### Phase 5: Surface AST Parse Errors (Low Risk)

Change silent-skip errors in `ast.go` to return structured errors that get logged at warn level with the file path. No API change — errors are still handled gracefully, just not silently.

```go
// Before (silent skip):
if err != nil {
    continue
}

// After (logged warning):
if err != nil {
    slog.Warn("api docs: failed to parse handler file", "file", filePath, "err", err)
    continue
}
```

---

## What We're NOT Doing

- **Not** creating sub-packages (`types/`, `registry/`, `parser/`, `schema/`, `version/`) — Go flat packages are idiomatic, sub-packages create circular import risk and hurt IDE navigation for what is a cohesive system
- **Not** adding disk-based AST caching — memory cache (Phase 4) is sufficient
- **Not** supporting GraphQL/gRPC — out of scope
- **Not** creating a CLI validation tool — YAGNI

---

## Backward Compatibility

These phases don't change any exported names. The public API:

```go
// These stay identical:
NewAPIRegistry(config *APIDocsConfig, astParser ASTParserInterface, schemaGenerator SchemaGeneratorInterface) *APIRegistry
NewASTParser() *ASTParser
NewSchemaGenerator() *SchemaGenerator
InitializeVersionedSystem(versions map[string]*APIDocsConfig, defaultVersion string) *VersionManager
DefaultAPIDocsConfig() *APIDocsConfig
```

`go build ./...` must be clean after each phase before moving to the next.

---

## Implementation Order

1. Phase 1 (split registry.go) — easiest, highest impact on navigability
2. Phase 2 (consolidate types) — low risk, makes type lookup obvious
3. Phase 5 (surface errors) — one-line changes, do alongside Phase 1 or 2
4. Phase 3 (split ast.go) — most work, do last for behavioral confidence
5. Phase 4 (spec caching) — optional, do only if profiling shows it matters

---

## Success Criteria

- [ ] `go build ./...` clean after each phase
- [ ] No file in the package exceeds 400 lines
- [ ] All 5 type files merged to 3
- [ ] `ASTParserInterface` and `SchemaGeneratorInterface` unchanged
- [ ] AST parse errors logged with file context (not silently swallowed)
- [ ] Swagger UI and `/api/docs` endpoints work end-to-end

---

**Last Updated**: 2026-02-20
**Status**: Draft
