# OpenAPI Build-Time Embedding (`go:embed` + JSON) — Implementation Plan (Revised for Reverted Baseline)

## Context (Current Baseline)

This plan is intentionally aligned to your **reverted runtime-only state**:

- `cmd/server/main.go` has no `--generate-spec` flag.
- `cmd/server/routes.go` has no spec generation helper.
- `core/server/api/registry_spec.go` currently always generates docs at runtime from registered routes + AST/schema pipeline.
- Production build (`cmd/scripts/internal/production.go`) does **not** generate OpenAPI artifacts before `go build`.
- Legacy `core/server/api/openapi_embedded.go` may still exist in repo but is not integrated in the runtime path.

So this plan starts from “runtime generation only” and adds clean build-time embedding in a controlled way.

---

## Goal

Introduce a reliable production architecture where OpenAPI specs are:

1. generated at build time as JSON,
2. embedded into the binary via `go:embed`,
3. served at runtime without requiring source files.

At the same time, preserve runtime generation as a fallback for dev/debug.

---

## Non-Goals

- No rewrite of AST parser internals.
- No change to route registration style (`VersionedAPIRouter`, etc.).
- No API contract changes for `/api/docs/{version}/spec` consumers.

---

## Target Architecture

## 1) Artifact format

Generate one file per version:

- `core/server/api/specs/v1.json`
- `core/server/api/specs/v2.json`
- (optional) `core/server/api/specs/index.json`

No more giant generated Go string-map as the source of truth.

## 2) Embedding

Add a loader in `core/server/api` that reads embedded JSON specs from the shared server embed filesystem:

- specs are embedded directly in `core/server/api/openapi_embedded_loader.go` via `go:embed`,
- resolves by version (`specs/<version>.json`),
- parses JSON into `APIDocs`,
- caches parsed values safely.

## 3) Runtime policy

When serving docs:

1. If embedded spec exists for requested version, return it.
2. Else fallback to runtime generation (`GetDocsWithComponents` existing behavior).
3. Optional strict mode (env var) can require embedded specs in production.

---

## Why this matches your reverted state

Because currently there is no active generation path from scripts to API package, this plan explicitly reintroduces generation as a **first-class build step** instead of relying on stale files like `openapi_embedded.go`.

---

## Implementation Phases

## Phase 1 — Build-time spec generation command

### Files to modify

- `cmd/server/main.go`
- `cmd/server/routes.go` (or new helper file under `cmd/server/` to avoid bloating routes)
- optionally `cmd/server/specgen.go` (recommended for separation)

### Actions

1. Re-add CLI flags in `main.go`:
   - `--generate-specs-dir <dir>`
   - optional: `--generate-spec-version <v>` (nice-to-have)
   - optional: `--validate-specs-dir <dir>`

2. Add a generation function that:
   - initializes version manager with current `createAPIVersions()`,
   - registers all versioned routes using the same registration path used in normal startup,
   - builds `APIDocs` per version,
   - writes JSON files to the target directory.

3. Ensure generated JSON formatting is deterministic:
   - `json.MarshalIndent(..., "", "\t")`
   - stable version iteration order (`sort.Strings(versions)`).

### Notes

- Keep generation logic outside `registerRoutes` if possible (avoid side effects / cleaner testability).
- Do not rely on runtime server startup for generation.

---

## Phase 2 — Embedded loader (new source of truth)

### New file

- `core/server/api/openapi_embedded_loader.go`

### Actions

1. Embed specs directory directly in the loader:
   - `//go:embed specs`
2. Expose:
   - `HasEmbeddedSpec(version string) bool`
   - `GetEmbeddedSpec(version string) (*APIDocs, error)`
   - `ListEmbeddedSpecVersions() []string`
3. Add thread-safe caching.
4. Return deep copies if runtime mutates docs fields (e.g. server URLs override).
5. Support optional disk override for development/debug with:
   - `PB_EXT_OPENAPI_SPECS_DIR=/absolute/path/to/specs`

### Important

This loader replaces the need for a generated `embeddedSpecsJSON` Go map.

---

## Phase 3 — Runtime integration with fallback

### File to modify

- `core/server/api/registry_spec.go`

### Actions

1. At start of `GetDocsWithComponents()`:
   - check embedded by registry version,
   - if found, return embedded docs (with server override behavior preserved if desired).
2. fallback to existing runtime generation path when not found.

### Additional fix (required)

- Ensure `APIRegistry.version` is populated when registry is created per version.

### File to modify

- `core/server/api/version_manager.go` (`RegisterVersion`)

### Action

After `registry := NewAPIRegistry(...)`, set version on registry (via field assignment or setter).

