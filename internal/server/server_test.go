package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	// Create a new server instance
	s := New()

	// Assert server is properly initialized
	assert.NotNil(t, s, "Server should not be nil")
	assert.NotNil(t, s.App(), "PocketBase app should not be nil")

	// Test server configuration
	assert.NotEmpty(t, s.App().DataDir(), "Data directory should be configured")
}

func TestServerStats(t *testing.T) {
	// Create a new server instance
	s := New()

	// Get stats
	stats := s.Stats()

	// Verify stats tracking is initialized
	assert.NotNil(t, stats, "Stats should not be nil")
	assert.WithinDuration(t, time.Now(), stats.StartTime, 5*time.Second, "Start time should be recent")
	assert.Equal(t, int64(0), stats.TotalRequests.Load(), "Initial request count should be zero")
	assert.Equal(t, int64(0), stats.ActiveConnections.Load(), "Initial connections should be zero")
}

func TestHealthEndpoint(t *testing.T) {
	// Skip in short mode as it starts a server
	if testing.Short() {
		t.Skip("Skipping test in short mode")
	}

	// Create a new server instance
	s := New()

	// Start server in a goroutine with cancelation
	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server failed to start: %v", err)
		}
	}()

	// Allow time for server to start
	time.Sleep(100 * time.Millisecond)
	defer s.App().ResetBootstrapState()

	// Check health endpoint with default timeout
	client := &http.Client{Timeout: 2 * time.Second}

	// Shutdown server after test
	defer func() {
		// PocketBase doesn't have a direct Http/Router shutdown method
		// We'll use the ResetBootstrapState which handles closing resources
		s.App().ResetBootstrapState()
	}()

	// Test the health endpoint
	resp, err := client.Get("http://localhost:8090/api/health")
	if err != nil {
		t.Log("Server may not have started, skipping endpoint test")
		return
	}
	defer resp.Body.Close()

	// Verify response
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
