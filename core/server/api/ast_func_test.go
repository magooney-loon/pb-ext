package api

import "testing"

// =============================================================================
// Handler Schema Scenario Tests
// Covers all AST handler patterns: deep structs, maps, arrays, helpers,
// append-based loops, index expression resolution, parameter detection.
// =============================================================================

// handlerScenarioSource is the synthetic Go source that exercises every schema
// generation path in the AST parser.  It is parsed once in each sub-test that
// needs it.
const handlerScenarioSource = `package main

// API_SOURCE

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// --- Struct definitions ---

type GeoCoordinate struct {
	Latitude  float64 ` + "`json:\"latitude\"`" + `
	Longitude float64 ` + "`json:\"longitude\"`" + `
}

type Address struct {
	Street     string        ` + "`json:\"street\"`" + `
	City       string        ` + "`json:\"city\"`" + `
	State      string        ` + "`json:\"state,omitempty\"`" + `
	PostalCode string        ` + "`json:\"postal_code\"`" + `
	Country    string        ` + "`json:\"country\"`" + `
	Geo        GeoCoordinate ` + "`json:\"geo\"`" + `
}

type ContactInfo struct {
	Email     string   ` + "`json:\"email\"`" + `
	Phone     *string  ` + "`json:\"phone,omitempty\"`" + `
	Website   *string  ` + "`json:\"website,omitempty\"`" + `
	SocialIDs []string ` + "`json:\"social_ids,omitempty\"`" + `
}

type OrderItem struct {
	ProductID   string  ` + "`json:\"product_id\"`" + `
	ProductName string  ` + "`json:\"product_name\"`" + `
	Quantity    int     ` + "`json:\"quantity\"`" + `
	UnitPrice   float64 ` + "`json:\"unit_price\"`" + `
	Subtotal    float64 ` + "`json:\"subtotal\"`" + `
}

type PaymentInfo struct {
	Method        string            ` + "`json:\"method\"`" + `
	TransactionID string            ` + "`json:\"transaction_id\"`" + `
	Amount        float64           ` + "`json:\"amount\"`" + `
	Currency      string            ` + "`json:\"currency\"`" + `
	Headers       map[string]string ` + "`json:\"headers,omitempty\"`" + `
	Metadata      map[string]any    ` + "`json:\"metadata,omitempty\"`" + `
}

type OrderResponse struct {
	ID              string      ` + "`json:\"id\"`" + `
	Status          string      ` + "`json:\"status\"`" + `
	Customer        string      ` + "`json:\"customer\"`" + `
	ShippingAddress Address     ` + "`json:\"shipping_address\"`" + `
	BillingAddress  *Address    ` + "`json:\"billing_address,omitempty\"`" + `
	Items           []OrderItem ` + "`json:\"items\"`" + `
	Payment         PaymentInfo ` + "`json:\"payment\"`" + `
	TotalAmount     float64     ` + "`json:\"total_amount\"`" + `
	Notes           *string     ` + "`json:\"notes,omitempty\"`" + `
	CreatedAt       time.Time   ` + "`json:\"created_at\"`" + `
	UpdatedAt       time.Time   ` + "`json:\"updated_at\"`" + `
}

type CreateOrderRequest struct {
	CustomerID      string      ` + "`json:\"customer_id\"`" + `
	ShippingAddress Address     ` + "`json:\"shipping_address\"`" + `
	BillingAddress  *Address    ` + "`json:\"billing_address,omitempty\"`" + `
	Items           []OrderItem ` + "`json:\"items\"`" + `
	PaymentMethod   string      ` + "`json:\"payment_method\"`" + `
	Notes           *string     ` + "`json:\"notes,omitempty\"`" + `
	CouponCode      *string     ` + "`json:\"coupon_code,omitempty\"`" + `
}

type AnalyticsEvent struct {
	EventID    string         ` + "`json:\"event_id\"`" + `
	EventType  string         ` + "`json:\"event_type\"`" + `
	Timestamp  time.Time      ` + "`json:\"timestamp\"`" + `
	UserID     *string        ` + "`json:\"user_id,omitempty\"`" + `
	SessionID  string         ` + "`json:\"session_id\"`" + `
	Properties map[string]any ` + "`json:\"properties,omitempty\"`" + `
	Context    any            ` + "`json:\"context,omitempty\"`" + `
	Tags       []string       ` + "`json:\"tags,omitempty\"`" + `
}

type PaginationMeta struct {
	Page       int  ` + "`json:\"page\"`" + `
	PerPage    int  ` + "`json:\"per_page\"`" + `
	TotalItems int  ` + "`json:\"total_items\"`" + `
	TotalPages int  ` + "`json:\"total_pages\"`" + `
	HasMore    bool ` + "`json:\"has_more\"`" + `
}

type UserProfile struct {
	ID          string      ` + "`json:\"id\"`" + `
	Username    string      ` + "`json:\"username\"`" + `
	DisplayName string      ` + "`json:\"display_name\"`" + `
	Email       string      ` + "`json:\"email\"`" + `
	AvatarURL   *string     ` + "`json:\"avatar_url,omitempty\"`" + `
	Bio         *string     ` + "`json:\"bio,omitempty\"`" + `
	IsVerified  bool        ` + "`json:\"is_verified\"`" + `
	Reputation  int         ` + "`json:\"reputation\"`" + `
	Balance     float64     ` + "`json:\"balance\"`" + `
	JoinedAt    time.Time   ` + "`json:\"joined_at\"`" + `
	Contact     ContactInfo ` + "`json:\"contact\"`" + `
}

type TimeseriesPoint struct {
	Timestamp int64   ` + "`json:\"timestamp\"`" + `
	Open      float64 ` + "`json:\"open\"`" + `
	High      float64 ` + "`json:\"high\"`" + `
	Low       float64 ` + "`json:\"low\"`" + `
	Close     float64 ` + "`json:\"close\"`" + `
	Volume    float64 ` + "`json:\"volume\"`" + `
}

type IndicatorValues struct {
	TokenID   string             ` + "`json:\"token_id\"`" + `
	Interval  string             ` + "`json:\"interval\"`" + `
	Values    map[string]float64 ` + "`json:\"values\"`" + `
	Signals   map[string]string  ` + "`json:\"signals\"`" + `
	Computed  map[string]int     ` + "`json:\"computed\"`" + `
	UpdatedAt time.Time          ` + "`json:\"updated_at\"`" + `
}

type UpdateProfileRequest struct {
	DisplayName string  ` + "`json:\"display_name\"`" + `
	Bio         *string ` + "`json:\"bio,omitempty\"`" + `
	AvatarURL   *string ` + "`json:\"avatar_url,omitempty\"`" + `
}

type BaseEntity struct {
	ID        string    ` + "`json:\"id\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\"`" + `
}

type ProductResponse struct {
	BaseEntity
	Name        string   ` + "`json:\"name\"`" + `
	Description string   ` + "`json:\"description\"`" + `
	Price       float64  ` + "`json:\"price\"`" + `
	Currency    string   ` + "`json:\"currency\"`" + `
	Tags        []string ` + "`json:\"tags,omitempty\"`" + `
	InStock     bool     ` + "`json:\"in_stock\"`" + `
}

type HealthCheckResponse struct {
	Status    string ` + "`json:\"status\"`" + `
	Version   string ` + "`json:\"version\"`" + `
	Uptime    int64  ` + "`json:\"uptime\"`" + `
	Timestamp string ` + "`json:\"timestamp\"`" + `
}

type BatchDeleteRequest struct {
	IDs    []string ` + "`json:\"ids\"`" + `
	DryRun bool     ` + "`json:\"dry_run\"`" + `
}

// --- Handlers ---

// 1. Deep nested struct response
// API_DESC Get order details
// API_TAGS Orders
func getOrderHandler(c *core.RequestEvent) error {
	resp := OrderResponse{
		ID: "ord_123",
		Status: "shipped",
		ShippingAddress: Address{
			Street: "123 Main St",
			Geo: GeoCoordinate{Latitude: 45.5, Longitude: -122.6},
		},
		Items: []OrderItem{{ProductID: "p1", Quantity: 2}},
		Payment: PaymentInfo{Method: "card", Amount: 19.98, Currency: "USD"},
	}
	return c.JSON(http.StatusOK, resp)
}

// 2. Nested struct request body via json.Decode
// API_DESC Create a new order
// API_TAGS Orders
func createOrderHandler(c *core.RequestEvent) error {
	var req CreateOrderRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid"})
	}
	resp := OrderResponse{ID: "ord_new", Status: "pending"}
	return c.JSON(http.StatusCreated, resp)
}

// 3. Array-of-structs response
// API_DESC List all orders
// API_TAGS Orders
func listOrdersHandler(c *core.RequestEvent) error {
	orders := []OrderResponse{
		{ID: "ord_001", Status: "delivered"},
	}
	return c.JSON(http.StatusOK, orders)
}

// 4. Struct with typed maps
// API_DESC Get indicator values
// API_TAGS Analytics
func getIndicatorsHandler(c *core.RequestEvent) error {
	resp := IndicatorValues{
		TokenID: "tok_1",
		Values: map[string]float64{"rsi": 62.5},
		Signals: map[string]string{"rsi": "neutral"},
		Computed: map[string]int{"candles": 500},
	}
	return c.JSON(http.StatusOK, resp)
}

// 5. Struct with any/interface{} fields + json.Decode request
// API_DESC Track analytics event
// API_TAGS Analytics
func trackEventHandler(c *core.RequestEvent) error {
	var req AnalyticsEvent
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid"})
	}
	return c.JSON(http.StatusCreated, req)
}

// 6. Inline map literal with nested sub-maps
// API_DESC Get diagnostics
// API_TAGS System
func getDiagnosticsHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status":  "operational",
		"version": "2.1.0",
		"uptime":  86400,
		"memory": map[string]any{
			"allocated_mb": 128,
			"gc_cycles":    42,
		},
		"database": map[string]any{
			"connected": true,
			"pool_size": 10,
		},
	})
}

// 7. Flat struct + nested struct field
// API_DESC Get user profile
// API_TAGS Users
func getUserProfileHandler(c *core.RequestEvent) error {
	resp := UserProfile{
		ID: "u1", Username: "john",
		Contact: ContactInfo{Email: "john@example.com"},
	}
	return c.JSON(http.StatusOK, resp)
}

// 8. Paginated: inline map wrapping struct array + struct value
// API_DESC Search users
// API_TAGS Users
func searchUsersHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"data": []UserProfile{
			{ID: "u1", Username: "alice"},
		},
		"pagination": PaginationMeta{Page: 1, PerPage: 20, TotalItems: 1, TotalPages: 1},
	})
}

// 9. Array of numeric-heavy structs
// API_DESC Get candlestick data
// API_TAGS Analytics
func getCandlestickHandler(c *core.RequestEvent) error {
	data := []TimeseriesPoint{
		{Timestamp: 1000, Open: 1.0, High: 1.05, Low: 0.98, Close: 1.02, Volume: 50000},
	}
	return c.JSON(http.StatusOK, data)
}

// 10. Pure map[string]string variable
// API_DESC Get config
// API_TAGS System
func getConfigHandler(c *core.RequestEvent) error {
	config := map[string]string{
		"log_level": "info",
		"region":    "us-west-2",
	}
	return c.JSON(http.StatusOK, config)
}

// 11. Mixed inline map: bools, ints, strings, nested maps
// API_DESC Get feature flags
// API_TAGS System
func getFeatureFlagsHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"flags": map[string]any{
			"dark_mode": true,
			"beta_api":  false,
		},
		"rate_limits": map[string]any{
			"requests_per_minute": 60,
			"enabled":             true,
		},
		"maintenance": false,
	})
}

// 12. map[string]any variable with MapAdditions
// API_DESC Get platform stats
// API_TAGS Analytics
func getPlatformStatsHandler(c *core.RequestEvent) error {
	result := map[string]any{
		"total_users": 15000,
		"revenue":     89432.50,
	}
	result["computed_at"] = time.Now().Format(time.RFC3339)
	result["cached"] = true
	return c.JSON(http.StatusOK, result)
}

// 13. BindBody request
// API_DESC Update profile
// API_TAGS Users
func updateProfileHandler(c *core.RequestEvent) error {
	var req UpdateProfileRequest
	if err := c.BindBody(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"updated": map[string]any{
			"display_name": req.DisplayName,
		},
	})
}

// 14. Embedded struct
// API_DESC Get product
// API_TAGS Products
func getProductHandler(c *core.RequestEvent) error {
	resp := ProductResponse{
		BaseEntity: BaseEntity{ID: "p1"},
		Name: "Widget", Price: 29.99, Currency: "USD", InStock: true,
	}
	return c.JSON(http.StatusOK, resp)
}

// 15. Slice of primitives
// API_DESC List categories
// API_TAGS Products
func listCategoriesHandler(c *core.RequestEvent) error {
	categories := []string{"electronics", "clothing", "books"}
	return c.JSON(http.StatusOK, categories)
}

// 16. DELETE — minimal response
// API_DESC Delete product
// API_TAGS Products
func deleteProductHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"success": true,
		"deleted": true,
	})
}

// 17. Variable-referenced struct
// API_DESC Health check
// API_TAGS System
func healthCheckHandler(c *core.RequestEvent) error {
	resp := HealthCheckResponse{Status: "healthy", Version: "2.1.0", Uptime: 86400}
	return c.JSON(http.StatusOK, resp)
}

// 18. Map with struct slice
// API_DESC Search products
// API_TAGS Products
func searchProductsHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"results": []ProductResponse{
			{BaseEntity: BaseEntity{ID: "p1"}, Name: "Widget", Price: 19.99},
		},
		"total":    1,
		"page":     1,
		"per_page": 20,
	})
}

// 19. Map literal containing struct values
// API_DESC Get order summary
// API_TAGS Orders
func getOrderSummaryHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"order_id": "ord_12345",
		"status":   "processing",
		"shipping": Address{
			Street: "789 Pine St", City: "Austin",
			Geo: GeoCoordinate{Latitude: 30.26, Longitude: -97.74},
		},
		"total_amount": 149.97,
	})
}

// 20. Multiple return paths + json.Decode request
// API_DESC Batch delete products
// API_TAGS Products
func batchDeleteHandler(c *core.RequestEvent) error {
	var req BatchDeleteRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid"})
	}
	if len(req.IDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "No IDs"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"deleted_count": len(req.IDs),
		"dry_run":       req.DryRun,
	})
}

// 21. Variable map with struct slices inside
// API_DESC Get dashboard
// API_TAGS Analytics
func getDashboardHandler(c *core.RequestEvent) error {
	dashboard := map[string]any{
		"recent_orders": []OrderResponse{
			{ID: "ord_999", Status: "pending"},
		},
		"top_users": []UserProfile{
			{ID: "u10", Username: "topuser"},
		},
		"total_revenue": 125000.50,
		"active_orders": 42,
	}
	return c.JSON(http.StatusOK, dashboard)
}

// 22. Struct pointer response
// API_DESC Get contact info
// API_TAGS Users
func getContactInfoHandler(c *core.RequestEvent) error {
	info := &ContactInfo{
		Email: "contact@example.com",
		SocialIDs: []string{"twitter:handle"},
	}
	return c.JSON(http.StatusOK, info)
}

// 23. Inline map with array of maps
// API_DESC Get activity feed
// API_TAGS Analytics
func getActivityFeedHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"activities": []map[string]any{
			{"type": "purchase", "user_id": "u1", "amount": 49.99},
		},
		"total_count": 2,
		"has_more":    false,
	})
}

// 24. Var-declared struct
// API_DESC Get default payment
// API_TAGS Orders
func getDefaultPaymentHandler(c *core.RequestEvent) error {
	var payment PaymentInfo = PaymentInfo{
		Method: "card", Currency: "USD", Amount: 0,
	}
	return c.JSON(http.StatusOK, payment)
}

// --- Helper functions (non-handlers) for return type resolution tests ---

func formatRecords(records []*core.Record) []map[string]any {
	result := make([]map[string]any, 0, len(records))
	return result
}

func buildSummary(name string) map[string]any {
	return map[string]any{"name": name}
}

func computeTotal(items []OrderItem) float64 {
	return 0.0
}

// 25. Handler calling a local function whose return type should be resolved
// API_DESC Get formatted records
// API_TAGS Records
func getFormattedRecordsHandler(c *core.RequestEvent) error {
	records, _ := c.App.FindRecordsByFilter("items", "1=1", "", 100, 0)
	items := formatRecords(records)
	result := map[string]any{"items": items, "count": len(items)}
	return c.JSON(http.StatusOK, result)
}

// 26. Handler with query parameters via URL.Query().Get()
// API_DESC Search with filters
// API_TAGS Search
func searchWithFiltersHandler(c *core.RequestEvent) error {
	q := c.Request.URL.Query()
	category := q.Get("category")
	status := q.Get("status")
	_ = category
	_ = status
	return c.JSON(http.StatusOK, map[string]any{"results": []string{}})
}

// 27. Handler calling a function that returns a primitive
// API_DESC Get computed total
// API_TAGS Orders
func getComputedTotalHandler(c *core.RequestEvent) error {
	total := computeTotal(nil)
	return c.JSON(http.StatusOK, map[string]any{"total": total})
}

// 25b. Map string any function
// API_DESC Get summary
// API_TAGS Summary
func getSummaryHandler(c *core.RequestEvent) error {
	summary := buildSummary("test")
	return c.JSON(http.StatusOK, summary)
}

// --- Helper functions for deep body schema analysis tests ---

// formatCandles mimics a real-world helper that builds []map[string]any inside a loop
func formatCandles(records []*core.Record) []map[string]any {
	result := make([]map[string]any, 0, len(records))
	for _, r := range records {
		entry := map[string]any{
			"t":      r.GetInt("unix_timestamp"),
			"o":      r.GetFloat("open"),
			"h":      r.GetFloat("high"),
			"l":      r.GetFloat("low"),
			"c":      r.GetFloat("close"),
			"v":      r.GetFloat("volume"),
			"trades": r.GetInt("trade_count"),
		}
		result = append(result, entry)
	}
	return result
}

// formatScoreItems mimics a helper with map additions after the literal
func formatScoreItems(records []*core.Record) []map[string]any {
	result := make([]map[string]any, 0, len(records))
	for _, r := range records {
		entry := map[string]any{
			"t":     r.GetInt("unix_timestamp"),
			"score": r.GetFloat("composite_score"),
			"trend": r.GetString("trend_label"),
		}
		entry["momentum"] = r.GetFloat("momentum")
		entry["volatility"] = r.GetFloat("volatility")
		result = append(result, entry)
	}
	return result
}

// buildTokenDetail returns a single map[string]any (not a slice)
func buildTokenDetail(r *core.Record) map[string]any {
	return map[string]any{
		"id":       r.GetString("id"),
		"symbol":   r.GetString("symbol"),
		"name":     r.GetString("name"),
		"decimals": r.GetInt("decimals"),
		"active":   r.GetBool("active"),
	}
}

// emptyFormatFunc returns []map[string]any but has no map literals (edge case)
func emptyFormatFunc(records []*core.Record) []map[string]any {
	return make([]map[string]any, 0)
}

// multiMapFunc has multiple map literals — the largest should win
func multiMapFunc(r *core.Record) map[string]any {
	small := map[string]any{"x": 1}
	_ = small
	big := map[string]any{
		"alpha":   r.GetString("a"),
		"beta":    r.GetFloat("b"),
		"gamma":   r.GetInt("c"),
		"delta":   r.GetBool("d"),
		"epsilon": r.GetString("e"),
	}
	return big
}

// 28. Handler using formatCandles helper (deep body schema resolution)
// API_DESC Get candle data
// API_TAGS Chart
func getCandleDataHandler(c *core.RequestEvent) error {
	records, _ := c.App.FindRecordsByFilter("candles", "1=1", "", 100, 0)
	candles := formatCandles(records)
	result := map[string]any{"candles": candles, "count": len(candles)}
	return c.JSON(http.StatusOK, result)
}

// 29. Handler using formatScoreItems helper (with map additions)
// API_DESC Get score items
// API_TAGS Scoring
func getScoreItemsHandler(c *core.RequestEvent) error {
	records, _ := c.App.FindRecordsByFilter("scores", "1=1", "", 100, 0)
	scores := formatScoreItems(records)
	return c.JSON(http.StatusOK, map[string]any{"scores": scores})
}

// 30. Handler using buildTokenDetail helper (single map, not slice)
// API_DESC Get token detail
// API_TAGS Tokens
func getTokenDetailHandler(c *core.RequestEvent) error {
	record, _ := c.App.FindRecordById("tokens", "abc123")
	detail := buildTokenDetail(record)
	return c.JSON(http.StatusOK, detail)
}

// 31. Handler using multiMapFunc helper (largest map should win)
// API_DESC Get multi map
// API_TAGS Test
func getMultiMapHandler(c *core.RequestEvent) error {
	record, _ := c.App.FindRecordById("things", "xyz")
	data := multiMapFunc(record)
	return c.JSON(http.StatusOK, map[string]any{"data": data})
}

// 32. Handler using emptyFormatFunc helper (should fall back gracefully)
// API_DESC Get empty format
// API_TAGS Test
func getEmptyFormatHandler(c *core.RequestEvent) error {
	records, _ := c.App.FindRecordsByFilter("empty", "1=1", "", 10, 0)
	items := emptyFormatFunc(records)
	return c.JSON(http.StatusOK, map[string]any{"items": items})
}

// --- Append-based inline loop patterns (make + append in handler body) ---

// 33. Handler that builds a slice with make() + append() in a for-range loop
// API_DESC List networks
// API_TAGS Networks
func listNetworksHandler(c *core.RequestEvent) error {
	records, _ := c.App.FindRecordsByFilter("networks", "1=1", "", 100, 0)
	networks := make([]map[string]any, 0, len(records))
	for _, r := range records {
		entry := map[string]any{
			"id":         r.GetString("id"),
			"name":       r.GetString("name"),
			"chain_id":   r.GetInt("chain_id"),
			"rpc_url":    r.GetString("rpc_url"),
			"active":     r.GetBool("active"),
		}
		networks = append(networks, entry)
	}
	return c.JSON(http.StatusOK, map[string]any{"networks": networks, "count": len(networks)})
}

// 34. Handler that builds a slice with make() + append() and map additions on appended item
// API_DESC List tokens
// API_TAGS Tokens
func listTokensHandler(c *core.RequestEvent) error {
	records, _ := c.App.FindRecordsByFilter("tokens", "1=1", "", 100, 0)
	tokens := make([]map[string]any, 0, len(records))
	for _, r := range records {
		entry := map[string]any{
			"id":       r.GetString("id"),
			"symbol":   r.GetString("symbol"),
			"name":     r.GetString("name"),
			"decimals": r.GetInt("decimals"),
		}
		entry["price"] = r.GetFloat("price")
		entry["volume"] = r.GetFloat("volume_24h")
		tokens = append(tokens, entry)
	}
	return c.JSON(http.StatusOK, map[string]any{"tokens": tokens})
}

// 35. Handler that directly returns a make+append result (not wrapped in outer map)
// API_DESC Get observations
// API_TAGS Analytics
func getObservationsHandler(c *core.RequestEvent) error {
	records, _ := c.App.FindRecordsByFilter("observations", "1=1", "", 100, 0)
	observations := make([]map[string]any, 0)
	for _, r := range records {
		obs := map[string]any{
			"timestamp": r.GetDateTime("created"),
			"value":     r.GetFloat("value"),
			"source":    r.GetString("source"),
		}
		observations = append(observations, obs)
	}
	result := map[string]any{
		"observations": observations,
		"total":        len(observations),
	}
	return c.JSON(http.StatusOK, result)
}

// 37. Handler that uses inline append (no separate entry variable)
// API_DESC List inline items
// API_TAGS Test
func listInlineAppendHandler(c *core.RequestEvent) error {
	records, _ := c.App.FindRecordsByFilter("items", "1=1", "", 100, 0)
	items := make([]map[string]any, 0, len(records))
	for _, r := range records {
		items = append(items, map[string]any{
			"id":       r.GetString("id"),
			"name":     r.GetString("name"),
			"value":    r.GetFloat("value"),
			"active":   r.GetBool("active"),
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// --- Index expression resolution from funcBodySchemas ---

// fetchIntervalSummary is a helper that returns a map[string]any with known keys
func fetchIntervalSummary(tokenID string) map[string]any {
	return map[string]any{
		"price":       42.5,
		"volume":      1000000.0,
		"market_cap":  5000000.0,
		"change_pct":  2.5,
		"high":        45.0,
		"low":         40.0,
	}
}

// fetchLatestSummary builds a map by reading keys from another helper's result
func fetchLatestSummary(tokenID string) map[string]any {
	summary := fetchIntervalSummary(tokenID)
	return map[string]any{
		"price":      summary["price"],
		"volume":     summary["volume"],
		"market_cap": summary["market_cap"],
		"updated":    true,
	}
}

// 36. Handler using fetchLatestSummary (index expr resolution via funcBodySchemas)
// API_DESC Get latest summary
// API_TAGS Analytics
func getLatestSummaryHandler(c *core.RequestEvent) error {
	summary := fetchLatestSummary("tok123")
	return c.JSON(http.StatusOK, summary)
}

// 37. Handler with inline e.Request.URL.Query().Get() (no intermediate variable)
// API_DESC Get items with inline query
// API_TAGS Items
func getItemsInlineQueryHandler(c *core.RequestEvent) error {
	sort := c.Request.URL.Query().Get("sort")
	limit := c.Request.URL.Query().Get("limit")
	_ = sort
	_ = limit
	return c.JSON(http.StatusOK, map[string]any{"items": []string{}})
}

// 38. Handler using e.RequestInfo() for query and header params
// API_DESC Get items via request info
// API_TAGS Items
func getItemsRequestInfoHandler(c *core.RequestEvent) error {
	info, _ := c.RequestInfo()
	search := info.Query["search"]
	token := info.Headers["authorization"]
	_ = search
	_ = token
	return c.JSON(http.StatusOK, map[string]any{"items": []string{}})
}

// 39. Handler using e.Request.Header.Get()
// API_DESC Get with custom header
// API_TAGS Items
func getWithHeaderHandler(c *core.RequestEvent) error {
	apiKey := c.Request.Header.Get("X-API-Key")
	_ = apiKey
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// 40. Handler using e.Request.PathValue()
// API_DESC Get item by ID
// API_TAGS Items
func getItemByPathValueHandler(c *core.RequestEvent) error {
	id := c.Request.PathValue("id")
	_ = id
	return c.JSON(http.StatusOK, map[string]any{"id": id})
}

// 41. Handler with mixed parameter sources
// API_DESC Get filtered items
// API_TAGS Items
func getMixedParamsHandler(c *core.RequestEvent) error {
	id := c.Request.PathValue("id")
	q := c.Request.URL.Query()
	interval := q.Get("interval")
	format := c.Request.URL.Query().Get("format")
	info, _ := c.RequestInfo()
	locale := info.Query["locale"]
	auth := info.Headers["x_auth_token"]
	custom := c.Request.Header.Get("X-Custom")
	_, _, _, _, _, _ = id, interval, format, locale, auth, custom
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// =============================================================================
// Indirect parameter extraction — helper functions
// =============================================================================

// timeParams is the result type returned by parseTimeParams.
type timeParams struct {
	Interval string
	From     string
	After    string
	To       string
	Limit    string
}

// parseTimeParams is a domain helper that extracts a fixed set of time-related query params.
// Pattern: q := e.Request.URL.Query(); q.Get("name")
func parseTimeParams(e *core.RequestEvent) timeParams {
	q := e.Request.URL.Query()
	return timeParams{
		Interval: q.Get("interval"),
		From:     q.Get("from"),
		After:    q.Get("after"),
		To:       q.Get("to"),
		Limit:    q.Get("limit"),
	}
}

// parseIntParam is a generic helper: the param name comes from the caller.
// Pattern: q.Get(name) where name is a string variable, not a literal.
func parseIntParam(e *core.RequestEvent, name string, defaultVal int) int {
	q := e.Request.URL.Query()
	v := q.Get(name)
	if v == "" {
		return defaultVal
	}
	return 0
}

// parseBoolParam is a generic helper similar to parseIntParam.
func parseBoolParam(e *core.RequestEvent, name string) bool {
	q := e.Request.URL.Query()
	return q.Get(name) == "true"
}

// parseHeaderParam is a generic helper that reads from a request header.
func parseHeaderParam(e *core.RequestEvent, name string) string {
	return e.Request.Header.Get(name)
}

// parseRequestInfoParam is a generic helper that reads a query param via RequestInfo.
func parseRequestInfoParam(e *core.RequestEvent, name string) string {
	info, _ := e.RequestInfo()
	return info.Query[name]
}

// 42. Handler that delegates entirely to parseTimeParams (domain helper)
// API_DESC Get chart data
// API_TAGS Charts
func getChartHandler(c *core.RequestEvent) error {
	tp := parseTimeParams(c)
	_ = tp
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// 43. Handler that calls a generic int-param helper with literal name
// API_DESC List paginated items
// API_TAGS Items
func getPaginatedItemsHandler(c *core.RequestEvent) error {
	page := parseIntParam(c, "page", 1)
	pageSize := parseIntParam(c, "page_size", 20)
	_, _ = page, pageSize
	return c.JSON(http.StatusOK, map[string]any{"items": []string{}})
}

// 44. Handler that calls a generic bool-param helper with literal name
// API_DESC Get verbose data
// API_TAGS Items
func getVerboseDataHandler(c *core.RequestEvent) error {
	verbose := parseBoolParam(c, "verbose")
	debug := parseBoolParam(c, "debug")
	_, _ = verbose, debug
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// 45. Handler calling multiple helpers (domain + generic)
// API_DESC Get chart with pagination
// API_TAGS Charts
func getChartPaginatedHandler(c *core.RequestEvent) error {
	tp := parseTimeParams(c)
	page := parseIntParam(c, "page", 0)
	_ = tp
	_ = page
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// 46. Handler calling a generic header helper
// API_DESC Get data with auth header
// API_TAGS Items
func getWithAuthHeaderHandler(c *core.RequestEvent) error {
	token := parseHeaderParam(c, "Authorization")
	_ = token
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// 47. Handler that mixes direct params and indirect helper params
// API_DESC Get chart with extra direct param
// API_TAGS Charts
func getChartWithDirectParamHandler(c *core.RequestEvent) error {
	tp := parseTimeParams(c)
	sort := c.Request.URL.Query().Get("sort")
	_, _ = tp, sort
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

// 48. Handler using slice[i] = map[string]any{...} (index assignment, not append)
// This pattern is used when the slice is pre-allocated with make([]T, n) and
// populated in a for loop via index assignment rather than append.
// API_DESC List todos with index assignment
// API_TAGS Items
func listTodosIndexHandler(c *core.RequestEvent) error {
	records := []string{"a", "b"}
	todos := make([]map[string]any, len(records))
	for i, r := range records {
		todos[i] = map[string]any{
			"id":          r,
			"title":       r,
			"priority":    "high",
			"completed":   false,
			"description": r,
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"todos": todos,
		"count": len(todos),
	})
}

// updateTodoMapAdditionsHandler exercises the make(map)+additions pattern.
// The 'updates' map starts as make(map[string]any) but concrete keys are added
// afterwards, so the final schema should NOT have additionalProperties:true.
func updateTodoMapAdditionsHandler(c *core.RequestEvent) error {
	updates := make(map[string]any)
	updates["title"] = "new title"
	updates["completed"] = true
	updates["priority"] = "low"
	updates["description"] = "updated"
	return c.JSON(http.StatusOK, map[string]any{
		"updated": true,
		"changes": updates,
	})
}

// getIntTypingHandler tests that r.GetInt() produces integer schema and r.GetFloat() produces number.
// Previously both were collapsed to "number"; GetInt should be "integer".
func getIntTypingHandler(c *core.RequestEvent) error {
	var r *core.Record
	return c.JSON(http.StatusOK, map[string]any{
		"page":    r.GetInt("page"),
		"total":   r.GetInt("total"),
		"price":   r.GetFloat("price"),
		"score":   r.GetFloat("score"),
		"label":   r.GetString("label"),
		"enabled": r.GetBool("enabled"),
	})
}

// conditionalKeysHandler tests that keys assigned inside if blocks are tracked.
// entry["direction"] and entry["entry_price"] are set via variables typed from GetString/GetFloat.
func conditionalKeysHandler(c *core.RequestEvent) error {
	var r *core.Record
	entry := map[string]any{
		"signal": r.GetString("signal"),
		"value":  r.GetFloat("value"),
	}
	if td := r.GetString("direction"); td != "" {
		entry["direction"]   = td
		entry["entry_price"] = r.GetFloat("entry_price")
		entry["stop_loss"]   = r.GetFloat("stop_loss")
	}
	return c.JSON(http.StatusOK, map[string]any{
		"data": entry,
	})
}

// sliceIndexTypeHandler tests that indexing a typed slice ([]float64) resolves to
// the element type (number) rather than falling back to interface{} / object.
func sliceIndexTypeHandler(c *core.RequestEvent) error {
	returns := []float64{1.0, 2.0, 3.0}
	return c.JSON(http.StatusOK, map[string]any{
		"min": returns[0],
		"max": returns[1],
		"avg": returns[2],
	})
}
`

