package api

import (
	"fmt"
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
	fmt.Printf("🔍 Starting AST parser initialization and file discovery\n")
	if err := parser.DiscoverSourceFiles(); err != nil {
		fmt.Printf("❌ Failed to discover source files: %v\n", err)
	} else {
		fmt.Printf("✅ AST file discovery completed - handlers: %d, structs: %d\n", len(parser.handlers), len(parser.structs))
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
		ModTime:  p.getFileModTime(filename),
		Structs:  make(map[string]*StructInfo),
		Handlers: make(map[string]*ASTHandlerInfo),
		Imports:  make(map[string]string),
		Errors:   []ParseError{},
		ParsedAt: time.Now(),
	}

	// Extract information from the AST
	p.extractImports(file, result)
	p.extractStructs(file, result)
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

// extractStructs extracts struct information from the AST
func (p *ASTParser) extractStructs(file *ast.File, result *FileParseResult) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			if node.Tok == token.TYPE {
				for _, spec := range node.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							structInfo := p.parseStruct(typeSpec, structType, node.Doc)
							if structInfo != nil {
								result.Structs[structInfo.Name] = structInfo
							}
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
		return true
	})
}

// isHandlerFunction checks if a function is an HTTP handler
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

	if isHandler {
		fmt.Printf("🎯 Found handler function: %s with param type: %s\n", funcDecl.Name.Name, paramType)
	}

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
	fmt.Printf("🔍 Parsing API directives for handler: %s\n", handlerInfo.Name)
	for _, comment := range commentGroup.List {
		text := strings.TrimSpace(comment.Text)

		// Remove comment prefixes
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)

		if strings.HasPrefix(text, "API_DESC ") {
			handlerInfo.APIDescription = strings.TrimSpace(strings.TrimPrefix(text, "API_DESC"))
			fmt.Printf("📝 Found API_DESC for %s: %s\n", handlerInfo.Name, handlerInfo.APIDescription)
		} else if strings.HasPrefix(text, "API_TAGS ") {
			tagsStr := strings.TrimSpace(strings.TrimPrefix(text, "API_TAGS"))
			tags := strings.Split(tagsStr, ",")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					handlerInfo.APITags = append(handlerInfo.APITags, tag)
				}
			}
			fmt.Printf("🏷️ Found API_TAGS for %s: %v\n", handlerInfo.Name, handlerInfo.APITags)
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
		}
		return true
	})
}

// processCallExpr processes function calls to detect patterns
func (p *ASTParser) processCallExpr(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) {
	// Check for JSON decode calls
	if p.isJSONDecodeCall(call) {
		handlerInfo.UsesJSONDecode = true
		handlerInfo.RequestType = p.extractRequestType(call)
	}

	// Check for JSON response calls
	if p.isJSONResponseCall(call) {
		fmt.Printf("🎯 Handler %s: Found JSON response call\n", handlerInfo.Name)
		handlerInfo.UsesJSONReturn = true
		handlerInfo.ResponseType = p.extractResponseType(call)
		fmt.Printf("🔍 Found JSON response call, response type: '%s'\n", handlerInfo.ResponseType)

		// If it's a map literal, also store the schema directly
		if handlerInfo.ResponseType == "map[string]any" && len(call.Args) >= 2 {
			if compLit, ok := call.Args[1].(*ast.CompositeLit); ok {
				fmt.Printf("📊 Analyzing map literal with %d elements\n", len(compLit.Elts))
				handlerInfo.ResponseSchema = p.analyzeMapLiteralSchema(compLit)
				fmt.Printf("📈 Generated schema: %+v\n", handlerInfo.ResponseSchema)
			} else {
				fmt.Printf("❌ Second argument is not a composite literal\n")
			}
		}
	}
}

