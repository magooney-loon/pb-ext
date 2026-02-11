package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// =============================================================================
// Simplified PocketBase-Focused AST Parser
// =============================================================================

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

// NewASTParser creates a new simplified PocketBase-focused AST parser
func NewASTParser() *ASTParser {
	ap := &ASTParser{
		fileSet:            token.NewFileSet(),
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
			p.logger.Error("Failed to parse %s: %v", f, parseErr)
		}
	}

	// Phase 2: Follow local imports from API_SOURCE files and parse their structs
	p.parseImportedPackages(apiSourceFiles)

	return nil
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
			continue
		}
		p.extractStructs(file)
	}
	if len(p.structs) > countBefore {
		added = true
	}
	return added
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

// extractStructs extracts struct definitions that might be used for requests/responses
// Uses a two-pass approach: first pass registers all structs and type aliases, second pass generates schemas
func (p *ASTParser) extractStructs(file *ast.File) {
	// First pass: register all structs with their fields (without JSONSchema) and type aliases
	ast.Inspect(file, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						// It's a struct definition
						structInfo := p.parseStruct(typeSpec, structType, false)
						p.structs[structInfo.Name] = structInfo
					} else {
						// It might be a type alias (type Alias = RealType)
						// Extract the real type name
						realTypeName := p.extractTypeName(typeSpec.Type)
						if realTypeName != "" {
							aliasName := typeSpec.Name.Name
							p.typeAliases[aliasName] = realTypeName
						}
					}
				}
			}
		}
		return true
	})

	// Second pass: generate JSON schemas now that all structs are known
	for _, structInfo := range p.structs {
		structInfo.JSONSchema = p.generateStructSchema(structInfo)
	}
}

// parseStruct parses a struct definition
// generateSchema: if false, only extracts fields without generating JSONSchema
func (p *ASTParser) parseStruct(typeSpec *ast.TypeSpec, structType *ast.StructType, generateSchema bool) *StructInfo {
	structInfo := &StructInfo{
		Name:     typeSpec.Name.Name,
		Fields:   make(map[string]*FieldInfo),
		Embedded: []string{},
	}

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			// Embedded (anonymous) field — record the type name for later flattening
			embeddedType := p.extractTypeName(field.Type)
			if embeddedType != "" {
				structInfo.Embedded = append(structInfo.Embedded, embeddedType)
			}
			continue
		}

		for _, name := range field.Names {
			fieldType := p.extractTypeName(field.Type)

			// Log error if type extraction failed
			if fieldType == "" {
				p.logger.Error("Failed to extract type for field '%s' in struct '%s'", name.Name, structInfo.Name)
			}

			// Detect pointer types before extractTypeName unwraps them
			_, isPointer := field.Type.(*ast.StarExpr)

			fieldInfo := &FieldInfo{
				Name:      name.Name,
				Type:      fieldType,
				IsPointer: isPointer,
			}

			// Parse JSON tags
			if field.Tag != nil {
				p.parseJSONTag(field.Tag.Value, fieldInfo)
			}

			structInfo.Fields[fieldInfo.Name] = fieldInfo
		}
	}

	// Generate JSON schema only if requested (second pass)
	if generateSchema {
		structInfo.JSONSchema = p.generateStructSchema(structInfo)
	}

	return structInfo
}

// parseJSONTag parses JSON struct tags including omitempty
func (p *ASTParser) parseJSONTag(tagValue string, fieldInfo *FieldInfo) {
	tagValue = strings.Trim(tagValue, "`")
	if jsonTag := p.extractTag(tagValue, "json"); jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if len(parts) > 0 && parts[0] != "" && parts[0] != "-" {
			fieldInfo.JSONName = parts[0]
		}
		// Check for omitempty option
		for _, opt := range parts[1:] {
			if strings.TrimSpace(opt) == "omitempty" {
				fieldInfo.JSONOmitEmpty = true
				break
			}
		}
	}
}

// extractTag extracts a specific tag value
func (p *ASTParser) extractTag(tagValue, tagName string) string {
	re := regexp.MustCompile(tagName + `:"([^"]*)"`)
	matches := re.FindStringSubmatch(tagValue)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractHandlers finds and analyzes PocketBase handler functions
func (p *ASTParser) extractHandlers(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if p.isPocketBaseHandler(funcDecl) {
				handlerInfo := p.analyzePocketBaseHandler(funcDecl)
				if handlerInfo != nil {
					p.handlers[handlerInfo.Name] = handlerInfo
				}
			}
		}
		return true
	})
}

// extractFuncReturnTypes extracts return types from non-handler function declarations.
// This enables resolving the return type of helper functions like formatCandlesFull()
// that are called within handlers but whose return types can't be inferred from the call site.
func (p *ASTParser) extractFuncReturnTypes(file *ast.File) {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Type.Results == nil || funcDecl.Recv != nil {
			continue // skip non-functions, no-return functions, and methods
		}
		if p.isPocketBaseHandler(funcDecl) {
			continue // skip handlers — they're analyzed separately
		}
		// Use the first non-error return type
		for _, result := range funcDecl.Type.Results.List {
			typeName := p.extractTypeName(result.Type)
			if typeName != "" && typeName != "error" {
				p.funcReturnTypes[funcDecl.Name.Name] = typeName
				// For functions returning map[string]any or []map[string]any,
				// analyze the body to extract the actual map keys being set
				if funcDecl.Body != nil && (typeName == "map[string]any" || typeName == "map[string]interface{}" ||
					typeName == "[]map[string]any" || typeName == "[]map[string]interface{}") {
					if schema := p.analyzeHelperFuncBody(funcDecl); schema != nil {
						p.funcBodySchemas[funcDecl.Name.Name] = schema
					}
				}
				break
			}
		}
	}
}

// analyzeHelperFuncBody walks a helper function's body to find map[string]any composite
// literals and extract their keys/value types. This enables rich schema generation for
// functions like formatDivergences() that build response objects inside loops.
func (p *ASTParser) analyzeHelperFuncBody(funcDecl *ast.FuncDecl) *OpenAPISchema {
	// Build a temporary handler info to track variables and map additions within this function
	tempInfo := &ASTHandlerInfo{
		Variables:        make(map[string]string),
		VariableExprs:    make(map[string]ast.Expr),
		MapAdditions:     make(map[string][]MapKeyAdd),
		SliceAppendExprs: make(map[string]ast.Expr),
	}

	// Collect all map[string]any composite literals found in the body.
	// Pick the one with the most keys — that's the primary response shape.
	var bestSchema *OpenAPISchema
	var bestVarName string
	bestKeyCount := 0

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		// Track variable assignments so we can resolve map additions
		if assign, ok := n.(*ast.AssignStmt); ok {
			p.trackVariableAssignment(assign, tempInfo)
		}

		// Look for map[string]any{...} composite literals
		compLit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		mapType, ok := compLit.Type.(*ast.MapType)
		if !ok {
			return true
		}
		// Verify it's map[string]any or map[string]interface{}
		keyType := p.extractTypeName(mapType.Key)
		valType := p.extractTypeName(mapType.Value)
		if keyType != "string" || (valType != "any" && valType != "interface{}") {
			return true
		}
		// Parse the map literal
		schema := p.parseMapLiteral(compLit, tempInfo)
		if schema != nil && len(schema.Properties) > bestKeyCount {
			bestSchema = schema
			bestKeyCount = len(schema.Properties)
			// Try to find which variable this literal is assigned to (for map additions)
			bestVarName = p.findAssignedVariable(funcDecl.Body, compLit)
		}
		return true
	})

	if bestSchema == nil {
		return nil
	}

	// Merge any dynamic map additions (e.g., entry["metadata"] = ...)
	if bestVarName != "" {
		p.mergeMapAdditions(bestSchema, bestVarName, tempInfo)
	}

	// If the function returns []map[string]any, wrap the item schema in an array
	retType := p.funcReturnTypes[funcDecl.Name.Name]
	if strings.HasPrefix(retType, "[]") {
		return &OpenAPISchema{
			Type:  "array",
			Items: bestSchema,
		}
	}

	return bestSchema
}

