package api

// PocketBaseFieldTypeMapping contains mappings from PocketBase field types to JSON Schema types
type PocketBaseFieldTypeMapping struct {
	// Core field type mappings
	fieldTypeMap map[string]func(fieldConfig map[string]interface{}) map[string]interface{}
}

// NewPocketBaseFieldTypeMapping creates a new field type mapping instance
func NewPocketBaseFieldTypeMapping() *PocketBaseFieldTypeMapping {
	mapping := &PocketBaseFieldTypeMapping{
		fieldTypeMap: make(map[string]func(fieldConfig map[string]interface{}) map[string]interface{}),
	}

	// Initialize all field type mappings
	mapping.initializeFieldMappings()
	return mapping
}

// GetSchemaForField returns the JSON schema for a given PocketBase field type
func (m *PocketBaseFieldTypeMapping) GetSchemaForField(fieldType string, fieldConfig map[string]interface{}) map[string]interface{} {
	if mapper, exists := m.fieldTypeMap[fieldType]; exists {
		schema := mapper(fieldConfig)
		return schema
	}
	// No fallback - return nil if no explicit mapping found
	return nil
}

// initializeFieldMappings sets up all the field type mappings
func (m *PocketBaseFieldTypeMapping) initializeFieldMappings() {

	// BoolField - boolean type
	m.fieldTypeMap["BoolField"] = func(config map[string]interface{}) map[string]interface{} {
		schema := map[string]interface{}{
			"type": "boolean",
		}

		if required, ok := config["required"].(bool); ok && required {
			schema["required"] = true
		}

		return schema
	}

	// NumberField - number or integer type
	m.fieldTypeMap["NumberField"] = func(config map[string]interface{}) map[string]interface{} {
		schema := map[string]interface{}{}

		// Check if OnlyInt is true
		if onlyInt, ok := config["onlyInt"].(bool); ok && onlyInt {
			schema["type"] = "integer"
		} else {
			schema["type"] = "number"
		}

		// Add min/max constraints
		if min, ok := config["min"].(float64); ok {
			schema["minimum"] = min
		}
		if max, ok := config["max"].(float64); ok {
			schema["maximum"] = max
		}

		return schema
	}

	// TextField - string type
	m.fieldTypeMap["TextField"] = func(config map[string]interface{}) map[string]interface{} {
		schema := map[string]interface{}{
			"type": "string",
		}

		// Add length constraints
		if min, ok := config["min"].(int); ok && min > 0 {
			schema["minLength"] = min
		}
		if max, ok := config["max"].(int); ok && max > 0 {
			schema["maxLength"] = max
		}

		// Add pattern if specified
		if pattern, ok := config["pattern"].(string); ok && pattern != "" {
			schema["pattern"] = pattern
		}

		return schema
	}

	// EmailField - string with email format
	m.fieldTypeMap["EmailField"] = func(config map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type":   "string",
			"format": "email",
		}
	}

	// URLField - string with URI format
	m.fieldTypeMap["URLField"] = func(config map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type":   "string",
			"format": "uri",
		}
	}

	// EditorField - string type (rich text)
	m.fieldTypeMap["EditorField"] = func(config map[string]interface{}) map[string]interface{} {
		schema := map[string]interface{}{
			"type": "string",
		}

		// Add max size constraint if specified
		if maxSize, ok := config["maxSize"].(int64); ok && maxSize > 0 {
			schema["maxLength"] = maxSize
		}

		return schema
	}

	// DateField - string with date-time format
	m.fieldTypeMap["DateField"] = func(config map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type":   "string",
			"format": "date-time",
		}
	}

	// AutodateField - string with date-time format
	m.fieldTypeMap["AutodateField"] = func(config map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type":   "string",
			"format": "date-time",
		}
	}

	// SelectField - string or array of strings
	m.fieldTypeMap["SelectField"] = func(config map[string]interface{}) map[string]interface{} {
		maxSelect := 1
		if ms, ok := config["maxSelect"].(int); ok {
			maxSelect = ms
		}

		// Get enum values
		var enumValues []string
		if values, ok := config["values"].([]string); ok {
			enumValues = values
		}

		if maxSelect > 1 {
			// Multiple select - array of strings
			schema := map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"maxItems": maxSelect,
			}

			if len(enumValues) > 0 {
				schema["items"].(map[string]interface{})["enum"] = enumValues
			}

			return schema
		} else {
			// Single select - string
			schema := map[string]interface{}{
				"type": "string",
			}

			if len(enumValues) > 0 {
				schema["enum"] = enumValues
			}

			return schema
		}
	}

	// FileField - string or array of strings (file URLs)
	m.fieldTypeMap["FileField"] = func(config map[string]interface{}) map[string]interface{} {
		maxSelect := 1
		if ms, ok := config["maxSelect"].(int); ok {
			maxSelect = ms
		}

		if maxSelect > 1 {
			// Multiple files - array of strings
			schema := map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type":   "string",
					"format": "uri",
				},
				"maxItems": maxSelect,
			}

			return schema
		} else {
			// Single file - string
			return map[string]interface{}{
				"type":   "string",
				"format": "uri",
			}
		}
	}

	// RelationField - string or array of strings (record IDs)
	m.fieldTypeMap["RelationField"] = func(config map[string]interface{}) map[string]interface{} {
		maxSelect := 1
		if ms, ok := config["maxSelect"].(int); ok {
			maxSelect = ms
		}

		if maxSelect > 1 {
			// Multiple relations - array of strings
			schema := map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"maxItems": maxSelect,
			}

			// Add min select constraint
			if minSelect, ok := config["minSelect"].(int); ok && minSelect > 0 {
				schema["minItems"] = minSelect
			}

			return schema
		} else {
			// Single relation - string
			return map[string]interface{}{
				"type": "string",
			}
		}
	}

	// JSONField - object with additional properties
	m.fieldTypeMap["JSONField"] = func(config map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type":                 "object",
			"additionalProperties": true,
		}
	}

	// GeoPointField - object with lat/lng coordinates
	m.fieldTypeMap["GeoPointField"] = func(config map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"lat": map[string]interface{}{
					"type":    "number",
					"minimum": -90,
					"maximum": 90,
				},
				"lng": map[string]interface{}{
					"type":    "number",
					"minimum": -180,
					"maximum": 180,
				},
			},
			"required": []string{"lat", "lng"},
		}
	}
}

