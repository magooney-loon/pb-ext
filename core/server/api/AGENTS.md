# API Documentation System — Agent Guide

This package (`core/server/api/`) is the OpenAPI documentation engine for pb-ext. It parses Go source at startup via AST analysis, extracts handler metadata and type schemas, and serves versioned OpenAPI 3.0.3 specs.

## File Map

### Types
| File | What it owns |
|---|---|
| `types_ast.go` | All AST data structures: `ASTParser`, `StructInfo`, `FieldInfo`, `ASTHandlerInfo`, `MapKeyAdd`, `ParamInfo`, `ParseError`, `PocketBasePatterns`, logger interface, `ASTParserInterface` |
| `types_api.go` | `APIEndpoint`, `APIDocs`, `APIDocsConfig`, `AuthInfo`, `HandlerInfo` — the route/endpoint model |
| `types_openapi.go` | Full OpenAPI 3.0 type hierarchy: `OpenAPISchema`, `OpenAPIPathItem`, `OpenAPIOperation`, `OpenAPIComponents`, `OpenAPIParameter`, etc. |

### AST Parser (split by responsibility)
| File | What it owns |
|---|---|
| `ast.go` | Entry points: `NewASTParser`, `DiscoverSourceFiles`, `ParseFile`, `EnhanceEndpoint`, all public interface methods (`GetAllStructs`, `GetAllHandlers`, `ClearCache`, etc.) |
| `ast_func.go` | Handler/function analysis: `extractHandlers`, `extractFuncReturnTypes`, `extractHelperFuncParams`, `extractParamsFromBody`, `extractQueryParameters`, `analyzeHelperFuncBody`, `isPocketBaseHandler`, `analyzePocketBaseHandler`, `analyzePocketBasePatterns`, `analyzePocketBaseCall`, `trackVariableAssignment`, `handleBindBody`, `handleJSONResponse`, all query/header/path detection helpers |
| `ast_struct.go` | Struct analysis and schema generation: `extractStructs`, `parseStruct`, `generateStructSchema`, `flattenEmbeddedFields`, `generateFieldSchema`, `generateSchemaForEndpoint`, `generateSchemaFromType`, `deepCopySchema` |
| `ast_metadata.go` | Value/type resolution: `analyzeMapLiteralSchema`, `parseMapLiteral`, `analyzeValueExpression`, `resolveTypeToSchema`, `schemaFromMakeArg`, `analyzeCompositeLitSchema`, `parseArrayLiteral`, `extractVariableDeclarations`, `extractLocalVariables`, `extractVarDecl`, `resolveTypeAlias`, `NewPocketBasePatterns` |
| `ast_file.go` | File-level utilities: `newFileSet`, `getModulePath`, `parseImportedPackages`, `parseDirectoryStructs` |

### Registry
| File | What it owns |
|---|---|
| `registry.go` | `APIRegistry` struct, constructor, `RegisterEndpoint`, helpers |
| `registry_routes.go` | `RegisterRoute`, `RegisterExplicitRoute`, `BatchRegisterRoutes`, `enhanceEndpointWithAST`, `createEndpointFromAnalysis` |
| `registry_spec.go` | `GetDocsWithComponents`, `buildPaths`, `endpointToOperation`, `extractPathParameters`, `collectRefsFromSchema` (pruning), `generateOperationId`, `buildTags` |

### Other
| File | What it owns |
|---|---|
| `schema.go` | `SchemaGenerator` — request/response schema inference, `GenerateComponentSchemas`, `promoteHandlerResponseSchemas` |
| `schema_conversion.go` | Go type → OpenAPI conversion: `ConvertGoTypeToOpenAPISchema`, `ConvertParamInfoToOpenAPIParameter`, validation tag parsing |
| `version_manager.go` | `APIVersionManager`, `VersionedAPIRouter`, `VersionedRouteChain`, `PrefixedRouter`, multi-version routing, per-version registries, `ServeSwaggerUI` (SwaggerDark CSS via `strings.NewReplacer`) |
| `discovery.go` | `RouteAnalyzer`, `MiddlewareAnalyzer`, `PathAnalyzer` — runtime route analysis utilities |
| `debug_dump.go` | `BuildDebugData()` — serves the `/api/docs/debug/ast` endpoint with full pipeline introspection |
| `utils.go` | String helpers: handler name extraction, camelCase/snake_case, description/tag generation, path conversion |

