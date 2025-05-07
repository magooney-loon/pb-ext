Create a root "frontend" dir for devving JS framework ala SvelteKit etc

go run cmd/scripts/main.go - builds frontend and runs server
go run cmd/scripts/main.go --install - installs dependencies, builds frontend, and runs server
go run cmd/scripts/main.go --build-only - only builds the frontend
go run cmd/scripts/main.go --run-only - only runs the server
go run cmd/scripts/main.go --production - creates a production build in the dist folder
go run cmd/scripts/main.go --production --dist customfolder - creates a production build in the customfolder directory