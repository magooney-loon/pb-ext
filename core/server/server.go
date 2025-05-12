package server

import (
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
			s.stats.TotalRequests.Add(1)

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
			oldAvg := s.stats.AverageRequestTime.Load()
			totalReqs := s.stats.TotalRequests.Load()
			if totalReqs > 1 {
				newAvg := (oldAvg*(int64(totalReqs)-1) + duration) / int64(totalReqs)
				s.stats.AverageRequestTime.Store(newAvg)
			} else {
				s.stats.AverageRequestTime.Store(duration)
			}

			if err != nil {
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