### Tests
| File | What it covers |
|---|---|
| `ast_test.go` | Core parser lifecycle, `ParseFile`, `EnhanceEndpoint`, `DiscoverSourceFiles`, benchmarks |
| `ast_struct_test.go` | Struct/schema/type extraction and JSON schema generation |
| `ast_func_test.go` | Handler scenarios (46+ handlers), func return type resolution, `funcBodySchemas`, append-based resolution, direct and indirect parameter detection |
| `ast_file_test.go` | Cross-package struct resolution via import following |
| `registry_test.go` | `APIRegistry` route registration and OpenAPI output |
| `schema_test.go` | `SchemaGenerator` and component schema generation |
| `version_manager_test.go` | `APIVersionManager` and versioned routing |
| `discovery_test.go` | Route/middleware/path analysis |
| `utils_test.go` | String helper utilities |

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
  |     |-- extractVariableDeclarations()  global var tracking
  |     |-- extractFuncReturnTypes()    scan non-handler functions for return type signatures
  |     |     \-- analyzeHelperFuncBody()  deep-analyze map[string]any helpers for key schemas
  |     |-- extractHelperFuncParams()   scan *core.RequestEvent helpers for params
  |     |     \-- extractParamsFromBody()  walks body detecting param access patterns
  |     |-- extractHandlers()           find func(c *core.RequestEvent) error
  |     |     |-- parseHandlerComments()   // API_DESC, // API_TAGS
  |     |     |-- analyzePocketBasePatterns()
  |     |     |     |-- BindBody / JSON / Decode detection
  |     |     |     |-- trackVariableAssignment()   vars + map["key"]=value additions + append() tracking
  |     |     |     \-- auth / database operation detection
  |     |     |-- extractLocalVariables()
  |     |     \-- extractQueryParameters()  direct + indirect (via funcParamSchemas) param detection
  |     \-- marks directory in parsedDirs
  |
  |-- Phase 2: Follow local imports (zero-config)
  |     |-- parseImportedPackages()     collect imports from all API_SOURCE files
  |     |     |-- getModulePath()       read go.mod for module name (cached on parser)
  |     |     |-- resolve imports       strip module prefix → local directory path
  |     |     |-- skip parsedDirs       avoid re-parsing API_SOURCE directories
  |     |     \-- parseDirectoryStructs()  extract structs only (no handlers)
  |     \-- re-run schema generation    imported structs may cross-reference each other
  v
APIRegistry.RegisterRoute(method, path, handler, middlewares)
  |-- RouteAnalyzer: handler name, auth, path params, tags
  |-- enhanceEndpointWithAST(): match handler name -> ASTHandlerInfo
  |     |-- copies description, tags, auth, request/response schemas
  |     |-- copies AST-detected Parameters (query/header/path) to endpoint
  |     priority: AST data > SchemaGenerator fallback
  \-- RegisterEndpoint() -> rebuild paths + tags
  v
endpointToOperation()
  |-- extract path params from URL pattern ({id} etc.)
  |-- append AST-detected params (query, header) from endpoint.Parameters (deduplicated)
  |-- promote inline response schemas to $ref (handlerResponseSchemaName)
  \-- build OpenAPIOperation with parameters, request body, responses, security
  v
GetDocsWithComponents()
  |-- embedded-first lookup by registry version
  |     |-- HasEmbeddedSpec(version)
  |     |-- GetEmbeddedSpec(version)
  |     \-- optional PB_EXT_OPENAPI_SPECS_DIR disk override
  |-- fallback: GenerateComponentSchemas()
  |     |-- struct schemas from AST
  |     |-- promoteHandlerResponseSchemas(): inline response → named component + $ref
  |     \-- PocketBaseRecord, Error always included
  |-- collect all $ref targets from paths (recursive)
  |-- prune component schemas to only referenced ones
  \-- return OpenAPI 3.0.3 spec
