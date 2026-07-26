package server

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/audit"
	"github.com/magooney-loon/pb-ext/core/jobs"
	"github.com/magooney-loon/pb-ext/core/monitoring"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Server wraps PocketBase with additional stats
type Server struct {
	app          *pocketbase.PocketBase
	stats        *ServerStats
	requestStats *monitoring.RequestStats
	analytics    *analytics.Analytics
	alerts       *alerts.Notifier
	alertHandler *alerts.Handlers
	auditor      *audit.Auditor
	auditHandler *audit.Handlers
	jobManager   *jobs.Manager
	jobHandlers  *jobs.Handlers
	options      *options
}

// ServerStats tracks server metrics
type ServerStats struct {
	StartTime          time.Time
	TotalRequests      atomic.Uint64
	ActiveConnections  atomic.Int32
	LastRequestTime    atomic.Int64 // Unix timestamp
	TotalErrors        atomic.Uint64
	AverageRequestTime atomic.Int64 // nanoseconds
}

// shouldExcludeFromStats returns true if the path should be excluded from server statistics
func shouldExcludeFromStats(path string) bool {
	return path == "/service-worker.js" || path == "/favicon.ico" || path == "/manifest.json"
}

// New creates a server instance. Options args used for precision setup - pocketbase.Config and pocketbase.Pocketbase instance injection.
func New(create_options ...Option) *Server {
	var (
		opts    *options = &options{}
		pb_conf *pocketbase.Config
		pb_app  *pocketbase.PocketBase
	)

	for _, opt := range create_options {
		opt(opts)
	}
	if opts.config != nil {
		pb_conf = opts.config
	} else {
		pb_conf = &pocketbase.Config{
			DefaultDev: opts.developer_mode,
		}
	}

	if opts.pocketbase != nil {
		pb_app = opts.pocketbase
		if opts.developer_mode && !pb_app.App.IsDev() {
			pb_app.Logger().Warn("cannot change developer mode for pocketbase.Pocketbase, cause you already pass instance of *pocketbase.Pocketbase with unchecked dev mode flag")
		}
	} else {
		pb_app = pocketbase.NewWithConfig(*pb_conf)
	}

	return &Server{
		app:     pb_app,
		options: opts,
		stats: &ServerStats{
			StartTime: time.Now(),
		},
		// Owned by the Server rather than by SetupLogging, so per-status request
		// counts are available to the dashboard and to alerting whether or not
		// the app opted into pb-ext's logging.
		requestStats: monitoring.NewRequestStats(),
	}
}

