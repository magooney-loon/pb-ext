package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// APIEndpoint matches the runtime API documentation structure
type APIEndpoint struct {
	Method      string                 `json:"method"`
	Path        string                 `json:"path"`
	Description string                 `json:"description"`
	Request     map[string]interface{} `json:"request,omitempty"`
	Response    map[string]interface{} `json:"response,omitempty"`
	Auth        bool                   `json:"requires_auth"`
	Tags        []string               `json:"tags,omitempty"`
	Handler     string                 `json:"handler_name,omitempty"`
}

// APIDocs matches the runtime API documentation structure
type APIDocs struct {
	Title       string        `json:"title"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	BaseURL     string        `json:"base_url"`
	Endpoints   []APIEndpoint `json:"endpoints"`
	Generated   string        `json:"generated_at"`
}

// RuntimeDocsFetcher handles fetching documentation from the running server
type RuntimeDocsFetcher struct {
	serverURL string
	client    *http.Client
}

// NewRuntimeDocsFetcher creates a new fetcher for runtime API docs
func NewRuntimeDocsFetcher(serverURL string) *RuntimeDocsFetcher {
	if serverURL == "" {
		serverURL = "http://localhost:8090"
	}

	return &RuntimeDocsFetcher{
		serverURL: serverURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchAPIDocs retrieves API documentation from the running server
func (f *RuntimeDocsFetcher) FetchAPIDocs() (*APIDocs, error) {
	url := f.serverURL + "/api/docs/json"

	PrintInfo("Fetching API documentation from: %s", url)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API docs from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var docs APIDocs
	if err := json.NewDecoder(resp.Body).Decode(&docs); err != nil {
		return nil, fmt.Errorf("failed to decode API docs: %w", err)
	}

	PrintInfo("Successfully fetched %d endpoints", len(docs.Endpoints))
	return &docs, nil
}

// GenerateAPIDocs generates API documentation files from runtime data
func GenerateAPIDocs() error {
	return GenerateAPIDocsFromRuntime("", "api")
}

// GenerateAPIDocsFromRuntime generates API documentation by fetching from running server
func GenerateAPIDocsFromRuntime(serverURL, outputDir string) error {
	PrintInfo("🤖 Generating API documentation from runtime discovery...")

	// Create fetcher
	fetcher := NewRuntimeDocsFetcher(serverURL)

	// Try to fetch docs from running server
	docs, err := fetcher.FetchAPIDocs()
	if err != nil {
		return fmt.Errorf("failed to fetch runtime API documentation: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate all documentation files
	if err := generateJSONDocs(docs, outputDir); err != nil {
		return err
	}

	if err := generateMarkdownDocs(docs, outputDir); err != nil {
		return err
	}

	if err := generateAPIReadme(docs, outputDir); err != nil {
		return err
	}

	if err := generateAPIIndex(docs, outputDir); err != nil {
		return err
	}

	PrintInfo("✅ API documentation generated successfully in '%s' directory", outputDir)
	PrintInfo("📊 Generated files:")
	PrintInfo("   • %s/api-docs.json (Complete API specification)", outputDir)
	PrintInfo("   • %s/API-GENERATED.md (Human-readable documentation)", outputDir)
	PrintInfo("   • %s/README.md (Overview and usage guide)", outputDir)
	PrintInfo("   • %s/index.html (Static HTML documentation viewer)", outputDir)

	return nil
}

// generateJSONDocs creates the complete API specification in JSON format
func generateJSONDocs(docs *APIDocs, outputDir string) error {
	jsonFile := filepath.Join(outputDir, "api-docs.json")

	file, err := os.Create(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to create JSON docs file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(docs); err != nil {
		return fmt.Errorf("failed to write JSON docs: %w", err)
	}

	return nil
}

// generateMarkdownDocs creates human-readable Markdown documentation
func generateMarkdownDocs(docs *APIDocs, outputDir string) error {
	mdFile := filepath.Join(outputDir, "API-GENERATED.md")

	file, err := os.Create(mdFile)
	if err != nil {
		return fmt.Errorf("failed to create Markdown docs file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "# %s\n\n", docs.Title)
	fmt.Fprintf(file, "%s\n\n", docs.Description)
	fmt.Fprintf(file, "**Version:** %s  \n", docs.Version)
	fmt.Fprintf(file, "**Base URL:** %s  \n", docs.BaseURL)
	fmt.Fprintf(file, "**Generated:** %s  \n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Group endpoints by tags
	tagGroups := make(map[string][]APIEndpoint)
	for _, endpoint := range docs.Endpoints {
		if len(endpoint.Tags) == 0 {
			tagGroups["General"] = append(tagGroups["General"], endpoint)
		} else {
			for _, tag := range endpoint.Tags {
				tagGroups[strings.Title(tag)] = append(tagGroups[strings.Title(tag)], endpoint)
			}
		}
	}

	// Sort tag names
	var tagNames []string
	for tag := range tagGroups {
		tagNames = append(tagNames, tag)
	}
	sort.Strings(tagNames)

	// Generate table of contents
	fmt.Fprintf(file, "## Table of Contents\n\n")
	for _, tag := range tagNames {
		fmt.Fprintf(file, "- [%s](#%s)\n", tag, strings.ToLower(strings.ReplaceAll(tag, " ", "-")))
	}
	fmt.Fprintf(file, "\n")

	// Write endpoints grouped by tags
	for _, tag := range tagNames {
		endpoints := tagGroups[tag]
		fmt.Fprintf(file, "## %s\n\n", tag)

		for _, endpoint := range endpoints {
			writeEndpointMarkdown(file, endpoint)
		}
	}

	return nil
}

// writeEndpointMarkdown writes a single endpoint in Markdown format
func writeEndpointMarkdown(file *os.File, endpoint APIEndpoint) {
	fmt.Fprintf(file, "### %s %s\n\n", endpoint.Method, endpoint.Path)

	if endpoint.Description != "" {
		fmt.Fprintf(file, "%s\n\n", endpoint.Description)
	}

	// Handler info
	if endpoint.Handler != "" {
		fmt.Fprintf(file, "**Handler:** `%s`\n\n", endpoint.Handler)
	}

	// Auth requirement
	if endpoint.Auth {
		fmt.Fprintf(file, "🔒 **Authentication Required**\n\n")
	}

	// Tags
	if len(endpoint.Tags) > 0 {
		fmt.Fprintf(file, "**Tags:** %s\n\n", strings.Join(endpoint.Tags, ", "))
	}

	// Request schema
	if endpoint.Request != nil {
		fmt.Fprintf(file, "**Request Body:**\n```json\n")
		if jsonBytes, err := json.MarshalIndent(endpoint.Request, "", "  "); err == nil {
			fmt.Fprintf(file, "%s\n", string(jsonBytes))
		}
		fmt.Fprintf(file, "```\n\n")
	}

	// Response schema
	if endpoint.Response != nil {
		fmt.Fprintf(file, "**Response:**\n```json\n")
		if jsonBytes, err := json.MarshalIndent(endpoint.Response, "", "  "); err == nil {
			fmt.Fprintf(file, "%s\n", string(jsonBytes))
		}
		fmt.Fprintf(file, "```\n\n")
	}

	fmt.Fprintf(file, "---\n\n")
}

