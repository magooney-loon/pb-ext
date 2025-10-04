package server

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ASTParser handles advanced Go AST parsing for API documentation
type ASTParser struct {
	fileSet   *token.FileSet
	packages  map[string]*ast.Package
	structs   map[string]*StructInfo
	handlers  map[string]*ASTHandlerInfo
	imports   map[string]string
	typeCache map[string]*TypeInfo
}

// StructInfo contains detailed information about a Go struct
type StructInfo struct {
	Name        string                 `json:"name"`
	Package     string                 `json:"package"`
	Fields      map[string]*FieldInfo  `json:"fields"`
	JSONSchema  map[string]interface{} `json:"json_schema"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
}

// FieldInfo contains information about struct fields
type FieldInfo struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	JSONName      string                 `json:"json_name"`
	JSONOmitEmpty bool                   `json:"json_omit_empty"`
	Required      bool                   `json:"required"`
	Validation    map[string]string      `json:"validation"`
	Description   string                 `json:"description"`
	Example       interface{}            `json:"example,omitempty"`
	Schema        map[string]interface{} `json:"schema"`
}

// ASTHandlerInfo contains information about handler functions
type ASTHandlerInfo struct {
	Name           string                `json:"name"`
	Package        string                `json:"package"`
	RequestType    string                `json:"request_type"`
	ResponseType   string                `json:"response_type"`
	Parameters     map[string]*ParamInfo `json:"parameters"`
	UsesJSONDecode bool                  `json:"uses_json_decode"`
	UsesJSONReturn bool                  `json:"uses_json_return"`
	APIDescription string                `json:"api_description,omitempty"` // From // API_DESC comment
	APITags        []string              `json:"api_tags,omitempty"`        // From // API_TAGS comment
}

// ParamInfo contains parameter information
type ParamInfo struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Source      string      `json:"source"` // "path", "query", "body"
	Required    bool        `json:"required"`
	Description string      `json:"description"`
	Example     interface{} `json:"example,omitempty"`
}

// TypeInfo contains type information for complex types
type TypeInfo struct {
	Name        string                 `json:"name"`
	Kind        string                 `json:"kind"` // "struct", "slice", "map", "basic"
	ElementType string                 `json:"element_type,omitempty"`
	KeyType     string                 `json:"key_type,omitempty"`
	ValueType   string                 `json:"value_type,omitempty"`
	Fields      map[string]*FieldInfo  `json:"fields,omitempty"`
	Schema      map[string]interface{} `json:"schema"`
}

// NewASTParser creates a new AST parser instance
func NewASTParser() *ASTParser {
	return &ASTParser{
		fileSet:   token.NewFileSet(),
		packages:  make(map[string]*ast.Package),
		structs:   make(map[string]*StructInfo),
		handlers:  make(map[string]*ASTHandlerInfo),
		imports:   make(map[string]string),
		typeCache: make(map[string]*TypeInfo),
	}
}

// ParseFile parses a Go source file and extracts struct and handler information
func (p *ASTParser) ParseFile(filename string) error {
	// Parse the file
	src, err := parser.ParseFile(p.fileSet, filename, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	// Extract package name
	packageName := src.Name.Name

	// Store package
	if p.packages[packageName] == nil {
		p.packages[packageName] = &ast.Package{
			Name:  packageName,
			Files: make(map[string]*ast.File),
		}
	}
	p.packages[packageName].Files[filename] = src

	// Extract imports
	p.extractImports(src)

	// Extract structs
	if err := p.extractStructs(src, packageName); err != nil {
		return fmt.Errorf("failed to extract structs: %w", err)
	}

	// Extract handlers
	if err := p.extractHandlers(src, packageName); err != nil {
		return fmt.Errorf("failed to extract handlers: %w", err)
	}

	return nil
}

// extractImports extracts import statements from the AST
func (p *ASTParser) extractImports(file *ast.File) {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		var name string
		if imp.Name != nil {
			name = imp.Name.Name
		} else {
			// Extract package name from path
			parts := strings.Split(path, "/")
			name = parts[len(parts)-1]
		}
		p.imports[name] = path
	}
}

// extractStructs extracts struct definitions from the AST
func (p *ASTParser) extractStructs(file *ast.File, packageName string) error {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.GenDecl:
			if node.Tok == token.TYPE {
				for _, spec := range node.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							structInfo := p.parseStruct(typeSpec.Name.Name, structType, packageName, node.Doc)
							p.structs[structInfo.Name] = structInfo
						}
					}
				}
			}
		}
		return true
	})
	return nil
}

// parseStruct parses a struct type and returns StructInfo
func (p *ASTParser) parseStruct(name string, structType *ast.StructType, packageName string, doc *ast.CommentGroup) *StructInfo {
	structInfo := &StructInfo{
		Name:    name,
		Package: packageName,
		Fields:  make(map[string]*FieldInfo),
		Tags:    []string{},
	}

	// Extract description from comments
	if doc != nil {
		structInfo.Description = p.extractComment(doc)
	}

	// Parse fields
	for _, field := range structType.Fields.List {
		fieldInfo := p.parseField(field)
		if fieldInfo != nil {
			// Handle multiple names (rare but possible)
			if len(field.Names) > 0 {
				for _, name := range field.Names {
					fieldCopy := *fieldInfo
					fieldCopy.Name = name.Name
					structInfo.Fields[name.Name] = &fieldCopy
				}
			} else {
				// Anonymous field
				structInfo.Fields[fieldInfo.Name] = fieldInfo
			}
		}
	}

	// Generate JSON schema
	structInfo.JSONSchema = p.generateStructSchema(structInfo)

	return structInfo
}

// parseField parses a struct field
func (p *ASTParser) parseField(field *ast.Field) *FieldInfo {
	if len(field.Names) == 0 {
		return nil // Skip anonymous fields for now
	}

	fieldInfo := &FieldInfo{
		Name:       field.Names[0].Name,
		Type:       p.typeToString(field.Type),
		Validation: make(map[string]string),
		Schema:     make(map[string]interface{}),
	}

	// Parse struct tags
	if field.Tag != nil {
		tagValue := strings.Trim(field.Tag.Value, "`")
		p.parseStructTags(fieldInfo, tagValue)
	}

	// Extract field comments
	if field.Doc != nil {
		fieldInfo.Description = p.extractComment(field.Doc)
	}

	// Generate field schema
	fieldInfo.Schema = p.generateFieldSchema(fieldInfo)

	return fieldInfo
}