// findAssignedVariable finds the variable name a composite literal is assigned to
// by scanning assignment statements in the given block.
func (p *ASTParser) findAssignedVariable(body *ast.BlockStmt, target *ast.CompositeLit) string {
	var result string
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range assign.Rhs {
			if rhs == target && i < len(assign.Lhs) {
				if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
					result = ident.Name
					return false
				}
			}
		}
		return true
	})
	return result
}

// extractQueryParameters detects query parameter usage patterns in handler bodies.
// It tracks variables assigned from URL.Query() and finds q.Get("paramName") calls.
func (p *ASTParser) extractQueryParameters(body *ast.BlockStmt, handlerInfo *ASTHandlerInfo) {
	queryVarNames := map[string]bool{}

	ast.Inspect(body, func(n ast.Node) bool {
		// Track: q := e.Request.URL.Query()
		if assign, ok := n.(*ast.AssignStmt); ok {
			for i, rhs := range assign.Rhs {
				if isURLQueryCall(rhs) {
					if i < len(assign.Lhs) {
						if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
							queryVarNames[ident.Name] = true
						}
					}
				}
			}
		}
		// Detect: q.Get("paramName")
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Get" {
				if ident, ok := sel.X.(*ast.Ident); ok && queryVarNames[ident.Name] {
					if len(call.Args) > 0 {
						if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							paramName := strings.Trim(lit.Value, `"`)
							handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, &ParamInfo{
								Name:     paramName,
								Type:     "string",
								Source:   "query",
								Required: false,
							})
						}
					}
				}
			}
		}
		return true
	})
}

// isURLQueryCall checks if an expression is a call to .URL.Query()
func isURLQueryCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Query" {
		return false
	}
	// Check for .URL.Query() pattern
	if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
		return innerSel.Sel.Name == "URL"
	}
	return false
}

// appendParamIfNew appends a ParamInfo to the slice if no param with the same name exists
func appendParamIfNew(params []*ParamInfo, param *ParamInfo) []*ParamInfo {
	for _, p := range params {
		if p.Name == param.Name {
			return params
		}
	}
	return append(params, param)
}

// isPocketBaseHandler checks if a function is a PocketBase handler
func (p *ASTParser) isPocketBaseHandler(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) != 1 {
		return false
	}

	param := funcDecl.Type.Params.List[0]
	if star, ok := param.Type.(*ast.StarExpr); ok {
		if sel, ok := star.X.(*ast.SelectorExpr); ok {
			return sel.Sel.Name == "RequestEvent"
		}
	}

	return false
}

// analyzePocketBaseHandler analyzes a PocketBase handler function
func (p *ASTParser) analyzePocketBaseHandler(funcDecl *ast.FuncDecl) *ASTHandlerInfo {
	handlerInfo := &ASTHandlerInfo{
		Name:             funcDecl.Name.Name,
		Variables:        make(map[string]string),
		VariableExprs:    make(map[string]ast.Expr),
		MapAdditions:     make(map[string][]MapKeyAdd),
		SliceAppendExprs: make(map[string]ast.Expr),
	}

	// Extract API description from comments
	if funcDecl.Doc != nil {
		p.parseHandlerComments(funcDecl.Doc, handlerInfo)
	}

	// Analyze handler body for PocketBase patterns
	if funcDecl.Body != nil {
		p.analyzePocketBasePatterns(funcDecl.Body, handlerInfo)
	}

	// Try to generate request schema if we have a request type
	// Use $ref for known struct types so they appear in components/schemas
	if handlerInfo.RequestType != "" {
		if schema := p.generateSchemaForEndpoint(handlerInfo.RequestType); schema != nil {
			handlerInfo.RequestSchema = schema
		}
	}

	// Additional pass to detect variable declarations within the handler
	if funcDecl.Body != nil {
		p.extractLocalVariables(funcDecl.Body, handlerInfo)
	}

	// Extract query parameters from URL.Query().Get() patterns
	if funcDecl.Body != nil {
		p.extractQueryParameters(funcDecl.Body, handlerInfo)
	}

	return handlerInfo
}

// parseHandlerComments extracts API information from function comments
func (p *ASTParser) parseHandlerComments(doc *ast.CommentGroup, handlerInfo *ASTHandlerInfo) {
	for _, comment := range doc.List {
		text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))

		if strings.HasPrefix(text, "API_DESC") {
			handlerInfo.APIDescription = strings.TrimSpace(strings.TrimPrefix(text, "API_DESC"))
		} else if strings.HasPrefix(text, "API_TAGS") {
			tags := strings.TrimSpace(strings.TrimPrefix(text, "API_TAGS"))
			handlerInfo.APITags = strings.Split(tags, ",")
			for i, tag := range handlerInfo.APITags {
				handlerInfo.APITags[i] = strings.TrimSpace(tag)
			}
		}
	}
}

// analyzePocketBasePatterns analyzes the handler body for PocketBase-specific patterns
func (p *ASTParser) analyzePocketBasePatterns(body *ast.BlockStmt, handlerInfo *ASTHandlerInfo) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			p.analyzePocketBaseCall(node, handlerInfo)
		case *ast.AssignStmt:
			p.trackVariableAssignment(node, handlerInfo)
		case *ast.DeclStmt:
			// Handle var declarations within functions
			if genDecl, ok := node.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
				p.extractVarDecl(genDecl, handlerInfo)
			}
		}
		return true
	})
}

// analyzePocketBaseCall analyzes PocketBase-specific method calls and general Go patterns
func (p *ASTParser) analyzePocketBaseCall(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		switch sel.Sel.Name {
		case "BindBody":
			p.handleBindBody(call, handlerInfo)
		case "JSON":
			p.handleJSONResponse(call, handlerInfo)
		case "RequireAuth", "RequireAdminAuth", "RequireRecordAuth":
			p.handleAuth(sel.Sel.Name, handlerInfo)
		case "FindRecordsByFilter", "FindRecordById", "CreateRecord", "UpdateRecord", "DeleteRecord":
			p.handleDatabaseOperation(sel.Sel.Name, handlerInfo)
		case "EnrichRecord", "EnrichRecords":
			handlerInfo.UsesEnrichRecords = true
		case "RequestInfo":
			handlerInfo.UsesRequestInfo = true
		case "Decode":
			// Handle json.NewDecoder().Decode() pattern
			p.handleJSONDecode(call, handlerInfo)
		case "NewDecoder":
			// Handle json.NewDecoder() calls
			p.handleNewDecoder(handlerInfo)
		}
	} else if ident, ok := call.Fun.(*ast.Ident); ok {
		// Handle direct function calls
		switch ident.Name {
		case "NewDecoder":
			p.handleNewDecoder(handlerInfo)
		}
	}
}

// handleBindBody handles e.BindBody(&data) pattern
func (p *ASTParser) handleBindBody(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) {
	handlerInfo.UsesBindBody = true

	if len(call.Args) > 0 {
		// Handle &struct pattern
		if unary, ok := call.Args[0].(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if ident, ok := unary.X.(*ast.Ident); ok {
				if varType, exists := handlerInfo.Variables[ident.Name]; exists {
					handlerInfo.RequestType = varType
				}
			}
		}
	}
}

