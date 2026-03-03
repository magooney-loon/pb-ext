# pb-cli Build Toolchain

The pb-cli toolchain automates frontend builds, server compilation, testing, and production deployments for pb-ext projects.

## 📋 Command Reference

| Command | Description |
|---------|-------------|
| `pb-cli` | Standard development mode (build + serve) |
| `pb-cli --install` | Install all dependencies |
| `pb-cli --build-only` | Build frontend only |
| `pb-cli --run-only` | Start server only |
| `pb-cli --production` | Create production build |
| `pb-cli --production --dist <dir>` | Production build with custom output |
| `pb-cli --test-only` | Run test suite |
| `pb-cli --help` | Show help message |

## 🔧 System Requirements

- **Go**: 1.19 or higher
- **Node.js**: 16 or higher
- **npm**: 8 or higher
- **Git**: For version control

## 📦 Installation

### Option 1: Global Installation (Recommended)

Install pb-cli as a global binary available anywhere on your system:

```bash
go install github.com/magooney-loon/pb-ext/cmd/pb-cli@latest
```

Verify installation:

```bash
pb-cli --help
```

**Note**: Ensure `$GOPATH/bin` or `$HOME/go/bin` is in your `PATH`.

### Option 2: Use Without Installation

Run directly from the pb-ext repository without installing:

```bash
go run cmd/pb-cli/main.go [command]
```

### Option 3: Import as a Package

Use programmatically in your Go applications:

```go
import "github.com/magooney-loon/pb-ext/pkg/scripts"

scripts.RunCLI()
```

## 🚀 Usage

### Development Mode

Builds frontend + starts development server.

**Global CLI:**
```bash
pb-cli
```

**Local:**
```bash
go run cmd/pb-cli/main.go
```

**Programmatic:**
```go
import "github.com/magooney-loon/pb-ext/pkg/scripts"
import "os"

os.Args = []string{"pb-cli"}
scripts.RunCLI()
```

---

### Install Dependencies

Downloads and installs all project dependencies (Go modules + npm packages).

**Global CLI:**
```bash
pb-cli --install
```

**Local:**
```bash
go run cmd/pb-cli/main.go --install
```

**Programmatic:**
```go
import "github.com/magooney-loon/pb-ext/pkg/scripts"
import "os"

os.Args = []string{"pb-cli", "--install"}
scripts.RunCLI()
```

---

### Build Frontend Only

Compiles frontend assets without starting the server.

**Global CLI:**
```bash
pb-cli --build-only
```

**Local:**
```bash
go run cmd/pb-cli/main.go --build-only
```

**Programmatic:**
```go
import "github.com/magooney-loon/pb-ext/pkg/scripts"
import "os"

os.Args = []string{"pb-cli", "--build-only"}
scripts.RunCLI()
```

---

### Start Server Only

Starts the development server without rebuilding the frontend.

**Global CLI:**
```bash
pb-cli --run-only
```

**Local:**
```bash
go run cmd/pb-cli/main.go --run-only
```

**Programmatic:**
```go
import "github.com/magooney-loon/pb-ext/pkg/scripts"
import "os"

os.Args = []string{"pb-cli", "--run-only"}
scripts.RunCLI()
```

---

### Production Build

Creates optimized production binary and assets in the `dist/` directory.

**Global CLI:**
```bash
pb-cli --production
```

**Local:**
```bash
go run cmd/pb-cli/main.go --production
```

**Custom Output Directory:**
```bash
pb-cli --production --dist release
```

**Programmatic:**
```go
import "github.com/magooney-loon/pb-ext/pkg/scripts"
import "os"

os.Args = []string{"pb-cli", "--production"}
scripts.RunCLI()
```

---

### Run Tests

Executes the test suite with coverage reports.

**Global CLI:**
```bash
pb-cli --test-only
```

**Local:**
```bash
go run cmd/pb-cli/main.go --test-only
```