// parseStructTags parses struct tags (json, validate, etc.)
func (p *ASTParser) parseStructTags(fieldInfo *FieldInfo, tagString string) {
	// Parse JSON tag
	if jsonTag := p.extractTag(tagString, "json"); jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if len(parts) > 0 && parts[0] != "-" {
			fieldInfo.JSONName = parts[0]
		}

		// Check for omitempty
		for _, part := range parts[1:] {
			if part == "omitempty" {
				fieldInfo.JSONOmitEmpty = true
			}
		}
	} else {
		// Use field name if no json tag
		fieldInfo.JSONName = strings.ToLower(fieldInfo.Name)
	}

	// Parse validation tag
	if validateTag := p.extractTag(tagString, "validate"); validateTag != "" {
		p.parseValidationTag(fieldInfo, validateTag)
	}
}

// extractTag extracts a specific tag from tag string
func (p *ASTParser) extractTag(tagString, tagName string) string {
	re := regexp.MustCompile(fmt.Sprintf(`%s:"([^"]*)"`, tagName))
	matches := re.FindStringSubmatch(tagString)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// parseValidationTag parses validation tags
func (p *ASTParser) parseValidationTag(fieldInfo *FieldInfo, validateTag string) {
	rules := strings.Split(validateTag, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "required" {
			fieldInfo.Required = true
			fieldInfo.Validation["required"] = "true"
		} else if strings.Contains(rule, "=") {
			parts := strings.SplitN(rule, "=", 2)
			if len(parts) == 2 {
				fieldInfo.Validation[parts[0]] = parts[1]
			}
		} else {
			fieldInfo.Validation[rule] = "true"
		}
	}
}

// extractHandlers extracts handler function information
func (p *ASTParser) extractHandlers(file *ast.File, packageName string) error {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			if p.isHandlerFunction(node) {
				handlerInfo := p.parseHandler(node, packageName)
				p.handlers[handlerInfo.Name] = handlerInfo
			}
		}
		return true
	})
	return nil
}