// handleJSONResponse handles e.JSON(status, data) pattern
func (p *ASTParser) handleJSONResponse(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) {
	if len(call.Args) >= 2 {
		arg := call.Args[1]

		// Unwrap &expr at the call site: e.JSON(200, &SomeStruct{...})
		if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			arg = unary.X
		}

		// Resolve the expression to analyze — either directly or via variable tracing
		exprsToAnalyze := []ast.Expr{arg}
		if ident, ok := arg.(*ast.Ident); ok {
			if expr, exists := handlerInfo.VariableExprs[ident.Name]; exists {
				tracedExpr := expr
				if unary, ok := tracedExpr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
					tracedExpr = unary.X
				}
				exprsToAnalyze = append(exprsToAnalyze, tracedExpr)
			}
		}

		// Try composite literal analysis on each candidate expression
		for _, candidate := range exprsToAnalyze {
			if schema := p.analyzeMapLiteralSchema(candidate, handlerInfo); schema != nil {
				// Also set ResponseType for metadata if we can infer it
				if responseType := p.inferTypeFromExpression(candidate, handlerInfo); responseType != "" {
					handlerInfo.ResponseType = responseType
				}
				// Merge dynamic map key additions (e.g., result["key"] = value)
				if ident, ok := arg.(*ast.Ident); ok {
					p.mergeMapAdditions(schema, ident.Name, handlerInfo)
				}
				handlerInfo.ResponseSchema = schema
				return
			}
		}

		// Try deep expression analysis (handles function calls → funcBodySchemas)
		for _, candidate := range exprsToAnalyze {
			if schema := p.analyzeValueExpression(candidate, handlerInfo); schema != nil &&
				schema.Type != "string" && (len(schema.Properties) > 0 || schema.Items != nil) {
				if responseType := p.inferTypeFromExpression(candidate, handlerInfo); responseType != "" {
					handlerInfo.ResponseType = responseType
				}
				if ident, ok := arg.(*ast.Ident); ok {
					p.mergeMapAdditions(schema, ident.Name, handlerInfo)
				}
				handlerInfo.ResponseSchema = schema
				return
			}
		}

		// Try type inference for struct-based responses (variable reference → type lookup)
		for _, candidate := range exprsToAnalyze {
			if responseType := p.inferTypeFromExpression(candidate, handlerInfo); responseType != "" {
				handlerInfo.ResponseType = responseType
				if schema := p.generateSchemaForEndpoint(responseType); schema != nil {
					handlerInfo.ResponseSchema = schema
					return
				}
			}
		}

		// Fallback: generate a basic object schema
		handlerInfo.ResponseSchema = &OpenAPISchema{
			Type:                 "object",
			Description:          "Response data",
			AdditionalProperties: true,
		}
	}
}

// handleJSONDecode handles json.Decoder.Decode(&struct) pattern
func (p *ASTParser) handleJSONDecode(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) {
	if len(call.Args) > 0 {
		// Handle &struct pattern in Decode(&req)
		if unary, ok := call.Args[0].(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if ident, ok := unary.X.(*ast.Ident); ok {
				if varType, exists := handlerInfo.Variables[ident.Name]; exists {
					handlerInfo.RequestType = varType
					// Generate request schema immediately
					// Use $ref for known struct types so they appear in components/schemas
					if schema := p.generateSchemaForEndpoint(varType); schema != nil {
						handlerInfo.RequestSchema = schema
					}
				}
			}
		}
	}
}

// handleNewDecoder handles json.NewDecoder(c.Request.Body) pattern
func (p *ASTParser) handleNewDecoder(handlerInfo *ASTHandlerInfo) {
	// This indicates JSON decoding is being used
	handlerInfo.UsesJSONDecode = true
}

// handleAuth handles authentication requirement patterns
func (p *ASTParser) handleAuth(authMethod string, handlerInfo *ASTHandlerInfo) {
	handlerInfo.RequiresAuth = true
	switch authMethod {
	case "RequireAuth":
		handlerInfo.AuthType = "user_auth"
	case "RequireAdminAuth":
		handlerInfo.AuthType = "admin_auth"
	case "RequireRecordAuth":
		handlerInfo.AuthType = "record_auth"
	}
}

// handleDatabaseOperation handles database operation patterns
func (p *ASTParser) handleDatabaseOperation(method string, handlerInfo *ASTHandlerInfo) {
	operation := map[string]string{
		"FindRecordsByFilter": "query",
		"FindRecordById":      "read",
		"CreateRecord":        "create",
		"UpdateRecord":        "update",
		"DeleteRecord":        "delete",
	}

	if op, exists := operation[method]; exists {
		handlerInfo.DatabaseOperations = append(handlerInfo.DatabaseOperations, op)
	}
}

// trackVariableAssignment tracks variable assignments for type inference
func (p *ASTParser) trackVariableAssignment(assign *ast.AssignStmt, handlerInfo *ASTHandlerInfo) {
	for i, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok && i < len(assign.Rhs) {
			rhs := assign.Rhs[i]
			if varType := p.inferTypeFromExpression(rhs, handlerInfo); varType != "" {
				handlerInfo.Variables[ident.Name] = varType
			}
			// Always store the RHS expression so we can analyze map literals later
			handlerInfo.VariableExprs[ident.Name] = rhs

			// Track append() calls: varName = append(varName, itemExpr)
			// This captures the item expression being appended to a slice variable
			if handlerInfo.SliceAppendExprs != nil {
				if callExpr, ok := rhs.(*ast.CallExpr); ok {
					if fnIdent, ok := callExpr.Fun.(*ast.Ident); ok && fnIdent.Name == "append" {
						if len(callExpr.Args) == 2 {
							// Verify first arg is the same variable (append(varName, item))
							if argIdent, ok := callExpr.Args[0].(*ast.Ident); ok && argIdent.Name == ident.Name {
								handlerInfo.SliceAppendExprs[ident.Name] = callExpr.Args[1]
							}
						}
					}
				}
			}
		}

		// Track dynamic map key additions: mapVar["key"] = value
		if indexExpr, ok := lhs.(*ast.IndexExpr); ok {
			if ident, ok := indexExpr.X.(*ast.Ident); ok {
				// Extract the string key from the index expression
				if basicLit, ok := indexExpr.Index.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
					key := strings.Trim(basicLit.Value, `"`)
					var valueExpr ast.Expr
					if i < len(assign.Rhs) {
						valueExpr = assign.Rhs[i]
					} else if len(assign.Rhs) == 1 {
						valueExpr = assign.Rhs[0]
					}
					if valueExpr != nil {
						handlerInfo.MapAdditions[ident.Name] = append(
							handlerInfo.MapAdditions[ident.Name],
							MapKeyAdd{Key: key, Value: valueExpr},
						)
					}
				}
			}
		}
	}
}

// mergeMapAdditions merges dynamically added map keys into an existing object schema
// This handles patterns like: result["computed_at"] = time.Now().Format(...)
func (p *ASTParser) mergeMapAdditions(schema *OpenAPISchema, varName string, handlerInfo *ASTHandlerInfo) {
	additions, exists := handlerInfo.MapAdditions[varName]
	if !exists || len(additions) == 0 {
		return
	}
	if schema.Properties == nil {
		schema.Properties = make(map[string]*OpenAPISchema)
	}
	for _, add := range additions {
		// Don't overwrite existing keys from the literal
		if _, exists := schema.Properties[add.Key]; exists {
			continue
		}
		valueSchema := p.analyzeValueExpression(add.Value, handlerInfo)
		if valueSchema != nil {
			schema.Properties[add.Key] = valueSchema
		}
	}
}

// enrichArraySchemaFromAppend checks if an array schema has generic items (no properties)
// and tries to resolve richer items from tracked append() patterns.
// For example, when a handler does:
//
//	entries := make([]map[string]any, 0)
//	for _, r := range records {
//	    entry := map[string]any{"name": r.GetString("name"), ...}
//	    entries = append(entries, entry)
//	}
//
// The make() produces a generic {type:"array", items:{type:"object", additionalProperties:true}}.
// This method replaces the items with the resolved schema from the appended expression (entry).
func (p *ASTParser) enrichArraySchemaFromAppend(schema *OpenAPISchema, varName string, handlerInfo *ASTHandlerInfo) *OpenAPISchema {
	// Only enrich arrays with generic items
	if schema.Type != "array" || schema.Items == nil {
		return schema
	}
	if len(schema.Items.Properties) > 0 || schema.Items.Ref != "" {
		return schema // items already have rich schema
	}

	if handlerInfo.SliceAppendExprs == nil {
		return schema
	}

	appendExpr, exists := handlerInfo.SliceAppendExprs[varName]
	if !exists {
		return schema
	}

	// The appended expression might be a variable (e.g., "entry") — resolve it
	itemSchema := p.analyzeValueExpression(appendExpr, handlerInfo)
	if itemSchema != nil && itemSchema.Type != "string" && (len(itemSchema.Properties) > 0 || itemSchema.Ref != "") {
		schema.Items = itemSchema
	}

	return schema
}

