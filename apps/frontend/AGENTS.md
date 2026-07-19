# Frontend Instructions

These rules extend the repository-level `AGENTS.md` for files under
`apps/frontend/`.

## Stack

SvelteKit SSR application using Svelte 5 runes, strict TypeScript, Vite,
Tailwind v4, DaisyUI 5, and `@lucide/svelte`.

## Commands

Run from `apps/frontend/`:

```sh
npm run dev
npm run check
npm run lint
npm test
npm run build
```

## Data Flow

- Reads use SvelteKit `load` functions.
- Writes currently use route-level client handlers that call same-origin
  endpoints. Keep backend calls behind the SvelteKit origin unless an existing
  route already establishes another boundary.
- Browser SSE connections must use same-origin routes when possible so CSP
  stays tight and backend services are not exposed directly.
- Do not add a generic browser-side API proxy or fetch data on component mount
  when a route `load` can provide the data.

## Svelte and TypeScript

- Keep strict TypeScript enabled. Prefer `unknown` over `any` and map transport
  DTOs deliberately.
- Use Svelte 5 runes and SvelteKit primitives already established in the
  codebase.
- Do not add `eslint-disable`, `@ts-ignore`, `@ts-expect-error`, or similar
  suppressions to get checks passing. Fix the underlying issue unless a
  documented external API, generated code, or tooling false positive requires a
  narrow exception.

## UI Conventions

- Prefer DaisyUI components and Tailwind utilities in templates.
- Configure themes CSS-first through `@plugin "daisyui/theme"` in
  `src/app.css`; use `@theme` for custom tokens.
- Do not add app-specific CSS selectors or component classes to `src/app.css`.
- When repeated UI gets noisy, extract small Svelte components with static
  Tailwind/DaisyUI classes and semantic props. Avoid shared class-string maps
  and `${...}` class interpolation for design primitives; use Svelte `class:`
  directives for local state variants, following the sibling frontend style.
- Use `@lucide/svelte` icons. Add inline SVG only when Lucide cannot represent
  the symbol.

## SSR and Browser Security

- Security headers are set in `src/hooks.server.ts`.
- Never render user-controlled HTML directly. Validate user-controlled `href`
  and `src` values against an explicit scheme and origin policy.
- Keep secrets out of browser storage, generated assets, and client-visible
  URLs.
