package main

// API_SOURCE

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Request types
type TodoRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"` // low, medium, high
	Completed   bool   `json:"completed"`
}

type TodoPatchRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Priority    *string `json:"priority,omitempty"`
	Completed   *bool   `json:"completed,omitempty"`
}

// API_DESC Get current server time in multiple formats
// API_TAGS Utility
func timeHandler(c *core.RequestEvent) error {
	now := time.Now()
	return c.JSON(http.StatusOK, map[string]any{
		"time": map[string]string{
			"iso":       now.Format(time.RFC3339),
			"unix":      strconv.FormatInt(now.Unix(), 10),
			"unix_nano": strconv.FormatInt(now.UnixNano(), 10),
			"utc":       now.UTC().Format(time.RFC3339),
		},
		"server":  "pb-ext",
		"version": "1.0.0",
	})
}

// API_DESC Create a new todo item
// API_TAGS Todos
func createTodoHandler(c *core.RequestEvent) error {
	// Check authentication - required for creation
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "Authentication required"})
	}

	var req TodoRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid JSON payload"})
	}

	if req.Title == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Title is required"})
	}

	// Validate priority if provided
	if req.Priority != "" && req.Priority != "low" && req.Priority != "medium" && req.Priority != "high" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Priority must be 'low', 'medium', or 'high'"})
	}

	// Default priority to medium if not provided
	if req.Priority == "" {
		req.Priority = "medium"
	}

	// Create todo record data
	todoData := map[string]any{
		"title":       req.Title,
		"description": req.Description,
		"priority":    req.Priority,
		"completed":   req.Completed,
	}

	// Only set user field if authenticated user is from users collection
	// Superusers/admins don't have records in users collection
	if c.Auth.Collection().Name == "users" {
		todoData["user"] = c.Auth.Id
	}
	// If authenticated as superuser, leave user field empty or handle differently

	// Create record in todos collection
	collection, err := c.App.FindCollectionByNameOrId("todos")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "Collection not found"})
	}

	record := core.NewRecord(collection)
	record.Load(todoData)

	if err := c.App.Save(record); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "Failed to create todo"})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"message": "Todo created successfully! ✅",
		"todo": map[string]any{
			"id":          record.Id,
			"title":       record.GetString("title"),
			"description": record.GetString("description"),
			"priority":    record.GetString("priority"),
			"completed":   record.GetBool("completed"),
			"created_at":  record.GetDateTime("created"),
			"user_id":     record.GetString("user"),
			"created_by":  c.Auth.Collection().Name, // Show if created by user or admin
		},
	})
}

// API_DESC Get all todos with optional filtering
// API_TAGS Todos
func getTodosHandler(c *core.RequestEvent) error {
	collection, err := c.App.FindCollectionByNameOrId("todos")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "Collection not found"})
	}

	// Build query with optional filters
	filter := ""
	filterParams := make(map[string]any)

	// Filter by completion status if provided
	if completed := c.Request.URL.Query().Get("completed"); completed != "" {
		if completed == "true" || completed == "1" {
			filter = "completed = true"
		} else if completed == "false" || completed == "0" {
			filter = "completed = false"
		}
	}

	// Filter by priority if provided
	if priority := c.Request.URL.Query().Get("priority"); priority != "" {
		if filter != "" {
			filter += " && "
		}
		filter += "priority = {:priority}"
		filterParams["priority"] = priority
	}

	// For authenticated requests, filter by user (only if user is from users collection)
	if c.Auth != nil && c.Auth.Collection().Name == "users" {
		if filter != "" {
			filter += " && "
		}
		filter += "user = {:userId}"
		filterParams["userId"] = c.Auth.Id
	}

	records, err := c.App.FindRecordsByFilter(collection, filter, "-created", 100, 0, filterParams)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "Failed to fetch todos"})
	}

	todos := make([]map[string]any, len(records))
	for i, record := range records {
		todos[i] = map[string]any{
			"id":          record.Id,
			"title":       record.GetString("title"),
			"description": record.GetString("description"),
			"priority":    record.GetString("priority"),
			"completed":   record.GetBool("completed"),
			"created_at":  record.GetDateTime("created"),
			"updated_at":  record.GetDateTime("updated"),
		}

		// Include user info if available
		if userId := record.GetString("user"); userId != "" {
			todos[i]["user_id"] = userId
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Todos retrieved successfully 📋",
		"todos":   todos,
		"count":   len(todos),
		"filters": map[string]any{
			"completed": c.Request.URL.Query().Get("completed"),
			"priority":  c.Request.URL.Query().Get("priority"),
		},
	})
}

