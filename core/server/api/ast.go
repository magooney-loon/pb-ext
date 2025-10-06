package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// =============================================================================
// AST Parser Implementation
// =============================================================================

// NewASTParser creates a new AST parser instance
func NewASTParser() *ASTParser {

	parser := &ASTParser{
		fileSet:    token.NewFileSet(),
		packages:   make(map[string]*ast.Package),
		structs:    make(map[string]*StructInfo),
		handlers:   make(map[string]*ASTHandlerInfo),
		imports:    make(map[string]string),
		typeCache:  make(map[string]*TypeInfo),
		fileCache:  make(map[string]*FileParseResult),
		validators: []TypeValidator{},
		logger:     &DefaultLogger{},
	}

	// Automatically discover and parse files with API_SOURCE directive
	if err := parser.DiscoverSourceFiles(); err != nil {
		// Discovery failed - continue without source files
	}

	return parser
}

// AddValidator adds a type validator to the parser
func (p *ASTParser) AddValidator(validator TypeValidator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.validators = append(p.validators, validator)
}

// ParseFile parses a single Go source file and extracts API information
func (p *ASTParser) ParseFile(filename string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if file needs reparsing
	if !p.shouldReparseFile(filename) {
		return nil
	}

	// Parse the file
	src, err := os.ReadFile(filename)
	if err != nil {
		return p.createParseError("file_read", err.Error(), filename, 0, 0, "")
	}

	file, err := parser.ParseFile(p.fileSet, filename, src, parser.ParseComments)
	if err != nil {
		return p.createParseError("parse_error", err.Error(), filename, 0, 0, string(src))
	}

	// Create parse result
	result := &FileParseResult{
		ModTime:            p.getFileModTime(filename),
		Structs:            make(map[string]*StructInfo),
		Handlers:           make(map[string]*ASTHandlerInfo),
		Imports:            make(map[string]string),
		RouteRegistrations: []*RouteRegistration{},
		Errors:             []ParseError{},
		ParsedAt:           time.Now(),
	}

	// Extract information from the AST
	p.extractImports(file, result)
	p.extractStructs(file)
	p.extractHandlers(file, result)

	// Cache the result
	p.fileCache[filename] = result
	p.mergeParseResult(result)

	return nil
}

// shouldReparseFile checks if a file needs to be reparsed
func (p *ASTParser) shouldReparseFile(filename string) bool {
	cached, exists := p.fileCache[filename]
	if !exists {
		return true
	}

	currentModTime := p.getFileModTime(filename)
	return currentModTime.After(cached.ModTime)
}

// getFileModTime gets the modification time of a file
func (p *ASTParser) getFileModTime(filename string) time.Time {
	if info, err := os.Stat(filename); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

// mergeParseResult merges a parse result into the main data structures
func (p *ASTParser) mergeParseResult(result *FileParseResult) {
	// Merge structs
	for name, structInfo := range result.Structs {
		p.structs[name] = structInfo
	}

	// Merge handlers
	for name, handlerInfo := range result.Handlers {
		p.handlers[name] = handlerInfo
	}

	// Process route registrations to improve handler mapping
	for _, routeReg := range result.RouteRegistrations {
		p.analyzeRouteRegistration(routeReg)
	}

	// Merge imports
	for alias, pkg := range result.Imports {
		p.imports[alias] = pkg
	}
}

// extractImports extracts import information from the AST
func (p *ASTParser) extractImports(file *ast.File, result *FileParseResult) {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		alias := ""

		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			// Use the last part of the path as default alias
			parts := strings.Split(path, "/")
			if len(parts) > 0 {
				alias = parts[len(parts)-1]
			}
		}

		if alias != "" {
			result.Imports[alias] = path
		}
	}
}

// extractStructs extracts struct definitions from the AST
func (p *ASTParser) extractStructs(node *ast.File) {
	structCount := 0

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				for _, spec := range x.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							structInfo := p.parseStruct(typeSpec, structType, x.Doc)
							structInfo.Package = node.Name.Name
							p.structs[structInfo.Name] = structInfo
							structCount++
						}
					}
				}
			}
		}
		return true
	})
}

// parseStruct parses a struct declaration and returns StructInfo
func (p *ASTParser) parseStruct(typeSpec *ast.TypeSpec, structType *ast.StructType, doc *ast.CommentGroup) *StructInfo {
	structInfo := &StructInfo{
		Name:     typeSpec.Name.Name,
		Package:  "", // Will be set by caller
		Fields:   make(map[string]*FieldInfo),
		Tags:     []string{},
		Embedded: []string{},
		Methods:  []string{},
	}

	// Parse documentation
	if doc != nil {
		structInfo.Documentation = p.parseDocumentation(doc)
		structInfo.Description = structInfo.Documentation.Summary
		structInfo.Tags = structInfo.Documentation.Tags
	}

	// Parse fields
	if structType.Fields != nil {
		for _, field := range structType.Fields.List {
			fieldInfo := p.parseField(field)
			if fieldInfo != nil {
				if len(field.Names) > 0 {
					structInfo.Fields[field.Names[0].Name] = fieldInfo
				} else {
					// Embedded field
					typeName := p.extractTypeName(field.Type)
					structInfo.Embedded = append(structInfo.Embedded, typeName)
				}
			}
		}
	}

	// Generate JSON schema
	structInfo.JSONSchema = p.generateStructSchema(structInfo)

	return structInfo
}