```

## Embedded OpenAPI Specs (Build-Time + Runtime)

`openapi_embedded_loader.go` is the runtime loader for build-generated specs in `core/server/api/specs`.

### Source selection policy

1. If `PB_EXT_OPENAPI_SPECS_DIR` is set, specs are discovered and read from that directory on disk.
2. Otherwise, specs are loaded from `go:embed` data bundled in the binary.

### Loader API

- `HasEmbeddedSpec(version string) bool`
- `ListEmbeddedSpecVersions() []string`
- `GetEmbeddedSpec(version string) (*APIDocs, error)`

### Behavior guarantees

- Parsed specs are cached per version.
- Parse/read errors are cached per version.
- Returned specs are deep-copied to avoid mutation leaks across requests.
- Runtime selection in `registry_spec.go` is embedded-first by version, then AST/runtime generation fallback.

### AST Parser Internals

### Two-Pass Struct Extraction

Pass 1 registers all structs with fields (no schemas) and type aliases. Pass 2 generates `JSONSchema` for each struct now that cross-references resolve. This is critical — changing it to single-pass will break nested struct `$ref` resolution.

### Handler Analysis

A function is a PocketBase handler if its signature is exactly `func(param *core.RequestEvent) error` — one `*core.RequestEvent` parameter and returns `error`. Analysis walks the body AST and tracks:

- **Variables**: `map[string]string` — variable name to inferred Go type
- **VariableExprs**: `map[string]ast.Expr` — variable name to RHS AST node (for deep map literal analysis)
- **MapAdditions**: `map[string][]MapKeyAdd` — dynamic `mapVar["key"] = value` assignments found after the initial literal

Request detection: `c.BindBody(&req)` or `json.NewDecoder(...).Decode(&req)` — type resolved from the variable's tracked type.

Response detection: `c.JSON(status, expr)` — the second argument is analyzed:
1. Try composite literal analysis (map/struct/slice)
2. If arg is a variable, trace to its stored expression
3. Merge any `MapAdditions` for that variable
4. Fall back to type inference → `$ref` for known structs
5. Last resort: generic object schema

### Function Return Type Resolution

`extractFuncReturnTypes()` runs before handler analysis. Scans all non-handler function declarations in API_SOURCE files and extracts the first non-error return type. Stored in `ASTParser.funcReturnTypes` as `map[string]string` (func name → Go type string).

Enables `inferTypeFromExpression` and `analyzeValueExpression` to resolve variables assigned from helper function calls (e.g., `candles := formatCandlesFull(records)` → type `[]map[string]any`).

### Helper Function Body Analysis (Deep Schema Resolution)

When `extractFuncReturnTypes()` finds a function returning `map[string]any` or `[]map[string]any`, it calls `analyzeHelperFuncBody()` to extract the map keys being set. Results go into `ASTParser.funcBodySchemas` as `map[string]*OpenAPISchema`.

**How it works:**
1. Creates a temporary `ASTHandlerInfo` to track variables and map additions within the function
2. Finds all `map[string]any{...}` composite literals, parses them via `parseMapLiteral`
3. Picks the literal with the most keys (the primary response shape)
4. Uses `findAssignedVariable()` to find the variable name and merges any dynamic `mapVar["key"] = value` additions
5. For `[]map[string]any` return types, wraps the item schema in an array schema

**Consumption**: In `analyzeValueExpression`, a `*ast.CallExpr` for a function with a `funcBodySchemas` entry returns the deep schema (via `deepCopySchema`) instead of the generic type-based schema.

**Limitation**: Only covers functions defined in API_SOURCE files, not imported packages.

### Indirect Parameter Extraction (Helper Functions)

`extractHelperFuncParams()` runs after `extractFuncReturnTypes` and before `extractHandlers`. It finds functions that accept `*core.RequestEvent` but are NOT handlers (return type ≠ `error`) and extracts the params they read into `ASTParser.funcParamSchemas`.

**Two categories of helpers:**

1. **Domain helpers** — read params with literal names:
   ```go
   func parseTimeParams(e *core.RequestEvent) timeParams {
       q := e.Request.URL.Query()
       return timeParams{Interval: q.Get("interval"), From: q.Get("from")}
   }
   ```
   → `funcParamSchemas["parseTimeParams"]` = `[{Name:"interval", Source:"query"}, ...]`

2. **Generic helpers** — param name is a function argument:
   ```go
   func parseIntParam(e *core.RequestEvent, name string, def int) int {
       return e.Request.URL.Query().Get(name)
   }
   ```
   → `funcParamSchemas["parseIntParam"]` = sentinel `[{Name:"", Source:"query"}]`

   Header-reading generics (`e.Request.Header.Get(name)`) get `Source:"header"` in the sentinel.

**Call-site merging in `extractQueryParameters`** (second pass after direct body scan):
- Domain helper call → merge all stored params directly
- Generic helper call → extract param name from 2nd string-literal arg, source from sentinel

### Append-Based Slice Item Resolution

When a handler builds a slice via `make([]map[string]any, ...)` and populates it with `append(varName, entry)`, the `make()` alone produces a generic array schema. The append-based resolution connects the appended item expression to the slice variable.

**How it works:**
1. `trackVariableAssignment()` detects `varName = append(varName, itemExpr)` and stores `itemExpr` in `SliceAppendExprs[varName]`
2. `enrichArraySchemaFromAppend()` is called after resolving a variable to a generic array schema — looks up the append expression and resolves it via `analyzeValueExpression()`

### Index Expression Resolution (map["key"] from funcBodySchemas)

When a helper reads values from another helper's return via index expressions (e.g., `summary["price"]`), the parser resolves the property type by looking up the source function's `funcBodySchemas` entry.

1. In `analyzeValueExpression`, `*ast.IndexExpr` handles `mapVar["key"]` patterns
2. Traces `mapVar` to its defining expression via `handlerInfo.VariableExprs`
3. If the defining expression is a function call, looks up the function name in `funcBodySchemas`
4. Returns the matching property's schema

### Parameter Detection (Query, Header, Path)

`extractQueryParameters()` runs in two passes:

**Pass 1 — direct body scan:**

| Pattern | Source |
|---|---|
| `q := e.Request.URL.Query(); q.Get("param")` | query |
| `e.Request.URL.Query().Get("param")` | query |
| `e.Request.URL.Query()["param"]` | query |
| `info.Query["param"]` (via `e.RequestInfo()`) | query |
| `e.Request.Header.Get("name")` | header |
| `info.Headers["name"]` (via `e.RequestInfo()`) | header |
| `e.Request.PathValue("id")` | path (Required: true) |

**Pass 2 — indirect helper scan** (via `funcParamSchemas`): merges params from known helper calls in the handler body.

Deduplication by name+source via `appendParamIfNew()`. Pipeline flow: `handlerInfo.Parameters` → `endpoint.Parameters` → `endpointToOperation` → `ConvertParamInfoToOpenAPIParameter`.

### Auto-Import Following (Cross-Package Struct Resolution)

After all API_SOURCE files are parsed, `parseImportedPackages()` automatically resolves local imports and parses their struct definitions. Zero-config — no directives needed on type files.

**Key fields on ASTParser:**
- `modulePath string` — from go.mod (e.g., `github.com/magooney-loon/pb-ext`)
- `parsedDirs map[string]bool` — tracks directories already parsed
- `funcBodySchemas map[string]*OpenAPISchema` — deep-analyzed schemas from helper bodies
- `funcParamSchemas map[string][]*ParamInfo` — params extracted from `*core.RequestEvent` helpers

**Edge cases:**
- No `go.mod` → feature silently disabled
- External imports → ignored
- API_SOURCE file's own directory → already in `parsedDirs`, skipped
- `_test.go` files → skipped in `parseDirectoryStructs`
- Circular imports → not an issue (structs only, no further import following)

### Value Expression Analysis

`analyzeValueExpression()` resolves map literal values to OpenAPI schemas: literals, variable references, struct field access, call expressions, nested composite literals. The `handlerInfo` parameter threads through the entire call chain.

### Type Resolution

`resolveTypeToSchema(typeName)` converts Go type strings to OpenAPI schemas: slices, maps, pointers, primitives, `time.Time`, `any`/`interface{}`, known structs (via `$ref`).

`generateSchemaForEndpoint(typeName)` always uses `$ref` for known structs. `generateSchemaFromType(typeName, inline)` controls inline vs. referenced.

### Response Schema Promotion

Handlers returning inline `map[string]any{...}` produce anonymous schemas. The promotion system lifts these into named component schemas.

**Two-part:**
1. **`promoteHandlerResponseSchemas()`** (schema.go) — during `GenerateComponentSchemas`, promotes inline handler schemas to `components/schemas` and replaces them with `$ref`s
2. **`endpointToOperation()`** (registry_spec.go) — uses the same deterministic name to replace inline responses with `$ref`s at operation-build time

**Naming:** `handlerResponseSchemaName()` strips package prefix and `Handler`/`Func`/`API`/`Endpoint` suffixes, uppercases first letter, appends `Response`:
- `getOrderHandler` → `GetOrderResponse`
- `pkg.listCategoriesHandler` → `ListCategoriesResponse`

**Qualifies:** object with ≥1 property, or array with non-nil items. Already-`$ref` schemas are skipped.

## Versioning System

Each API version gets its own isolated `ASTParser`, `SchemaGenerator`, and `APIRegistry`. `APIVersionManager` coordinates them.

`VersionedAPIRouter` wraps PocketBase's router. `VersionedRouteChain` handles `.Bind(middleware)` chaining. `PrefixedRouter` adds a path prefix to all registered routes.

Component schemas are pruned per-version — only schemas `$ref`'d from that version's paths are included. `Error` and `PocketBaseRecord` are always included.

`ServeSwaggerUI()` renders Swagger UI with dark mode CSS (SwaggerDark by Amoenus, MIT). Uses `strings.NewReplacer` instead of `fmt.Sprintf` so CSS `%` characters need no escaping.

## Source File Directives

| Directive | Where | Purpose |
|---|---|---|
| `// API_SOURCE` | Top of .go file | Marks file for AST parsing |
| `// API_DESC <text>` | Function doc comment | Handler description in OpenAPI |
| `// API_TAGS <csv>` | Function doc comment | Comma-separated endpoint tags |

