package api

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// =============================================================================
// Schema Conversion Utilities
// =============================================================================

// ConvertGoTypeToOpenAPISchema converts a Go type string to an OpenAPI schema
func ConvertGoTypeToOpenAPISchema(goType string, fieldInfo *FieldInfo) *OpenAPISchema {
	schema := &OpenAPISchema{}

	// Handle pointer types
	if strings.HasPrefix(goType, "*") {
		goType = strings.TrimPrefix(goType, "*")
		schema.Nullable = boolPtr(true)
	}

	// Handle array/slice types
	if strings.HasPrefix(goType, "[]") {
		elementType := strings.TrimPrefix(goType, "[]")
		schema.Type = "array"
		schema.Items = ConvertGoTypeToOpenAPISchema(elementType, nil)
		return schema
	}

	// Handle map types
	if strings.HasPrefix(goType, "map[") {
		schema.Type = "object"
		schema.AdditionalProperties = true
		// Could be enhanced to handle typed maps like map[string]SomeStruct
		return schema
	}

	// Convert basic Go types
	switch goType {
	case "string":
		schema.Type = "string"
	case "int", "int8", "int16", "int32", "int64":
		schema.Type = "integer"
		if strings.Contains(goType, "64") {
			schema.Format = "int64"
		} else {
			schema.Format = "int32"
		}
	case "uint", "uint8", "uint16", "uint32", "uint64":
		schema.Type = "integer"
		schema.Minimum = floatPtr(0)
		if strings.Contains(goType, "64") {
			schema.Format = "int64"
		} else {
			schema.Format = "int32"
		}
	case "float32":
		schema.Type = "number"
		schema.Format = "float"
	case "float64":
		schema.Type = "number"
		schema.Format = "double"
	case "bool":
		schema.Type = "boolean"
	case "time.Time":
		schema.Type = "string"
		schema.Format = "date-time"
	case "interface{}":
		// Any type - no constraints
		schema = &OpenAPISchema{}
	default:
		// Assume it's a custom struct type
		schema.Type = "object"
		schema.Extensions = map[string]interface{}{
			"x-go-type": goType,
		}
	}

	// Apply field-specific information if available
	if fieldInfo != nil {
		applyFieldInfoToSchema(schema, fieldInfo)
	}

	return schema
}

// ConvertStructInfoToOpenAPISchema converts a StructInfo to an OpenAPI object schema
func ConvertStructInfoToOpenAPISchema(structInfo *StructInfo) *OpenAPISchema {
	schema := &OpenAPISchema{
		Type:       "object",
		Title:      structInfo.Name,
		Properties: make(map[string]*OpenAPISchema),
	}

	if structInfo.Description != "" {
		schema.Description = structInfo.Description
	}

	var required []string

	// Convert fields
	for fieldName, fieldInfo := range structInfo.Fields {
		if !fieldInfo.IsExported {
			continue // Skip unexported fields
		}

		jsonName := fieldInfo.JSONName
		if jsonName == "" {
			jsonName = fieldName
		}

		// Skip fields with json:"-"
		if jsonName == "-" {
			continue
		}

		fieldSchema := ConvertGoTypeToOpenAPISchema(fieldInfo.Type, fieldInfo)

		if fieldInfo.Description != "" {
			fieldSchema.Description = fieldInfo.Description
		}

		if fieldInfo.Example != nil {
			fieldSchema.Example = fieldInfo.Example
		}

		schema.Properties[jsonName] = fieldSchema

		// Add to required if not omitempty and not a pointer
		if fieldInfo.Required || (!fieldInfo.JSONOmitEmpty && !fieldInfo.IsPointer) {
			required = append(required, jsonName)
		}
	}

	if len(required) > 0 {
		schema.Required = required
	}

	// Add extensions for Go-specific metadata
	schema.Extensions = map[string]interface{}{
		"x-go-package": structInfo.Package,
		"x-go-name":    structInfo.Name,
	}

	if len(structInfo.Tags) > 0 {
		schema.Extensions["x-tags"] = structInfo.Tags
	}

	return schema
}

// ConvertParamInfoToOpenAPIParameter converts a ParamInfo to an OpenAPI parameter
func ConvertParamInfoToOpenAPIParameter(paramInfo *ParamInfo) *OpenAPIParameter {
	param := &OpenAPIParameter{
		Name:        paramInfo.Name,
		In:          paramInfo.Source,
		Description: paramInfo.Description,
		Required:    boolPtr(paramInfo.Required),
		Schema:      ConvertGoTypeToOpenAPISchema(paramInfo.Type, nil),
	}

	if paramInfo.Example != nil {
		param.Example = paramInfo.Example
	}

	if paramInfo.DefaultValue != nil {
		param.Schema.Default = paramInfo.DefaultValue
	}

	// Apply validation constraints
	applyValidationToParameter(param, paramInfo.Validation)

	return param
}