// parseField parses a struct field and returns FieldInfo
func (p *ASTParser) parseField(field *ast.Field) *FieldInfo {
	if len(field.Names) == 0 {
		return nil // Skip embedded fields for now
	}

	fieldName := field.Names[0].Name
	typeName := p.extractTypeName(field.Type)

	fieldInfo := &FieldInfo{
		Name:       fieldName,
		Type:       typeName,
		IsPointer:  p.isPointerType(field.Type),
		IsExported: fieldName[0] >= 'A' && fieldName[0] <= 'Z',
		Validation: make(map[string]string),
	}

	// Parse struct tags
	if field.Tag != nil {
		p.parseStructTags(field.Tag.Value, fieldInfo)
	}

	// Parse field documentation
	if field.Doc != nil {
		doc := p.parseDocumentation(field.Doc)
		fieldInfo.Description = doc.Summary
	}

	// Generate field schema
	fieldInfo.Schema = p.generateFieldSchema(fieldInfo)

	return fieldInfo
}

// parseStructTags parses struct tags and extracts JSON and validation info
func (p *ASTParser) parseStructTags(tagValue string, fieldInfo *FieldInfo) {
	// Remove backticks
	tagValue = strings.Trim(tagValue, "`")

	// Parse JSON tag
	if jsonTag := p.extractTag(tagValue, "json"); jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if len(parts) > 0 && parts[0] != "" {
			fieldInfo.JSONName = parts[0]
		}

		// Check for omitempty
		for _, part := range parts[1:] {
			if strings.TrimSpace(part) == "omitempty" {
				fieldInfo.JSONOmitEmpty = true
				break
			}
		}
	}

	// Parse validation tags
	if validateTag := p.extractTag(tagValue, "validate"); validateTag != "" {
		p.parseValidationTag(validateTag, fieldInfo)
	}

	// Parse form tag
	if formTag := p.extractTag(tagValue, "form"); formTag != "" {
		fieldInfo.Validation["form"] = formTag
	}

	// Parse db tag
	if dbTag := p.extractTag(tagValue, "db"); dbTag != "" {
		fieldInfo.Validation["db"] = dbTag
	}
}

// extractTag extracts a specific tag value from struct tags
func (p *ASTParser) extractTag(tags, tagName string) string {
	re := regexp.MustCompile(tagName + `:"([^"]*)"`)
	matches := re.FindStringSubmatch(tags)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// parseValidationTag parses validation tags and extracts rules
func (p *ASTParser) parseValidationTag(validateTag string, fieldInfo *FieldInfo) {
	rules := strings.Split(validateTag, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "required" {
			fieldInfo.Required = true
			fieldInfo.Validation["required"] = "true"
		} else if strings.HasPrefix(rule, "min=") {
			fieldInfo.Validation["min"] = strings.TrimPrefix(rule, "min=")
		} else if strings.HasPrefix(rule, "max=") {
			fieldInfo.Validation["max"] = strings.TrimPrefix(rule, "max=")
		} else if strings.HasPrefix(rule, "len=") {
			fieldInfo.Validation["len"] = strings.TrimPrefix(rule, "len=")
		} else {
			fieldInfo.Validation[rule] = "true"
		}
	}
}

// extractHandlers extracts handler function information from the AST
func (p *ASTParser) extractHandlers(file *ast.File, result *FileParseResult) {
	ast.Inspect(file, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if p.isHandlerFunction(funcDecl) {
				handlerInfo := p.parseHandler(funcDecl)
				if handlerInfo != nil {
					result.Handlers[handlerInfo.Name] = handlerInfo
				}
			}
		}
		// Also look for route registrations
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if routeReg := p.parseRouteRegistration(callExpr); routeReg != nil {
				result.RouteRegistrations = append(result.RouteRegistrations, routeReg)
			}
		}
		return true
	})
}

// isHandlerFunction checks if a function declaration is a handler function
func (p *ASTParser) isHandlerFunction(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type == nil || len(funcDecl.Type.Params.List) != 1 {
		return false
	}

	param := funcDecl.Type.Params.List[0]
	paramType := p.extractTypeName(param.Type)

	// Check for core.RequestEvent parameter
	isHandler := strings.Contains(paramType, "RequestEvent") ||
		strings.Contains(paramType, "core.RequestEvent")

	return isHandler
}

// parseHandler parses a handler function and returns ASTHandlerInfo
func (p *ASTParser) parseHandler(funcDecl *ast.FuncDecl) *ASTHandlerInfo {
	handlerInfo := &ASTHandlerInfo{
		Name:        funcDecl.Name.Name,
		Parameters:  []*ParamInfo{},
		HTTPMethods: []string{},
		Middleware:  []string{},
	}

	// Parse documentation and directives
	if funcDecl.Doc != nil {
		handlerInfo.Documentation = p.parseDocumentation(funcDecl.Doc)
		p.parseAPIDirectives(funcDecl.Doc, handlerInfo)
	}

	// Analyze function body for patterns
	if funcDecl.Body != nil {
		p.analyzeHandlerBody(funcDecl.Body, handlerInfo)
	}

	return handlerInfo
}

// parseAPIDirectives parses API directive comments (API_DESC, API_TAGS, etc.)
func (p *ASTParser) parseAPIDirectives(commentGroup *ast.CommentGroup, handlerInfo *ASTHandlerInfo) {
	for _, comment := range commentGroup.List {
		text := strings.TrimSpace(comment.Text)

		// Remove comment prefixes
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)

		if strings.HasPrefix(text, "API_DESC ") {
			handlerInfo.APIDescription = strings.TrimSpace(strings.TrimPrefix(text, "API_DESC"))
		} else if strings.HasPrefix(text, "API_TAGS ") {
			tagsStr := strings.TrimSpace(strings.TrimPrefix(text, "API_TAGS"))
			tags := strings.Split(tagsStr, ",")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					handlerInfo.APITags = append(handlerInfo.APITags, tag)
				}
			}
		}
	}
}

