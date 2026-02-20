package api

import (
	"go/ast"
	"go/token"
	"strings"
)

// =============================================================================
// Second Pass: Function and Handler Extraction
// =============================================================================

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

// extractHelperFuncParams analyzes non-handler functions that accept *core.RequestEvent and
// extracts the query/header/path parameters they read. Results are stored in p.funcParamSchemas
// so that handlers calling those helpers automatically inherit the detected parameters.
//
// This covers patterns like:
//
//	func parseTimeParams(e *core.RequestEvent) timeParams {
//	    q := e.Request.URL.Query()
//	    q.Get("interval") ...
//	}
//
//	func parseIntParam(e *core.RequestEvent, name string, def int) int {
//	    e.Request.URL.Query().Get(name)   // ← name is a variable, handled via second-arg detection
//	}
func (p *ASTParser) extractHelperFuncParams(file *ast.File) {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil || funcDecl.Recv != nil {
			continue
		}
		// Skip handlers — they're processed separately
		if p.isPocketBaseHandler(funcDecl) {
			continue
		}
		// Must accept at least one *core.RequestEvent parameter
		if !hasRequestEventParam(funcDecl) {
			continue
		}

		// Collect the name(s) of the *core.RequestEvent parameter(s)
		eventParamNames := requestEventParamNames(funcDecl)

		// Collect the name(s) of plain string parameters (for generic helpers like parseIntParam)
		stringParamNames := stringParamNames(funcDecl)

		params := p.extractParamsFromBody(funcDecl.Body, eventParamNames, stringParamNames)
		if len(params) > 0 {
			// Store params (may include sentinel entries with Name="" for generic helpers)
			p.funcParamSchemas[funcDecl.Name.Name] = params
		} else if len(stringParamNames) > 0 {
			// Generic helper whose body uses only variable param names with no detectable source context
			// (e.g. the receiver is not a known URL.Query or Header selector). Register a default
			// "query" sentinel so the call-site can still extract the literal from the 2nd arg.
			p.funcParamSchemas[funcDecl.Name.Name] = []*ParamInfo{{Name: "", Type: "string", Source: "query"}}
		}
	}
}

// extractParamsFromBody is the core walker used by both handler and helper extraction.
// eventParamNames: identifiers bound to *core.RequestEvent (e.g. "e", "c")
// stringParamNames: identifiers that hold a query/header param name as a string variable
// (used for generic helpers like parseIntParam(e, name, default) where name is a variable)
func (p *ASTParser) extractParamsFromBody(body *ast.BlockStmt, eventParamNames map[string]bool, stringParamNames map[string]bool) []*ParamInfo {
	queryVarNames := map[string]bool{}
	requestInfoVars := map[string]bool{}
	var params []*ParamInfo

	ast.Inspect(body, func(n ast.Node) bool {
		// Track q := e.Request.URL.Query() and info, _ := e.RequestInfo()
		if assign, ok := n.(*ast.AssignStmt); ok {
			for i, rhs := range assign.Rhs {
				if i >= len(assign.Lhs) {
					continue
				}
				ident, ok := assign.Lhs[i].(*ast.Ident)
				if !ok {
					continue
				}
				if isURLQueryCallForEvent(rhs, eventParamNames) || isURLQueryCall(rhs) {
					queryVarNames[ident.Name] = true
				}
				if isRequestInfoCallForEvent(rhs, eventParamNames) || isRequestInfoCall(rhs) {
					requestInfoVars[ident.Name] = true
				}
			}
		}

		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				switch sel.Sel.Name {
				case "Get":
					if paramName, paramOK := firstStringArg(call); paramOK {
						// q.Get("name") where q is a URL.Query() var
						if ident, ok := sel.X.(*ast.Ident); ok && queryVarNames[ident.Name] {
							params = appendParamIfNew(params, &ParamInfo{Name: paramName, Type: "string", Source: "query"})
						}
						// e.Request.URL.Query().Get("name") inline
						if isURLQueryCall(sel.X) || isURLQueryCallForEvent(sel.X, eventParamNames) {
							params = appendParamIfNew(params, &ParamInfo{Name: paramName, Type: "string", Source: "query"})
						}
						// e.Request.Header.Get("name")
						if isRequestHeaderSelector(sel.X) || isRequestHeaderSelectorForEvent(sel.X, eventParamNames) {
							params = appendParamIfNew(params, &ParamInfo{Name: paramName, Type: "string", Source: "header"})
						}
					} else if len(call.Args) > 0 {
						// Generic: varName is a known string param variable — record source via sentinel.
						if ident, ok := call.Args[0].(*ast.Ident); ok && stringParamNames[ident.Name] {
							if (isRequestHeaderSelector(sel.X) || isRequestHeaderSelectorForEvent(sel.X, eventParamNames)) {
								// e.Request.Header.Get(name) — sentinel with source="header", name=""
								params = appendParamIfNew(params, &ParamInfo{Name: "", Type: "string", Source: "header"})
							} else if recv, ok := sel.X.(*ast.Ident); ok && queryVarNames[recv.Name] {
								// q.Get(name) — sentinel with source="query", name=""
								params = appendParamIfNew(params, &ParamInfo{Name: "", Type: "string", Source: "query"})
								_ = recv
							} else if isURLQueryCall(sel.X) || isURLQueryCallForEvent(sel.X, eventParamNames) {
								// e.Request.URL.Query().Get(name) — sentinel with source="query", name=""
								params = appendParamIfNew(params, &ParamInfo{Name: "", Type: "string", Source: "query"})
							}
						}
					}
				case "PathValue":
					if paramName, ok := firstStringArg(call); ok {
						params = appendParamIfNew(params, &ParamInfo{Name: paramName, Type: "string", Source: "path", Required: true})
					}
				}
			}
		}

		// info.Query["name"] and info.Headers["name"]
		if indexExpr, ok := n.(*ast.IndexExpr); ok {
			if paramName, ok := stringLiteralValue(indexExpr.Index); ok {
				if sel, ok := indexExpr.X.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok && requestInfoVars[ident.Name] {
						switch sel.Sel.Name {
						case "Query":
							params = appendParamIfNew(params, &ParamInfo{Name: paramName, Type: "string", Source: "query"})
						case "Headers":
							params = appendParamIfNew(params, &ParamInfo{Name: paramName, Type: "string", Source: "header"})
						}
					}
				}
				// q["name"] where q is a URL.Query() var
				if ident, ok := indexExpr.X.(*ast.Ident); ok && queryVarNames[ident.Name] {
					params = appendParamIfNew(params, &ParamInfo{Name: paramName, Type: "string", Source: "query"})
				}
			}
		}

		return true
	})

	return params
}