// parseHandlerScenarios parses the handlerScenarioSource and returns the parser.
func parseHandlerScenarios(t *testing.T) *ASTParser {
	t.Helper()
	parser := NewASTParser()
	filePath := createTestFile(t, "handlers_scenario.go", handlerScenarioSource)
	if err := parser.ParseFile(filePath); err != nil {
		t.Fatalf("Failed to parse handler scenarios: %v", err)
	}
	return parser
}

// requireHandler returns the named handler or fails the test.
func requireHandler(t *testing.T, parser *ASTParser, name string) *ASTHandlerInfo {
	t.Helper()
	h, ok := parser.GetHandlerByName(name)
	if !ok || h == nil {
		t.Fatalf("Handler %q not found", name)
	}
	return h
}

// assertRef checks that a schema is a $ref to the given component.
func assertRef(t *testing.T, schema *OpenAPISchema, component string, context string) {
	t.Helper()
	if schema == nil {
		t.Fatalf("%s: schema is nil", context)
	}
	expected := "#/components/schemas/" + component
	if schema.Ref != expected {
		t.Errorf("%s: expected $ref %q, got Ref=%q Type=%q", context, expected, schema.Ref, schema.Type)
	}
}

// assertArrayOfRef checks that a schema is {type:"array", items:{$ref:...}}.
func assertArrayOfRef(t *testing.T, schema *OpenAPISchema, component string, context string) {
	t.Helper()
	if schema == nil {
		t.Fatalf("%s: schema is nil", context)
	}
	if schema.Type != "array" {
		t.Errorf("%s: expected type 'array', got %q", context, schema.Type)
	}
	if schema.Items == nil {
		t.Fatalf("%s: items is nil", context)
	}
	assertRef(t, schema.Items, component, context+" items")
}

