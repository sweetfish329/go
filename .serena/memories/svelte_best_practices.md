# Svelte 5 & General Svelte Best Practices

## Documentation References
- Main Index: https://svelte.dev/llms.txt
- Svelte Developer Documentation: https://svelte.dev/docs/svelte/llms.txt
- SvelteKit Documentation: https://svelte.dev/docs/kit/llms.txt

## Key Concepts & Best Practices (Svelte 5 Runes)

### 1. Reactivity via Runes
Svelte 5 introduces **Runes** for reactivity. Legacy syntax like reactive statements (`$:`) and magic variables like `$$props` or `$$restProps` are disabled in runes mode.
- Use `$state()` to declare reactive state.
- Use `$derived()` to declare derived state.
- Use `$effect()` as an escape hatch for side-effects.

### 2. State (`$state`)
- **Use raw state for read-only / replacement-only data:** Objects and arrays declared with `$state(...)` are made deeply reactive via Proxies, which introduces performance overhead. For large objects (like API responses) that are only ever reassigned (not mutated), use `$state.raw(...)` instead.
- `$state.raw(...)` values cannot be mutated; they must be reassigned to trigger updates (e.g., replace the whole array/object).
- **TypeScript Casting:** If `$state()` has no initial value, its type will include `undefined`. Use `as` casting (e.g. `let count = $state() as number`) if you know the state will be initialized before use (useful in classes).
- Only use `$state` for variables that actually cause template or reactive updates. Keep other variables as normal constants/variables.

### 3. Derived State (`$derived`)
- Always prefer `$derived()` to synchronize state.
- Svelte uses **push-pull reactivity**: when state updates, dependents are notified immediately (push), but the derived values are not re-evaluated until read (pull).
- If the new derived value is referentially identical to the old one, downstream updates are skipped.

### 4. Effects (`$effect`)
- **Escape Hatch:** Treat `$effect` as an escape hatch. Use it for analytics, logging, or direct DOM manipulation.
- **Do NOT use `$effect` to synchronize state.** Use `$derived` instead.
- Use `$effect.pre` to run the effect before the DOM updates.
- Use `$effect.tracking()` (advanced) to check if the current context is running within a tracking context.

### 5. Props (`$props`)
- Use `$props()` to declare component properties.
- You must use an **object destructuring pattern** with `$props()` (e.g., `let { name, age } = $props();`).
- Do not use nested patterns or computed keys in the destructuring of `$props()`.
- **Children Prop:** Child nodes are passed as the `children` prop. Render them in the template using `{@render children()}` or `{@render children?.()}`. Avoid naming custom props `children`.

### 6. Built-in Reactivity Classes
- Imported from `svelte/reactivity`, Svelte provides reactive classes like `Set`, `Map`, `Date`, and `URL`. Use them when you need reactive collections or objects.

### 7. Runes in External JavaScript/TypeScript Files
- **Mandatory File Extension:** If you use Svelte 5 runes (`$state()`, `$derived()`, etc.) inside a separate, external JS/TS module (not inside a `.svelte` component), you **must** use the `.svelte.js` or `.svelte.ts` file extension.
- **ReferenceError Prevention:** If you use normal `.js` or `.ts` extensions, the Svelte compiler will bypass these files. The runes will not be compiled, resulting in a runtime `ReferenceError: $state is not defined` and crashing the application (causing a blank page).
- **Import Suffix Requirement:** When importing symbols from a `.svelte.ts` / `.svelte.js` file, you must specify the `.svelte` suffix (e.g., `import { auth } from './lib/auth.svelte'`) to allow Vite / TypeScript to resolve the compiled outputs correctly. Do not omit the extension or use `.ts`.