// generateAPIReadme creates an overview and usage guide
func generateAPIReadme(docs *APIDocs, outputDir string) error {
	readmeFile := filepath.Join(outputDir, "README.md")

	file, err := os.Create(readmeFile)
	if err != nil {
		return fmt.Errorf("failed to create README file: %w", err)
	}
	defer file.Close()

	fmt.Fprintf(file, `# %s Documentation

## Overview

This directory contains automatically generated API documentation for the PocketBase extension.

**🤖 Zero Configuration Required** - All documentation is automatically discovered at runtime!

## Quick Access

- **📊 JSON API**: [api-docs.json](api-docs.json) - Complete machine-readable API specification
- **📖 Documentation**: [API-GENERATED.md](API-GENERATED.md) - Human-readable documentation
- **🌐 Web Viewer**: [index.html](index.html) - Interactive HTML documentation

## API Information

- **Base URL**: %s
- **Total Endpoints**: %d
- **Version**: %s
- **Generated**: %s

## How It Works

This documentation is automatically generated from a running PocketBase server using runtime route discovery:

1. **Zero Setup**: No configuration files or annotations needed
2. **Runtime Discovery**: Routes are automatically discovered as they're registered
3. **Smart Analysis**: Intelligent analysis of function names, paths, and patterns
4. **Always Current**: Documentation reflects the actual running application

## Usage

### Access Live Documentation

Visit the running server to get real-time documentation:

- **JSON API**: http://localhost:8090/api/docs/json
- **Live Data**: Always reflects currently registered routes

### Developer Integration

Enable automatic documentation in your routes:

`+"```"+`go
func registerRoutes(app core.App) {
    app.OnServe().BindFunc(func(e *core.ServeEvent) error {
        // One line to enable automatic documentation
        router := server.EnableAutoDocumentation(e)

        // Register routes normally - documentation is automatic!
        router.GET("/api/users", getUsersHandler)        // ✅ Auto-documented!
        router.POST("/api/users", createUserHandler)     // ✅ Auto-documented!
        router.DELETE("/api/users/{id}", deleteHandler)  // ✅ Auto-documented!

        return e.Next()
    })
}
`+"```"+`

That's it! No configuration needed.

## Files in this Directory

| File | Description |
|------|-------------|
| `+"`"+`api-docs.json`+"`"+` | Complete API specification in JSON format |
| `+"`"+`API-GENERATED.md`+"`"+` | Human-readable Markdown documentation |
| `+"`"+`README.md`+"`"+` | This overview file |
| `+"`"+`index.html`+"`"+` | Interactive HTML documentation viewer |

## Benefits

- ✅ **Zero Configuration**: No setup or maintenance required
- ✅ **Always Up-to-Date**: Generated from actual running code
- ✅ **Intelligent**: Smart analysis of routes, auth, and patterns
- ✅ **Multiple Formats**: JSON, Markdown, and HTML outputs
- ✅ **Developer Friendly**: Standard PocketBase route registration

---

*Generated automatically by PocketBase Extension API Documentation System*
`, docs.Title, docs.BaseURL, len(docs.Endpoints), docs.Version, time.Now().Format("2006-01-02 15:04:05"))

	return nil
}

