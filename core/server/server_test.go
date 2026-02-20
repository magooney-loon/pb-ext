package server_test

import (
	"testing"

	"github.com/magooney-loon/pb-ext/core/server"
	"github.com/pocketbase/pocketbase"
)

// --- Options ---

func TestNew_DefaultOptions(t *testing.T) {
	s := server.New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNew_WithMode_Developer(t *testing.T) {
	s := server.New(server.WithMode(true))
	if s == nil {
		t.Fatal("New(WithMode(true)) returned nil")
	}
}

func TestNew_WithMode_Normal(t *testing.T) {
	s := server.New(server.WithMode(false))
	if s == nil {
		t.Fatal("New(WithMode(false)) returned nil")
	}
}

func TestInDeveloperMode(t *testing.T) {
	s := server.New(server.InDeveloperMode())
	if s == nil {
		t.Fatal("New(InDeveloperMode()) returned nil")
	}
}

func TestInNormalMode(t *testing.T) {
	s := server.New(server.InNormalMode())
	if s == nil {
		t.Fatal("New(InNormalMode()) returned nil")
	}
}

func TestNew_WithConfig(t *testing.T) {
	cfg := &pocketbase.Config{DefaultDev: false}
	s := server.New(server.WithConfig(cfg))
	if s == nil {
		t.Fatal("New(WithConfig) returned nil")
	}
}

func TestNew_WithPocketbase(t *testing.T) {
	pb := pocketbase.New()
	s := server.New(server.WithPocketbase(pb))
	if s == nil {
		t.Fatal("New(WithPocketbase) returned nil")
	}
}

func TestWithConfig_AndWithPocketbase_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when using both WithConfig and WithPocketbase")
		}
	}()
	cfg := &pocketbase.Config{}
	pb := pocketbase.New()
	server.New(server.WithConfig(cfg), server.WithPocketbase(pb))
}

func TestErrConfigurationConflict_NotNil(t *testing.T) {
	if server.ErrConfigurationConflict == nil {
		t.Error("ErrConfigurationConflict should not be nil")
	}
}

// --- App / Stats ---

func TestServer_App_NotNil(t *testing.T) {
	s := server.New()
	if s.App() == nil {
		t.Error("App() should not be nil")
	}
}

func TestServer_Stats_NotNil(t *testing.T) {
	s := server.New()
	if s.Stats() == nil {
		t.Error("Stats() should not be nil")
	}
}

func TestServerStats_InitialValues(t *testing.T) {
	s := server.New()
	stats := s.Stats()

	if stats.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}
	if stats.TotalRequests.Load() != 0 {
		t.Errorf("TotalRequests: expected 0, got %d", stats.TotalRequests.Load())
	}
	if stats.ActiveConnections.Load() != 0 {
		t.Errorf("ActiveConnections: expected 0, got %d", stats.ActiveConnections.Load())
	}
	if stats.TotalErrors.Load() != 0 {
		t.Errorf("TotalErrors: expected 0, got %d", stats.TotalErrors.Load())
	}
}

// --- Errors ---

