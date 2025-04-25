package server

import (
	"os"
	"sync/atomic"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// Server represents our PocketBase server instance
type Server struct {
	app   *pocketbase.PocketBase
	stats *ServerStats
}

// ServerStats holds server statistics
type ServerStats struct {
	StartTime          time.Time
	TotalRequests      atomic.Uint64
	ActiveConnections  atomic.Int32
	LastRequestTime    atomic.Int64 // Unix timestamp
	TotalErrors        atomic.Uint64
	AverageRequestTime atomic.Int64 // nanoseconds
}

// New creates a new server instance
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

	// Setup bootstrap hook for initialization
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		// Log bootstrap start
		app.Logger().Info("🌱 Server bootstrapping",
			"time", time.Now(),
			"pid", os.Getpid(),
		)

		// Initialize core resources
		if err := e.Next(); err != nil {
			return NewInternalError("bootstrap_initialization", "Failed to initialize core resources", err)
		}

		// Log successful bootstrap
		app.Logger().Info("✨ Server bootstrap complete",
			"time", time.Now(),
			"pid", os.Getpid(),
			"db_path", app.DataDir(),
		)

		return nil
	})

	// Setup request tracking and static files
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Log server initialization
		app.Logger().Info("🚀 Server initialized",
			"start_time", s.stats.StartTime,
			"pid", os.Getpid(),
			"db_path", app.DataDir(),
		)

		// Track request statistics using middleware
		e.Router.BindFunc(func(c *core.RequestEvent) error {
			start := time.Now()
			s.stats.ActiveConnections.Add(1)
			s.stats.TotalRequests.Add(1)

			// Execute the next handler
			err := c.Next()

			s.stats.ActiveConnections.Add(-1)
			s.stats.LastRequestTime.Store(time.Now().Unix())

			// Update average request time
			duration := time.Since(start).Nanoseconds()
			oldAvg := s.stats.AverageRequestTime.Load()
			totalReqs := s.stats.TotalRequests.Load()
			if totalReqs > 1 {
				newAvg := (oldAvg*(int64(totalReqs)-1) + duration) / int64(totalReqs)
				s.stats.AverageRequestTime.Store(newAvg)
			} else {
				s.stats.AverageRequestTime.Store(duration)
			}

			// Check for errors
			if err != nil {
				s.stats.TotalErrors.Add(1)
			}

			// Log request completion
			app.Logger().Debug("Request completed",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"error", err,
				"duration_ms", duration/1e6,
				"active_connections", s.stats.ActiveConnections.Load(),
			)

			return err
		})

		// Register health check endpoint
		s.RegisterHealthRoute(e)

		// serves static files from the provided public dir (if exists)
		e.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		return e.Next()
	})

	// Start the server
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
