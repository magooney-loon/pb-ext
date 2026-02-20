package api

import (
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// Cross-Package Struct Resolution Tests
// =============================================================================

func TestCrossPackageStructResolution(t *testing.T) {
	// Simulates a project structure:
	//   go.mod            (module testmod)
	//   handlers/api.go   (// API_SOURCE, imports "testmod/models", handler uses models.Item)
	//   models/types.go   (defines Item struct)
	//
	// After DiscoverSourceFiles, the parser should automatically follow the import
	// and parse Item from models/types.go.

	tmpDir := t.TempDir()

	// Create go.mod
	goMod := "module testmod\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create models/ directory with struct definitions
	modelsDir := filepath.Join(tmpDir, "models")
	if err := os.MkdirAll(modelsDir, 0755); err != nil {
		t.Fatalf("Failed to create models dir: %v", err)
	}

	modelsContent := `package models

type Item struct {
	ID    string  ` + "`json:\"id\"`" + `
	Name  string  ` + "`json:\"name\"`" + `
	Price float64 ` + "`json:\"price\"`" + `
}

type Category struct {
	ID   string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(modelsDir, "types.go"), []byte(modelsContent), 0644); err != nil {
		t.Fatalf("Failed to create models/types.go: %v", err)
	}

	// Create handlers/ directory with API_SOURCE file that imports models
	handlersDir := filepath.Join(tmpDir, "handlers")
	if err := os.MkdirAll(handlersDir, 0755); err != nil {
		t.Fatalf("Failed to create handlers dir: %v", err)
	}

	handlersContent := `package handlers

// API_SOURCE

import (
	"testmod/models"
	"github.com/pocketbase/pocketbase/core"
)

// API_DESC List items
// API_TAGS Items
func listItemsHandler(c *core.RequestEvent) error {
	items := []models.Item{}
	return c.JSON(200, items)
}

// API_DESC Get item by ID
// API_TAGS Items
func getItemHandler(c *core.RequestEvent) error {
	item := models.Item{ID: "1", Name: "Widget", Price: 9.99}
	return c.JSON(200, item)
}
`
	if err := os.WriteFile(filepath.Join(handlersDir, "api.go"), []byte(handlersContent), 0644); err != nil {
		t.Fatalf("Failed to create handlers/api.go: %v", err)
	}

	// Change to tmp directory so DiscoverSourceFiles can find go.mod and walk "."
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	err := parser.DiscoverSourceFiles()
	if err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	// Verify handlers from the API_SOURCE file were discovered
	allHandlers := parser.GetAllHandlers()
	if len(allHandlers) != 2 {
		t.Errorf("Expected 2 handlers, got %d", len(allHandlers))
	}
	if _, ok := allHandlers["listItemsHandler"]; !ok {
		t.Error("Expected listItemsHandler to be discovered")
	}
	if _, ok := allHandlers["getItemHandler"]; !ok {
		t.Error("Expected getItemHandler to be discovered")
	}

	// Verify structs from the imported models package were auto-parsed
	allStructs := parser.GetAllStructs()
	if _, ok := allStructs["Item"]; !ok {
		t.Error("Expected Item struct from models package to be auto-parsed")
	}
	if _, ok := allStructs["Category"]; !ok {
		t.Error("Expected Category struct from models package to be auto-parsed")
	}

	// Verify Item struct has correct fields
	itemStruct, _ := parser.GetStructByName("Item")
	if itemStruct == nil {
		t.Fatal("Expected Item struct to exist")
	}
	if len(itemStruct.Fields) != 3 {
		t.Errorf("Expected Item to have 3 fields, got %d", len(itemStruct.Fields))
	}

	// Verify Item struct has a valid JSON schema
	if itemStruct.JSONSchema == nil {
		t.Fatal("Expected Item struct to have JSONSchema generated")
	}
	if itemStruct.JSONSchema.Type != "object" {
		t.Errorf("Expected Item schema type 'object', got %q", itemStruct.JSONSchema.Type)
	}
	for _, field := range []string{"id", "name", "price"} {
		if _, ok := itemStruct.JSONSchema.Properties[field]; !ok {
			t.Errorf("Expected Item schema to have property %q", field)
		}
	}

	// Verify module path was detected
	if parser.modulePath != "testmod" {
		t.Errorf("Expected modulePath 'testmod', got %q", parser.modulePath)
	}

	// Verify the models directory was tracked as parsed
	if !parser.parsedDirs["models"] {
		t.Error("Expected 'models' directory to be tracked in parsedDirs")
	}
}

func TestCrossPackageStructResolution_NoGoMod(t *testing.T) {
	// When there's no go.mod, import following should be silently disabled
	tmpDir := t.TempDir()

	// Only create the handler file, no go.mod
	handlersContent := `package main

// API_SOURCE

import "github.com/pocketbase/pocketbase/core"

// API_DESC Simple handler
// API_TAGS Test
func simpleHandler(c *core.RequestEvent) error {
	return c.JSON(200, map[string]any{"ok": true})
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "api.go"), []byte(handlersContent), 0644); err != nil {
		t.Fatalf("Failed to create api.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	err := parser.DiscoverSourceFiles()
	if err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	// Should still discover handlers (basic functionality unaffected)
	allHandlers := parser.GetAllHandlers()
	if len(allHandlers) != 1 {
		t.Errorf("Expected 1 handler, got %d", len(allHandlers))
	}

	// Module path should be empty
	if parser.modulePath != "" {
		t.Errorf("Expected empty modulePath without go.mod, got %q", parser.modulePath)
	}
}

func TestCrossPackageStructResolution_NestedImports(t *testing.T) {
	// Test that deeply nested import paths are resolved correctly
	// e.g., "testmod/internal/domain/models"
	tmpDir := t.TempDir()

	goMod := "module testmod\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create deeply nested models directory
	deepDir := filepath.Join(tmpDir, "internal", "domain", "models")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("Failed to create deep dir: %v", err)
	}

	modelsContent := `package models

type DeepItem struct {
	ID   string ` + "`json:\"id\"`" + `
	Data string ` + "`json:\"data\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(deepDir, "types.go"), []byte(modelsContent), 0644); err != nil {
		t.Fatalf("Failed to create types.go: %v", err)
	}

	// Create handler importing the deep package
	handlersContent := `package main

// API_SOURCE

import (
	"testmod/internal/domain/models"
	"github.com/pocketbase/pocketbase/core"
)

// API_DESC Get deep item
// API_TAGS Test
func getDeepItemHandler(c *core.RequestEvent) error {
	item := models.DeepItem{ID: "1"}
	return c.JSON(200, item)
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "api.go"), []byte(handlersContent), 0644); err != nil {
		t.Fatalf("Failed to create api.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	err := parser.DiscoverSourceFiles()
	if err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	// Verify DeepItem was auto-parsed from the nested import
	allStructs := parser.GetAllStructs()
	if _, ok := allStructs["DeepItem"]; !ok {
		t.Error("Expected DeepItem struct from deeply nested import to be auto-parsed")
	}

	// Verify the nested directory was tracked
	expectedDir := filepath.FromSlash("internal/domain/models")
	if !parser.parsedDirs[expectedDir] {
		t.Errorf("Expected %q to be in parsedDirs", expectedDir)
	}
}

func TestCrossPackageStructResolution_ExternalImportsIgnored(t *testing.T) {
	// Verify that external (non-module) imports are NOT followed
	tmpDir := t.TempDir()

	goMod := "module testmod\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	handlersContent := `package main

// API_SOURCE

import (
	"fmt"
	"net/http"
	"github.com/pocketbase/pocketbase/core"
	"github.com/some-external/library/models"
)

// API_DESC Test handler
// API_TAGS Test
func testExternalHandler(c *core.RequestEvent) error {
	fmt.Println("test")
	_ = http.StatusOK
	return c.JSON(200, map[string]any{"ok": true})
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "api.go"), []byte(handlersContent), 0644); err != nil {
		t.Fatalf("Failed to create api.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	err := parser.DiscoverSourceFiles()
	if err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	// Only the handler dir should be in parsedDirs, not any external package dirs
	for dir := range parser.parsedDirs {
		if dir == "." {
			continue
		}
		t.Errorf("Unexpected directory in parsedDirs: %q (should only contain API_SOURCE dirs)", dir)
	}
}

func TestCrossPackageStructResolution_NoDuplicateParsing(t *testing.T) {
	// Verify that a directory already parsed via API_SOURCE is not re-parsed
	tmpDir := t.TempDir()

	goMod := "module testmod\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create shared package
	sharedDir := filepath.Join(tmpDir, "shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("Failed to create shared dir: %v", err)
	}

	// shared/ has BOTH API_SOURCE and struct definitions
	sharedContent := `package shared

// API_SOURCE

import "github.com/pocketbase/pocketbase/core"

type SharedItem struct {
	ID string ` + "`json:\"id\"`" + `
}

// API_DESC Get shared item
// API_TAGS Shared
func getSharedHandler(c *core.RequestEvent) error {
	return c.JSON(200, SharedItem{ID: "1"})
}
`
	if err := os.WriteFile(filepath.Join(sharedDir, "api.go"), []byte(sharedContent), 0644); err != nil {
		t.Fatalf("Failed to create shared/api.go: %v", err)
	}

	// Another file imports from shared
	mainContent := `package main

// API_SOURCE

import (
	"testmod/shared"
	"github.com/pocketbase/pocketbase/core"
)

// API_DESC Use shared
// API_TAGS Main
func useSharedHandler(c *core.RequestEvent) error {
	item := shared.SharedItem{ID: "2"}
	return c.JSON(200, item)
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	err := parser.DiscoverSourceFiles()
	if err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	// Should have both handlers
	allHandlers := parser.GetAllHandlers()
	if len(allHandlers) != 2 {
		t.Errorf("Expected 2 handlers, got %d", len(allHandlers))
	}

	// SharedItem should exist exactly once (not duplicated)
	allStructs := parser.GetAllStructs()
	if _, ok := allStructs["SharedItem"]; !ok {
		t.Error("Expected SharedItem struct to be discovered")
	}

	// The shared directory should be in parsedDirs (marked during ParseFile of shared/api.go)
	if !parser.parsedDirs["shared"] {
		t.Error("Expected 'shared' to be in parsedDirs")
	}
}

// =============================================================================
// Cross-File Helper Parameter Detection Tests
// =============================================================================

// TestSiblingFileHelperParams verifies that a domain helper (e.g. parseTimeParams)
// defined in a sibling file WITHOUT // API_SOURCE still has its params registered
// in funcParamSchemas and propagated to handlers that call it.
func TestSiblingFileHelperParams_DomainHelper(t *testing.T) {
	tmpDir := t.TempDir()

	// helpers.go — no API_SOURCE directive
	helpersContent := `package app

import "github.com/pocketbase/pocketbase/core"

type timeParams struct {
	Interval string
	From     string
	To       string
	Limit    string
}

func parseTimeParams(e *core.RequestEvent) timeParams {
	q := e.Request.URL.Query()
	return timeParams{
		Interval: q.Get("interval"),
		From:     q.Get("from"),
		To:       q.Get("to"),
		Limit:    q.Get("limit"),
	}
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "helpers.go"), []byte(helpersContent), 0644); err != nil {
		t.Fatalf("Failed to create helpers.go: %v", err)
	}

	// handlers.go — has API_SOURCE, calls the helper
	handlersContent := `package app

// API_SOURCE

import "github.com/pocketbase/pocketbase/core"

// API_DESC Get candles
// API_TAGS Market
func getCandlesHandler(c *core.RequestEvent) error {
	p := parseTimeParams(c)
	_ = p
	return c.JSON(200, map[string]any{"ok": true})
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "handlers.go"), []byte(handlersContent), 0644); err != nil {
		t.Fatalf("Failed to create handlers.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	if err := parser.DiscoverSourceFiles(); err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	// parseTimeParams should be registered in funcParamSchemas
	tp, ok := parser.funcParamSchemas["parseTimeParams"]
	if !ok {
		t.Fatal("Expected parseTimeParams from sibling file to be registered in funcParamSchemas")
	}
	names := make(map[string]bool)
	for _, p := range tp {
		names[p.Name] = true
	}
	for _, want := range []string{"interval", "from", "to", "limit"} {
		if !names[want] {
			t.Errorf("Expected parseTimeParams to include param %q", want)
		}
	}

	// getCandlesHandler should inherit all time params
	h, ok := parser.GetHandlerByName("getCandlesHandler")
	if !ok || h == nil {
		t.Fatal("Expected getCandlesHandler to be discovered")
	}
	pm := paramMap(h.Parameters)
	assertParam(t, pm, "interval", "query", "string")
	assertParam(t, pm, "from", "query", "string")
	assertParam(t, pm, "to", "query", "string")
	assertParam(t, pm, "limit", "query", "string")
}

// TestSiblingFileHelperParams_GenericHelper verifies that a generic helper
// (e.g. parseIntParam(e, name, default)) in a sibling non-API_SOURCE file
// is registered as a sentinel and its params are resolved from the call site.
func TestSiblingFileHelperParams_GenericHelper(t *testing.T) {
	tmpDir := t.TempDir()

	// helpers.go — no API_SOURCE
	helpersContent := `package app

import (
	"strconv"
	"github.com/pocketbase/pocketbase/core"
)

func parseIntParam(e *core.RequestEvent, name string, def int) int {
	v := e.Request.URL.Query().Get(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "helpers.go"), []byte(helpersContent), 0644); err != nil {
		t.Fatalf("Failed to create helpers.go: %v", err)
	}

	// handlers.go — has API_SOURCE
	handlersContent := `package app

// API_SOURCE

import "github.com/pocketbase/pocketbase/core"

// API_DESC List items paginated
// API_TAGS Items
func listItemsPaginatedHandler(c *core.RequestEvent) error {
	page := parseIntParam(c, "page", 1)
	size := parseIntParam(c, "page_size", 20)
	_, _ = page, size
	return c.JSON(200, map[string]any{"ok": true})
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "handlers.go"), []byte(handlersContent), 0644); err != nil {
		t.Fatalf("Failed to create handlers.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	if err := parser.DiscoverSourceFiles(); err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	// parseIntParam should be registered as a sentinel
	ip, ok := parser.funcParamSchemas["parseIntParam"]
	if !ok {
		t.Fatal("Expected parseIntParam from sibling file to be registered in funcParamSchemas")
	}
	hasSentinel := false
	for _, p := range ip {
		if p.Name == "" && p.Source == "query" {
			hasSentinel = true
		}
	}
	if !hasSentinel {
		t.Error("Expected parseIntParam to have a query sentinel entry")
	}

	// listItemsPaginatedHandler should resolve params from call site string literals
	h, ok := parser.GetHandlerByName("listItemsPaginatedHandler")
	if !ok || h == nil {
		t.Fatal("Expected listItemsPaginatedHandler to be discovered")
	}
	pm := paramMap(h.Parameters)
	assertParam(t, pm, "page", "query", "string")
	assertParam(t, pm, "page_size", "query", "string")
}

// TestSiblingFileHelperParams_MultipleHelperFiles verifies that helpers spread
// across multiple sibling files are all discovered correctly.
func TestSiblingFileHelperParams_MultipleHelperFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// time_helpers.go — parseTimeParams
	timeHelpersContent := `package app

import "github.com/pocketbase/pocketbase/core"

type timeParams struct { Interval, From, To string }

func parseTimeParams(e *core.RequestEvent) timeParams {
	q := e.Request.URL.Query()
	return timeParams{
		Interval: q.Get("interval"),
		From:     q.Get("from"),
		To:       q.Get("to"),
	}
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "time_helpers.go"), []byte(timeHelpersContent), 0644); err != nil {
		t.Fatalf("Failed to create time_helpers.go: %v", err)
	}

	// bool_helpers.go — parseBoolParam
	boolHelpersContent := `package app

import (
	"github.com/pocketbase/pocketbase/core"
)

func parseBoolParam(e *core.RequestEvent, name string) bool {
	return e.Request.URL.Query().Get(name) == "true"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "bool_helpers.go"), []byte(boolHelpersContent), 0644); err != nil {
		t.Fatalf("Failed to create bool_helpers.go: %v", err)
	}

	// handlers.go — API_SOURCE, calls both helpers
	handlersContent := `package app