// generateAPIIndex creates an interactive HTML documentation viewer
func generateAPIIndex(docs *APIDocs, outputDir string) error {
	indexFile := filepath.Join(outputDir, "index.html")

	file, err := os.Create(indexFile)
	if err != nil {
		return fmt.Errorf("failed to create HTML index file: %w", err)
	}
	defer file.Close()

	// Write basic HTML structure
	fmt.Fprintf(file, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s - API Documentation</title>
    <style>
        body { font-family: system-ui, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .container { max-width: 1000px; margin: 0 auto; background: white; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { padding: 30px; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; text-align: center; border-radius: 8px 8px 0 0; }
        .header h1 { margin: 0 0 10px 0; font-size: 2.5em; }
        .content { padding: 30px; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .stat-card { padding: 20px; text-align: center; background: #f8f9fa; border-radius: 8px; border: 1px solid #e5e5e5; }
        .stat-number { font-size: 2em; font-weight: bold; color: #667eea; }
        .stat-label { color: #666; margin-top: 5px; }
        .endpoint { margin-bottom: 20px; border: 1px solid #e5e5e5; border-radius: 8px; }
        .endpoint-header { padding: 15px; background: #f8f9fa; border-bottom: 1px solid #e5e5e5; }
        .method { display: inline-block; padding: 4px 8px; border-radius: 4px; font-weight: bold; font-size: 0.9em; margin-right: 10px; }
        .method.GET { background: #d1ecf1; color: #0c5460; }
        .method.POST { background: #d4edda; color: #155724; }
        .method.PATCH { background: #fff3cd; color: #856404; }
        .method.DELETE { background: #f8d7da; color: #721c24; }
        .path { font-family: monospace; font-weight: bold; }
        .endpoint-body { padding: 15px; }
        .description { margin-bottom: 10px; color: #666; }
        .tags { margin-bottom: 10px; }
        .tag { display: inline-block; padding: 2px 6px; background: #e3f2fd; color: #1565c0; border-radius: 10px; font-size: 0.8em; margin-right: 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>%s</h1>
            <p>%s • Version %s</p>
            <p>🤖 Runtime Generated Documentation</p>
        </div>
        <div class="content">
            <div class="stats">
                <div class="stat-card">
                    <div class="stat-number">%d</div>
                    <div class="stat-label">Endpoints</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number">%s</div>
                    <div class="stat-label">Base URL</div>
                </div>
                <div class="stat-card">
                    <div class="stat-number"><a href="api-docs.json">JSON</a></div>
                    <div class="stat-label">API Docs</div>
                </div>
            </div>`, docs.Title, docs.Title, docs.Description, docs.Version, len(docs.Endpoints), docs.BaseURL)

	// Write endpoints
	for _, endpoint := range docs.Endpoints {
		fmt.Fprintf(file, `
            <div class="endpoint">
                <div class="endpoint-header">
                    <span class="method %s">%s</span>
                    <span class="path">%s</span>
                    %s
                </div>
                <div class="endpoint-body">`,
			endpoint.Method, endpoint.Method, endpoint.Path,
			func() string {
				if endpoint.Auth {
					return `<span style="float: right; background: #ff9800; color: white; padding: 2px 6px; border-radius: 10px; font-size: 0.8em;">🔒 Auth</span>`
				}
				return ""
			}())

		if endpoint.Description != "" {
			fmt.Fprintf(file, `<div class="description">%s</div>`, endpoint.Description)
		}

		if len(endpoint.Tags) > 0 {
			fmt.Fprintf(file, `<div class="tags">`)
			for _, tag := range endpoint.Tags {
				fmt.Fprintf(file, `<span class="tag">%s</span>`, tag)
			}
			fmt.Fprintf(file, `</div>`)
		}

		fmt.Fprintf(file, `</div></div>`)
	}

	// Close HTML
	fmt.Fprintf(file, `
        </div>
        <div style="text-align: center; padding: 20px; color: #666; border-top: 1px solid #e5e5e5;">
            <p>Generated: %s</p>
        </div>
    </div>
</body>
</html>`, time.Now().Format("2006-01-02 15:04:05"))

	return nil
}
