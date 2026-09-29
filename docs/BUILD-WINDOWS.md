# Windows build

Use Go 1.23+, Node.js 22.12+ and Wails CLI v2.15.0.

```powershell
cd C:\path\to\jtty-go

go mod tidy
go test ./internal/... ./tests/...

cd frontend
npm install
npm run build
cd ..

wails dev
```

For a production Windows x64 executable:

```powershell
wails build -platform windows/amd64 -clean
```

With Wails v2.15.0, `frontend:dev:build` is the short-lived development build
step. `frontend:dev:watcher` is the separate long-running Vite process.