// GetRecordGetterMethodMapping returns the mapping for record getter methods
func GetRecordGetterMethodMapping() map[string]map[string]interface{} {
	mapping := map[string]map[string]interface{}{
		// Basic getter methods
		"GetBool": {
			"type": "boolean",
		},
		"GetInt": {
			"type": "integer",
		},
		"GetInt64": {
			"type": "integer",
		},
		"GetUint": {
			"type":    "integer",
			"minimum": 0,
		},
		"GetUint64": {
			"type":    "integer",
			"minimum": 0,
		},
		"GetFloat": {
			"type": "number",
		},
		"GetFloat64": {
			"type": "number",
		},
		"GetString": {
			"type": "string",
		},
		"GetDateTime": {
			"type":   "string",
			"format": "date-time",
		},
		"GetStringSlice": {
			"type": "array",
			"items": map[string]interface{}{
				"type": "string",
			},
		},
		"GetBytes": {
			"type":   "string",
			"format": "byte",
		},
		"Get": {
			"type": "string",
		},

		// Specialized getter methods
		"GetTime": {
			"type":   "string",
			"format": "date-time",
		},
		"GetJSON": {
			"type":                 "object",
			"additionalProperties": true,
		},
	}

	return mapping
}

// IsSystemField checks if a field name represents a system-generated field - exact matches only
func IsSystemField(fieldName string) bool {
	systemFields := []string{
		"id",
		"created_at",
		"updated_at",
	}

	// Only exact matches - no case conversion or pattern matching
	for _, sysField := range systemFields {
		if fieldName == sysField {
			return true
		}
	}

	return false
}

// GetPocketBaseSystemFieldSchema returns schema for common PocketBase system fields
func GetPocketBaseSystemFieldSchema() map[string]map[string]interface{} {
	return map[string]map[string]interface{}{
		"id": {
			"type":        "string",
			"description": "Record ID",
			"readOnly":    true,
		},
		"created": {
			"type":        "string",
			"format":      "date-time",
			"description": "Created timestamp",
			"readOnly":    true,
		},
		"updated": {
			"type":        "string",
			"format":      "date-time",
			"description": "Updated timestamp",
			"readOnly":    true,
		},
		"collectionId": {
			"type":        "string",
			"description": "Collection ID",
			"readOnly":    true,
		},
		"collectionName": {
			"type":        "string",
			"description": "Collection name",
			"readOnly":    true,
		},
		"expand": {
			"type":                 "object",
			"additionalProperties": true,
			"description":          "Expanded relations",
			"readOnly":             true,
		},
	}
}
