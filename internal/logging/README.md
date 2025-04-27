# Logging Module

Structured logging system with comprehensive error tracking.

## Components

- **logging.go**: Core logging implementation with log levels and formatters
- **error_handler.go**: Centralized error capture, formatting, and reporting

## Features

- JSON-structured logs for machine parsing
- Configurable log levels (DEBUG, INFO, WARN, ERROR)
- Request tracing with correlation IDs
- Contextual metadata enrichment
- Integration with monitoring alerts

## Usage

```go
import "github.com/yourusername/pb-ext/internal/logging"

// Basic logging
logging.Info("Service started", map[string]interface{}{"port": 8090})

// Error handling
err := someOperation()
if err != nil {
    logging.ErrorWithContext(ctx, "Operation failed", err, map[string]interface{}{
        "operation": "someOperation",
    })
}
``` 