// isHandlerFunction checks if a function is an HTTP handler
func (p *ASTParser) isHandlerFunction(funcDecl *ast.FuncDecl) bool {
	// Check if function has the right signature for PocketBase handler
	if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) != 1 {
		return false
	}

	// Check parameter type (should be *core.RequestEvent)
	param := funcDecl.Type.Params.List[0]
	if starExpr, ok := param.Type.(*ast.StarExpr); ok {
		if selectorExpr, ok := starExpr.X.(*ast.SelectorExpr); ok {
			if ident, ok := selectorExpr.X.(*ast.Ident); ok {
				return ident.Name == "core" && selectorExpr.Sel.Name == "RequestEvent"
			}
		}
	}

	return false
}

// parseHandler parses a handler function
func (p *ASTParser) parseHandler(funcDecl *ast.FuncDecl, packageName string) *ASTHandlerInfo {
	handlerInfo := &ASTHandlerInfo{
		Name:       funcDecl.Name.Name,
		Package:    packageName,
		Parameters: make(map[string]*ParamInfo),
	}

	// Parse API directive comments
	p.parseAPIDirectives(handlerInfo, funcDecl)

	// Analyze function body for request/response types
	if funcDecl.Body != nil {
		p.analyzeHandlerBody(handlerInfo, funcDecl.Body)
	}

	return handlerInfo
}

// parseAPIDirectives extracts API_DESC and API_TAGS from comments above the function
func (p *ASTParser) parseAPIDirectives(handlerInfo *ASTHandlerInfo, funcDecl *ast.FuncDecl) {
	if funcDecl.Doc == nil {
		return
	}

	for _, comment := range funcDecl.Doc.List {
		commentText := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))

		// Parse API_DESC directive
		if strings.HasPrefix(commentText, "API_DESC ") {
			desc := strings.TrimSpace(strings.TrimPrefix(commentText, "API_DESC "))
			if desc != "" {
				handlerInfo.APIDescription = desc
			}
		}

		// Parse API_TAGS directive
		if strings.HasPrefix(commentText, "API_TAGS ") {
			tagsStr := strings.TrimSpace(strings.TrimPrefix(commentText, "API_TAGS "))
			if tagsStr != "" {
				// Split by comma and clean up each tag
				tags := strings.Split(tagsStr, ",")
				for i, tag := range tags {
					tags[i] = strings.TrimSpace(tag)
				}
				// Filter out empty tags
				var cleanTags []string
				for _, tag := range tags {
					if tag != "" {
						cleanTags = append(cleanTags, tag)
					}
				}
				if len(cleanTags) > 0 {
					handlerInfo.APITags = cleanTags
				}
			}
		}
	}
}

