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

## OGP & Grid Layout (Updated 2026-07-17)
- **Database/Model**: Added `HasOgp` flag to `model.Kifu` in the backend. It checks `(ogp_image IS NOT NULL AND octet_length(ogp_image) > 0)` dynamically in `FindAllByUser` and `FindAllPublicByUser` queries.
- **API**: Added `GET /api/kifus/{id}/og-image` to retrieve the owner's own private OGP image.
- **Frontend Grid**: Modified `KifuList.svelte` to display games as a grid card layout.
  - OGP exists: Renders a square 1:1 image using `aspect-ratio: 1/1` and `object-fit: cover`.
  - OGP missing: Displays an elegant Go board SVG placeholder that fits perfectly within the Washi Clay theme.
  - Delete Button: Positioned absolutely at the bottom-right and visually separated to prevent click propagation conflicts.