package api

// API Version Manager - Multi-version API Documentation System
//
// This module provides support for multiple simultaneous API versions,
// allowing developers to work on new versions while keeping old ones active.
//
// Features:
//   - Multiple concurrent API versions
//   - Version-specific documentation systems
//   - Automatic version routing
//   - Independent configuration per version
//   - Backward compatibility with single-version systems
//
// Usage:
//   // Create version manager
//   versionManager := NewAPIVersionManager()
//
//   // Register multiple versions
//   v1Config := &APIDocsConfig{Title: "API v1", Version: "1.0.0"}
//   v2Config := &APIDocsConfig{Title: "API v2", Version: "2.0.0"}
//
//   versionManager.RegisterVersion("v1", v1Config)
//   versionManager.RegisterVersion("v2", v2Config)
//
//   // Use version-specific routers
//   v1Router := versionManager.GetVersionRouter("v1", e)
//   v2Router := versionManager.GetVersionRouter("v2", e)
//
//   // Routes are version-isolated
//   v1Router.GET("/api/v1/users", v1UsersHandler)
//   v2Router.GET("/api/v2/users", v2UsersHandler)

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Version Manager Core Types
// =============================================================================

// APIVersionManager manages multiple API documentation systems
type APIVersionManager struct {
	mu             sync.RWMutex
	versions       map[string]*APIDocumentationSystem
	defaultVersion string
	versionConfigs map[string]*APIDocsConfig
	createdAt      time.Time
	lastModified   time.Time
}

// VersionInfo contains metadata about an API version
type VersionInfo struct {
	Version   string                 `json:"version"`
	Status    string                 `json:"status"` // "active", "deprecated", "development"
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Config    *APIDocsConfig         `json:"config"`
	Stats     map[string]interface{} `json:"stats"`
	Endpoints int                    `json:"endpoints"`
}

// VersionedAPIRouter wraps an AutoAPIRouter with version information
type VersionedAPIRouter struct {
	*AutoAPIRouter
	version string
	manager *APIVersionManager
}

// =============================================================================
// Constructor and Setup
// =============================================================================

// NewAPIVersionManager creates a new API version manager
func NewAPIVersionManager() *APIVersionManager {
	return &APIVersionManager{
		versions:       make(map[string]*APIDocumentationSystem),
		versionConfigs: make(map[string]*APIDocsConfig),
		createdAt:      time.Now(),
		lastModified:   time.Now(),
	}
}

// NewAPIVersionManagerWithDefault creates a version manager with a default version
func NewAPIVersionManagerWithDefault(defaultVersion string, config *APIDocsConfig) *APIVersionManager {
	manager := NewAPIVersionManager()
	manager.RegisterVersion(defaultVersion, config)
	manager.SetDefaultVersion(defaultVersion)
	return manager
}

// =============================================================================
// Version Management
// =============================================================================

// RegisterVersion registers a new API version with its configuration
func (vm *APIVersionManager) RegisterVersion(version string, config *APIDocsConfig) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Validate version string
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Check if version already exists
	if _, exists := vm.versionConfigs[version]; exists {
		return fmt.Errorf("version %s already exists", version)
	}

	// Use provided config or create default with version
	if config == nil {
		config = DefaultAPIDocsConfig()
		config.Version = version
	} else {
		// Ensure version in config matches the key
		config.Version = version
	}

	// Store version config only - we use the global system for endpoints
	vm.versionConfigs[version] = config
	vm.lastModified = time.Now()

	// Set as default if it's the first version
	if len(vm.versionConfigs) == 1 {
		vm.defaultVersion = version
	}

	return nil
}

