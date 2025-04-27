# Server Module

Core server implementation extending PocketBase functionality.

## Components

- **server.go**: Main server initialization and configuration
- **health.go**: Health check endpoints and system diagnostics
- **errors.go**: Standardized error handling patterns
- **templates/**: HTML templates for server-specific pages

## Usage

The server module is initialized in `cmd/server/main.go` and provides:
- Extended PocketBase functionality
- Custom routes and middleware
- Integration with monitoring and logging systems
- Health check endpoints 