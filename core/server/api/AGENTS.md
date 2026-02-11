# API Documentation System — Agent Guide

This package (`core/server/api/`) is the OpenAPI documentation engine for pb-ext. It parses Go source at startup via AST analysis, extracts handler metadata and type schemas, and serves versioned OpenAPI 3.0.3 specs.

## File Map

| File | What it owns |
|---|---|
| `ast.go` | `ASTParser` — core pipeline: file discovery, struct extraction (two-pass), handler analysis, map literal / variable / expression analysis, schema generation from Go types |
| `ast_types.go` | All AST data structures: `ASTParser`, `StructInfo`, `FieldInfo`, `ASTHandlerInfo`, `MapKeyAdd`, `ParamInfo`, interfaces (`ASTParserInterface`) |
| `api_types.go` | `APIEndpoint`, `APIDocs`, `APIDocsConfig`, `AuthInfo`, `HandlerInfo` — the route/endpoint model |
| `openapi_schema_types.go` | Full OpenAPI 3.0 type hierarchy: `OpenAPISchema`, `OpenAPIPathItem`, `OpenAPIOperation`, `OpenAPIComponents`, etc. |
| `schema_types.go` | `SchemaGenerator` type, `SchemaAnalysisResult`, `SchemaConfig`, `SchemaGeneratorInterface` |
| `common_types.go` | `Logger` interface, `DefaultLogger` |
| `registry.go` | `APIRegistry` — coordinates endpoints, AST parser, schema generator; builds final OpenAPI paths and components; prunes `$ref` targets to only referenced schemas |
| `schema.go` | `SchemaGenerator` implementation — request/response schema inference, component schema collection, fallback patterns |
| `schema_conversion.go` | Go type to OpenAPI conversion, validation tag parsing, struct-to-schema conversion |
| `discovery.go` | `RouteAnalyzer`, `MiddlewareAnalyzer`, `PathAnalyzer` — runtime route analysis utilities |
| `version_manager.go` | `APIVersionManager`, `VersionedAPIRouter`, `VersionedRouteChain` — multi-version routing and per-version registries |
| `debug_dump.go` | `BuildDebugData()` — serves the `/api/docs/debug/ast` endpoint with full pipeline introspection |
| `utils.go` | String helpers: handler name extraction, camelCase/snake_case, description/tag generation, path conversion |

## Pipeline Overview

```
Source files (// API_SOURCE)
  |
  v
ASTParser.DiscoverSourceFiles()
  |
  |-- Phase 1: Parse API_SOURCE files
  |     |
  |     v
  |   ASTParser.ParseFile()  (for each API_SOURCE file)
  |     |-- extractStructs()            two-pass: register all structs, then generate JSONSchemas
  |     |-- extractFuncReturnTypes()    scan non-handler functions for return type signatures
  |     |-- extractHandlers()           find func(c *core.RequestEvent) error
  |     |     |-- parseHandlerComments()   // API_DESC, // API_TAGS
  |     |     |-- analyzePocketBasePatterns()
  |     |     |     |-- BindBody / JSON / Decode detection
  |     |     |     |-- trackVariableAssignment()   vars + map["key"]=value additions
  |     |     |     \-- auth / database operation detection
  |     |     |-- extractLocalVariables()
  |     |     \-- extractQueryParameters()  detect q := URL.Query(); q.Get("param") patterns
  |     \-- marks directory in parsedDirs
  |
  |-- Phase 2: Follow local imports (zero-config)
  |     |-- parseImportedPackages()     collect imports from all API_SOURCE files
  |     |     |-- getModulePath()       read go.mod for module name (cached on parser)
  |     |     |-- resolve imports       strip module prefix → local directory path
  |     |     |-- skip parsedDirs       avoid re-parsing API_SOURCE directories
  |     |     \-- parseDirectoryStructs()  extract structs only from each dir (no handlers)
  |     \-- re-run schema generation    imported structs may cross-reference each other
  v
APIRegistry.RegisterRoute(method, path, handler, middlewares)
  |-- RouteAnalyzer: handler name, auth, path params, tags
  |-- enhanceEndpointWithAST(): match handler name -> ASTHandlerInfo
  |     priority: AST data > SchemaGenerator fallback
  \-- RegisterEndpoint() -> rebuild paths + tags
  v
GetDocsWithComponents()
  |-- collect all $ref targets from paths (recursive)
  |-- prune component schemas to only referenced ones
  \-- return OpenAPI 3.0.3 spec
```