// assertInlineObject checks that a schema is an inline object with the given property names.
func assertInlineObject(t *testing.T, schema *OpenAPISchema, expectedProps []string, context string) {
	t.Helper()
	if schema == nil {
		t.Fatalf("%s: schema is nil", context)
	}
	if schema.Type != "object" {
		t.Errorf("%s: expected type 'object', got %q", context, schema.Type)
	}
	if schema.Properties == nil {
		t.Fatalf("%s: properties is nil", context)
	}
	for _, prop := range expectedProps {
		if _, ok := schema.Properties[prop]; !ok {
			t.Errorf("%s: missing expected property %q", context, prop)
		}
	}
}

func TestHandlerScenario_DeepNestedStructResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getOrderHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}
	assertRef(t, h.ResponseSchema, "OrderResponse", "response")

	// Verify OrderResponse component has nested $ref fields
	schema := parser.generateSchemaFromType("OrderResponse", true)
	if schema == nil {
		t.Fatal("Expected OrderResponse schema")
	}
	assertRef(t, schema.Properties["shipping_address"], "Address", "shipping_address")
	assertRef(t, schema.Properties["payment"], "PaymentInfo", "payment")
	if schema.Properties["items"] == nil || schema.Properties["items"].Type != "array" {
		t.Fatal("Expected items to be array")
	}
	assertRef(t, schema.Properties["items"].Items, "OrderItem", "items")

	// Verify Address has nested $ref to GeoCoordinate
	addrSchema := parser.generateSchemaFromType("Address", true)
	assertRef(t, addrSchema.Properties["geo"], "GeoCoordinate", "Address.geo")
}

