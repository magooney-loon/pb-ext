package server

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/magooney-loon/pb-ext/core/alerts"
	"github.com/magooney-loon/pb-ext/core/analytics"
	"github.com/magooney-loon/pb-ext/core/audit"
	"github.com/magooney-loon/pb-ext/core/monitoring"
	"github.com/pocketbase/pocketbase/core"
)

// The dashboard reads a lot of optional, platform-dependent data: a container
// may expose no sensors, no addressed network interfaces, and a "/" that says
// nothing useful. These tests render the whole page against those shapes and
// assert it neither panics nor prints NaN/Inf, because in production the only
// thing standing between a bad metric and a broken dashboard is this template.

// renderDashboard executes index.tmpl and fails on any template error.
func renderDashboard(t *testing.T, data DashboardData) string {
	t.Helper()

	tmpl, err := parseDashboardTemplates()
	if err != nil {
		t.Fatalf("parseDashboardTemplates: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "index.tmpl", data); err != nil {
		t.Fatalf("rendering index.tmpl: %v", err)
	}
	return buf.String()
}

// assertNoBadNumbers catches division-by-zero and similar arithmetic leaking
// into the page.
func assertNoBadNumbers(t *testing.T, out string) {
	t.Helper()

	for _, bad := range []string{"NaN", "+Inf", "-Inf"} {
		if strings.Contains(out, bad) {
			t.Errorf("rendered dashboard contains %q", bad)
		}
	}
}

func baseDashboardData() DashboardData {
	return DashboardData{
		Status:           "Healthy",
		UptimeDuration:   "1h0m0s",
		ServerStats:      &ServerStats{StartTime: time.Now().Add(-time.Hour)},
		AvgRequestTimeMs: 1.5,
		MemoryUsageStr:   "9.00/31.10 GB",
		DiskUsageStr:     "214.90/1905.10 GB",
		LastCheckTime:    time.Now(),
		RequestRate:      2.5,
		AnalyticsData:    analytics.DefaultData(),
		AlertsData:       alerts.DefaultData(),
		AuditData:        audit.DefaultData(),
		PBAdminURL:       "/_/",
	}
}

// populatedAuditData exercises the admin access card: every event kind, a
// source rollup, and a dropped-event count.
func populatedAuditData() *audit.Data {
	return &audit.Data{
		Stats: audit.Stats{
			Enabled: true, Pending: 3, Dropped: 0, Written: 128,
			TotalEvents: 128, AuthFailures: 9, AuthSuccesses: 2,
			DeniedAttempts: 41, DistinctIPs: 6,
			LastFailure: time.Now().Add(-2 * time.Minute),
			LastSuccess: time.Now().Add(-3 * time.Hour),
		},
		Recent: []audit.Record{
			{Created: time.Now(), Kind: "auth_failure", Method: "POST", Path: "/api/collections/_superusers/auth-with-password",
				Status: 400, Outcome: "denied", AuthState: "anonymous", Identity: "admin@example.com", IP: "203.0.113.7", Count: 4},
			{Created: time.Now(), Kind: "auth_success", Method: "POST", Path: "/api/collections/_superusers/auth-with-password",
				Status: 200, Outcome: "allowed", AuthState: "superuser", Identity: "ops@example.com", IP: "198.51.100.4", Count: 1},
			{Created: time.Now(), Kind: "pbext_dashboard", Method: "GET", Path: "/_/_",
				Status: 200, Outcome: "denied", AuthState: "anonymous", IP: "203.0.113.99", Count: 17},
			{Created: time.Now(), Kind: "admin_api", Method: "PATCH", Path: "/api/collections/todos/records/x1", Query: "token=<redacted>",
				Status: 200, Outcome: "allowed", AuthState: "superuser", Identity: "ops@example.com", IP: "198.51.100.4", Count: 1},
		},
		TopIPs: []audit.IPSummary{
			{IP: "203.0.113.7", Events: 42, Failures: 9, LastSeen: time.Now()},
			{IP: "198.51.100.4", Events: 12, Successes: 2, LastSeen: time.Now()},
		},
	}
}

// populatedAlertsData exercises every branch of the alerts card: a configured
// transport, non-zero counters, a delivery error and history at each level.
func populatedAlertsData() *alerts.Data {
	return &alerts.Data{
		Stats: alerts.Stats{
			Enabled: true, Transport: "telegram", Target: "chat -100123",
			Instance: "prod-1", QueueSize: 256, Queued: 2,
			Sent: 41, Failed: 2, Dropped: 1, Suppressed: 7,
			Rules: 4, Firing: 1,
			LastSent:      time.Now().Add(-3 * time.Minute),
			LastError:     "sendMessage failed: 502 Bad Gateway",
			LastErrorTime: time.Now().Add(-time.Hour),
		},
		Recent: []alerts.Record{
			{Created: time.Now(), Level: "critical", Title: "Server recovered from an unexpected exit", Status: "sent"},
			{Created: time.Now(), Level: "error", Title: "Cron job failed", Status: "failed", Error: "timeout"},
			{Created: time.Now(), Level: "warn", Title: "CPU usage 94.0%", Status: "sent"},
			{Created: time.Now(), Level: "info", Title: "Server started", Status: "sent"},
		},
	}
}

func healthySystemStats() *monitoring.SystemStats {
	return &monitoring.SystemStats{
		Hostname:      "testhost",
		Platform:      "fedora",
		OS:            "linux",
		KernelVersion: "7.1.3",
		CPUInfo: []monitoring.CPUInfo{
			{ModelName: "Test CPU", Cores: 8, Frequency: 4900, Usage: 12.5, Temperature: 49},
			{ModelName: "Test CPU", Cores: 8, Frequency: 4900, Usage: 8.25, Temperature: 49},
		},
		MemoryInfo: monitoring.MemoryInfo{
			Total: 31 << 30, Used: 9 << 30, Free: 1 << 30,
			Available: 20 << 30, Cached: 21 << 30, UsedPercent: 29,
			SwapTotal: 8 << 30, SwapUsed: 1 << 30, SwapPercent: 12.5,
		},
		DiskPath: "./pb_data", DiskTotal: 1905 << 30, DiskUsed: 214 << 30,
		DiskFree: 1684 << 30, DiskUsagePercent: 11.3,
		RuntimeStats: monitoring.RuntimeStats{
			NumGoroutines: 42, NumCPU: 20, AllocatedBytes: 1 << 21,
			TotalAllocBytes: 1 << 24, HeapObjects: 9529, NextGC: 1 << 22,
			NumGC: 3, LastGCTime: time.Now(), LastGCDuration: 120 * time.Microsecond,
		},
		ProcessStats: monitoring.ProcessInfo{
			PID: 1234, CPUPercent: 1.5, MemoryPercent: 0.4,
			RSS: 40 << 20, VMS: 800 << 20, OpenFiles: 32, NumThreads: 12,
		},
		StartTime: time.Now().Add(-time.Hour), UptimeSecs: 3600,
		HasTempData: true,
		Temperatures: monitoring.TemperatureInfo{
			CPUTemp: 49, SystemTemp: 38, DiskTemp: 74.8, HasTempData: true,
		},
		NetworkInterfaces: []monitoring.NetworkInterface{
			{Name: "wlo1", IPAddress: "192.168.1.7", BytesSent: 1 << 32, BytesRecv: 1 << 34,
				PacketsSent: 1000, PacketsRecv: 5000},
		},
		NetworkConnections: 134,
		NetworkBytesSent:   1 << 32,
		NetworkBytesRecv:   1 << 34,
	}
}

func TestDashboard_RendersHealthyData(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)

	for _, want := range []string{"testhost", "wlo1", "49.0°C", "Test CPU"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered dashboard is missing %q", want)
		}
	}
}