**Programmatic:**
```go
import "github.com/magooney-loon/pb-ext/pkg/scripts"
import "os"

os.Args = []string{"pb-cli", "--test-only"}
scripts.RunCLI()
```

---

### Help

Displays all available commands and options.

**Global CLI:**
```bash
pb-cli --help
```

**Local:**
```bash
go run cmd/pb-cli/main.go --help
```

## 🏗️ Build Process

### Development Mode

1. System validation → Check Go/Node/npm availability
2. Dependency install → `npm install` + `go mod tidy`
3. Frontend build → `npm run build`
4. Asset deployment → Copy to `pb_public/`
5. Server startup → `go run ./cmd/server --dev serve`

### Production Mode

1. Environment prep → Clean `dist/` directory
2. Dependency install → Full dependency resolution
3. Frontend build → Optimized production build
4. Server compilation → `go build -ldflags="-s -w"`
5. Asset packaging → Create deployment archive
6. Metadata generation → Build info + package metadata

### Test Mode

1. System validation → Verify test environment
2. Test execution → Run all test suites
3. Coverage analysis → Generate coverage reports
4. Report generation → HTML/JSON/TXT outputs

## 🐛 Troubleshooting

### "Command not found"

**Solution**: Ensure Go, Node.js, and npm are installed and in your system `PATH`.

If using global installation, verify `$GOPATH/bin` or `$HOME/go/bin` is in your `PATH`:

```bash
# Add to your ~/.bashrc or ~/.zshrc
export PATH=$PATH:$(go env GOPATH)/bin
# or
export PATH=$PATH:$HOME/go/bin
```

### "Frontend build failed"

**Solution**: 
- Check that `package.json` exists in the `frontend/` directory
- Run `npm install` manually to check for errors
- Verify Node.js version: `node --version`

### "Server compilation failed"

**Solution**:
- Run `go mod tidy` to resolve dependencies
- Verify `cmd/server/main.go` exists
- Check Go version: `go version`

### "Permission denied"

**Solution**:
- Ensure write permissions for `pb_public/` and `dist/` directories
- Run with appropriate permissions or change directory ownership

## 💻 Programmatic Usage Guide

For advanced use cases, you can import the scripts package and use it programmatically in your Go applications.

### Basic Usage

```go
package main

import "github.com/magooney-loon/pb-ext/pkg/scripts"

func main() {
    scripts.RunCLI()
}
```

### Custom Workflow

```go
package main

import (
    "flag"
    "github.com/magooney-loon/pb-ext/pkg/scripts/internal"
)

func main() {
    // Parse your custom flags
    customFlag := flag.Bool("custom", false, "Custom flag")
    flag.Parse()
    
    if *customFlag {
        // Run custom build logic
        if err := internal.BuildFrontend(".", true); err != nil {
            panic(err)
        }
    } else {
        // Use default CLI
        scripts.RunCLI()
    }
}
```

### Integration with Your Build System

```go
package main

import (
    "fmt"
    "github.com/magooney-loon/pb-ext/pkg/scripts/internal"
)

func main() {
    // Check system requirements
    if err := internal.CheckSystemRequirements(); err != nil {
        fmt.Printf("System check failed: %v\n", err)
        return
    }
    
    // Install dependencies
    if err := internal.BuildFrontend(".", true); err != nil {
        fmt.Printf("Build failed: %v\n", err)
        return
    }
    
    // Generate OpenAPI specs
    if err := internal.GenerateOpenAPISpecs("."); err != nil {
        fmt.Printf("Spec generation failed: %v\n", err)
        return
    }
    
    fmt.Println("Build completed successfully!")
}
```

## 📝 Notes

- The tool automatically detects your project structure
- All build artifacts are placed in appropriate directories
- Production builds are optimized for size and performance
- Test coverage reports are generated in multiple formats (HTML, JSON, TXT)

## 📄 License

MIT License - see LICENSE file for details
