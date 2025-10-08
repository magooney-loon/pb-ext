package api

// API Version Manager - Multi-version API Documentation System
//
// This module provides support for multiple simultaneous API versions,
// allowing developers to work on new versions while keeping old ones active.
//
// Features:
//   - Multiple concurrent API versions with separate registries
//   - Version-specific documentation systems
//   - Automatic version routing
//   - Clean separation of version concerns
//
// Example Usage:
//   versions := map[string]*APIDocsConfig{
//       "v1": v1Config,
//       "v2": v2Config,
//   }
//   manager := InitializeVersionedSystem(versions, "v1")
//
//   v1Router, _ := manager.GetVersionRouter("v1", e)
//   v2Router, _ := manager.GetVersionRouter("v2", e)
//
//   v1Router.GET("/api/v1/users", handler)
//   v2Router.GET("/api/v2/users", handler)

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Core Types
// =============================================================================

// APIVersionManager manages multiple API versions with separate registries
type APIVersionManager struct {
	mu             sync.RWMutex
	versions       []string                  // ordered list of versions
	defaultVersion string                    // default version to use
	registries     map[string]*APIRegistry   // separate registry per version
	configs        map[string]*APIDocsConfig // version-specific configs
	createdAt      time.Time                 // when manager was created
	lastModified   time.Time                 // last time versions were modified
}

// VersionInfo contains information about a specific API version
type VersionInfo struct {
	Version   string         `json:"version"`
	Status    string         `json:"status"` // "stable", "development", "deprecated"
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Config    *APIDocsConfig `json:"config"`
	Stats     map[string]int `json:"stats"`
	Endpoints int            `json:"endpoints"`
}

// VersionedAPIRouter provides version-specific route registration
type VersionedAPIRouter struct {
	*AutoAPIRouter
	version  string
	manager  *APIVersionManager
	registry *APIRegistry // version-specific registry
}

// InitializeVersionedSystem initializes a versioned documentation system
func InitializeVersionedSystem(versions map[string]*APIDocsConfig, defaultVersion string) *APIVersionManager {
	return InitializeVersionManager(versions, defaultVersion)
}

// =============================================================================
// Configuration Utilities
// =============================================================================

// ValidateConfiguration validates an API documentation configuration
func ValidateConfiguration(config *APIDocsConfig) []string {
	var errors []string

	if config == nil {
		errors = append(errors, "configuration is nil")
		return errors
	}

	if config.Title == "" {
		errors = append(errors, "title is required")
	}
	if config.Version == "" {
		errors = append(errors, "version is required")
	}
	if config.BaseURL == "" {
		errors = append(errors, "base_url is required")
	}

	return errors
}

// =============================================================================
// Constructor Functions
// =============================================================================

// NewAPIVersionManager creates a new version manager
func NewAPIVersionManager() *APIVersionManager {
	return &APIVersionManager{
		versions:     []string{},
		registries:   make(map[string]*APIRegistry),
		configs:      make(map[string]*APIDocsConfig),
		createdAt:    time.Now(),
		lastModified: time.Now(),
	}
}

// NewAPIVersionManagerWithDefault creates a version manager with a default version
func NewAPIVersionManagerWithDefault(defaultVersion string) *APIVersionManager {
	vm := NewAPIVersionManager()
	vm.defaultVersion = defaultVersion
	return vm
}

// =============================================================================
// Version Management
// =============================================================================

// RegisterVersion registers a new API version with its own registry
func (vm *APIVersionManager) RegisterVersion(version string, config *APIDocsConfig) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Validate version string
	if err := ValidateVersionString(version); err != nil {
		return err
	}

	// Check if version already exists
	if _, exists := vm.configs[version]; exists {
		return fmt.Errorf("version %s already exists", version)
	}

	// Create version-specific registry with its own AST parser and schema generator
	astParser := NewASTParser()
	schemaGenerator := NewSchemaGenerator(astParser)
	registry := NewAPIRegistry(config, astParser, schemaGenerator)

	// Store version information
	vm.versions = append(vm.versions, version)
	vm.registries[version] = registry
	vm.configs[version] = config
	vm.lastModified = time.Now()

	// Set as default if it's the first version
	if vm.defaultVersion == "" {
		vm.defaultVersion = version
	}

	// Sort versions to maintain consistent ordering
	sort.Strings(vm.versions)

	return nil
}