func TestDashboard_RendersPopulatedAlerts(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()
	data.AlertsData = populatedAlertsData()

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)

	for _, want := range []string{"Cron job failed", "chat -100123", "Bad Gateway", "Active"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered dashboard is missing %q", want)
		}
	}
}

// The dashboard renders with text/template, which escapes nothing. Alert titles
// and delivery errors quote request paths and panic values, so they are
// attacker-influenced text landing in a page.
func TestDashboard_EscapesAlertText(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()
	data.AlertsData = populatedAlertsData()
	data.AlertsData.Recent[0].Title = `<script>alert(1)</script>`
	data.AlertsData.Recent[0].Text = `" onmouseover="alert(2)`
	data.AlertsData.LastError = `<img src=x onerror=alert(3)>`

	out := renderDashboard(t, data)

	for _, bad := range []string{"<script>alert(1)", `" onmouseover="`, "<img src=x"} {
		if strings.Contains(out, bad) {
			t.Errorf("unescaped alert text reached the page: %q", bad)
		}
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Error("expected the alert title to render escaped")
	}
}

// A nil AlertsData means the notifier never came up. That must degrade to a
// missing card, not a template error.
func TestDashboard_RendersWithNilAlerts(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()
	data.AlertsData = nil

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)
}

// The sidebar is wired from a single list in scripts/main.tmpl, and each entry
// needs a matching nav item and section in index.tmpl. Nothing at build time
// connects the three, so a tab added to one and not the others produces a
// sidebar button that does nothing, or content with no way to reach it — and
// the tab wiring bails out entirely if any section is missing, which would take
// the whole sidebar down rather than just the new tab.
func TestDashboard_EveryRegisteredTabHasMarkup(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()

	out := renderDashboard(t, data)

	entry := regexp.MustCompile(`\{ name: '([\w-]+)',\s*tab: '\.([\w-]+)',\s*section: '([\w-]+)'`)
	matches := entry.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		t.Fatal("found no tab definitions in the rendered page; has DASHBOARD_TABS been renamed?")
	}

	seen := map[string]bool{}
	for _, m := range matches {
		name, tabClass, sectionID := m[1], m[2], m[3]
		seen[name] = true

		if !strings.Contains(out, tabClass) {
			t.Errorf("tab %q is registered but no element carries the class %q", name, tabClass)
		}
		if !strings.Contains(out, `id="`+sectionID+`"`) {
			t.Errorf("tab %q is registered but there is no section with id %q", name, sectionID)
		}
		if !strings.Contains(out, `href="#`+name+`"`) {
			t.Errorf("tab %q is registered but no sidebar item links to #%s", name, name)
		}
	}

	// The tabs that must not silently disappear.
	for _, required := range []string{"health", "analytics", "alerts", "api", "cron", "rules"} {
		if !seen[required] {
			t.Errorf("the %q tab is no longer registered", required)
		}
	}
}