// analyzeHandlerBody analyzes the handler function body for patterns
func (p *ASTParser) analyzeHandlerBody(body *ast.BlockStmt, handlerInfo *ASTHandlerInfo) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			p.processCallExpr(node, handlerInfo)
		case *ast.AssignStmt:
			p.processAssignment(node, handlerInfo)
		case *ast.GenDecl:
			p.processVarDeclaration(node, handlerInfo)
		}
		return true
	})
}

// processCallExpr processes function calls to detect patterns
func (p *ASTParser) processCallExpr(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) {
	// Check for JSON decode calls
	if p.isJSONDecodeCall(call) {
		handlerInfo.UsesJSONDecode = true
		// Only set RequestType if not already detected from variable declaration
		if handlerInfo.RequestType == "" {
			handlerInfo.RequestType = p.extractRequestTypeWithContext(call, handlerInfo)
		}
	}

	// Check for JSON response calls
	if p.isJSONResponseCall(call) {
		handlerInfo.UsesJSONReturn = true
		handlerInfo.ResponseType = p.extractResponseTypeWithContext(call, handlerInfo)

		// If it's a map literal, also store the schema directly
		if handlerInfo.ResponseType == "map[string]any" && len(call.Args) >= 2 {
			if compLit, ok := call.Args[1].(*ast.CompositeLit); ok {
				handlerInfo.ResponseSchema = p.analyzeMapLiteralSchema(compLit)
			}
		}
	}
}

// processAssignment processes assignments to detect variable declarations
func (p *ASTParser) processAssignment(assign *ast.AssignStmt, handlerInfo *ASTHandlerInfo) {
	// Initialize Variables map if not already done
	if handlerInfo.Variables == nil {
		handlerInfo.Variables = make(map[string]string)
	}

	// Process both assignments and short variable declarations
	for i, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok && i < len(assign.Rhs) {
			varName := ident.Name

			// Extract type from the right-hand side expression
			varType := p.extractTypeFromExpressionWithContext(assign.Rhs[i], handlerInfo)

			// Handle basic literal types when no other type is detected
			if varType == "" {
				if basicLit, ok := assign.Rhs[i].(*ast.BasicLit); ok {
					switch basicLit.Kind.String() {
					case "STRING":
						varType = "string"
					case "INT":
						varType = "int"
					case "FLOAT":
						varType = "float64"
					case "CHAR":
						varType = "rune"
					}
				}
			}
			if varType != "" {
				// Track the variable in our map
				handlerInfo.Variables[varName] = varType
			}

			// Only exact variable name matches
			if varName == "request" || varName == "req" {
				if handlerInfo.RequestType == "" {
					handlerInfo.RequestType = varType
				}
			}
			if varName == "response" || varName == "resp" || varName == "result" {
				if handlerInfo.ResponseType == "" {
					handlerInfo.ResponseType = varType
				}
			}
		}
	}
}

// processVarDeclaration processes variable declarations to detect request/response types
func (p *ASTParser) processVarDeclaration(decl *ast.GenDecl, handlerInfo *ASTHandlerInfo) {
	if decl.Tok != token.VAR {
		return
	}

	// Initialize Variables map if not already done
	if handlerInfo.Variables == nil {
		handlerInfo.Variables = make(map[string]string)
	}

	for _, spec := range decl.Specs {
		if valueSpec, ok := spec.(*ast.ValueSpec); ok {
			for _, name := range valueSpec.Names {
				varName := name.Name

				if valueSpec.Type != nil {
					typeName := p.extractTypeName(valueSpec.Type)

					// Track the variable in our map
					handlerInfo.Variables[varName] = typeName

					// Check if this looks like a request variable
					if varName == "req" || varName == "request" {
						if handlerInfo.RequestType == "" {
							handlerInfo.RequestType = typeName
						}
					}
				}
			}
		}
	}
}

// isJSONDecodeCall checks if a call expression is a JSON decode operation
func (p *ASTParser) isJSONDecodeCall(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Name == "Decode" || sel.Sel.Name == "Unmarshal"
	}
	return false
}

// isJSONResponseCall checks if a call expression is a JSON response operation
func (p *ASTParser) isJSONResponseCall(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Name == "JSON" || sel.Sel.Name == "WriteJSON"
	}
	return false
}

// extractRequestType extracts the request type from a JSON decode call
func (p *ASTParser) extractRequestType(call *ast.CallExpr) string {
	return p.extractRequestTypeWithContext(call, nil)
}

func (p *ASTParser) extractRequestTypeWithContext(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) string {
	// This is a simplified implementation
	// In practice, you'd need more sophisticated type analysis
	for _, arg := range call.Args {
		if typeName := p.extractTypeFromExpressionWithContext(arg, handlerInfo); typeName != "" {
			return typeName
		}
	}
	return ""
}

// extractResponseType extracts the response type from a JSON response call
func (p *ASTParser) extractResponseType(call *ast.CallExpr) string {
	return p.extractResponseTypeWithContext(call, nil)
}

func (p *ASTParser) extractResponseTypeWithContext(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) string {
	// Look at the arguments to find the response data
	if len(call.Args) >= 2 {
		responseType := p.extractTypeFromExpressionWithContext(call.Args[1], handlerInfo)
		return responseType
	}
	return ""
}

