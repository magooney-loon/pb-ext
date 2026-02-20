package api

import (
	"go/ast"
	"go/token"
	"strings"
)

// =============================================================================
// Schema Analysis and Type Resolution
// =============================================================================

// analyzeMapLiteralSchema analyzes composite literals (map, struct, slice) to generate schemas.
// Returns nil if the expression is not a composite literal.
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
			var keyName string
			if basicLit, ok := kv.Key.(*ast.BasicLit); ok && basicLit.Kind.String() == "STRING" {
				keyName = strings.Trim(basicLit.Value, `"`)
			}

			if keyName != "" {
				valueSchema := p.analyzeValueExpression(kv.Value, handlerInfo)
				if valueSchema != nil {
					schema.Properties[keyName] = valueSchema
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
			if handlerInfo != nil {
				if tracedExpr, exists := handlerInfo.VariableExprs[e.Name]; exists {
					inner := tracedExpr
					if unary, ok := inner.(*ast.UnaryExpr); ok && unary.Op == token.AND {
						inner = unary.X
					}
					if schema := p.analyzeMapLiteralSchema(inner, handlerInfo); schema != nil {
						p.mergeMapAdditions(schema, e.Name, handlerInfo)
						return schema
					}
					if schema := p.analyzeValueExpression(inner, handlerInfo); schema != nil && schema.Type != "string" {
						p.mergeMapAdditions(schema, e.Name, handlerInfo)
						schema = p.enrichArraySchemaFromAppend(schema, e.Name, handlerInfo)
						return schema
					}
				}
				if varType, exists := handlerInfo.Variables[e.Name]; exists {
					if schema := p.resolveTypeToSchema(varType); schema != nil {
						p.mergeMapAdditions(schema, e.Name, handlerInfo)
						schema = p.enrichArraySchemaFromAppend(schema, e.Name, handlerInfo)
						return schema
					}
				}
			}
			return &OpenAPISchema{Type: "string"}
		}
	case *ast.CompositeLit:
		return p.analyzeCompositeLitSchema(e, handlerInfo)
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return p.analyzeValueExpression(e.X, handlerInfo)
		}
	case *ast.StarExpr:
		return p.analyzeValueExpression(e.X, handlerInfo)
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			switch sel.Sel.Name {
			case "GetString":
				return &OpenAPISchema{Type: "string"}
			case "GetBool":
				return &OpenAPISchema{Type: "boolean"}
			case "GetInt", "GetFloat":
				return &OpenAPISchema{Type: "number"}
			case "GetDateTime":
				return &OpenAPISchema{Type: "string", Format: "date-time"}
			case "Format":
				if x, ok := sel.X.(*ast.CallExpr); ok {
					if s, ok := x.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Now" {
						return &OpenAPISchema{Type: "string", Format: "date-time"}
					}
				}
				return &OpenAPISchema{Type: "string"}
			case "Unix", "UnixNano":
				return &OpenAPISchema{Type: "integer"}
			}
		}
		if ident, ok := e.Fun.(*ast.Ident); ok {
			switch ident.Name {
			case "len":
				return &OpenAPISchema{Type: "integer", Minimum: floatPtr(0)}
			case "make":
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
				if bodySchema, ok := p.funcBodySchemas[ident.Name]; ok {
					return p.deepCopySchema(bodySchema)
				}
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
		if ident, ok := e.X.(*ast.Ident); ok && handlerInfo != nil {
			if keyLit, ok := e.Index.(*ast.BasicLit); ok && keyLit.Kind == token.STRING {
				key := strings.Trim(keyLit.Value, `"`)
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
		return &OpenAPISchema{Type: "string"}
	case *ast.SelectorExpr:
		if ident, ok := e.X.(*ast.Ident); ok && handlerInfo != nil {
			fieldName := e.Sel.Name
			if varType, exists := handlerInfo.Variables[ident.Name]; exists {
				structName := strings.TrimPrefix(varType, "*")
				structName = strings.TrimPrefix(structName, "[]")
				if structInfo, exists := p.structs[structName]; exists {
					for _, fi := range structInfo.Fields {
						if fi.Name == fieldName {
							return p.resolveTypeToSchema(fi.Type)
						}
					}
				}
			}
		}

		if sel := e.Sel.Name; sel != "" {
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

	return &OpenAPISchema{Type: "string"}
}

// resolveTypeToSchema converts a Go type name to an OpenAPI schema
func (p *ASTParser) resolveTypeToSchema(typeName string) *OpenAPISchema {
	if strings.HasPrefix(typeName, "[]") {
		elemType := strings.TrimPrefix(typeName, "[]")
		elemSchema := p.resolveTypeToSchema(elemType)
		if elemSchema != nil {
			return &OpenAPISchema{Type: "array", Items: elemSchema}
		}
		return &OpenAPISchema{Type: "array", Items: &OpenAPISchema{Type: "object"}}
	}

	if strings.HasPrefix(typeName, "map[") {
		closeBracket := strings.Index(typeName, "]")
		if closeBracket > 0 && closeBracket < len(typeName)-1 {
			valueType := typeName[closeBracket+1:]
			if valueType == "any" || valueType == "interface{}" {
				return &OpenAPISchema{Type: "object", AdditionalProperties: true}
			}
			valueSchema := p.resolveTypeToSchema(valueType)
			if valueSchema != nil {
				return &OpenAPISchema{Type: "object", AdditionalProperties: valueSchema}
			}
		}
		return &OpenAPISchema{Type: "object", AdditionalProperties: true}
	}

	if strings.HasPrefix(typeName, "*") {
		return p.resolveTypeToSchema(strings.TrimPrefix(typeName, "*"))
	}

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
		if len(e.Elts) > 0 {
			if _, ok := e.Elts[0].(*ast.KeyValueExpr); ok {
				schema := &OpenAPISchema{
					Type:       "object",
					Properties: make(map[string]*OpenAPISchema),
				}
				for _, elt := range e.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						var keyName string
						if ident, ok := kv.Key.(*ast.Ident); ok {
							keyName = ident.Name
						}
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

	if _, ok := e.Type.(*ast.MapType); ok {
		return p.parseMapLiteral(e, handlerInfo)
	}

	if arrayType, ok := e.Type.(*ast.ArrayType); ok {
		elemTypeName := p.extractTypeName(arrayType.Elt)
		if elemTypeName != "" {
			elemSchema := p.generateSchemaForEndpoint(elemTypeName)
			if elemSchema != nil {
				return &OpenAPISchema{Type: "array", Items: elemSchema}
			}
		}
		return p.parseArrayLiteral(e, handlerInfo)
	}

	typeName := p.extractTypeName(e.Type)
	if typeName != "" {
		resolvedType, _ := p.resolveTypeAlias(typeName, nil)
		if _, exists := p.structs[resolvedType]; exists {
			return &OpenAPISchema{Ref: "#/components/schemas/" + resolvedType}
		}
		if _, exists := p.structs[typeName]; exists {
			return &OpenAPISchema{Ref: "#/components/schemas/" + typeName}
		}
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
								if typeName := p.inferTypeFromExpression(valueSpec.Values[i], globalVars); typeName != "" {
									globalVars.Variables[name.Name] = typeName
								}
							}
						}
					}
				}
			}
		case *ast.AssignStmt:
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
			if node.Tok == token.DEFINE {
				for i, lhs := range node.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && i < len(node.Rhs) {
						rhs := node.Rhs[i]
						if typeName := p.inferTypeFromExpression(rhs, handlerInfo); typeName != "" {
							handlerInfo.Variables[ident.Name] = typeName
						}
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
						if typeName := p.inferTypeFromExpression(rhs, handlerInfo); typeName != "" {
							handlerInfo.Variables[name.Name] = typeName
						}
					}
					handlerInfo.VariableExprs[name.Name] = rhs
				}
			}
		}
	}
}

