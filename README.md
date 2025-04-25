# pb-ext

Enhanced PocketBase server with extensive monitoring, logging, and health check capabilities.

## Architecture

```
├── cmd/
│   └── server/          # Server entrypoint and initialization
├── internal/
│   ├── monitoring/      # System metrics collection (CPU, Memory, Network, etc.)
│   ├── logging/         # Structured logging and error handling
│   └── server/          # Core server implementation and health checks
```

## Core Features

- **Enhanced PocketBase Server**: Extended PocketBase implementation with robust monitoring
- **System Monitoring**: Real-time metrics for CPU, memory, disk, network, and runtime stats
- **Structured Logging**: Comprehensive logging with error tracking and request tracing
- **Health Checks**: Built-in health monitoring endpoints
- **Request Statistics**: Detailed request tracking with timing and error rate metrics

## Quick Start

```bash
go run cmd/server/main.go serve
```

## Webmaster Panel

Admin panel `127.0.0.1:8090/_`
Server panel `127.0.0.1:8090/_/_`
