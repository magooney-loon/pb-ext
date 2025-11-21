# PocketBase + SvelteKit Frontend

## Quick Start

The easiest way to start building a SvelteKit app:

```bash
npx sv create frontend
```

## Configuration

- Make sure you choose the **static-adapter** when prompted
- Use the scripts in `cmd/scripts/main.go` for building
- Create a root `+layout.ts` file and add:

```typescript
export const prerender = true;
export const trailingSlash = 'always';
```

## Prebuilt Template

For a ready-to-use starter template, check out: https://github.com/magooney-loon/svelte-gui