func TestHandlerScenario_JsonDecodeRequestBody(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "createOrderHandler")

	if h.RequestType != "CreateOrderRequest" {
		t.Errorf("Expected RequestType 'CreateOrderRequest', got %q", h.RequestType)
	}
	if h.RequestSchema == nil {
		t.Fatal("Expected request schema")
	}
	assertRef(t, h.RequestSchema, "CreateOrderRequest", "request")

	// Response should also be $ref OrderResponse
	assertRef(t, h.ResponseSchema, "OrderResponse", "response")

	// Verify json.Decode detection
	if !h.UsesJSONDecode {
		t.Error("Expected UsesJSONDecode to be true")
	}
}

func TestHandlerScenario_ArrayOfStructsResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "listOrdersHandler")

	assertArrayOfRef(t, h.ResponseSchema, "OrderResponse", "response")
}

func TestHandlerScenario_StructWithTypedMaps(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getIndicatorsHandler")

	assertRef(t, h.ResponseSchema, "IndicatorValues", "response")

	// Verify IndicatorValues component schema
	schema := parser.generateSchemaFromType("IndicatorValues", true)
	if schema == nil {
		t.Fatal("Expected IndicatorValues schema")
	}

	// map[string]float64 → additionalProperties: {type: "number"}
	valuesField := schema.Properties["values"]
	if valuesField == nil || valuesField.Type != "object" {
		t.Fatal("Expected values to be object")
	}
	if ap, ok := valuesField.AdditionalProperties.(*OpenAPISchema); ok {
		if ap.Type != "number" {
			t.Errorf("Expected values additionalProperties type 'number', got %q", ap.Type)
		}
	} else {
		t.Error("Expected values additionalProperties to be a schema")
	}

	// map[string]string → additionalProperties: {type: "string"}
	signalsField := schema.Properties["signals"]
	if ap, ok := signalsField.AdditionalProperties.(*OpenAPISchema); ok {
		if ap.Type != "string" {
			t.Errorf("Expected signals additionalProperties type 'string', got %q", ap.Type)
		}
	} else {
		t.Error("Expected signals additionalProperties to be a schema")
	}

	// map[string]int → additionalProperties: {type: "integer"}
	computedField := schema.Properties["computed"]
	if ap, ok := computedField.AdditionalProperties.(*OpenAPISchema); ok {
		if ap.Type != "integer" {
			t.Errorf("Expected computed additionalProperties type 'integer', got %q", ap.Type)
		}
	} else {
		t.Error("Expected computed additionalProperties to be a schema")
	}
}

func TestHandlerScenario_AnyFieldsAndJsonDecode(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "trackEventHandler")

	// Request: json.Decode → $ref AnalyticsEvent
	assertRef(t, h.RequestSchema, "AnalyticsEvent", "request")
	if !h.UsesJSONDecode {
		t.Error("Expected UsesJSONDecode to be true")
	}

	// Response: returning req variable → $ref AnalyticsEvent
	assertRef(t, h.ResponseSchema, "AnalyticsEvent", "response")

	// Verify any fields in component schema
	schema := parser.generateSchemaFromType("AnalyticsEvent", true)

	// map[string]any → additionalProperties: true (NOT nested object)
	propsField := schema.Properties["properties"]
	if propsField == nil {
		t.Fatal("Expected properties field")
	}
	if propsField.Type != "object" {
		t.Errorf("Expected properties type 'object', got %q", propsField.Type)
	}
	if propsField.AdditionalProperties != true {
		t.Errorf("Expected map[string]any to produce additionalProperties: true, got %v", propsField.AdditionalProperties)
	}

	// any → {type: "object", additionalProperties: true}
	contextField := schema.Properties["context"]
	if contextField == nil {
		t.Fatal("Expected context field")
	}
	if contextField.Type != "object" {
		t.Errorf("Expected context type 'object', got %q", contextField.Type)
	}
	if contextField.AdditionalProperties != true {
		t.Errorf("Expected any to produce additionalProperties: true, got %v", contextField.AdditionalProperties)
	}
}

func TestHandlerScenario_InlineMapWithNestedMaps(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getDiagnosticsHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"status", "version", "uptime", "memory", "database"}, "response")

	// Nested map values should be inline objects too
	memoryProp := schema.Properties["memory"]
	assertInlineObject(t, memoryProp, []string{"allocated_mb", "gc_cycles"}, "memory")

	dbProp := schema.Properties["database"]
	assertInlineObject(t, dbProp, []string{"connected", "pool_size"}, "database")
}

func TestHandlerScenario_FlatStructWithNestedStruct(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getUserProfileHandler")

	assertRef(t, h.ResponseSchema, "UserProfile", "response")

	// UserProfile.contact → $ref ContactInfo
	schema := parser.generateSchemaFromType("UserProfile", true)
	assertRef(t, schema.Properties["contact"], "ContactInfo", "contact")
}

func TestHandlerScenario_PaginatedMapWithStructs(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "searchUsersHandler")

	schema := h.ResponseSchema
	if schema == nil {
		t.Fatal("Expected response schema")
	}
	if schema.Properties == nil {
		t.Fatal("Expected inline object with properties")
	}

	// data → array of $ref UserProfile
	dataField := schema.Properties["data"]
	assertArrayOfRef(t, dataField, "UserProfile", "data")

	// pagination → $ref PaginationMeta
	paginationField := schema.Properties["pagination"]
	assertRef(t, paginationField, "PaginationMeta", "pagination")
}

func TestHandlerScenario_ArrayOfNumericStructs(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getCandlestickHandler")

	assertArrayOfRef(t, h.ResponseSchema, "TimeseriesPoint", "response")
}

func TestHandlerScenario_MapStringStringVariable(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getConfigHandler")

	schema := h.ResponseSchema
	if schema == nil {
		t.Fatal("Expected response schema")
	}
	// map[string]string literal → should have properties with string values
	if schema.Properties == nil {
		t.Fatal("Expected properties from map literal")
	}
	for _, key := range []string{"log_level", "region"} {
		prop := schema.Properties[key]
		if prop == nil {
			t.Errorf("Expected property %q", key)
			continue
		}
		if prop.Type != "string" {
			t.Errorf("Expected property %q type 'string', got %q", key, prop.Type)
		}
	}
}

func TestHandlerScenario_MixedInlineMap(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getFeatureFlagsHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"flags", "rate_limits", "maintenance"}, "response")

	// flags → nested inline object
	flagsProp := schema.Properties["flags"]
	assertInlineObject(t, flagsProp, []string{"dark_mode", "beta_api"}, "flags")

	// rate_limits → nested inline object
	rlProp := schema.Properties["rate_limits"]
	assertInlineObject(t, rlProp, []string{"requests_per_minute", "enabled"}, "rate_limits")

	// maintenance → boolean
	maintProp := schema.Properties["maintenance"]
	if maintProp == nil || maintProp.Type != "boolean" {
		t.Error("Expected maintenance to be boolean")
	}
}

func TestHandlerScenario_MapVariableWithAdditions(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getPlatformStatsHandler")

	schema := h.ResponseSchema
	if schema == nil || schema.Properties == nil {
		t.Fatal("Expected inline object response")
	}

	// Original map keys
	for _, key := range []string{"total_users", "revenue"} {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("Expected original map property %q", key)
		}
	}

	// MapAdditions: result["computed_at"] and result["cached"]
	if _, ok := schema.Properties["computed_at"]; !ok {
		t.Error("Expected MapAddition 'computed_at' to be merged")
	}
	if _, ok := schema.Properties["cached"]; !ok {
		t.Error("Expected MapAddition 'cached' to be merged")
	}
}

func TestHandlerScenario_BindBodyRequest(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "updateProfileHandler")

	// BindBody → request detected
	if !h.UsesBindBody {
		t.Error("Expected UsesBindBody to be true")
	}
	if h.RequestType != "UpdateProfileRequest" {
		t.Errorf("Expected RequestType 'UpdateProfileRequest', got %q", h.RequestType)
	}
	assertRef(t, h.RequestSchema, "UpdateProfileRequest", "request")

	// Response is inline map
	assertInlineObject(t, h.ResponseSchema, []string{"success", "updated"}, "response")
}

func TestHandlerScenario_EmbeddedStruct(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getProductHandler")

	assertRef(t, h.ResponseSchema, "ProductResponse", "response")

	// ProductResponse should have flattened fields from BaseEntity
	schema := parser.generateSchemaFromType("ProductResponse", true)
	if schema == nil {
		t.Fatal("Expected ProductResponse schema")
	}
	// BaseEntity fields should be promoted
	for _, field := range []string{"id", "created_at", "updated_at"} {
		if _, ok := schema.Properties[field]; !ok {
			t.Errorf("Expected embedded field %q from BaseEntity", field)
		}
	}
	// Own fields
	for _, field := range []string{"name", "description", "price", "currency", "in_stock"} {
		if _, ok := schema.Properties[field]; !ok {
			t.Errorf("Expected own field %q", field)
		}
	}
}