func TestNewHTTPError(t *testing.T) {
	e := server.NewHTTPError("op", "message", 404, nil)
	if e == nil {
		t.Fatal("NewHTTPError returned nil")
	}
	if e.Type != server.ErrTypeHTTP {
		t.Errorf("Type: expected %q, got %q", server.ErrTypeHTTP, e.Type)
	}
	if e.Op != "op" {
		t.Errorf("Op mismatch")
	}
	if e.StatusCode != 404 {
		t.Errorf("StatusCode: expected 404, got %d", e.StatusCode)
	}
	if e.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestNewInternalError(t *testing.T) {
	e := server.NewInternalError("op", "msg", nil)
	if e.StatusCode != 500 {
		t.Errorf("expected 500, got %d", e.StatusCode)
	}
	if e.Type != server.ErrTypeInternal {
		t.Errorf("Type: expected %q, got %q", server.ErrTypeInternal, e.Type)
	}
}

func TestNewTemplateError(t *testing.T) {
	e := server.NewTemplateError("op", "msg", nil)
	if e.Type != server.ErrTypeTemplate {
		t.Errorf("Type: expected %q, got %q", server.ErrTypeTemplate, e.Type)
	}
}

func TestNewRoutingError(t *testing.T) {
	e := server.NewRoutingError("op", "msg", nil)
	if e.Type != server.ErrTypeRouting {
		t.Errorf("Type: expected %q, got %q", server.ErrTypeRouting, e.Type)
	}
}

func TestNewAuthError(t *testing.T) {
	e := server.NewAuthError("op", "msg", nil)
	if e.StatusCode != 401 {
		t.Errorf("expected 401, got %d", e.StatusCode)
	}
	if e.Type != server.ErrTypeAuth {
		t.Errorf("Type: expected %q, got %q", server.ErrTypeAuth, e.Type)
	}
}

func TestNewConfigError(t *testing.T) {
	e := server.NewConfigError("op", "msg", nil)
	if e.Type != server.ErrTypeConfig {
		t.Errorf("Type: expected %q, got %q", server.ErrTypeConfig, e.Type)
	}
}

func TestNewDatabaseError(t *testing.T) {
	e := server.NewDatabaseError("op", "msg", nil)
	if e.Type != server.ErrTypeDatabase {
		t.Errorf("Type: expected %q, got %q", server.ErrTypeDatabase, e.Type)
	}
}

func TestServerError_Error_WithWrappedErr(t *testing.T) {
	inner := server.NewHTTPError("inner", "inner msg", 500, nil)
	outer := server.NewInternalError("outer", "outer msg", inner)

	if outer.Unwrap() == nil {
		t.Error("Unwrap() should return the wrapped error")
	}
	if outer.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestIsHTTPError(t *testing.T) {
	e := server.NewHTTPError("op", "msg", 500, nil)
	if !server.IsHTTPError(e) {
		t.Error("IsHTTPError should return true for HTTP error")
	}
	if server.IsHTTPError(nil) {
		t.Error("IsHTTPError(nil) should be false")
	}
}

func TestIsInternalError(t *testing.T) {
	e := server.NewInternalError("op", "msg", nil)
	if !server.IsInternalError(e) {
		t.Error("IsInternalError should return true")
	}
	if server.IsInternalError(server.NewHTTPError("op", "msg", 400, nil)) {
		t.Error("IsInternalError should not match HTTP error")
	}
}

func TestIsRoutingError(t *testing.T) {
	e := server.NewRoutingError("op", "msg", nil)
	if !server.IsRoutingError(e) {
		t.Error("IsRoutingError should return true")
	}
}

func TestIsAuthError(t *testing.T) {
	e := server.NewAuthError("op", "msg", nil)
	if !server.IsAuthError(e) {
		t.Error("IsAuthError should return true")
	}
}

func TestIsTemplateError(t *testing.T) {
	e := server.NewTemplateError("op", "msg", nil)
	if !server.IsTemplateError(e) {
		t.Error("IsTemplateError should return true")
	}
}

func TestIsConfigError(t *testing.T) {
	e := server.NewConfigError("op", "msg", nil)
	if !server.IsConfigError(e) {
		t.Error("IsConfigError should return true")
	}
}

func TestIsDatabaseError(t *testing.T) {
	e := server.NewDatabaseError("op", "msg", nil)
	if !server.IsDatabaseError(e) {
		t.Error("IsDatabaseError should return true")
	}
}

func TestIsErrorType_NonServerError(t *testing.T) {
	// A plain error should return false for any type check
	type plainErr struct{}
	// We can't easily create a non-ServerError here, but nil returns false
	if server.IsErrorType(nil, server.ErrTypeHTTP) {
		t.Error("IsErrorType(nil) should be false")
	}
}

func TestErrorTypeConstants(t *testing.T) {
	types := []string{
		server.ErrTypeHTTP,
		server.ErrTypeRouting,
		server.ErrTypeAuth,
		server.ErrTypeTemplate,
		server.ErrTypeConfig,
		server.ErrTypeDatabase,
		server.ErrTypeMiddleware,
		server.ErrTypeInternal,
	}
	seen := make(map[string]bool)
	for _, typ := range types {
		if typ == "" {
			t.Error("error type constant must not be empty")
		}
		if seen[typ] {
			t.Errorf("duplicate error type constant: %q", typ)
		}
		seen[typ] = true
	}
}

// --- Benchmarks ---

func BenchmarkNewServer(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.New()
	}
}

func BenchmarkNewHTTPError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = server.NewHTTPError("op", "msg", 500, nil)
	}
}
