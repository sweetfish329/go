# Tech Stack

## System Requirements
- OS: Windows 11
- Package Manager: Bun (strictly npm/npx free)

## Frontend (`kifu/frontend/`)
- Framework: Svelte 5 (using Runes mode)
- Build Tool: Vite
- CSS: Materialize CSS (via CDN)
- SGF parser/stringifier: `@sabaki/sgf` (along with `@types/sabaki__sgf` for type safety)

## Backend (`kifu/backend/`)
- Language: Go 1.26+
- Routing: net/http (Standard Library)
- Database: PostgreSQL 15
- SGF Operations: `github.com/rooklift/sgf` (standardized for SGF parsing & board rendering)

## WASM (`wasm-kifu/`)
- Logic: Go WASM (compiled using Go 1.26+)
- Image processing: OpenCV.js (custom board corner detection and color classification)
- Integration: Integrated into `kifu/frontend` via relative imports and Vite ambient declarations (`vite-env.d.ts`).