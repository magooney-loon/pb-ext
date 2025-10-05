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

Create a file named `main.go` in `cmd/server` and copy the following code:

```go
// Check cmd/server/main.go
	// For the full example
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

For more details on available field types and collection options, refer to the [PocketBase documentation](https://pocketbase.io/docs/collections/).

For more information on using PocketBase as a Go framework, refer to the [PocketBase documentation](https://pocketbase.io/docs/go-overview/).

## Access your application

- PocketBase Admin panel: `http://127.0.0.1:8090/_`
- pb-ext Dashboard: `http://127.0.0.1:8090/_/_`
- Your website: `http://127.0.0.1:8090/`