// RemoveVersion removes an API version
func (vm *APIVersionManager) RemoveVersion(version string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if _, exists := vm.versionConfigs[version]; !exists {
		return fmt.Errorf("version %s does not exist", version)
	}

	// Don't allow removing the default version if it's the only one
	if vm.defaultVersion == version && len(vm.versionConfigs) == 1 {
		return fmt.Errorf("cannot remove the only version")
	}

	delete(vm.versionConfigs, version)

	// If we removed the default version, set a new default
	if vm.defaultVersion == version {
		// Pick the first available version as new default
		for v := range vm.versionConfigs {
			vm.defaultVersion = v
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

	if config, exists := vm.versionConfigs[version]; exists {
		return config, nil
	}

	return nil, fmt.Errorf("version %s not found", version)
}

// GetVersionRouter creates a versioned API router for the specified version
func (vm *APIVersionManager) GetVersionRouter(version string, e *core.ServeEvent) (*VersionedAPIRouter, error) {
	_, err := vm.GetVersionConfig(version)
	if err != nil {
		return nil, err
	}

	// Use the global documentation system and tag endpoints with version
	globalSystem := GetGlobalDocumentationSystem()
	autoRouter := globalSystem.CreateAutoRouter(e)

	return &VersionedAPIRouter{
		AutoAPIRouter: autoRouter,
		version:       version,
		manager:       vm,
	}, nil
}

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

	if _, exists := vm.versions[version]; !exists {
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

	versions := make([]string, 0, len(vm.versionConfigs))
	for version := range vm.versionConfigs {
		versions = append(versions, version)
	}

	sort.Strings(versions)
	return versions
}

// =============================================================================
// Version Information and Stats
// =============================================================================

// GetVersionInfo returns detailed information about a specific version
func (vm *APIVersionManager) GetVersionInfo(version string) (*VersionInfo, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	config, exists := vm.versionConfigs[version]
	if !exists {
		return nil, fmt.Errorf("version %s not found", version)
	}

	// Get endpoints for this version from global registry
	globalRegistry := GetGlobalRegistry()
	allEndpoints := globalRegistry.GetDocs().Endpoints

	var versionEndpoints []APIEndpoint
	for _, endpoint := range allEndpoints {
		// Check if endpoint has version tag matching our version
		for _, tag := range endpoint.Tags {
			if tag == "version:"+version {
				versionEndpoints = append(versionEndpoints, endpoint)
				break
			}
		}
	}

	// Determine version status
	status := "active"
	if len(vm.versionConfigs) > 1 {
		// Simple heuristic: if it's not the default and there are newer versions, it might be deprecated
		allVersions := vm.GetAllVersions()
		if version != vm.defaultVersion && allVersions[len(allVersions)-1] != version {
			status = "deprecated"
		}
		// If it's the newest version but not default, it might be in development
		if version != vm.defaultVersion && allVersions[len(allVersions)-1] == version {
			status = "development"
		}
	}

	return &VersionInfo{
		Version:   version,
		Status:    status,
		CreatedAt: vm.createdAt, // This would ideally be per-version creation time
		UpdatedAt: vm.lastModified,
		Config:    config,
		Stats:     calculateComprehensiveStats(versionEndpoints),
		Endpoints: len(versionEndpoints),
	}, nil
}

// GetAllVersionsInfo returns information about all versions
func (vm *APIVersionManager) GetAllVersionsInfo() map[string]*VersionInfo {
	versions := vm.GetAllVersions()
	info := make(map[string]*VersionInfo)

	for _, version := range versions {
		if vInfo, err := vm.GetVersionInfo(version); err == nil {
			info[version] = vInfo
		}
	}

	return info
}

// =============================================================================
// Server Integration
// =============================================================================

// RegisterWithServer registers all version-specific documentation routes
func (vm *APIVersionManager) RegisterWithServer(app core.App) {
	// Register version management routes - simplified
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Version listing endpoint
		e.Router.GET("/api/docs/versions", func(c *core.RequestEvent) error {
			return vm.VersionsHandler(c)
		})

		// Simple version-specific OpenAPI endpoints
		e.Router.GET("/api/docs/v1", func(c *core.RequestEvent) error {
			return vm.GetVersionOpenAPI(c, "v1")
		})
		e.Router.GET("/api/docs/v2", func(c *core.RequestEvent) error {
			return vm.GetVersionOpenAPI(c, "v2")
		})

		return e.Next()
	})

	// Note: Individual version systems should NOT register their own routes
	// to avoid conflicts. The version manager handles all documentation routes.
}

// =============================================================================
// HTTP Handlers
// =============================================================================