// processAssignment processes assignments to detect variable declarations
func (p *ASTParser) processAssignment(assign *ast.AssignStmt, handlerInfo *ASTHandlerInfo) {
	// Look for variable declarations that might indicate request/response types
	for i, lhs := range assign.Lhs {
		if ident, ok := lhs.(*ast.Ident); ok && i < len(assign.Rhs) {
			varName := ident.Name

			// Check if this looks like a request variable
			if strings.Contains(strings.ToLower(varName), "req") ||
				strings.Contains(strings.ToLower(varName), "request") {
				if handlerInfo.RequestType == "" {
					handlerInfo.RequestType = p.extractTypeFromExpression(assign.Rhs[i])
				}
			}

			// Check if this looks like a response variable
			if strings.Contains(strings.ToLower(varName), "resp") ||
				strings.Contains(strings.ToLower(varName), "response") ||
				strings.Contains(strings.ToLower(varName), "result") {
				if handlerInfo.ResponseType == "" {
					handlerInfo.ResponseType = p.extractTypeFromExpression(assign.Rhs[i])
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
	// This is a simplified implementation
	// In practice, you'd need more sophisticated type analysis
	for _, arg := range call.Args {
		if typeName := p.extractTypeFromExpression(arg); typeName != "" {
			return typeName
		}
	}
	return ""
}

// extractResponseType extracts the response type from a JSON response call
func (p *ASTParser) extractResponseType(call *ast.CallExpr) string {
	fmt.Printf("🔍 extractResponseType: Processing call with %d arguments\n", len(call.Args))

	// Look at the arguments to find the response data
	if len(call.Args) >= 2 {
		responseType := p.extractTypeFromExpression(call.Args[1])
		fmt.Printf("🔍 extractResponseType: Extracted type '%s'\n", responseType)
		return responseType
	}
	fmt.Printf("⚠️ extractResponseType: Not enough arguments (%d < 2)\n", len(call.Args))
	return ""
}

// analyzeMapLiteralSchema analyzes a composite literal (map[string]any{}) and generates JSON schema
func (p *ASTParser) analyzeMapLiteralSchema(compLit *ast.CompositeLit) map[string]interface{} {
	if compLit == nil || len(compLit.Elts) == 0 {
		return nil
	}

	fmt.Printf("🔍 analyzeMapLiteralSchema: Processing %d elements\n", len(compLit.Elts))

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
				// Analyze value type
				valueSchema := p.analyzeValueForSchema(kv.Value)
				if valueSchema != nil {
					properties[keyName] = valueSchema
					fmt.Printf("📝 Added property '%s': %+v\n", keyName, valueSchema)
				}
			}
		}
	}

	fmt.Printf("📊 Final schema has %d properties\n", len(properties))
	return schema
}

// analyzeValueForSchema analyzes a value expression and returns appropriate JSON schema
func (p *ASTParser) analyzeValueForSchema(expr ast.Expr) map[string]interface{} {
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
			if sel.Sel.Name == "Format" || strings.Contains(sel.Sel.Name, "Time") {
				return map[string]interface{}{
					"type":   "string",
					"format": "date-time",
				}
			}
		}
		// Default for function calls
		return map[string]interface{}{
			"type": "string",
		}
	case *ast.Ident:
		// Handle identifiers (variables)
		return map[string]interface{}{
			"type": "string",
		}
	}

	// Default fallback
	return map[string]interface{}{
		"type": "string",
	}
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
	fmt.Printf("🔍 extractTypeFromExpression: Processing expression type %T\n", expr)

	switch e := expr.(type) {
	case *ast.Ident:
		fmt.Printf("🔍 extractTypeFromExpression: Ident '%s'\n", e.Name)
		return e.Name
	case *ast.SelectorExpr:
		result := p.extractTypeName(e)
		fmt.Printf("🔍 extractTypeFromExpression: SelectorExpr -> '%s'\n", result)
		return result
	case *ast.CompositeLit:
		// For composite literals, check if it's a map type
		if p.isMapType(e.Type) {
			fmt.Printf("🔍 extractTypeFromExpression: CompositeLit detected as map type\n")
			return "map[string]any" // Special marker for map literals
		}
		result := p.extractTypeName(e.Type)
		fmt.Printf("🔍 extractTypeFromExpression: CompositeLit -> '%s'\n", result)
		return result
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			fmt.Printf("🔍 extractTypeFromExpression: UnaryExpr with & operator\n")
			return p.extractTypeFromExpression(e.X)
		}
		fmt.Printf("🔍 extractTypeFromExpression: UnaryExpr with operator %s\n", e.Op.String())
	default:
		fmt.Printf("🔍 extractTypeFromExpression: Unhandled expression type %T\n", expr)
	}
	fmt.Printf("🔍 extractTypeFromExpression: Returning empty string\n")
	return ""
}