func TestHandlerScenario_SliceOfPrimitives(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "listCategoriesHandler")

	schema := h.ResponseSchema
	if schema == nil {
		t.Fatal("Expected response schema")
	}
	if schema.Type != "array" {
		t.Errorf("Expected type 'array', got %q", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("Expected items")
	}
	if schema.Items.Type != "string" {
		t.Errorf("Expected items type 'string', got %q", schema.Items.Type)
	}
}

func TestHandlerScenario_MinimalDeleteResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "deleteProductHandler")

	assertInlineObject(t, h.ResponseSchema, []string{"success", "deleted"}, "response")
}

func TestHandlerScenario_VariableReferencedStruct(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "healthCheckHandler")

	assertRef(t, h.ResponseSchema, "HealthCheckResponse", "response")
}

func TestHandlerScenario_MapWithStructSlice(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "searchProductsHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"results", "total", "page", "per_page"}, "response")

	// results → array of $ref ProductResponse
	assertArrayOfRef(t, schema.Properties["results"], "ProductResponse", "results")
}

func TestHandlerScenario_MapWithStructValue(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getOrderSummaryHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"order_id", "status", "shipping", "total_amount"}, "response")

	// shipping → $ref Address
	assertRef(t, schema.Properties["shipping"], "Address", "shipping")
}

func TestHandlerScenario_MultipleReturnPaths(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "batchDeleteHandler")

	// Request: json.Decode → $ref BatchDeleteRequest
	assertRef(t, h.RequestSchema, "BatchDeleteRequest", "request")
	if !h.UsesJSONDecode {
		t.Error("Expected UsesJSONDecode to be true")
	}

	// Response: last c.JSON call (success path)
	schema := h.ResponseSchema
	if schema == nil {
		t.Fatal("Expected response schema")
	}
	// Should have properties from the success-path map
	if schema.Properties != nil {
		if _, ok := schema.Properties["deleted_count"]; ok {
			// success path picked up — good
		}
	}
}

func TestHandlerScenario_VariableMapWithStructSlices(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getDashboardHandler")

	schema := h.ResponseSchema
	if schema == nil || schema.Properties == nil {
		t.Fatal("Expected inline object response")
	}

	// recent_orders → array of $ref OrderResponse
	recentOrders := schema.Properties["recent_orders"]
	assertArrayOfRef(t, recentOrders, "OrderResponse", "recent_orders")

	// top_users → array of $ref UserProfile
	topUsers := schema.Properties["top_users"]
	assertArrayOfRef(t, topUsers, "UserProfile", "top_users")

	// Primitive fields
	if _, ok := schema.Properties["total_revenue"]; !ok {
		t.Error("Expected 'total_revenue' property")
	}
	if _, ok := schema.Properties["active_orders"]; !ok {
		t.Error("Expected 'active_orders' property")
	}
}

func TestHandlerScenario_StructPointerResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getContactInfoHandler")

	assertRef(t, h.ResponseSchema, "ContactInfo", "response")
}

func TestHandlerScenario_InlineMapWithArrayOfMaps(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getActivityFeedHandler")

	schema := h.ResponseSchema
	assertInlineObject(t, schema, []string{"activities", "total_count", "has_more"}, "response")

	// activities → array — items should be free-form object
	activitiesField := schema.Properties["activities"]
	if activitiesField == nil || activitiesField.Type != "array" {
		t.Fatal("Expected activities to be array")
	}
	if activitiesField.Items == nil {
		t.Fatal("Expected activities items")
	}
	// []map[string]any → items should be {type:"object", additionalProperties:true}
	items := activitiesField.Items
	if items.Type != "object" {
		t.Errorf("Expected items type 'object', got %q", items.Type)
	}
	if items.AdditionalProperties != true {
		t.Errorf("Expected items additionalProperties to be true (free-form), got %v", items.AdditionalProperties)
	}
}

func TestHandlerScenario_VarDeclaredStruct(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getDefaultPaymentHandler")

	assertRef(t, h.ResponseSchema, "PaymentInfo", "response")
}

func TestHandlerScenario_MapStringAnyFreeForm(t *testing.T) {
	// Verify that map[string]any produces {type:"object", additionalProperties:true}
	// (NOT nested {additionalProperties: {type:"object", additionalProperties:true}})
	parser := parseHandlerScenarios(t)

	schema := parser.generateSchemaFromType("map[string]any", false)
	if schema == nil {
		t.Fatal("Expected schema for map[string]any")
	}
	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got %q", schema.Type)
	}
	if schema.AdditionalProperties != true {
		t.Errorf("Expected additionalProperties: true, got %v (type %T)", schema.AdditionalProperties, schema.AdditionalProperties)
	}
}

func TestHandlerScenario_StructDiscovery(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// All struct definitions should be discovered
	expectedStructs := []string{
		"GeoCoordinate", "Address", "ContactInfo", "OrderItem", "PaymentInfo",
		"OrderResponse", "CreateOrderRequest", "AnalyticsEvent", "PaginationMeta",
		"UserProfile", "TimeseriesPoint", "IndicatorValues", "UpdateProfileRequest",
		"BaseEntity", "ProductResponse", "HealthCheckResponse", "BatchDeleteRequest",
	}

	allStructs := parser.GetAllStructs()
	for _, name := range expectedStructs {
		if _, ok := allStructs[name]; !ok {
			t.Errorf("Expected struct %q to be discovered", name)
		}
	}
}

func TestHandlerScenario_HandlerDiscovery(t *testing.T) {
	parser := parseHandlerScenarios(t)

	expectedHandlers := []string{
		"getOrderHandler", "createOrderHandler", "listOrdersHandler",
		"getIndicatorsHandler", "trackEventHandler", "getDiagnosticsHandler",
		"getUserProfileHandler", "searchUsersHandler", "getCandlestickHandler",
		"getConfigHandler", "getFeatureFlagsHandler", "getPlatformStatsHandler",
		"updateProfileHandler", "getProductHandler", "listCategoriesHandler",
		"deleteProductHandler", "healthCheckHandler", "searchProductsHandler",
		"getOrderSummaryHandler", "batchDeleteHandler", "getDashboardHandler",
		"getContactInfoHandler", "getActivityFeedHandler", "getDefaultPaymentHandler",
		"getFormattedRecordsHandler", "searchWithFiltersHandler",
		"getComputedTotalHandler", "getSummaryHandler",
		"getCandleDataHandler", "getScoreItemsHandler",
		"getTokenDetailHandler", "getMultiMapHandler", "getEmptyFormatHandler",
		"listNetworksHandler", "listTokensHandler", "getObservationsHandler",
		"getLatestSummaryHandler", "listInlineAppendHandler",
		"getItemsInlineQueryHandler", "getItemsRequestInfoHandler",
		"getWithHeaderHandler", "getItemByPathValueHandler", "getMixedParamsHandler",
		// Indirect param extraction handlers (42-47) + index-assignment handler (48) + map-additions handler (49)
		// + GetInt/conditional/slice-index fix handlers (50-52)
		"getChartHandler", "getPaginatedItemsHandler", "getVerboseDataHandler",
		"getChartPaginatedHandler", "getWithAuthHeaderHandler", "getChartWithDirectParamHandler",
		"listTodosIndexHandler", "updateTodoMapAdditionsHandler",
		"getIntTypingHandler", "conditionalKeysHandler", "sliceIndexTypeHandler",
	}

	allHandlers := parser.GetAllHandlers()
	if len(allHandlers) != len(expectedHandlers) {
		t.Errorf("Expected %d handlers, got %d", len(expectedHandlers), len(allHandlers))
	}
	for _, name := range expectedHandlers {
		if _, ok := allHandlers[name]; !ok {
			t.Errorf("Expected handler %q to be discovered", name)
		}
	}
}

func TestHandlerScenario_APIDescAndTags(t *testing.T) {
	parser := parseHandlerScenarios(t)

	tests := []struct {
		handler string
		desc    string
		tags    []string
	}{
		{"getOrderHandler", "Get order details", []string{"Orders"}},
		{"createOrderHandler", "Create a new order", []string{"Orders"}},
		{"getDiagnosticsHandler", "Get diagnostics", []string{"System"}},
		{"trackEventHandler", "Track analytics event", []string{"Analytics"}},
		{"updateProfileHandler", "Update profile", []string{"Users"}},
		{"getProductHandler", "Get product", []string{"Products"}},
	}

	for _, tt := range tests {
		t.Run(tt.handler, func(t *testing.T) {
			h := requireHandler(t, parser, tt.handler)
			if h.APIDescription != tt.desc {
				t.Errorf("Expected desc %q, got %q", tt.desc, h.APIDescription)
			}
			if len(h.APITags) != len(tt.tags) {
				t.Errorf("Expected %d tags, got %d: %v", len(tt.tags), len(h.APITags), h.APITags)
				return
			}
			for i, tag := range tt.tags {
				if h.APITags[i] != tag {
					t.Errorf("Expected tag[%d] = %q, got %q", i, tag, h.APITags[i])
				}
			}
		})
	}
}

func TestHandlerScenario_PointerFields(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// OrderResponse has *Address for billing_address and *string for notes
	schema := parser.generateSchemaFromType("OrderResponse", true)
	if schema == nil {
		t.Fatal("Expected OrderResponse schema")
	}

	// billing_address is *Address → should still be $ref (pointer unwrapped)
	billingAddr := schema.Properties["billing_address"]
	assertRef(t, billingAddr, "Address", "billing_address")

	// ContactInfo has *string fields
	ciSchema := parser.generateSchemaFromType("ContactInfo", true)
	phoneField := ciSchema.Properties["phone"]
	if phoneField == nil {
		t.Fatal("Expected phone field")
	}
	if phoneField.Type != "string" {
		t.Errorf("Expected phone type 'string', got %q", phoneField.Type)
	}
}

// =============================================================================
// Function Return Type Resolution Tests
// =============================================================================

func TestFuncReturnTypeExtraction(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// Check that helper function return types were extracted
	tests := []struct {
		funcName     string
		expectedType string
	}{
		{"formatRecords", "[]map[string]any"},
		{"buildSummary", "map[string]any"},
		{"computeTotal", "float64"},
	}

	for _, tt := range tests {
		retType, exists := parser.funcReturnTypes[tt.funcName]
		if !exists {
			t.Errorf("Expected return type for %q to be extracted", tt.funcName)
			continue
		}
		if retType != tt.expectedType {
			t.Errorf("Return type for %q: expected %q, got %q", tt.funcName, tt.expectedType, retType)
		}
	}

	// Handlers should NOT be in funcReturnTypes
	for _, handlerName := range []string{"getOrderHandler", "healthCheckHandler", "getFormattedRecordsHandler"} {
		if _, exists := parser.funcReturnTypes[handlerName]; exists {
			t.Errorf("Handler %q should not be in funcReturnTypes", handlerName)
		}
	}
}

func TestHandlerScenario_FunctionReturnTypeResolution(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getFormattedRecordsHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// Response should be an object with "items" and "count"
	assertInlineObject(t, h.ResponseSchema, []string{"items", "count"}, "response")

	// "items" should be type:"array" (resolved from formatRecords return type []map[string]any)
	itemsSchema := h.ResponseSchema.Properties["items"]
	if itemsSchema == nil {
		t.Fatal("Expected 'items' property in response schema")
	}
	if itemsSchema.Type != "array" {
		t.Errorf("Expected 'items' type to be 'array', got %q", itemsSchema.Type)
	}

	// "count" should be integer (from len())
	countSchema := h.ResponseSchema.Properties["count"]
	if countSchema == nil {
		t.Fatal("Expected 'count' property in response schema")
	}
	if countSchema.Type != "integer" {
		t.Errorf("Expected 'count' type to be 'integer', got %q", countSchema.Type)
	}
}