// RemoveVersion removes a version and its registry
func (vm *APIVersionManager) RemoveVersion(version string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Check if version exists
	if _, exists := vm.configs[version]; !exists {
		return fmt.Errorf("version %s does not exist", version)
	}

	// Cannot remove the default version
	if version == vm.defaultVersion {
		return fmt.Errorf("cannot remove default version %s", version)
	}

	// Remove from all maps and slices
	delete(vm.configs, version)
	delete(vm.registries, version)

	for i, v := range vm.versions {
		if v == version {
			vm.versions = append(vm.versions[:i], vm.versions[i+1:]...)
			break
		}
	}

	vm.lastModified = time.Now()
	return nil
}

// GetVersionConfig returns the configuration for a specific version
func (vm *APIVersionManager) GetVersionConfig(version string) (*APIDocsConfig, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if config, exists := vm.configs[version]; exists {
		return config, nil
	}
	return nil, fmt.Errorf("version %s not found", version)
}

// GetVersionRegistry returns the registry for a specific version
func (vm *APIVersionManager) GetVersionRegistry(version string) (*APIRegistry, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if registry, exists := vm.registries[version]; exists {
		return registry, nil
	}
	return nil, fmt.Errorf("registry for version %s not found", version)
}

// GetVersionRouter creates a versioned API router for the specified version
func (vm *APIVersionManager) GetVersionRouter(version string, e *core.ServeEvent) (*VersionedAPIRouter, error) {
	// Get version-specific registry
	registry, err := vm.GetVersionRegistry(version)
	if err != nil {
		return nil, err
	}

	// Create auto router with version-specific registry
	autoRouter := &AutoAPIRouter{
		router:   e.Router,
		registry: registry, // Use version-specific registry!
	}

	return &VersionedAPIRouter{
		AutoAPIRouter: autoRouter,
		version:       version,
		manager:       vm,
		registry:      registry, // Store reference for easy access
	}, nil
}

// =============================================================================
// Version Information
// =============================================================================

// GetDefaultVersion returns the default version
func (vm *APIVersionManager) GetDefaultVersion() string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.defaultVersion
}

// SetDefaultVersion sets the default API version
func (vm *APIVersionManager) SetDefaultVersion(version string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if _, exists := vm.configs[version]; !exists {
		return fmt.Errorf("version %s does not exist", version)
	}

	vm.defaultVersion = version
	vm.lastModified = time.Now()
	return nil
}

// GetAllVersions returns all registered versions
func (vm *APIVersionManager) GetAllVersions() []string {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make([]string, len(vm.versions))
	copy(result, vm.versions)
	return result
}

// GetVersionInfo returns detailed information about a specific version
func (vm *APIVersionManager) GetVersionInfo(version string) (*VersionInfo, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	config, exists := vm.configs[version]
	if !exists {
		return nil, fmt.Errorf("version %s not found", version)
	}

	registry, exists := vm.registries[version]
	if !exists {
		return nil, fmt.Errorf("registry for version %s not found", version)
	}

	// Get endpoints from version-specific registry
	docs := registry.GetDocs()
	endpoints := docs.Endpoints

	// Use configured status or default to "stable"
	status := config.Status
	if status == "" {
		status = "stable"
	}

	return &VersionInfo{
		Version:   version,
		Status:    status,
		CreatedAt: vm.createdAt,
		UpdatedAt: vm.lastModified,
		Config:    config,
		Stats:     calculateVersionStats(endpoints),
		Endpoints: len(endpoints),
	}, nil
}