// analyzeHandlerBody analyzes handler function body to extract request/response info
func (p *ASTParser) analyzeHandlerBody(handlerInfo *ASTHandlerInfo, body *ast.BlockStmt) {
	// First pass: collect all variable declarations and their types
	variables := make(map[string]string)

	// Collect variable declarations
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.DeclStmt:
			if genDecl, ok := node.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valueSpec.Names {
							if valueSpec.Type != nil {
								varType := p.typeToString(valueSpec.Type)
								variables[name.Name] = varType
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
			// Handle short variable declarations with types
			if len(node.Lhs) == 1 && len(node.Rhs) == 1 {
				if ident, ok := node.Lhs[0].(*ast.Ident); ok {
					// Look for composite literal with type
					if compLit, ok := node.Rhs[0].(*ast.CompositeLit); ok && compLit.Type != nil {
						varType := p.typeToString(compLit.Type)
						variables[ident.Name] = varType
					}
				}
			}
		}
		return true
	})

	// Second pass: analyze function calls and usage
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			// Look for JSON decode calls
			if selectorExpr, ok := node.Fun.(*ast.SelectorExpr); ok {
				if p.isJSONDecodeCall(selectorExpr) {
					handlerInfo.UsesJSONDecode = true
					// Extract request type from decode call - Decode method has 1 argument (&req)
					if len(node.Args) > 0 {
						if unaryExpr, ok := node.Args[0].(*ast.UnaryExpr); ok {
							if ident, ok := unaryExpr.X.(*ast.Ident); ok {
								if varType, exists := variables[ident.Name]; exists {
									handlerInfo.RequestType = varType
								}
							}
						}
					}
				}
			}

			// Look for c.JSON calls (responses)
			if selectorExpr, ok := node.Fun.(*ast.SelectorExpr); ok {
				if p.isJSONResponseCall(selectorExpr) {
					handlerInfo.UsesJSONReturn = true
					// Extract response type from c.JSON call
					if len(node.Args) > 1 {
						responseType := p.analyzeResponseArgument(node.Args[1], variables)
						if responseType != "" {
							handlerInfo.ResponseType = responseType
						}
					}
				}
			}

			// Look for path parameter calls
			if selectorExpr, ok := node.Fun.(*ast.SelectorExpr); ok {
				if p.isPathValueCall(selectorExpr) && len(node.Args) > 0 {
					if basicLit, ok := node.Args[0].(*ast.BasicLit); ok {
						paramName := strings.Trim(basicLit.Value, `"`)
						handlerInfo.Parameters[paramName] = &ParamInfo{
							Name:     paramName,
							Type:     "string",
							Source:   "path",
							Required: true,
						}
					}
				}
			}

			// Look for query parameter calls
			if selectorExpr, ok := node.Fun.(*ast.SelectorExpr); ok {
				if p.isQueryParamCall(selectorExpr) && len(node.Args) > 0 {
					if basicLit, ok := node.Args[0].(*ast.BasicLit); ok {
						paramName := strings.Trim(basicLit.Value, `"`)
						handlerInfo.Parameters[paramName] = &ParamInfo{
							Name:     paramName,
							Type:     "string",
							Source:   "query",
							Required: false,
						}
					}
				}
			}
		}
		return true
	})
}

// Helper functions for analyzing function calls

func (p *ASTParser) isJSONDecodeCall(selectorExpr *ast.SelectorExpr) bool {
	if selectorExpr.Sel.Name != "Decode" {
		return false
	}

	// Handle chained method calls like json.NewDecoder(...).Decode(...)
	if callExpr, ok := selectorExpr.X.(*ast.CallExpr); ok {
		if selectorExpr2, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := selectorExpr2.X.(*ast.Ident); ok {
				// Check for json.NewDecoder pattern
				if ident.Name == "json" && selectorExpr2.Sel.Name == "NewDecoder" {
					return true
				}
			}
			// Also check for imported json package with alias
			if _, ok := selectorExpr2.X.(*ast.Ident); ok && selectorExpr2.Sel.Name == "NewDecoder" {
				// Could be an imported json package
				return true
			}
		}
	}

	// Additional check for direct method calls on json decoder
	if ident, ok := selectorExpr.X.(*ast.Ident); ok {
		// This might be a decoder variable directly calling Decode
		return strings.Contains(strings.ToLower(ident.Name), "decoder")
	}

	return false
}

func (p *ASTParser) isJSONResponseCall(selectorExpr *ast.SelectorExpr) bool {
	return selectorExpr.Sel.Name == "JSON"
}

func (p *ASTParser) isPathValueCall(selectorExpr *ast.SelectorExpr) bool {
	if selectorExpr.Sel.Name != "PathValue" {
		return false
	}

	if selectorExpr2, ok := selectorExpr.X.(*ast.SelectorExpr); ok {
		return selectorExpr2.Sel.Name == "Request"
	}
	return false
}