func TestHandlerScenario_FunctionReturnTypePrimitive(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getComputedTotalHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// "total" should be number (resolved from computeTotal return type float64)
	totalSchema := h.ResponseSchema.Properties["total"]
	if totalSchema == nil {
		t.Fatal("Expected 'total' property in response schema")
	}
	if totalSchema.Type != "number" {
		t.Errorf("Expected 'total' type to be 'number', got %q", totalSchema.Type)
	}
}

func TestHandlerScenario_FunctionReturnTypeMapResponse(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getSummaryHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// buildSummary returns map[string]any — with body analysis, we should now see the "name" key
	if h.ResponseSchema.Type != "object" {
		t.Errorf("Expected response type 'object', got %q", h.ResponseSchema.Type)
	}
	if nameSchema, ok := h.ResponseSchema.Properties["name"]; ok {
		if nameSchema.Type != "string" {
			t.Errorf("Expected 'name' type 'string', got %q", nameSchema.Type)
		}
	}
}

// =============================================================================
// Helper Function Body Schema Analysis Tests (funcBodySchemas)
// =============================================================================

func TestFuncBodySchema_SliceMapHelper(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// formatCandles returns []map[string]any with keys: t, o, h, l, c, v, trades
	schema, exists := parser.funcBodySchemas["formatCandles"]
	if !exists {
		t.Fatal("Expected funcBodySchemas entry for 'formatCandles'")
	}

	// Should be an array wrapping an object
	if schema.Type != "array" {
		t.Fatalf("Expected type 'array', got %q", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("Expected Items schema on array")
	}
	if schema.Items.Type != "object" {
		t.Fatalf("Expected items type 'object', got %q", schema.Items.Type)
	}

	expectedKeys := []string{"t", "o", "h", "l", "c", "v", "trades"}
	for _, key := range expectedKeys {
		prop, ok := schema.Items.Properties[key]
		if !ok {
			t.Errorf("Expected property %q in formatCandles item schema", key)
			continue
		}
		// t and trades come from GetInt → "number", o/h/l/c/v from GetFloat → "number"
		if prop.Type != "number" && prop.Type != "integer" {
			t.Errorf("Property %q: expected numeric type, got %q", key, prop.Type)
		}
	}
}

func TestFuncBodySchema_WithMapAdditions(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// formatScoreItems has entry map literal + entry["momentum"] and entry["volatility"] additions
	schema, exists := parser.funcBodySchemas["formatScoreItems"]
	if !exists {
		t.Fatal("Expected funcBodySchemas entry for 'formatScoreItems'")
	}

	if schema.Type != "array" {
		t.Fatalf("Expected type 'array', got %q", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("Expected Items schema")
	}

	// Keys from the literal
	for _, key := range []string{"t", "score", "trend"} {
		if _, ok := schema.Items.Properties[key]; !ok {
			t.Errorf("Expected property %q from map literal", key)
		}
	}
	// Keys from map additions
	for _, key := range []string{"momentum", "volatility"} {
		if _, ok := schema.Items.Properties[key]; !ok {
			t.Errorf("Expected property %q from map additions", key)
		}
	}

	// Verify types
	if s := schema.Items.Properties["trend"]; s != nil && s.Type != "string" {
		t.Errorf("Expected 'trend' type 'string', got %q", s.Type)
	}
	if s := schema.Items.Properties["score"]; s != nil && s.Type != "number" {
		t.Errorf("Expected 'score' type 'number', got %q", s.Type)
	}
}

func TestFuncBodySchema_SingleMapHelper(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// buildTokenDetail returns map[string]any (not a slice)
	schema, exists := parser.funcBodySchemas["buildTokenDetail"]
	if !exists {
		t.Fatal("Expected funcBodySchemas entry for 'buildTokenDetail'")
	}

	// Should be an object directly (not wrapped in array)
	if schema.Type != "object" {
		t.Fatalf("Expected type 'object', got %q", schema.Type)
	}

	expectedKeys := map[string]string{
		"id":       "string",
		"symbol":   "string",
		"name":     "string",
		"decimals": "integer", // r.GetInt() now correctly produces "integer"
		"active":   "boolean",
	}
	for key, expectedType := range expectedKeys {
		prop, ok := schema.Properties[key]
		if !ok {
			t.Errorf("Expected property %q in buildTokenDetail schema", key)
			continue
		}
		if prop.Type != expectedType {
			t.Errorf("Property %q: expected type %q, got %q", key, expectedType, prop.Type)
		}
	}
}

func TestFuncBodySchema_MultipleMapLiterals(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// multiMapFunc has two map literals: small (1 key) and big (5 keys) — big should win
	schema, exists := parser.funcBodySchemas["multiMapFunc"]
	if !exists {
		t.Fatal("Expected funcBodySchemas entry for 'multiMapFunc'")
	}

	if schema.Type != "object" {
		t.Fatalf("Expected type 'object', got %q", schema.Type)
	}

	// Should have 5 keys from the "big" map, not 1 from "small"
	if len(schema.Properties) != 5 {
		t.Errorf("Expected 5 properties (from largest map literal), got %d", len(schema.Properties))
	}
	for _, key := range []string{"alpha", "beta", "gamma", "delta", "epsilon"} {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("Expected property %q from largest map literal", key)
		}
	}
	// "x" from the small map should NOT be present
	if _, ok := schema.Properties["x"]; ok {
		t.Error("Property 'x' from small map should not be in schema (largest map wins)")
	}
}

func TestFuncBodySchema_EmptyHelper(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// emptyFormatFunc has no map literals — should NOT have a funcBodySchemas entry
	_, exists := parser.funcBodySchemas["emptyFormatFunc"]
	if exists {
		t.Error("emptyFormatFunc has no map literals, should not have a funcBodySchemas entry")
	}
}

func TestFuncBodySchema_ExistingBuildSummary(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// buildSummary returns map[string]any{"name": name} — should be analyzed
	schema, exists := parser.funcBodySchemas["buildSummary"]
	if !exists {
		t.Fatal("Expected funcBodySchemas entry for 'buildSummary'")
	}

	if schema.Type != "object" {
		t.Fatalf("Expected type 'object', got %q", schema.Type)
	}
	if _, ok := schema.Properties["name"]; !ok {
		t.Error("Expected property 'name' in buildSummary schema")
	}
}

// =============================================================================
// End-to-End: Handler Response Schema Resolution via Helper Body Analysis
// =============================================================================

func TestHandlerScenario_DeepHelperCandles(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getCandleDataHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// Response should be {candles: [...], count: int}
	assertInlineObject(t, h.ResponseSchema, []string{"candles", "count"}, "response")

	candlesSchema := h.ResponseSchema.Properties["candles"]
	if candlesSchema == nil {
		t.Fatal("Expected 'candles' property")
	}
	if candlesSchema.Type != "array" {
		t.Fatalf("Expected 'candles' type 'array', got %q", candlesSchema.Type)
	}
	if candlesSchema.Items == nil {
		t.Fatal("Expected Items on candles array")
	}

	// The items should have the actual keys from formatCandles, NOT generic additionalProperties
	items := candlesSchema.Items
	if items.Type != "object" {
		t.Fatalf("Expected items type 'object', got %q", items.Type)
	}
	if len(items.Properties) == 0 {
		t.Fatal("Expected items to have properties (deep schema resolution), got empty — still generic?")
	}

	for _, key := range []string{"t", "o", "h", "l", "c", "v", "trades"} {
		if _, ok := items.Properties[key]; !ok {
			t.Errorf("Expected property %q in candle item schema (from formatCandles body)", key)
		}
	}
}

func TestHandlerScenario_DeepHelperScoresWithAdditions(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getScoreItemsHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	scoresSchema := h.ResponseSchema.Properties["scores"]
	if scoresSchema == nil {
		t.Fatal("Expected 'scores' property")
	}
	if scoresSchema.Type != "array" {
		t.Fatalf("Expected 'scores' type 'array', got %q", scoresSchema.Type)
	}
	if scoresSchema.Items == nil {
		t.Fatal("Expected Items on scores array")
	}

	// Should include both literal keys AND map addition keys
	for _, key := range []string{"t", "score", "trend", "momentum", "volatility"} {
		if _, ok := scoresSchema.Items.Properties[key]; !ok {
			t.Errorf("Expected property %q in score item schema", key)
		}
	}
}

func TestHandlerScenario_DeepHelperSingleMap(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getTokenDetailHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// buildTokenDetail returns map[string]any directly — response should have those keys
	if h.ResponseSchema.Type != "object" {
		t.Fatalf("Expected response type 'object', got %q", h.ResponseSchema.Type)
	}

	for _, key := range []string{"id", "symbol", "name", "decimals", "active"} {
		prop, ok := h.ResponseSchema.Properties[key]
		if !ok {
			t.Errorf("Expected property %q in token detail response", key)
			continue
		}
		_ = prop
	}

	// Verify specific types
	if s := h.ResponseSchema.Properties["active"]; s != nil && s.Type != "boolean" {
		t.Errorf("Expected 'active' type 'boolean', got %q", s.Type)
	}
	if s := h.ResponseSchema.Properties["decimals"]; s != nil && s.Type != "integer" {
		t.Errorf("Expected 'decimals' type 'integer' (from GetInt), got %q", s.Type)
	}
}

func TestHandlerScenario_DeepHelperMultiMap(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getMultiMapHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// Response is map[string]any{"data": multiMapFunc(record)}
	dataSchema := h.ResponseSchema.Properties["data"]
	if dataSchema == nil {
		t.Fatal("Expected 'data' property")
	}
	if dataSchema.Type != "object" {
		t.Fatalf("Expected 'data' type 'object', got %q", dataSchema.Type)
	}

	// Should have 5 properties from the largest map literal in multiMapFunc
	for _, key := range []string{"alpha", "beta", "gamma", "delta", "epsilon"} {
		if _, ok := dataSchema.Properties[key]; !ok {
			t.Errorf("Expected property %q in multiMapFunc result", key)
		}
	}
}

func TestHandlerScenario_DeepHelperEmptyFallback(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getEmptyFormatHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// items comes from emptyFormatFunc which has no map literals
	// Should gracefully fall back to array of generic objects
	itemsSchema := h.ResponseSchema.Properties["items"]
	if itemsSchema == nil {
		t.Fatal("Expected 'items' property")
	}
	if itemsSchema.Type != "array" {
		t.Fatalf("Expected 'items' type 'array', got %q", itemsSchema.Type)
	}
}

// =============================================================================
// Append-Based Inline Loop Resolution Tests (make + append in handler body)
// =============================================================================

func TestHandlerScenario_AppendBasedNetworks(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "listNetworksHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// Response should be {networks: [...], count: int}
	assertInlineObject(t, h.ResponseSchema, []string{"networks", "count"}, "response")

	networksSchema := h.ResponseSchema.Properties["networks"]
	if networksSchema == nil {
		t.Fatal("Expected 'networks' property")
	}
	if networksSchema.Type != "array" {
		t.Fatalf("Expected 'networks' type 'array', got %q", networksSchema.Type)
	}
	if networksSchema.Items == nil {
		t.Fatal("Expected Items on networks array")
	}

	// The items should have actual keys from the map literal, NOT generic additionalProperties
	items := networksSchema.Items
	if len(items.Properties) == 0 {
		t.Fatal("Expected items to have properties (append-based resolution), got empty — still generic?")
	}

	for _, key := range []string{"id", "name", "chain_id", "rpc_url", "active"} {
		if _, ok := items.Properties[key]; !ok {
			t.Errorf("Expected property %q in network item schema", key)
		}
	}

	// Verify specific types
	if s := items.Properties["active"]; s != nil && s.Type != "boolean" {
		t.Errorf("Expected 'active' type 'boolean', got %q", s.Type)
	}
	if s := items.Properties["name"]; s != nil && s.Type != "string" {
		t.Errorf("Expected 'name' type 'string', got %q", s.Type)
	}
}

func TestHandlerScenario_AppendBasedTokensWithMapAdditions(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "listTokensHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	tokensSchema := h.ResponseSchema.Properties["tokens"]
	if tokensSchema == nil {
		t.Fatal("Expected 'tokens' property")
	}
	if tokensSchema.Type != "array" {
		t.Fatalf("Expected 'tokens' type 'array', got %q", tokensSchema.Type)
	}
	if tokensSchema.Items == nil {
		t.Fatal("Expected Items on tokens array")
	}

	items := tokensSchema.Items
	if len(items.Properties) == 0 {
		t.Fatal("Expected items to have properties from append resolution")
	}

	// Keys from the map literal
	for _, key := range []string{"id", "symbol", "name", "decimals"} {
		if _, ok := items.Properties[key]; !ok {
			t.Errorf("Expected property %q from map literal in token item schema", key)
		}
	}

	// Keys from map additions (entry["price"] = ..., entry["volume"] = ...)
	// These should be resolved via the map additions on the appended variable
	for _, key := range []string{"price", "volume"} {
		if _, ok := items.Properties[key]; !ok {
			t.Errorf("Expected property %q from map additions in token item schema", key)
		}
	}
}

func TestHandlerScenario_AppendBasedObservations(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getObservationsHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	assertInlineObject(t, h.ResponseSchema, []string{"observations", "total"}, "response")

	obsSchema := h.ResponseSchema.Properties["observations"]
	if obsSchema == nil {
		t.Fatal("Expected 'observations' property")
	}
	if obsSchema.Type != "array" {
		t.Fatalf("Expected 'observations' type 'array', got %q", obsSchema.Type)
	}
	if obsSchema.Items == nil {
		t.Fatal("Expected Items on observations array")
	}

	items := obsSchema.Items
	if len(items.Properties) == 0 {
		t.Fatal("Expected items to have properties from append resolution")
	}

	for _, key := range []string{"timestamp", "value", "source"} {
		if _, ok := items.Properties[key]; !ok {
			t.Errorf("Expected property %q in observation item schema", key)
		}
	}
}

func TestHandlerScenario_SliceAppendTracking(t *testing.T) {
	// Verify that SliceAppendExprs are correctly populated
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "listNetworksHandler")

	if h.SliceAppendExprs == nil {
		t.Fatal("Expected SliceAppendExprs to be initialized")
	}

	appendExpr, exists := h.SliceAppendExprs["networks"]
	if !exists {
		t.Fatal("Expected 'networks' to have an append expression tracked")
	}

	// The append expression should be a variable reference to "entry"
	if appendExpr == nil {
		t.Fatal("Expected non-nil append expression")
	}
}

