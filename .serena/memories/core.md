# Core

## Project Layout

- `kifu/` — Online Go Kifu Store & Editing Tool
  - `backend/` — Go 1.26+ REST API
  - `frontend/` — Svelte 5 + Vite + Materialize CSS (Bun package manager)
- `wasm-kifu/` — Go WASM Kifu Recording & Vision Library (OpenCV.js board detection)
  - Directly imported by `kifu/frontend` via Vite relative path configuration.

## Key Domains & References

- Technical details and dependencies: `mem:tech_stack`
- Project & environment specific commands: `mem:suggested_commands`
- Coding style, conventions and Svelte 5 details: `mem:conventions`
- Task completion Checklist & verification: `mem:task_completion`