// analyzeMapLiteralSchema analyzes a composite literal (map[string]any{}) and generates JSON schema
func (p *ASTParser) analyzeMapLiteralSchema(compLit *ast.CompositeLit) map[string]interface{} {
	if compLit == nil || len(compLit.Elts) == 0 {
		return nil
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	properties := schema["properties"].(map[string]interface{})

	for _, elt := range compLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			// Extract key name
			var keyName string
			if key, ok := kv.Key.(*ast.BasicLit); ok && key.Kind == token.STRING {
				keyName = strings.Trim(key.Value, `"`)
			} else if key, ok := kv.Key.(*ast.Ident); ok {
				keyName = key.Name
			}

			if keyName != "" {
				// Analyze value type with field name context
				valueSchema := p.analyzeValueForSchemaWithContext(kv.Value, keyName)
				if valueSchema != nil {
					properties[keyName] = valueSchema
				}
			}
		}
	}

	return schema
}

// analyzeValueForSchema analyzes a value expression and returns appropriate JSON schema
func (p *ASTParser) analyzeValueForSchema(expr ast.Expr) map[string]interface{} {
	return p.analyzeValueForSchemaWithContext(expr, "")
}

func (p *ASTParser) analyzeValueForSchemaWithContext(expr ast.Expr, fieldName string) map[string]interface{} {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.STRING:
			return map[string]interface{}{
				"type":    "string",
				"example": strings.Trim(e.Value, `"`),
			}
		case token.INT:
			if intVal, err := strconv.Atoi(e.Value); err == nil {
				return map[string]interface{}{
					"type":    "integer",
					"example": intVal,
				}
			}
			return map[string]interface{}{
				"type": "integer",
			}
		case token.FLOAT:
			if floatVal, err := strconv.ParseFloat(e.Value, 64); err == nil {
				return map[string]interface{}{
					"type":    "number",
					"example": floatVal,
				}
			}
			return map[string]interface{}{
				"type": "number",
			}
		}
	case *ast.CompositeLit:
		// Handle nested maps or arrays
		if p.isMapType(e.Type) {
			return p.analyzeMapLiteralSchema(e)
		}
		// For arrays or other composite types
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": true,
		}
	case *ast.CallExpr:
		// Handle function calls like time.Now().Format()
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			methodName := sel.Sel.Name

			// Use comprehensive PocketBase record getter method mapping
			getterMappings := GetRecordGetterMethodMapping()
			if schema, exists := getterMappings[methodName]; exists {
				return schema
			}

			// Handle time-related functions only if explicitly identifiable
			if methodName == "Format" {
				return map[string]interface{}{
					"type":   "string",
					"format": "date-time",
				}
			}
		}

		// Handle len() function calls
		if sel, ok := e.Fun.(*ast.Ident); ok && sel.Name == "len" {
			return map[string]interface{}{
				"type": "integer",
			}
		}

		// No fallback for unknown function calls
		return nil
	case *ast.Ident:
		// Handle variable references - try to infer from common patterns

		// Common variable patterns
		if strings.Contains(e.Name, "todos") || strings.Contains(e.Name, "items") || strings.Contains(e.Name, "records") {
			return map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
				},
			}
		}

		// ID patterns
		if strings.Contains(e.Name, "ID") || strings.Contains(e.Name, "Id") || e.Name == "id" {
			return map[string]interface{}{
				"type": "string",
			}
		}

		// Updates/data patterns
		if strings.Contains(e.Name, "updates") || strings.Contains(e.Name, "data") || strings.Contains(e.Name, "changes") {
			return map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true,
			}
		}

		return nil
	case *ast.SelectorExpr:
		// Handle property access like record.Id, record.Name, etc.

		// e.Sel is already *ast.Ident, no need to type assert
		sel := e.Sel
		// Common field patterns
		if sel.Name == "Id" || sel.Name == "ID" {
			return map[string]interface{}{
				"type": "string",
			}
		}
		if strings.Contains(strings.ToLower(sel.Name), "time") || strings.Contains(strings.ToLower(sel.Name), "date") {
			return map[string]interface{}{
				"type":   "string",
				"format": "date-time",
			}
		}

		// Default for selector expressions
		return map[string]interface{}{
			"type": "string",
		}
	}

	// No fallback - return nil for unknown expressions
	return nil
}

// isMapType checks if a type expression represents a map type
func (p *ASTParser) isMapType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if mapType, ok := expr.(*ast.MapType); ok {
		return mapType != nil
	}
	return false
}

// extractTypeFromExpression extracts type information from an expression
func (p *ASTParser) extractTypeFromExpression(expr ast.Expr) string {
	return p.extractTypeFromExpressionWithContext(expr, nil)
}

