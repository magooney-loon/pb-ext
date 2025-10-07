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

// NewASTParser creates a new simplified PocketBase-focused AST parser
func NewASTParser() *ASTParser {
	parser := &ASTParser{
		fileSet:            token.NewFileSet(),
		structs:            make(map[string]*StructInfo),
		handlers:           make(map[string]*ASTHandlerInfo),
		pocketbasePatterns: NewPocketBasePatterns(),
		logger:             &DefaultLogger{},
	}

	// Auto-discover source files
	if err := parser.DiscoverSourceFiles(); err != nil {
		parser.logger.Error("Failed to discover source files: %v", err)
	}

	return parser
}

// DiscoverSourceFiles finds and parses files with API_SOURCE directive
func (p *ASTParser) DiscoverSourceFiles() error {
	return filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		if strings.Contains(string(content), "// API_SOURCE") {
			return p.ParseFile(path)
		}

		return nil
	})
}

// ParseFile parses a single Go file for PocketBase patterns
func (p *ASTParser) ParseFile(filename string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	file, err := parser.ParseFile(p.fileSet, filename, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	// Extract structs (for request/response types)
	p.extractStructs(file)

	// Extract variable declarations first
	p.extractVariableDeclarations(file, &ASTHandlerInfo{Variables: make(map[string]string)})

	// Extract handlers
	p.extractHandlers(file)

	return nil
}

// extractStructs extracts struct definitions that might be used for requests/responses
func (p *ASTParser) extractStructs(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						structInfo := p.parseStruct(typeSpec, structType)
						p.structs[structInfo.Name] = structInfo
					}
				}
			}
		}
		return true
	})
}

// parseStruct parses a struct definition
func (p *ASTParser) parseStruct(typeSpec *ast.TypeSpec, structType *ast.StructType) *StructInfo {
	structInfo := &StructInfo{
		Name:   typeSpec.Name.Name,
		Fields: make(map[string]*FieldInfo),
	}

	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			fieldInfo := &FieldInfo{
				Name: name.Name,
				Type: p.extractTypeName(field.Type),
			}

			// Parse JSON tags
			if field.Tag != nil {
				p.parseJSONTag(field.Tag.Value, fieldInfo)
			}

			structInfo.Fields[fieldInfo.Name] = fieldInfo
		}
	}

	// Generate JSON schema
	structInfo.JSONSchema = p.generateStructSchema(structInfo)
	return structInfo
}

// parseJSONTag parses JSON struct tags
func (p *ASTParser) parseJSONTag(tagValue string, fieldInfo *FieldInfo) {
	tagValue = strings.Trim(tagValue, "`")
	if jsonTag := p.extractTag(tagValue, "json"); jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if len(parts) > 0 && parts[0] != "" && parts[0] != "-" {
			fieldInfo.JSONName = parts[0]
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
		Name:      funcDecl.Name.Name,
		Variables: make(map[string]string),
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
	if handlerInfo.RequestType != "" {
		if schema := p.generateSchemaFromType(handlerInfo.RequestType); schema != nil {
			handlerInfo.RequestSchema = schema
		}
	}

	// Additional pass to detect variable declarations within the handler
	if funcDecl.Body != nil {
		p.extractLocalVariables(funcDecl.Body, handlerInfo)
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
			p.handleNewDecoder(call, handlerInfo)
		}
	} else if ident, ok := call.Fun.(*ast.Ident); ok {
		// Handle direct function calls
		switch ident.Name {
		case "NewDecoder":
			p.handleNewDecoder(call, handlerInfo)
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
		// First try to analyze map literals for detailed schemas
		if schema := p.analyzeMapLiteralSchema(call.Args[1]); schema != nil {
			handlerInfo.ResponseSchema = schema
			return
		}

		// Then try type inference for struct-based responses
		if responseType := p.inferTypeFromExpression(call.Args[1], handlerInfo); responseType != "" {
			handlerInfo.ResponseType = responseType
			if schema := p.generateSchemaFromType(responseType); schema != nil {
				handlerInfo.ResponseSchema = schema
				return
			}
		}

		// Fallback: generate a basic object schema
		handlerInfo.ResponseSchema = map[string]interface{}{
			"type":                 "object",
			"description":          "Response data",
			"additionalProperties": true,
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
					if schema := p.generateSchemaFromType(varType); schema != nil {
						handlerInfo.RequestSchema = schema
					}
				}
			}
		}
	}
}