// hasRequestEventParam returns true if funcDecl has at least one *core.RequestEvent parameter.
func hasRequestEventParam(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type.Params == nil {
		return false
	}
	for _, field := range funcDecl.Type.Params.List {
		if star, ok := field.Type.(*ast.StarExpr); ok {
			if sel, ok := star.X.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "RequestEvent" {
					return true
				}
			}
		}
	}
	return false
}

// requestEventParamNames returns a set of parameter names bound to *core.RequestEvent.
func requestEventParamNames(funcDecl *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	if funcDecl.Type.Params == nil {
		return names
	}
	for _, field := range funcDecl.Type.Params.List {
		if star, ok := field.Type.(*ast.StarExpr); ok {
			if sel, ok := star.X.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "RequestEvent" {
					for _, name := range field.Names {
						names[name.Name] = true
					}
				}
			}
		}
	}
	return names
}

// stringParamNames returns a set of parameter names that are plain strings
// (used for generic helpers like parseIntParam(e, name string, def int)).
func stringParamNames(funcDecl *ast.FuncDecl) map[string]bool {
	names := map[string]bool{}
	if funcDecl.Type.Params == nil {
		return names
	}
	for _, field := range funcDecl.Type.Params.List {
		if ident, ok := field.Type.(*ast.Ident); ok && ident.Name == "string" {
			for _, name := range field.Names {
				names[name.Name] = true
			}
		}
	}
	return names
}

// isURLQueryCallForEvent is like isURLQueryCall but also matches e.Request.URL.Query()
// when the receiver matches one of the known event param names.
func isURLQueryCallForEvent(expr ast.Expr, eventParams map[string]bool) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Query" {
		return false
	}
	urlSel, ok := sel.X.(*ast.SelectorExpr)
	if !ok || urlSel.Sel.Name != "URL" {
		return false
	}
	reqSel, ok := urlSel.X.(*ast.SelectorExpr)
	if !ok || reqSel.Sel.Name != "Request" {
		return false
	}
	if ident, ok := reqSel.X.(*ast.Ident); ok {
		return eventParams[ident.Name]
	}
	return false
}

