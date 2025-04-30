package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/magooney-loon/pb-ext/core/monitoring"

	"github.com/pocketbase/pocketbase/core"
	"github.com/shirou/gopsutil/v3/host"
)

// HealthResponse represents health check response data
type HealthResponse struct {
	Status        string       `json:"status"`
	ServerStats   *ServerStats `json:"server_stats"`
	SystemStats   interface{}  `json:"system_stats"`
	LastCheckTime time.Time    `json:"last_check_time"`
}

// Template functions map
var templateFuncs = template.FuncMap{
	"multiply": func(a, b float64) float64 {
		return a * b
	},
	"divide": func(a, b any) float64 {
		var af, bf float64

		switch v := a.(type) {
		case float64:
			af = v
		case uint64:
			af = float64(v)
		default:
			return 0
		}

		switch v := b.(type) {
		case float64:
			bf = v
		case uint64:
			bf = float64(v)
		default:
			return 0
		}

		if bf == 0 {
			return 0
		}
		return af / bf
	},
	"divideFloat64": func(a interface{}, b float64) float64 {
		if b == 0 {
			return 0
		}

		var af float64
		switch v := a.(type) {
		case float64:
			af = v
		case uint64:
			af = float64(v)
		case int64:
			af = float64(v)
		case int:
			af = float64(v)
		default:
			return 0
		}

		return af / b
	},
	"int64": func(v interface{}) int64 {
		switch val := v.(type) {
		case int64:
			return val
		case int:
			return int64(val)
		case float64:
			return int64(val)
		case time.Duration:
			return int64(val)
		default:
			return 0
		}
	},
	"errorRate": func(errors, total uint64) float64 {
		if total == 0 {
			return 0
		}
		return float64(errors) * 100 / float64(total)
	},
	"avgCPUUsage": func(cpus []monitoring.CPUInfo) float64 {
		if len(cpus) == 0 {
			return 0
		}
		var total float64
		for _, cpu := range cpus {
			total += cpu.Usage
		}
		return total / float64(len(cpus))
	},
	"formatBytes": func(bytes uint64) string {
		const unit = 1024
		if bytes < unit {
			return fmt.Sprintf("%d B", bytes)
		}
		div, exp := uint64(unit), 0
		for n := bytes / unit; n >= unit; n /= unit {
			div *= unit
			exp++
		}
		return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
	},
	"getDiskTemp": func(stats *monitoring.SystemStats) float64 {
		if !stats.HasTempData {
			return 0
		}

		temps, err := host.SensorsTemperatures()
		if err != nil {
			return 0
		}

		for _, temp := range temps {
			if monitoring.IsDiskTemp(temp.SensorKey) {
				return temp.Temperature
			}
		}
		return 0
	},
	"getSystemTemp": func(stats *monitoring.SystemStats) float64 {
		if !stats.HasTempData {
			return 0
		}

		temps, err := host.SensorsTemperatures()
		if err != nil {
			return 0
		}

		for _, temp := range temps {
			if monitoring.IsSystemTemp(temp.SensorKey) {
				return temp.Temperature
			}
		}
		return 0
	},
	"getAmbientTemp": func(stats *monitoring.SystemStats) float64 {
		if !stats.HasTempData {
			return 0
		}

		temps, err := host.SensorsTemperatures()
		if err != nil {
			return 0
		}

		for _, temp := range temps {
			if strings.Contains(strings.ToLower(temp.SensorKey), "ambient") {
				return temp.Temperature
			}
		}
		return 0
	},
	"hasDiskTemps": func(stats *monitoring.SystemStats) bool {
		if !stats.HasTempData {
			return false
		}

		temps, err := host.SensorsTemperatures()
		if err != nil {
			return false
		}

		for _, temp := range temps {
			if monitoring.IsDiskTemp(temp.SensorKey) {
				return true
			}
		}
		return false
	},
	"formatTime": func(t time.Time) string {
		return t.Format("15:04:05")
	},
	"inc": func(i int) int {
		return i + 1
	},
}

