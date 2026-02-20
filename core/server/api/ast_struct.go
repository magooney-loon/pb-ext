package api

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

// =============================================================================
// First Pass: Struct Extraction
// =============================================================================

// extractStructs extracts struct definitions that might be used for requests/responses.
// Uses a two-pass approach: first pass registers all structs and type aliases,
// second pass generates schemas once all struct names are known.
func (p *ASTParser) extractStructs(file *ast.File) {
	// First pass: register all structs with their fields (without JSONSchema) and type aliases
	ast.Inspect(file, func(n ast.Node) bool {
		if genDecl, ok := n.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						structInfo := p.parseStruct(typeSpec, structType, false)
						p.structs[structInfo.Name] = structInfo
					} else {
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

// parseStruct parses a struct definition.
// generateSchema: if false, only extracts fields without generating JSONSchema.
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

			if field.Tag != nil {
				p.parseJSONTag(field.Tag.Value, fieldInfo)
			}

			structInfo.Fields[fieldInfo.Name] = fieldInfo
		}
	}

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
		for _, opt := range parts[1:] {
			if strings.TrimSpace(opt) == "omitempty" {
				fieldInfo.JSONOmitEmpty = true
				break
			}
		}
	}
}

// extractTag extracts a specific tag value from a struct tag string
func (p *ASTParser) extractTag(tagValue, tagName string) string {
	re := regexp.MustCompile(tagName + `:"([^"]*)"`)
	matches := re.FindStringSubmatch(tagValue)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// generateStructSchema generates OpenAPI schema for a struct.
// It flattens embedded struct fields into the parent schema (Go's promotion semantics).
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

// generateFieldSchema generates OpenAPI schema for a field type.
// Uses inline=false to generate $ref for nested struct types.
func (p *ASTParser) generateFieldSchema(fieldType string) *OpenAPISchema {
	return p.generateSchemaFromType(fieldType, false)
}

// deepCopySchema creates a deep copy of an OpenAPISchema to avoid modifying the original
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

	if src.Properties != nil {
		dst.Properties = make(map[string]*OpenAPISchema)
		for k, v := range src.Properties {
			dst.Properties[k] = p.deepCopySchema(v)
		}
	}

	dst.AdditionalProperties = src.AdditionalProperties

	if src.Items != nil {
		dst.Items = p.deepCopySchema(src.Items)
	}

	if src.Not != nil {
		dst.Not = p.deepCopySchema(src.Not)
	}

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

	if src.ExternalDocs != nil {
		dst.ExternalDocs = &OpenAPIExternalDocs{
			Description: src.ExternalDocs.Description,
			URL:         src.ExternalDocs.URL,
		}
	}

	return dst
}