// Start initializes and starts the server
func (s *Server) Start() error {
	app := s.app

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		app.Logger().Info("🌱 Server bootstrapping",
			"time", time.Now(),
			"pid", os.Getpid(),
		)

		if err := e.Next(); err != nil {
			return NewInternalError("bootstrap_initialization", "Failed to initialize core resources", err)
		}

		// pb-ext's schemas must exist before the job manager and analytics come
		// up, and OnBootstrap runs before apis.Serve reaches RunAllMigrations.
		// Applying them here records them in _migrations, so the later run is a
		// no-op. Only pb-ext's own migrations are applied — the app's run at
		// their normal time.
		if err := applyOwnMigrations(app); err != nil {
			return NewInternalError("bootstrap_migrations", "Failed to apply pb-ext migrations", err)
		}

		// Alerts come up before the job manager so a failure registering or
		// running a job can already be reported. Initialize never fails: an
		// unconfigured or broken notifier is a working no-op, because a server
		// that refuses to boot over its notification channel has turned a
		// convenience into an outage.
		s.alerts = alerts.Initialize(app, append(s.options.alerts, alerts.WithMetrics(s.metricsSnapshot))...)
		s.alertHandler = alerts.NewHandlers(s.alerts)

		// Admin access auditing. The auth hooks bind to the app rather than the
		// router, so they must be registered here, before serving starts.
		s.auditor = audit.Initialize(app, s.options.audit...)
		s.auditHandler = audit.NewHandlers(s.auditor)
		s.auditor.RegisterHooks(app)

		// Initialize job management system
		jobManager, err := jobs.Initialize(app)
		if err != nil {
			app.Logger().Error("Failed to initialize job management system", "error", err)
		} else {
			s.jobManager = jobManager

			if err := jobManager.RegisterInternalSystemJobs(); err != nil {
				app.Logger().Error("Failed to register internal system jobs", "error", err)
			}

			s.jobHandlers = jobs.NewHandlers(jobManager)

			app.Logger().Info("✅ Job management system initialized")
		}

		app.Logger().Info("✨ Server bootstrap complete",
			"time", time.Now(),
			"pid", os.Getpid(),
			"db_path", app.DataDir(),
		)

		return nil
	})

	// Persist any analytics counters still buffered in memory before exit.
	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		if s.analytics != nil {
			if err := s.analytics.Close(); err != nil {
				app.Logger().Error("Failed to flush analytics on shutdown", "error", err)
			}
		}

		// Persist any admin access still buffered. This runs before the alert
		// shutdown below, so a final flush that trips a detector can still send.
		if s.auditor != nil {
			if err := s.auditor.Close(); err != nil {
				app.Logger().Error("Failed to flush the admin access log on shutdown", "error", err)
			}
		}

		// Marking the run clean is what stops the next boot reporting a crash,
		// so it happens on a restart too — a dev-mode reload is not an incident.
		// NotifyStopped only queues; Close is what drains, within its own
		// deadline, so a wedged transport cannot hold the process open.
		if s.alerts != nil {
			s.alerts.NotifyStopped(e.IsRestart)
			_ = s.alerts.Close()
		}

		return e.Next()
	})

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		app.Logger().Info("🚀 Server initialized",
			"start_time", s.stats.StartTime,
			"pid", os.Getpid(),
			"db_path", app.DataDir(),
		)

		e.Router.BindFunc(func(c *core.RequestEvent) error {
			start := time.Now()
			s.stats.ActiveConnections.Add(1)

			// Only count requests that aren't excluded from stats
			if !shouldExcludeFromStats(c.Request.URL.Path) {
				s.stats.TotalRequests.Add(1)
			}

			// Debug log the counter increment
			/* app.Logger().Debug("Request counter incremented",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"total_requests", s.stats.TotalRequests.Load(),
			) */

			err := c.Next()

			s.stats.ActiveConnections.Add(-1)
			s.stats.LastRequestTime.Store(time.Now().Unix())

			duration := time.Since(start).Nanoseconds()

			// Per-status tracking lives here rather than in SetupLogging so it
			// runs for every app, and reads the response status directly —
			// TotalErrors below only counts handlers that *returned* an error,
			// so a handler that writes its own 500 never reaches it.
			if !shouldExcludeFromStats(c.Request.URL.Path) {
				s.requestStats.TrackRequest(monitoring.RequestMetrics{
					Path:          c.Request.URL.Path,
					Method:        c.Request.Method,
					StatusCode:    responseStatus(c),
					Duration:      time.Duration(duration),
					Timestamp:     start,
					UserAgent:     c.Request.UserAgent(),
					ContentLength: c.Request.ContentLength,
					RemoteAddr:    c.Request.RemoteAddr,
				})
			}

			// Only update average request time for non-excluded requests
			if !shouldExcludeFromStats(c.Request.URL.Path) {
				oldAvg := s.stats.AverageRequestTime.Load()
				totalReqs := s.stats.TotalRequests.Load()
				if totalReqs > 1 {
					newAvg := (oldAvg*(int64(totalReqs)-1) + duration) / int64(totalReqs)
					s.stats.AverageRequestTime.Store(newAvg)
				} else {
					s.stats.AverageRequestTime.Store(duration)
				}
			}

			// Only count errors for requests that aren't excluded from stats
			if err != nil && !shouldExcludeFromStats(c.Request.URL.Path) {
				s.stats.TotalErrors.Add(1)
			}

			/* app.Logger().Debug("Request completed",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"error", err,
				"duration_ms", duration/1e6,
				"active_connections", s.stats.ActiveConnections.Load(),
			) */

			return err
		})

		s.RegisterHealthRoute(e)

		// Initialize analytics system
		analyticsInst, err := analytics.Initialize(app)
		if err != nil {
			app.Logger().Error("Failed to initialize analytics", "error", err)
		} else {
			s.analytics = analyticsInst
			analyticsInst.RegisterRoutes(e)
			app.Logger().Info("✅ Analytics system initialized")
		}

		// Register job API routes
		if s.jobHandlers != nil {
			s.jobHandlers.RegisterRoutes(e)
			app.Logger().Info("⚡ Job API routes registered")
		}

		// Register alert routes and claim the run marker. NotifyStarted is what
		// reports an unclean previous exit, so it must run on every boot —
		// including when alerting itself is disabled, which still maintains the
		// marker so enabling it later needs no clean shutdown first.
		if s.alerts != nil {
			s.alertHandler.RegisterRoutes(e)
			s.alerts.NotifyStarted()
		}

		// Admin access auditing. Registered after the analytics middleware so
		// the two never see each other's work: analytics excludes /_/ outright,
		// this records only /_/ and superuser activity.
		if s.auditor != nil {
			s.auditor.RegisterRoutes(e)
			s.auditHandler.RegisterRoutes(e)
		}

		// Initialize API documentation system
		app.Logger().Info("📚 AST API system initialized")

		// Legacy cron routes are now handled by JobHandlers
		app.Logger().Info("⏰ Job management API initialized")

		// Serve static files from pb_public with improved path resolution
		publicDirPath := "./pb_public"

		// Check if the directory exists
		if _, err := os.Stat(publicDirPath); os.IsNotExist(err) {
			// Try with absolute path
			exePath, err := os.Executable()
			if err == nil {
				exeDir := filepath.Dir(exePath)
				possiblePaths := []string{
					filepath.Join(exeDir, "pb_public"),
					filepath.Join(exeDir, "../pb_public"),
					filepath.Join(exeDir, "../../pb_public"),
				}

				for _, path := range possiblePaths {
					if _, err := os.Stat(path); err == nil {
						publicDirPath = path
						app.Logger().Info("Using pb_public from absolute path", "path", publicDirPath)
						break
					}
				}
			}
		}

		app.Logger().Info("Serving static files from", "path", publicDirPath)
		e.Router.GET("/{path...}", apis.Static(os.DirFS(publicDirPath), false))

		return e.Next()
	})

	// Add extended server URLs after PocketBase initialization
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Extract the server address from the ServeEvent
		serverAddr := e.Server.Addr
		if serverAddr == "" {
			serverAddr = "127.0.0.1:8090" // fallback default
		}

		// Wait for the next tick to ensure PocketBase has logged its URLs first
		go func() {
			time.Sleep(100 * time.Millisecond)
			log.Printf("└─ pb-ext Dashboard:  http://%s/_/_", serverAddr)
		}()
		return e.Next()
	})

	// We don't need to set the args here as they should be set by the caller
	// before calling Start()

	// Log the command line args for debugging
	app.Logger().Debug("Starting server with args", "args", app.RootCmd.Flags().Args())

	if err := app.Start(); err != nil {
		return NewInternalError("server_start", "Failed to start server", err)
	}
	return nil
}