// VersionsHandler returns all available API versions
func (vm *APIVersionManager) VersionsHandler(c *core.RequestEvent) error {
	fmt.Println("DEBUG: VersionsHandler called")
	versionsInfo := vm.GetAllVersionsInfo()

	response := map[string]interface{}{
		"versions":        versionsInfo,
		"default_version": vm.GetDefaultVersion(),
		"total_versions":  len(versionsInfo),
		"last_modified":   vm.lastModified,
	}

	return c.JSON(http.StatusOK, response)
}

// GetVersionOpenAPI returns the complete OpenAPI schema for a specific version
func (vm *APIVersionManager) GetVersionOpenAPI(c *core.RequestEvent, version string) error {
	fmt.Println("DEBUG: GetVersionOpenAPI called for version:", version)

	config, err := vm.GetVersionConfig(version)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "Version " + version + " not found",
		})
	}

	// Get all endpoints from global registry and filter by version
	globalRegistry := GetGlobalRegistry()
	allEndpoints := globalRegistry.GetDocs().Endpoints

	var versionEndpoints []APIEndpoint
	for _, endpoint := range allEndpoints {
		// Check if endpoint has version tag matching our version
		for _, tag := range endpoint.Tags {
			if tag == "version:"+version {
				versionEndpoints = append(versionEndpoints, endpoint)
				break
			}
		}
	}

	// Create complete version-specific OpenAPI docs with generated components
	docs := &APIDocs{
		Title:       config.Title,
		Version:     config.Version,
		Description: config.Description,
		BaseURL:     config.BaseURL,
		Endpoints:   versionEndpoints,
		Generated:   time.Now().Format(time.RFC3339),
		Components:  make(map[string]interface{}),
	}

	// Generate components specifically for version-specific endpoints using a temporary generator
	globalRegistry = GetGlobalRegistry()
	if globalRegistry != nil && globalRegistry.astParser != nil {
		// Create a temporary schema generator for this version's endpoints only
		tempSchemaGenerator := NewSchemaGenerator(globalRegistry.astParser)

		// Create a temporary registry with only version-specific endpoints
		tempConfig := &APIDocsConfig{
			Title:   config.Title,
			Version: config.Version,
		}
		tempRegistry := NewAPIRegistry(tempConfig, globalRegistry.astParser, tempSchemaGenerator)

		// Register only the version-specific endpoints
		for _, endpoint := range versionEndpoints {
			tempRegistry.RegisterEndpoint(endpoint)
		}

		// Generate components from the temporary registry
		docs.Components = tempSchemaGenerator.GenerateComponentSchemas()
	}

	return c.JSON(http.StatusOK, docs)
}

// Backward compatibility handlers removed - use versioned endpoints only

// =============================================================================
// Versioned Router Extensions
// =============================================================================

// GetVersion returns the version of this router
func (vr *VersionedAPIRouter) GetVersion() string {
	return vr.version
}

// GetVersionManager returns the version manager
func (vr *VersionedAPIRouter) GetVersionManager() *APIVersionManager {
	return vr.manager
}

// Override HTTP method functions to automatically add version tags

// GET registers a GET route and automatically adds version tag
func (vr *VersionedAPIRouter) GET(path string, handler func(*core.RequestEvent) error) *RouteChain {
	route := vr.AutoAPIRouter.GET(path, handler)
	vr.addVersionTag(path, "GET")
	return route
}

// POST registers a POST route and automatically adds version tag
func (vr *VersionedAPIRouter) POST(path string, handler func(*core.RequestEvent) error) *RouteChain {
	route := vr.AutoAPIRouter.POST(path, handler)
	vr.addVersionTag(path, "POST")
	return route
}

// PUT registers a PUT route and automatically adds version tag
func (vr *VersionedAPIRouter) PUT(path string, handler func(*core.RequestEvent) error) *RouteChain {
	route := vr.AutoAPIRouter.PUT(path, handler)
	vr.addVersionTag(path, "PUT")
	return route
}

// PATCH registers a PATCH route and automatically adds version tag
func (vr *VersionedAPIRouter) PATCH(path string, handler func(*core.RequestEvent) error) *RouteChain {
	route := vr.AutoAPIRouter.PATCH(path, handler)
	vr.addVersionTag(path, "PATCH")
	return route
}

