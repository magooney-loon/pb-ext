package server

import (
	"os"
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

// New creates a server instance
func New() *Server {
	return &Server{
		app: pocketbase.New(),
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

		e.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))

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
