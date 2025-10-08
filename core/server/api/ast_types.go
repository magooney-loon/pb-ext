package api

import (
	"fmt"
	"go/ast"
	"go/token"
	"sync"
	"time"
)

// =============================================================================
// AST Parsing Types
// =============================================================================

// ASTParser provides robust AST parsing with improved error handling and performance
type ASTParser struct {
	mu                 sync.RWMutex
	fileSet            *token.FileSet
	structs            map[string]*StructInfo
	handlers           map[string]*ASTHandlerInfo
	pocketbasePatterns *PocketBasePatterns
	logger             Logger
}

// FileParseResult stores parsing results with metadata
type FileParseResult struct {
	ModTime            time.Time
	Structs            map[string]*StructInfo
	Handlers           map[string]*ASTHandlerInfo
	Imports            map[string]string
	RouteRegistrations []*RouteRegistration
	Errors             []ParseError
	ParsedAt           time.Time
}

// RouteRegistration represents a discovered route registration in the code
type RouteRegistration struct {
	Method     string          `json:"method"`
	Path       string          `json:"path"`
	HandlerRef string          `json:"handler_ref"`
	CallExpr   *ast.CallExpr   `json:"-"`
	Location   *SourceLocation `json:"location,omitempty"`
}

// ParseError represents a parsing error with context
type ParseError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Context string `json:"context,omitempty"`
}

func (e ParseError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s:%d:%d: %s: %s", e.File, e.Line, e.Column, e.Type, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// StructInfo contains comprehensive struct information
type StructInfo struct {
	Name           string                `json:"name"`
	Package        string                `json:"package"`
	Fields         map[string]*FieldInfo `json:"fields"`
	JSONSchema     *OpenAPISchema        `json:"json_schema"`
	Description    string                `json:"description"`
	Tags           []string              `json:"tags"`
	Embedded       []string              `json:"embedded,omitempty"`
	Methods        []string              `json:"methods,omitempty"`
	Implements     []string              `json:"implements,omitempty"`
	IsGeneric      bool                  `json:"is_generic"`
	TypeParams     []string              `json:"type_params,omitempty"`
	Documentation  *Documentation        `json:"documentation,omitempty"`
	SourceLocation *SourceLocation       `json:"source_location,omitempty"`
}

// FieldInfo contains detailed field information
type FieldInfo struct {
	Name           string                 `json:"name"`
	Type           string                 `json:"type"`
	JSONName       string                 `json:"json_name"`
	JSONOmitEmpty  bool                   `json:"json_omit_empty"`
	Required       bool                   `json:"required"`
	Validation     map[string]string      `json:"validation"`
	Description    string                 `json:"description"`
	Example        interface{}            `json:"example,omitempty"`
	Schema         *OpenAPISchema         `json:"schema"`
	IsPointer      bool                   `json:"is_pointer"`
	IsEmbedded     bool                   `json:"is_embedded"`
	IsExported     bool                   `json:"is_exported"`
	DefaultValue   interface{}            `json:"default_value,omitempty"`
	Constraints    map[string]interface{} `json:"constraints,omitempty"`
	SourceLocation *SourceLocation        `json:"source_location,omitempty"`
}

// ASTHandlerInfo contains comprehensive handler information
type ASTHandlerInfo struct {
	Name           string            `json:"name"`
	Package        string            `json:"package"`
	RequestType    string            `json:"request_type"`
	ResponseType   string            `json:"response_type"`
	RequestSchema  *OpenAPISchema    `json:"request_schema,omitempty"`
	ResponseSchema *OpenAPISchema    `json:"response_schema,omitempty"`
	Parameters     []*ParamInfo      `json:"parameters,omitempty"`
	UsesJSONDecode bool              `json:"uses_json_decode"`
	UsesJSONReturn bool              `json:"uses_json_return"`
	APIDescription string            `json:"api_description"`
	APITags        []string          `json:"api_tags"`
	HTTPMethods    []string          `json:"http_methods"`
	RoutePath      string            `json:"route_path,omitempty"`
	Middleware     []string          `json:"middleware,omitempty"`
	Documentation  *Documentation    `json:"documentation,omitempty"`
	Complexity     int               `json:"complexity"`
	SourceLocation *SourceLocation   `json:"source_location,omitempty"`
	Variables      map[string]string `json:"variables,omitempty"` // Track variable names to types

	// PocketBase-specific fields
	RequiresAuth       bool     `json:"requires_auth"`
	UsesBindBody       bool     `json:"uses_bind_body"`
	UsesRequestInfo    bool     `json:"uses_request_info"`
	DatabaseOperations []string `json:"database_operations,omitempty"`
	AuthType           string   `json:"auth_type,omitempty"`
	Collection         string   `json:"collection,omitempty"`
	UsesEnrichRecords  bool     `json:"uses_enrich_records"`
}

// ParamInfo contains parameter information
type ParamInfo struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Source       string                 `json:"source"` // "path", "query", "body", "header"
	Required     bool                   `json:"required"`
	Description  string                 `json:"description"`
	Example      interface{}            `json:"example,omitempty"`
	Validation   map[string]string      `json:"validation,omitempty"`
	DefaultValue interface{}            `json:"default_value,omitempty"`
	Constraints  map[string]interface{} `json:"constraints,omitempty"`
}

