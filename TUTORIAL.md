# Beginner's Guide to pb-ext

This guide will help you get started with pb-ext even if you're new to Golang.

## What is pb-ext?

pb-ext is a wrapper around PocketBase that adds enhanced monitoring, logging, and analytics features. Since it builds on top of PocketBase, you can continue using all PocketBase features and documentation for extending it as a Go framework.

## Prerequisites

1. Install Golang:
   - Download from [golang.org/dl](https://go.dev/dl/)
   - Follow the installation instructions for your OS
   - Verify installation with `go version`

## Step-by-Step Setup

### 1. Create a project folder

```bash
mkdir my-pb-project
cd my-pb-project
```

### 2. Initialize Go module

```bash
go mod init my-pb-project
```

### 3. Create main.go file

Create a file named `main.go` in your project root and copy the following code:

```go
package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	app "github.com/magooney-loon/pb-ext/core"
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
```

### 4. Download dependencies

```bash
go mod tidy
```

This will download pb-ext and all required dependencies.

### 5. Run your server

```bash
go run . serve
```

Your server should now be running!

### 6. Add Static Files (Website)

PocketBase automatically serves static files from the `pb_public` folder. Create this folder and add an `index.html` file:

```bash
mkdir pb_public
```

Create a file `pb_public/index.html` with basic content:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>My PocketBase App</title>
    <style>
        body {
            font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            line-height: 1.6;
        }
        h1 {
            color: #333;
        }
        .card {
            border: 1px solid #ddd;
            border-radius: 8px;
            padding: 20px;
            margin-top: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
    </style>
</head>
<body>
    <h1>Welcome to my PocketBase App!</h1>
    <div class="card">
        <h2>Getting Started</h2>
        <p>Your static website is now live! You can modify the files in the <code>pb_public</code> folder to build your frontend.</p>
        <p>The time API endpoint is available at: <a href="/api/time">/api/time</a></p>
    </div>
</body>
</html>
```

Once you restart your server, you can access your website at `http://127.0.0.1:8090/`. PocketBase automatically serves the `index.html` file from the `pb_public` folder as the root route.

### 7. Adding Custom Collections

To add your own collections, follow the same pattern as our example. Create a new function for each collection:

```go
func myCustomCollection(app core.App) error {
    // Check if collection exists
    existingCollection, _ := app.FindCollectionByNameOrId("my_collection")
    if existingCollection != nil {
        app.Logger().Info("My collection already exists")
        return nil
    }

    // Create new collection
    collection := core.NewBaseCollection("my_collection")

    // Add your fields here
    collection.Fields.Add(&core.TextField{
        Name:     "title",
        Required: true,
        Max:      100,
    })

    // Save the collection
    if err := app.Save(collection); err != nil {
        return err
    }

    return nil
}
```

Then add it to your `registerCollections` function:

```go
func registerCollections(app core.App) {
    app.OnServe().BindFunc(func(e *core.ServeEvent) error {
        if err := exampleCollection(e.App); err != nil {
            app.Logger().Error("Failed to create example collection", "error", err)
        }
        if err := myCustomCollection(e.App); err != nil {
            app.Logger().Error("Failed to create my collection", "error", err)
        }
        return e.Next()
    })
}
```

For more details on available field types and collection options, refer to the [PocketBase documentation](https://pocketbase.io/docs/collections/).

## Access your application

- PocketBase Admin panel: `http://127.0.0.1:8090/_`
- pb-ext Dashboard: `http://127.0.0.1:8090/_/_`
- Default example route: `http://127.0.0.1:8090/api/time`
- Your website: `http://127.0.0.1:8090/`

## Next Steps

Now that you have pb-ext running, you can:

1. Use the PocketBase Admin UI to create collections and manage your data
2. Use the pb-ext dashboard to monitor your server health and visitor analytics
3. Extend your application with additional routes and functionality
4. Build your frontend in the `pb_public` folder

For more information on using PocketBase as a Go framework, refer to the [PocketBase documentation](https://pocketbase.io/docs/go-overview/).