// isRequestInfoCallForEvent matches e.RequestInfo() for known event param names.
func isRequestInfoCallForEvent(expr ast.Expr, eventParams map[string]bool) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "RequestInfo" {
		return false
	}
	if ident, ok := sel.X.(*ast.Ident); ok {
		return eventParams[ident.Name]
	}
	return false
}

// isRequestHeaderSelectorForEvent matches e.Request.Header for known event param names.
func isRequestHeaderSelectorForEvent(expr ast.Expr, eventParams map[string]bool) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Header" {
		return false
	}
	reqSel, ok := sel.X.(*ast.SelectorExpr)
	if !ok || reqSel.Sel.Name != "Request" {
		return false
	}
	if ident, ok := reqSel.X.(*ast.Ident); ok {
		return eventParams[ident.Name]
	}
	return false
}

// analyzeHelperFuncBody walks a helper function's body to find map[string]any composite
// literals and extract their keys/value types.
func (p *ASTParser) analyzeHelperFuncBody(funcDecl *ast.FuncDecl) *OpenAPISchema {
	tempInfo := &ASTHandlerInfo{
		Variables:        make(map[string]string),
		VariableExprs:    make(map[string]ast.Expr),
		MapAdditions:     make(map[string][]MapKeyAdd),
		SliceAppendExprs: make(map[string]ast.Expr),
	}

	var bestSchema *OpenAPISchema
	var bestVarName string
	bestKeyCount := 0

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			p.trackVariableAssignment(assign, tempInfo)
		}

		compLit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		mapType, ok := compLit.Type.(*ast.MapType)
		if !ok {
			return true
		}
		keyType := p.extractTypeName(mapType.Key)
		valType := p.extractTypeName(mapType.Value)
		if keyType != "string" || (valType != "any" && valType != "interface{}") {
			return true
		}
		schema := p.parseMapLiteral(compLit, tempInfo)
		// Use properties+required as the key count so that nil-initialized maps
		// (which have empty Properties but non-empty Required after Fix 1) are
		// still selected as the best schema and can be enriched by mergeMapAdditions.
		keyCount := len(schema.Properties) + len(schema.Required)
		if schema != nil && keyCount > bestKeyCount {
			bestSchema = schema
			bestKeyCount = keyCount
			bestVarName = p.findAssignedVariable(funcDecl.Body, compLit)
		}
		return true
	})

	if bestSchema == nil {
		return nil
	}

	if bestVarName != "" {
		p.mergeMapAdditions(bestSchema, bestVarName, tempInfo)
	}

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

