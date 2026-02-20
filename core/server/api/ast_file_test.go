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