func (p *ASTParser) extractTypeFromExpressionWithContext(expr ast.Expr, handlerInfo *ASTHandlerInfo) string {
	switch e := expr.(type) {
	case *ast.Ident:
		// If we have handler context and the identifier is a tracked variable, return its type
		if handlerInfo != nil && handlerInfo.Variables != nil {
			if varType, exists := handlerInfo.Variables[e.Name]; exists {
				return varType
			}
		}
		return e.Name
	case *ast.SelectorExpr:
		return p.extractTypeName(e)
	case *ast.CompositeLit:
		// Handle composite literals (struct/map literals)
		if p.isMapType(e.Type) {
			return "map[string]any" // Special marker for map literals
		}
		typeName := p.extractTypeName(e.Type)

		// Handle &StructType{} patterns - the type is still StructType
		if typeName != "" {
			return typeName
		}

		// If no explicit type, try to infer from the context
		if e.Type == nil {
			return ""
		}

		return typeName
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			// For &variable, recursively get the variable's type
			underlyingType := p.extractTypeFromExpressionWithContext(e.X, handlerInfo)
			return underlyingType
		}
	case *ast.CallExpr:
		// Handle function calls - try to infer return type
		// Handle method calls like SomeType{} or pkg.NewType()
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			// Handle method calls like SomeType{} or pkg.NewType()
			if ident, ok := sel.X.(*ast.Ident); ok {
				methodName := sel.Sel.Name
				receiverName := ident.Name

				// Check if this is a known variable with a tracked type
				if handlerInfo != nil && handlerInfo.Variables != nil {
					if varType, exists := handlerInfo.Variables[receiverName]; exists {
						// Handle methods based on variable type
						switch varType {
						case "*models.Record":
							switch methodName {
							case "GetString":
								return "string"
							case "GetInt":
								return "int"
							case "GetFloat":
								return "float64"
							case "GetBool":
								return "bool"
							case "GetDateTime":
								return "time.Time"
							case "Get":
								return "interface{}"
							case "Save", "Delete", "Validate":
								return "error"
							}
						case "*models.Collection":
							switch methodName {
							case "FindRecordById", "FindFirstRecordByFilter":
								return "*models.Record"
							case "FindRecordsByFilter":
								return "[]*models.Record"
							case "Save", "Delete":
								return "error"
							}
						}
					}
				}

				// Handle record getter methods by receiver name (fallback)
				if receiverName == "record" {
					switch methodName {
					case "GetString":
						return "string"
					case "GetInt":
						return "int"
					case "GetFloat":
						return "float64"
					case "GetBool":
						return "bool"
					case "GetDateTime":
						return "time.Time"
					case "Get":
						return "interface{}"
					case "Save", "Delete", "Validate":
						return "error"
					}
				}

				// Handle app/collection methods
				if receiverName == "app" || receiverName == "e" {
					switch methodName {
					case "FindCollectionByNameOrId":
						return "*models.Collection"
					case "FindRecordById":
						return "*models.Record"
					case "PathParam":
						return "string"
					case "QueryParam":
						return "string"
					}
				}

				// Handle collection methods by receiver name (fallback)
				if receiverName == "collection" || receiverName == "coll" {
					switch methodName {
					case "FindRecordById", "FindFirstRecordByFilter":
						return "*models.Record"
					case "FindRecordsByFilter":
						return "[]*models.Record"
					case "Save", "Delete":
						return "error"
					}
				}

				// Handle constructor patterns like NewType(), CreateType(), etc.
				if strings.HasPrefix(methodName, "New") || strings.HasPrefix(methodName, "Create") {
					constructorType := strings.TrimPrefix(methodName, "New")
					constructorType = strings.TrimPrefix(constructorType, "Create")
					if constructorType != "" {
						return constructorType
					}
				}

				// Handle specific known functions
				switch receiverName + "." + methodName {
				case "strconv.Atoi":
					return "int"
				case "strconv.ParseInt":
					return "int64"
				case "strconv.ParseFloat":
					return "float64"
				case "strconv.ParseBool":
					return "bool"
				case "json.Marshal":
					return "[]byte"
				case "json.Unmarshal":
					return "error"
				}

				// For other method calls, we can't easily determine the return type
				return ""
			}
		}

		// Handle direct function calls like make(), new(), etc.
		if ident, ok := e.Fun.(*ast.Ident); ok {
			switch ident.Name {
			case "make":
				// make(Type, ...) - first argument is the type
				if len(e.Args) > 0 {
					typeResult := p.extractTypeFromExpressionWithContext(e.Args[0], handlerInfo)
					if typeResult == "" {
						// Try extractTypeName directly if context-aware version fails
						typeResult = p.extractTypeName(e.Args[0])
					}
					return typeResult
				}
			case "new":
				// new(Type) - returns *Type, but we want Type
				if len(e.Args) > 0 {
					typeResult := p.extractTypeFromExpressionWithContext(e.Args[0], handlerInfo)
					return typeResult
				}
			case "len":
				// len() always returns int
				return "int"
			case "cap":
				// cap() always returns int
				return "int"
			case "append":
				// append returns same type as first argument (slice)
				if len(e.Args) > 0 {
					return p.extractTypeFromExpressionWithContext(e.Args[0], handlerInfo)
				}
			case "string":
				// string() conversion
				return "string"
			case "int":
				// int() conversion
				return "int"
			case "float64":
				// float64() conversion
				return "float64"
			case "bool":
				// bool() conversion
				return "bool"
			default:
				// Handle constructor patterns like NewType(), CreateType(), etc.
				if strings.HasPrefix(ident.Name, "New") {
					constructorType := strings.TrimPrefix(ident.Name, "New")
					if constructorType != "" {
						return constructorType
					}
				}
				if strings.HasPrefix(ident.Name, "Create") {
					constructorType := strings.TrimPrefix(ident.Name, "Create")
					if constructorType != "" {
						return constructorType
					}
				}
				// For other method calls, we can't easily determine the return type
				return ""
			}
		}

		// Try to detect return type from function name
		if ident, ok := e.Fun.(*ast.Ident); ok {
			if returnType := p.detectPocketBaseReturnType(ident.Name, e.Args); returnType != "" {
				return returnType
			}
		}

		// Try to detect return type from method calls
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			methodName := sel.Sel.Name
			if returnType := p.detectPocketBaseReturnType(methodName, e.Args); returnType != "" {
				return returnType
			}
		}
	case *ast.TypeAssertExpr:
		// Handle type assertions like value.(Type)
		assertedType := p.extractTypeName(e.Type)
		return assertedType
	}
	return ""
}

