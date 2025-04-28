# Monitoring Module

Real-time system metrics collection for enhanced observability.

## Components

- **cpu.go**: CPU usage and performance metrics
- **disk.go**: Disk space and I/O operations monitoring
- **memory.go**: RAM usage statistics
- **network.go**: Network traffic and connectivity metrics
- **process.go**: Process-level resource consumption
- **requests.go**: HTTP request tracking and analytics
- **runtime.go**: Go runtime statistics
- **stats.go**: Aggregate statistics and metrics handling
- **temperature.go**: System temperature monitoring
- **errors.go**: Error handling for monitoring systems

## Integration

Monitoring data is exposed through:
- Admin dashboard metrics
- JSON API endpoints
- Structured logs when thresholds are exceeded 