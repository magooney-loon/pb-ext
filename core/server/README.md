# Server Module

Enhanced PocketBase server wrapper with analytics, health monitoring, and comprehensive request tracking.

## Overview

This module extends PocketBase with production-ready features:
- **Request Analytics**: Track page views, visitors, device/browser statistics
- **Health Monitoring**: Real-time system metrics and server statistics dashboard
- **Error Handling**: Structured error types with HTTP status mapping
- **API Documentation**: Integration with automatic API documentation system
- **Static File Serving**: Enhanced static file serving with path resolution

## Architecture

```
Server
├── PocketBase Core        # Wrapped PocketBase instance
├── Analytics             # Visitor tracking & statistics
├── Health Monitor        # System metrics & dashboard
├── Error System         # Structured error handling
└── Template System      # Embedded UI templates
```

## Core Types

### Server & Configuration
```go
type Server struct {
    app       *pocketbase.PocketBase
    stats     *ServerStats
    analytics *Analytics
    options   *options
}

type ServerStats struct {
    StartTime          time.Time
    TotalRequests      atomic.Uint64
    ActiveConnections  atomic.Int32
    LastRequestTime    atomic.Int64
    TotalErrors        atomic.Uint64
    AverageRequestTime atomic.Int64
}

type Option func(*options)
```

### Analytics System
```go
type Analytics struct {
    app           *pocketbase.PocketBase
    buffer        []PageView
    flushInterval time.Duration
    batchSize     int
    knownVisitors map[string]time.Time
    sessionWindow time.Duration
}

type PageView struct {
    Path        string    `json:"path"`
    Method      string    `json:"method"`
    IP          string    `json:"ip"`
    UserAgent   string    `json:"user_agent"`
    Referrer    string    `json:"referrer"`
    Duration    int64     `json:"duration_ms"`
    Timestamp   time.Time `json:"timestamp"`
    VisitorID   string    `json:"visitor_id"`
    DeviceType  string    `json:"device_type"`
    Browser     string    `json:"browser"`
    OS          string    `json:"os"`
    Country     string    `json:"country"`
    UTMSource   string    `json:"utm_source"`
    UTMMedium   string    `json:"utm_medium"`
    UTMCampaign string    `json:"utm_campaign"`
    IsNewVisit  bool      `json:"is_new_visit"`
    QueryParams string    `json:"query_params"`
}

type AnalyticsData struct {
    UniqueVisitors          int                `json:"unique_visitors"`
    NewVisitors            int                `json:"new_visitors"`
    ReturningVisitors      int                `json:"returning_visitors"`
    TotalPageViews         int                `json:"total_page_views"`
    ViewsPerVisitor        float64            `json:"views_per_visitor"`
    TodayPageViews         int                `json:"today_page_views"`
    YesterdayPageViews     int                `json:"yesterday_page_views"`
    TopDeviceType          string             `json:"top_device_type"`
    TopDevicePercentage    float64            `json:"top_device_percentage"`
    DesktopPercentage      float64            `json:"desktop_percentage"`
    MobilePercentage       float64            `json:"mobile_percentage"`
    TabletPercentage       float64            `json:"tablet_percentage"`
    TopBrowser             string             `json:"top_browser"`
    BrowserBreakdown       map[string]float64 `json:"browser_breakdown"`
    TopPages               []PageStat         `json:"top_pages"`
    RecentVisits           []RecentVisit      `json:"recent_visits"`
    RecentVisitCount       int                `json:"recent_visit_count"`
    HourlyActivityPercentage float64          `json:"hourly_activity_percentage"`
}

type PageStat struct {
    Path  string `json:"path"`
    Views int    `json:"views"`
}

type RecentVisit struct {
    Time       time.Time `json:"time"`
    Path       string    `json:"path"`
    DeviceType string    `json:"device_type"`
    Browser    string    `json:"browser"`
    OS         string    `json:"os"`
}
```

### Error Handling
```go
type ServerError struct {
    Type       string // Error category
    Message    string // Human-readable message
    Op         string // Operation name
    StatusCode int    // HTTP status code
    Err        error  // Original error
}

// Error type constants
const (
    ErrTypeHTTP       = "http_error"
    ErrTypeRouting    = "routing_error"
    ErrTypeAuth       = "auth_error"
    ErrTypeTemplate   = "template_error"
    ErrTypeConfig     = "config_error"
    ErrTypeDatabase   = "database_error"
    ErrTypeMiddleware = "middleware_error"
    ErrTypeInternal   = "internal_error"
)
```