// detectPocketBaseReturnType attempts to detect return types for common PocketBase function calls
func (p *ASTParser) detectPocketBaseReturnType(funcName string, args []ast.Expr) string {
	// Handle common PocketBase API patterns
	switch funcName {
	case "FindCollectionByNameOrId":
		return "*models.Collection"
	case "FindRecordById", "FindFirstRecordByFilter":
		return "*models.Record"
	case "FindRecordsByFilter":
		return "[]*models.Record"
	case "PathParam", "QueryParam":
		return "string"
	case "GetString":
		return "string"
	case "GetInt":
		return "int"
	case "GetFloat":
		return "float64"
	case "GetBool":
		return "bool"
	case "GetDateTime":
		return "time.Time"
	case "Get":
		return "interface{}"
	case "Save", "Delete", "Validate":
		return "error"
	case "JSON", "String", "NoContent", "Redirect", "File":
		return "error"
	}

	// Handle function name patterns
	if strings.HasSuffix(funcName, "Handler") {
		return "error"
	}

	if strings.HasPrefix(funcName, "New") && len(funcName) > 3 {
		typeName := funcName[3:] // Remove "New" prefix
		if typeName != "" {
			return "*" + typeName
		}
	}

	if strings.HasPrefix(funcName, "Create") && len(funcName) > 6 {
		typeName := funcName[6:] // Remove "Create" prefix
		if typeName != "" {
			return "*" + typeName
		}
	}

	return ""
}

// extractTypeName extracts type names from type expressions
func (p *ASTParser) extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + p.extractTypeName(t.X)
	case *ast.SelectorExpr:
		x := p.extractTypeName(t.X)
		if x != "" {
			return x + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.ArrayType:
		return "[]" + p.extractTypeName(t.Elt)
	case *ast.MapType:
		key := p.extractTypeName(t.Key)
		value := p.extractTypeName(t.Value)
		return "map[" + key + "]" + value
	case *ast.InterfaceType:
		// Handle interface{} types
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface"
	default:
		return ""
	}
}

// isPointerType checks if a type expression represents a pointer type
func (p *ASTParser) isPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

// parseDocumentation parses documentation comments and returns Documentation
func (p *ASTParser) parseDocumentation(commentGroup *ast.CommentGroup) *Documentation {
	if commentGroup == nil {
		return &Documentation{}
	}

	doc := &Documentation{
		Parameters: make(map[string]string),
		Examples:   []string{},
		SeeAlso:    []string{},
		Authors:    []string{},
		Tags:       []string{},
	}

	var lines []string
	for _, comment := range commentGroup.List {
		text := strings.TrimSpace(comment.Text)
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)

		if text != "" {
			lines = append(lines, text)
		}
	}

	if len(lines) > 0 {
		doc.Summary = lines[0]
		if len(lines) > 1 {
			doc.Description = strings.Join(lines[1:], "\n")
		}
	}

	return doc
}

// generateStructSchema generates a JSON schema for a struct
func (p *ASTParser) generateStructSchema(structInfo *StructInfo) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	properties := schema["properties"].(map[string]interface{})
	var required []string

	for fieldName, fieldInfo := range structInfo.Fields {
		fieldSchema := p.generateFieldSchema(fieldInfo)
		if fieldSchema == nil {
			continue
		}

		jsonName := fieldInfo.JSONName
		if jsonName == "" {
			jsonName = fieldName
		}

		properties[jsonName] = fieldSchema

		if fieldInfo.Required {
			required = append(required, jsonName)
		}
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	if structInfo.Description != "" {
		schema["description"] = structInfo.Description
	}

	return schema
}

// generateFieldSchema generates a JSON schema for a struct field
func (p *ASTParser) generateFieldSchema(fieldInfo *FieldInfo) map[string]interface{} {
	// Try PocketBase field type mapping first
	pbMapping := NewPocketBaseFieldTypeMapping()

	// Convert validation map from map[string]string to map[string]interface{}
	validationConfig := make(map[string]interface{})
	for k, v := range fieldInfo.Validation {
		validationConfig[k] = v
	}

	if pbSchema := pbMapping.GetSchemaForField(fieldInfo.Type, validationConfig); pbSchema != nil {
		schema := pbSchema

		// Add description if available
		if fieldInfo.Description != "" {
			schema["description"] = fieldInfo.Description
		}

		// Add example if available
		if fieldInfo.Example != nil {
			schema["example"] = fieldInfo.Example
		}

		return schema
	}

	// Try Go type mapping - only for known types
	if goSchema := p.goTypeToJSONSchema(fieldInfo.Type); goSchema != nil {
		schema := goSchema

		if fieldInfo.Description != "" {
			schema["description"] = fieldInfo.Description
		}

		// Add validation constraints only for known constraint types
		for key, value := range fieldInfo.Validation {
			switch key {
			case "min":
				if minVal, err := strconv.Atoi(value); err == nil {
					schema["minimum"] = minVal
				}
			case "max":
				if maxVal, err := strconv.Atoi(value); err == nil {
					schema["maximum"] = maxVal
				}
			case "len":
				if lenVal, err := strconv.Atoi(value); err == nil {
					schema["minLength"] = lenVal
					schema["maxLength"] = lenVal
				}
			}
		}

		if fieldInfo.Example != nil {
			schema["example"] = fieldInfo.Example
		}

		return schema
	}

	// No fallback - return nil for unknown types
	return nil
}

