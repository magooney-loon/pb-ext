## [COMMAND SEQUENCES]

### > Standard Development Mode
```
$ go run cmd/scripts/main.go
```
*Builds frontend + starts development server*

### > Full System Installation
```
$ go run cmd/scripts/main.go --install
```
*Downloads dependencies + builds + runs in dev mode*

### > Frontend Compilation Only
```
$ go run cmd/scripts/main.go --build-only
```
*Compiles frontend assets without server daemon*

### > Development Server Only
```
$ go run cmd/scripts/main.go --run-only
```
*Starts server daemon, skips build sequence*

### > Production Deployment Build
```
$ go run cmd/scripts/main.go --production
```
*Creates optimized production binary + assets*

### > Test Suite Execution
```
$ go run cmd/scripts/main.go --test-only
```
*Runs comprehensive test suite with coverage reports*

### > Custom Output Directory
```
$ go run cmd/scripts/main.go --production --dist release
```
*Production build with custom target directory*

### > System Help Terminal
```
$ go run cmd/scripts/main.go --help
```
*Displays all available command flags and options*

## [DEPLOYMENT INTEGRATION]

### Automated VPS Deployment via pb-deployer:
```
$ git clone https://github.com/magooney-loon/pb-deployer
$ cd pb-deployer && go run cmd/scripts/main.go --install
```

### pb-deployer Features:
    [✓] Automated server provisioning + security hardening
    [✓] Zero-downtime deployment cycles with rollback
    [✓] Production systemd service management
    [✓] Full PocketBase v0.20+ compatibility

## [SYSTEM REQUIREMENTS]

    [REQUIRED]
    ├── Go 1.19+        (backend compilation)
    ├── Node.js 16+     (frontend build system)
    ├── npm 8+          (dependency management)
    └── Git             (version control)

    [OPTIONAL]
    └── pb-deployer     (production deployment automation)

## [BUILD PROCESS]

    [DEVELOPMENT MODE]
    1. System validation    → Check Go/Node/npm availability
    2. Dependency install   → npm install + go mod tidy
    3. Frontend build       → npm run build
    4. Asset deployment     → Copy to pb_public/
    5. Server startup       → go run ./cmd/server --dev serve

    [PRODUCTION MODE]
    1. Environment prep     → Clean dist/ directory
    2. Dependency install   → Full dependency resolution
    3. Frontend build       → Optimized production build
    4. Server compilation   → go build -ldflags="-s -w"
    5. Asset packaging      → Create deployment archive
    6. Metadata generation  → Build info + package metadata

    [TEST MODE]
    1. System validation    → Verify test environment
    2. Test execution       → Run all test suites
    3. Coverage analysis    → Generate coverage reports
    4. Report generation    → HTML/JSON/TXT outputs

## [TROUBLESHOOTING]

    [ERROR: Command not found]
    → Ensure Go/Node/npm are installed and in system PATH

    [ERROR: Frontend build failed]
    → Check package.json and run 'npm install' manually
    → Verify frontend/ directory exists with valid source

    [ERROR: Server compilation failed]
    → Run 'go mod tidy' to resolve dependencies
    → Check cmd/server/main.go exists

    [ERROR: Permission denied]
    → Ensure write permissions for pb_public/ and dist/