// API_SOURCE

import "github.com/pocketbase/pocketbase/core"

// API_DESC Get chart
// API_TAGS Chart
func getChartFullHandler(c *core.RequestEvent) error {
	p := parseTimeParams(c)
	verbose := parseBoolParam(c, "verbose")
	_, _ = p, verbose
	return c.JSON(200, map[string]any{"ok": true})
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "handlers.go"), []byte(handlersContent), 0644); err != nil {
		t.Fatalf("Failed to create handlers.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	if err := parser.DiscoverSourceFiles(); err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	h, ok := parser.GetHandlerByName("getChartFullHandler")
	if !ok || h == nil {
		t.Fatal("Expected getChartFullHandler to be discovered")
	}

	pm := paramMap(h.Parameters)
	// From parseTimeParams
	assertParam(t, pm, "interval", "query", "string")
	assertParam(t, pm, "from", "query", "string")
	assertParam(t, pm, "to", "query", "string")
	// From parseBoolParam call site
	assertParam(t, pm, "verbose", "query", "string")
}

// TestSiblingFileHelperParams_APISourceFileHelpersStillWork verifies that helpers
// defined IN an API_SOURCE file (not siblings) still work correctly after the refactor.
func TestSiblingFileHelperParams_APISourceFileHelpersStillWork(t *testing.T) {
	tmpDir := t.TempDir()

	// Single file — API_SOURCE, has both helper and handler
	content := `package app

// API_SOURCE

import "github.com/pocketbase/pocketbase/core"

func parseQueryHelper(e *core.RequestEvent) string {
	return e.Request.URL.Query().Get("q")
}

// API_DESC Search
// API_TAGS Search
func searchHandler(c *core.RequestEvent) error {
	q := parseQueryHelper(c)
	_ = q
	return c.JSON(200, map[string]any{"ok": true})
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "api.go"), []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create api.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	if err := parser.DiscoverSourceFiles(); err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	h, ok := parser.GetHandlerByName("searchHandler")
	if !ok || h == nil {
		t.Fatal("Expected searchHandler to be discovered")
	}

	pm := paramMap(h.Parameters)
	assertParam(t, pm, "q", "query", "string")
}

// =============================================================================
// *http.Request Helper Detection Tests
// =============================================================================

// TestHTTPRequestHelper_HeaderGet verifies that a helper taking *http.Request directly
// and using r.Header.Get("name") is detected and propagated to calling handlers.
func TestHTTPRequestHelper_HeaderGet(t *testing.T) {
	tmpDir := t.TempDir()

	// helpers.go — takes *http.Request, not *core.RequestEvent
	helpersContent := `package app

import "net/http"

func getAuthHeader(r *http.Request) string {
	return r.Header.Get("Authorization")
}

func getPaymentSig(r *http.Request) string {
	return r.Header.Get("PAYMENT-SIGNATURE")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "helpers.go"), []byte(helpersContent), 0644); err != nil {
		t.Fatalf("Failed to create helpers.go: %v", err)
	}

	// handlers.go — API_SOURCE, handler calls the helpers via e.Request
	handlersContent := `package app

// API_SOURCE

import (
	"net/http"
	"github.com/pocketbase/pocketbase/core"
)

// API_DESC Purchase access
// API_TAGS Access
func purchaseHandler(c *core.RequestEvent) error {
	auth := getAuthHeader(c.Request)
	sig := getPaymentSig(c.Request)
	_, _ = auth, sig
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "handlers.go"), []byte(handlersContent), 0644); err != nil {
		t.Fatalf("Failed to create handlers.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	if err := parser.DiscoverSourceFiles(); err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	// Both helpers should be registered
	if _, ok := parser.funcParamSchemas["getAuthHeader"]; !ok {
		t.Error("Expected getAuthHeader to be registered in funcParamSchemas")
	}
	if _, ok := parser.funcParamSchemas["getPaymentSig"]; !ok {
		t.Error("Expected getPaymentSig to be registered in funcParamSchemas")
	}

	// purchaseHandler should inherit both header params
	h, ok := parser.GetHandlerByName("purchaseHandler")
	if !ok || h == nil {
		t.Fatal("Expected purchaseHandler to be discovered")
	}
	pm := paramMap(h.Parameters)
	assertParam(t, pm, "Authorization", "header", "string")
	assertParam(t, pm, "PAYMENT-SIGNATURE", "header", "string")
}

// TestHTTPRequestHelper_QueryGet verifies that a helper taking *http.Request
// and using r.URL.Query().Get("name") or q := r.URL.Query(); q.Get("name")
// is detected and propagated to calling handlers.
func TestHTTPRequestHelper_QueryGet(t *testing.T) {
	tmpDir := t.TempDir()

	helpersContent := `package app

import "net/http"

func getETag(r *http.Request) string {
	return r.Header.Get("If-None-Match")
}

func getPageParam(r *http.Request) string {
	return r.URL.Query().Get("page")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "helpers.go"), []byte(helpersContent), 0644); err != nil {
		t.Fatalf("Failed to create helpers.go: %v", err)
	}

	handlersContent := `package app

// API_SOURCE

import (
	"net/http"
	"github.com/pocketbase/pocketbase/core"
)

// API_DESC List with pagination
// API_TAGS List
func listHandler(c *core.RequestEvent) error {
	etag := getETag(c.Request)
	page := getPageParam(c.Request)
	_, _ = etag, page
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "handlers.go"), []byte(handlersContent), 0644); err != nil {
		t.Fatalf("Failed to create handlers.go: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	parser := NewASTParser()
	if err := parser.DiscoverSourceFiles(); err != nil {
		t.Fatalf("DiscoverSourceFiles failed: %v", err)
	}

	h, ok := parser.GetHandlerByName("listHandler")
	if !ok || h == nil {
		t.Fatal("Expected listHandler to be discovered")
	}
	pm := paramMap(h.Parameters)
	assertParam(t, pm, "If-None-Match", "header", "string")
	assertParam(t, pm, "page", "query", "string")
}