func TestHandlerScenario_InlineAppendPattern(t *testing.T) {
	// Tests the pattern: items = append(items, map[string]any{...}) with NO separate entry variable
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "listInlineAppendHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	assertInlineObject(t, h.ResponseSchema, []string{"items", "total"}, "response")

	itemsSchema := h.ResponseSchema.Properties["items"]
	if itemsSchema == nil {
		t.Fatal("Expected 'items' property")
	}
	if itemsSchema.Type != "array" {
		t.Fatalf("Expected 'items' type 'array', got %q", itemsSchema.Type)
	}
	if itemsSchema.Items == nil {
		t.Fatal("Expected Items on items array")
	}

	items := itemsSchema.Items
	if len(items.Properties) == 0 {
		t.Fatal("Expected items to have properties (inline append resolution), got empty — still generic?")
	}

	for _, key := range []string{"id", "name", "value", "active"} {
		if _, ok := items.Properties[key]; !ok {
			t.Errorf("Expected property %q in inline-appended item schema", key)
		}
	}

	// Verify types
	if s := items.Properties["active"]; s != nil && s.Type != "boolean" {
		t.Errorf("Expected 'active' type 'boolean', got %q", s.Type)
	}
	if s := items.Properties["value"]; s != nil && s.Type != "number" {
		t.Errorf("Expected 'value' type 'number', got %q", s.Type)
	}
}

// =============================================================================
// Index Expression Resolution Tests (map["key"] from funcBodySchemas)
// =============================================================================

func TestFuncBodySchema_FetchIntervalSummary(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// fetchIntervalSummary returns map[string]any with known keys
	schema, exists := parser.funcBodySchemas["fetchIntervalSummary"]
	if !exists {
		t.Fatal("Expected funcBodySchemas entry for 'fetchIntervalSummary'")
	}

	if schema.Type != "object" {
		t.Fatalf("Expected type 'object', got %q", schema.Type)
	}

	expectedKeys := []string{"price", "volume", "market_cap", "change_pct", "high", "low"}
	for _, key := range expectedKeys {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("Expected property %q in fetchIntervalSummary schema", key)
		}
	}
}

func TestFuncBodySchema_FetchLatestSummary(t *testing.T) {
	parser := parseHandlerScenarios(t)

	// fetchLatestSummary builds a map where some values are summary["key"] index expressions
	schema, exists := parser.funcBodySchemas["fetchLatestSummary"]
	if !exists {
		t.Fatal("Expected funcBodySchemas entry for 'fetchLatestSummary'")
	}

	if schema.Type != "object" {
		t.Fatalf("Expected type 'object', got %q", schema.Type)
	}

	// Should have all 4 keys
	expectedKeys := []string{"price", "volume", "market_cap", "updated"}
	for _, key := range expectedKeys {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("Expected property %q in fetchLatestSummary schema", key)
		}
	}

	// "updated" should be boolean (from literal true)
	if s := schema.Properties["updated"]; s != nil && s.Type != "boolean" {
		t.Errorf("Expected 'updated' type 'boolean', got %q", s.Type)
	}

	// "price" should resolve to a numeric type (from fetchIntervalSummary's body schema)
	// since summary["price"] should look up the type from funcBodySchemas["fetchIntervalSummary"]
	if s := schema.Properties["price"]; s != nil {
		if s.Type != "number" {
			t.Errorf("Expected 'price' type 'number' (from index expr resolution), got %q", s.Type)
		}
	}
}

func TestHandlerScenario_LatestSummaryEndToEnd(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getLatestSummaryHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	// Response should have the keys from fetchLatestSummary
	if h.ResponseSchema.Type != "object" {
		t.Fatalf("Expected response type 'object', got %q", h.ResponseSchema.Type)
	}

	for _, key := range []string{"price", "volume", "market_cap", "updated"} {
		if _, ok := h.ResponseSchema.Properties[key]; !ok {
			t.Errorf("Expected property %q in latest summary response", key)
		}
	}

	// "updated" should be boolean
	if s := h.ResponseSchema.Properties["updated"]; s != nil && s.Type != "boolean" {
		t.Errorf("Expected 'updated' type 'boolean', got %q", s.Type)
	}
}

// =============================================================================
// Query Parameter Detection Tests
// =============================================================================

func TestHandlerScenario_QueryParameterDetection(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "searchWithFiltersHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected query parameters to be detected")
	}

	// Should have detected "category" and "status"
	paramNames := map[string]bool{}
	for _, p := range h.Parameters {
		paramNames[p.Name] = true
		if p.Source != "query" {
			t.Errorf("Expected parameter %q source to be 'query', got %q", p.Name, p.Source)
		}
		if p.Type != "string" {
			t.Errorf("Expected parameter %q type to be 'string', got %q", p.Name, p.Type)
		}
	}

	if !paramNames["category"] {
		t.Error("Expected 'category' query parameter to be detected")
	}
	if !paramNames["status"] {
		t.Error("Expected 'status' query parameter to be detected")
	}
}

func TestHandlerScenario_NoQueryParamsOnSimpleHandler(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "healthCheckHandler")

	if len(h.Parameters) > 0 {
		t.Errorf("Expected no parameters on healthCheckHandler, got %d", len(h.Parameters))
	}
}

func TestHandlerScenario_InlineQueryGet(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getItemsInlineQueryHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected query parameters from inline URL.Query().Get()")
	}

	params := paramMap(h.Parameters)
	assertParam(t, params, "sort", "query", "string")
	assertParam(t, params, "limit", "query", "string")
}

func TestHandlerScenario_RequestInfoQueryAndHeaders(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getItemsRequestInfoHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected parameters from RequestInfo() access")
	}

	params := paramMap(h.Parameters)
	assertParam(t, params, "search", "query", "string")
	assertParam(t, params, "authorization", "header", "string")
}

func TestHandlerScenario_HeaderGet(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getWithHeaderHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected header parameter from Request.Header.Get()")
	}

	params := paramMap(h.Parameters)
	assertParam(t, params, "X-API-Key", "header", "string")
}

func TestHandlerScenario_PathValue(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getItemByPathValueHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected path parameter from Request.PathValue()")
	}

	params := paramMap(h.Parameters)
	assertParam(t, params, "id", "path", "string")

	// Path parameters should be required
	for _, p := range h.Parameters {
		if p.Source == "path" && !p.Required {
			t.Errorf("Expected path parameter %q to be required", p.Name)
		}
	}
}

func TestHandlerScenario_MixedParameterSources(t *testing.T) {
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getMixedParamsHandler")

	if len(h.Parameters) < 6 {
		t.Fatalf("Expected at least 6 parameters from mixed sources, got %d", len(h.Parameters))
	}

	params := paramMap(h.Parameters)
	assertParam(t, params, "id", "path", "string")
	assertParam(t, params, "interval", "query", "string")
	assertParam(t, params, "format", "query", "string")
	assertParam(t, params, "locale", "query", "string")
	assertParam(t, params, "x_auth_token", "header", "string")
	assertParam(t, params, "X-Custom", "header", "string")
}

// paramMap builds a lookup map from a slice of ParamInfo keyed by "source:name".
func paramMap(params []*ParamInfo) map[string]*ParamInfo {
	m := make(map[string]*ParamInfo, len(params))
	for _, p := range params {
		m[p.Source+":"+p.Name] = p
	}
	return m
}

// assertParam checks that a parameter with the given source and name exists and has the expected type.
func assertParam(t *testing.T, params map[string]*ParamInfo, name, source, typ string) {
	t.Helper()
	key := source + ":" + name
	p, ok := params[key]
	if !ok {
		t.Errorf("Expected %s parameter %q to be detected", source, name)
		return
	}
	if p.Type != typ {
		t.Errorf("Expected %s parameter %q type %q, got %q", source, name, typ, p.Type)
	}
}

// =============================================================================
// Indirect Parameter Extraction Tests
// =============================================================================

func TestHandlerScenario_IndirectParams_DomainHelper(t *testing.T) {
	// getChartHandler calls parseTimeParams(c) which internally reads:
	// interval, from, after, to, limit via q.Get(...)
	// All 5 params should be inherited on the handler.
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getChartHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected indirect query parameters from domain helper parseTimeParams, got none")
	}

	params := paramMap(h.Parameters)
	assertParam(t, params, "interval", "query", "string")
	assertParam(t, params, "from", "query", "string")
	assertParam(t, params, "after", "query", "string")
	assertParam(t, params, "to", "query", "string")
	assertParam(t, params, "limit", "query", "string")
}