// ConvertHandlerInfoToOpenAPIOperation converts an ASTHandlerInfo to an OpenAPI operation
func ConvertHandlerInfoToOpenAPIOperation(handlerInfo *ASTHandlerInfo) *OpenAPIOperation {
	operation := &OpenAPIOperation{
		Summary:     handlerInfo.APIDescription,
		Description: handlerInfo.APIDescription,
		Tags:        handlerInfo.APITags,
	}

	// Generate operation ID from handler name
	if handlerInfo.Name != "" {
		operation.OperationId = generateOperationId(handlerInfo.Name)
	}

	// Convert parameters
	if len(handlerInfo.Parameters) > 0 {
		operation.Parameters = make([]*OpenAPIParameter, 0, len(handlerInfo.Parameters))
		for _, paramInfo := range handlerInfo.Parameters {
			param := ConvertParamInfoToOpenAPIParameter(paramInfo)
			operation.Parameters = append(operation.Parameters, param)
		}
	}

	// Convert request body if available
	if handlerInfo.RequestSchema != nil && handlerInfo.UsesJSONDecode {
		operation.RequestBody = &OpenAPIRequestBody{
			Description: "Request body",
			Required:    boolPtr(true),
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: handlerInfo.RequestSchema,
				},
			},
		}
	}

	// Convert responses
	operation.Responses = make(map[string]*OpenAPIResponse)

	if handlerInfo.ResponseSchema != nil {
		operation.Responses["200"] = &OpenAPIResponse{
			Description: "Successful response",
			Content: map[string]*OpenAPIMediaType{
				"application/json": {
					Schema: handlerInfo.ResponseSchema,
				},
			},
		}
	} else {
		// Default success response
		operation.Responses["200"] = &OpenAPIResponse{
			Description: "Successful response",
		}
	}

	// Add error responses
	addCommonErrorResponses(operation.Responses)

	// Add security if required
	if handlerInfo.RequiresAuth {
		operation.Security = []map[string][]string{
			{"bearerAuth": {}},
		}
	}

	return operation
}

// =============================================================================
// Validation Conversion
// =============================================================================

// applyFieldInfoToSchema applies field-specific information to a schema
func applyFieldInfoToSchema(schema *OpenAPISchema, fieldInfo *FieldInfo) {
	if fieldInfo.Description != "" {
		schema.Description = fieldInfo.Description
	}

	if fieldInfo.Example != nil {
		schema.Example = fieldInfo.Example
	}

	if fieldInfo.DefaultValue != nil {
		schema.Default = fieldInfo.DefaultValue
	}

	// Apply validation rules
	applyValidationToSchema(schema, fieldInfo.Validation)
}

// applyValidationToSchema converts validation tags to OpenAPI validation constraints
func applyValidationToSchema(schema *OpenAPISchema, validation map[string]string) {
	for tag, value := range validation {
		switch tag {
		case "required":
			// Required is handled at the parent level
		case "min":
			if min, err := strconv.ParseFloat(value, 64); err == nil {
				schema.Minimum = &min
			}
		case "max":
			if max, err := strconv.ParseFloat(value, 64); err == nil {
				schema.Maximum = &max
			}
		case "minLength":
			if minLen, err := strconv.Atoi(value); err == nil {
				schema.MinLength = &minLen
			}
		case "maxLength":
			if maxLen, err := strconv.Atoi(value); err == nil {
				schema.MaxLength = &maxLen
			}
		case "pattern":
			schema.Pattern = value
		case "email":
			schema.Format = "email"
		case "uuid":
			schema.Format = "uuid"
		case "url":
			schema.Format = "uri"
		case "enum":
			// Parse enum values
			enumValues := strings.Split(value, ",")
			schema.Enum = make([]interface{}, len(enumValues))
			for i, v := range enumValues {
				schema.Enum[i] = strings.TrimSpace(v)
			}
		case "multipleOf":
			if mult, err := strconv.ParseFloat(value, 64); err == nil {
				schema.MultipleOf = &mult
			}
		}
	}
}

// applyValidationToParameter applies validation to an OpenAPI parameter
func applyValidationToParameter(param *OpenAPIParameter, validation map[string]string) {
	applyValidationToSchema(param.Schema, validation)
}

// =============================================================================
// Response Generation
// =============================================================================

