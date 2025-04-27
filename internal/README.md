# Internal Packages

Core implementation modules not intended for external usage.

## Directory Structure

- **[server/](./server/README.md)**: Extended PocketBase server implementation
- **[monitoring/](./monitoring/README.md)**: System metrics collection and reporting
- **[logging/](./logging/README.md)**: Structured logging and error handling

## Design Philosophy

These packages follow internal design principles:

- Encapsulated implementation details
- Clear separation of concerns
- Consistent error handling patterns
- Comprehensive monitoring and logging
- Performance-optimized core functionality

## Usage

These packages are initialized and used by the application entry point in `cmd/server/main.go` and not intended to be imported by external applications. 