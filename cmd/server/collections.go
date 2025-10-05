package main

// Collection definitions and database setup for the pb-ext server

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// registerCollections sets up all database collections for the application
func registerCollections(app core.App) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := exampleCollection(e.App); err != nil {
			app.Logger().Error("Failed to create example collection", "error", err)
		}

		return e.Next()
	})
}

// exampleCollection creates an example collection with various field types and relationships
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
