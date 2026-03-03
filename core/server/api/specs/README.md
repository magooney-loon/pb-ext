OpenAPI specs directory sentinel

This file exists so the specs directory is always non-empty and embeddable.
Generated spec artifacts are written here as versioned JSON files, for example:
- v1.json
- v2.json

Generation command:
go run ./cmd/server --generate-specs-dir ./core/server/api/specs

Validation command:
go run ./cmd/server --validate-specs-dir ./core/server/api/specs