func (p *ASTParser) isQueryParamCall(selectorExpr *ast.SelectorExpr) bool {
	if selectorExpr.Sel.Name != "Get" {
		return false
	}

	// Check if it's a query parameter call by looking for URL.Query().Get() pattern
	if callExpr, ok := selectorExpr.X.(*ast.CallExpr); ok {
		if selectorExpr2, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			if selectorExpr2.Sel.Name == "Query" {
				if selectorExpr3, ok := selectorExpr2.X.(*ast.SelectorExpr); ok {
					return selectorExpr3.Sel.Name == "URL"
				}
			}
		}
	}
	return false
}

// findVariableType finds the type of a variable in the function body
func (p *ASTParser) findVariableType(body *ast.BlockStmt, varName string) string {
	var varType string

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.DeclStmt:
			if genDecl, ok := node.Decl.(*ast.GenDecl); ok {
				if genDecl.Tok == token.VAR {
					for _, spec := range genDecl.Specs {
						if valueSpec, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range valueSpec.Names {
								if name.Name == varName && valueSpec.Type != nil {
									varType = p.typeToString(valueSpec.Type)
									return false
								}
							}
						}
					}
				}
			}
		}
		return varType == ""
	})

	return varType
}

// analyzeResponseArgument analyzes the argument passed to c.JSON to determine response type
func (p *ASTParser) analyzeResponseArgument(arg ast.Expr, variables map[string]string) string {
	switch node := arg.(type) {
	case *ast.Ident:
		// Variable reference - look up its type
		if varType, exists := variables[node.Name]; exists {
			return varType
		}
		return node.Name
	case *ast.CompositeLit:
		// Composite literal with explicit type
		if node.Type != nil {
			return p.typeToString(node.Type)
		}
		// Anonymous struct - try to infer from context
		return ""
	case *ast.CallExpr:
		// Constructor call or function call
		if ident, ok := node.Fun.(*ast.Ident); ok {
			// Direct constructor call like UserResponse{...}
			return ident.Name
		}
		if selectorExpr, ok := node.Fun.(*ast.SelectorExpr); ok {
			// Method call like something.Build()
			return p.typeToString(selectorExpr)
		}
	case *ast.UnaryExpr:
		// Address-of expression &someVar
		if node.Op == token.AND {
			return p.analyzeResponseArgument(node.X, variables)
		}
	case *ast.SelectorExpr:
		// Field access like obj.Field
		return p.typeToString(node)
	}
	return ""
}

// typeToString converts ast.Expr to string representation
func (p *ASTParser) typeToString(expr ast.Expr) string {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.SelectorExpr:
		return p.typeToString(node.X) + "." + node.Sel.Name
	case *ast.StarExpr:
		return "*" + p.typeToString(node.X)
	case *ast.ArrayType:
		return "[]" + p.typeToString(node.Elt)
	case *ast.MapType:
		return "map[" + p.typeToString(node.Key) + "]" + p.typeToString(node.Value)
	case *ast.CompositeLit:
		if node.Type != nil {
			return p.typeToString(node.Type)
		}
		return ""
	default:
		return ""
	}
}

// extractComment extracts comment text
func (p *ASTParser) extractComment(commentGroup *ast.CommentGroup) string {
	if commentGroup == nil {
		return ""
	}

	var comments []string
	for _, comment := range commentGroup.List {
		text := strings.TrimPrefix(comment.Text, "//")
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		text = strings.TrimSpace(text)
		if text != "" {
			comments = append(comments, text)
		}
	}

	return strings.Join(comments, " ")
}

// Schema generation functions

