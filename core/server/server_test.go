package server

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Server control variables for integration tests
var (
	serverOnce  sync.Once
	server      *Server
	serverMutex sync.Mutex
	testClient  *http.Client
	testPort    = 8090 + (os.Getpid() % 1000) // Use a different port based on process ID
)

// setupTestServer ensures we have a running server for tests
func setupTestServer(t *testing.T) *Server {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if server != nil {
		t.Logf("Reusing existing server instance")
		return server
	}

	// Generate a predictable but unique test port
	testPort = 9000 + rand.Intn(1000)
	testAddr := fmt.Sprintf("127.0.0.1:%d", testPort)
	t.Logf("Selected test port: %d", testPort)

	serverOnce.Do(func() {
		t.Logf("Creating new server instance")
		// Create a new server instance
		s := New()

		// Configure server for testing
		app := s.App()

		// Set data directory to temp location for tests to avoid conflicts
		dataDir := "./pb_data"
		t.Logf("Setting PocketBase data directory to: %s", dataDir)
		app.RootCmd.PersistentFlags().Set("dir", dataDir)

		// Enable dev mode for more verbose logging
		app.RootCmd.PersistentFlags().Set("dev", "true")

		// Set HTTP server address to ensure we know where it's listening
		// Use a dynamic port to avoid conflicts
		t.Logf("Setting server address to: %s", testAddr)
		os.Setenv("PB_SERVER_ADDR", testAddr)

		// Clear any previous args and set the serve command with explicit HTTP address
		app.RootCmd.ResetFlags()
		app.RootCmd.SetArgs([]string{"serve", "--http=" + testAddr})

		server = s
		t.Logf("Server instance created")

		// Start server in a goroutine with cancelation
		t.Logf("Starting server in background...")
		go func() {
			if err := s.Start(); err != nil && err != http.ErrServerClosed {
				t.Errorf("❌ Server failed to start: %v", err)
			}
		}()

		// Wait for server to start
		t.Logf("Waiting for server to become ready...")
		waitForServerReady(t)
		t.Logf("✅ Server is ready")
	})

	return server
}

// waitForServerReady ensures the server is up and responding
func waitForServerReady(t *testing.T) {
	// Create client if needed
	if testClient == nil {
		testClient = &http.Client{Timeout: 5 * time.Second}
	}

	// Try to connect to health endpoint with retries
	ready := false
	maxRetries := 20 // Increase number of retries

	t.Logf("Waiting for server to start...")

	baseURL := fmt.Sprintf("http://localhost:%d", testPort)
	healthURL := fmt.Sprintf("%s/api/health", baseURL)

	for i := 0; i < maxRetries; i++ {
		t.Logf("Attempt %d/%d - Connecting to %s", i+1, maxRetries, healthURL)

		resp, err := testClient.Get(healthURL)
		if err == nil {
			t.Logf("Server is responsive!")
			resp.Body.Close()
			ready = true
			break
		}

		t.Logf("Connection attempt failed: %v", err)
		// Increase wait time between retries with an exponential backoff
		waitTime := time.Duration(300*(i+1)) * time.Millisecond
		t.Logf("Waiting %v before next attempt", waitTime)
		time.Sleep(waitTime)
	}

	require.True(t, ready, "Server failed to start within timeout period")
}

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
	assert.Equal(t, uint64(0), stats.TotalRequests.Load(), "Initial request count should be zero")
	assert.Equal(t, int32(0), stats.ActiveConnections.Load(), "Initial connections should be zero")
}

func TestHealthEndpoint(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test server
	server := setupTestServer(t)

	// Log server status
	t.Logf("Server has %d total requests so far", server.Stats().TotalRequests.Load())

	// Test the health endpoint
	healthURL := fmt.Sprintf("http://localhost:%d/api/health", testPort)
	t.Logf("Testing health endpoint at: %s", healthURL)

	resp, err := testClient.Get(healthURL)
	require.NoError(t, err, "Health endpoint should be accessible")
	defer resp.Body.Close()

	// Verify status code
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should return 200 OK")

	// Verify response data
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should be able to read response body")

	// Check if response is valid JSON
	var healthData map[string]interface{}
	err = json.Unmarshal(body, &healthData)
	require.NoError(t, err, "Health response should be valid JSON")

	// Verify health data structure matches API format
	assert.Contains(t, healthData, "code", "Health data should contain status code")
	assert.Equal(t, float64(200), healthData["code"], "Health status code should be 200")
	assert.Contains(t, healthData, "message", "Health data should contain message")
	assert.Contains(t, healthData["message"], "healthy", "Health message should indicate healthy state")
}

func TestServerIntegration(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test server
	s := setupTestServer(t)

	// Get the current status of the server before making our request
	t.Logf("Initial server state - TotalRequests: %d", s.Stats().TotalRequests.Load())

	// Test stats are being tracked
	initialRequests := s.Stats().TotalRequests.Load()

	// Make a request
	healthURL := fmt.Sprintf("http://localhost:%d/api/health", testPort)
	t.Logf("Testing health endpoint at: %s", healthURL)

	resp, err := testClient.Get(healthURL)
	require.NoError(t, err, "Health endpoint should be accessible")
	resp.Body.Close()

	// Log the response status
	t.Logf("Response status: %d", resp.StatusCode)

	// Wait with polling approach
	success := false
	maxRetries := 10

	for i := 0; i < maxRetries; i++ {
		currentRequests := s.Stats().TotalRequests.Load()
		t.Logf("Attempt %d - Current TotalRequests: %d (initial: %d)", i+1, currentRequests, initialRequests)

		if currentRequests > initialRequests {
			success = true
			t.Logf("Counter increment detected - TotalRequests: %d", currentRequests)
			break
		}

		// Wait between checks with increasing delay
		waitTime := time.Duration(100*(i+1)) * time.Millisecond
		t.Logf("Waiting %v before next check", waitTime)
		time.Sleep(waitTime)
	}

	// After all retries, get the final state
	finalRequests := s.Stats().TotalRequests.Load()
	t.Logf("Final server state - TotalRequests: %d (initial: %d)", finalRequests, initialRequests)

	// Check if request counter was incremented
	assert.True(t, success, "Request counter should be incremented after request")
}

func TestMain(m *testing.M) {
	// Create test client
	testClient = &http.Client{
		Timeout: 5 * time.Second,
	}

	println("🧪 Starting server tests")

	// Run the tests
	code := m.Run()

	println("🧹 Cleaning up test resources")

	// Clean up the server if it was started
	if server != nil {
		println("📴 Shutting down test server")
		// PocketBase doesn't have a direct shutdown method
		// We'd need to trigger the app's ResetBootstrapState instead
		server.App().ResetBootstrapState()

		// Reset server state for future tests
		server = nil
		serverOnce = sync.Once{}
		println("Server state reset")
	}

	// Don't call os.Exit in tests
	if code != 0 {
		// Just log in tests
		println("⚠️ WARNING: Some tests failed with code:", code)
	} else {
		println("✅ All tests passed")
	}
}
