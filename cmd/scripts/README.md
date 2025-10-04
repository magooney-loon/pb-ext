## 📋 Command Reference

| Command | Description | Example Output |
|---------|-------------|----------------|
| `go run cmd/scripts/main.go` | 🔄 **Development Build** | Builds frontend + starts server |
| `go run cmd/scripts/main.go --install` | 📦 **Install + Build** | Downloads deps + builds + runs |
| `go run cmd/scripts/main.go --build-only` | 🔨 **Build Only** | Just builds, doesn't run server |
| `go run cmd/scripts/main.go --run-only` | ▶️ **Run Only** | Skips build, just runs server |
| `go run cmd/scripts/main.go --production` | 🚀 **Production Build** | Creates optimized dist package |
| `go run cmd/scripts/main.go --test-only` | 🧪 **Test Suite** | Runs tests and generates reports |
| `go run cmd/scripts/main.go --production --dist <dir>` | 📁 **Custom Output** | Production build to custom dir |
| `go run cmd/scripts/main.go --help` | ❓ **Show Help** | Displays all available flags and options |