// inferTypeFromExpression infers type from expressions (generic approach)
func (p *ASTParser) inferTypeFromExpression(expr ast.Expr, handlerInfo *ASTHandlerInfo) string {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		// Handle struct literals like SomeStruct{...}
		if typeName := p.extractTypeName(e.Type); typeName != "" {
			return typeName
		}
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			// Handle &struct patterns
			return p.extractTypeName(e.X)
		}
	case *ast.Ident:
		// Check variables first
		if varType, exists := handlerInfo.Variables[e.Name]; exists {
			return varType
		}
		// Check if it's a known struct type
		if _, exists := p.structs[e.Name]; exists {
			return e.Name
		}
		// Generic pattern matching for common types
		name := e.Name
		if strings.HasSuffix(name, "Request") || strings.HasSuffix(name, "Response") ||
			strings.HasSuffix(name, "Data") || strings.HasSuffix(name, "Input") ||
			strings.HasSuffix(name, "Output") || strings.HasSuffix(name, "Payload") {
			return name
		}
	case *ast.CallExpr:
		// Handle constructor patterns generically
		if ident, ok := e.Fun.(*ast.Ident); ok {
			// Handle make() — extract type from first argument
			if ident.Name == "make" && len(e.Args) > 0 {
				typeName := p.extractTypeName(e.Args[0])
				if typeName != "" {
					return typeName
				}
			}
			if strings.HasPrefix(ident.Name, "New") && len(ident.Name) > 3 {
				return strings.TrimPrefix(ident.Name, "New")
			}
			// Check function return types from signature analysis
			if retType, exists := p.funcReturnTypes[ident.Name]; exists {
				return retType
			}
		}
		// Handle method calls that return records/arrays
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			methodName := sel.Sel.Name
			// PocketBase record methods
			if strings.Contains(methodName, "Record") {
				if strings.Contains(methodName, "s") || strings.Contains(methodName, "Filter") {
					return "[]Record"
				}
				return "Record"
			}
			// Generic collection methods
			if strings.Contains(methodName, "Find") && strings.Contains(methodName, "s") {
				return "[]interface{}"
			}
			if strings.Contains(methodName, "Find") || strings.Contains(methodName, "Get") {
				return "interface{}"
			}
		}
	case *ast.SliceExpr, *ast.IndexExpr:
		// Handle slice/array expressions
		return "[]interface{}"
	}
	return ""
}

// extractTypeName extracts type name from AST expressions
// For qualified types like searchresult.ExportData, returns the full qualified name
func (p *ASTParser) extractTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		// For qualified types like searchresult.ExportData, return the full qualified name
		if x, ok := e.X.(*ast.Ident); ok {
			return x.Name + "." + e.Sel.Name
		}
		return e.Sel.Name
	case *ast.StarExpr:
		return p.extractTypeName(e.X)
	case *ast.ArrayType:
		return "[]" + p.extractTypeName(e.Elt)
	case *ast.MapType:
		// Extract key and value types for map types
		keyType := p.extractTypeName(e.Key)
		valueType := p.extractTypeName(e.Value)
		return "map[" + keyType + "]" + valueType
	}
	return ""
}

// generateStructSchema generates OpenAPI schema for a struct
// It flattens embedded struct fields into the parent schema (Go's promotion semantics)
func (p *ASTParser) generateStructSchema(structInfo *StructInfo) *OpenAPISchema {
	schema := &OpenAPISchema{
		Type:       "object",
		Properties: make(map[string]*OpenAPISchema),
	}

	// First, add fields from embedded structs (promoted fields)
	for _, embeddedType := range structInfo.Embedded {
		resolved, _ := p.resolveTypeAlias(embeddedType, nil)
		if embeddedStruct, exists := p.structs[resolved]; exists {
			for _, fieldInfo := range embeddedStruct.Fields {
				fieldName := fieldInfo.JSONName
				if fieldName == "" {
					fieldName = fieldInfo.Name
				}
				// Only add if not already present (parent fields take precedence)
				if _, exists := schema.Properties[fieldName]; !exists {
					schema.Properties[fieldName] = p.generateFieldSchema(fieldInfo.Type)
				}
			}
			// Recurse: if the embedded struct itself has embeds, they're already
			// flattened into embeddedStruct.Fields during its own schema generation,
			// BUT that only happens for JSONSchema. For Fields map, we need to also
			// walk the embedded struct's Embedded list.
			p.flattenEmbeddedFields(embeddedStruct, schema)
		}
	}

	// Then add the struct's own fields (override any promoted fields)
	for _, fieldInfo := range structInfo.Fields {
		fieldName := fieldInfo.JSONName
		if fieldName == "" {
			fieldName = fieldInfo.Name
		}

		fieldSchema := p.generateFieldSchema(fieldInfo.Type)
		// Mark pointer fields as nullable in OpenAPI 3.0
		if fieldInfo.IsPointer && fieldSchema != nil && fieldSchema.Ref == "" {
			fieldSchema.Nullable = boolPtr(true)
		}
		schema.Properties[fieldName] = fieldSchema
	}

	return schema
}

// flattenEmbeddedFields recursively adds promoted fields from nested embeds
func (p *ASTParser) flattenEmbeddedFields(structInfo *StructInfo, schema *OpenAPISchema) {
	for _, embeddedType := range structInfo.Embedded {
		resolved, _ := p.resolveTypeAlias(embeddedType, nil)
		if embeddedStruct, exists := p.structs[resolved]; exists {
			for _, fieldInfo := range embeddedStruct.Fields {
				fieldName := fieldInfo.JSONName
				if fieldName == "" {
					fieldName = fieldInfo.Name
				}
				if _, exists := schema.Properties[fieldName]; !exists {
					schema.Properties[fieldName] = p.generateFieldSchema(fieldInfo.Type)
				}
			}
			p.flattenEmbeddedFields(embeddedStruct, schema)
		}
	}
}

// generateFieldSchema generates OpenAPI schema for a field type
// This is now a wrapper around generateSchemaFromType for consistency
// Uses inline=false to generate $ref for nested types (2nd level)
func (p *ASTParser) generateFieldSchema(fieldType string) *OpenAPISchema {
	return p.generateSchemaFromType(fieldType, false)
}

