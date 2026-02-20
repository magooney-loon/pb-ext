package api

import (
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// =============================================================================
// File Discovery and Import Resolution
// =============================================================================

// newFileSet creates a new token.FileSet (extracted to allow future sharing)
func newFileSet() *token.FileSet {
	return token.NewFileSet()
}

// getModulePath reads go.mod from the current directory (or parent directories)
// and extracts the module path. Returns empty string if go.mod is not found.
func getModulePath() string {
	dir, _ := filepath.Abs(".")
	for {
		content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module"))
				}
			}
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // reached filesystem root
		}
		dir = parent
	}
}

// parseImportedPackages collects imports from API_SOURCE files, resolves local ones
// (within the same Go module) to filesystem paths, and parses their struct definitions.
// This enables handlers to reference types from imported packages without extra directives.
func (p *ASTParser) parseImportedPackages(apiSourceFiles []string) {
	if p.modulePath == "" {
		return // no go.mod found, can't resolve imports
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Collect all local import directories from API_SOURCE files
	localDirs := map[string]bool{}
	for _, f := range apiSourceFiles {
		file, err := parser.ParseFile(p.fileSet, f, nil, parser.ParseComments)
		if err != nil {
			slog.Warn("api docs: failed to parse handler file for import scan", "file", f, "err", err)
			continue
		}
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(importPath, p.modulePath) {
				continue
			}
			suffix := strings.TrimPrefix(importPath, p.modulePath)
			suffix = strings.TrimPrefix(suffix, "/")
			if suffix == "" {
				continue // skip root module import
			}
			localDir := filepath.FromSlash(suffix)
			localDirs[localDir] = true
		}
	}

	// Parse each local directory for structs (skip already-parsed dirs)
	newStructsAdded := false
	for dir := range localDirs {
		if p.parsedDirs[dir] {
			continue
		}
		p.parsedDirs[dir] = true
		if p.parseDirectoryStructs(dir) {
			newStructsAdded = true
		}
	}

	// Re-run schema generation if new structs were added (they may cross-reference)
	if newStructsAdded {
		for _, structInfo := range p.structs {
			structInfo.JSONSchema = p.generateStructSchema(structInfo)
		}
	}
}

// parseDirectoryStructs parses all .go files in a directory for struct definitions
// and type aliases only (no handlers or function return types). Returns true if any
// new structs were added.
func (p *ASTParser) parseDirectoryStructs(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	added := false
	countBefore := len(p.structs)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(p.fileSet, filePath, nil, parser.ParseComments)
		if err != nil {
			slog.Warn("api docs: failed to parse struct file", "file", filePath, "err", err)
			continue
		}
		p.extractStructs(file)
	}
	if len(p.structs) > countBefore {
		added = true
	}
	return added
}