// RegisterHealthRoute registers the health check endpoint
func (s *Server) RegisterHealthRoute(e *core.ServeEvent) {
	// Parse templates from embedded filesystem
	tmpl, err := template.New("index.tmpl").Funcs(templateFuncs).ParseFS(templateFS,
		"templates/index.tmpl",
		"templates/scripts/main.tmpl",
		"templates/components/header.tmpl",
		"templates/components/critical_metrics.tmpl",
		"templates/components/cpu_details.tmpl",
		"templates/components/memory_details.tmpl",
		"templates/components/network_details.tmpl",
		"templates/components/visitor_analytics.tmpl",
		"templates/components/pb_integration.tmpl",
	)
	if err != nil {
		log.Printf("Error parsing health templates: %v", err)
		return
	}

	// Health check endpoint handler
	healthHandler := func(c *core.RequestEvent) error {
		// Log the authorization header for debugging
		authHeader := c.Request.Header.Get("Authorization")
		log.Printf("Health dashboard access - Auth header: %s", authHeader)

		// If not already authenticated, try to use localStorage token directly
		if c.Auth == nil || !c.Auth.IsSuperuser() {
			// Add client-side script to get token from localStorage and make a fetch request with proper Authorization header
			html := `
			<!DOCTYPE html>
			<html>
			<head>
				<title>Health Dashboard</title>
				<style>
					body { font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; padding: 20px; }
					.container { max-width: 800px; margin: 0 auto; }
					.loading { text-align: center; margin-top: 100px; }
					.error { background-color: #ffebee; padding: 15px; border-radius: 4px; margin-top: 20px; }
					.debug { background-color: #e3f2fd; padding: 15px; border-radius: 4px; margin-top: 20px; font-family: monospace; white-space: pre-wrap; }
					button { padding: 8px 16px; background: #2196f3; color: white; border: none; border-radius: 4px; cursor: pointer; margin: 10px 0; }
					button:hover { background: #1976d2; }
				</style>
			</head>
			<body>
				<div class="container">
					<div id="loading" class="loading">
						<h2>Loading Health Dashboard...</h2>
						<p>Authenticating with your admin credentials...</p>
					</div>
					<div id="error" style="display:none;" class="error">
						<h3>Authentication Error</h3>
						<p>You need to be logged in as a superuser to access the health dashboard.</p>
						<p>Please <a href="/_/" target="_blank">login to the admin panel</a> first and then try again.</p>
						<button id="retryBtn">I've logged in, try again</button>
					</div>
					<div id="debug" style="display:none;" class="debug"></div>
				</div>

				<script>
					// Debug function to show what's happening
					function showDebug(message) {
						const debugEl = document.getElementById('debug');
						debugEl.style.display = 'block';
						debugEl.innerHTML += message + "\n";
					}

					document.addEventListener('DOMContentLoaded', function() {
						// Add retryBtn event listener
						document.getElementById('retryBtn').addEventListener('click', function() {
							location.reload();
						});

						// Possible superuser token keys (different PocketBase versions use different formats)
						const possibleTokenKeys = [
							'_pb_superuser_auth_',      // Single underscore format
							'__pb_superuser_auth__',    // Double underscore format
							'pocketbase_superuser_auth' // No underscore format
						];
						
						// First try the known token keys directly
						let foundSuperuserToken = false;
						for (const tokenKey of possibleTokenKeys) {
							const token = localStorage.getItem(tokenKey);
							if (token) {
								showDebug("Found token with key: " + tokenKey);
								foundSuperuserToken = true;
								
								// Try to parse as JSON
								try {
									const tokenObj = JSON.parse(token);
									showDebug("- Parsed as JSON: " + JSON.stringify(tokenObj));
									
									if (tokenObj && tokenObj.token) {
										showDebug("- Using token.token property");
										tryAuth(tokenObj.token);
										return;
									}
								} catch (e) {
									showDebug("- Not valid JSON, using as raw string");
									tryAuth(token);
									return;
								}
							}
						}
						
						// If we didn't find any of the known token keys, scan all localStorage for PB tokens
						if (!foundSuperuserToken) {
							showDebug("No superuser token found with known keys, scanning all localStorage...");
							
							// Check all items in localStorage to find any PocketBase tokens
							let pbTokenFound = false;
							
							for (let i = 0; i < localStorage.length; i++) {
								const key = localStorage.key(i);
								
								// Look for PocketBase tokens (pb and auth in the name)
								if (key && (key.includes('pb') || key.includes('pocketbase')) && key.includes('auth')) {
									pbTokenFound = true;
									const value = localStorage.getItem(key);
									showDebug("Found possible token with key: " + key);
									
									// Try to parse it if it's JSON
									try {
										const parsed = JSON.parse(value);
										showDebug("- Parsed as JSON: " + JSON.stringify(parsed));
										
										// Check if it has a token property
										if (parsed && parsed.token) {
											// Check if it's a superuser token by checking the collection name or type
											const record = parsed.record || {};
											if (record.collectionName === '_superusers' || 
												key.includes('superuser') || 
												key.includes('admin')) {
												showDebug("- Looks like a superuser token!");
												tryAuth(parsed.token);
												return;
											} else {
												showDebug("- Not a superuser token, skipping");
											}
										}
									} catch (e) {
										showDebug("- Not valid JSON, trying as raw string");
										
										// If key name suggests it's a superuser token, try it directly
										if (key.includes('superuser') || key.includes('admin')) {
											tryAuth(value);
											return;
										}
									}
								}
							}
							
							if (!pbTokenFound) {
								showDebug("No PocketBase tokens found in localStorage.");
							} else {
								showDebug("Found PocketBase tokens but none were valid superuser tokens.");
							}
						}
						
						// Show error if we get here
						document.getElementById('loading').style.display = 'none';
						document.getElementById('error').style.display = 'block';
					});
					
					function tryAuth(tokenValue) {
						showDebug("Making fetch request with Authorization: Bearer " + tokenValue.substring(0, 10) + "...");
						
						// Make request to the current URL with proper Authorization header
						fetch(window.location.href, {
							method: 'GET',
							headers: {
								'Authorization': 'Bearer ' + tokenValue,
								'Accept': 'text/html'
							}
						})
						.then(response => {
							showDebug("Response status: " + response.status);
							if (!response.ok) {
								throw new Error('Authentication failed with status: ' + response.status);
							}
							return response.text();
						})
						.then(html => {
							// Check if the response contains an error message
							if (html.includes("Authentication Error")) {
								showDebug("Response contains error message, authentication likely failed");
								document.getElementById('loading').style.display = 'none';
								document.getElementById('error').style.display = 'block';
								return;
							}
							
							showDebug("Auth successful, replacing content");
							// Replace entire page with the response
							document.open();
							document.write(html);
							document.close();
						})
						.catch(error => {
							// Show error message
							showDebug("Fetch error: " + error.message);
							document.getElementById('loading').style.display = 'none';
							document.getElementById('error').style.display = 'block';
							document.getElementById('error').innerHTML += '<p>Error details: ' + error.message + '</p>';
						});
					}
				</script>
			</body>
			</html>
			`
			return c.HTML(http.StatusOK, html)
		}

		// User is authenticated, show the dashboard
		// Create a timeout context for stats collection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Collect system stats with context
		sysStats, err := monitoring.CollectSystemStats(ctx, s.stats.StartTime)
		if err != nil {
			return NewHTTPError("health_check", "Failed to collect system stats", http.StatusInternalServerError, err)
		}

		// Get analytics data if available
		var analyticsData *AnalyticsData
		if s.analytics != nil {
			analyticsData, _ = s.analytics.GetAnalyticsData()
		} else {
			analyticsData = defaultAnalyticsData()
		}

		// Prepare template data
		data := struct {
			Status           string
			UptimeDuration   string
			ServerStats      *ServerStats
			SystemStats      *monitoring.SystemStats
			AvgRequestTimeMs float64
			MemoryUsageStr   string
			DiskUsageStr     string
			LastCheckTime    time.Time
			RequestRate      float64
			AnalyticsData    *AnalyticsData
			PBAdminURL       string
		}{
			Status:           "Healthy",
			UptimeDuration:   time.Since(s.stats.StartTime).Round(time.Second).String(),
			ServerStats:      s.stats,
			SystemStats:      sysStats,
			AvgRequestTimeMs: float64(s.stats.AverageRequestTime.Load()) / 1e6,
			MemoryUsageStr:   fmt.Sprintf("%.2f/%.2f GB", float64(sysStats.MemoryInfo.Used)/1024/1024/1024, float64(sysStats.MemoryInfo.Total)/1024/1024/1024),
			DiskUsageStr:     fmt.Sprintf("%.2f/%.2f GB", float64(sysStats.DiskUsed)/1024/1024/1024, float64(sysStats.DiskTotal)/1024/1024/1024),
			LastCheckTime:    time.Now(),
			RequestRate:      float64(s.stats.TotalRequests.Load()) / time.Since(s.stats.StartTime).Seconds(),
			AnalyticsData:    analyticsData,
			PBAdminURL:       "/_/",
		}

		// Execute template
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return NewTemplateError("health_template_execution", "Failed to execute template", err)
		}

		return c.HTML(http.StatusOK, buf.String())
	}

	// Register the main health route
	e.Router.GET("/_/_", healthHandler)
}