// GetAllVersionsInfo returns information about all versions
func (vm *APIVersionManager) GetAllVersionsInfo() ([]*VersionInfo, error) {
	versions := vm.GetAllVersions()
	infos := make([]*VersionInfo, 0, len(versions))

	for _, version := range versions {
		info, err := vm.GetVersionInfo(version)
		if err != nil {
			continue // Skip versions with errors
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// =============================================================================
// HTTP Handlers
// =============================================================================

// RegisterWithServer registers version management endpoints
func (vm *APIVersionManager) RegisterWithServer(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Version listing endpoint
		e.Router.GET("/api/docs/versions", func(c *core.RequestEvent) error {
			return vm.VersionsHandler(c)
		}).Bind(apis.RequireSuperuserAuth())

		// Debug AST endpoint
		e.Router.GET("/api/docs/debug/ast", func(c *core.RequestEvent) error {
			// Check authentication
			if c.Auth == nil {
				return c.JSON(http.StatusUnauthorized, map[string]any{"error": "SuperUser Authentication required"})
			}

			// Create a temporary AST parser for debugging
			astParser := NewASTParser()

			allStructs := astParser.GetAllStructs()
			allHandlers := astParser.GetAllHandlers()

			debugData := map[string]interface{}{
				"structs":  make(map[string]interface{}),
				"handlers": make(map[string]interface{}),
				"summary": map[string]interface{}{
					"total_structs":  len(allStructs),
					"total_handlers": len(allHandlers),
				},
			}

			// Add struct information
			structsMap := debugData["structs"].(map[string]interface{})
			for name, structInfo := range allStructs {
				structsMap[name] = map[string]interface{}{
					"name":        structInfo.Name,
					"field_count": len(structInfo.Fields),
					"fields":      structInfo.Fields,
					"json_schema": structInfo.JSONSchema,
				}
			}

			// Add handler information
			handlersMap := debugData["handlers"].(map[string]interface{})
			for name, handlerInfo := range allHandlers {
				handlersMap[name] = map[string]interface{}{
					"name":             handlerInfo.Name,
					"request_type":     handlerInfo.RequestType,
					"response_type":    handlerInfo.ResponseType,
					"request_schema":   handlerInfo.RequestSchema,
					"response_schema":  handlerInfo.ResponseSchema,
					"api_description":  handlerInfo.APIDescription,
					"api_tags":         handlerInfo.APITags,
					"variables":        handlerInfo.Variables,
					"uses_bind_body":   handlerInfo.UsesBindBody,
					"uses_json_decode": handlerInfo.UsesJSONDecode,
					"requires_auth":    handlerInfo.RequiresAuth,
					"auth_type":        handlerInfo.AuthType,
				}
			}

			return c.JSON(http.StatusOK, debugData)
		})

		// Version-specific OpenAPI endpoints
		for _, version := range vm.GetAllVersions() {
			versionPath := fmt.Sprintf("/api/docs/%s", version)
			e.Router.GET(versionPath, func(c *core.RequestEvent) error {
				return vm.GetVersionOpenAPI(c, version)
			}).Bind(apis.RequireSuperuserAuth())

			// Version-specific schema configuration endpoints
			schemaConfigPath := fmt.Sprintf("/api/%s/schema/config", version)
			e.Router.GET(schemaConfigPath, func(c *core.RequestEvent) error {
				return vm.GetVersionSchemaConfig(c, version)
			}).Bind(apis.RequireSuperuserAuth())
		}

		return e.Next()
	})
}

// VersionsHandler returns list of all available API versions
func (vm *APIVersionManager) VersionsHandler(c *core.RequestEvent) error {
	// Check authentication
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "SuperUser Authentication required"})
	}

	infos, err := vm.GetAllVersionsInfo()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to retrieve version information",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"versions":        infos,
		"default_version": vm.GetDefaultVersion(),
		"total_versions":  len(infos),
		"generated_at":    time.Now().Format(time.RFC3339),
	})
}