// deepCopySchema creates a deep copy of an OpenAPISchema
// This is needed when returning inline schemas to avoid modifying the original
func (p *ASTParser) deepCopySchema(src *OpenAPISchema) *OpenAPISchema {
	if src == nil {
		return nil
	}

	dst := &OpenAPISchema{
		Type:        src.Type,
		Format:      src.Format,
		Title:       src.Title,
		Description: src.Description,
		Default:     src.Default,
		Example:     src.Example,
		Pattern:     src.Pattern,
		Ref:         src.Ref,
	}

	// Copy validation fields
	if src.MultipleOf != nil {
		val := *src.MultipleOf
		dst.MultipleOf = &val
	}
	if src.Maximum != nil {
		val := *src.Maximum
		dst.Maximum = &val
	}
	if src.ExclusiveMaximum != nil {
		val := *src.ExclusiveMaximum
		dst.ExclusiveMaximum = &val
	}
	if src.Minimum != nil {
		val := *src.Minimum
		dst.Minimum = &val
	}
	if src.ExclusiveMinimum != nil {
		val := *src.ExclusiveMinimum
		dst.ExclusiveMinimum = &val
	}
	if src.MaxLength != nil {
		val := *src.MaxLength
		dst.MaxLength = &val
	}
	if src.MinLength != nil {
		val := *src.MinLength
		dst.MinLength = &val
	}
	if src.MaxItems != nil {
		val := *src.MaxItems
		dst.MaxItems = &val
	}
	if src.MinItems != nil {
		val := *src.MinItems
		dst.MinItems = &val
	}
	if src.UniqueItems != nil {
		val := *src.UniqueItems
		dst.UniqueItems = &val
	}
	if src.MaxProperties != nil {
		val := *src.MaxProperties
		dst.MaxProperties = &val
	}
	if src.MinProperties != nil {
		val := *src.MinProperties
		dst.MinProperties = &val
	}

	// Copy arrays and slices
	if src.Required != nil {
		dst.Required = make([]string, len(src.Required))
		copy(dst.Required, src.Required)
	}
	if src.Enum != nil {
		dst.Enum = make([]interface{}, len(src.Enum))
		copy(dst.Enum, src.Enum)
	}
	if src.AllOf != nil {
		dst.AllOf = make([]*OpenAPISchema, len(src.AllOf))
		for i, schema := range src.AllOf {
			dst.AllOf[i] = p.deepCopySchema(schema)
		}
	}
	if src.OneOf != nil {
		dst.OneOf = make([]*OpenAPISchema, len(src.OneOf))
		for i, schema := range src.OneOf {
			dst.OneOf[i] = p.deepCopySchema(schema)
		}
	}
	if src.AnyOf != nil {
		dst.AnyOf = make([]*OpenAPISchema, len(src.AnyOf))
		for i, schema := range src.AnyOf {
			dst.AnyOf[i] = p.deepCopySchema(schema)
		}
	}

	// Copy properties map
	if src.Properties != nil {
		dst.Properties = make(map[string]*OpenAPISchema)
		for k, v := range src.Properties {
			dst.Properties[k] = p.deepCopySchema(v)
		}
	}

	// Copy AdditionalProperties
	dst.AdditionalProperties = src.AdditionalProperties

	// Copy Items
	if src.Items != nil {
		dst.Items = p.deepCopySchema(src.Items)
	}

	// Copy Not
	if src.Not != nil {
		dst.Not = p.deepCopySchema(src.Not)
	}

	// Copy Discriminator
	if src.Discriminator != nil {
		dst.Discriminator = &OpenAPIDiscriminator{
			PropertyName: src.Discriminator.PropertyName,
		}
		if src.Discriminator.Mapping != nil {
			dst.Discriminator.Mapping = make(map[string]string)
			for k, v := range src.Discriminator.Mapping {
				dst.Discriminator.Mapping[k] = v
			}
		}
	}

	// Copy boolean pointers
	if src.ReadOnly != nil {
		val := *src.ReadOnly
		dst.ReadOnly = &val
	}
	if src.WriteOnly != nil {
		val := *src.WriteOnly
		dst.WriteOnly = &val
	}
	if src.Deprecated != nil {
		val := *src.Deprecated
		dst.Deprecated = &val
	}
	if src.Nullable != nil {
		val := *src.Nullable
		dst.Nullable = &val
	}

	// Copy ExternalDocs
	if src.ExternalDocs != nil {
		dst.ExternalDocs = &OpenAPIExternalDocs{
			Description: src.ExternalDocs.Description,
			URL:         src.ExternalDocs.URL,
		}
	}

	return dst
}

// =============================================================================
// Enhanced API Methods for Schema Generator Integration
// =============================================================================

// EnhanceEndpoint enhances an endpoint with AST analysis
func (p *ASTParser) EnhanceEndpoint(endpoint *APIEndpoint) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Try multiple handler name variations for better matching
	handlerNames := []string{
		endpoint.Handler, // Full name
		ExtractHandlerBaseName(endpoint.Handler, false), // Without package, keep suffixes
		ExtractHandlerBaseName(endpoint.Handler, true),  // Without package and suffixes
	}

	for _, handlerName := range handlerNames {
		if handlerInfo, exists := p.handlers[handlerName]; exists {
			// Set authentication info
			if handlerInfo.RequiresAuth {
				endpoint.Auth = &AuthInfo{
					Required:    true,
					Type:        handlerInfo.AuthType,
					Description: p.getAuthDescription(handlerInfo.AuthType),
				}
			}

			// Set description and tags
			if handlerInfo.APIDescription != "" {
				endpoint.Description = handlerInfo.APIDescription
			}
			if len(handlerInfo.APITags) > 0 {
				endpoint.Tags = handlerInfo.APITags
			}

			// Set request and response schemas
			if handlerInfo.RequestSchema != nil {
				endpoint.Request = handlerInfo.RequestSchema
			}
			if handlerInfo.ResponseSchema != nil {
				endpoint.Response = handlerInfo.ResponseSchema
			}

			// Store enhanced data in handler info for later use
			// Note: APIEndpoint doesn't have Data field, so we store in handler info
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

// analyzeMapLiteralSchema analyzes composite literals (map, struct, slice) to generate schemas
// Returns nil if the expression is not a composite literal, so callers can fall through to other checks
func (p *ASTParser) analyzeMapLiteralSchema(expr ast.Expr, handlerInfo ...*ASTHandlerInfo) *OpenAPISchema {
	var hi *ASTHandlerInfo
	if len(handlerInfo) > 0 {
		hi = handlerInfo[0]
	}
	if compLit, ok := expr.(*ast.CompositeLit); ok {
		return p.analyzeCompositeLitSchema(compLit, hi)
	}
	return nil
}

// parseMapLiteral parses a map literal and generates a JSON schema
func (p *ASTParser) parseMapLiteral(mapLit *ast.CompositeLit, handlerInfo *ASTHandlerInfo) *OpenAPISchema {
	schema := &OpenAPISchema{
		Type:       "object",
		Properties: make(map[string]*OpenAPISchema),
	}

	required := []string{}

	for _, elt := range mapLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			// Extract key name
			var keyName string
			if basicLit, ok := kv.Key.(*ast.BasicLit); ok && basicLit.Kind.String() == "STRING" {
				keyName = strings.Trim(basicLit.Value, `"`)
			}

			if keyName != "" {
				// Analyze value type using generic inference
				valueSchema := p.analyzeValueExpression(kv.Value, handlerInfo)
				if valueSchema != nil {
					schema.Properties[keyName] = valueSchema
					// Consider most fields required (can be refined later)
					if keyName != "error" && keyName != "message" && keyName != "description" {
						required = append(required, keyName)
					}
				}
			}
		}
	}

	if len(required) > 0 {
		schema.Required = required
	}

	return schema
}