// extractQueryParameters detects query, header, and path parameter usage patterns in handler bodies.
func (p *ASTParser) extractQueryParameters(body *ast.BlockStmt, handlerInfo *ASTHandlerInfo) {
	queryVarNames := map[string]bool{}
	requestInfoVars := map[string]bool{}

	ast.Inspect(body, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			for i, rhs := range assign.Rhs {
				if i >= len(assign.Lhs) {
					continue
				}
				ident, ok := assign.Lhs[i].(*ast.Ident)
				if !ok {
					continue
				}
				if isURLQueryCall(rhs) {
					queryVarNames[ident.Name] = true
				}
				if isRequestInfoCall(rhs) {
					requestInfoVars[ident.Name] = true
				}
			}
		}

		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				switch sel.Sel.Name {
				case "Get":
					if paramName, ok := firstStringArg(call); ok {
						if ident, ok := sel.X.(*ast.Ident); ok && queryVarNames[ident.Name] {
							handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, &ParamInfo{
								Name:   paramName,
								Type:   "string",
								Source: "query",
							})
						}
						if isURLQueryCall(sel.X) {
							handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, &ParamInfo{
								Name:   paramName,
								Type:   "string",
								Source: "query",
							})
						}
						if isRequestHeaderSelector(sel.X) {
							handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, &ParamInfo{
								Name:   paramName,
								Type:   "string",
								Source: "header",
							})
						}
					}
				case "PathValue":
					if paramName, ok := firstStringArg(call); ok {
						handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, &ParamInfo{
							Name:     paramName,
							Type:     "string",
							Source:   "path",
							Required: true,
						})
					}
				}
			}
		}

		if indexExpr, ok := n.(*ast.IndexExpr); ok {
			if paramName, ok := stringLiteralValue(indexExpr.Index); ok {
				if sel, ok := indexExpr.X.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok && requestInfoVars[ident.Name] {
						switch sel.Sel.Name {
						case "Query":
							handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, &ParamInfo{
								Name:   paramName,
								Type:   "string",
								Source: "query",
							})
						case "Headers":
							handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, &ParamInfo{
								Name:   paramName,
								Type:   "string",
								Source: "header",
							})
						}
					}
				}
				if ident, ok := indexExpr.X.(*ast.Ident); ok && queryVarNames[ident.Name] {
					handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, &ParamInfo{
						Name:   paramName,
						Type:   "string",
						Source: "query",
					})
				}
			}
		}

		return true
	})

	// Merge params from known helper functions called in this handler body.
	// Two patterns:
	//   1. Domain helper:   parseTimeParams(e)          → merge all params stored in funcParamSchemas
	//   2. Generic helper:  parseIntParam(e, "page", 0) → extract param name from 2nd string-literal arg
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}

		if helperParams, exists := p.funcParamSchemas[ident.Name]; exists {
			// Separate literal params (Name != "") from sentinel params (Name == "").
			// Sentinels encode which source type a generic helper reads (query vs header).
			var literalParams []*ParamInfo
			sentinelSource := "query" // default if no sentinel found
			for _, param := range helperParams {
				if param.Name == "" {
					sentinelSource = param.Source
				} else {
					literalParams = append(literalParams, param)
				}
			}

			if len(literalParams) > 0 {
				// Domain helper: all params were statically resolved in the helper body
				for _, param := range literalParams {
					handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, param)
				}
			} else {
				// Generic helper: try to extract param name from 2nd string-literal arg at the call site.
				// e.g. parseIntParam(e, "page", 0)      → "page" query param
				//      parseHeaderParam(e, "Authorization") → "Authorization" header param
				if len(call.Args) >= 2 {
					if paramName, ok := stringLiteralValue(call.Args[1]); ok {
						handlerInfo.Parameters = appendParamIfNew(handlerInfo.Parameters, &ParamInfo{
							Name:   paramName,
							Type:   "string",
							Source: sentinelSource,
						})
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
	if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
		return innerSel.Sel.Name == "URL"
	}
	return false
}

// isRequestInfoCall checks if an expression is a call to e.RequestInfo()
func isRequestInfoCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "RequestInfo"
}

// isRequestHeaderSelector checks if an expression matches e.Request.Header
func isRequestHeaderSelector(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Header" {
		return false
	}
	if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
		return innerSel.Sel.Name == "Request"
	}
	return false
}

// firstStringArg extracts the first string literal argument from a call expression
func firstStringArg(call *ast.CallExpr) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}
	return stringLiteralValue(call.Args[0])
}

// stringLiteralValue extracts the string value from a BasicLit string expression
func stringLiteralValue(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	return strings.Trim(lit.Value, `"`), true
}

// appendParamIfNew appends a ParamInfo to the slice if no param with the same name and source exists
func appendParamIfNew(params []*ParamInfo, param *ParamInfo) []*ParamInfo {
	for _, p := range params {
		if p.Name == param.Name && p.Source == param.Source {
			return params
		}
	}
	return append(params, param)
}

// isPocketBaseHandler checks if a function is a PocketBase handler.
// A handler has the signature: func(c *core.RequestEvent) error
// (exactly one *core.RequestEvent param and returns error).
func (p *ASTParser) isPocketBaseHandler(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) != 1 {
		return false
	}

	param := funcDecl.Type.Params.List[0]
	if star, ok := param.Type.(*ast.StarExpr); ok {
		if sel, ok := star.X.(*ast.SelectorExpr); ok {
			if sel.Sel.Name != "RequestEvent" {
				return false
			}
		} else {
			return false
		}
	} else {
		return false
	}

	// Must return exactly one value: the built-in `error` type.
	results := funcDecl.Type.Results
	if results == nil || len(results.List) != 1 {
		return false
	}
	if ident, ok := results.List[0].Type.(*ast.Ident); ok {
		return ident.Name == "error"
	}
	return false
}

