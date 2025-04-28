# Server Command

Entry point for the enhanced PocketBase server application.

## Overview

This directory contains the main application entry point that:
- Initializes the server
- Configures logging and monitoring
- Registers API routes
- Starts the server with proper error handling

## Components

- **main.go**: Primary entry point with initialization code
- **main_test.go**: Integration tests for server startup

## Running Tests

### Test Types

- **Unit Tests**: Focused on testing individual components in isolation
- **Integration Tests**: Test multiple components working together, including server startup and API endpoints

### Test Commands

Run all tests with:

```bash
go test ./... -v
```

Run unit tests only (skip integration tests):

```bash
go test ./... -v -short
```

Run tests with coverage report:

```bash
go test ./... -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Run specific test packages:

```bash
# Run server package tests only
go test ./internal/server -v

# Run main command tests only
go test ./cmd/server -v
```

Run individual tests:

```bash
# Run a specific test by name
go test ./internal/server -run TestHealthEndpointIntegration -v
```

### Test Implementation Details

The test architecture includes:

- **Dynamic Port Assignment**: Tests use random ports based on process ID to avoid conflicts when running tests in parallel.
- **Environment Variable Configuration**: The `PB_SERVER_ADDR` environment variable allows tests to control the server's listening port.
- **Wait and Retry Logic**: Tests include robust retry mechanisms to ensure the server has time to start before attempting connections.
- **Shared Test Server Instance**: Integration tests reuse the same server instance when possible to improve performance.

### Common Test Issues

If tests fail with connection errors:
- The server might not be properly starting in test mode
- Ensure that the Server implementation correctly sets the "serve" command
- Check for port conflicts if the tests can't connect to the expected port

## Usage

Start the server with:

```bash
go run .
``` 