// analyzeValueExpression analyzes the value in a key-value pair to determine its schema
// handlerInfo is optional — when provided, enables resolving variable and field references
func (p *ASTParser) analyzeValueExpression(expr ast.Expr, handlerInfo *ASTHandlerInfo) *OpenAPISchema {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind.String() {
		case "STRING":
			return &OpenAPISchema{
				Type:    "string",
				Example: strings.Trim(e.Value, `"`),
			}
		case "INT":
			return &OpenAPISchema{
				Type:    "integer",
				Example: e.Value,
			}
		case "FLOAT":
			return &OpenAPISchema{
				Type:    "number",
				Example: e.Value,
			}
		}
	case *ast.Ident:
		switch e.Name {
		case "true", "false":
			return &OpenAPISchema{
				Type:    "boolean",
				Example: e.Name == "true",
			}
		case "nil":
			return &OpenAPISchema{Type: "object"}
		default:
			// Try to resolve the variable type from handler context
			if handlerInfo != nil {
				// First check if there's a traced expression we can analyze
				if tracedExpr, exists := handlerInfo.VariableExprs[e.Name]; exists {
					// Unwrap &expr
					inner := tracedExpr
					if unary, ok := inner.(*ast.UnaryExpr); ok && unary.Op == token.AND {
						inner = unary.X
					}
					// Try composite literal analysis (map/struct/slice literals)
					if schema := p.analyzeMapLiteralSchema(inner, handlerInfo); schema != nil {
						// Merge dynamic map additions for this variable
						p.mergeMapAdditions(schema, e.Name, handlerInfo)
						return schema
					}
					// Try full expression analysis (handles make(), function calls, etc.)
					if schema := p.analyzeValueExpression(inner, handlerInfo); schema != nil && schema.Type != "string" {
						// Merge dynamic map additions for this variable
						p.mergeMapAdditions(schema, e.Name, handlerInfo)
						// If this is a generic array, try to resolve items from append() patterns
						schema = p.enrichArraySchemaFromAppend(schema, e.Name, handlerInfo)
						return schema
					}
				}
				// Then check the inferred type name
				if varType, exists := handlerInfo.Variables[e.Name]; exists {
					if schema := p.resolveTypeToSchema(varType); schema != nil {
						// Merge dynamic map additions for this variable
						p.mergeMapAdditions(schema, e.Name, handlerInfo)
						// If this is a generic array, try to resolve items from append() patterns
						schema = p.enrichArraySchemaFromAppend(schema, e.Name, handlerInfo)
						return schema
					}
				}
			}
			// Default to string for identifiers we can't resolve
			return &OpenAPISchema{Type: "string"}
		}
	case *ast.CompositeLit:
		return p.analyzeCompositeLitSchema(e, handlerInfo)
	case *ast.UnaryExpr:
		// Handle &SomeStruct{...} — unwrap and analyze
		if e.Op == token.AND {
			return p.analyzeValueExpression(e.X, handlerInfo)
		}
	case *ast.StarExpr:
		// Handle pointer dereference *expr — unwrap and analyze the inner expression
		return p.analyzeValueExpression(e.X, handlerInfo)
	case *ast.CallExpr:
		// Handle method calls that return specific types
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "GetString":
				return &OpenAPISchema{Type: "string"}
			case "GetBool":
				return &OpenAPISchema{Type: "boolean"}
			case "GetInt", "GetFloat":
				return &OpenAPISchema{Type: "number"}
			case "GetDateTime":
				return &OpenAPISchema{
					Type:   "string",
					Format: "date-time",
				}
			case "Format":
				// Handle time.Format calls
				if x, ok := sel.X.(*ast.CallExpr); ok {
					if s, ok := x.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Now" {
						return &OpenAPISchema{
							Type:   "string",
							Format: "date-time",
						}
					}
				}
				return &OpenAPISchema{Type: "string"}
			case "Unix", "UnixNano":
				return &OpenAPISchema{Type: "integer"}
			}
		}
		// Handle direct function calls
		if ident, ok := e.Fun.(*ast.Ident); ok {
			switch ident.Name {
			case "len":
				return &OpenAPISchema{Type: "integer", Minimum: floatPtr(0)}
			case "make":
				// Try to infer the type from the make() argument
				if len(e.Args) > 0 {
					if schema := p.schemaFromMakeArg(e.Args[0]); schema != nil {
						return schema
					}
				}
				return &OpenAPISchema{
					Type:  "array",
					Items: &OpenAPISchema{Type: "object"},
				}
			default:
				// Check if we have a deep-analyzed schema from function body analysis
				if bodySchema, ok := p.funcBodySchemas[ident.Name]; ok {
					return p.deepCopySchema(bodySchema)
				}
				// Fall back to return type from function signature analysis
				if retType, ok := p.funcReturnTypes[ident.Name]; ok {
					if schema := p.resolveTypeToSchema(retType); schema != nil {
						return schema
					}
				}
				if strings.Contains(ident.Name, "String") {
					return &OpenAPISchema{Type: "string"}
				}
				if strings.Contains(ident.Name, "Int") || strings.Contains(ident.Name, "Count") {
					return &OpenAPISchema{Type: "integer"}
				}
			}
		}
	case *ast.IndexExpr:
		// Handle map index expressions: mapVar["key"]
		// If mapVar was assigned from a function call whose body schema is known,
		// look up the key's type from that schema.
		if ident, ok := e.X.(*ast.Ident); ok && handlerInfo != nil {
			if keyLit, ok := e.Index.(*ast.BasicLit); ok && keyLit.Kind == token.STRING {
				key := strings.Trim(keyLit.Value, `"`)
				// Try to resolve mapVar to a funcBodySchemas entry via its traced expression
				if tracedExpr, exists := handlerInfo.VariableExprs[ident.Name]; exists {
					if callExpr, ok := tracedExpr.(*ast.CallExpr); ok {
						if fnIdent, ok := callExpr.Fun.(*ast.Ident); ok {
							if bodySchema, ok := p.funcBodySchemas[fnIdent.Name]; ok {
								if bodySchema.Properties != nil {
									if propSchema, ok := bodySchema.Properties[key]; ok {
										return p.deepCopySchema(propSchema)
									}
								}
							}
						}
					}
				}
			}
		}
		// Fall through to generic handling — we can't determine type from map[string]any index
		return &OpenAPISchema{Type: "string"}
	case *ast.SelectorExpr:
		// Handle req.Field access — try to resolve the field type from struct definitions
		if ident, ok := e.X.(*ast.Ident); ok && handlerInfo != nil {
			fieldName := e.Sel.Name
			// Check if the receiver is a known variable with a known struct type
			if varType, exists := handlerInfo.Variables[ident.Name]; exists {
				// Strip pointer/slice prefixes to get the struct name
				structName := strings.TrimPrefix(varType, "*")
				structName = strings.TrimPrefix(structName, "[]")
				if structInfo, exists := p.structs[structName]; exists {
					// Look up the field in the struct
					for _, fi := range structInfo.Fields {
						if fi.Name == fieldName {
							return p.resolveTypeToSchema(fi.Type)
						}
					}
				}
			}
		}

		// Handle method calls and property access generically
		if sel := e.Sel.Name; sel != "" {
			// PocketBase record getter methods
			if strings.HasPrefix(sel, "Get") {
				switch {
				case strings.Contains(sel, "String"):
					return &OpenAPISchema{Type: "string"}
				case strings.Contains(sel, "Bool"):
					return &OpenAPISchema{Type: "boolean"}
				case strings.Contains(sel, "Int") || strings.Contains(sel, "Float"):
					return &OpenAPISchema{Type: "number"}
				case strings.Contains(sel, "DateTime") || strings.Contains(sel, "Time"):
					return &OpenAPISchema{Type: "string", Format: "date-time"}
				default:
					return &OpenAPISchema{Type: "string"}
				}
			}

			// Generic property inference
			if strings.Contains(sel, "Id") || strings.HasSuffix(sel, "ID") {
				return &OpenAPISchema{Type: "string"}
			}
			if strings.Contains(sel, "Time") || strings.Contains(sel, "At") || strings.Contains(sel, "Date") {
				return &OpenAPISchema{Type: "string", Format: "date-time"}
			}
			if strings.Contains(sel, "Count") || sel == "Unix" || sel == "UnixNano" {
				return &OpenAPISchema{Type: "integer"}
			}
		}
	}

	// Default to string for unknown expressions instead of generic object
	return &OpenAPISchema{Type: "string"}
}

// resolveTypeToSchema converts a Go type name to an OpenAPI schema
// Used for resolving variable and field types in map literal values
func (p *ASTParser) resolveTypeToSchema(typeName string) *OpenAPISchema {
	// Handle slice types
	if strings.HasPrefix(typeName, "[]") {
		elemType := strings.TrimPrefix(typeName, "[]")
		elemSchema := p.resolveTypeToSchema(elemType)
		if elemSchema != nil {
			return &OpenAPISchema{
				Type:  "array",
				Items: elemSchema,
			}
		}
		return &OpenAPISchema{
			Type:  "array",
			Items: &OpenAPISchema{Type: "object"},
		}
	}

	// Handle map types
	if strings.HasPrefix(typeName, "map[") {
		closeBracket := strings.Index(typeName, "]")
		if closeBracket > 0 && closeBracket < len(typeName)-1 {
			valueType := typeName[closeBracket+1:]
			// map[string]any / map[string]interface{} → free-form object
			if valueType == "any" || valueType == "interface{}" {
				return &OpenAPISchema{
					Type:                 "object",
					AdditionalProperties: true,
				}
			}
			valueSchema := p.resolveTypeToSchema(valueType)
			if valueSchema != nil {
				return &OpenAPISchema{
					Type:                 "object",
					AdditionalProperties: valueSchema,
				}
			}
		}
		return &OpenAPISchema{
			Type:                 "object",
			AdditionalProperties: true,
		}
	}

	// Handle pointer types
	if strings.HasPrefix(typeName, "*") {
		return p.resolveTypeToSchema(strings.TrimPrefix(typeName, "*"))
	}

	// Primitives
	switch typeName {
	case "string":
		return &OpenAPISchema{Type: "string"}
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return &OpenAPISchema{Type: "integer"}
	case "float32", "float64":
		return &OpenAPISchema{Type: "number"}
	case "bool":
		return &OpenAPISchema{Type: "boolean"}
	case "time.Time", "Time":
		return &OpenAPISchema{Type: "string", Format: "date-time"}
	case "interface{}", "any":
		return &OpenAPISchema{Type: "object", AdditionalProperties: true}
	}

	// Known struct → $ref
	resolved, _ := p.resolveTypeAlias(typeName, nil)
	if _, exists := p.structs[resolved]; exists {
		return &OpenAPISchema{Ref: "#/components/schemas/" + resolved}
	}
	if _, exists := p.structs[typeName]; exists {
		return &OpenAPISchema{Ref: "#/components/schemas/" + typeName}
	}

	return nil
}

