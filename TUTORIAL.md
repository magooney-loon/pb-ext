# Beginner's Guide to pb-ext

This guide will help you get started with pb-ext even if you're new to Golang.

## What is pb-ext?

pb-ext is a wrapper around PocketBase that adds enhanced monitoring, logging, analytics and OpenAPI/Swagger features. Since it builds on top of PocketBase, you can continue using all PocketBase features and documentation for extending it as a Go framework.

## Prerequisites

1. Install Golang:
   - Download from [golang.org/dl](https://go.dev/dl/)
   - Follow the installation instructions for your OS
   - Verify installation with `go version`

## Quick Start with Examples

If you want to jump right in with working examples, you can clone the example server from the pb-ext repository:

```bash
# Clone the cmd/server directory with all example files
mkdir -p cmd/server
curl -s https://raw.githubusercontent.com/magooney-loon/pb-ext/main/cmd/server/main.go -o cmd/server/main.go
curl -s https://raw.githubusercontent.com/magooney-loon/pb-ext/main/cmd/server/routes.go -o cmd/server/routes.go
curl -s https://raw.githubusercontent.com/magooney-loon/pb-ext/main/cmd/server/handlers.go -o cmd/server/handlers.go
curl -s https://raw.githubusercontent.com/magooney-loon/pb-ext/main/cmd/server/jobs.go -o cmd/server/jobs.go
curl -s https://raw.githubusercontent.com/magooney-loon/pb-ext/main/cmd/server/collections.go -o cmd/server/collections.go

# Or clone the entire pb-ext repo and use cmd/server directly
git clone https://github.com/magooney-loon/pb-ext.git
cd pb-ext

# Initialize module
go mod init my-pb-project

# Download dependencies
go mod tidy

# Install the build toolchain
go install github.com/magooney-loon/pb-ext/cmd/pb-cli@latest

# Run in development mode
pb-cli --run-only
```

This gives you a complete working setup with:
- ✅ Example routes and handlers
- ✅ Cron job examples
- ✅ Collection definitions
- ✅ OpenAPI documentation setup

## Manual Setup (From Scratch)

If you prefer to build everything yourself, follow these steps:

### 1. Create a project folder

```bash
mkdir my-pb-project
cd my-pb-project
```

### 2. Initialize Go module

```bash
go mod init my-pb-project
```

### 3. Create main.go file

Create a file named `main.go` in `cmd/server` and copy the following code:

```go
package main

import (
	"log"

	app "github.com/magooney-loon/pb-ext/core"
	"github.com/pocketbase/pocketbase/core"
)

func main() {
	// Initialize server in development mode
	srv := app.New(app.InDeveloperMode())

	// Setup logging
	app.SetupLogging(srv)

	// Register your routes, collections, and jobs
	registerRoutes(srv.App())
	registerCollections(srv.App())

	// Setup error recovery
	srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
		app.SetupRecovery(srv.App(), e)
		return e.Next()
	})

	// Start the server
	if err := srv.Start(); err != nil {
		srv.App().Logger().Error("Fatal application error", "error", err)
		log.Fatal(err)
	}
}

// Example route registration
func registerRoutes(app core.App) {
	// Add your routes here
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Example: Add a simple time endpoint
		e.Router.GET("/api/time", func(c *core.RequestEvent) error {
			return c.JSON(200, map[string]string{
				"time": c.Request.Context().Value("time").(string),
			})
		})
		return e.Next()
	})
}

// Example collection registration
func registerCollections(app core.App) {
	// Add your collections here
}
```

This creates a basic server with:
- ✅ Development mode enabled
- ✅ Structured logging
- ✅ Error recovery
- ✅ Example API endpoint at `/api/time`

### 4. Download dependencies

```bash
go mod tidy
```

This will download pb-ext and all required dependencies.

### 5. Install the build toolchain

```bash
go install github.com/magooney-loon/pb-ext/cmd/pb-cli@latest
```

### 6. Run your server

```bash
pb-cli --run-only
```

Your server should now be running!

### 6. Add Static Files (Website)

PocketBase automatically serves static files from the `pb_public` folder. Create this folder and add an `index.html` file:

```bash
mkdir pb_public
```

Create a file `pb_public/index.html` with basic content:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>My PocketBase App</title>
    <style>
        body {
            font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1 {
            color: #333;
        }
        .card {
            border: 1px solid #ddd;
            border-radius: 8px;
            padding: 20px;
            margin-top: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
    </style>
</head>
<body>
    <h1>Welcome to my PocketBase App!</h1>
    <div class="card">
        <h2>Getting Started</h2>
        <p>Your static website is now live! You can modify the files in the <code>pb_public</code> folder to build your frontend.</p>
        <p>The example time API endpoint is available at: <a href="/api/time">/api/time</a></p>
        <p>Access the pb-ext dashboard at: <a href="/_/_">/_/_</a></p>
    </div>
</body>
</html>
```

For more details on available field types and collection options, refer to the [PocketBase documentation](https://pocketbase.io/docs/collections/).

For more information on using PocketBase as a Go framework, refer to the [PocketBase documentation](https://pocketbase.io/docs/go-overview/).

## Access your application

- PocketBase Admin panel: `http://127.0.0.1:8090/_`
- pb-ext Dashboard: `http://127.0.0.1:8090/_/_`
- Your website: `http://127.0.0.1:8090/`

## Frontend Development

### Building with SvelteKit (Recommended)

pb-ext works great with SvelteKit for building modern web applications. Create your frontend in the `frontend/` directory:

```bash
# Create a new SvelteKit app
npx sv create frontend

# When prompted:
# - Choose "Skeleton project" or your preferred template
# - Select "TypeScript" (recommended)
# - Choose "static-adapter" for PocketBase compatibility
# - Enable ESLint, Prettier, and Playwright as needed
```

Configure your SvelteKit app by creating a `frontend/src/routes/+layout.ts` file:

```typescript
export const prerender = true;
export const trailingSlash = 'always';
```

### Building Your Frontend

Use the **pb-cli** toolchain to build your frontend and start the development server:

```bash
# Development mode (builds frontend + starts server)
pb-cli

# Build frontend only
pb-cli --build-only

# Start server only (skip frontend build)
pb-cli --run-only

# Production build
pb-cli --production
```

The pb-cli toolchain automatically:
- Installs npm dependencies (`npm install`)
- Runs the SvelteKit build (`npm run build`)
- Copies built assets to `pb_public/`
- Starts the PocketBase server
- Generates OpenAPI documentation

### Prebuilt Starter Template

If you want a ready-to-use SvelteKit frontend with pb-ext integration, check out the **svelte-gui** template:

```
https://github.com/magooney-loon/svelte-gui
```

This template includes:
- ✅ Complete SvelteKit setup
- ✅ Authentication UI
- ✅ Dashboard layouts
- ✅ API integration examples
- ✅ Dark mode support
- ✅ Responsive design

To use it:

```bash
# Clone the template
git clone https://github.com/magooney-loon/svelte-gui.git
cd svelte-gui

# Follow the setup instructions in the README
```

## Production Deployment

### Automated VPS Deployment with pb-deployer

For production deployments to VPS servers, use **pb-deployer** for automated provisioning and deployment:

```
https://github.com/magooney-loon/pb-deployer
```

pb-deployer provides:
- ✅ Automated server provisioning
- ✅ Security hardening (SSL, firewall, etc.)
- ✅ Zero-downtime deployments
- ✅ Rollback capabilities
- ✅ Systemd service management
- ✅ Auto-updates
- ✅ Backup management

To use pb-deployer:

```bash
# Clone pb-deployer
git clone https://github.com/magooney-loon/pb-deployer.git
cd pb-deployer

# Install dependencies
go mod tidy

# Run the deployment wizard
go run cmd/scripts/main.go --install
```

Follow the interactive prompts to configure your server, and pb-deployer will handle the rest!

### Manual Production Build

If you prefer manual deployment, use pb-cli to create a production build:

```bash
# Create production build in dist/ directory
pb-cli --production

# Or specify custom output directory
pb-cli --production --dist release
```

This creates:
- Optimized server binary with `-ldflags="-s -w"`
- Compiled frontend assets
- OpenAPI specification files
- Deployment archive ready for upload

Then upload the `dist/` directory to your server and run the binary.

## Next Steps

Now that you have pb-ext running, explore these features:

- **OpenAPI Documentation**: Visit `/_/_` to view your API docs
- **Cron Jobs**: Schedule background tasks in `cmd/server/jobs.go`
- **Analytics**: Track visitors automatically (GDPR-compliant)
- **System Monitoring**: View real-time metrics in the dashboard
- **Structured Logging**: Check logs with trace IDs for debugging

For more advanced usage and examples, refer to:
- **Main README**: Complete feature documentation
- **pkg/scripts/README.md**: Build toolchain documentation
- **CLAUDE.md**: Architecture and internals guide