// generateStructSchema generates JSON schema for a struct
func (p *ASTParser) generateStructSchema(structInfo *StructInfo) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	required := []string{}
	properties := schema["properties"].(map[string]interface{})

	for _, field := range structInfo.Fields {
		if field.JSONName == "-" {
			continue
		}

		fieldSchema := p.generateFieldSchema(field)
		properties[field.JSONName] = fieldSchema

		if field.Required && !field.JSONOmitEmpty {
			required = append(required, field.JSONName)
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

// generateFieldSchema generates JSON schema for a field
func (p *ASTParser) generateFieldSchema(field *FieldInfo) map[string]interface{} {
	schema := make(map[string]interface{})

	// Determine JSON type from Go type
	jsonType, format := p.goTypeToJSONType(field.Type)
	schema["type"] = jsonType

	if format != "" {
		schema["format"] = format
	}

	if field.Description != "" {
		schema["description"] = field.Description
	}

	// Add validation constraints
	for key, value := range field.Validation {
		switch key {
		case "min":
			if intVal, err := strconv.Atoi(value); err == nil {
				if jsonType == "string" {
					schema["minLength"] = intVal
				} else {
					schema["minimum"] = intVal
				}
			}
		case "max":
			if intVal, err := strconv.Atoi(value); err == nil {
				if jsonType == "string" {
					schema["maxLength"] = intVal
				} else {
					schema["maximum"] = intVal
				}
			}
		case "email":
			schema["format"] = "email"
		case "oneof":
			schema["enum"] = strings.Split(value, " ")
		}
	}

	// Add example if available
	if field.Example != nil {
		schema["example"] = field.Example
	}

	return schema
}

// goTypeToJSONType converts Go types to JSON Schema types
func (p *ASTParser) goTypeToJSONType(goType string) (string, string) {
	// Remove pointer prefix
	goType = strings.TrimPrefix(goType, "*")

	switch goType {
	case "string":
		return "string", ""
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return "integer", ""
	case "float32", "float64":
		return "number", ""
	case "bool":
		return "boolean", ""
	case "time.Time":
		return "string", "date-time"
	default:
		if strings.HasPrefix(goType, "[]") {
			return "array", ""
		}
		if strings.HasPrefix(goType, "map[") {
			return "object", ""
		}
		return "object", ""
	}
}

// GetStructByName returns struct information by name
func (p *ASTParser) GetStructByName(name string) (*StructInfo, bool) {
	structInfo, exists := p.structs[name]
	return structInfo, exists
}

// GetHandlerByName returns handler information by name
func (p *ASTParser) GetHandlerByName(name string) (*ASTHandlerInfo, bool) {
	handlerInfo, exists := p.handlers[name]
	return handlerInfo, exists
}

// GetAllStructs returns all parsed structs
func (p *ASTParser) GetAllStructs() map[string]*StructInfo {
	return p.structs
}

// GetAllHandlers returns all parsed handlers
func (p *ASTParser) GetAllHandlers() map[string]*ASTHandlerInfo {
	return p.handlers
}

// GenerateAPISchema generates complete API schema using parsed information
func (p *ASTParser) GenerateAPISchema(handlerName string) (map[string]interface{}, map[string]interface{}) {
	handler, exists := p.handlers[handlerName]
	if !exists {
		return nil, nil
	}

	var requestSchema, responseSchema map[string]interface{}

	// Generate request schema
	if handler.RequestType != "" {
		if structInfo, exists := p.structs[handler.RequestType]; exists {
			requestSchema = structInfo.JSONSchema
		}
	}

	// Generate response schema
	if handler.ResponseType != "" {
		if structInfo, exists := p.structs[handler.ResponseType]; exists {
			responseSchema = structInfo.JSONSchema
		}
	}

	return requestSchema, responseSchema
}

// Integration function for existing API docs system
// EnhanceEndpoint enhances an endpoint using AST analysis
func (p *ASTParser) EnhanceEndpoint(endpoint *APIEndpoint) {
	handlerName := endpoint.Handler

	// Minimal debug
	if handlerName == "createUserHandler" || handlerName == "updateUserHandler" || handlerName == "searchUsersHandler" {
		fmt.Printf("AST: Processing %s %s -> %s\n", endpoint.Method, endpoint.Path, handlerName)
	}

	// GET and DELETE methods typically don't have request bodies
	skipRequestProcessing := endpoint.Method == "GET" || endpoint.Method == "DELETE"
	if skipRequestProcessing {
		endpoint.Request = nil
	}

	if handler, exists := p.handlers[handlerName]; exists {
		// Apply API directive comments if present
		if handler.APIDescription != "" {
			endpoint.Description = handler.APIDescription
		}

		if len(handler.APITags) > 0 {
			endpoint.Tags = handler.APITags
		}

		// Update request schema - completely override existing schema if we have AST data
		if !skipRequestProcessing && handler.RequestType != "" {
			if structInfo := p.findStructByName(handler.RequestType); structInfo != nil && structInfo.JSONSchema != nil {
				if handlerName == "createUserHandler" || handlerName == "updateUserHandler" || handlerName == "searchUsersHandler" {
					fmt.Printf("AST: Replacing request schema with %s\n", handler.RequestType)
				}
				endpoint.Request = structInfo.JSONSchema
			}
		}

		// Update response schema - completely override existing schema if we have AST data
		if handler.ResponseType != "" {
			if structInfo := p.findStructByName(handler.ResponseType); structInfo != nil && structInfo.JSONSchema != nil {
				endpoint.Response = structInfo.JSONSchema
			}
		}
	}

	// Fallback: try to match based on handler name patterns only if no AST data found
	if !skipRequestProcessing && endpoint.Request == nil {
		if reqStruct := p.inferRequestStructFromHandler(handlerName, endpoint.Method); reqStruct != nil {
			endpoint.Request = reqStruct.JSONSchema
		}
	}

	if endpoint.Response == nil {
		if respStruct := p.inferResponseStructFromHandler(handlerName, endpoint.Method); respStruct != nil {
			endpoint.Response = respStruct.JSONSchema
		}
	}

	// Debug: Show final endpoint state
	if handlerName == "createUserHandler" || handlerName == "updateUserHandler" || handlerName == "searchUsersHandler" {
		fmt.Printf("AST: FINAL STATE for %s:\n", handlerName)
		if endpoint.Request != nil {
			reqMap := endpoint.Request
			if props, exists := reqMap["properties"]; exists {
				if propsMap, ok := props.(map[string]interface{}); ok {
					keys := make([]string, 0, len(propsMap))
					for k := range propsMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					fmt.Printf("AST:   Request properties: %v\n", keys)
				}
			}
		} else {
			fmt.Printf("AST: Final request: nil\n")
		}
		fmt.Println()
	}
}

// cleanTypeName removes pointer prefixes and package prefixes from type names
func (p *ASTParser) cleanTypeName(typeName string) string {
	// Remove pointer prefix
	typeName = strings.TrimPrefix(typeName, "*")

	// Remove package prefix (e.g., "main.UserRequest" -> "UserRequest")
	if dotIndex := strings.LastIndex(typeName, "."); dotIndex != -1 {
		typeName = typeName[dotIndex+1:]
	}

	return typeName
}

// findStructByName finds a struct with fallback matching strategies
func (p *ASTParser) findStructByName(typeName string) *StructInfo {
	// Clean the type name first
	cleanName := p.cleanTypeName(typeName)

	// Direct match
	if structInfo, exists := p.structs[cleanName]; exists {
		return structInfo
	}

	// Try original name
	if structInfo, exists := p.structs[typeName]; exists {
		return structInfo
	}

	// Partial matching for common patterns
	for structName, structInfo := range p.structs {
		// Case-insensitive match
		if strings.EqualFold(structName, cleanName) {
			return structInfo
		}

		// Suffix matching (e.g., "CreateUserRequest" matches "UserRequest")
		if strings.HasSuffix(structName, cleanName) {
			return structInfo
		}

		// Prefix matching (e.g., "User" matches "UserRequest")
		if strings.HasPrefix(structName, cleanName) {
			return structInfo
		}
	}

	return nil
}

// inferRequestStructFromHandler tries to infer request struct from handler name
func (p *ASTParser) inferRequestStructFromHandler(handlerName, method string) *StructInfo {
	// Extract base name from handler (e.g., "createUserHandler" -> "User")
	baseName := p.extractBaseNameFromHandler(handlerName)
	if baseName == "" {
		return nil
	}

	// Try common request naming patterns
	patterns := []string{
		"Create" + baseName + "Request",
		"Update" + baseName + "Request",
		baseName + "Request",
		"Create" + baseName,
		"Update" + baseName,
	}

	for _, pattern := range patterns {
		if structInfo := p.findStructByName(pattern); structInfo != nil {
			// Check if the pattern makes sense for the HTTP method
			if p.isPatternValidForMethod(pattern, method) {
				return structInfo
			}
		}
	}

	return nil
}

// inferResponseStructFromHandler tries to infer response struct from handler name
func (p *ASTParser) inferResponseStructFromHandler(handlerName, method string) *StructInfo {
	// Extract base name from handler
	baseName := p.extractBaseNameFromHandler(handlerName)
	if baseName == "" {
		return nil
	}

	// Try common response naming patterns
	patterns := []string{
		baseName + "Response",
		baseName + "ListResponse",  // For list endpoints
		baseName + "sListResponse", // Plural form
		baseName,
	}

	for _, pattern := range patterns {
		if structInfo := p.findStructByName(pattern); structInfo != nil {
			return structInfo
		}
	}

	return nil
}

// extractBaseNameFromHandler extracts the base entity name from handler name
func (p *ASTParser) extractBaseNameFromHandler(handlerName string) string {
	// Remove "Handler" suffix
	handlerName = strings.TrimSuffix(handlerName, "Handler")

	// Extract base name by removing common prefixes
	prefixes := []string{"create", "get", "update", "delete", "search", "list"}

	lowerName := strings.ToLower(handlerName)
	for _, prefix := range prefixes {
		if strings.HasPrefix(lowerName, prefix) {
			// Remove prefix and capitalize the result
			baseName := handlerName[len(prefix):]
			if len(baseName) > 0 {
				return strings.ToUpper(baseName[:1]) + baseName[1:]
			}
		}
	}

	// If no prefix found, use the handler name as is (capitalized)
	if len(handlerName) > 0 {
		return strings.ToUpper(handlerName[:1]) + handlerName[1:]
	}

	return ""
}

// isPatternValidForMethod checks if a request pattern makes sense for HTTP method
func (p *ASTParser) isPatternValidForMethod(pattern, method string) bool {
	pattern = strings.ToLower(pattern)
	method = strings.ToUpper(method)

	switch method {
	case "POST":
		return strings.Contains(pattern, "create") || strings.Contains(pattern, "request")
	case "PUT", "PATCH":
		return strings.Contains(pattern, "update") || strings.Contains(pattern, "request")
	case "GET", "DELETE":
		return !strings.Contains(pattern, "create") && !strings.Contains(pattern, "update")
	default:
		return true
	}
}

// isGenericSchema checks if the current schema is generic and should be replaced
func (p *ASTParser) isGenericSchema(schema map[string]interface{}) bool {
	if schema == nil {
		return true
	}

	// Check if it's a generic schema with additionalProperties: true
	if additionalProps, exists := schema["additionalProperties"]; exists {
		if additionalProps == true {
			return true
		}
	}

	// Check if it has generic description indicating it's auto-generated
	if desc, exists := schema["description"]; exists {
		if descStr, ok := desc.(string); ok {
			genericDescriptions := []string{
				"Request body",
				"Response data",
				"Record data to create",
				"Record data to update",
			}
			for _, genericDesc := range genericDescriptions {
				if descStr == genericDesc {
					return true
				}
			}
		}
	}

	// Check for PocketBase pattern-generated schemas
	if properties, exists := schema["properties"].(map[string]interface{}); exists {
		// Check for generic "data" field pattern
		if dataField, exists := properties["data"].(map[string]interface{}); exists {
			if desc, exists := dataField["description"].(string); exists {
				if desc == "Record data to create" || desc == "Record data to update" {
					return true
				}
			}
			// Check if data field has additionalProperties: true (generic object)
			if additionalProps, exists := dataField["additionalProperties"]; exists {
				if additionalProps == true {
					return true
				}
			}
		}

		// Check if schema only has generic record fields (id, created, updated)
		if len(properties) <= 3 {
			hasOnlyGenericFields := true
			for fieldName := range properties {
				if fieldName != "id" && fieldName != "created" && fieldName != "updated" {
					hasOnlyGenericFields = false
					break
				}
			}
			if hasOnlyGenericFields {
				return true
			}
		}
	}

	return false
}
