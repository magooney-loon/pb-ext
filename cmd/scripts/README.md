## 📋 Command Reference

### 🔄 Development Build
Builds frontend + starts server in dev mode
```bash
go run cmd/scripts/main.go
```

### 📦 Install + Build  
Downloads deps + builds + runs in dev mode
```bash
go run cmd/scripts/main.go --install
```

### 🔨 Build Only
Just builds, doesn't run server
```bash
go run cmd/scripts/main.go --build-only
```

### ▶️ Run Only
Skips build, runs server in dev mode
```bash
go run cmd/scripts/main.go --run-only
```

### 🚀 Production Build
Creates optimized production binary
```bash
go run cmd/scripts/main.go --production
```

### 🧪 Test Suite
Runs tests and generates reports
```bash
go run cmd/scripts/main.go --test-only
```

### ❓ Show Help
Displays all available flags and options
```bash
go run cmd/scripts/main.go --help
```