// GetVersionOpenAPI returns the complete OpenAPI schema for a specific version
func (vm *APIVersionManager) GetVersionOpenAPI(c *core.RequestEvent, version string) error {
	// Check authentication
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "SuperUser Authentication required"})
	}

	// Get version-specific registry
	registry, err := vm.GetVersionRegistry(version)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("Version %s not found", version),
		})
	}

	// Get documentation from version-specific registry
	docs := registry.GetDocsWithComponents()

	return c.JSON(http.StatusOK, docs)
}

// GetVersionSchemaConfig returns the schema configuration for a specific version
func (vm *APIVersionManager) GetVersionSchemaConfig(c *core.RequestEvent, version string) error {
	// Check authentication
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "SuperUser Authentication required"})
	}

	// Verify version exists
	if _, err := vm.GetVersionConfig(version); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("Version %s not found", version),
		})
	}

	// Return schema configuration
	// Only return minimal config - no system fields or generic schemas
	return c.JSON(http.StatusOK, map[string]any{
		"message": "Schema config disabled - using exact AST data only",
		"success": true,
	})
}

// GetSystemFields disabled - no system fields added
func GetSystemFields() []string {
	return []string{}
}

// =============================================================================
// VersionedAPIRouter Methods
// =============================================================================

// GetVersion returns the version of this router
func (vr *VersionedAPIRouter) GetVersion() string {
	return vr.version
}

// GetVersionManager returns the version manager
func (vr *VersionedAPIRouter) GetVersionManager() *APIVersionManager {
	return vr.manager
}

// GetRegistry returns the version-specific registry
func (vr *VersionedAPIRouter) GetRegistry() *APIRegistry {
	return vr.registry
}

// Note: HTTP method handlers (GET, POST, PUT, etc.) are inherited from AutoAPIRouter
// and will automatically use the version-specific registry set in the constructor

// =============================================================================
// Global Version Manager
// =============================================================================

var globalVersionManager *APIVersionManager

// GetGlobalVersionManager returns the global version manager instance
func GetGlobalVersionManager() *APIVersionManager {
	if globalVersionManager == nil {
		globalVersionManager = NewAPIVersionManager()
	}
	return globalVersionManager
}

// SetGlobalVersionManager sets the global version manager
func SetGlobalVersionManager(vm *APIVersionManager) {
	globalVersionManager = vm
}

// InitializeVersionManager creates and configures a version manager
func InitializeVersionManager(versions map[string]*APIDocsConfig, defaultVersion string) *APIVersionManager {
	vm := NewAPIVersionManagerWithDefault(defaultVersion)

	// Register all versions
	for version, config := range versions {
		if err := vm.RegisterVersion(version, config); err != nil {
			// Skip failed version registration
			continue
		}
	}

	// Set global instance
	SetGlobalVersionManager(vm)

	return vm
}

// =============================================================================
// Utility Functions
// =============================================================================

// ValidateVersionString validates that a version string is valid
func ValidateVersionString(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Allow alphanumeric characters, dots, and hyphens
	for _, char := range version {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '-' || char == '_') {
			return fmt.Errorf("version contains invalid character: %c", char)
		}
	}

	return nil
}

// calculateVersionStats calculates statistics for a version's endpoints
func calculateVersionStats(endpoints []APIEndpoint) map[string]int {
	stats := make(map[string]int)

	// Count by method
	for _, endpoint := range endpoints {
		method := strings.ToLower(endpoint.Method)
		stats[method]++
		stats["total"]++

		// Count by auth type
		if endpoint.Auth != nil {
			authKey := fmt.Sprintf("auth_%s", endpoint.Auth.Type)
			stats[authKey]++
		} else {
			stats["auth_none"]++
		}
	}

	return stats
}