// goTypeToJSONSchema converts Go types to JSON schema format
func (p *ASTParser) goTypeToJSONSchema(goType string) map[string]interface{} {
	// Remove pointer prefix
	goType = strings.TrimPrefix(goType, "*")

	switch goType {
	case "string":
		return map[string]interface{}{"type": "string"}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return map[string]interface{}{"type": "integer"}
	case "float32", "float64":
		return map[string]interface{}{"type": "number"}
	case "bool":
		return map[string]interface{}{"type": "boolean"}
	case "time.Time":
		return map[string]interface{}{
			"type":   "string",
			"format": "date-time",
		}
	default:
		if strings.HasPrefix(goType, "[]") {
			itemType := strings.TrimPrefix(goType, "[]")
			if itemSchema := p.goTypeToJSONSchema(itemType); itemSchema != nil {
				return map[string]interface{}{
					"type":  "array",
					"items": itemSchema,
				}
			}
			return nil
		}
		if strings.HasPrefix(goType, "map[") {
			return map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true,
			}
		}
		// For custom types, reference them only if they exist in parsed structs
		if _, exists := p.structs[goType]; exists {
			return map[string]interface{}{
				"$ref": "#/components/schemas/" + CleanTypeName(goType),
			}
		}
		// No fallback for unknown custom types
		return nil
	}
}

// createParseError creates a ParseError with context
func (p *ASTParser) createParseError(errorType, message, file string, line, column int, context string) ParseError {
	return ParseError{
		Type:    errorType,
		Message: message,
		File:    file,
		Line:    line,
		Column:  column,
		Context: context,
	}
}

// =============================================================================
// Interface Implementation Methods
// =============================================================================

// GetAllStructs returns all parsed struct information
func (p *ASTParser) GetAllStructs() map[string]*StructInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*StructInfo)
	for k, v := range p.structs {
		result[k] = v
	}
	return result
}

// GetAllHandlers returns all parsed handler information
func (p *ASTParser) GetAllHandlers() map[string]*ASTHandlerInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*ASTHandlerInfo)
	for k, v := range p.handlers {
		result[k] = v
	}
	return result
}

// GetStructByName retrieves a specific struct by name
func (p *ASTParser) GetStructByName(name string) (*StructInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	structInfo, exists := p.structs[name]
	return structInfo, exists
}

// GetHandlerByName retrieves a specific handler by name
func (p *ASTParser) GetHandlerByName(name string) (*ASTHandlerInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	handlerInfo, exists := p.handlers[name]
	return handlerInfo, exists
}

// GetParseErrors returns all parsing errors
func (p *ASTParser) GetParseErrors() []ParseError {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var errors []ParseError
	for _, result := range p.fileCache {
		errors = append(errors, result.Errors...)
	}
	return errors
}

// ClearCache clears all cached data
func (p *ASTParser) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.structs = make(map[string]*StructInfo)
	p.handlers = make(map[string]*ASTHandlerInfo)
	p.typeCache = make(map[string]*TypeInfo)
	p.fileCache = make(map[string]*FileParseResult)
	p.imports = make(map[string]string)
}

// EnhanceEndpoint enhances an API endpoint with AST-derived information
func (p *ASTParser) EnhanceEndpoint(endpoint *APIEndpoint) error {
	if endpoint == nil {
		return nil
	}

	handlerName := ExtractHandlerNameFromPath(endpoint.Handler)

	// Get available handler names
	availableHandlers := make([]string, 0, len(p.handlers))
	for name := range p.handlers {
		availableHandlers = append(availableHandlers, name)
	}

	var handlerInfo *ASTHandlerInfo
	var exists bool

	// Try exact match first
	if handlerInfo, exists = p.GetHandlerByName(handlerName); exists {
		// Found exact match
	} else {
		// Try matching by route registration
		if handlerInfo, exists = p.findHandlerByRoute(endpoint); exists {
			// Found route-based match
		} else {
			// Try fuzzy matching as last resort
			if handlerInfo, exists = p.findHandlerByFuzzyMatch(endpoint, availableHandlers); exists {
				// Found fuzzy match
			}
		}
	}

	if exists && handlerInfo != nil {
		// Apply AST-derived information
		if handlerInfo.APIDescription != "" {
			endpoint.Description = handlerInfo.APIDescription
		}

		if len(handlerInfo.APITags) > 0 {
			endpoint.Tags = handlerInfo.APITags
		}

		// Set request/response schemas if available
		// Handle request schema if available
		if handlerInfo.RequestType != "" {
			if structInfo, exists := p.GetStructByName(handlerInfo.RequestType); exists {
				endpoint.Request = structInfo.JSONSchema
			}
		}

		// Handle response schema
		if handlerInfo.ResponseType != "" {
			if handlerInfo.ResponseType == "map[string]any" {
				// Handle inline map literals - try to find the actual composite literal
				if handlerInfo.ResponseSchema != nil {
					endpoint.Response = handlerInfo.ResponseSchema
				}
			} else if structInfo, exists := p.GetStructByName(handlerInfo.ResponseType); exists {
				endpoint.Response = structInfo.JSONSchema
			}
		}
	}

	return nil
}

