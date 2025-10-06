package main

// Collection example

import (
	"github.com/pocketbase/pocketbase/core"
)

// registerCollections sets up all database collections for the application
func registerCollections(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := todoCollection(e.App); err != nil {
			app.Logger().Error("Failed to create todo collection", "error", err)
		}

		return e.Next()
	})
}

// todoCollection creates a todos collection for CRUD demo
func todoCollection(app core.App) error {
	// Check if todos collection already exists
	existingCollection, _ := app.FindCollectionByNameOrId("todos")
	if existingCollection != nil {
		app.Logger().Info("Todos collection already exists")
		return nil
	}

	// Create new todos collection
	collection := core.NewBaseCollection("todos")

	// Find users collection for optional relation (v2 auth)
	usersCollection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	// Add optional user relation
	collection.Fields.Add(&core.RelationField{
		Name:          "user",
		Required:      false, // Optional - v1 routes won't use this
		CollectionId:  usersCollection.Id,
		CascadeDelete: true,
	})

	// Add title field (required)
	collection.Fields.Add(&core.TextField{
		Name:     "title",
		Required: true,
		Max:      200,
	})

	// Add description field (optional)
	collection.Fields.Add(&core.TextField{
		Name:     "description",
		Required: false,
		Max:      1000,
	})

	// Add completed field (boolean, default false)
	collection.Fields.Add(&core.BoolField{
		Name: "completed",
	})

	// Add priority field (select)
	collection.Fields.Add(&core.SelectField{
		Name:   "priority",
		Values: []string{"low", "medium", "high"},
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

	// Set collection rules - public access for v1
	collection.ViewRule = nil   // Public read
	collection.CreateRule = nil // Public create
	collection.UpdateRule = nil // Public update
	collection.DeleteRule = nil // Public delete

	// Add indexes
	collection.AddIndex("idx_todos_user", false, "user", "")
	collection.AddIndex("idx_todos_completed", false, "completed", "")

	// Save the collection
	if err := app.Save(collection); err != nil {
		app.Logger().Error("Failed to create todos collection", "error", err)
		return err
	}

	app.Logger().Info("Created todos collection")
	return nil
}
