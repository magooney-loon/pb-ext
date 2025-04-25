# pb-ext

Enhanced PocketBase server with extensive monitoring & logging.

## Architecture

```
├── cmd/
│   └── server/          # Server initialization
├── internal/
│   ├── monitoring/      # System metrics collection 
│   ├── logging/         # Logging and error handling
│   └── server/          # Core server implementation
├── pkg/
│   └── api/             # Custom API endpoints
```

## Core Features

- **System Monitoring**: Real-time metrics for CPU, memory, disk, network, and runtime stats
- **Structured Logging**: Comprehensive logging with error tracking and request tracing
- **Utility API Endpoints**: Custom example endpoints:
  - `/api/utils/time`: Server time
  - `/api/utils/uuid`: UUID generation

## Quick Start

```bash
go run cmd/server/main.go serve
```

## Webmaster Panel

Admin panel `127.0.0.1:8090/_`
Server panel `127.0.0.1:8090/_/_`
