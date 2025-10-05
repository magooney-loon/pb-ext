package main

// API_SOURCE
// Shared handlers across all API versions demonstrating HTTP methods and auth patterns

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Request types
type PostRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
}

type PostPatchRequest struct {
	Title   *string   `json:"title,omitempty"`
	Content *string   `json:"content,omitempty"`
	Tags    *[]string `json:"tags,omitempty"`
	Status  *string   `json:"status,omitempty"`
}

// =============================================================================
// Public Handlers (No Auth Required)
// =============================================================================

// API_DESC Get current server time in multiple formats
// API_TAGS public,utility,time
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

// =============================================================================
// Guest-Only Handlers
// =============================================================================

// API_DESC Get onboarding information for guest users
// API_TAGS guest,onboarding
func guestInfoHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"message": "Welcome Guest! 👋",
		"info":    "This endpoint is only accessible to unauthenticated users",
		"onboarding": map[string]any{
			"signup_url":   "/signup",
			"login_url":    "/login",
			"features":     []string{"Create Posts", "Join Community", "Access Premium Content"},
			"trial_period": "7 days",
		},
		"public_stats": map[string]any{
			"total_posts":  1250,
			"active_users": 89,
			"categories":   15,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// =============================================================================
// Authenticated User Handlers
// =============================================================================

// API_DESC Create a new post
// API_TAGS posts,create,authenticated
func createPostHandler(c *core.RequestEvent) error {
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "Authentication required"})
	}

	var req PostRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid JSON payload"})
	}

	if req.Title == "" || req.Content == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Title and content are required"})
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"message": "Post created successfully! 📝",
		"post": map[string]any{
			"id":      generateID(),
			"title":   req.Title,
			"content": req.Content,
			"tags":    req.Tags,
			"author": map[string]any{
				"id":       c.Auth.Id,
				"username": getUserDisplayName(c),
			},
			"status":     "draft",
			"created_at": time.Now().Format(time.RFC3339),
		},
	})
}

// API_DESC Partially update a post
// API_TAGS posts,update,patch,authenticated
func patchPostHandler(c *core.RequestEvent) error {
	postID := c.Request.PathValue("id")
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "Authentication required"})
	}

	var req PostPatchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid JSON payload"})
	}

	// Simulate ownership check (would query DB in real app)
	isOwner := true
	if !isOwner {
		return c.JSON(http.StatusForbidden, map[string]any{"error": "You can only update your own posts"})
	}

	updates := make(map[string]any)
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.Tags != nil {
		updates["tags"] = *req.Tags
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Post updated successfully! ✏️",
		"post": map[string]any{
			"id":         postID,
			"updates":    updates,
			"updated_at": time.Now().Format(time.RFC3339),
		},
	})
}

// =============================================================================
// Superuser or Owner Handlers
// =============================================================================

// API_DESC Fully update/replace a post
// API_TAGS posts,update,put,superuser,owner
func updatePostHandler(c *core.RequestEvent) error {
	postID := c.Request.PathValue("id")
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "Authentication required"})
	}

	isSuperuser := c.Auth.IsSuperuser()
	isOwner := c.Auth.Id == "example_owner_id" // Simulated ownership check

	var req PostRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Invalid JSON payload"})
	}

	if req.Title == "" || req.Content == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "Title and content are required"})
	}

	accessType := "owner"
	if isSuperuser && !isOwner {
		accessType = "superuser"
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Post fully updated! 🔄",
		"post": map[string]any{
			"id":         postID,
			"title":      req.Title,
			"content":    req.Content,
			"tags":       req.Tags,
			"status":     "published",
			"updated_at": time.Now().Format(time.RFC3339),
		},
		"access": map[string]any{
			"type":         accessType,
			"is_owner":     isOwner,
			"is_superuser": isSuperuser,
		},
	})
}

// =============================================================================
// Superuser Only Handlers
// =============================================================================

// API_DESC Delete a post
// API_TAGS posts,delete,superuser
func deletePostHandler(c *core.RequestEvent) error {
	postID := c.Request.PathValue("id")
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "Authentication required"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Post deleted successfully! 🗑️",
		"deleted_post": map[string]any{
			"id": postID,
		},
		"deleted_by": map[string]any{
			"id":       c.Auth.Id,
			"username": getUserDisplayName(c),
			"role":     "superuser",
		},
		"deleted_at": time.Now().Format(time.RFC3339),
	})
}

// API_DESC Get admin dashboard statistics
// API_TAGS admin,stats,superuser
func adminStatsHandler(c *core.RequestEvent) error {
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{"error": "Authentication required"})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Admin Dashboard 👑",
		"stats": map[string]any{
			"total_posts":     1250,
			"published_posts": 1100,
			"draft_posts":     150,
			"total_users":     2450,
			"active_users":    1890,
			"new_users_today": 15,
			"reports":         3,
			"storage_used":    "2.3 GB",
			"bandwidth_month": "45.2 GB",
		},
		"recent_activity": []map[string]any{
			{
				"action":    "user_registered",
				"timestamp": time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
				"details":   "New user: john_doe",
			},
			{
				"action":    "post_published",
				"timestamp": time.Now().Add(-25 * time.Minute).Format(time.RFC3339),
				"details":   "Post: 'Getting started with Go'",
			},
		},
		"admin": map[string]any{
			"id":       c.Auth.Id,
			"username": getUserDisplayName(c),
		},
		"generated_at": time.Now().Format(time.RFC3339),
	})
}

// =============================================================================
// Utility Functions
// =============================================================================

func getUserDisplayName(c *core.RequestEvent) string {
	if c.Auth == nil {
		return "Anonymous"
	}
	if username := c.Auth.GetString("username"); username != "" {
		return username
	}
	if email := c.Auth.GetString("email"); email != "" {
		return email
	}
	return "User"
}

func generateID() string {
	return fmt.Sprintf("post_%d", time.Now().UnixNano())
}