## Debug Endpoint

`GET /api/docs/debug/ast` returns the full pipeline state: structs, handlers, per-version endpoints, component schemas, complete OpenAPI output. Requires auth.

## Common Pitfalls

- **Adding detection in `analyzePocketBaseCall`**: check how data flows into `handleJSONResponse` or `handleBindBody`. Test via debug endpoint.
- **Modifying `trackVariableAssignment`**: order matters — `VariableExprs` must be stored even when type inference fails.
- **Struct schema generation**: `generateStructSchema` handles embedded struct flattening. Pointer fields get `nullable: true` only when `Ref == ""`.
- **`$ref` vs inline**: endpoint-level use `$ref` via `generateSchemaForEndpoint`; struct fields use `$ref` via `generateSchemaFromType(type, inline=false)`; map literal values use inline from `analyzeValueExpression`.
- **Type aliases**: `resolveTypeAlias()` has cycle detection. Qualified types (`pkg.Type`) resolve to simple names.
- **Component pruning**: `collectRefsFromSchema` must cover every place `$ref` can appear.
- **Response schema promotion naming**: `handlerResponseSchemaName()` must produce identical names in both `promoteHandlerResponseSchemas()` (schema.go) and `endpointToOperation()` (registry_spec.go).
- **Parameter deduplication**: `appendParamIfNew` deduplicates by name+source. Query `id` and path `id` coexist. `endpointToOperation` deduplicates again using `in:name` keys.
- **Adding direct parameter patterns**: new access patterns go in `extractQueryParameters()` in `ast_func.go`.
- **Adding indirect parameter patterns**: helpers must take `*core.RequestEvent` and NOT return `error`. `extractHelperFuncParams` picks them up automatically.
- **`isPocketBaseHandler` check**: validates both parameter type (`*core.RequestEvent`) AND return type (`error`). Helpers like `parseTimeParams(e *core.RequestEvent) timeParams` are correctly excluded from handler analysis.
- **Ordering in `ParseFile`**: `extractFuncReturnTypes` → `extractHelperFuncParams` → `extractHandlers`. All three must run in this order.
- **`funcParamSchemas` sentinels**: generic helpers store `{Name:"", Source:"query"|"header"}`. Call-site extraction in `extractQueryParameters` uses the sentinel's source.
- **Import following scope**: only `extractStructs()` is called on imported directories — no handlers, func return types, or type aliases extracted.
- **Duplicate struct names**: if an imported package defines a struct with the same name as one in an API_SOURCE file, last-parsed wins.

## Test Coverage

```
go test ./core/server/api/... -v
go test ./core/server/api/... -run TestHandlerScenario     # handler scenarios
go test ./core/server/api/... -run TestIndirectParams      # indirect param extraction
go test ./core/server/api/... -run TestCrossPackage        # import following
```
