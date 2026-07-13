# Conventions

## Global Conventions
- All git commits must be in Japanese.
- Avoid duplicate code copy-paste between `kifu/frontend` and `wasm-kifu`. Use relative imports configured in Vite (`@wasm-kifu` alias).

## Svelte 5 / Runes Mode
- Reactivity: Use `$state()`, `$derived()`, `$props()`. Avoid legacy `$:` syntax.
- For large read-only/replacement-only objects (e.g. API responses), use `$state.raw(...)` to avoid Proxy overhead.
- external JS/TS files containing runes MUST end with `.svelte.js` / `.svelte.ts`. Import them with explicit `.svelte` suffix (e.g., `import { x } from './x.svelte'`).
- Styling: For Materialize CDN classes, use Svelte `:global()` selector (e.g., `:global(.card-content)`) to bypass Svelte component scoping.

## Go Backend Invariants
- SGF parser build dependency order in `Containerfile`: Must `COPY . .` first before running `go mod tidy` so Go can scan imports.
- PostgreSQL Date Parse: Format raw SGF date (`DT` tag) using Go's regex-based query cleansing to Postgres compatible `YYYY-MM-DD` before inserting. Fallback to `time.Now()` on error.

## WASM Integration
- `syscall/js` is utilized for exposing Go functions (like `wasmNewGame`, `wasmExportSGF`) since `go:wasmexport` does not support string returns yet.
- Keep Vite's `server.fs.allow` containing `../wasm-kifu` in `vite.config.js` to allow cross-directory relative loads.