Without this, embedded lookup by version cannot work reliably.

---

## Phase 4 — Build pipeline integration (mandatory for production)

### Files to modify

- `cmd/scripts/internal/build.go`
- `cmd/scripts/internal/production.go`

### Actions

1. Add `GenerateOpenAPISpecs(rootDir string)` in build utilities:
   - runs `go run ./cmd/server --generate-specs-dir ./core/server/api/specs`
2. In `ProductionBuild(...)`:
   - call generation **before** `BuildServerBinary(...)`,
   - fail hard on generation error (no warning-only behavior).
3. Optional: add explicit `ValidateOpenAPISpecs(...)` step.

### Why

Prevents shipping binaries with missing specs and broken docs endpoint behavior.

---

## Phase 5 — Validation rules

Validation should fail build if any rule fails.

For each generated version file:

1. file exists at `specs/<version>.json`,
2. valid JSON parse,
3. required fields present:
   - `openapi`,
   - `info.title`,
   - `info.version`,
   - `paths` (allow empty only if explicitly intended),
4. optional stronger checks:
   - unresolved `$ref` detection,
   - duplicate operationId detection.

---

## Phase 6 — Legacy cleanup

### Files to remove/retire

- `core/server/api/openapi_embedded.go` (legacy generated map file)

### Actions

1. Stop regenerating it.
2. Remove all references to `embeddedSpecsJSON` map approach.
3. Update comments/docs/commands that mention the old file.

---

## Suggested command UX

- Generate specs:
  - `go run ./cmd/server --generate-specs-dir ./core/server/api/specs`
- Optional validate:
  - `go run ./cmd/server --validate-specs-dir ./core/server/api/specs`
- Production build:
  - unchanged user command (`--production`) but internally runs generation + validation before binary compile.

---

## Test Plan

## Unit tests

1. Loader tests:
   - existing/missing version detection,
   - parse failures,
   - caching behavior,
   - copy safety if mutation occurs.
2. Runtime selection tests:
   - embedded-first,
   - runtime fallback when embedded missing.
3. Version wiring test:
   - registry version is set and usable for embedded lookup.

## Integration tests

1. Generation command produces all configured versions.
2. Production build fails if generation fails.
3. Docs endpoints return expected version specs from embedded data in production-like run.

---

## Rollout Strategy

1. Implement generation + loader + runtime integration.
2. Keep runtime fallback enabled initially.
3. Add strict mode env var later if you want hard enforcement in production runtime.
4. Remove legacy generated-Go embedding file after stabilization.

---

## Risks and Mitigations

1. **Risk:** stale JSON specs committed.
   - **Mitigation:** always generate in production build pipeline and fail on mismatch.
2. **Risk:** embedded doc object mutation leaks across requests.
   - **Mitigation:** deep copy returned docs.
3. **Risk:** version mismatch between routes/config/spec files.
   - **Mitigation:** compare `createAPIVersions()` keys with generated files in validation step.

---

## Acceptance Criteria

Done means:

1. Production binary serves docs with no source files required.
2. Specs are embedded from `specs/*.json` via `go:embed`.
3. `registry_spec.go` uses embedded-first by version, then runtime fallback.
4. Production build fails when generation/validation fails.
5. Legacy `openapi_embedded.go` path is removed from active architecture.
6. Tests cover generation, loading, selection, and production gating.

---

## Implementation Checklist (Execution Order)

- [x] Re-add generation CLI flags and generator entrypoint in `cmd/server`.
- [x] Generate versioned JSON files to `core/server/api/specs`.
- [x] Add `openapi_embedded_loader.go` loader (direct `go:embed` of `core/server/api/specs` with optional disk override via `PB_EXT_OPENAPI_SPECS_DIR`).
- [x] Wire embedded-first + fallback into `registry_spec.go`.
- [x] Set registry version in `version_manager.go`.
- [x] Add build script generation step in production flow.
- [x] Make generation failure fatal in production build.
- [x] Add validation step.
- [ ] Retire/remove legacy `openapi_embedded.go`.
- [ ] Update docs (`README`, `CLAUDE.md`, `AGENTS.md`).

---

## Practical Note for Your Next Step

Given your reverted baseline, the implementation sequence was completed as:

1. **generator command** ✅,  
2. **embedded loader** ✅ (implemented directly in `openapi_embedded_loader.go` via `go:embed`),  
3. **runtime integration** ✅,  
4. **production build gating** ✅,  
5. **legacy cleanup + tests** ⏳ (legacy-file retirement + remaining docs follow-up still pending).

This minimized regressions and kept each step verifiable.