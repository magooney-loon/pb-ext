package server

import (
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Server wraps PocketBase with additional stats
type Server struct {
	app       *pocketbase.PocketBase
	stats     *ServerStats
	analytics *Analytics
	options   *options
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

		app.Logger().Info("✨ Server bootstrap complete",
			"time", time.Now(),
			"pid", os.Getpid(),
			"db_path", app.DataDir(),
		)

		return nil
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
		analytics, err := InitializeAnalytics(app)
		if err != nil {
			app.Logger().Error("Failed to initialize analytics", "error", err)
		} else {
			s.analytics = analytics
			analytics.RegisterRoutes(e)
			app.Logger().Info("✅ Analytics system initialized")
		}

		// Initialize API documentation system
		// NOTE: API docs routes are now handled by the version manager to prevent conflicts
		// s.RegisterAPIDocsRoutes(e)
		app.Logger().Info("📚 AST API system initialized (using version manager)")

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
		// Wait for the next tick to ensure PocketBase has logged its URLs first
		go func() {
			time.Sleep(100 * time.Millisecond)
			// Match PocketBase's log format for additional URLs
			log.Println("└─ pb-ext Dashboard:  http://127.0.0.1:8090/_/_")
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

// RegisterAPIDocsRoutes initializes and registers the API documentation routes

// Stats returns the current server statistics
func (s *Server) Stats() *ServerStats {
	return s.stats
}