// DELETE registers a DELETE route and automatically adds version tag
func (vr *VersionedAPIRouter) DELETE(path string, handler func(*core.RequestEvent) error) *RouteChain {
	route := vr.AutoAPIRouter.DELETE(path, handler)
	vr.addVersionTag(path, "DELETE")
	return route
}

// OPTIONS registers an OPTIONS route and automatically adds version tag
func (vr *VersionedAPIRouter) OPTIONS(path string, handler func(*core.RequestEvent) error) *RouteChain {
	route := vr.AutoAPIRouter.OPTIONS(path, handler)
	vr.addVersionTag(path, "OPTIONS")
	return route
}

// HEAD registers a HEAD route and automatically adds version tag
func (vr *VersionedAPIRouter) HEAD(path string, handler func(*core.RequestEvent) error) *RouteChain {
	route := vr.AutoAPIRouter.HEAD(path, handler)
	vr.addVersionTag(path, "HEAD")
	return route
}

// addVersionTag adds version tag to the endpoint
func (vr *VersionedAPIRouter) addVersionTag(path, method string) {
	// Get the global registry and add version tag to the endpoint
	registry := GetGlobalRegistry()
	if endpoint, exists := registry.GetEndpoint(method, path); exists {
		// Add version tag if not already present
		versionTag := "version:" + vr.version
		hasVersionTag := false
		for _, tag := range endpoint.Tags {
			if tag == versionTag {
				hasVersionTag = true
				break
			}
		}
		if !hasVersionTag {
			// Create new endpoint with version tag
			updatedEndpoint := *endpoint
			updatedEndpoint.Tags = append(updatedEndpoint.Tags, versionTag)
			// Re-register the endpoint with updated tags
			registry.RegisterEndpoint(updatedEndpoint)
		}
	}
}

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
func SetGlobalVersionManager(manager *APIVersionManager) {
	globalVersionManager = manager
}

// InitializeVersionManager initializes the global version manager with versions
func InitializeVersionManager(versions map[string]*APIDocsConfig, defaultVersion string) *APIVersionManager {
	manager := NewAPIVersionManager()

	// Register all versions
	for version, config := range versions {
		if err := manager.RegisterVersion(version, config); err != nil {
			// Log error but continue with other versions
			continue
		}
	}

	// Set default version if specified and exists
	if defaultVersion != "" {
		if err := manager.SetDefaultVersion(defaultVersion); err == nil {
			// Successfully set default
		}
	}

	SetGlobalVersionManager(manager)
	return manager
}

// =============================================================================
// Migration Utilities
// =============================================================================

// MigrateFromSingleVersion migrates from a single global system to versioned system
func MigrateFromSingleVersion(version string) *APIVersionManager {
	// Get current global system config
	currentSystem := GetGlobalDocumentationSystem()

	// Create version manager
	manager := NewAPIVersionManager()

	// Extract config from current system
	config := currentSystem.config
	if config == nil {
		config = DefaultAPIDocsConfig()
	}
	config.Version = version

	// Register as a version
	if err := manager.RegisterVersion(version, config); err == nil {
		manager.SetDefaultVersion(version)
	}

	return manager
}

// =============================================================================
// Helper Functions
// =============================================================================

// ValidateVersionString validates a version string format
func ValidateVersionString(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Basic validation - can be extended with semver parsing
	if strings.Contains(version, " ") {
		return fmt.Errorf("version cannot contain spaces")
	}

	if len(version) > 50 {
		return fmt.Errorf("version string too long (max 50 characters)")
	}

	return nil
}

// CreateVersionFromTemplate creates a new version based on an existing version
func (vm *APIVersionManager) CreateVersionFromTemplate(newVersion, templateVersion string) error {
	// Get template version config
	vm.mu.RLock()
	templateConfig, exists := vm.versionConfigs[templateVersion]
	vm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("template version %s does not exist", templateVersion)
	}

	// Create new config based on template
	newConfig := *templateConfig // Copy config
	newConfig.Version = newVersion
	newConfig.Title = strings.Replace(templateConfig.Title, templateVersion, newVersion, -1)

	// Register new version
	return vm.RegisterVersion(newVersion, &newConfig)
}
