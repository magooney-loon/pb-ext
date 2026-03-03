OpenAPI specs directory sentinel

This file exists so the specs directory is always non-empty and embeddable.
Generated spec artifacts are written here as versioned JSON files, for example:
- v1.json
- v2.json

Generation command:
go run ./cmd/server --generate-specs-dir ./core/server/api/specs

Validation command:
go run ./cmd/server --validate-specs-dir ./core/server/api/specs

The pb-cli toolchain runs OpenAPI generation + validation automatically before server compilation. First install it globally:

go install github.com/magooney-loon/pb-ext/cmd/pb-cli@latest

Then use it in your project:

pb-cli              # Development mode
pb-cli --build-only # Build frontend only
pb-cli --production # Production build

For programmatic usage, see pkg/scripts/README.md.
