# pb-ext

Enhanced PocketBase server with extensive monitoring & logging.

![image](https://github.com/user-attachments/assets/4466de28-d885-4112-95a9-84dde7f67dc7)

## Architecture

```
├── cmd/
│   └── server/          # Server initialization
├── core/
│   ├── logging/         # Logging and error handling
│   ├── monitoring/      # System metrics collection 
│   └── server/          # Core server implementation
├── pkg/
│   └── api/             # Custom API endpoints
```

## Core Features

- **System Monitoring**: Real-time metrics for CPU, memory, disk, network, and runtime stats
- **Structured Logging**: Comprehensive logging with error tracking and request tracing
- **API Group Endpoints**: Custom example endpoints:
  - `/api/utils/time`: Server time
- [Core Overview](core/README.md) - Core implementation modules

## Quick Start

```bash
go run cmd/server/main.go
```

## Webmaster Panel

- Admin panel `127.0.0.1:8090/_`
- Server panel `127.0.0.1:8090/_/_`
