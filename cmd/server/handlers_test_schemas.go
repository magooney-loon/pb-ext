package main

// API_SOURCE

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// =============================================================================
// Struct responses — tests struct schema generation, nested structs, $ref, etc.
// =============================================================================

// GeoCoordinate represents a lat/lng pair
type GeoCoordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Address is a nested struct used inside other structs
type Address struct {
	Street     string        `json:"street"`
	City       string        `json:"city"`
	State      string        `json:"state,omitempty"`
	PostalCode string        `json:"postal_code"`
	Country    string        `json:"country"`
	Geo        GeoCoordinate `json:"geo"`
}

// ContactInfo has pointer fields and slices of primitives
type ContactInfo struct {
	Email     string   `json:"email"`
	Phone     *string  `json:"phone,omitempty"`
	Website   *string  `json:"website,omitempty"`
	SocialIDs []string `json:"social_ids,omitempty"`
}

// OrderItem is a line-item inside an order — tests nested struct in slice
type OrderItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Subtotal    float64 `json:"subtotal"`
}

// PaymentInfo tests typed maps and any fields
type PaymentInfo struct {
	Method        string            `json:"method"`
	TransactionID string            `json:"transaction_id"`
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	Headers       map[string]string `json:"headers,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty"`
}

// OrderResponse is the big deeply-nested struct:
//
//	OrderResponse → Address (→ GeoCoordinate), []OrderItem, PaymentInfo (→ map fields)
type OrderResponse struct {
	ID              string      `json:"id"`
	Status          string      `json:"status"`
	Customer        string      `json:"customer"`
	ShippingAddress Address     `json:"shipping_address"`
	BillingAddress  *Address    `json:"billing_address,omitempty"`
	Items           []OrderItem `json:"items"`
	Payment         PaymentInfo `json:"payment"`
	TotalAmount     float64     `json:"total_amount"`
	Notes           *string     `json:"notes,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// CreateOrderRequest tests json.Decode request body with nested structs
type CreateOrderRequest struct {
	CustomerID      string      `json:"customer_id"`
	ShippingAddress Address     `json:"shipping_address"`
	BillingAddress  *Address    `json:"billing_address,omitempty"`
	Items           []OrderItem `json:"items"`
	PaymentMethod   string      `json:"payment_method"`
	Notes           *string     `json:"notes,omitempty"`
	CouponCode      *string     `json:"coupon_code,omitempty"`
}

// AnalyticsEvent tests interface{}/any fields inside structs
type AnalyticsEvent struct {
	EventID    string         `json:"event_id"`
	EventType  string         `json:"event_type"`
	Timestamp  time.Time      `json:"timestamp"`
	UserID     *string        `json:"user_id,omitempty"`
	SessionID  string         `json:"session_id"`
	Properties map[string]any `json:"properties,omitempty"`
	Context    any            `json:"context,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
}

// PaginationMeta tests a small helper struct used inline
type PaginationMeta struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	TotalItems int  `json:"total_items"`
	TotalPages int  `json:"total_pages"`
	HasMore    bool `json:"has_more"`
}

// UserProfile tests a flat struct with various primitive types
type UserProfile struct {
	ID          string      `json:"id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"display_name"`
	Email       string      `json:"email"`
	AvatarURL   *string     `json:"avatar_url,omitempty"`
	Bio         *string     `json:"bio,omitempty"`
	IsVerified  bool        `json:"is_verified"`
	Reputation  int         `json:"reputation"`
	Balance     float64     `json:"balance"`
	JoinedAt    time.Time   `json:"joined_at"`
	Contact     ContactInfo `json:"contact"`
}

