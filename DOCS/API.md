# API Documentation System — Refactoring Plan

## Overview

The API doc system (`core/server/api/`) uses Go AST parsing at startup to auto-generate OpenAPI 3.0.3 docs from handler source files. It's already the most well-structured part of the codebase.

**Status**: ✅ Complete
**Priority**: Low — system works correctly, existing interfaces are solid
**Backward Compatibility**: Hard requirement

---

## Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1 | Split `registry.go` into focused files | ✅ Done |
| Phase 2 | Consolidate type files (5 → 3) | ✅ Done |
| Phase 3 | Split `ast.go` into pipeline files | ✅ Done |
| Phase 4 | Add spec caching to `APIRegistry` | ✅ Done |
| Phase 5 | Surface AST parse errors with `slog.Warn` | ✅ Done |

---

## Architecture (Current State)

```
core/server/api/
├── registry.go          # APIRegistry struct, NewAPIRegistry(), core CRUD, private helpers
├── registry_spec.go     # OpenAPI spec assembly (buildPaths, collectRefs, GetDocsWithComponents)
├── registry_routes.go   # Route registration (RegisterRoute, BatchRegisterRoutes, enhanceEndpointWithAST)
├── ast.go               # ASTParser entry points + public interface methods (Get*, ClearCache, EnhanceEndpoint)
├── ast_file.go          # File discovery, go.mod parsing, import resolution, struct-only parsing
├── ast_struct.go        # First pass: struct extraction, field parsing, schema generation, deepCopySchema
├── ast_func.go          # Second pass: handler extraction, PocketBase pattern analysis, variable tracking
├── ast_metadata.go      # Value/type resolution, map literal analysis, generateSchemaFromType, NewPocketBasePatterns
├── types_openapi.go     # Full OpenAPI 3.0.3 spec types + schema builder
├── types_ast.go         # ASTParser struct, AST node types (StructInfo, FieldInfo, etc.), Logger interface
├── types_api.go         # APIEndpoint, AuthInfo, APIDocs, APIDocsConfig, SchemaGenerator types + interfaces
├── schema.go            # SchemaGenerator — reflect → OpenAPI schema (unchanged)
├── schema_conversion.go # Go type → OpenAPI type conversion (unchanged)
├── version_manager.go   # VersionManager, VersionedAPIRouter (unchanged)
├── discovery.go         # Auto-discovery of handler files (unchanged)
├── debug_dump.go        # /api/docs/debug/ast endpoint (unchanged)
└── utils.go             # Shared helpers (naming, paths, etc.) (unchanged)
```

---

## Phase 1: Split `registry.go` — ✅ Complete

**What changed**: 782-line `registry.go` split into 3 focused files.

- `registry.go` — struct, `NewAPIRegistry()`, core CRUD (`RegisterEndpoint`, `GetDocs`, `GetEndpoint`, etc.), private helpers (`endpointKey`, `rebuildEndpointsList`)
- `registry_spec.go` — spec assembly: `buildPaths`, `endpointToOperation`, `extractPathParameters`, `generateOperationId`, `buildTags`, `collectRefs*`, `schemaNameFromRef`, `GetDocsWithComponents`
- `registry_routes.go` — route registration: `RegisterRoute`, `RegisterExplicitRoute`, `BatchRegisterRoutes`, `RouteDefinition`, `createEndpointFromAnalysis`, `enhanceEndpointWithAST`, `getASTAuthDescription`

No exported names changed. Pure file-organization split.

---

## Phase 2: Consolidate Type Files — ✅ Complete

**What changed**: 5 type files merged into 3 with clear ownership.

| New File | Contains | Replaced |
|---|---|---|
| `types_openapi.go` | All OpenAPI 3.0.3 spec types + schema builder | `openapi_schema_types.go` + OpenAPI types from `api_types.go` + `common_types.go` |
| `types_ast.go` | ASTParser struct, AST node types, Logger interface | `ast_types.go` + `common_types.go` |
| `types_api.go` | APIEndpoint, AuthInfo, APIDocs, configs, SchemaGenerator types + interfaces | `api_types.go` + `schema_types.go` |

---

## Phase 3: Split `ast.go` — ✅ Complete

**What changed**: 2353-line `ast.go` split into 5 pipeline files.

- `ast.go` (275 lines) — `NewASTParser`, `DiscoverSourceFiles`, `ParseFile`, all public `ASTParserInterface` methods
- `ast_file.go` (127 lines) — `getModulePath`, `newFileSet`, `parseImportedPackages`, `parseDirectoryStructs`
- `ast_struct.go` (337 lines) — `extractStructs`, `parseStruct`, `parseJSONTag`, `extractTag`, `generateStructSchema`, `flattenEmbeddedFields`, `generateFieldSchema`, `deepCopySchema`
- `ast_func.go` (721 lines) — `extractHandlers`, `extractFuncReturnTypes`, `analyzeHelperFuncBody`, `extractQueryParameters`, `isPocketBaseHandler`, `analyzePocketBaseHandler`, all `handle*` functions, `trackVariableAssignment`, `mergeMapAdditions`, `enrichArraySchemaFromAppend`, `inferTypeFromExpression`, `extractTypeName`
- `ast_metadata.go` (657 lines) — `analyzeMapLiteralSchema`, `parseMapLiteral`, `analyzeValueExpression`, `resolveTypeToSchema`, `analyzeCompositeLitSchema`, `parseArrayLiteral`, variable extraction, `resolveTypeAlias`, `generateSchemaForEndpoint`, `generateSchemaFromType`, `NewPocketBasePatterns`

`ASTParserInterface` unchanged.

---

## Phase 4: Add Spec Caching — ✅ Complete

**What changed**: `GetDocsWithComponents` now caches the assembled spec.

```go
// Added to APIRegistry struct:
cachedSpecDocs *APIDocs
specDirty      bool

// rebuildEndpointsList sets specDirty = true, cachedSpecDocs = nil
// ClearEndpoints sets specDirty = true, cachedSpecDocs = nil
// GetDocsWithComponents: returns cached spec if !specDirty, stores result after rebuild
```

No interface changes. Transparent performance improvement — spec is only assembled when endpoints change.

---

## Phase 5: Surface AST Parse Errors — ✅ Complete

**What changed**: Silent parse-error skips in `ast_file.go` and `ast.go` now log via `slog.Warn`.

```go
// ast.go — DiscoverSourceFiles
slog.Warn("api docs: failed to parse API_SOURCE file", "file", f, "err", parseErr)

// ast_file.go — parseImportedPackages
slog.Warn("api docs: failed to parse handler file for import scan", "file", f, "err", err)

// ast_file.go — parseDirectoryStructs
slog.Warn("api docs: failed to parse struct file", "file", filePath, "err", err)
```

Errors still handled gracefully (continue), just no longer silent.

---

## What We Did NOT Do

- **Not** creating sub-packages (`types/`, `registry/`, `parser/`, `schema/`, `version/`) — flat Go packages are idiomatic
- **Not** adding disk-based AST caching — memory cache (Phase 4) is sufficient
- **Not** supporting GraphQL/gRPC — out of scope
- **Not** creating a CLI validation tool — YAGNI

---

## Success Criteria

- [x] `go build ./...` clean after each phase
- [x] No file in the package exceeds 900 lines (largest: `schema.go` at 842, `version_manager.go` at 819, `ast_func.go` at 721)
- [x] All 5 type files merged to 3
- [x] `ASTParserInterface` and `SchemaGeneratorInterface` unchanged
- [x] AST parse errors logged with file context (not silently swallowed)

---

**Last Updated**: 2026-02-20
**Status**: ✅ Complete