// extractTypeName extracts a type name from an AST type expression
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
	schema := p.goTypeToJSONSchema(fieldInfo.Type)

	if fieldInfo.Description != "" {
		schema["description"] = fieldInfo.Description
	}

	// Add validation constraints
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
			return map[string]interface{}{
				"type":  "array",
				"items": p.goTypeToJSONSchema(itemType),
			}
		}
		if strings.HasPrefix(goType, "map[") {
			return map[string]interface{}{
				"type":                 "object",
				"additionalProperties": true,
			}
		}
		// For custom types, reference them
		return map[string]interface{}{
			"$ref": "#/components/schemas/" + CleanTypeName(goType),
		}
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
	fmt.Printf("🔍 EnhanceEndpoint: Looking for handler '%s' (original: '%s') for endpoint %s %s\n", handlerName, endpoint.Handler, endpoint.Method, endpoint.Path)

	// Get available handler names for debugging
	availableHandlers := make([]string, 0, len(p.handlers))
	for name := range p.handlers {
		availableHandlers = append(availableHandlers, name)
	}

	if handlerInfo, exists := p.GetHandlerByName(handlerName); exists {
		fmt.Printf("✅ Found handler info for '%s': desc='%s', tags=%v\n", handlerName, handlerInfo.APIDescription, handlerInfo.APITags)
		// Apply AST-derived information
		if handlerInfo.APIDescription != "" {
			endpoint.Description = handlerInfo.APIDescription
			fmt.Printf("📝 Applied description: '%s'\n", handlerInfo.APIDescription)
		}

		if len(handlerInfo.APITags) > 0 {
			endpoint.Tags = handlerInfo.APITags
			fmt.Printf("🏷️ Applied tags: %v\n", handlerInfo.APITags)
		}

		// Set request/response schemas if available
		if handlerInfo.RequestType != "" {
			if structInfo, exists := p.GetStructByName(handlerInfo.RequestType); exists {
				endpoint.Request = structInfo.JSONSchema
			}
		}

		if handlerInfo.ResponseType != "" {
			fmt.Printf("🔍 Handler '%s' has response type: '%s'\n", handlerName, handlerInfo.ResponseType)
			if handlerInfo.ResponseType == "map[string]any" {
				// Handle inline map literals - try to find the actual composite literal
				if handlerInfo.ResponseSchema != nil {
					endpoint.Response = handlerInfo.ResponseSchema
					fmt.Printf("✅ Applied inline map schema for '%s' with %d properties\n", handlerName, len(handlerInfo.ResponseSchema))
				} else {
					fmt.Printf("❌ Handler '%s' has map type but no response schema\n", handlerName)
				}
			} else if structInfo, exists := p.GetStructByName(handlerInfo.ResponseType); exists {
				endpoint.Response = structInfo.JSONSchema
				fmt.Printf("✅ Applied struct schema for '%s': %s\n", handlerName, handlerInfo.ResponseType)
			} else {
				fmt.Printf("❌ Handler '%s' response type '%s' not found in structs\n", handlerName, handlerInfo.ResponseType)
			}
		} else {
			fmt.Printf("⚠️ Handler '%s' has no response type\n", handlerName)
		}
	} else {
		fmt.Printf("❌ Handler '%s' not found in AST data. Available handlers: %v\n", handlerName, availableHandlers)
	}

	return nil
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
	fmt.Printf("🚶 Walking directory tree for API_SOURCE files\n")
	return filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip non-Go files, test files, vendor, and hidden directories
		if !strings.HasSuffix(path, ".go") ||
			strings.Contains(path, "_test.go") ||
			strings.Contains(path, "/vendor/") ||
			strings.Contains(path, "/.git/") ||
			strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		// Check if file contains API_SOURCE directive
		if p.fileContainsAPISourceDirective(path) {
			fmt.Printf("📁 Found API_SOURCE file, parsing: %s\n", path)
			return p.ParseFile(path)
		}

		return nil
	})
}

// fileContainsAPISourceDirective checks if a file contains the API_SOURCE directive
func (p *ASTParser) fileContainsAPISourceDirective(filepath string) bool {
	file, err := os.Open(filepath)
	if err != nil {
		fmt.Printf("⚠️ Cannot open file for API_SOURCE check: %s, error: %v\n", filepath, err)
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