// resolveTypeAlias resolves a type alias recursively to find the real type.
// Handles qualified types (e.g., searchresult.ExportData) by extracting the simple name.
func (p *ASTParser) resolveTypeAlias(typeName string, visited map[string]bool) (string, bool) {
	if visited == nil {
		visited = make(map[string]bool)
	}

	if visited[typeName] {
		return typeName, false
	}
	visited[typeName] = true

	if realType, exists := p.typeAliases[typeName]; exists {
		resolved, _ := p.resolveTypeAlias(realType, visited)
		return resolved, true
	}

	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		simpleName := parts[len(parts)-1]

		if realType, exists := p.typeAliases[simpleName]; exists {
			resolved, _ := p.resolveTypeAlias(realType, visited)
			return resolved, true
		}

		if _, exists := p.structs[simpleName]; exists {
			return simpleName, false
		}

		return simpleName, false
	}

	return typeName, false
}

// generateSchemaForEndpoint generates an OpenAPI schema for a top-level endpoint request/response.
// Always uses $ref for known struct types so they appear in components/schemas.
func (p *ASTParser) generateSchemaForEndpoint(typeName string) *OpenAPISchema {
	if strings.HasPrefix(typeName, "[]") {
		elementType := strings.TrimPrefix(typeName, "[]")
		elementSchema := p.generateSchemaForEndpoint(elementType)
		if elementSchema != nil {
			return &OpenAPISchema{Type: "array", Items: elementSchema}
		}
		return &OpenAPISchema{Type: "array", Items: &OpenAPISchema{Type: "object"}}
	}

	resolvedType, _ := p.resolveTypeAlias(typeName, nil)

	if _, exists := p.structs[resolvedType]; exists {
		return &OpenAPISchema{Ref: "#/components/schemas/" + resolvedType}
	}
	if _, exists := p.structs[typeName]; exists {
		return &OpenAPISchema{Ref: "#/components/schemas/" + typeName}
	}

	return p.generateSchemaFromType(typeName, false)
}