// handleNewDecoder handles json.NewDecoder(c.Request.Body) pattern
func (p *ASTParser) handleNewDecoder(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) {
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
			// Handle var declarations like: var req TodoRequest
			if varType := p.inferTypeFromExpression(assign.Rhs[i], handlerInfo); varType != "" {
				handlerInfo.Variables[ident.Name] = varType
			}
		}
	}
}

// inferTypeFromExpression infers type from expressions (enhanced)
func (p *ASTParser) inferTypeFromExpression(expr ast.Expr, handlerInfo *ASTHandlerInfo) string {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		// Handle struct literals like TodoRequest{...}
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
		// Handle common variable patterns
		if strings.HasSuffix(e.Name, "Request") || strings.HasSuffix(e.Name, "Response") {
			return e.Name
		}
	case *ast.CallExpr:
		// Handle constructor patterns
		if ident, ok := e.Fun.(*ast.Ident); ok {
			if strings.HasPrefix(ident.Name, "New") {
				return strings.TrimPrefix(ident.Name, "New")
			}
		}
		// Handle method calls that return specific types
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "FindRecordsByFilter", "FindRecordsByIds":
				return "[]Record"
			case "FindRecordById":
				return "Record"
			case "CreateRecord", "UpdateRecord":
				return "Record"
			}
		}
	case *ast.SliceExpr, *ast.IndexExpr:
		// Handle slice/array expressions
		return "[]interface{}"
	}
	return ""
}

// extractTypeName extracts type name from AST expressions
func (p *ASTParser) extractTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.StarExpr:
		return p.extractTypeName(e.X)
	case *ast.ArrayType:
		return "[]" + p.extractTypeName(e.Elt)
	}
	return ""
}

// generateStructSchema generates JSON schema for a struct
func (p *ASTParser) generateStructSchema(structInfo *StructInfo) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	properties := schema["properties"].(map[string]interface{})

	for _, fieldInfo := range structInfo.Fields {
		fieldName := fieldInfo.JSONName
		if fieldName == "" {
			fieldName = fieldInfo.Name
		}

		properties[fieldName] = p.generateFieldSchema(fieldInfo.Type)
	}

	return schema
}

// generateFieldSchema generates schema for a field type
func (p *ASTParser) generateFieldSchema(fieldType string) map[string]interface{} {
	switch fieldType {
	case "string":
		return map[string]interface{}{"type": "string"}
	case "int", "int64", "int32":
		return map[string]interface{}{"type": "integer"}
	case "float64", "float32":
		return map[string]interface{}{"type": "number"}
	case "bool":
		return map[string]interface{}{"type": "boolean"}
	default:
		if strings.HasPrefix(fieldType, "[]") {
			return map[string]interface{}{
				"type":  "array",
				"items": p.generateFieldSchema(strings.TrimPrefix(fieldType, "[]")),
			}
		}
		return map[string]interface{}{"type": "object"}
	}
}

// =============================================================================
// Enhanced API Methods for Schema Generator Integration
// =============================================================================

// EnhanceEndpoint enhances an endpoint with AST analysis
func (p *ASTParser) EnhanceEndpoint(endpoint *APIEndpoint) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	handlerName := ExtractHandlerNameFromPath(endpoint.Handler)
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

		// Store enhanced data in handler info for later use
		// Note: APIEndpoint doesn't have Data field, so we store in handler info
		handlerInfo.Variables["enhanced"] = "true"
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
	// Simplified implementation - no complex error tracking
	return []ParseError{}
}

// ClearCache clears all cached data (interface compatibility)
func (p *ASTParser) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.structs = make(map[string]*StructInfo)
	p.handlers = make(map[string]*ASTHandlerInfo)
}