// addCommonErrorResponses adds common error responses to an operation
func addCommonErrorResponses(responses map[string]*OpenAPIResponse) {
	responses["400"] = &OpenAPIResponse{
		Description: "Bad Request",
		Content: map[string]*OpenAPIMediaType{
			"application/json": {
				Schema: ErrorResponseSchema,
			},
		},
	}

	responses["401"] = &OpenAPIResponse{
		Description: "Unauthorized",
		Content: map[string]*OpenAPIMediaType{
			"application/json": {
				Schema: ErrorResponseSchema,
			},
		},
	}

	responses["403"] = &OpenAPIResponse{
		Description: "Forbidden",
		Content: map[string]*OpenAPIMediaType{
			"application/json": {
				Schema: ErrorResponseSchema,
			},
		},
	}

	responses["404"] = &OpenAPIResponse{
		Description: "Not Found",
		Content: map[string]*OpenAPIMediaType{
			"application/json": {
				Schema: ErrorResponseSchema,
			},
		},
	}

	responses["500"] = &OpenAPIResponse{
		Description: "Internal Server Error",
		Content: map[string]*OpenAPIMediaType{
			"application/json": {
				Schema: ErrorResponseSchema,
			},
		},
	}
}

// =============================================================================
// PocketBase-Specific Conversions
// =============================================================================

// ConvertPocketBaseRecordSchema creates an OpenAPI schema for a PocketBase record
func ConvertPocketBaseRecordSchema(collection string, fields map[string]*FieldInfo) *OpenAPISchema {
	schema := &OpenAPISchema{
		Type:       "object",
		Title:      fmt.Sprintf("%sRecord", strings.Title(collection)),
		Properties: make(map[string]*OpenAPISchema),
		AllOf: []*OpenAPISchema{
			{Ref: "#/components/schemas/PocketBaseRecord"},
		},
	}

	// Add custom fields
	for fieldName, fieldInfo := range fields {
		if fieldInfo.IsExported {
			jsonName := fieldInfo.JSONName
			if jsonName == "" {
				jsonName = fieldName
			}

			schema.Properties[jsonName] = ConvertGoTypeToOpenAPISchema(fieldInfo.Type, fieldInfo)
		}
	}

	schema.Extensions = map[string]interface{}{
		"x-pocketbase-collection": collection,
	}

	return schema
}

// =============================================================================
// Utility Functions
// =============================================================================

// generateOperationId generates a valid OpenAPI operation ID from a handler name
func generateOperationId(handlerName string) string {
	// Remove package path
	parts := strings.Split(handlerName, ".")
	if len(parts) > 1 {
		handlerName = parts[len(parts)-1]
	}

	// Remove common suffixes
	handlerName = strings.TrimSuffix(handlerName, "Handler")
	handlerName = strings.TrimSuffix(handlerName, "Handle")

	// Convert to camelCase
	return toCamelCase(handlerName)
}

// toCamelCase converts a string to camelCase
func toCamelCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})

	if len(parts) == 0 {
		return s
	}

	result := strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			result += strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		}
	}

	return result
}

// boolPtr returns a pointer to a boolean value
func boolPtr(b bool) *bool {
	return &b
}

// intPtr returns a pointer to an integer value
func intPtr(i int) *int {
	return &i
}

// floatPtr returns a pointer to a float64 value
func floatPtr(f float64) *float64 {
	return &f
}

// stringPtr returns a pointer to a string value
func stringPtr(s string) *string {
	return &s
}

// =============================================================================
// Validation Tag Parsing
// =============================================================================

// ParseValidationTags parses struct tag validation rules
func ParseValidationTags(tag string) map[string]string {
	validation := make(map[string]string)

	if tag == "" {
		return validation
	}

	// Split by comma and parse each validation rule
	rules := strings.Split(tag, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		// Handle rules with values (e.g., "min=1", "max=100")
		if strings.Contains(rule, "=") {
			parts := strings.SplitN(rule, "=", 2)
			if len(parts) == 2 {
				validation[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		} else {
			// Handle boolean rules (e.g., "required", "email")
			validation[rule] = "true"
		}
	}

	return validation
}

// ExtractValidationFromStructField extracts validation information from a struct field
func ExtractValidationFromStructField(field reflect.StructField) map[string]string {
	validation := make(map[string]string)

	// Parse validate tag
	if validateTag := field.Tag.Get("validate"); validateTag != "" {
		for k, v := range ParseValidationTags(validateTag) {
			validation[k] = v
		}
	}

	// Parse json tag for omitempty
	if jsonTag := field.Tag.Get("json"); jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		for _, part := range parts[1:] { // Skip the name part
			part = strings.TrimSpace(part)
			if part == "omitempty" {
				validation["omitempty"] = "true"
			}
		}
	}

	return validation
}
