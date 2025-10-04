package main

// API_SOURCE
// Use API_SOURCE to mark all routes in this file for AST API docs generation
import (
	"flag"
	"log"
	"net/http"

	"strconv"

	"time"

	app "github.com/magooney-loon/pb-ext/core"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

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

func registerRoutes(pbApp core.App) {
	pbApp.OnServe().BindFunc(func(e *core.ServeEvent) error {
		router := app.EnableAutoDocumentation(e)

		// Files marked with API_SOURCE directive are automatically discovered for AST parsing
		// This file is marked with the directive at the top, so handlers here will be parsed

		router.GET("/api/time", timeHandler)
		router.GET("/api/hello-auth", helloAuthHandler).Bind(apis.RequireAuth())
		router.GET("/api/guest-only", guestOnlyHandler).Bind(apis.RequireGuestOnly())
		router.GET("/api/superuser", superuserHandler).Bind(apis.RequireSuperuserAuth())
		router.GET("/api/user/:id", userProfileHandler).Bind(apis.RequireSuperuserOrOwnerAuth("id"))

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

// API_DESC Authenticated hello world
// API_TAGS auth,hello,secure
func helloAuthHandler(c *core.RequestEvent) error {
	userID := c.Auth.Id

	// Get user info for personalized response
	username := "User"
	if c.Auth.GetString("username") != "" {
		username = c.Auth.GetString("username")
	} else if c.Auth.GetString("email") != "" {
		username = c.Auth.GetString("email")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Hello Auth World! 🔒",
		"user": map[string]any{
			"id":            userID,
			"username":      username,
			"authenticated": true,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// API_DESC Demonstrates guest-only access for unauthenticated users
// API_TAGS guest,public,demo
func guestOnlyHandler(c *core.RequestEvent) error {
	return c.JSON(http.StatusOK, map[string]any{
		"message":   "Guest Only Access! 👤",
		"info":      "This endpoint requires the user to be unauthenticated (guest)",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// API_DESC Demonstrates superuser-only access for admin operations
// API_TAGS superuser,admin,secure
func superuserHandler(c *core.RequestEvent) error {
	userID := c.Auth.Id
	username := "Superuser"
	if c.Auth.GetString("username") != "" {
		username = c.Auth.GetString("username")
	} else if c.Auth.GetString("email") != "" {
		username = c.Auth.GetString("email")
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "Superuser Access! 👑",
		"info":    "This endpoint requires superuser authentication",
		"user": map[string]any{
			"id":       userID,
			"username": username,
			"role":     "superuser",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// API_DESC Get user profile with superuser or owner access control
// API_TAGS users,profile,owner,access-control
func userProfileHandler(c *core.RequestEvent) error {
	requestedID := c.Request.PathValue("id")
	currentUserID := c.Auth.Id

	isOwner := currentUserID == requestedID
	isSuperuser := c.Auth.IsSuperuser()

	accessType := "owner"
	if isSuperuser && !isOwner {
		accessType = "superuser"
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message": "User Profile Access! 🔐",
		"info":    "This endpoint requires superuser or record owner authentication",
		"access": map[string]any{
			"type":            accessType,
			"is_owner":        isOwner,
			"is_superuser":    isSuperuser,
			"requested_id":    requestedID,
			"current_user_id": currentUserID,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
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