// schemaFromMakeArg infers a schema from the type argument of make()
func (p *ASTParser) schemaFromMakeArg(typeExpr ast.Expr) *OpenAPISchema {
	typeName := p.extractTypeName(typeExpr)
	if typeName == "" {
		return nil
	}
	return p.resolveTypeToSchema(typeName)
}

// analyzeCompositeLitSchema analyzes a composite literal (struct, map, or slice) and returns a schema
func (p *ASTParser) analyzeCompositeLitSchema(e *ast.CompositeLit, handlerInfo *ASTHandlerInfo) *OpenAPISchema {
	if e.Type == nil {
		// Bare composite literal inside a slice: []SomeType{ {field: val}, {field: val} }
		// These appear as CompositeLit with nil Type — they inherit the type from parent
		// Try to build an inline object from key-value pairs
		if len(e.Elts) > 0 {
			if _, ok := e.Elts[0].(*ast.KeyValueExpr); ok {
				// Looks like a struct or map literal with keys
				schema := &OpenAPISchema{
					Type:       "object",
					Properties: make(map[string]*OpenAPISchema),
				}
				for _, elt := range e.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						var keyName string
						// Struct field: Key is *ast.Ident
						if ident, ok := kv.Key.(*ast.Ident); ok {
							keyName = ident.Name
						}
						// Map key: Key is *ast.BasicLit
						if basicLit, ok := kv.Key.(*ast.BasicLit); ok && basicLit.Kind.String() == "STRING" {
							keyName = strings.Trim(basicLit.Value, `"`)
						}
						if keyName != "" {
							schema.Properties[keyName] = p.analyzeValueExpression(kv.Value, handlerInfo)
						}
					}
				}
				return schema
			}
		}
		return &OpenAPISchema{Type: "object"}
	}

	// Handle nested map literals: map[string]any{...}
	if _, ok := e.Type.(*ast.MapType); ok {
		return p.parseMapLiteral(e, handlerInfo)
	}

	// Handle slice/array literals: []Type{...}
	if arrayType, ok := e.Type.(*ast.ArrayType); ok {
		elemTypeName := p.extractTypeName(arrayType.Elt)
		if elemTypeName != "" {
			// Use generateSchemaForEndpoint to get $ref for known structs
			elemSchema := p.generateSchemaForEndpoint(elemTypeName)
			if elemSchema != nil {
				return &OpenAPISchema{
					Type:  "array",
					Items: elemSchema,
				}
			}
		}
		// Fallback: try to infer from first element
		return p.parseArrayLiteral(e, handlerInfo)
	}

	// Handle struct composite literals: SomeStruct{...}
	typeName := p.extractTypeName(e.Type)
	if typeName != "" {
		// Check if it's a known struct — use $ref
		resolvedType, _ := p.resolveTypeAlias(typeName, nil)
		if _, exists := p.structs[resolvedType]; exists {
			return &OpenAPISchema{
				Ref: "#/components/schemas/" + resolvedType,
			}
		}
		if _, exists := p.structs[typeName]; exists {
			return &OpenAPISchema{
				Ref: "#/components/schemas/" + typeName,
			}
		}
		// If it's a map type written as a named composite literal
		if strings.HasPrefix(typeName, "map[") {
			return p.generateSchemaFromType(typeName, false)
		}
	}

	return &OpenAPISchema{Type: "object"}
}

// parseArrayLiteral parses an array literal
func (p *ASTParser) parseArrayLiteral(arrayLit *ast.CompositeLit, handlerInfo *ASTHandlerInfo) *OpenAPISchema {
	schema := &OpenAPISchema{
		Type:  "array",
		Items: &OpenAPISchema{Type: "object"},
	}

	// Try to infer item type from first element
	if len(arrayLit.Elts) > 0 {
		if itemSchema := p.analyzeValueExpression(arrayLit.Elts[0], handlerInfo); itemSchema != nil {
			schema.Items = itemSchema
		}
	}

	return schema
}

// extractVariableDeclarations extracts variable declarations from the file
func (p *ASTParser) extractVariableDeclarations(file *ast.File, globalVars *ASTHandlerInfo) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			if node.Tok == token.VAR {
				for _, spec := range node.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range valueSpec.Names {
							if valueSpec.Type != nil {
								typeName := p.extractTypeName(valueSpec.Type)
								if typeName != "" {
									globalVars.Variables[name.Name] = typeName
								}
							} else if i < len(valueSpec.Values) {
								// Infer type from value
								if typeName := p.inferTypeFromExpression(valueSpec.Values[i], globalVars); typeName != "" {
									globalVars.Variables[name.Name] = typeName
								}
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			// Handle short variable declarations: req := TodoRequest{}
			if node.Tok == token.DEFINE {
				for i, lhs := range node.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && i < len(node.Rhs) {
						if typeName := p.inferTypeFromExpression(node.Rhs[i], globalVars); typeName != "" {
							globalVars.Variables[ident.Name] = typeName
						}
					}
				}
			}
		}
		return true
	})
}

// extractLocalVariables extracts variable declarations from within handler functions
func (p *ASTParser) extractLocalVariables(body *ast.BlockStmt, handlerInfo *ASTHandlerInfo) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.DeclStmt:
			if genDecl, ok := node.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
				p.extractVarDecl(genDecl, handlerInfo)
			}
		case *ast.AssignStmt:
			// Handle short variable declarations: req := TodoRequest{}
			if node.Tok == token.DEFINE {
				for i, lhs := range node.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && i < len(node.Rhs) {
						rhs := node.Rhs[i]
						if typeName := p.inferTypeFromExpression(rhs, handlerInfo); typeName != "" {
							handlerInfo.Variables[ident.Name] = typeName
						}
						// Store the RHS expression for map literal analysis
						handlerInfo.VariableExprs[ident.Name] = rhs
					}
				}
			}
		}
		return true
	})
}

// extractVarDecl extracts variable declarations from GenDecl nodes
func (p *ASTParser) extractVarDecl(genDecl *ast.GenDecl, handlerInfo *ASTHandlerInfo) {
	for _, spec := range genDecl.Specs {
		if valueSpec, ok := spec.(*ast.ValueSpec); ok {
			for i, name := range valueSpec.Names {
				if valueSpec.Type != nil {
					typeName := p.extractTypeName(valueSpec.Type)
					if typeName != "" {
						handlerInfo.Variables[name.Name] = typeName
					}
				}
				if i < len(valueSpec.Values) {
					rhs := valueSpec.Values[i]
					if valueSpec.Type == nil {
						// Infer type from value
						if typeName := p.inferTypeFromExpression(rhs, handlerInfo); typeName != "" {
							handlerInfo.Variables[name.Name] = typeName
						}
					}
					// Store the RHS expression for map literal analysis
					handlerInfo.VariableExprs[name.Name] = rhs
				}
			}
		}
	}
}