// The theme control cycles light → dark → auto through one button, so each mode
// needs its own glyph. Auto used to reuse the sun or the moon depending on what
// the system resolved to, which made it indistinguishable from an explicit
// choice — cycling into it looked like the button had failed.
//
// The icons come from the Remix set PocketBase's admin UI ships, which is
// loaded at runtime by the CSS piggyback, so a name that set does not define
// renders as an empty box with nothing to catch it. These are the three that
// exist in that bundle.
func TestDashboard_ThemeToggleHasADistinctIconPerMode(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()

	out := renderDashboard(t, data)

	for mode, icon := range map[string]string{
		"light": "ri-sun-line",
		"dark":  "ri-moon-line",
		"auto":  "ri-contrast-2-line",
	} {
		if !strings.Contains(out, icon) {
			t.Errorf("theme mode %q has no icon: %q is not referenced", mode, icon)
		}
	}

	// Auto must not share a glyph with either explicit mode.
	if strings.Contains(out, "auto: 'ri-sun-line'") || strings.Contains(out, "auto: 'ri-moon-line'") {
		t.Error("the auto theme reuses the light or dark icon; it needs its own")
	}
}

func TestDashboard_RendersAdminAccess(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()
	data.AuditData = populatedAuditData()

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)

	for _, want := range []string{"Admin Access", "admin@example.com", "203.0.113.7", "9 failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered dashboard is missing %q", want)
		}
	}
}