// analyzeMapLiteralSchema analyzes map[string]any{...} literals to generate schemas
func (p *ASTParser) analyzeMapLiteralSchema(expr ast.Expr) map[string]interface{} {
	switch e := expr.(type) {
	case *ast.CompositeLit:
		// Check if it's a map literal
		if _, ok := e.Type.(*ast.MapType); ok {
			return p.parseMapLiteral(e)
		}
	}
	return nil
}

// parseMapLiteral parses a map literal and generates a JSON schema
func (p *ASTParser) parseMapLiteral(mapLit *ast.CompositeLit) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	properties := schema["properties"].(map[string]interface{})
	required := []string{}

	for _, elt := range mapLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			// Extract key name
			var keyName string
			if basicLit, ok := kv.Key.(*ast.BasicLit); ok && basicLit.Kind.String() == "STRING" {
				keyName = strings.Trim(basicLit.Value, `"`)
			}

			if keyName != "" {
				// Analyze value type with enhanced logic
				valueSchema := p.analyzeValueExpression(kv.Value)
				if valueSchema != nil {
					// Special handling for known patterns
					switch keyName {
					case "id", "user_id", "created_by":
						// IDs are typically strings, but check if the value suggests otherwise
						if valueSchema["type"] == "string" {
							valueSchema["description"] = "Unique identifier"
						}
					case "count", "total", "totalItems", "totalPages":
						// Counts should be integers
						if valueSchema["type"] != "integer" {
							valueSchema = map[string]interface{}{
								"type":        "integer",
								"minimum":     0,
								"description": "Count of items",
							}
						}
					case "todos", "items", "records":
						// These are typically arrays
						if valueSchema["type"] != "array" {
							valueSchema = map[string]interface{}{
								"type":  "array",
								"items": map[string]interface{}{"type": "object"},
							}
						}
					}

					properties[keyName] = valueSchema
					// Consider most fields required (can be refined later)
					if keyName != "error" && keyName != "message" && keyName != "description" {
						required = append(required, keyName)
					}
				}
			}
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// analyzeValueExpression analyzes the value in a key-value pair to determine its schema
func (p *ASTParser) analyzeValueExpression(expr ast.Expr) map[string]interface{} {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind.String() {
		case "STRING":
			return map[string]interface{}{
				"type":    "string",
				"example": strings.Trim(e.Value, `"`),
			}
		case "INT":
			return map[string]interface{}{
				"type":    "integer",
				"example": e.Value,
			}
		case "FLOAT":
			return map[string]interface{}{
				"type":    "number",
				"example": e.Value,
			}
		}
	case *ast.Ident:
		switch e.Name {
		case "true", "false":
			return map[string]interface{}{
				"type":    "boolean",
				"example": e.Name == "true",
			}
		default:
			// Enhanced variable reference handling
			varName := e.Name
			switch {
			case varName == "todos" || varName == "records" || varName == "items":
				return map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "object"},
				}
			case strings.Contains(varName, "count") || strings.Contains(varName, "len"):
				return map[string]interface{}{"type": "integer"}
			case strings.Contains(varName, "ID") || strings.Contains(varName, "Id"):
				return map[string]interface{}{"type": "string"}
			}
		}
	case *ast.CompositeLit:
		// Handle nested map literals
		if _, ok := e.Type.(*ast.MapType); ok {
			return p.parseMapLiteral(e)
		}
		// Handle slice literals
		if _, ok := e.Type.(*ast.ArrayType); ok {
			return p.parseArrayLiteral(e)
		}
	case *ast.CallExpr:
		// Handle method calls that return specific types
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "GetString":
				return map[string]interface{}{"type": "string"}
			case "GetBool":
				return map[string]interface{}{"type": "boolean"}
			case "GetInt", "GetFloat":
				return map[string]interface{}{"type": "number"}
			case "GetDateTime":
				return map[string]interface{}{
					"type":   "string",
					"format": "date-time",
				}
			case "Format":
				// Handle time.Format calls
				if x, ok := sel.X.(*ast.CallExpr); ok {
					if s, ok := x.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Now" {
						return map[string]interface{}{
							"type":   "string",
							"format": "date-time",
						}
					}
				}
				return map[string]interface{}{"type": "string"}
			case "Unix", "UnixNano":
				return map[string]interface{}{"type": "integer"}
			}
		}
		// Handle direct function calls
		if ident, ok := e.Fun.(*ast.Ident); ok {
			switch ident.Name {
			case "len":
				return map[string]interface{}{
					"type":        "integer",
					"minimum":     0,
					"description": "Length/count",
				}
			case "strconv.FormatInt":
				return map[string]interface{}{"type": "string"}
			case "make":
				// Handle make() calls - usually for slices/maps
				return map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "object"},
				}
			}
		}
	case *ast.SelectorExpr:
		// Handle property access like record.Id, record.Title
		if sel := e.Sel.Name; sel != "" {
			switch {
			case sel == "Id" || strings.HasSuffix(sel, "ID"):
				return map[string]interface{}{
					"type":        "string",
					"description": "Unique identifier",
				}
			case sel == "Name" || sel == "Title":
				return map[string]interface{}{"type": "string"}
			case strings.Contains(sel, "At") || strings.Contains(sel, "Time"):
				return map[string]interface{}{
					"type":   "string",
					"format": "date-time",
				}
			case strings.Contains(sel, "Bool"):
				return map[string]interface{}{"type": "boolean"}
			case strings.Contains(sel, "Int") || strings.Contains(sel, "Count"):
				return map[string]interface{}{"type": "integer"}
			}
		}
		// Handle variable references through selectors
		if x, ok := e.X.(*ast.Ident); ok {
			varName := x.Name
			// Check common variable patterns
			switch {
			case varName == "todos" || varName == "records" || varName == "items":
				return map[string]interface{}{
					"type":  "array",
					"items": map[string]interface{}{"type": "object"},
				}
			case strings.Contains(varName, "count") || strings.Contains(varName, "len"):
				return map[string]interface{}{"type": "integer"}
			}
		}
	}

	// Default to string for unknown expressions instead of generic object
	return map[string]interface{}{"type": "string"}
}

