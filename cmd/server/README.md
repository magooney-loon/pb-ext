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

Run all tests with:

```bash
go test ./... -v
```

Run unit tests only (skip integration tests):

```bash
go test ./... -v -short
```

### Test Dependencies

Before running tests, you'll need to install the test dependencies:

```bash
go get github.com/stretchr/testify
```

### Linter Issues

Some tests may show linter errors since they require mocking PocketBase functionality. These issues typically include:

- Missing imports for test-only code
- References to undefined functions from the main application
- Mock implementations of interfaces

In most cases, these can be safely ignored as they're part of the test infrastructure and don't affect the actual application code.

## Usage

Start the server with:

```bash
go run main.go serve
``` 