## AST Parser Internals

### Two-Pass Struct Extraction

Pass 1 registers all structs with fields (no schemas) and type aliases. Pass 2 generates `JSONSchema` for each struct now that cross-references resolve. This is critical — changing it to single-pass will break nested struct `$ref` resolution.

### Handler Analysis

A function is a PocketBase handler if its signature is `func(param *core.RequestEvent) error`. Analysis walks the body AST and tracks:

- **Variables**: `map[string]string` — variable name to inferred Go type
- **VariableExprs**: `map[string]ast.Expr` — variable name to RHS AST node (for deep map literal analysis)
- **MapAdditions**: `map[string][]MapKeyAdd` — dynamic `mapVar["key"] = value` assignments found after the initial literal

Request detection: `c.BindBody(&req)` or `json.NewDecoder(...).Decode(&req)` — type resolved from the variable's tracked type.

Response detection: `c.JSON(status, expr)` — the second argument is analyzed:
1. Try composite literal analysis (map/struct/slice)
2. If arg is a variable, trace to its stored expression
3. Merge any `MapAdditions` for that variable
4. Fall back to type inference -> `$ref` for known structs
5. Last resort: generic object schema

### Function Return Type Resolution

`extractFuncReturnTypes()` runs before handler analysis. It scans all non-handler function declarations in API_SOURCE files and extracts the first non-error return type from the function signature. Stored in `ASTParser.funcReturnTypes` as `map[string]string` (func name → Go type string).

This enables `inferTypeFromExpression` and `analyzeValueExpression` to resolve variables assigned from helper function calls (e.g., `candles := formatCandlesFull(records)` → type `[]map[string]any` → schema `{type: "array", items: {type: "object", additionalProperties: true}}`).

**Limitation**: Only covers functions defined in API_SOURCE files, not imported packages.

### Query Parameter Detection

`extractQueryParameters()` detects `q := e.Request.URL.Query()` variable assignments and subsequent `q.Get("paramName")` calls. Detected parameters are added to `handlerInfo.Parameters` as `ParamInfo` entries with `Source: "query"`. The conversion pipeline in `schema_conversion.go` already handles `ParamInfo` → `OpenAPIParameter`.

### Auto-Import Following (Cross-Package Struct Resolution)

After all API_SOURCE files are parsed, `parseImportedPackages()` automatically resolves local imports (same Go module) and parses their struct definitions. This is zero-config — no directives needed on type definition files.

**How it works:**
1. `getModulePath()` reads `go.mod` (walking up from cwd) to get the module name. Cached in `ASTParser.modulePath`.
2. For each API_SOURCE file, re-parse its imports. If an import path starts with `modulePath`, strip the prefix to get a relative directory.
3. Skip directories already in `parsedDirs` (marked during Phase 1 when parsing API_SOURCE files).
4. `parseDirectoryStructs(dir)` parses all `.go` files (excluding `_test.go`) in the directory, calling `extractStructs()` only — no handler analysis.
5. After all imported directories are processed, re-run `generateStructSchema()` for all structs so cross-references between imported structs resolve correctly.

**Key fields on ASTParser:**
- `modulePath string` — Go module path from go.mod (e.g., `github.com/magooney-loon/pb-ext`)
- `parsedDirs map[string]bool` — tracks directories already parsed to avoid duplicates

**Edge cases:**
- No `go.mod` found → `modulePath` stays empty, feature silently disabled (falls back to current behavior)
- External imports (different module) → ignored, only local module imports are followed
- API_SOURCE file's own directory → already in `parsedDirs` from Phase 1, won't be re-parsed
- `_test.go` files → skipped in `parseDirectoryStructs`
- Circular imports → not an issue since we only extract structs, never follow further imports

### Value Expression Analysis