// TypeInfo contains type information for complex types
type TypeInfo struct {
	Name        string                 `json:"name"`
	Kind        string                 `json:"kind"` // "struct", "slice", "map", "interface", etc.
	ElementType *TypeInfo              `json:"element_type,omitempty"`
	KeyType     *TypeInfo              `json:"key_type,omitempty"`
	ValueType   *TypeInfo              `json:"value_type,omitempty"`
	Fields      map[string]*FieldInfo  `json:"fields,omitempty"`
	Schema      *OpenAPISchema         `json:"schema,omitempty"`
	IsPointer   bool                   `json:"is_pointer"`
	IsGeneric   bool                   `json:"is_generic"`
	TypeParams  []string               `json:"type_params,omitempty"`
	Package     string                 `json:"package"`
	Constraints map[string]interface{} `json:"constraints,omitempty"`
}

// Documentation contains extracted documentation information
type Documentation struct {
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Returns     string            `json:"returns,omitempty"`
	Examples    []string          `json:"examples,omitempty"`
	SeeAlso     []string          `json:"see_also,omitempty"`
	Since       string            `json:"since,omitempty"`
	Deprecated  string            `json:"deprecated,omitempty"`
	Authors     []string          `json:"authors,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// SourceLocation contains source code location information
type SourceLocation struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// =============================================================================
// Interface Definitions
// =============================================================================

// ASTParserInterface defines the contract for AST parsing operations
type ASTParserInterface interface {
	ParseFile(filename string) error
	GetAllStructs() map[string]*StructInfo
	GetAllHandlers() map[string]*ASTHandlerInfo
	GetStructByName(name string) (*StructInfo, bool)
	GetHandlerByName(name string) (*ASTHandlerInfo, bool)
	GetParseErrors() []ParseError
	ClearCache()
	EnhanceEndpoint(endpoint *APIEndpoint) error
	GetHandlerDescription(handlerName string) string
	GetHandlerTags(handlerName string) []string
	GetStructsForFinding() map[string]*StructInfo
}

// PocketBasePatterns contains PocketBase-specific parsing patterns
type PocketBasePatterns struct {
	RequestPatterns  map[string]RequestPattern  `json:"request_patterns"`
	ResponsePatterns map[string]ResponsePattern `json:"response_patterns"`
	AuthPatterns     []AuthPattern              `json:"auth_patterns"`
}

// RequestPattern defines a PocketBase request pattern
type RequestPattern struct {
	Method      string `json:"method"`
	StructType  string `json:"struct_type"`
	Description string `json:"description"`
}

// ResponsePattern defines a PocketBase response pattern
type ResponsePattern struct {
	Method      string `json:"method"`
	ReturnType  string `json:"return_type"`
	Description string `json:"description"`
}

// AuthPattern defines a PocketBase authentication pattern
type AuthPattern struct {
	Pattern     string `json:"pattern"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}
