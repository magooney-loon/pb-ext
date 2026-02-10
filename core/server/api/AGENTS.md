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
ASTParser.ParseFile()
  |-- extractStructs()        two-pass: register all structs, then generate JSONSchemas
  |-- extractHandlers()        find func(c *core.RequestEvent) error
  |     |-- parseHandlerComments()   // API_DESC, // API_TAGS
  |     |-- analyzePocketBasePatterns()
  |     |     |-- BindBody / JSON / Decode detection
  |     |     |-- trackVariableAssignment()   vars + map["key"]=value additions
  |     |     \-- auth / database operation detection
  |     \-- extractLocalVariables()
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

## Test Coverage

Tests are in `*_test.go` files in this package. Run with:
```
go test ./core/server/api/... -v
```

Many version manager tests require PocketBase mocks and are skipped. The AST parser, registry, and schema tests run fully.