`analyzeValueExpression()` resolves map literal values to OpenAPI schemas. It handles:
- Literals (string, int, float, bool)
- Variable references (looks up `VariableExprs` then `Variables`)
- Struct field access (`req.DryRun` -> looks up struct definition -> field type)
- Call expressions (`time.Now().Format()`, `len()`, `make()`, PocketBase getters)
- Nested composite literals (maps, slices, structs)

The `handlerInfo` parameter is threaded through the entire chain: `analyzeMapLiteralSchema` -> `parseMapLiteral` -> `analyzeValueExpression` -> `analyzeCompositeLitSchema` -> `parseArrayLiteral`.

### Type Resolution

`resolveTypeToSchema(typeName)` converts Go type strings to OpenAPI schemas. Handles: slices (`[]T`), maps (`map[K]V`), pointers (`*T`), primitives, `time.Time`, `any`/`interface{}`, and known structs (via `$ref`).

`generateSchemaForEndpoint(typeName)` is the top-level variant that always uses `$ref` for known structs (so they land in `components/schemas`).

`generateSchemaFromType(typeName, inline)` controls whether structs are inlined or referenced.

## Versioning System

Each API version (`v1`, `v2`, ...) gets its own isolated `ASTParser`, `SchemaGenerator`, and `APIRegistry`. `APIVersionManager` coordinates them.

`VersionedAPIRouter` wraps PocketBase's router and registers routes into the version's registry. `VersionedRouteChain` handles `.Bind(middleware)` chaining.

`GetDocsWithComponents()` prunes component schemas per-version — only schemas actually `$ref`'d from that version's paths are included. `Error` and `PocketBaseRecord` are always included.

## Source File Directives

| Directive | Where | Purpose |
|---|---|---|
| `// API_SOURCE` | Top of .go file | Marks file for AST parsing |
| `// API_DESC <text>` | Function doc comment | Handler description in OpenAPI |
| `// API_TAGS <csv>` | Function doc comment | Comma-separated endpoint tags |

## Debug Endpoint

`GET /api/docs/debug/ast` returns the full pipeline state: parsed structs (with fields, json tags, pointer info), handlers (with variables, expressions, map additions, schemas), per-version endpoints, component schemas, and the complete OpenAPI output. Requires auth.

## Common Pitfalls

- **Adding a new detection pattern in `analyzePocketBaseCall`**: must also consider how the detected data flows into `handleJSONResponse` or `handleBindBody`. Test with the debug endpoint.
- **Modifying `trackVariableAssignment`**: the order matters — `VariableExprs` must be stored even when type inference fails, because `handleJSONResponse` uses it for map literal tracing.
- **Changing struct schema generation**: `generateStructSchema` handles embedded struct flattening recursively. Pointer fields get `nullable: true` only when `Ref == ""` (inline schemas, not `$ref`s).
- **`$ref` vs inline**: endpoint-level schemas use `$ref` via `generateSchemaForEndpoint`. Nested types in struct fields use `$ref` via `generateFieldSchema` -> `generateSchemaFromType(type, inline=false)`. Map literal values use inline schemas from `analyzeValueExpression`.
- **Type aliases**: resolved via `resolveTypeAlias()` with cycle detection. Qualified types (`pkg.Type`) are resolved to simple names.
- **Component pruning**: `GetDocsWithComponents()` walks all `$ref` targets recursively. If you add a new place where `$ref` can appear, make sure `collectRefsFromSchema` covers it.
- **`extractFuncReturnTypes` ordering**: must run BEFORE `extractHandlers` in `ParseFile`, so that `inferTypeFromExpression` can resolve function call return types during handler body analysis.
- **`funcReturnTypes` scope**: only covers functions in API_SOURCE files. Functions from imported packages won't be resolved — their call sites fall through to heuristic matching in `analyzeValueExpression`.
- **Import following scope**: only resolves struct definitions from local imports. Handlers, func return types, and type aliases in imported packages are NOT extracted — only `extractStructs()` is called on imported directories.
- **Duplicate struct names**: if an imported package defines a struct with the same name as one in an API_SOURCE file, last-parsed wins. No package-qualified naming yet.

## Test Coverage

Tests are in `*_test.go` files in this package. Run with:
```
go test ./core/server/api/... -v
```

Many version manager tests require PocketBase mocks and are skipped. The AST parser, registry, and schema tests run fully.
