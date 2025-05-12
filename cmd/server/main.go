package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	app "github.com/magooney-loon/pb-ext/core"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func main() {
	initApp()
}

func initApp() {
	srv := app.New()

	app.SetupLogging(srv)

	registerCollections(srv.App())

	srv.App().OnServe().BindFunc(func(e *core.ServeEvent) error {
		app.SetupRecovery(srv.App(), e)
		return e.Next()
	})

	registerRoutes(srv.App())

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
		e.Router.GET("/api/time", func(c *core.RequestEvent) error {
			now := time.Now()
			return c.JSON(http.StatusOK, map[string]any{
				"time": map[string]string{
					"iso":       now.Format(time.RFC3339),
					"unix":      strconv.FormatInt(now.Unix(), 10),
					"unix_nano": strconv.FormatInt(now.UnixNano(), 10),
					"utc":       now.UTC().Format(time.RFC3339),
				},
			})
		})

		// Serve static files from pb_public with improved path resolution
		publicDirPath := "./pb_public"

		// Check if the directory exists
		if _, err := os.Stat(publicDirPath); os.IsNotExist(err) {
			// Try with absolute path
			exePath, err := os.Executable()
			if err == nil {
				exeDir := filepath.Dir(exePath)
				possiblePaths := []string{
					filepath.Join(exeDir, "pb_public"),
					filepath.Join(exeDir, "../pb_public"),
					filepath.Join(exeDir, "../../pb_public"),
				}

				for _, path := range possiblePaths {
					if _, err := os.Stat(path); err == nil {
						publicDirPath = path
						app.Logger().Info("Using pb_public from absolute path", "path", publicDirPath)
						break
					}
				}
			}
		}

		app.Logger().Info("Serving static files from", "path", publicDirPath)
		e.Router.GET("/{path...}", apis.Static(os.DirFS(publicDirPath), false))

		// You can use POST /api/collections/users/records to create a new user
		// See PocketBase documentation for more details: https://pocketbase.io/docs/api-records/

		return e.Next()
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
