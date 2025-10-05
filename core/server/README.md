# Server Module

Enhanced PocketBase server wrapper with analytics, health monitoring, and comprehensive request tracking.

## Overview

This module extends PocketBase with production-ready features:
- **Request Analytics**: Track page views, visitors, device/browser statistics
- **Health Monitoring**: Real-time system metrics and server statistics dashboard
- **Error Handling**: Structured error types with HTTP status mapping
- **API Documentation**: Integration with automatic API documentation system
- **Static File Serving**: Enhanced static file serving with path resolution

## Architecture

```
Server
├── PocketBase Core        # Wrapped PocketBase instance
├── Analytics             # Visitor tracking & statistics
├── Health Monitor        # System metrics & dashboard
├── Error System         # Structured error handling
└── Template System      # Embedded UI templates
```

## Quick Start

### Basic Server Setup

```go
package main

import (
    "flag"
    "log"

    app "github.com/magooney-loon/pb-ext/core"
    "github.com/pocketbase/pocketbase/core"
)

func main() {
    devMode := flag.Bool("dev", false, "Run in developer mode")
    flag.Parse()

    // Create server with options
    var opts []app.Option
    if *devMode {
        opts = append(opts, app.InDeveloperMode())
    } else {
        opts = append(opts, app.InNormalMode())
    }

    srv := app.New(opts...)

    // Setup application components
    app.SetupLogging(srv)
    registerRoutes(srv.App())

    // Setup recovery middleware
    srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
        app.SetupRecovery(srv.App(), e)
        return e.Next()
    })

    // Start the server
    if err := srv.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### Configuration Options

The server supports flexible configuration through functional options:

```go
// Different ways to initialize the server
srv := app.New()                                    // Default config
srv := app.New(app.InDeveloperMode())              // Developer mode
srv := app.New(app.InNormalMode())                 // Production mode
srv := app.New(app.WithConfig(customConfig))       // Custom PocketBase config
srv := app.New(app.WithPocketbase(existingApp))    // Use existing PocketBase instance
```

## Core Features

### 1. Analytics System

Comprehensive visitor and usage analytics with zero configuration:

**Automatic Tracking**
- Page views and unique visitors
- Device type classification (desktop/mobile/tablet)
- Browser and OS detection
- Geographic location (country-level)
- UTM campaign parameters
- Session management and return visitor detection

**Performance Optimized**
- Background buffering and batching
- Configurable flush intervals
- Minimal performance impact on requests

**Dashboard Integration**
- Real-time visitor statistics
- Device and browser breakdowns
- Popular pages tracking
- Recent visitor activity

### 2. Health Monitoring

Real-time system and application health monitoring:

**System Metrics**
- CPU usage and load averages
- Memory consumption and availability
- Disk space utilization
- Network connection counts
- Temperature sensors (when available)

**Application Stats**
- Request counts and error rates
- Average response times
- Active connections
- Uptime tracking

**Secure Dashboard**
- Accessible at `/_/_` endpoint
- Requires superuser authentication
- Live updating metrics
- Responsive design for mobile monitoring

### 3. Error Handling

Structured error handling with automatic HTTP status mapping:

```go
// Errors are automatically categorized and logged
// Types include: http, routing, auth, template, config, database, internal
// Proper HTTP status codes are automatically assigned
```

### 4. Request Recovery

Built-in panic recovery with detailed logging:

```go
srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
    app.SetupRecovery(srv.App(), e)
    return e.Next()
})
```

## Usage Patterns

### Full Application Setup

```go
func initApp(devMode bool) {
    // Configure server options
    var opts []app.Option
    if devMode {
        opts = append(opts, app.InDeveloperMode())
    } else {
        opts = append(opts, app.InNormalMode())
    }

    // Create and configure server
    srv := app.New(opts...)
    app.SetupLogging(srv)

    // Register application components
    registerCollections(srv.App())  // Your database models
    registerRoutes(srv.App())       // Your API routes
    registerJobs(srv.App())         // Your background jobs

    // Setup middleware and start
    srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
        app.SetupRecovery(srv.App(), e)
        return e.Next()
    })

    if err := srv.Start(); err != nil {
        srv.App().Logger().Error("Fatal application error",
            "error", err,
            "uptime", srv.Stats().StartTime,
            "total_requests", srv.Stats().TotalRequests.Load(),
            "active_connections", srv.Stats().ActiveConnections.Load(),
        )
        log.Fatal(err)
    }
}
```

### Accessing Server Statistics

```go
// Get real-time server statistics
stats := srv.Stats()
fmt.Printf("Total requests: %d\n", stats.TotalRequests.Load())
fmt.Printf("Active connections: %d\n", stats.ActiveConnections.Load())
fmt.Printf("Uptime: %v\n", time.Since(stats.StartTime))
```

### Custom PocketBase Integration

```go
// Use with existing PocketBase instance
existingApp := pocketbase.New()
// ... configure your PocketBase app
srv := app.New(app.WithPocketbase(existingApp))
```

## Project Structure

The server module works best with a structured project layout:

```
cmd/server/
├── main.go           # Application entry point
├── collections.go    # Database model definitions
├── routes.go         # API route registration
├── handlers.go       # Request handlers
└── jobs.go          # Background job definitions
```

## Monitoring Endpoints

### Health Dashboard
- **URL**: `/_/_`
- **Auth**: Superuser required
- **Features**: Real-time metrics, system stats, visitor analytics, API docs

### API Documentation (if using versioned API system)
- **OpenAPI Docs**: `/api/v1/docs/openapi`
- **API Statistics**: `/api/v1/docs/stats`
- **Version Management**: `/api/versions`

## Analytics Dashboard Features

**Visitor Insights**
- Unique vs returning visitors
- Real-time visitor counts
- Session duration tracking
- Geographic distribution

**Device Analytics**
- Desktop/mobile/tablet breakdown
- Browser market share
- Operating system distribution
- Screen resolution patterns

**Performance Tracking**
- Popular pages and endpoints
- Request duration metrics
- Error rate monitoring
- Peak usage patterns

## Production Deployment

The server module is designed for production use with:

- **Zero Configuration**: Works with sensible defaults
- **Performance Optimized**: Background processing and efficient batching
- **Thread Safe**: Atomic counters and proper synchronization
- **Comprehensive Logging**: Structured logs with request context
- **Error Recovery**: Automatic panic recovery with detailed reporting
- **Health Monitoring**: Built-in system and application monitoring

For production deployments, consider using:
- [pb-deployer](https://github.com/magooney-loon/pb-deployer) for streamlined deployment

## Development Workflow

**Developer Mode Benefits:**
- Enhanced error messages
- Debug logging enabled
- Hot reload support
- Development-specific middleware

**Production Mode:**
- Optimized performance
- Minimal logging
- Security-focused defaults
- Production middleware stack
