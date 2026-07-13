# Task Completion

## Verification checklist
Before considering any task complete, perform these checks:

1. **Frontend check**:
   - Go to `kifu/frontend/`
   - Run `bun run lint`
   - Run `bun run build` (Ensures Svelte and TypeScript compiled cleanly)
2. **Backend check**:
   - Go to `kifu/backend/`
   - Run `go fmt ./...`
   - Run `go vet ./...`
   - Run `go test ./...`
3. **WASM check** (if modified):
   - Compile WASM using `GOOS=js GOARCH=wasm go build -o ../kifu/frontend/src/lib/kifu.wasm ./cmd/wasm` or custom script.
4. **Git Hook Verification**:
   - Run `bunx lefthook run pre-commit` to ensure all linters pass.