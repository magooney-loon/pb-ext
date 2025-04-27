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

## Usage

Start the server with:

```bash
go run main.go serve
``` 