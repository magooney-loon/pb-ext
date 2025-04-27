package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthEndpointIntegration tests the full health endpoint response
// including monitoring and system stats
func TestHealthEndpointIntegration(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start the server with proper initialization
	t.Log("Starting test server for health endpoint integration test")
	server := setupTestServer(t)
	app := server.App()
	defer app.ResetBootstrapState()

	healthURL := fmt.Sprintf("http://localhost:%d/api/health", testPort)
	t.Logf("Testing health endpoint at: %s", healthURL)

	t.Logf("Initial server state - TotalRequests: %d", server.Stats().TotalRequests.Load())

	// Check initial request count
	initialReqs := server.Stats().TotalRequests.Load()

	// Make request to health endpoint with retry logic
	client := &http.Client{Timeout: 5 * time.Second}
	var resp *http.Response
	var err error

	// Retry logic for connection
	for i := 0; i < 5; i++ {
		t.Logf("Health check attempt %d/5", i+1)
		resp, err = client.Get(healthURL)
		if err == nil {
			break
		}
		t.Logf("Health check attempt failed: %v", err)
		time.Sleep(500 * time.Millisecond)
	}

	require.NoError(t, err, "Health request should not error after retries")
	defer resp.Body.Close()

	// Check status code
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should return 200 OK")

	// Log the response status
	t.Logf("Response status: %d", resp.StatusCode)

	// Get response body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Should read response body")

	// Parse health data
	var data map[string]interface{}
	err = json.Unmarshal(body, &data)
	require.NoError(t, err, "Should parse JSON response")

	// Validate response format
	assert.Contains(t, data, "code", "Response should contain a code field")
	assert.Equal(t, float64(200), data["code"], "Code should be 200")
	assert.Contains(t, data, "message", "Response should contain a message field")
	assert.Contains(t, data["message"], "healthy", "Message should indicate healthy state")

	// Wait with polling approach to verify request counter
	success := false
	maxRetries := 10

	for i := 0; i < maxRetries; i++ {
		currentReqs := server.Stats().TotalRequests.Load()
		t.Logf("Attempt %d - Current TotalRequests: %d (initial: %d)", i+1, currentReqs, initialReqs)

		if currentReqs > initialReqs {
			success = true
			t.Logf("Counter increment detected - TotalRequests: %d", currentReqs)
			break
		}

		// Wait between checks with increasing delay
		waitTime := time.Duration(100*(i+1)) * time.Millisecond
		t.Logf("Waiting %v before next check", waitTime)
		time.Sleep(waitTime)
	}

	// After all retries, get the final state
	finalReqs := server.Stats().TotalRequests.Load()
	t.Logf("Final server state - TotalRequests: %d (initial: %d)", finalReqs, initialReqs)

	// Check request counter was incremented
	assert.True(t, success, "Request counter should increment after request")
}

// TestComponentIntegration verifies that various components work together
func TestComponentIntegration(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test server with proper initialization
	t.Log("Starting test server for component integration test")
	s := setupTestServer(t)

	// Verify tracking is working
	startTime := s.Stats().StartTime
	assert.WithinDuration(t, time.Now(), startTime, 10*time.Second,
		"Server start time should be recent")

	// Verify app instance is properly configured
	app := s.App()
	assert.NotNil(t, app, "PocketBase app should be initialized")
	assert.NotEmpty(t, app.DataDir(), "App data directory should be set")

	// Make a series of requests with retry logic
	initialRequests := s.Stats().TotalRequests.Load()
	client := &http.Client{Timeout: 5 * time.Second}

	healthURL := fmt.Sprintf("http://localhost:%d/api/health", testPort)
	t.Logf("Testing health endpoint at: %s", healthURL)

	for i := 0; i < 3; i++ {
		t.Logf("Making request %d/3", i+1)

		// Retry logic for connection
		var resp *http.Response
		var err error

		for j := 0; j < 3; j++ {
			t.Logf("  Attempt %d/3", j+1)
			resp, err = client.Get(healthURL)
			if err == nil {
				break
			}
			t.Logf("  Request failed: %v", err)
			time.Sleep(300 * time.Millisecond)
		}

		require.NoError(t, err, "Health endpoint should be reachable")
		resp.Body.Close()
		time.Sleep(300 * time.Millisecond) // Longer wait between requests
	}

	// Wait longer with retry logic for request counter update
	var success bool
	var currentRequests uint64

	// Try several times with increasing waits
	for i := 0; i < 5; i++ {
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
		currentRequests = s.Stats().TotalRequests.Load()
		if currentRequests > initialRequests {
			success = true
			break
		}
	}

	// Verify request tracking works with multiple requests
	assert.True(t, success, "Request counter should track requests (current: %d, initial: %d)",
		currentRequests, initialRequests)
}