// API_DESC Get a specific todo by ID
// API_TAGS Todos
func getTodoHandler(c *core.RequestEvent) error {
	todoID := c.Request.PathValue("id")

	collection, err := c.App.FindCollectionByNameOrId("todos")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "Collection not found"})
	}

	record, err := c.App.FindRecordById(collection, todoID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "Todo not found"})
	}

	// For authenticated requests, check ownership (only enforce for regular users)
	if c.Auth != nil && c.Auth.Collection().Name == "users" {
		if userID := record.GetString("user"); userID != "" && userID != c.Auth.Id {
			return c.JSON(http.StatusForbidden, map[string]any{"error": "Access denied"})
		}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Todo retrieved successfully 📖",
		"todo": map[string]any{
			"id":          record.Id,
			"title":       record.GetString("title"),
			"description": record.GetString("description"),
			"priority":    record.GetString("priority"),
			"completed":   record.GetBool("completed"),
			"created_at":  record.GetDateTime("created"),
			"updated_at":  record.GetDateTime("updated"),
			"user_id":     record.GetString("user"),
		},
	})
}

// API_DESC Update a todo item (partial update)
// API_TAGS Todos
func updateTodoHandler(c *core.RequestEvent) error {
	// Check authentication - required for updates
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "Authentication required"})
	}

	todoID := c.Request.PathValue("id")

	collection, err := c.App.FindCollectionByNameOrId("todos")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "Collection not found"})
	}

	record, err := c.App.FindRecordById(collection, todoID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "Todo not found"})
	}

	// Check ownership - regular users can only update their own todos, superusers can update any
	if c.Auth.Collection().Name == "users" {
		if userID := record.GetString("user"); userID != c.Auth.Id {
			return c.JSON(http.StatusForbidden, map[string]any{"error": "Access denied - you can only update your own todos"})
		}
	}

	var req TodoPatchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid JSON payload"})
	}

	// Apply updates
	updates := make(map[string]any)
	if req.Title != nil {
		if *req.Title == "" {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "Title cannot be empty"})
		}
		record.Set("title", *req.Title)
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		record.Set("description", *req.Description)
		updates["description"] = *req.Description
	}
	if req.Priority != nil {
		if *req.Priority != "low" && *req.Priority != "medium" && *req.Priority != "high" {
			return c.JSON(http.StatusBadRequest, map[string]any{"error": "Priority must be 'low', 'medium', or 'high'"})
		}
		record.Set("priority", *req.Priority)
		updates["priority"] = *req.Priority
	}
	if req.Completed != nil {
		record.Set("completed", *req.Completed)
		updates["completed"] = *req.Completed
	}

	if err := c.App.Save(record); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "Failed to update todo"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Todo updated successfully! ✏️",
		"todo": map[string]any{
			"id":          record.Id,
			"title":       record.GetString("title"),
			"description": record.GetString("description"),
			"priority":    record.GetString("priority"),
			"completed":   record.GetBool("completed"),
			"updated_at":  record.GetDateTime("updated"),
		},
		"updates": updates,
	})
}

// API_DESC Delete a todo item
// API_TAGS Todos
func deleteTodoHandler(c *core.RequestEvent) error {
	// Check authentication - required for deletion
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "Authentication required"})
	}

	todoID := c.Request.PathValue("id")

	collection, err := c.App.FindCollectionByNameOrId("todos")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "Collection not found"})
	}

	record, err := c.App.FindRecordById(collection, todoID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"error": "Todo not found"})
	}

	// Check ownership - regular users can only delete their own todos, superusers can delete any
	if c.Auth.Collection().Name == "users" {
		if userID := record.GetString("user"); userID != c.Auth.Id {
			return c.JSON(http.StatusForbidden, map[string]any{"error": "Access denied - you can only delete your own todos"})
		}
	}

	if err := c.App.Delete(record); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "Failed to delete todo"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Todo deleted successfully! 🗑️",
		"deleted_todo": map[string]any{
			"id":    todoID,
			"title": record.GetString("title"),
		},
		"deleted_at": time.Now().Format(time.RFC3339),
	})
}
