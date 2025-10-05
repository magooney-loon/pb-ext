package main

// API_SOURCE
// HTTP handlers demonstrating various HTTP methods and authentication types

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// PostRequest represents the request body for creating/updating posts
type PostRequest struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
}

// PostPatchRequest represents the request body for partially updating posts
type PostPatchRequest struct {
	Title   *string   `json:"title,omitempty"`
	Content *string   `json:"content,omitempty"`
	Tags    *[]string `json:"tags,omitempty"`
	Status  *string   `json:"status,omitempty"`
}

// =============================================================================
// Public Endpoints (No Authentication Required)
// =============================================================================

// API_DESC Get current server time in multiple formats
// API_TAGS v1,public,utility,time
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
// Guest-Only Endpoints (Unauthenticated Users Only)
// =============================================================================

// API_DESC Get information and onboarding data for guest users
// API_TAGS v1,guest,onboarding,public
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
// Authenticated User Endpoints (Any Authenticated User)
// =============================================================================

// API_DESC Create a new post
// API_TAGS v2,posts,create,authenticated
func createPostHandler(c *core.RequestEvent) error {
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error": "Authentication required",
		})
	}

	userID := c.Auth.Id
	username := getUserDisplayName(c)

	var req PostRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "Invalid JSON payload",
			"details": err.Error(),
		})
	}

	// Basic validation
	if req.Title == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "Title is required",
		})
	}

	if req.Content == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "Content is required",
		})
	}

	// Simulate post creation
	postID := generateID()

	return c.JSON(http.StatusCreated, map[string]any{
		"message": "Post created successfully! 📝",
		"post": map[string]any{
			"id":      postID,
			"title":   req.Title,
			"content": req.Content,
			"tags":    req.Tags,
			"author": map[string]any{
				"id":       userID,
				"username": username,
			},
			"status":     "draft",
			"created_at": time.Now().Format(time.RFC3339),
			"updated_at": time.Now().Format(time.RFC3339),
		},
	})
}

// API_DESC Partially update a post (authenticated users can update their own posts)
// API_TAGS v2,posts,update,partial,authenticated
func patchPostHandler(c *core.RequestEvent) error {
	postID := c.Request.PathValue("id")
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error": "Authentication required",
		})
	}

	userID := c.Auth.Id
	username := getUserDisplayName(c)

	var req PostPatchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "Invalid JSON payload",
			"details": err.Error(),
		})
	}

	// Simulate ownership check (in real app, query database)
	isOwner := true // Simulated

	if !isOwner {
		return c.JSON(http.StatusForbidden, map[string]any{
			"error": "You can only update your own posts",
		})
	}

	// Build update info
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
			"id":      postID,
			"updates": updates,
			"author": map[string]any{
				"id":       userID,
				"username": username,
			},
			"updated_at": time.Now().Format(time.RFC3339),
		},
	})
}

// =============================================================================
// Superuser or Owner Endpoints
// =============================================================================

// API_DESC Fully update/replace a post (superuser or post owner)
// API_TAGS v2-beta,posts,update,replace,superuser,owner
func updatePostHandler(c *core.RequestEvent) error {
	postID := c.Request.PathValue("id")
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error": "Authentication required",
		})
	}

	currentUserID := c.Auth.Id
	isSuperuser := c.Auth.IsSuperuser()

	// Simulate ownership check
	postOwnerID := "example_owner_id" // In real app, get from database
	isOwner := currentUserID == postOwnerID

	var req PostRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error":   "Invalid JSON payload",
			"details": err.Error(),
		})
	}

	// Validate required fields
	if req.Title == "" || req.Content == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{
			"error": "Title and content are required",
		})
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
// Superuser Only Endpoints
// =============================================================================

// API_DESC Delete a post (admin only)
// API_TAGS v1,posts,delete,admin,superuser
func deletePostHandler(c *core.RequestEvent) error {
	postID := c.Request.PathValue("id")
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error": "Authentication required",
		})
	}

	userID := c.Auth.Id
	username := getUserDisplayName(c)

	// Simulate post existence check
	postExists := true
	if !postExists {
		return c.JSON(http.StatusNotFound, map[string]any{
			"error": "Post not found",
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Post deleted successfully! 🗑️",
		"deleted_post": map[string]any{
			"id": postID,
		},
		"deleted_by": map[string]any{
			"id":       userID,
			"username": username,
			"role":     "superuser",
		},
		"deleted_at": time.Now().Format(time.RFC3339),
	})
}

// API_DESC Get admin dashboard statistics
// API_TAGS admin,statistics,dashboard,superuser
func adminStatsHandler(c *core.RequestEvent) error {
	if c.Auth == nil {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"error": "Authentication required",
		})
	}

	userID := c.Auth.Id
	username := getUserDisplayName(c)

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
			{
				"action":    "post_reported",
				"timestamp": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				"details":   "Post flagged for review",
			},
		},
		"admin": map[string]any{
			"id":       userID,
			"username": username,
		},
		"generated_at": time.Now().Format(time.RFC3339),
	})
}

// =============================================================================
// Utility Functions
// =============================================================================

// getUserDisplayName extracts display name from authenticated user
func getUserDisplayName(c *core.RequestEvent) string {
	if c.Auth == nil {
		return "Anonymous"
	}
	if c.Auth.GetString("username") != "" {
		return c.Auth.GetString("username")
	}
	if c.Auth.GetString("email") != "" {
		return c.Auth.GetString("email")
	}
	return "User"
}

// generateID generates a simple ID for demo purposes
func generateID() string {
	return fmt.Sprintf("post_%d", time.Now().UnixNano())
}
