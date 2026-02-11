package api

import (
	"fmt"
	"go/printer"
	"sort"
	"strings"
	"time"
)

// BuildDebugData returns a comprehensive JSON-friendly debug dump of the entire
// API documentation pipeline. This covers:
//   - AST parsing: discovered structs, type aliases, handlers
//   - Handler analysis: variables, variable expressions, request/response types and schemas
//   - Per-version: registered endpoints, component schemas, final OpenAPI output
//
// Served at /api/docs/debug/ast
func (vm *APIVersionManager) BuildDebugData() map[string]any {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	result := map[string]any{
		"generated_at": time.Now().Format(time.RFC3339),
		"versions":     vm.versions,
	}

	// -------------------------------------------------------------------------
	// 1. AST Parser: Structs & Type Aliases (from a fresh parse)
	// -------------------------------------------------------------------------
	parser := NewASTParser()
	allStructs := parser.GetAllStructs()
	allHandlers := parser.GetAllHandlers()

	// Structs
	structNames := sortedKeys(allStructs)
	structsOut := make(map[string]any, len(allStructs))
	for _, name := range structNames {
		info := allStructs[name]
		fields := make([]map[string]any, 0, len(info.Fields))
		for _, f := range info.Fields {
			fieldEntry := map[string]any{
				"name":            f.Name,
				"type":            f.Type,
				"json_name":       f.JSONName,
				"json_omit_empty": f.JSONOmitEmpty,
				"is_pointer":      f.IsPointer,
			}
			fields = append(fields, fieldEntry)
		}
		structsOut[name] = map[string]any{
			"name":        info.Name,
			"field_count": len(info.Fields),
			"fields":      fields,
			"json_schema": info.JSONSchema,
		}
	}

	// Type aliases
	aliases := make(map[string]string, len(parser.typeAliases))
	for k, v := range parser.typeAliases {
		aliases[k] = v
	}

	// Function return types
	funcRetTypes := make(map[string]string, len(parser.funcReturnTypes))
	for k, v := range parser.funcReturnTypes {
		funcRetTypes[k] = v
	}

	// Function body schemas (deep-analyzed helper functions)
	funcBodySchemasOut := make(map[string]any, len(parser.funcBodySchemas))
	for k, v := range parser.funcBodySchemas {
		funcBodySchemasOut[k] = schemaDigest(v)
	}

	// Parsed directories
	parsedDirsList := make([]string, 0, len(parser.parsedDirs))
	for dir := range parser.parsedDirs {
		parsedDirsList = append(parsedDirsList, dir)
	}
	sort.Strings(parsedDirsList)

	result["ast"] = map[string]any{
		"total_structs":     len(allStructs),
		"total_handlers":    len(allHandlers),
		"structs":           structsOut,
		"type_aliases":      aliases,
		"func_return_types": funcRetTypes,
		"func_body_schemas": funcBodySchemasOut,
		"module_path":       parser.modulePath,
		"imported_packages": parsedDirsList,
	}

	// -------------------------------------------------------------------------
	// 2. Handler Analysis
	// -------------------------------------------------------------------------
	handlerNames := sortedKeys(allHandlers)
	handlersOut := make(map[string]any, len(allHandlers))
	for _, name := range handlerNames {
		h := allHandlers[name]

		// Build variable info including expression representations
		vars := make(map[string]any, len(h.Variables))
		for vn, vt := range h.Variables {
			entry := map[string]any{"type": vt}
			if h.VariableExprs != nil {
				if expr, ok := h.VariableExprs[vn]; ok {
					entry["has_expr"] = true
					entry["expr_type"] = fmt.Sprintf("%T", expr)
					// Print a compact representation of the expression
					var sb strings.Builder
					if err := printer.Fprint(&sb, parser.fileSet, expr); err == nil {
						src := sb.String()
						if len(src) > 200 {
							src = src[:200] + "..."
						}
						entry["expr_source"] = src
					}
				}
			}
			vars[vn] = entry
		}

		// Build map additions info
		var mapAdds map[string]any
		if len(h.MapAdditions) > 0 {
			mapAdds = make(map[string]any, len(h.MapAdditions))
			for varName, additions := range h.MapAdditions {
				addList := make([]map[string]any, 0, len(additions))
				for _, add := range additions {
					entry := map[string]any{"key": add.Key}
					var sb strings.Builder
					if err := printer.Fprint(&sb, parser.fileSet, add.Value); err == nil {
						src := sb.String()
						if len(src) > 200 {
							src = src[:200] + "..."
						}
						entry["value_source"] = src
					}
					entry["value_type"] = fmt.Sprintf("%T", add.Value)
					addList = append(addList, entry)
				}
				mapAdds[varName] = addList
			}
		}

		// Build slice append expressions info
		var sliceAppends map[string]any
		if len(h.SliceAppendExprs) > 0 {
			sliceAppends = make(map[string]any, len(h.SliceAppendExprs))
			for varName, appendExpr := range h.SliceAppendExprs {
				entry := map[string]any{"expr_type": fmt.Sprintf("%T", appendExpr)}
				var sb strings.Builder
				if err := printer.Fprint(&sb, parser.fileSet, appendExpr); err == nil {
					src := sb.String()
					if len(src) > 200 {
						src = src[:200] + "..."
					}
					entry["expr_source"] = src
				}
				sliceAppends[varName] = entry
			}
		}

		handlersOut[name] = map[string]any{
			"name":             h.Name,
			"api_description":  h.APIDescription,
			"api_tags":         h.APITags,
			"request_type":     h.RequestType,
			"response_type":    h.ResponseType,
			"request_schema":   h.RequestSchema,
			"response_schema":  h.ResponseSchema,
			"parameters":       h.Parameters,
			"variables":        vars,
			"map_additions":    mapAdds,
			"slice_appends":    sliceAppends,
			"requires_auth":    h.RequiresAuth,
			"auth_type":        h.AuthType,
			"uses_bind_body":   h.UsesBindBody,
			"uses_json_decode": h.UsesJSONDecode,
		}
	}
	result["handlers"] = handlersOut

	// -------------------------------------------------------------------------
	// 3. Per-Version: Registry, Components, Final OpenAPI
	// -------------------------------------------------------------------------
	versionsOut := make(map[string]any, len(vm.versions))
	for _, version := range vm.versions {
		registry := vm.registries[version]
		if registry == nil {
			versionsOut[version] = map[string]any{"error": "no registry"}
			continue
		}

		// Registered endpoints (internal representation)
		endpoints := registry.GetEndpointsInternal()
		epList := make([]map[string]any, 0, len(endpoints))
		for _, ep := range endpoints {
			epEntry := map[string]any{
				"method": ep.Method,
				"path":   ep.Path,
			}
			if ep.Request != nil {
				epEntry["request"] = schemaDigest(ep.Request)
			}
			if ep.Response != nil {
				epEntry["response"] = schemaDigest(ep.Response)
			}
			epList = append(epList, epEntry)
		}

		// Full OpenAPI docs with components
		docs := registry.GetDocsWithComponents()

		// Component schemas digest
		componentDigest := map[string]any{}
		if docs.Components != nil && len(docs.Components.Schemas) > 0 {
			for sName, schema := range docs.Components.Schemas {
				componentDigest[sName] = schemaDigest(schema)
			}
		}

		// Final paths digest
		pathsDigest := map[string]any{}
		pathNames := make([]string, 0, len(docs.Paths))
		for p := range docs.Paths {
			pathNames = append(pathNames, p)
		}
		sort.Strings(pathNames)

		for _, path := range pathNames {
			pathItem := docs.Paths[path]
			ops := map[string]any{}
			for _, entry := range []struct {
				method string
				op     *OpenAPIOperation
			}{
				{"GET", pathItem.Get},
				{"POST", pathItem.Post},
				{"PUT", pathItem.Put},
				{"DELETE", pathItem.Delete},
				{"PATCH", pathItem.Patch},
			} {
				if entry.op == nil {
					continue
				}
				opData := map[string]any{
					"summary": entry.op.Summary,
					"tags":    entry.op.Tags,
				}
				// Request body
				if entry.op.RequestBody != nil {
					reqBody := map[string]any{}
					for ct, mt := range entry.op.RequestBody.Content {
						if mt.Schema != nil {
							reqBody[ct] = schemaDigest(mt.Schema)
						}
					}
					opData["request_body"] = reqBody
				}
				// Response 200
				if resp, ok := entry.op.Responses["200"]; ok && resp.Content != nil {
					respBody := map[string]any{}
					for ct, mt := range resp.Content {
						if mt.Schema != nil {
							respBody[ct] = schemaDigest(mt.Schema)
						}
					}
					opData["response_200"] = respBody
				}
				ops[entry.method] = opData
			}
			pathsDigest[path] = ops
		}

		// Include the full OpenAPI JSON too for copy-paste debugging
		versionsOut[version] = map[string]any{
			"endpoint_count":    len(endpoints),
			"endpoints":         epList,
			"component_schemas": componentDigest,
			"paths":             pathsDigest,
			"full_openapi":      docs,
		}
	}
	result["versions_detail"] = versionsOut

	return result
}

// schemaDigest returns a compact JSON-friendly representation of an OpenAPISchema
func schemaDigest(s *OpenAPISchema) map[string]any {
	if s == nil {
		return nil
	}
	d := map[string]any{}

	if s.Ref != "" {
		d["$ref"] = s.Ref
		return d
	}

	d["type"] = s.Type
	if s.Format != "" {
		d["format"] = s.Format
	}
	if s.Description != "" {
		d["description"] = s.Description
	}

	if len(s.Properties) > 0 {
		props := map[string]any{}
		for pName, pSchema := range s.Properties {
			props[pName] = schemaDigest(pSchema)
		}
		d["properties"] = props
		d["prop_count"] = len(s.Properties)
	}

	if s.Items != nil {
		d["items"] = schemaDigest(s.Items)
	}

	if s.AdditionalProperties != nil {
		switch v := s.AdditionalProperties.(type) {
		case bool:
			d["additionalProperties"] = v
		case *OpenAPISchema:
			d["additionalProperties"] = schemaDigest(v)
		}
	}

	if len(s.Required) > 0 {
		d["required"] = s.Required
	}

	if len(s.Enum) > 0 {
		d["enum"] = s.Enum
	}

	return d
}

// sortedKeys returns sorted keys from a map with string keys
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