// Every identifying field on the audit card comes from the network: the user
// agent, the referer, the path, and the account name a login attempt supplied.
func TestDashboard_EscapesAuditText(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()
	data.AuditData = populatedAuditData()
	data.AuditData.Recent[0].Identity = `<script>alert(1)</script>`
	data.AuditData.Recent[0].UserAgent = `" onmouseover="alert(2)`
	data.AuditData.Recent[0].Path = `/_/<img src=x onerror=alert(3)>`
	data.AuditData.TopIPs[0].IP = `<b>1.2.3.4</b>`

	out := renderDashboard(t, data)

	for _, bad := range []string{"<script>alert(1)", `" onmouseover="`, "<img src=x", "<b>1.2.3.4</b>"} {
		if strings.Contains(out, bad) {
			t.Errorf("unescaped audit text reached the page: %q", bad)
		}
	}
}

// A nil AuditData means the auditor never came up.
func TestDashboard_RendersWithNilAudit(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()
	data.AuditData = nil

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)
}

// A disabled auditor must say it is not recording, rather than showing an empty
// log that reads as "nothing happened".
func TestDashboard_SaysWhenAuditingIsOff(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()
	data.AuditData = audit.DefaultData()

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)

	if !strings.Contains(out, "Not recording") {
		t.Error("the dashboard does not say that admin access auditing is off")
	}
}

// TestDashboard_RendersWithNoSensors is the case behind the "N/C" question:
// with no temperature data the page must say so rather than print 0.0°C.
func TestDashboard_RendersWithNoSensors(t *testing.T) {
	stats := healthySystemStats()
	stats.HasTempData = false
	stats.Temperatures = monitoring.TemperatureInfo{}
	for i := range stats.CPUInfo {
		stats.CPUInfo[i].Temperature = 0
	}

	data := baseDashboardData()
	data.SystemStats = stats

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)

	if strings.Contains(out, "0.0°C") {
		t.Error("rendered 0.0°C for an absent sensor; should read N/C")
	}
	if !strings.Contains(out, "N/C") {
		t.Error("expected N/C where no temperature is available")
	}
}

// TestDashboard_ShowsTemperatureWhenAvailable is the regression test for the
// CPU Details card reading N/C while System Metrics showed a real reading: the
// guard there used isset on a struct, which always reports false.
func TestDashboard_ShowsTemperatureWhenAvailable(t *testing.T) {
	stats := healthySystemStats()
	data := baseDashboardData()
	data.SystemStats = stats

	out := renderDashboard(t, data)

	if strings.Contains(out, "N/C") {
		t.Error("dashboard shows N/C even though CPU and system temperatures are present")
	}

	// Both cards read from the same source, so the reading must appear in both.
	if got := strings.Count(out, "49.0°C"); got < 2 {
		t.Errorf("CPU temperature rendered %d times, want it on both the System Metrics and CPU Details cards", got)
	}
}

// TestDashboard_RendersEmptyStats is the worst case: a collection where nothing
// succeeded. Every card must degrade rather than panic.
func TestDashboard_RendersEmptyStats(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = &monitoring.SystemStats{}

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)

	if !strings.Contains(out, "No data") {
		t.Error("expected the CPU card to fall back to its no-data branch")
	}
}

// TestDashboard_RendersWithNilSlices covers a partial collection: CPU and
// network failed, so their slices are nil while the rest is populated.
func TestDashboard_RendersWithNilSlices(t *testing.T) {
	stats := healthySystemStats()
	stats.CPUInfo = nil
	stats.NetworkInterfaces = nil

	data := baseDashboardData()
	data.SystemStats = stats

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)
}

// TestDashboard_RendersWithZeroTotals is the division-by-zero sweep: every
// denominator on the page is zero at once.
func TestDashboard_RendersWithZeroTotals(t *testing.T) {
	stats := healthySystemStats()
	stats.MemoryInfo = monitoring.MemoryInfo{}
	stats.DiskTotal, stats.DiskUsed, stats.DiskFree, stats.DiskUsagePercent = 0, 0, 0, 0
	stats.RuntimeStats = monitoring.RuntimeStats{}
	stats.ProcessStats = monitoring.ProcessInfo{}
	stats.NetworkBytesSent, stats.NetworkBytesRecv, stats.NetworkConnections = 0, 0, 0

	data := baseDashboardData()
	data.SystemStats = stats
	data.AnalyticsData = analytics.DefaultData()
	data.RequestRate = 0
	data.AvgRequestTimeMs = 0

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)
}