// resolveTypeAlias resolves a type alias recursively to find the real type
// Returns the resolved type name and a boolean indicating if it was an alias
// Handles qualified types (e.g., searchresult.ExportData) by extracting the simple name
func (p *ASTParser) resolveTypeAlias(typeName string, visited map[string]bool) (string, bool) {
	if visited == nil {
		visited = make(map[string]bool)
	}

	// Prevent infinite loops
	if visited[typeName] {
		return typeName, false
	}
	visited[typeName] = true

	// Check if it's a direct alias
	if realType, exists := p.typeAliases[typeName]; exists {
		// Resolve recursively in case of alias chains
		resolved, _ := p.resolveTypeAlias(realType, visited)
		return resolved, true
	}

	// Handle qualified types (e.g., searchresult.ExportData)
	// Extract the simple name (ExportData) and check if it's registered as a struct
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		simpleName := parts[len(parts)-1]

		// Check if the simple name is an alias
		if realType, exists := p.typeAliases[simpleName]; exists {
			resolved, _ := p.resolveTypeAlias(realType, visited)
			return resolved, true
		}

		// Check if the simple name is a registered struct
		if _, exists := p.structs[simpleName]; exists {
			return simpleName, false
		}

		// Return the simple name as fallback
		return simpleName, false
	}

	return typeName, false
}

// generateSchemaForEndpoint generates an OpenAPI schema for a top-level endpoint request/response.
// Unlike generateSchemaFromType, this always uses $ref for known struct types so they appear
// in components/schemas rather than being inlined. For arrays of structs, it wraps $ref in items.
// For non-struct types (primitives, maps, etc.) it delegates to generateSchemaFromType.
func (p *ASTParser) generateSchemaForEndpoint(typeName string) *OpenAPISchema {
	// Handle array types: []StructName → { type: "array", items: { $ref: ... } }
	if strings.HasPrefix(typeName, "[]") {
		elementType := strings.TrimPrefix(typeName, "[]")
		elementSchema := p.generateSchemaForEndpoint(elementType)
		if elementSchema != nil {
			return &OpenAPISchema{
				Type:  "array",
				Items: elementSchema,
			}
		}
		return &OpenAPISchema{
			Type:  "array",
			Items: &OpenAPISchema{Type: "object"},
		}
	}

	// Resolve type aliases
	resolvedType, _ := p.resolveTypeAlias(typeName, nil)

	// If it's a known struct, always use $ref (standard OpenAPI pattern)
	if _, exists := p.structs[resolvedType]; exists {
		return &OpenAPISchema{
			Ref: "#/components/schemas/" + resolvedType,
		}
	}
	// Also check original name
	if _, exists := p.structs[typeName]; exists {
		return &OpenAPISchema{
			Ref: "#/components/schemas/" + typeName,
		}
	}

	// For everything else (primitives, maps, any, etc.), use the existing logic
	return p.generateSchemaFromType(typeName, false)
}

// generateSchemaFromType generates an OpenAPI schema from a type name
// inline: if true, returns the full schema for struct types; if false, returns $ref for struct types
// This allows endpoints to have inline schemas while nested types use $ref
func (p *ASTParser) generateSchemaFromType(typeName string, inline bool) *OpenAPISchema {
	// Handle array types
	if strings.HasPrefix(typeName, "[]") {
		elementType := strings.TrimPrefix(typeName, "[]")
		elementSchema := p.generateSchemaFromType(elementType, inline)
		if elementSchema != nil {
			return &OpenAPISchema{
				Type:  "array",
				Items: elementSchema,
			}
		}
		return &OpenAPISchema{
			Type:  "array",
			Items: &OpenAPISchema{Type: "object"},
		}
	}

	// Resolve type alias before checking for structs
	resolvedType, isAlias := p.resolveTypeAlias(typeName, nil)

	// Use resolved type for further processing
	// If it was an alias, we want to use the resolved type name for the schema reference
	typeToUse := resolvedType
	if isAlias {
		// If we resolved an alias, use the resolved type
		typeToUse = resolvedType
	} else {
		// If not an alias, use the original type name
		typeToUse = typeName
	}

	// Handle primitive types and well-known stdlib types (check both original and resolved)
	for _, t := range []string{typeName, typeToUse} {
		switch t {
		case "string":
			return &OpenAPISchema{Type: "string"}
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64":
			return &OpenAPISchema{Type: "integer"}
		case "float64", "float32":
			return &OpenAPISchema{Type: "number"}
		case "bool":
			return &OpenAPISchema{Type: "boolean"}
		case "time.Time", "Time":
			return &OpenAPISchema{Type: "string", Format: "date-time"}
		case "interface{}", "any":
			return &OpenAPISchema{Type: "object", AdditionalProperties: true}
		}
	}

	// Look for struct definition using resolved type
	if structInfo, exists := p.structs[typeToUse]; exists {
		// Check if JSONSchema is nil (shouldn't happen after two-pass parsing, but safety first)
		if structInfo.JSONSchema == nil {
			// Fallback: return a reference anyway, schema will be generated later
			return &OpenAPISchema{
				Ref: "#/components/schemas/" + typeToUse,
			}
		}

		// If inline=true, return the full schema; if inline=false, return $ref
		if inline {
			// Return a deep copy of the schema to avoid modifying the original
			return p.deepCopySchema(structInfo.JSONSchema)
		}

		// Use $ref for nested types (2nd level) to avoid duplication and handle circular references
		return &OpenAPISchema{
			Ref: "#/components/schemas/" + typeToUse,
		}
	}

	// Handle map types
	if strings.HasPrefix(typeName, "map[") {
		// Parse map type: map[keyType]valueType
		// Extract value type between ] and end
		closeBracket := strings.Index(typeName, "]")
		if closeBracket > 0 && closeBracket < len(typeName)-1 {
			valueType := typeName[closeBracket+1:]
			// map[string]any / map[string]interface{} → free-form object
			if valueType == "any" || valueType == "interface{}" {
				return &OpenAPISchema{
					Type:                 "object",
					AdditionalProperties: true,
				}
			}
			// Generate schema for the value type recursively
			valueSchema := p.generateSchemaFromType(valueType, false)
			if valueSchema != nil {
				return &OpenAPISchema{
					Type:                 "object",
					AdditionalProperties: valueSchema,
				}
			}
		}
		// Fallback if parsing fails
		return &OpenAPISchema{
			Type:                 "object",
			AdditionalProperties: true,
		}
	}

	// Handle interface{} / any or unknown types
	if typeName == "interface{}" || typeName == "any" {
		return &OpenAPISchema{
			Type:                 "object",
			AdditionalProperties: true,
		}
	}

	// If we resolved an alias but the resolved type is not a known struct,
	// it might be in another package. Try to use the resolved type name anyway.
	if isAlias {
		// Extract simple name from qualified type if needed
		simpleName := resolvedType
		if strings.Contains(resolvedType, ".") {
			parts := strings.Split(resolvedType, ".")
			simpleName = parts[len(parts)-1]
		}

		// Check if the simple name is a known struct
		if structInfo, exists := p.structs[simpleName]; exists {
			if inline {
				return p.deepCopySchema(structInfo.JSONSchema)
			}
			return &OpenAPISchema{
				Ref: "#/components/schemas/" + simpleName,
			}
		}

		// Use the resolved type name for the reference
		return &OpenAPISchema{
			Ref: "#/components/schemas/" + simpleName,
		}
	}

	// Default to object type with reference (but don't use additionalProperties for unknown types)
	// This was the original problematic behavior - we should avoid it now
	// Instead, try to create a reference based on the type name
	simpleName := typeName
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		simpleName = parts[len(parts)-1]
	}

	return &OpenAPISchema{
		Ref: "#/components/schemas/" + simpleName,
	}
}

// NewPocketBasePatterns creates PocketBase-specific patterns
func NewPocketBasePatterns() *PocketBasePatterns {
	return &PocketBasePatterns{
		RequestPatterns: map[string]RequestPattern{
			"BindBody": {
				Method:      "BindBody",
				Description: "PocketBase request body binding",
			},
		},
		ResponsePatterns: map[string]ResponsePattern{
			"EnrichRecord": {
				Method:      "EnrichRecord",
				ReturnType:  "Record",
				Description: "PocketBase record enrichment",
			},
		},
		AuthPatterns: []AuthPattern{
			{
				Pattern:     "RequireAuth",
				Required:    true,
				Description: "PocketBase authentication required",
			},
		},
	}
}