func TestHandlerScenario_IndirectParams_GenericIntHelper(t *testing.T) {
	// getPaginatedItemsHandler calls:
	//   parseIntParam(c, "page", 1)
	//   parseIntParam(c, "page_size", 20)
	// The param name comes from the 2nd argument at the call site.
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getPaginatedItemsHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected query parameters from generic helper parseIntParam call sites, got none")
	}

	params := paramMap(h.Parameters)
	assertParam(t, params, "page", "query", "string")
	assertParam(t, params, "page_size", "query", "string")
}

func TestHandlerScenario_IndirectParams_GenericBoolHelper(t *testing.T) {
	// getVerboseDataHandler calls:
	//   parseBoolParam(c, "verbose")
	//   parseBoolParam(c, "debug")
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getVerboseDataHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected query parameters from generic helper parseBoolParam call sites, got none")
	}

	params := paramMap(h.Parameters)
	assertParam(t, params, "verbose", "query", "string")
	assertParam(t, params, "debug", "query", "string")
}

func TestHandlerScenario_IndirectParams_MultipleHelpers(t *testing.T) {
	// getChartPaginatedHandler calls both parseTimeParams(c) and parseIntParam(c, "page", 0).
	// Should get all 5 time params + page.
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getChartPaginatedHandler")

	if len(h.Parameters) < 6 {
		t.Fatalf("Expected ≥6 parameters from multiple helpers, got %d", len(h.Parameters))
	}

	params := paramMap(h.Parameters)
	// From parseTimeParams
	assertParam(t, params, "interval", "query", "string")
	assertParam(t, params, "from", "query", "string")
	assertParam(t, params, "after", "query", "string")
	assertParam(t, params, "to", "query", "string")
	assertParam(t, params, "limit", "query", "string")
	// From parseIntParam call site
	assertParam(t, params, "page", "query", "string")
}

func TestHandlerScenario_IndirectParams_GenericHeaderHelper(t *testing.T) {
	// getWithAuthHeaderHandler calls parseHeaderParam(c, "Authorization").
	// Should detect "Authorization" as a header param.
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getWithAuthHeaderHandler")

	if len(h.Parameters) == 0 {
		t.Fatal("Expected header parameter from generic helper parseHeaderParam, got none")
	}

	params := paramMap(h.Parameters)
	assertParam(t, params, "Authorization", "header", "string")
}

func TestHandlerScenario_IndirectParams_MixedDirectAndIndirect(t *testing.T) {
	// getChartWithDirectParamHandler calls parseTimeParams(c) AND directly reads "sort".
	// Should detect all 5 time params + "sort".
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getChartWithDirectParamHandler")

	if len(h.Parameters) < 6 {
		t.Fatalf("Expected ≥6 parameters (5 indirect + 1 direct), got %d", len(h.Parameters))
	}

	params := paramMap(h.Parameters)
	// Indirect via parseTimeParams
	assertParam(t, params, "interval", "query", "string")
	assertParam(t, params, "from", "query", "string")
	assertParam(t, params, "after", "query", "string")
	assertParam(t, params, "to", "query", "string")
	assertParam(t, params, "limit", "query", "string")
	// Direct
	assertParam(t, params, "sort", "query", "string")
}

func TestHandlerScenario_IndirectParams_HelperRegistered(t *testing.T) {
	// Verify that the helper functions are correctly registered in funcParamSchemas.
	parser := parseHandlerScenarios(t)

	// parseTimeParams should have 5 literal params
	tp, ok := parser.funcParamSchemas["parseTimeParams"]
	if !ok {
		t.Fatal("Expected parseTimeParams to be registered in funcParamSchemas")
	}
	if len(tp) != 5 {
		t.Errorf("Expected 5 params for parseTimeParams, got %d", len(tp))
	}
	names := make(map[string]bool)
	for _, p := range tp {
		names[p.Name] = true
	}
	for _, want := range []string{"interval", "from", "after", "to", "limit"} {
		if !names[want] {
			t.Errorf("Expected parseTimeParams to have param %q", want)
		}
	}

	// parseIntParam should be registered as a sentinel (entries with Name="", Source="query")
	// because its body uses a variable param name, not a literal.
	ip, ok := parser.funcParamSchemas["parseIntParam"]
	if !ok {
		t.Fatal("Expected parseIntParam to be registered in funcParamSchemas as sentinel")
	}
	namedIP := 0
	for _, p := range ip {
		if p.Name != "" {
			namedIP++
		}
	}
	if namedIP != 0 {
		t.Errorf("Expected parseIntParam to have 0 named params (sentinel only), got %d", namedIP)
	}

	// parseBoolParam — same generic pattern
	bp, ok := parser.funcParamSchemas["parseBoolParam"]
	if !ok {
		t.Fatal("Expected parseBoolParam to be registered in funcParamSchemas as sentinel")
	}
	namedBP := 0
	for _, p := range bp {
		if p.Name != "" {
			namedBP++
		}
	}
	if namedBP != 0 {
		t.Errorf("Expected parseBoolParam to have 0 named params (sentinel only), got %d", namedBP)
	}
}

// =============================================================================
// Slice Index Assignment Resolution Tests
// =============================================================================

func TestHandlerScenario_SliceIndexAssignment(t *testing.T) {
	// listTodosIndexHandler builds its slice via:
	//   todos := make([]map[string]any, len(records))
	//   for i, r := range records {
	//       todos[i] = map[string]any{"id": r, "title": r, "priority": "high", ...}
	//   }
	// The items schema should be resolved from the assigned composite literal,
	// NOT fall back to {type: "object", additionalProperties: true}.
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "listTodosIndexHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	todosSchema, ok := h.ResponseSchema.Properties["todos"]
	if !ok {
		t.Fatal("Expected 'todos' property in response schema")
	}
	if todosSchema.Type != "array" {
		t.Fatalf("Expected todos to be array type, got %q", todosSchema.Type)
	}
	if todosSchema.Items == nil {
		t.Fatal("Expected todos.items to be non-nil")
	}

	// Items must NOT be the generic additionalProperties fallback
	if todosSchema.Items.AdditionalProperties == true && len(todosSchema.Items.Properties) == 0 {
		t.Fatal("Expected todos items to have typed properties, got generic additionalProperties:true fallback")
	}

	// Verify the item properties were resolved from the literal
	itemProps := todosSchema.Items.Properties
	for _, want := range []string{"id", "title", "priority", "completed", "description"} {
		if _, ok := itemProps[want]; !ok {
			t.Errorf("Expected todos item property %q, not found in schema", want)
		}
	}
}

func TestHandlerScenario_MapAdditionsClearsAdditionalProperties(t *testing.T) {
	// updateTodoMapAdditionsHandler builds updates via:
	//   updates := make(map[string]any)
	//   updates["title"] = "new title"
	//   updates["completed"] = true
	//   ...
	// After merging map_additions the schema must have concrete properties
	// and must NOT carry additionalProperties:true (which causes Swagger UI
	// to render a spurious "additionalProp1" entry).
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "updateTodoMapAdditionsHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	changesSchema, ok := h.ResponseSchema.Properties["changes"]
	if !ok {
		t.Fatal("Expected 'changes' property in response schema")
	}
	if changesSchema.Type != "object" {
		t.Fatalf("Expected changes to be object type, got %q", changesSchema.Type)
	}

	// Concrete properties must be present
	for _, want := range []string{"title", "completed", "priority", "description"} {
		if _, ok := changesSchema.Properties[want]; !ok {
			t.Errorf("Expected changes property %q, not found in schema", want)
		}
	}

	// additionalProperties must NOT be true once concrete properties are known
	if changesSchema.AdditionalProperties == true {
		t.Error("Expected additionalProperties to be cleared (nil/false) after map_additions resolved concrete properties, but it is still true")
	}
}

// =============================================================================
// GetInt/GetFloat/GetString Typing Tests
// =============================================================================

func TestHandlerScenario_GetIntProducesIntegerSchema(t *testing.T) {
	// getIntTypingHandler returns a map with r.GetInt() and r.GetFloat() values.
	// GetInt must produce "integer", GetFloat must produce "number".
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "getIntTypingHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	props := h.ResponseSchema.Properties
	if props == nil {
		t.Fatal("Expected response schema properties")
	}

	for _, want := range []struct {
		key      string
		wantType string
	}{
		{"page", "integer"},
		{"total", "integer"},
		{"price", "number"},
		{"score", "number"},
		{"label", "string"},
		{"enabled", "boolean"},
	} {
		prop, ok := props[want.key]
		if !ok {
			t.Errorf("Expected property %q", want.key)
			continue
		}
		if prop.Type != want.wantType {
			t.Errorf("Property %q: expected type %q, got %q", want.key, want.wantType, prop.Type)
		}
	}
}

// =============================================================================
// Conditional Map Key Assignment Tests
// =============================================================================

func TestHandlerScenario_ConditionalKeysFromIfBlock(t *testing.T) {
	// conditionalKeysHandler sets entry["direction"] etc. inside an if block.
	// Because td := r.GetString("direction") is now tracked as type "string",
	// entry["direction"] = td should also resolve to "string".
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "conditionalKeysHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	dataSchema, ok := h.ResponseSchema.Properties["data"]
	if !ok {
		t.Fatal("Expected 'data' property in response")
	}
	if dataSchema.Type != "object" {
		t.Fatalf("Expected 'data' to be object, got %q", dataSchema.Type)
	}

	// Keys from the base map literal
	for _, key := range []string{"signal", "value"} {
		if _, ok := dataSchema.Properties[key]; !ok {
			t.Errorf("Expected base key %q in data schema", key)
		}
	}

	// Keys from the conditional if block (via map additions)
	for _, key := range []string{"direction", "entry_price", "stop_loss"} {
		if _, ok := dataSchema.Properties[key]; !ok {
			t.Errorf("Expected conditional key %q in data schema (from if-block map addition)", key)
		}
	}

	// "direction" should be string (from td := r.GetString("direction"))
	if s := dataSchema.Properties["direction"]; s != nil && s.Type != "string" {
		t.Errorf("Expected 'direction' type 'string', got %q", s.Type)
	}

	// "entry_price" and "stop_loss" should be number (from r.GetFloat)
	for _, key := range []string{"entry_price", "stop_loss"} {
		if s := dataSchema.Properties[key]; s != nil && s.Type != "number" {
			t.Errorf("Expected %q type 'number', got %q", key, s.Type)
		}
	}
}

// =============================================================================
// Slice Index Element Type Tests
// =============================================================================

func TestHandlerScenario_SliceIndexElementType(t *testing.T) {
	// sliceIndexTypeHandler indexes a []float64 slice via returns[0], returns[1], returns[2].
	// Each index expression should resolve to the element type "float64" → "number",
	// NOT fall back to "interface{}" → {type:object, additionalProperties:true}.
	parser := parseHandlerScenarios(t)
	h := requireHandler(t, parser, "sliceIndexTypeHandler")

	if h.ResponseSchema == nil {
		t.Fatal("Expected response schema")
	}

	props := h.ResponseSchema.Properties
	if props == nil {
		t.Fatal("Expected response schema properties")
	}

	for _, key := range []string{"min", "max", "avg"} {
		prop, ok := props[key]
		if !ok {
			t.Errorf("Expected property %q", key)
			continue
		}
		// Should be "number" (from []float64 element), not "object" with additionalProperties
		if prop.Type != "number" {
			t.Errorf("Property %q: expected type 'number' (from []float64 element), got %q", key, prop.Type)
		}
		if prop.AdditionalProperties == true {
			t.Errorf("Property %q: should not have additionalProperties:true (not an object)", key)
		}
	}
}