// TestDashboard_RendersWithNilAnalytics covers analytics failing to initialise.
func TestDashboard_RendersWithNilAnalytics(t *testing.T) {
	data := baseDashboardData()
	data.SystemStats = healthySystemStats()
	data.AnalyticsData = analytics.DefaultData()

	out := renderDashboard(t, data)
	assertNoBadNumbers(t, out)
}

// TestDashboard_RendersLoginTemplate covers the unauthenticated path.
func TestDashboard_RendersLoginTemplate(t *testing.T) {
	tmpl, err := parseDashboardTemplates()
	if err != nil {
		t.Fatalf("parseDashboardTemplates: %v", err)
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "login.tmpl", struct{ PBAdminURL string }{PBAdminURL: "/_/"})
	if err != nil {
		t.Fatalf("rendering login.tmpl: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("login template rendered nothing")
	}
}

// --- helper hardening ---

func TestIsset_HandlesStructs(t *testing.T) {
	fn, ok := templateFuncs["isset"].(func(interface{}, interface{}) (bool, error))
	if !ok {
		t.Fatal("isset missing or has an unexpected signature")
	}

	cpu := monitoring.CPUInfo{Temperature: 49}

	// The original implementation had no struct case and silently returned
	// false, which is what made the CPU Details card always read N/C.
	got, err := fn(cpu, "Temperature")
	if err != nil {
		t.Fatalf("isset on a struct: %v", err)
	}
	if !got {
		t.Error("isset(CPUInfo, \"Temperature\") = false; struct fields must be detected")
	}

	if got, _ := fn(cpu, "NoSuchField"); got {
		t.Error("isset reported a field that does not exist")
	}
	if got, _ := fn(&cpu, "Temperature"); !got {
		t.Error("isset should follow pointers")
	}

	var nilStats *monitoring.SystemStats
	if got, _ := fn(nilStats, "CPUInfo"); got {
		t.Error("isset on a nil pointer should be false, not a panic")
	}
	if got, _ := fn(nil, "anything"); got {
		t.Error("isset(nil, ...) should be false")
	}
}

func TestIsset_StillHandlesSlicesAndMaps(t *testing.T) {
	fn := templateFuncs["isset"].(func(interface{}, interface{}) (bool, error))

	slice := []monitoring.CPUInfo{{}, {}}
	if got, _ := fn(slice, 0); !got {
		t.Error("isset(slice, 0) = false")
	}
	if got, _ := fn(slice, 5); got {
		t.Error("isset(slice, 5) = true for a 2-element slice")
	}
	if got, _ := fn([]monitoring.CPUInfo{}, 0); got {
		t.Error("isset(empty slice, 0) = true")
	}

	m := map[string]float64{"a": 1}
	if got, _ := fn(m, "a"); !got {
		t.Error("isset(map, present key) = false")
	}
	if got, _ := fn(m, "b"); got {
		t.Error("isset(map, absent key) = true")
	}
}

func TestTemperatureHelpers_NilSafe(t *testing.T) {
	getDisk := templateFuncs["getDiskTemp"].(func(*monitoring.SystemStats) float64)
	getSystem := templateFuncs["getSystemTemp"].(func(*monitoring.SystemStats) float64)
	getAmbient := templateFuncs["getAmbientTemp"].(func(*monitoring.SystemStats) float64)
	getCPU := templateFuncs["getCPUTemp"].(func(*monitoring.SystemStats) float64)
	hasDisk := templateFuncs["hasDiskTemps"].(func(*monitoring.SystemStats) bool)

	// Must not panic on nil.
	if getDisk(nil) != 0 || getSystem(nil) != 0 || getAmbient(nil) != 0 || getCPU(nil) != 0 {
		t.Error("temperature helpers should return 0 for nil stats")
	}
	if hasDisk(nil) {
		t.Error("hasDiskTemps(nil) = true")
	}

	stats := healthySystemStats()
	if got := getCPU(stats); got != 49 {
		t.Errorf("getCPUTemp = %v, want 49", got)
	}
	if got := getDisk(stats); got != 74.8 {
		t.Errorf("getDiskTemp = %v, want 74.8", got)
	}
	if !hasDisk(stats) {
		t.Error("hasDiskTemps = false with a disk sensor present")
	}

	// Falls back to the classified sensor when per-CPU readings are absent.
	stats.CPUInfo = nil
	if got := getCPU(stats); got != 49 {
		t.Errorf("getCPUTemp with no CPUInfo = %v, want the classified 49", got)
	}
}

func TestRequestRate_GuardsZeroUptime(t *testing.T) {
	if got := requestRate(100, 0); got != 0 {
		t.Errorf("requestRate(100, 0) = %v, want 0 rather than +Inf", got)
	}
	if got := requestRate(0, 0); got != 0 {
		t.Errorf("requestRate(0, 0) = %v, want 0 rather than NaN", got)
	}
	if got := requestRate(100, -time.Second); got != 0 {
		t.Errorf("requestRate with negative uptime = %v, want 0", got)
	}
	if got := requestRate(100, 10*time.Second); got != 10 {
		t.Errorf("requestRate(100, 10s) = %v, want 10", got)
	}
}

func TestFormatCount(t *testing.T) {
	fn := templateFuncs["formatCount"].(func(uint64) string)

	cases := map[uint64]string{
		0: "0", 7: "7", 999: "999", 1000: "1,000",
		9529: "9,529", 1234567: "1,234,567", 1000000000: "1,000,000,000",
	}
	for in, want := range cases {
		if got := fn(in); got != want {
			t.Errorf("formatCount(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestAvgCPUUsage_EmptyAndNil(t *testing.T) {
	fn := templateFuncs["avgCPUUsage"].(func([]monitoring.CPUInfo) float64)

	if got := fn(nil); got != 0 {
		t.Errorf("avgCPUUsage(nil) = %v, want 0", got)
	}
	if got := fn([]monitoring.CPUInfo{}); got != 0 {
		t.Errorf("avgCPUUsage(empty) = %v, want 0", got)
	}
	if got := fn([]monitoring.CPUInfo{{Usage: 10}, {Usage: 20}}); got != 15 {
		t.Errorf("avgCPUUsage = %v, want 15", got)
	}
}

// --- panic containment ---

// TestRecoverDashboardPanic_ContainsPanics proves a panic anywhere in the
// dashboard turns into an error for that request rather than unwinding the
// handler, without depending on the embedding app wiring up SetupRecovery.
func TestRecoverDashboardPanic_ContainsPanics(t *testing.T) {
	handler := recoverDashboardPanic(nil, func(c *core.RequestEvent) error {
		panic("collector exploded")
	})

	err := handler(nil)
	if err == nil {
		t.Fatal("panic was swallowed without producing an error")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Errorf("err = %v, want it to record the panic", err)
	}
	if !strings.Contains(err.Error(), "collector exploded") {
		t.Errorf("err = %v, want the panic value preserved for debugging", err)
	}
}

func TestRecoverDashboardPanic_PassesThroughSuccess(t *testing.T) {
	called := false
	handler := recoverDashboardPanic(nil, func(c *core.RequestEvent) error {
		called = true
		return nil
	})

	if err := handler(nil); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if !called {
		t.Error("wrapped handler was not called")
	}
}

func TestRecoverDashboardPanic_PassesThroughErrors(t *testing.T) {
	sentinel := NewHTTPError("op", "boom", 500, nil)
	handler := recoverDashboardPanic(nil, func(c *core.RequestEvent) error {
		return sentinel
	})

	if err := handler(nil); err != sentinel {
		t.Errorf("err = %v, want the original error passed through unchanged", err)
	}
}
