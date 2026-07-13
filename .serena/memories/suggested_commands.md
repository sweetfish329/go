# Suggested Commands

## Frontend Development
- Directory: `kifu/frontend/`
- Start dev server: `bun run dev`
- Run build check: `bun run build`
- Linter & Formatter check: `bun run lint`

## Backend Development
- Directory: `kifu/backend/`
- Run local server: `go run cmd/server/main.go`
- Run tests: `go test ./...`

## Container Orchestration (Windows Podman Compose)
- Directory: `kifu/`
- Spin up services: `podman-compose up -d`
- Stop services: `podman-compose down`
- Check logs: `podman-compose logs`
- Access Web App: http://localhost:8822 (Port 8080 is bypassed due to host conflicts on Windows)

## Git Hooks
- Run pre-commit checks: `bunx lefthook run pre-commit`