// generateSchemaFromType generates an OpenAPI schema from a type name.
// inline: if true, returns the full schema for struct types; if false, returns $ref.
func (p *ASTParser) generateSchemaFromType(typeName string, inline bool) *OpenAPISchema {
	if strings.HasPrefix(typeName, "[]") {
		elementType := strings.TrimPrefix(typeName, "[]")
		elementSchema := p.generateSchemaFromType(elementType, inline)
		if elementSchema != nil {
			return &OpenAPISchema{Type: "array", Items: elementSchema}
		}
		return &OpenAPISchema{Type: "array", Items: &OpenAPISchema{Type: "object"}}
	}

	resolvedType, isAlias := p.resolveTypeAlias(typeName, nil)

	typeToUse := resolvedType
	if !isAlias {
		typeToUse = typeName
	}

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

	if structInfo, exists := p.structs[typeToUse]; exists {
		if structInfo.JSONSchema == nil {
			return &OpenAPISchema{Ref: "#/components/schemas/" + typeToUse}
		}
		if inline {
			return p.deepCopySchema(structInfo.JSONSchema)
		}
		return &OpenAPISchema{Ref: "#/components/schemas/" + typeToUse}
	}

	if strings.HasPrefix(typeName, "map[") {
		closeBracket := strings.Index(typeName, "]")
		if closeBracket > 0 && closeBracket < len(typeName)-1 {
			valueType := typeName[closeBracket+1:]
			if valueType == "any" || valueType == "interface{}" {
				return &OpenAPISchema{Type: "object", AdditionalProperties: true}
			}
			valueSchema := p.generateSchemaFromType(valueType, false)
			if valueSchema != nil {
				return &OpenAPISchema{Type: "object", AdditionalProperties: valueSchema}
			}
		}
		return &OpenAPISchema{Type: "object", AdditionalProperties: true}
	}

	if typeName == "interface{}" || typeName == "any" {
		return &OpenAPISchema{Type: "object", AdditionalProperties: true}
	}

	if isAlias {
		simpleName := resolvedType
		if strings.Contains(resolvedType, ".") {
			parts := strings.Split(resolvedType, ".")
			simpleName = parts[len(parts)-1]
		}

		if structInfo, exists := p.structs[simpleName]; exists {
			if inline {
				return p.deepCopySchema(structInfo.JSONSchema)
			}
			return &OpenAPISchema{Ref: "#/components/schemas/" + simpleName}
		}

		return &OpenAPISchema{Ref: "#/components/schemas/" + simpleName}
	}

	simpleName := typeName
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		simpleName = parts[len(parts)-1]
	}

	return &OpenAPISchema{Ref: "#/components/schemas/" + simpleName}
}

// =============================================================================
// PocketBase Pattern Initialization
// =============================================================================

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