// analyzePocketBaseHandler analyzes a PocketBase handler function
func (p *ASTParser) analyzePocketBaseHandler(funcDecl *ast.FuncDecl) *ASTHandlerInfo {
	handlerInfo := &ASTHandlerInfo{
		Name:              funcDecl.Name.Name,
		Variables:         make(map[string]string),
		VariableExprs:     make(map[string]ast.Expr),
		MapAdditions:      make(map[string][]MapKeyAdd),
		SliceAppendExprs:  make(map[string]ast.Expr),
		AnonStructSchemas: make(map[string]*OpenAPISchema),
	}

	if funcDecl.Doc != nil {
		p.parseHandlerComments(funcDecl.Doc, handlerInfo)
	}

	if funcDecl.Body != nil {
		p.analyzePocketBasePatterns(funcDecl.Body, handlerInfo)
	}

	if handlerInfo.RequestType != "" {
		if schema := p.generateSchemaForEndpoint(handlerInfo.RequestType); schema != nil {
			handlerInfo.RequestSchema = schema
		}
	}

	if funcDecl.Body != nil {
		p.extractLocalVariables(funcDecl.Body, handlerInfo)
	}

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
			p.handleJSONDecode(call, handlerInfo)
		case "NewDecoder":
			p.handleNewDecoder(handlerInfo)
		}
	} else if ident, ok := call.Fun.(*ast.Ident); ok {
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
		if unary, ok := call.Args[0].(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if ident, ok := unary.X.(*ast.Ident); ok {
				if varType, exists := handlerInfo.Variables[ident.Name]; exists && varType != "" {
					handlerInfo.RequestType = varType
				} else if schema, exists := handlerInfo.AnonStructSchemas[ident.Name]; exists {
					// var body struct{...} — anonymous inline struct, no named type
					handlerInfo.RequestSchema = schema
				}
			}
		}
	}
}