// parseArrayLiteral parses an array literal
func (p *ASTParser) parseArrayLiteral(arrayLit *ast.CompositeLit) map[string]interface{} {
	schema := map[string]interface{}{
		"type":  "array",
		"items": map[string]interface{}{"type": "object"},
	}

	// Try to infer item type from first element
	if len(arrayLit.Elts) > 0 {
		if itemSchema := p.analyzeValueExpression(arrayLit.Elts[0]); itemSchema != nil {
			schema["items"] = itemSchema
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
						if typeName := p.inferTypeFromExpression(node.Rhs[i], handlerInfo); typeName != "" {
							handlerInfo.Variables[ident.Name] = typeName
						}
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
				} else if i < len(valueSpec.Values) {
					// Infer type from value
					if typeName := p.inferTypeFromExpression(valueSpec.Values[i], handlerInfo); typeName != "" {
						handlerInfo.Variables[name.Name] = typeName
					}
				}
			}
		}
	}
}

// generateSchemaFromType generates a JSON schema from a type name
func (p *ASTParser) generateSchemaFromType(typeName string) map[string]interface{} {
	// Handle array types
	if strings.HasPrefix(typeName, "[]") {
		elementType := strings.TrimPrefix(typeName, "[]")
		elementSchema := p.generateSchemaFromType(elementType)
		if elementSchema != nil {
			return map[string]interface{}{
				"type":  "array",
				"items": elementSchema,
			}
		}
		return map[string]interface{}{
			"type":  "array",
			"items": map[string]interface{}{"type": "object"},
		}
	}

	// Handle primitive types
	switch typeName {
	case "string":
		return map[string]interface{}{"type": "string"}
	case "int", "int64", "int32":
		return map[string]interface{}{"type": "integer"}
	case "float64", "float32":
		return map[string]interface{}{"type": "number"}
	case "bool":
		return map[string]interface{}{"type": "boolean"}
	}

	// Look for struct definition
	if structInfo, exists := p.structs[typeName]; exists {
		return structInfo.JSONSchema
	}

	// Handle map types or unknown structs
	if strings.Contains(typeName, "map[") || typeName == "interface{}" {
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": true,
		}
	}

	// Default to object type with reference
	return map[string]interface{}{
		"type":                 "object",
		"description":          "Response object of type " + typeName,
		"additionalProperties": true,
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