// App returns the underlying PocketBase instance
func (s *Server) App() *pocketbase.PocketBase {
	return s.app
}

// Stats returns the current server statistics
func (s *Server) Stats() *ServerStats {
	return s.stats
}

// RequestStats returns the per-path, per-status request tracker.
func (s *Server) RequestStats() *monitoring.RequestStats {
	return s.requestStats
}

// Alerts returns the notifier. It is nil until OnBootstrap has run; use
// alerts.Get from application code, which is never nil.
func (s *Server) Alerts() *alerts.Notifier {
	return s.alerts
}

// Auditor returns the admin access auditor. It is nil until OnBootstrap has run.
func (s *Server) Auditor() *audit.Auditor {
	return s.auditor
}

// responseStatus reports the status a request finished with. A zero status
// means the handler wrote a body without an explicit code, which net/http
// reports to the client as 200.
func responseStatus(c *core.RequestEvent) int {
	if status := c.Status(); status != 0 {
		return status
	}
	return 200
}

// metricsSnapshot feeds the alert rules. It is called on the evaluator tick
// (every 30s by default); CollectSystemStats memoizes for 2s, so the cost is
// one real collection per tick.
func (s *Server) metricsSnapshot() alerts.Metrics {
	totals := s.requestStats.Totals()

	m := alerts.Metrics{
		Requests:     totals.Requests,
		ClientErrors: totals.ClientErrors,
		ServerErrors: totals.ServerErrors,
		Goroutines:   runtime.NumGoroutine(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// A partial collection still returns usable stats alongside its error, the
	// same way the dashboard treats it — a host without sensors is not a
	// reason to stop watching memory.
	sys, _ := monitoring.CollectSystemStats(ctx, s.stats.StartTime, s.app.DataDir())
	if sys != nil {
		m.CPUPercent = averageCPUUsage(sys.CPUInfo)
		m.MemoryPercent = sys.MemoryInfo.UsedPercent
		m.DiskPercent = sys.DiskUsagePercent
	}

	return m
}