// handleJSONResponse handles e.JSON(status, data) pattern
func (p *ASTParser) handleJSONResponse(call *ast.CallExpr, handlerInfo *ASTHandlerInfo) {
	if len(call.Args) >= 2 {
		arg := call.Args[1]

		if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.AND {
			arg = unary.X
		}

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

		for _, candidate := range exprsToAnalyze {
			if schema := p.analyzeMapLiteralSchema(candidate, handlerInfo); schema != nil {
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

		for _, candidate := range exprsToAnalyze {
			if responseType := p.inferTypeFromExpression(candidate, handlerInfo); responseType != "" {
				handlerInfo.ResponseType = responseType
				if schema := p.generateSchemaForEndpoint(responseType); schema != nil {
					handlerInfo.ResponseSchema = schema
					return
				}
			}
		}

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
		if unary, ok := call.Args[0].(*ast.UnaryExpr); ok && unary.Op == token.AND {
			if ident, ok := unary.X.(*ast.Ident); ok {
				if varType, exists := handlerInfo.Variables[ident.Name]; exists {
					handlerInfo.RequestType = varType
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
			handlerInfo.VariableExprs[ident.Name] = rhs

			if handlerInfo.SliceAppendExprs != nil {
				if callExpr, ok := rhs.(*ast.CallExpr); ok {
					if fnIdent, ok := callExpr.Fun.(*ast.Ident); ok && fnIdent.Name == "append" {
						if len(callExpr.Args) == 2 {
							if argIdent, ok := callExpr.Args[0].(*ast.Ident); ok && argIdent.Name == ident.Name {
								handlerInfo.SliceAppendExprs[ident.Name] = callExpr.Args[1]
							}
						}
					}
				}
			}
		}

		if indexExpr, ok := lhs.(*ast.IndexExpr); ok {
			if ident, ok := indexExpr.X.(*ast.Ident); ok {
				if basicLit, ok := indexExpr.Index.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
					// map[string]any string-key assignment: mapVar["key"] = value
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
				} else {
					// Numeric/variable index assignment: slice[i] = expr
					// If the slice is a known []map[string]any, treat the RHS the same as an
					// append item so enrichArraySchemaFromAppend can resolve the item schema.
					if handlerInfo.SliceAppendExprs != nil {
						varType := handlerInfo.Variables[ident.Name]
						if varType == "[]map[string]any" || varType == "[]map[string]interface{}" {
							var valueExpr ast.Expr
							if i < len(assign.Rhs) {
								valueExpr = assign.Rhs[i]
							} else if len(assign.Rhs) == 1 {
								valueExpr = assign.Rhs[0]
							}
							if valueExpr != nil {
								// Only store if we don't already have a richer source
								if _, exists := handlerInfo.SliceAppendExprs[ident.Name]; !exists {
									handlerInfo.SliceAppendExprs[ident.Name] = valueExpr
								}
							}
						}
					}
				}
			}
		}
	}
}

// mergeMapAdditions merges dynamically added map keys into an existing object schema
func (p *ASTParser) mergeMapAdditions(schema *OpenAPISchema, varName string, handlerInfo *ASTHandlerInfo) {
	additions, exists := handlerInfo.MapAdditions[varName]
	if !exists || len(additions) == 0 {
		return
	}
	if schema.Properties == nil {
		schema.Properties = make(map[string]*OpenAPISchema)
	}
	for _, add := range additions {
		if _, exists := schema.Properties[add.Key]; exists {
			continue
		}
		valueSchema := p.analyzeValueExpression(add.Value, handlerInfo)
		if valueSchema != nil {
			schema.Properties[add.Key] = valueSchema
		}
	}
	// If we successfully added concrete properties, clear the generic additionalProperties flag
	// so Swagger UI doesn't render a spurious "additionalProp1" entry.
	if len(schema.Properties) > 0 && schema.AdditionalProperties == true {
		schema.AdditionalProperties = nil
	}
}

// enrichArraySchemaFromAppend checks if an array schema has generic items and tries to
// resolve richer items from tracked append() patterns.
func (p *ASTParser) enrichArraySchemaFromAppend(schema *OpenAPISchema, varName string, handlerInfo *ASTHandlerInfo) *OpenAPISchema {
	if schema.Type != "array" || schema.Items == nil {
		return schema
	}
	if len(schema.Items.Properties) > 0 || schema.Items.Ref != "" {
		return schema
	}

	if handlerInfo.SliceAppendExprs == nil {
		return schema
	}

	appendExpr, exists := handlerInfo.SliceAppendExprs[varName]
	if !exists {
		return schema
	}

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
		if typeName := p.extractTypeName(e.Type); typeName != "" {
			return typeName
		}
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return p.extractTypeName(e.X)
		}
	case *ast.Ident:
		if varType, exists := handlerInfo.Variables[e.Name]; exists {
			return varType
		}
		if _, exists := p.structs[e.Name]; exists {
			return e.Name
		}
		name := e.Name
		if strings.HasSuffix(name, "Request") || strings.HasSuffix(name, "Response") ||
			strings.HasSuffix(name, "Data") || strings.HasSuffix(name, "Input") ||
			strings.HasSuffix(name, "Output") || strings.HasSuffix(name, "Payload") {
			return name
		}
	case *ast.CallExpr:
		if ident, ok := e.Fun.(*ast.Ident); ok {
			if ident.Name == "make" && len(e.Args) > 0 {
				typeName := p.extractTypeName(e.Args[0])
				if typeName != "" {
					return typeName
				}
			}
			if strings.HasPrefix(ident.Name, "New") && len(ident.Name) > 3 {
				return strings.TrimPrefix(ident.Name, "New")
			}
			if retType, exists := p.funcReturnTypes[ident.Name]; exists {
				return retType
			}
		}
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			methodName := sel.Sel.Name
			if strings.Contains(methodName, "Record") {
				if strings.Contains(methodName, "s") || strings.Contains(methodName, "Filter") {
					return "[]Record"
				}
				return "Record"
			}
			// PocketBase getter methods — resolve to their actual Go return types so
			// that variables assigned from them (e.g. td := r.GetString("x")) carry
			// the right type through analyzeValueExpression rather than "interface{}".
			switch methodName {
			case "GetString", "GetDateTime":
				return "string"
			case "GetInt":
				return "int"
			case "GetFloat":
				return "float64"
			case "GetBool":
				return "bool"
			}
			if strings.Contains(methodName, "Find") && strings.Contains(methodName, "s") {
				return "[]interface{}"
			}
			if strings.Contains(methodName, "Find") || strings.Contains(methodName, "Get") {
				return "interface{}"
			}
		}
	case *ast.SliceExpr:
		return "[]interface{}"
	case *ast.IndexExpr:
		// Try to resolve the element type from the base variable's known type.
		// e.g. returns[0] where returns is []float64 → "float64"
		if ident, ok := e.X.(*ast.Ident); ok && handlerInfo != nil {
			if varType, exists := handlerInfo.Variables[ident.Name]; exists {
				if strings.HasPrefix(varType, "[]") {
					return strings.TrimPrefix(varType, "[]")
				}
			}
		}
		return "interface{}"
	}
	return ""
}

// extractTypeName extracts type name from AST expressions
func (p *ASTParser) extractTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			return x.Name + "." + e.Sel.Name
		}
		return e.Sel.Name
	case *ast.StarExpr:
		return p.extractTypeName(e.X)
	case *ast.ArrayType:
		return "[]" + p.extractTypeName(e.Elt)
	case *ast.MapType:
		keyType := p.extractTypeName(e.Key)
		valueType := p.extractTypeName(e.Value)
		return "map[" + keyType + "]" + valueType
	}
	return ""
}
