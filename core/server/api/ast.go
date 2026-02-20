package api

import (
	"go/ast"
	"go/parser"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// =============================================================================
// ASTParser — Entry Points and Public Interface
// =============================================================================

// NewASTParser creates a new simplified PocketBase-focused AST parser
func NewASTParser() *ASTParser {
	ap := &ASTParser{
		fileSet:            newFileSet(),
		structs:            make(map[string]*StructInfo),
		handlers:           make(map[string]*ASTHandlerInfo),
		pocketbasePatterns: NewPocketBasePatterns(),
		logger:             &DefaultLogger{},
		parseErrors:        make([]ParseError, 0),
		typeAliases:        make(map[string]string),
		funcReturnTypes:    make(map[string]string),
		funcBodySchemas:    make(map[string]*OpenAPISchema),
		modulePath:         getModulePath(),
		parsedDirs:         make(map[string]bool),
	}

	// Auto-discover source files
	if err := ap.DiscoverSourceFiles(); err != nil {
		ap.logger.Error("Failed to discover source files: %v", err)
	}

	return ap
}

// DiscoverSourceFiles finds and parses files with API_SOURCE directive,
// then follows local imports to parse struct definitions from imported packages.
func (p *ASTParser) DiscoverSourceFiles() error {
	// Phase 1: Find and parse all API_SOURCE files
	var apiSourceFiles []string
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if strings.Contains(string(content), "// API_SOURCE") {
			apiSourceFiles = append(apiSourceFiles, path)
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, f := range apiSourceFiles {
		if parseErr := p.ParseFile(f); parseErr != nil {
			slog.Warn("api docs: failed to parse API_SOURCE file", "file", f, "err", parseErr)
		}
	}

	// Phase 2: Follow local imports from API_SOURCE files and parse their structs
	p.parseImportedPackages(apiSourceFiles)

	return nil
}

// ParseFile parses a single Go file for PocketBase patterns
func (p *ASTParser) ParseFile(filename string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// First, try to parse the file to validate syntax
	file, err := parser.ParseFile(p.fileSet, filename, nil, parser.ParseComments)
	if err != nil {
		p.parseErrors = append(p.parseErrors, ParseError{
			Type:    "syntax",
			Message: err.Error(),
			File:    filename,
		})
		return err
	}

	// Check if file contains API_SOURCE directive
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	if !strings.Contains(string(content), "// API_SOURCE") {
		// File doesn't have API_SOURCE directive, skip processing
		return nil
	}

	// Track this file's directory as already parsed (avoid re-parsing in import resolution)
	dir := filepath.Dir(filename)
	p.parsedDirs[dir] = true

	// Extract structs (for request/response types)
	p.extractStructs(file)

	// Extract variable declarations first
	p.extractVariableDeclarations(file, &ASTHandlerInfo{Variables: make(map[string]string), VariableExprs: make(map[string]ast.Expr)})

	// Extract return types from non-handler functions BEFORE handler analysis,
	// so that inferTypeFromExpression can resolve function call return types
	p.extractFuncReturnTypes(file)

	// Extract handlers
	p.extractHandlers(file)

	return nil
}

// =============================================================================
// Public Interface Methods (ASTParserInterface implementation)
// =============================================================================

// EnhanceEndpoint enhances an endpoint with AST analysis
func (p *ASTParser) EnhanceEndpoint(endpoint *APIEndpoint) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try multiple handler name variations for better matching
	handlerNames := []string{
		endpoint.Handler,
		ExtractHandlerBaseName(endpoint.Handler, false),
		ExtractHandlerBaseName(endpoint.Handler, true),
	}

	for _, handlerName := range handlerNames {
		if handlerInfo, exists := p.handlers[handlerName]; exists {
			if handlerInfo.RequiresAuth {
				endpoint.Auth = &AuthInfo{
					Required:    true,
					Type:        handlerInfo.AuthType,
					Description: p.getAuthDescription(handlerInfo.AuthType),
				}
			}

			if handlerInfo.APIDescription != "" {
				endpoint.Description = handlerInfo.APIDescription
			}
			if len(handlerInfo.APITags) > 0 {
				endpoint.Tags = handlerInfo.APITags
			}

			if handlerInfo.RequestSchema != nil {
				endpoint.Request = handlerInfo.RequestSchema
			}
			if handlerInfo.ResponseSchema != nil {
				endpoint.Response = handlerInfo.ResponseSchema
			}

			if len(handlerInfo.Parameters) > 0 {
				endpoint.Parameters = handlerInfo.Parameters
			}

			handlerInfo.Variables["enhanced"] = "true"
		}
	}

	return nil
}

// getAuthDescription returns user-friendly auth description
func (p *ASTParser) getAuthDescription(authType string) string {
	switch authType {
	case "user_auth":
		return "User authentication required"
	case "admin_auth":
		return "Admin authentication required"
	case "record_auth":
		return "Record-level authentication required"
	default:
		return "Authentication required"
	}
}

// GetHandlerDescription returns description for a handler
func (p *ASTParser) GetHandlerDescription(handlerName string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if handlerInfo, exists := p.handlers[handlerName]; exists {
		return handlerInfo.APIDescription
	}
	return ""
}

// GetHandlerTags returns tags for a handler
func (p *ASTParser) GetHandlerTags(handlerName string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if handlerInfo, exists := p.handlers[handlerName]; exists {
		return handlerInfo.APITags
	}
	return []string{}
}

// GetStructByName returns a struct by name
func (p *ASTParser) GetStructByName(name string) (*StructInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	structInfo, exists := p.structs[name]
	return structInfo, exists
}

// GetAllStructs returns all parsed structs
func (p *ASTParser) GetAllStructs() map[string]*StructInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*StructInfo)
	for k, v := range p.structs {
		result[k] = v
	}
	return result
}

// GetStructsForFinding returns all structs for searching operations (interface compatibility)
func (p *ASTParser) GetStructsForFinding() map[string]*StructInfo {
	return p.GetAllStructs()
}

// GetAllHandlers returns all parsed handlers
func (p *ASTParser) GetAllHandlers() map[string]*ASTHandlerInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*ASTHandlerInfo)
	for k, v := range p.handlers {
		result[k] = v
	}
	return result
}

// GetHandlerByName returns a handler by name
func (p *ASTParser) GetHandlerByName(name string) (*ASTHandlerInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	handlerInfo, exists := p.handlers[name]
	return handlerInfo, exists
}

// GetParseErrors returns parse errors (interface compatibility)
func (p *ASTParser) GetParseErrors() []ParseError {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.parseErrors
}

// ClearCache clears all cached data (interface compatibility)
func (p *ASTParser) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.structs = make(map[string]*StructInfo)
	p.handlers = make(map[string]*ASTHandlerInfo)
	p.funcReturnTypes = make(map[string]string)
	p.funcBodySchemas = make(map[string]*OpenAPISchema)
	p.parsedDirs = make(map[string]bool)
}
