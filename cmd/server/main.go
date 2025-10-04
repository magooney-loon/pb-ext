package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"strconv"

	"time"

	app "github.com/magooney-loon/pb-ext/core"
	"github.com/magooney-loon/pb-ext/core/server"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Request/Response types for API documentation testing
type CreateUserRequest struct {
	Name     string            `json:"name" validate:"required,min=2,max=100"`
	Email    string            `json:"email" validate:"required,email"`
	Age      int               `json:"age" validate:"min=18,max=120"`
	Tags     []string          `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Active   bool              `json:"active"`
}

type UpdateUserRequest struct {
	Name     *string           `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Email    *string           `json:"email,omitempty" validate:"omitempty,email"`
	Age      *int              `json:"age,omitempty" validate:"omitempty,min=18,max=120"`
	Tags     []string          `json:"tags,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Active   *bool             `json:"active,omitempty"`
}

type UserResponse struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Email     string            `json:"email"`
	Age       int               `json:"age"`
	Tags      []string          `json:"tags"`
	Metadata  map[string]string `json:"metadata"`
	Active    bool              `json:"active"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type UsersListResponse struct {
	Users      []UserResponse `json:"users"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
	TotalPages int            `json:"total_pages"`
}

type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    int               `json:"code"`
	Details map[string]string `json:"details,omitempty"`
}

type SearchRequest struct {
	Query   string   `json:"query" validate:"required,min=1"`
	Filters []string `json:"filters,omitempty"`
	SortBy  string   `json:"sort_by,omitempty"`
	Order   string   `json:"order,omitempty" validate:"omitempty,oneof=asc desc"`
}

type AnalyticsResponse struct {
	TotalUsers   int                    `json:"total_users"`
	ActiveUsers  int                    `json:"active_users"`
	UsersByAge   map[string]int         `json:"users_by_age"`
	TagStats     map[string]int         `json:"tag_stats"`
	CreatedToday int                    `json:"created_today"`
	LastActivity time.Time              `json:"last_activity"`
	Trends       map[string]interface{} `json:"trends"`
}

func main() {
	devMode := flag.Bool("dev", false, "Run in developer mode")
	flag.Parse()

	initApp(*devMode)
}

func initApp(devMode bool) {
	var srv *app.Server
	if devMode {
		srv = app.New(app.InDeveloperMode())
		log.Println("🔧 Developer mode enabled")
	} else {
		srv = app.New(app.InNormalMode())
		log.Println("🚀 Production mode")
	}

	app.SetupLogging(srv)

	registerCollections(srv.App())
	registerRoutes(srv.App())
	registerJobs(srv.App())

	srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
		app.SetupRecovery(srv.App(), e)
		return e.Next()
	})

	if err := srv.Start(); err != nil {
		srv.App().Logger().Error("Fatal application error",
			"error", err,
			"uptime", srv.Stats().StartTime,
			"total_requests", srv.Stats().TotalRequests.Load(),
			"active_connections", srv.Stats().ActiveConnections.Load(),
			"last_request_time", srv.Stats().LastRequestTime.Load(),
		)
		log.Fatal(err)
	}
}

func registerCollections(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := exampleCollection(e.App); err != nil {
			app.Logger().Error("Failed to create example collection", "error", err)
		}
		return e.Next()
	})
}

func registerRoutes(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		router := server.EnableAutoDocumentation(e)

		// Simple time endpoint
		router.GET("/api/time", timeHandler)

		// Complex user management endpoints
		router.GET("/api/users", getUsersHandler)
		router.GET("/api/users/{id}", getUserHandler)
		router.POST("/api/users", createUserHandler)
		router.PUT("/api/users/{id}", updateUserHandler)
		router.DELETE("/api/users/{id}", deleteUserHandler)

		// Advanced search endpoint
		router.POST("/api/users/search", searchUsersHandler)

		// Analytics endpoint
		router.GET("/api/analytics", getAnalyticsHandler)

		return e.Next()
	})
}

func registerJobs(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := helloJob(app); err != nil {
			return err
		}

		return e.Next()
	})
}

func timeHandler(c *core.RequestEvent) error {
	now := time.Now()
	return c.JSON(http.StatusOK, map[string]any{
		"time": map[string]string{
			"iso":       now.Format(time.RFC3339),
			"unix":      strconv.FormatInt(now.Unix(), 10),
			"unix_nano": strconv.FormatInt(now.UnixNano(), 10),
			"utc":       now.UTC().Format(time.RFC3339),
		},
	})
}

// getUsersHandler handles GET /api/users with query parameters
func getUsersHandler(c *core.RequestEvent) error {
	// Parse query parameters
	page := 1
	perPage := 10
	search := ""

	if p := c.Request.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if pp := c.Request.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
			perPage = parsed
		}
	}

	search = c.Request.URL.Query().Get("search")
	_ = search // Use search variable to avoid unused variable error

	// Mock data generation
	totalUsers := 50
	users := make([]UserResponse, 0, perPage)

	for i := 0; i < perPage && (page-1)*perPage+i < totalUsers; i++ {
		userID := (page-1)*perPage + i + 1
		users = append(users, UserResponse{
			ID:        strconv.Itoa(userID),
			Name:      "User " + strconv.Itoa(userID),
			Email:     "user" + strconv.Itoa(userID) + "@example.com",
			Age:       20 + (userID % 50),
			Tags:      []string{"tag1", "tag2"},
			Metadata:  map[string]string{"role": "user", "department": "engineering"},
			Active:    userID%2 == 0,
			CreatedAt: time.Now().AddDate(0, 0, -userID),
			UpdatedAt: time.Now().AddDate(0, 0, -userID/2),
		})
	}

	totalPages := (totalUsers + perPage - 1) / perPage

	response := UsersListResponse{
		Users:      users,
		Total:      totalUsers,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

// getUserHandler handles GET /api/users/{id}
func getUserHandler(c *core.RequestEvent) error {
	userID := c.Request.PathValue("id")

	if userID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "User ID is required",
			Code:  400,
		})
	}

	// Mock user data
	user := UserResponse{
		ID:        userID,
		Name:      "User " + userID,
		Email:     "user" + userID + "@example.com",
		Age:       25,
		Tags:      []string{"admin", "developer"},
		Metadata:  map[string]string{"role": "admin", "department": "engineering"},
		Active:    true,
		CreatedAt: time.Now().AddDate(0, 0, -30),
		UpdatedAt: time.Now().AddDate(0, 0, -1),
	}

	return c.JSON(http.StatusOK, user)
}

// createUserHandler handles POST /api/users
func createUserHandler(c *core.RequestEvent) error {
	var req CreateUserRequest

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    400,
			Details: map[string]string{"binding_error": err.Error()},
		})
	}

	// Mock validation
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Name is required",
			Code:    400,
			Details: map[string]string{"field": "name"},
		})
	}

	if req.Email == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Email is required",
			Code:    400,
			Details: map[string]string{"field": "email"},
		})
	}

	// Mock user creation
	newUser := UserResponse{
		ID:        strconv.FormatInt(time.Now().UnixNano(), 10),
		Name:      req.Name,
		Email:     req.Email,
		Age:       req.Age,
		Tags:      req.Tags,
		Metadata:  req.Metadata,
		Active:    req.Active,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return c.JSON(http.StatusCreated, newUser)
}

// updateUserHandler handles PUT /api/users/{id}
func updateUserHandler(c *core.RequestEvent) error {
	userID := c.Request.PathValue("id")

	if userID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "User ID is required",
			Code:  400,
		})
	}

	var req UpdateUserRequest

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Code:    400,
			Details: map[string]string{"binding_error": err.Error()},
		})
	}

	// Mock updated user
	updatedUser := UserResponse{
		ID:        userID,
		Name:      "Updated User " + userID,
		Email:     "updated" + userID + "@example.com",
		Age:       30,
		Tags:      []string{"updated", "user"},
		Metadata:  map[string]string{"role": "updated", "department": "updated"},
		Active:    true,
		CreatedAt: time.Now().AddDate(0, 0, -30),
		UpdatedAt: time.Now(),
	}

	// Apply updates from request
	if req.Name != nil {
		updatedUser.Name = *req.Name
	}
	if req.Email != nil {
		updatedUser.Email = *req.Email
	}
	if req.Age != nil {
		updatedUser.Age = *req.Age
	}
	if req.Tags != nil {
		updatedUser.Tags = req.Tags
	}
	if req.Metadata != nil {
		updatedUser.Metadata = req.Metadata
	}
	if req.Active != nil {
		updatedUser.Active = *req.Active
	}

	return c.JSON(http.StatusOK, updatedUser)
}

// deleteUserHandler handles DELETE /api/users/{id}
func deleteUserHandler(c *core.RequestEvent) error {
	userID := c.Request.PathValue("id")

	if userID == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "User ID is required",
			Code:  400,
		})
	}

	// Mock deletion response
	response := map[string]interface{}{
		"message":    "User deleted successfully",
		"deleted_id": userID,
		"deleted_at": time.Now().Format(time.RFC3339),
	}

	return c.JSON(http.StatusOK, response)
}

// searchUsersHandler handles POST /api/users/search
func searchUsersHandler(c *core.RequestEvent) error {
	var req SearchRequest

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid search request",
			Code:    400,
			Details: map[string]string{"binding_error": err.Error()},
		})
	}

	if req.Query == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Search query is required",
			Code:    400,
			Details: map[string]string{"field": "query"},
		})
	}

	// Mock search results
	users := []UserResponse{
		{
			ID:        "search1",
			Name:      "Search Result 1",
			Email:     "search1@example.com",
			Age:       28,
			Tags:      []string{"searched", "found"},
			Metadata:  map[string]string{"relevance": "high"},
			Active:    true,
			CreatedAt: time.Now().AddDate(0, 0, -10),
			UpdatedAt: time.Now().AddDate(0, 0, -2),
		},
		{
			ID:        "search2",
			Name:      "Search Result 2",
			Email:     "search2@example.com",
			Age:       32,
			Tags:      []string{"searched", "matched"},
			Metadata:  map[string]string{"relevance": "medium"},
			Active:    false,
			CreatedAt: time.Now().AddDate(0, 0, -20),
			UpdatedAt: time.Now().AddDate(0, 0, -5),
		},
	}

	response := UsersListResponse{
		Users:      users,
		Total:      len(users),
		Page:       1,
		PerPage:    10,
		TotalPages: 1,
	}

	return c.JSON(http.StatusOK, response)
}

// getAnalyticsHandler handles GET /api/analytics
func getAnalyticsHandler(c *core.RequestEvent) error {
	// Parse optional query parameters
	timeRange := c.Request.URL.Query().Get("time_range")
	if timeRange == "" {
		timeRange = "7d"
	}

	includeInactive := c.Request.URL.Query().Get("include_inactive") == "true"

	// Mock analytics data
	analytics := AnalyticsResponse{
		TotalUsers:  150,
		ActiveUsers: 120,
		UsersByAge: map[string]int{
			"18-25": 30,
			"26-35": 50,
			"36-45": 40,
			"46+":   30,
		},
		TagStats: map[string]int{
			"developer": 45,
			"admin":     15,
			"user":      90,
		},
		CreatedToday: 5,
		LastActivity: time.Now().Add(-time.Hour * 2),
		Trends: map[string]interface{}{
			"weekly_growth":    12.5,
			"monthly_growth":   8.3,
			"retention_rate":   0.85,
			"popular_features": []string{"dashboard", "reports", "settings"},
		},
	}

	if !includeInactive {
		analytics.TotalUsers = analytics.ActiveUsers
	}

	return c.JSON(http.StatusOK, analytics)
}

func helloJob(app core.App) error {
	return app.Cron().Add("helloWorld", "*/1 * * * *", func() {
		log.Println("Hello from cron job!")
	})
}

func exampleCollection(app core.App) error {
	// Example: Create a simple collection
	existingCollection, _ := app.FindCollectionByNameOrId("example_collection")
	if existingCollection != nil {
		app.Logger().Info("Example collection already exists")
		return nil
	}

	// Create new collection
	collection := core.NewBaseCollection("example_collection")

	// Find users collection for relation
	usersCollection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	// Add relation field to user FIRST
	collection.Fields.Add(&core.RelationField{
		Name:          "user",
		Required:      true,
		CollectionId:  usersCollection.Id,
		CascadeDelete: true,
	})

	// Set collection rules AFTER adding the relation field
	collection.ViewRule = types.Pointer("@request.auth.id != ''")
	collection.CreateRule = types.Pointer("@request.auth.id != ''")
	collection.UpdateRule = types.Pointer("@request.auth.id = user.id")
	collection.DeleteRule = types.Pointer("@request.auth.id = user.id")

	// Add other fields to collection
	collection.Fields.Add(&core.TextField{
		Name:     "title",
		Required: true,
		Max:      100,
	})

	// Add auto-date fields
	collection.Fields.Add(&core.AutodateField{
		Name:     "created",
		OnCreate: true,
	})

	collection.Fields.Add(&core.AutodateField{
		Name:     "updated",
		OnCreate: true,
		OnUpdate: true,
	})

	// Add index for user relation
	collection.AddIndex("idx_example_user", true, "user", "")

	// Save the collection
	if err := app.Save(collection); err != nil {
		app.Logger().Error("Failed to create example collection", "error", err)
		return err
	}

	app.Logger().Info("Created example collection")
	return nil
}