### Health Monitoring
```go
type HealthResponse struct {
    Status        string       `json:"status"`
    ServerStats   *ServerStats `json:"server_stats"`
    SystemStats   interface{}  `json:"system_stats"`
    LastCheckTime time.Time    `json:"last_check_time"`
}
```

## Configuration Options

### Functional Options Pattern
```go
// WithConfig sets PocketBase configuration
func WithConfig(config *pocketbase.Config) Option

// WithPocketbase uses existing PocketBase instance  
func WithPocketbase(pocketbase *pocketbase.PocketBase) Option

// WithMode sets developer mode
func WithMode(developer_mode bool) Option

// InDeveloperMode enables developer mode
func InDeveloperMode() Option

// InNormalMode disables developer mode
func InNormalMode() Option
```

## Main Components

### Server Core
- **Purpose**: Enhanced PocketBase wrapper with production features
- **Features**: Request tracking, middleware integration, static file serving
- **Key Methods**: `New()`, `Start()`, `App()`, `Stats()`

### Analytics System
- **Purpose**: Comprehensive visitor and usage analytics
- **Features**: Page view tracking, visitor identification, device/browser detection
- **Processing**: Background buffering and batching for performance
- **Storage**: PocketBase collections for persistence

### Health Monitor  
- **Purpose**: Real-time system and application health dashboard
- **Features**: CPU, memory, disk usage, request metrics, temperature monitoring
- **UI**: Template-based dashboard with authentication
- **Access**: `/_/_` endpoint for superuser access

### Error System
- **Purpose**: Structured error handling with HTTP status mapping
- **Features**: Error categorization, unwrapping support, type checking
- **Categories**: HTTP, routing, auth, template, config, database, internal

### Template System
- **Purpose**: Embedded UI templates for health dashboard
- **Features**: Component-based templates, custom template functions
- **Components**: Header, metrics, CPU details, memory details, visitor analytics

## Usage Patterns

### Basic Server Setup
```go
server := New()
server.Start()
```

### Custom Configuration
```go
server := New(
    WithConfig(&pocketbase.Config{DefaultDev: true}),
    InDeveloperMode(),
)
```

### With Existing PocketBase
```go
app := pocketbase.New()
server := New(WithPocketbase(app))
```

## Endpoints

### Health & Monitoring
- **Health Dashboard**: `GET /_/_` (superuser auth required)
- **Analytics Data**: Available through dashboard interface

### API Integration
- **OpenAPI Docs**: `GET /api/docs/openapi`  
- **API Statistics**: `GET /api/docs/stats`
- **API Components**: `GET /api/docs/components`

## Features

- ✅ **Zero Configuration**: Works with PocketBase defaults
- ✅ **Request Analytics**: Track visitors, devices, browsers, pages
- ✅ **Real-time Monitoring**: System metrics with live dashboard
- ✅ **Error Handling**: Structured errors with HTTP status codes
- ✅ **Template System**: Embedded UI components and scripts
- ✅ **Static File Serving**: Enhanced path resolution
- ✅ **Performance Optimized**: Background processing with batching
- ✅ **Thread Safe**: Atomic counters and proper mutex usage
- ✅ **Production Ready**: Comprehensive logging and error handling

## Analytics Features

- **Visitor Tracking**: Anonymous visitor identification with session management
- **Device Detection**: Desktop/mobile/tablet classification
- **Browser Analysis**: User agent parsing for browser/OS identification  
- **UTM Tracking**: Marketing campaign parameter capture
- **Geographic Data**: Country-level location tracking
- **Performance Metrics**: Request duration and error rate tracking
- **Real-time Stats**: Live visitor counts and activity percentages

## Health Dashboard Features

- **System Metrics**: CPU usage, memory consumption, disk space
- **Application Stats**: Request counts, error rates, response times  
- **Temperature Monitoring**: System and disk temperature sensors
- **Network Activity**: Connection counts and request patterns
- **Uptime Tracking**: Server start time and running duration
- **Authentication**: Secure superuser access with localStorage token handling