// TimeseriesPoint tests numeric-heavy structs (like charting data)
type TimeseriesPoint struct {
	Timestamp int64   `json:"timestamp"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
}

// IndicatorValues tests map[string]float64 — typed map values
type IndicatorValues struct {
	TokenID   string             `json:"token_id"`
	Interval  string             `json:"interval"`
	Values    map[string]float64 `json:"values"`
	Signals   map[string]string  `json:"signals"`
	Computed  map[string]int     `json:"computed"`
	UpdatedAt time.Time          `json:"updated_at"`
}

// =============================================================================
// 1. Deep nested struct response (struct → struct → struct)
// =============================================================================

// API_DESC Get order details with full shipping, billing, items, and payment info
// API_TAGS Orders
func getOrderHandler(c *core.RequestEvent) error {
	orderID := c.Request.PathValue("id")

	phone := "+1-555-0100"
	notes := "Leave at front door"
	resp := OrderResponse{
		ID:       orderID,
		Status:   "shipped",
		Customer: "cust_abc123",
		ShippingAddress: Address{
			Street:     "123 Main St",
			City:       "Portland",
			State:      "OR",
			PostalCode: "97201",
			Country:    "US",
			Geo:        GeoCoordinate{Latitude: 45.5155, Longitude: -122.6789},
		},
		Items: []OrderItem{
			{ProductID: "prod_1", ProductName: "Widget", Quantity: 2, UnitPrice: 9.99, Subtotal: 19.98},
		},
		Payment: PaymentInfo{
			Method:        "card",
			TransactionID: "txn_xyz",
			Amount:        19.98,
			Currency:      "USD",
			Headers:       map[string]string{"X-Idempotency-Key": "abc"},
			Metadata:      map[string]any{"risk_score": 0.12, "processor": "stripe", "approved": true},
		},
		Notes:       &notes,
		TotalAmount: 19.98,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_ = phone

	return c.JSON(http.StatusOK, resp)
}

// =============================================================================
// 2. Create with nested struct request body (json.Decode path)
// =============================================================================

// API_DESC Create a new order with shipping address, items, and payment
// API_TAGS Orders
func createOrderHandler(c *core.RequestEvent) error {
	var req CreateOrderRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid request body"})
	}

	if len(req.Items) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "At least one item is required"})
	}

	var total float64
	for _, item := range req.Items {
		total += item.Subtotal
	}

	resp := OrderResponse{
		ID:              "ord_new_123",
		Status:          "pending",
		Customer:        req.CustomerID,
		ShippingAddress: req.ShippingAddress,
		BillingAddress:  req.BillingAddress,
		Items:           req.Items,
		Payment: PaymentInfo{
			Method:   req.PaymentMethod,
			Amount:   total,
			Currency: "USD",
		},
		Notes:       req.Notes,
		TotalAmount: total,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return c.JSON(http.StatusCreated, resp)
}

// =============================================================================
// 3. Array of structs response
// =============================================================================

// API_DESC List all orders for the authenticated user
// API_TAGS Orders
func listOrdersHandler(c *core.RequestEvent) error {
	orders := []OrderResponse{
		{
			ID:     "ord_001",
			Status: "delivered",
			ShippingAddress: Address{
				Street: "456 Oak Ave", City: "Seattle", PostalCode: "98101", Country: "US",
				Geo: GeoCoordinate{Latitude: 47.6062, Longitude: -122.3321},
			},
			Items:       []OrderItem{{ProductID: "p1", ProductName: "Gadget", Quantity: 1, UnitPrice: 49.99, Subtotal: 49.99}},
			Payment:     PaymentInfo{Method: "card", Amount: 49.99, Currency: "USD"},
			TotalAmount: 49.99,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	return c.JSON(http.StatusOK, orders)
}

// =============================================================================
// 4. Struct with typed maps (map[string]float64, map[string]string, map[string]int)
// =============================================================================

// API_DESC Get technical indicator values for a token
// API_TAGS Analytics
func getIndicatorsHandler(c *core.RequestEvent) error {
	tokenID := c.Request.PathValue("id")

	resp := IndicatorValues{
		TokenID:  tokenID,
		Interval: "1h",
		Values: map[string]float64{
			"rsi_14":    62.5,
			"macd":      0.0023,
			"bollinger": 1.05,
			"atr_14":    0.0089,
		},
		Signals: map[string]string{
			"rsi_14": "neutral",
			"macd":   "bullish",
		},
		Computed: map[string]int{
			"candles_analyzed": 500,
			"signals_fired":    12,
		},
		UpdatedAt: time.Now(),
	}

	return c.JSON(http.StatusOK, resp)
}

// =============================================================================
// 5. Struct with any/interface{} fields
// =============================================================================

// API_DESC Track an analytics event with arbitrary properties
// API_TAGS Analytics
func trackEventHandler(c *core.RequestEvent) error {
	var req AnalyticsEvent
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid event payload"})
	}

	req.EventID = "evt_" + time.Now().Format("20060102150405")
	req.Timestamp = time.Now()

	return c.JSON(http.StatusCreated, req)
}

// =============================================================================
// 6. Inline map literal with deeply nested maps
// =============================================================================

// API_DESC Get full system diagnostics with nested subsystem details
// API_TAGS System
func getDiagnosticsHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"status":  "operational",
		"version": "2.1.0",
		"uptime":  86400,
		"memory": map[string]any{
			"allocated_mb": 128,
			"system_mb":    256,
			"gc_cycles":    42,
			"heap_objects": 150000,
		},
		"database": map[string]any{
			"connected":      true,
			"pool_size":      10,
			"active_queries": 3,
			"latency_ms":     1.2,
		},
		"cache": map[string]any{
			"hit_rate":  0.95,
			"entries":   4200,
			"evictions": 15,
			"memory_mb": 32,
		},
		"workers": map[string]any{
			"active":    8,
			"idle":      2,
			"completed": 15000,
			"failed":    3,
		},
	})
}

// =============================================================================
// 7. User profile response (flat struct + nested ContactInfo)
// =============================================================================

// API_DESC Get user profile with contact information
// API_TAGS Users
func getUserProfileHandler(c *core.RequestEvent) error {
	userID := c.Request.PathValue("id")

	website := "https://example.com"
	bio := "Software engineer"
	avatar := "https://avatars.example.com/u/123"
	resp := UserProfile{
		ID:          userID,
		Username:    "johndoe",
		DisplayName: "John Doe",
		Email:       "john@example.com",
		AvatarURL:   &avatar,
		Bio:         &bio,
		IsVerified:  true,
		Reputation:  1250,
		Balance:     99.50,
		JoinedAt:    time.Now().AddDate(-1, 0, 0),
		Contact: ContactInfo{
			Email:     "john@example.com",
			Phone:     nil,
			Website:   &website,
			SocialIDs: []string{"twitter:johndoe", "github:johndoe"},
		},
	}

	return c.JSON(http.StatusOK, resp)
}

// =============================================================================
// 8. Paginated list with struct wrapper — inline map literal + struct array
// =============================================================================

// API_DESC Search users with pagination
// API_TAGS Users
func searchUsersHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"data": []UserProfile{
			{ID: "u1", Username: "alice", DisplayName: "Alice", Email: "alice@example.com", IsVerified: true, Reputation: 800, Balance: 50.0, JoinedAt: time.Now()},
			{ID: "u2", Username: "bob", DisplayName: "Bob", Email: "bob@example.com", IsVerified: false, Reputation: 120, Balance: 10.0, JoinedAt: time.Now()},
		},
		"pagination": PaginationMeta{
			Page:       1,
			PerPage:    20,
			TotalItems: 2,
			TotalPages: 1,
			HasMore:    false,
		},
	})
}

// =============================================================================
// 9. Timeseries response — array of numeric-heavy structs
// =============================================================================

// API_DESC Get OHLCV candlestick data for a token
// API_TAGS Analytics
func getCandlestickHandler(c *core.RequestEvent) error {
	data := []TimeseriesPoint{
		{Timestamp: time.Now().Add(-2 * time.Hour).Unix(), Open: 1.0, High: 1.05, Low: 0.98, Close: 1.02, Volume: 50000},
		{Timestamp: time.Now().Add(-1 * time.Hour).Unix(), Open: 1.02, High: 1.08, Low: 1.01, Close: 1.07, Volume: 62000},
		{Timestamp: time.Now().Unix(), Open: 1.07, High: 1.10, Low: 1.05, Close: 1.09, Volume: 48000},
	}

	return c.JSON(http.StatusOK, data)
}

// =============================================================================
// 10. Pure map[string]string variable response (typed map, not literal)
// =============================================================================

// API_DESC Get configuration key-value pairs
// API_TAGS System
func getConfigHandler(c *core.RequestEvent) error {
	config := map[string]string{
		"log_level":       "info",
		"max_connections": "100",
		"region":          "us-west-2",
		"feature_flags":   "dark_mode,beta_api",
	}

	return c.JSON(http.StatusOK, config)
}

// =============================================================================
// 11. Mixed: inline map with booleans, ints, strings, nested literals
// =============================================================================

// API_DESC Get feature flags and rate limit configuration
// API_TAGS System
func getFeatureFlagsHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"flags": map[string]any{
			"dark_mode":       true,
			"beta_api":        false,
			"new_dashboard":   true,
			"max_upload_mb":   50,
			"allowed_origins": "*.example.com",
		},
		"rate_limits": map[string]any{
			"requests_per_minute": 60,
			"burst_size":          10,
			"enabled":             true,
		},
		"maintenance":  false,
		"announced_at": "2025-01-15T00:00:00Z",
	})
}

// =============================================================================
// 12. Returning a map[string]any variable (not literal) — the original bug case
// =============================================================================

// API_DESC Get aggregated platform statistics
// API_TAGS Analytics
func getPlatformStatsHandler(c *core.RequestEvent) error {
	result := map[string]any{
		"total_users":     15000,
		"active_today":    1200,
		"revenue":         89432.50,
		"top_country":     "US",
		"avg_session_min": 12.5,
	}

	// Simulate building the map dynamically
	result["computed_at"] = time.Now().Format(time.RFC3339)
	result["cached"] = true

	return c.JSON(http.StatusOK, result)
}