// findHandlerByRoute finds a handler by matching against discovered route registrations
func (p *ASTParser) findHandlerByRoute(endpoint *APIEndpoint) (*ASTHandlerInfo, bool) {
	// Look for handlers that have matching route information
	for _, handlerInfo := range p.handlers {
		if handlerInfo.RoutePath == endpoint.Path {
			// Check if method matches
			for _, method := range handlerInfo.HTTPMethods {
				if method == endpoint.Method {
					return handlerInfo, true
				}
			}
			// If path matches but no method info, still consider it a match
			if len(handlerInfo.HTTPMethods) == 0 {
				return handlerInfo, true
			}
		}
	}
	return nil, false
}

// findHandlerByFuzzyMatch tries to find a handler using various matching strategies
func (p *ASTParser) findHandlerByFuzzyMatch(endpoint *APIEndpoint, availableHandlers []string) (*ASTHandlerInfo, bool) {
	// No fuzzy matching - only exact handler name matches are allowed
	return nil, false
}

// handlerMatchesPathPattern checks if a handler name matches the endpoint path pattern
func (p *ASTParser) handlerMatchesPathPattern(handlerName string, pathSegments []string) bool {
	// No pattern matching - only exact matches allowed
	return false
}

// handlerMatchesMethod checks if a handler name suggests it handles a specific HTTP method
func (p *ASTParser) handlerMatchesMethod(handlerName string, method string) bool {
	// No method pattern matching - only exact matches allowed
	return false
}

// parseRouteRegistration detects explicit route registrations only
func (p *ASTParser) parseRouteRegistration(call *ast.CallExpr) *RouteRegistration {
	if call.Fun == nil || len(call.Args) != 2 {
		return nil
	}

	// Only handle exact method calls - no pattern matching
	var method, path, handlerRef string

	// Extract method from function call - must be exact HTTP method name
	if selExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		method = strings.ToUpper(selExpr.Sel.Name)
		if !isHTTPMethod(method) {
			return nil
		}
	} else {
		return nil
	}

	// Extract path from first argument - must be string literal
	if basicLit, ok := call.Args[0].(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
		path = strings.Trim(basicLit.Value, `"`)
	} else {
		return nil
	}

	// Extract handler reference from second argument
	handlerRef = p.extractHandlerReference(call.Args[1])

	if method != "" && path != "" && handlerRef != "" {
		return &RouteRegistration{
			Method:     method,
			Path:       path,
			HandlerRef: handlerRef,
		}
	}

	return nil
}

// extractHandlerReference extracts the handler reference from an AST expression
func (p *ASTParser) extractHandlerReference(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name + "." + e.Sel.Name
		}
	case *ast.CallExpr:
		// Handle wrapped handlers like middleware(handler)
		if len(e.Args) > 0 {
			return p.extractHandlerReference(e.Args[0])
		}
	}
	return ""
}

// analyzeRouteRegistration processes a route registration - only exact matches
func (p *ASTParser) analyzeRouteRegistration(routeReg *RouteRegistration) {
	// Only use exact handler name matches - no fuzzy matching
	if handlerInfo, exists := p.handlers[routeReg.HandlerRef]; exists {
		handlerInfo.HTTPMethods = append(handlerInfo.HTTPMethods, routeReg.Method)
		handlerInfo.RoutePath = routeReg.Path
	}
}

// isHTTPMethod checks if a string is a valid HTTP method
func isHTTPMethod(method string) bool {
	httpMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, m := range httpMethods {
		if method == m {
			return true
		}
	}
	return false
}

// GetHandlerDescription returns the description for a handler
func (p *ASTParser) GetHandlerDescription(handlerName string) string {
	if handlerInfo, exists := p.GetHandlerByName(handlerName); exists {
		if handlerInfo.APIDescription != "" {
			return handlerInfo.APIDescription
		}
		if handlerInfo.Documentation != nil {
			return handlerInfo.Documentation.Summary
		}
	}
	return ""
}

// GetHandlerTags returns the tags for a handler
func (p *ASTParser) GetHandlerTags(handlerName string) []string {
	if handlerInfo, exists := p.GetHandlerByName(handlerName); exists {
		return handlerInfo.APITags
	}
	return []string{}
}

// GetStructsForFinding returns all structs for searching/finding operations
func (p *ASTParser) GetStructsForFinding() map[string]*StructInfo {
	return p.GetAllStructs()
}

// =============================================================================
// Discovery and File Operations
// =============================================================================

// DiscoverSourceFiles discovers Go source files with API_SOURCE directive
func (p *ASTParser) DiscoverSourceFiles() error {
	return filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip non-Go files, test files, vendor, and hidden directories
		if !strings.HasSuffix(path, ".go") ||
			strings.Contains(path, "_test.go") ||
			strings.Contains(path, "/vendor/") ||
			strings.Contains(path, "/.") ||
			strings.Contains(path, "node_modules") {
			return nil
		}

		// Check if file contains API_SOURCE directive
		if p.fileContainsAPISourceDirective(path) {
			return p.ParseFile(path)
		}

		return nil
	})
}

// fileContainsAPISourceDirective checks if a file contains the API_SOURCE directive
func (p *ASTParser) fileContainsAPISourceDirective(filepath string) bool {
	file, err := os.Open(filepath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read only the first part of the file for performance
	buf := make([]byte, 2048)
	n, _ := file.Read(buf)
	content := string(buf[:n])

	// Look for API_SOURCE directive
	return strings.Contains(content, "API_SOURCE") ||
		strings.Contains(content, "api_source") ||
		regexp.MustCompile(`//\s*API_SOURCE`).MatchString(content) ||
		regexp.MustCompile(`/\*\s*API_SOURCE`).MatchString(content)
}
